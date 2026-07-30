# Metrics Server 部署手册

> 本手册记录了在 Kubernetes 集群（以 `192.168.1.181` master 节点、K8s v1.31.14 为例）上部署 metrics-server 的完整流程，包含内网镜像选型、踩坑记录和故障排查。
>
> metrics-server 提供 Metrics API（`metrics.k8s.io/v1beta1`），是 `kubectl top` 和本工具 `--node-metrics` 模式的依赖。未安装时，节点资源监控将不可用。

---

## 1. 前置条件

| 项 | 要求 | 说明 |
|----|------|------|
| K8s 版本 | ≥ 1.16 | metrics-server v0.8.x 官方支持 1.27+，1.16+ 基本可用 |
| metrics-server 版本 | 与 K8s 匹配 | 见下表「版本对应」 |
| kube-apiserver 聚合层 | 已启用 | kubeadm 默认开启（`requestheader-*` 参数） |
| kubelet | `rotateCertificates: true` | 推荐，kubeadm 集群默认开启 |
| 内网镜像仓库 | 可达 | 本文使用 `harbor.sktill.top:7000` |

### 版本对应关系

| K8s 版本 | 推荐 metrics-server | 备注 |
|----------|--------------------|------|
| 1.27 ~ 最新 | **v0.8.x** | 本文使用 |
| 1.21 ~ 1.26 | v0.7.x | |
| ≤ 1.20 | v0.6.x 及以下 | 老集群 |

---

## 2. 镜像选型（内网 Harbor）

从内网 Harbor `harbor.sktill.top:7000` 中查找可用镜像：

```bash
# Harbor v2 API 搜索（账号 admin / <你的密码>）
curl -sk "https://harbor.sktill.top:7000/api/v2.0/search?q=metrics-server" \
  -u "admin:<密码>"
```

本环境候选镜像：

| 仓库 | Tag | 大小 | 评价 |
|------|-----|------|------|
| `kubesphere/metrics-server` | **v0.8.0** | 21.5 MB | ✅ 选用，最新稳定版，匹配 K8s 1.31 |
| `kubesphere/metrics-server` | v0.7.0 | 87.8 MB | 偏旧，且体积异常大（历史脏数据） |
| `k8s-deploy/metrics-server` | v0.7.0 | 18.5 MB | 可用但偏旧 |
| `k8s-deploy/metrics-server` | v0.3.1 | 20.5 MB | ❌ 太老（2018），不兼容新 K8s |

**最终选用镜像**：

```
harbor.sktill.top:7000/kubesphere/metrics-server:v0.8.0
```

> 选型原则：版本优先匹配 K8s 大版本，其次看推送时间（越新越好），最后看体积。

---

## 3. 部署清单

以下清单已验证可正常工作，关键配置点：

- 镜像换为内网 Harbor 地址，`imagePullPolicy: IfNotPresent`
- 容器参数加 `--kubelet-insecure-tls`（开发/内网环境跳过 kubelet 证书校验，避免 `serverTLSBootstrap` 未开启导致的采集失败）
- 证书目录显式指向 `/tmp` 并挂载 emptyDir（**修复只读文件系统 panic，见第 5 节**）
- 调度到控制平面节点（容忍 `control-plane` 污点）
- `APIService.insecureSkipTLSVerify: true`（配合自签名证书）

```yaml
# metrics-server.yaml
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: metrics-server
  namespace: kube-system
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: system:aggregated-metrics-reader
  labels:
    rbac.authorization.k8s.io/aggregate-to-view: "true"
    rbac.authorization.k8s.io/aggregate-to-edit: "true"
    rbac.authorization.k8s.io/aggregate-to-admin: "true"
rules:
- apiGroups: ["metrics.k8s.io"]
  resources: ["pods", "nodes"]
  verbs: ["get", "list", "watch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: system:metrics-server
rules:
- apiGroups: [""]
  resources: ["nodes/metrics"]
  verbs: ["get"]
- apiGroups: [""]
  resources: ["pods", "nodes"]
  verbs: ["get", "list", "watch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: metrics-server-auth-reader
  namespace: kube-system
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: extension-apiserver-authentication-reader
subjects:
- kind: ServiceAccount
  name: metrics-server
  namespace: kube-system
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: metrics-server:system:auth-delegator
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: system:auth-delegator
subjects:
- kind: ServiceAccount
  name: metrics-server
  namespace: kube-system
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: system:metrics-server
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: system:metrics-server
subjects:
- kind: ServiceAccount
  name: metrics-server
  namespace: kube-system
---
apiVersion: v1
kind: Service
metadata:
  name: metrics-server
  namespace: kube-system
  labels:
    kubernetes.io/name: Metrics-server
    kubernetes.io/cluster-service: "true"
spec:
  selector:
    k8s-app: metrics-server
  ports:
  - port: 443
    protocol: TCP
    targetPort: 10250
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: metrics-server
  namespace: kube-system
  labels:
    k8s-app: metrics-server
spec:
  selector:
    matchLabels:
      k8s-app: metrics-server
  strategy:
    rollingUpdate:
      maxUnavailable: 0
  template:
    metadata:
      labels:
        k8s-app: metrics-server
    spec:
      tolerations:
      - key: node-role.kubernetes.io/control-plane
        operator: Exists
        effect: NoSchedule
      affinity:
        nodeAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
            - matchExpressions:
              - key: node-role.kubernetes.io/control-plane
                operator: Exists
      containers:
      - name: metrics-server
        image: harbor.sktill.top:7000/kubesphere/metrics-server:v0.8.0
        imagePullPolicy: IfNotPresent
        ports:
        - containerPort: 10250
          protocol: TCP
        command:
        - /metrics-server
        - --secure-port=10250
        - --kubelet-preferred-address-types=InternalIP,ExternalIP,Hostname
        - --kubelet-use-node-status-port
        - --metric-resolution=15s
        - --kubelet-insecure-tls
        - --cert-dir=/tmp                       # ★ 关键：证书生成目录，须可写
        livenessProbe:
          httpGet:
            path: /livez
            port: 10250
            scheme: HTTPS
          initialDelaySeconds: 20
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /readyz
            port: 10250
            scheme: HTTPS
          initialDelaySeconds: 20
          periodSeconds: 10
        resources:
          requests:
            cpu: 100m
            memory: 200Mi
          limits:
            cpu: 500m
            memory: 500Mi
        volumeMounts:
        - name: tmp
          mountPath: /tmp
      serviceAccountName: metrics-server
      volumes:
      - emptyDir: {}
        name: tmp
---
apiVersion: apiregistration.k8s.io/v1
kind: APIService
metadata:
  name: v1beta1.metrics.k8s.io
spec:
  service:
    name: metrics-server
    namespace: kube-system
  group: metrics.k8s.io
  version: v1beta1
  insecureSkipTLSVerify: true
  groupPriorityMinimum: 100
  versionPriority: 100
```

---

## 4. 部署与验证

### 4.1 部署

```bash
kubectl apply -f metrics-server.yaml
```

### 4.2 验证（三步走）

**第一步：Pod 状态**

```bash
kubectl get pods -n kube-system -l k8s-app=metrics-server -o wide
# 期望：1/1 Running，RESTARTS 持续为 0
```

**第二步：APIService 可用性**

```bash
kubectl get apiservice v1beta1.metrics.k8s.io
# 期望：AVAILABLE=True
```

**第三步：指标可读**

```bash
kubectl top nodes
kubectl top pods -n kube-system
# 期望：正常输出 CPU/内存数据，不再报 "metrics API not available"
```

### 4.3 实际验证输出（192.168.1.181 集群）

```
$ kubectl top nodes
NAME          CPU(cores)   CPU%   MEMORY(bytes)   MEMORY%
kk-master01   90m          5%     2070Mi          75%
kk-master02   68m          4%     1816Mi          66%
kk-master03   216m         13%    1870Mi          68%
kk-work01     43m          2%     1514Mi          55%
```

---

## 5. 踩坑记录

### 5.1 ❌ 错误：只读文件系统 panic（最常见）

**现象**：Pod 启动即 CrashLoopBackOff，日志报：

```
panic: error creating self-signed certificates: mkdir apiserver.local.config: read-only file system
goroutine 1 [running]:
main.main()
    sigs.k8s.io/metrics-server/cmd/metrics-server/metrics-server.go:37 +0x88
```

**根因**：metrics-server 启动时需在工作目录生成自签名证书并写入 `apiserver.local.config/`。若 Pod 配置了 `readOnlyRootFilesystem: true` 且未挂载可写目录，则写盘失败直接 panic。

**修复**（二选一）：

1. **挂载 emptyDir 到证书目录**（推荐，保留只读根文件系统的安全性）：
   ```yaml
   command:
   - /metrics-server
   - --cert-dir=/tmp          # ← 显式指定证书目录
   # ...
   volumeMounts:
   - name: tmp
     mountPath: /tmp
   volumes:
   - emptyDir: {}
     name: tmp
   ```
2. **去掉 `readOnlyRootFilesystem: true`**：让容器根目录可写。简单但安全性略低。

> 本手册的清单已采用方案 1。

### 5.2 ❌ 错误：镜像太老不兼容

`k8s-deploy/metrics-server:v0.3.1`（2018 年）在新版 K8s（1.31）上无法正常工作。务必按第 2 节选匹配的版本。

### 5.3 ❌ 错误：kubelet 证书校验失败

**现象**：metrics-server Pod Running，但 `kubectl top nodes` 报错或无数据；metrics-server 日志反复出现 `x509: certificate signed by unknown authority`。

**根因**：kubelet 的服务端证书未被集群 CA 签发（`serverTLSBootstrap` 默认 false）。

**修复**（二选一）：

1. **快速方案（内网/开发环境）**：加 `--kubelet-insecure-tls` 跳过校验（本手册采用）。
2. **生产方案**：在所有节点开启 kubelet 服务证书签发：
   ```bash
   # 编辑 /var/lib/kubelet/config.yaml，加入：
   serverTLSBootstrap: true
   # 重启 kubelet
   systemctl restart kubelet
   ```
   随后批准 metrics-server 发起的 CSR：
   ```bash
   kubectl get csr | grep Pending
   kubectl certificate approve <csr-name>
   ```

### 5.4 ⚠️ 注意：requestheader-allowed-names

若 kube-apiserver 的 `--requestheader-allowed-names` 仅允许特定 CN（如 `front-proxy-client`），而 metrics-server 的客户端证书 CN 不在列表中，会导致 401/403。生产环境需确保 CN 匹配或调整该参数；测试环境用 `insecureSkipTLSVerify` 可绕过。

---

## 6. 卸载

```bash
kubectl delete -f metrics-server.yaml
```

---

## 7. 快速命令速查

```bash
# 搜索 Harbor 镜像
curl -sk "https://harbor.sktill.top:7000/api/v2.0/search?q=metrics-server" -u "admin:<密码>"

# 部署
kubectl apply -f metrics-server.yaml

# 查状态
kubectl get pods -n kube-system -l k8s-app=metrics-server -o wide
kubectl get apiservice v1beta1.metrics.k8s.io

# 查日志（排障）
kubectl logs -n kube-system -l k8s-app=metrics-server --tail=50

# 验证指标
kubectl top nodes
kubectl top pods -A
```
