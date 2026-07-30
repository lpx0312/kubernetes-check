package collector

import (
	"testing"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s-patrol/internal/report"
)

// TestClassifyStorageHealth 验证 PVC 行的健康度判定。
func TestClassifyStorageHealth(t *testing.T) {
	tests := []struct {
		name     string
		row      report.StorageRow
		expected string
	}{
		{
			name:     "Bound未挂载_健康",
			row:      report.StorageRow{Phase: "Bound", Mounted: false},
			expected: report.HealthOK,
		},
		{
			name:     "Bound已挂载低使用率_健康",
			row:      report.StorageRow{Phase: "Bound", Mounted: true, UsagePct: 50.0},
			expected: report.HealthOK,
		},
		{
			name:     "Pending_警告",
			row:      report.StorageRow{Phase: "Pending"},
			expected: report.HealthWarn,
		},
		{
			name:     "Bound已挂载超85%_警告",
			row:      report.StorageRow{Phase: "Bound", Mounted: true, UsagePct: 88.0},
			expected: report.HealthWarn,
		},
		{
			name:     "Lost_严重",
			row:      report.StorageRow{Phase: "Lost"},
			expected: report.HealthSevere,
		},
		{
			name:     "Bound已挂载超95%_严重",
			row:      report.StorageRow{Phase: "Bound", Mounted: true, UsagePct: 97.0},
			expected: report.HealthSevere,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyStorageHealth(&tt.row)
			if got != tt.expected {
				t.Errorf("classifyStorageHealth() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestFormatAccessModes 验证 AccessModes 映射为缩写字符串。
func TestFormatAccessModes(t *testing.T) {
	tests := []struct {
		name     string
		modes    []v1.PersistentVolumeAccessMode
		expected string
	}{
		{"空", nil, "-"},
		{"RWO", []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce}, "RWO"},
		{"RWX", []v1.PersistentVolumeAccessMode{v1.ReadWriteMany}, "RWX"},
		{"多模式", []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce, v1.ReadOnlyMany}, "RWO/ROX"},
		{"RWOP", []v1.PersistentVolumeAccessMode{v1.ReadWriteOncePod}, "RWOP"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatAccessModes(tt.modes)
			if got != tt.expected {
				t.Errorf("formatAccessModes() = %q, want %q", got, tt.expected)
			}
		})
	}
}

// TestExtractPVBoundNode 验证从 PV.NodeAffinity 解析绑定节点。
func TestExtractPVBoundNode(t *testing.T) {
	t.Run("无节点亲和性(网络存储)", func(t *testing.T) {
		pv := &v1.PersistentVolume{}
		if got := extractPVBoundNode(pv); got != "" {
			t.Errorf("期望空，得到 %q", got)
		}
	})

	t.Run("有节点亲和性(local存储)", func(t *testing.T) {
		pv := &v1.PersistentVolume{
			Spec: v1.PersistentVolumeSpec{
				NodeAffinity: &v1.VolumeNodeAffinity{
					Required: &v1.NodeSelector{
						NodeSelectorTerms: []v1.NodeSelectorTerm{
							{
								MatchExpressions: []v1.NodeSelectorRequirement{
									{Key: "kubernetes.io/hostname", Operator: v1.NodeSelectorOpIn, Values: []string{"kk-work01"}},
								},
							},
						},
					},
				},
			},
		}
		got := extractPVBoundNode(pv)
		if got != "kk-work01" {
			t.Errorf("期望 kk-work01，得到 %q", got)
		}
	})
}

// TestBuildStorageRow 验证 PVC 行组装（Quantity 解析、PV 关联、使用量填充）。
func TestBuildStorageRow(t *testing.T) {
	scName := "nfs-client"
	pvMap := map[string]*v1.PersistentVolume{
		"pv-001": {
			Status: v1.PersistentVolumeStatus{Phase: v1.VolumeBound},
		},
	}

	t.Run("Bound且有使用量", func(t *testing.T) {
		pvc := &v1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{Name: "data-pvc", Namespace: "db"},
			Spec: v1.PersistentVolumeClaimSpec{
				VolumeName:       "pv-001",
				StorageClassName: &scName,
				Resources: v1.VolumeResourceRequirements{
					Requests: v1.ResourceList{v1.ResourceStorage: resource.MustParse("100Gi")},
				},
			},
			Status: v1.PersistentVolumeClaimStatus{
				Phase:    v1.ClaimBound,
				Capacity: v1.ResourceList{v1.ResourceStorage: resource.MustParse("100Gi")},
			},
		}
		// 85Gi used
		usage := map[string]uint64{"db/data-pvc": 85 * (1 << 30)}
		row := buildStorageRow(pvc, pvMap, usage)
		if row == nil {
			t.Fatal("期望非 nil")
		}
		if row.RequestedGi != 100 {
			t.Errorf("RequestedGi = %d, want 100", row.RequestedGi)
		}
		if row.UsedGi != 85 {
			t.Errorf("UsedGi = %d, want 85", row.UsedGi)
		}
		if row.UsagePct < 84.9 || row.UsagePct > 85.1 {
			t.Errorf("UsagePct = %.1f, want ~85", row.UsagePct)
		}
		if row.PVPhase != "Bound" {
			t.Errorf("PVPhase = %q, want Bound", row.PVPhase)
		}
		if !row.Mounted {
			t.Error("期望 Mounted=true")
		}
		if row.Health != report.HealthWarn {
			t.Errorf("Health = %q, want 警告(使用率85)", row.Health)
		}
	})

	t.Run("Pending未挂载", func(t *testing.T) {
		pvc := &v1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{Name: "log-pvc", Namespace: "app"},
			Spec: v1.PersistentVolumeClaimSpec{
				StorageClassName: &scName,
				Resources: v1.VolumeResourceRequirements{
					Requests: v1.ResourceList{v1.ResourceStorage: resource.MustParse("50Gi")},
				},
			},
			Status: v1.PersistentVolumeClaimStatus{Phase: v1.ClaimPending},
		}
		row := buildStorageRow(pvc, pvMap, map[string]uint64{})
		if row == nil {
			t.Fatal("期望非 nil")
		}
		if row.Mounted {
			t.Error("期望 Mounted=false")
		}
		if row.Health != report.HealthWarn {
			t.Errorf("Health = %q, want 警告(Pending)", row.Health)
		}
	})
}

// TestIsPVReferencedByPVC 验证孤儿 PV 筛选（PV 是否被 PVC 引用）。
func TestIsPVReferencedByPVC(t *testing.T) {
	pvcs := []v1.PersistentVolumeClaim{
		{ObjectMeta: metav1.ObjectMeta{Name: "data-pvc", Namespace: "db"}},
	}

	t.Run("PV有ClaimRef且PVC存在", func(t *testing.T) {
		pv := &v1.PersistentVolume{
			Spec: v1.PersistentVolumeSpec{
				ClaimRef: &v1.ObjectReference{Name: "data-pvc", Namespace: "db"},
			},
		}
		if !isPVReferencedByPVC(pv, pvcs) {
			t.Error("期望被引用")
		}
	})

	t.Run("PV有ClaimRef但PVC已删", func(t *testing.T) {
		pv := &v1.PersistentVolume{
			Spec: v1.PersistentVolumeSpec{
				ClaimRef: &v1.ObjectReference{Name: "deleted-pvc", Namespace: "db"},
			},
		}
		if isPVReferencedByPVC(pv, pvcs) {
			t.Error("期望不被引用(孤儿)")
		}
	})

	t.Run("PV无ClaimRef", func(t *testing.T) {
		pv := &v1.PersistentVolume{}
		if isPVReferencedByPVC(pv, pvcs) {
			t.Error("期望不被引用")
		}
	})
}

// TestBuildOrphanPVRow 验证孤儿 PV 行组装。
func TestBuildOrphanPVRow(t *testing.T) {
	pv := &v1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{Name: "pv-old"},
		Spec: v1.PersistentVolumeSpec{
			PersistentVolumeReclaimPolicy: v1.PersistentVolumeReclaimRetain,
			AccessModes:                   []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
			Capacity:                      v1.ResourceList{v1.ResourceStorage: resource.MustParse("100Gi")},
		},
		Status: v1.PersistentVolumeStatus{Phase: v1.VolumeReleased},
	}
	row := buildOrphanPVRow(pv)
	if row.Name != "pv-old" {
		t.Errorf("Name = %q", row.Name)
	}
	if row.Phase != "Released" {
		t.Errorf("Phase = %q, want Released", row.Phase)
	}
	if row.ReclaimPolicy != "Retain" {
		t.Errorf("ReclaimPolicy = %q, want Retain", row.ReclaimPolicy)
	}
	if row.AccessModes != "RWO" {
		t.Errorf("AccessModes = %q, want RWO", row.AccessModes)
	}
	if row.CapacityGi != 100 {
		t.Errorf("CapacityGi = %d, want 100", row.CapacityGi)
	}
}

// TestGetPVCStorageClassName 验证取 StorageClass 名（Spec 和注解两种方式）。
func TestGetPVCStorageClassName(t *testing.T) {
	t.Run("Spec指定", func(t *testing.T) {
		sc := "nfs-client"
		pvc := &v1.PersistentVolumeClaim{Spec: v1.PersistentVolumeClaimSpec{StorageClassName: &sc}}
		if got := getPVCStorageClassName(pvc); got != "nfs-client" {
			t.Errorf("got %q", got)
		}
	})
	t.Run("注解方式", func(t *testing.T) {
		pvc := &v1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{"volume.beta.kubernetes.io/storage-class": "openebs"},
			},
		}
		if got := getPVCStorageClassName(pvc); got != "openebs" {
			t.Errorf("got %q", got)
		}
	})
	t.Run("都没有", func(t *testing.T) {
		pvc := &v1.PersistentVolumeClaim{}
		if got := getPVCStorageClassName(pvc); got != "" {
			t.Errorf("期望空，got %q", got)
		}
	})
}
