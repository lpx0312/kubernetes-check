package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	v1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s-patrol/internal/report"
)

// PVC 使用率告警阈值（与节点内存阈值对齐）。
const (
	pvcUsageWarnPct   = 85.0 // PVC 使用率 >=85% 标警告
	pvcUsageSeverePct = 95.0 // >=95% 标严重
)

// ---- kubelet /stats/summary 的最小类型定义（零依赖，仅取需要的字段）----
// 原始类型在 k8s.io/kubelet/pkg/apis/stats/v1alpha1，但引入整个 kubelet 模块代价过大，
// 这里自定义最小子集，用 encoding/json 反序列化 DoRaw 返回的字节。

type kubeletSummary struct {
	Pods []podStats `json:"pods"`
}
type podStats struct {
	PodRef podRef     `json:"podRef"`
	Volume []volStats `json:"volume"`
}
type podRef struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}
type volStats struct {
	Name          string  `json:"name"`
	PVCRef        *pvcRef `json:"pvcRef,omitempty"` // 非 nil = 这是 PVC 卷
	UsedBytes     *uint64 `json:"usedBytes"`
	CapacityBytes *uint64 `json:"capacityBytes"`
}
type pvcRef struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

// collectStorage 执行存储检查：PVC 绑定状态 + 使用量 + 孤儿 PV + SC 健康。
// 失败时降级为 Note 提示，不阻塞报告。
func (c *Collector) collectStorage(ctx context.Context, rep *report.Report) {
	// Step 1: 列 PVC
	ns := c.cfg.Namespace
	if c.cfg.AllNamespace {
		ns = ""
	}
	pvcs, err := c.clientset.CoreV1().PersistentVolumeClaims(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		rep.AddNote("获取 PVC 列表失败: " + err.Error())
		return
	}

	// Step 2: 列 PV（集群级），用于关联 PVC 和筛孤儿 PV
	pvs, err := c.clientset.CoreV1().PersistentVolumes().List(ctx, metav1.ListOptions{})
	if err != nil {
		rep.AddNote("获取 PV 列表失败: " + err.Error())
		// PV 拿不到也继续，PVC 基础信息仍能采集
		pvs = &v1.PersistentVolumeList{}
	}

	// 建立 PV map（按 name 索引）和"已被 PVC 引用的 PV"集合
	pvMap := make(map[string]*v1.PersistentVolume, len(pvs.Items))
	referencedPV := make(map[string]bool) // PV name → 是否被某 PVC 引用
	for i := range pvs.Items {
		pv := &pvs.Items[i]
		pvMap[pv.Name] = pv
	}

	// Step 3: 调 kubelet stats 拿 PVC 使用量（按节点并发友好，这里串行先求可用）
	pvcUsage := c.collectPVCUsage(ctx, rep)

	// Step 4: 组装 PVC 行
	for i := range pvcs.Items {
		pvc := &pvcs.Items[i]
		row := buildStorageRow(pvc, pvMap, pvcUsage)
		if row != nil {
			referencedPV[row.PVName] = true
			rep.StorageRows = append(rep.StorageRows, *row)
		}
	}

	// Step 5: 筛孤儿 PV（Released/Failed 且未被任何 PVC 引用）
	for i := range pvs.Items {
		pv := &pvs.Items[i]
		if pv.Status.Phase != v1.VolumeReleased && pv.Status.Phase != v1.VolumeFailed {
			continue
		}
		// 检查是否被某 PVC 引用（有 ClaimRef 且对应 PVC 存在）
		if isPVReferencedByPVC(pv, pvcs.Items) {
			continue
		}
		rep.OrphanPVRows = append(rep.OrphanPVRows, buildOrphanPVRow(pv))
	}
}

// collectPVCUsage 调每个节点的 kubelet /stats/summary，汇总每个 PVC 的已用字节。
// 返回 pvcKey("namespace/name") → usedBytes。kubelet 不可达时降级，返回空 map。
func (c *Collector) collectPVCUsage(ctx context.Context, rep *report.Report) map[string]uint64 {
	usage := make(map[string]uint64)

	nodes, err := c.clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		rep.AddNote("获取节点列表失败（存储使用量采集）: " + err.Error())
		return usage
	}

	failedNodes := 0
	for i := range nodes.Items {
		nodeName := nodes.Items[i].Name
		summary, err := c.fetchKubeletSummary(ctx, nodeName)
		if err != nil {
			failedNodes++
			continue // 单节点失败不阻塞
		}
		for _, pod := range summary.Pods {
			for _, vol := range pod.Volume {
				if vol.PVCRef == nil || vol.UsedBytes == nil {
					continue
				}
				key := vol.PVCRef.Namespace + "/" + vol.PVCRef.Name
				// 同一 PVC 多 Pod 共享（RWX）取累加值；RWO 通常单 Pod
				usage[key] += *vol.UsedBytes
			}
		}
	}
	if failedNodes > 0 {
		rep.AddNote(fmt.Sprintf("%d 个节点的卷使用量采集失败（kubelet /stats/summary 不可达）", failedNodes))
	}
	return usage
}

// fetchKubeletSummary 调用单节点的 kubelet /stats/summary。
// 走 apiserver 代理（/api/v1/nodes/{name}/proxy/stats/summary），复用 kubeconfig 凭证。
func (c *Collector) fetchKubeletSummary(ctx context.Context, nodeName string) (*kubeletSummary, error) {
	raw, err := c.restClient.Get().
		Resource("nodes").
		Name(nodeName).
		SubResource("proxy").
		Suffix("stats", "summary").
		DoRaw(ctx)
	if err != nil {
		return nil, err
	}
	var summary kubeletSummary
	if err := json.Unmarshal(raw, &summary); err != nil {
		return nil, fmt.Errorf("解析 kubelet stats 失败: %w", err)
	}
	return &summary, nil
}

// buildStorageRow 从 PVC 组装报告行，关联 PV 和使用量。
func buildStorageRow(pvc *v1.PersistentVolumeClaim, pvMap map[string]*v1.PersistentVolume, pvcUsage map[string]uint64) *report.StorageRow {
	row := &report.StorageRow{
		Namespace:    pvc.Namespace,
		Name:         pvc.Name,
		Phase:        string(pvc.Status.Phase),
		StorageClass: getPVCStorageClassName(pvc),
		PVName:       pvc.Spec.VolumeName,
	}

	// 申请量
	if req, ok := pvc.Spec.Resources.Requests[v1.ResourceStorage]; ok {
		row.RequestedGi = req.Value() / (1 << 30)
	}

	// 实际容量（Bound 后 Status.Capacity 才有值；否则用申请量兜底展示）
	if cap, ok := pvc.Status.Capacity[v1.ResourceStorage]; ok {
		row.CapacityGi = cap.Value() / (1 << 30)
	} else {
		row.CapacityGi = row.RequestedGi
	}

	// 关联 PV，取 PV 状态
	if row.PVName != "" {
		if pv, ok := pvMap[row.PVName]; ok {
			row.PVPhase = string(pv.Status.Phase)
		}
	}

	// 使用量（kubelet stats，按 pvcKey 查）
	key := pvc.Namespace + "/" + pvc.Name
	if used, ok := pvcUsage[key]; ok {
		row.Mounted = true
		row.UsedGi = int64(used) / (1 << 30)
		if row.CapacityGi > 0 {
			row.UsagePct = float64(row.UsedGi) / float64(row.CapacityGi) * 100
		}
	}

	// 健康判定（本行）
	row.Health = classifyStorageHealth(row)
	return row
}

// classifyStorageHealth 判定单个 PVC 行的健康度。
func classifyStorageHealth(row *report.StorageRow) string {
	// 严重：Lost / 使用率 ≥95%
	if row.Phase == "Lost" {
		return report.HealthSevere
	}
	if row.Mounted && row.UsagePct >= pvcUsageSeverePct {
		return report.HealthSevere
	}
	// 警告：Pending(卡绑定) / 使用率 85%-95%
	if row.Phase == "Pending" {
		return report.HealthWarn
	}
	if row.Mounted && row.UsagePct >= pvcUsageWarnPct {
		return report.HealthWarn
	}
	return report.HealthOK
}

// buildOrphanPVRow 从孤儿 PV 组装报告行。
func buildOrphanPVRow(pv *v1.PersistentVolume) report.OrphanPVRow {
	row := report.OrphanPVRow{
		Name:          pv.Name,
		Phase:         string(pv.Status.Phase),
		ReclaimPolicy: string(pv.Spec.PersistentVolumeReclaimPolicy),
		AccessModes:   formatAccessModes(pv.Spec.AccessModes),
		BoundNode:     extractPVBoundNode(pv),
	}
	if cap, ok := pv.Spec.Capacity[v1.ResourceStorage]; ok {
		row.CapacityGi = cap.Value() / (1 << 30)
	}
	return row
}

// isPVReferencedByPVC 判断 PV 是否仍被某个 PVC 引用（ClaimRef 存在且对应 PVC 也在列表中）。
func isPVReferencedByPVC(pv *v1.PersistentVolume, pvcs []v1.PersistentVolumeClaim) bool {
	if pv.Spec.ClaimRef == nil {
		return false
	}
	claimRef := pv.Spec.ClaimRef
	for i := range pvcs {
		if pvcs[i].Name == claimRef.Name && pvcs[i].Namespace == claimRef.Namespace {
			return true
		}
	}
	return false
}

// getPVCStorageClassName 取 PVC 的 StorageClass 名（兼容 Spec.StorageClassName 和注解两种方式）。
func getPVCStorageClassName(pvc *v1.PersistentVolumeClaim) string {
	if pvc.Spec.StorageClassName != nil && *pvc.Spec.StorageClassName != "" {
		return *pvc.Spec.StorageClassName
	}
	// 旧版用注解 volume.beta.kubernetes.io/storage-class
	if sc, ok := pvc.Annotations["volume.beta.kubernetes.io/storage-class"]; ok && sc != "" {
		return sc
	}
	return ""
}

// formatAccessModes 把 AccessModes 数组映射为 RWO/ROX/RWX 拼接字符串。
func formatAccessModes(modes []v1.PersistentVolumeAccessMode) string {
	if len(modes) == 0 {
		return "-"
	}
	names := make([]string, 0, len(modes))
	for _, m := range modes {
		switch m {
		case v1.ReadWriteOnce:
			names = append(names, "RWO")
		case v1.ReadOnlyMany:
			names = append(names, "ROX")
		case v1.ReadWriteMany:
			names = append(names, "RWX")
		case v1.ReadWriteOncePod:
			names = append(names, "RWOP")
		default:
			names = append(names, string(m))
		}
	}
	return strings.Join(names, "/")
}

// extractPVBoundNode 从 PV.Spec.NodeAffinity 解析出绑定的节点名（local/hostpath 存储）。
// 网络存储（NFS/Ceph）无节点亲和性，返回空。
func extractPVBoundNode(pv *v1.PersistentVolume) string {
	if pv.Spec.NodeAffinity == nil || pv.Spec.NodeAffinity.Required == nil {
		return ""
	}
	for _, term := range pv.Spec.NodeAffinity.Required.NodeSelectorTerms {
		for _, me := range term.MatchExpressions {
			if me.Key == "kubernetes.io/hostname" {
				if len(me.Values) > 0 {
					return me.Values[0]
				}
			}
		}
		// 也处理 MatchFields
		for _, mf := range term.MatchFields {
			if mf.Key == "kubernetes.io/hostname" && len(mf.Values) > 0 {
				return mf.Values[0]
			}
		}
	}
	return ""
}

// 确保未使用的 import 被引用（storagev1 供后续 SC 健康检查扩展使用）
var _ = (*storagev1.StorageClass)(nil)
