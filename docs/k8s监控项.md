要全面评估一个 Kubernetes 集群的健康状态，需要从控制平面、工作节点、核心插件、工作负载、网络、存储、安全、资源、日志等多个维度进行。以下是一份尽可能详尽的检查清单，包含了具体的检查项和常用命令。

---

### 一、控制平面健康检查

1. **API Server 可用性**
   ```bash
   kubectl get --raw='/healthz?verbose'
   kubectl get --raw='/readyz?verbose'
   ```
   检查返回 `ok`，确保各核心组件（etcd、poststarthook 等）状态健康。

2. **etcd 集群健康**
   - 如果是静态 Pod 部署，可以查看 etcd Pod 日志，或进入 etcd 容器执行命令：
     ```bash
     # 查看 etcd Pod 列表
     kubectl -n kube-system get pods -l component=etcd
     # 进入任一 etcd Pod 检查成员列表
     kubectl -n kube-system exec -it etcd-<node> -- etcdctl \
       --cacert=/etc/kubernetes/pki/etcd/ca.crt \
       --cert=/etc/kubernetes/pki/etcd/server.crt \
       --key=/etc/kubernetes/pki/etcd/server.key \
       endpoint health --cluster
     ```
   - 检查 etcd 集群是否有 Leader，成员数量是否为奇数，有无告警。

3. **kube-scheduler 与 kube-controller-manager 状态**
   - 这两个组件往往没有直接的 `kubectl` 对象，可通过检查对应静态 Pod 或 endpoint 来判断：
     ```bash
     kubectl -n kube-system get pods | grep -E 'kube-scheduler|kube-controller-manager'
     kubectl -n kube-system logs <scheduler-pod>
     ```
   - 也可查看 `/healthz` 端点（默认端口 10259/10257）。

4. **控制平面证书有效期**
   ```bash
   # 若集群由 kubeadm 部署
   kubeadm certs check-expiration
   ```
   确保证书未过期，或提前续期。

---

### 二、工作节点健康检查

1. **节点基础状态**
   ```bash
   kubectl get nodes -o wide
   ```
   - 所有节点应处于 `Ready`，版本一致，无不可达情况。

2. **节点详细状况**
   ```bash
   kubectl describe node <node-name> | grep -A 10 Conditions
   ```
   检查 `MemoryPressure`、`DiskPressure`、`PIDPressure`、`NetworkUnavailable` 均为 `False`。

3. **节点资源使用**
   ```bash
   kubectl top nodes
   ```
   - CPU / Memory 使用率不应长时间接近 100%。
   - 关注节点磁盘空间（`/var/lib/kubelet`、`/var/lib/docker` 等），避免磁盘压力。

4. **kubelet 服务状态**
   登录节点检查：
   ```bash
   systemctl status kubelet
   journalctl -u kubelet -f
   ```
   确保 kubelet 运行正常，且能正常向 API Server 注册。

5. **节点问题检测器**（若安装了 Node Problem Detector）
   ```bash
   kubectl describe node <node-name> | grep -A 5 'NodeProblem'
   ```
   或者查看 `kube-system` 下相关 Pod 日志。

6. **内核参数与系统配置**
   - 确保 `ip_forward=1`，网桥过滤等参数正确。
   - 检查容器运行时状态（如 containerd / dockerd）：
     ```bash
     systemctl status containerd
     ```

---

### 三、核心插件与组件状态

1. **DNS 服务（CoreDNS / kube-dns）**
   ```bash
   kubectl -n kube-system get pods -l k8s-app=kube-dns
   kubectl -n kube-system logs -l k8s-app=kube-dns --tail=50
   ```
   - 所有 DNS Pod 应 `Running`，数量正常（至少 2 个）。
   - 进行 DNS 解析测试：
     ```bash
     kubectl run -it --rm debug --image=busybox --restart=Never -- nslookup kubernetes.default
     ```

2. **网络插件 (CNI)**
   - 根据实际 CNI（Calico、Flannel、Cilium 等）检查对应 Pod 或 DaemonSet：
     ```bash
     kubectl -n kube-system get pods -l k8s-app=calico-node   # 示例
     kubectl -n kube-system get ds
     ```
   - 确保所有节点上的网络代理 Pod 均处于 `Running`，无错误日志。

3. **kube-proxy**
   ```bash
   kubectl -n kube-system get pods -l k8s-app=kube-proxy
   kubectl -n kube-system logs <kube-proxy-pod>
   ```
   所有节点上的 kube-proxy 应正常运行。

4. **其他关键插件**
   - 监控系统：Prometheus, Grafana, Metrics Server（`kubectl get pods -n monitoring`）
   - 日志收集：Elasticsearch/Fluentd/Loki 等
   - 存储驱动：CSI 插件状态（如 EBS CSI、Ceph CSI 等）
   - Ingress 控制器：nginx-ingress, traefik 等

---

### 四、工作负载健康检查

1. **Pod 状态总览**
   ```bash
   kubectl get pods --all-namespaces
   ```
   重点过滤非 `Running` 和长时间 `Pending`、`CrashLoopBackOff`、`Error` 的 Pod。

2. **Pod 重启次数与 OOMKilled**
   ```bash
   kubectl get pods --all-namespaces -o wide | grep -v '0.*Running'
   ```
   或查看特定 Pod 的 `RESTARTS` 次数，高重启可能预示问题。
   ```bash
   kubectl describe pod <pod> -n <ns> | grep -i 'oomkilled\|exit code'
   ```

3. **控制器状态**
   ```bash
   kubectl get deploy,sts,ds -A
   ```
   确保期望副本数 = 就绪副本数。

4. **Job / CronJob 完成情况**
   ```bash
   kubectl get jobs,cronjobs -A
   ```
   检查有无持续失败或挂起的任务。

5. **挂起的资源请求**
   ```bash
   kubectl get events -A | grep -i 'insufficient\|failed scheduling\|failed to admit'
   ```
   排查是否存在因资源不足或限制无法调度的 Pod。

---

### 五、存储检查

1. **PV 与 PVC 状态**
   ```bash
   kubectl get pv,pvc -A
   ```
   - PV 状态应为 `Bound` 或 `Available`（未被绑定时），无 `Failed`。
   - PVC 状态应为 `Bound`，无长期 `Pending`。

2. **StorageClass 配置**
   ```bash
   kubectl get sc
   ```
   确认默认 StorageClass（如果有）存在且正常。

3. **存储插件健康**
   - 检查 CSI 插件 Driver 注册：
     ```bash
     kubectl get csidrivers
     ```
   - 查看 CSI 控制器和节点 DaemonSet 的 Pod 日志，确保挂卷和卸卷无错误。

4. **卷健康监控**
   - 通过 PV 的事件和状态检测挂载错误。

---

### 六、网络与 Service 检查

1. **Service 与 Endpoint**
   ```bash
   kubectl get svc,endpoints -A
   ```
   - 每个 Service 应有对应的 Endpoints（除非是 headless service 且不期望有 Pod）。
   - 有 External-IP 的 LoadBalancer 类型 Service 应正确分配 IP。

2. **Ingress 状态**
   ```bash
   kubectl get ingress -A
   ```
   检查 ADDRESS 是否分配，TLS 证书是否正常。

3. **网络连通性**
   - 使用测试 Pod 访问集群内服务：
     ```bash
     kubectl run test-$RANDOM --rm -it --image=alpine -- sh
     # 在容器内 curl http://<service>.<namespace>:<port>
     ```
   - 检查跨节点 Pod 通信。

---

### 七、安全与合规检查

1. **RBAC 权限审计**
   ```bash
   kubectl get clusterroles,roles -A
   kubectl get clusterrolebindings,rolebindings -A
   ```
   - 确认没有过度绑定 `cluster-admin` 权限，尤其对用户或 ServiceAccount。

2. **Pod 安全上下文**
   - 检查是否有特权容器或以 root 运行的 Pod（可根据组织策略）：
     ```bash
     kubectl get pods -A -o jsonpath='{range .items[*]}{.metadata.namespace}{" "}{.metadata.name}{" - SecurityContext: "}{.spec.containers[*].securityContext}{"\n"}{end}'
     ```
   - 应避免滥用 `privileged: true`。

3. **Secrets 与 ConfigMaps 管理**
   - 定期轮换密钥，避免明文存放敏感信息。
   - 检查 Secret 数量和数据内容。

4. **网络策略**
   ```bash
   kubectl get networkpolicies -A
   ```
   确认关键命名空间（如 kube-system）有合适的隔离策略。

5. **审计日志**
   - 检查 API Server 审计日志配置，对非授权访问、敏感操作进行记录和报警。

---

### 八、事件、日志与监控

1. **集群事件**
   ```bash
   kubectl get events -A --sort-by='.lastTimestamp'
   ```
   重点关注 `Warning` 类型事件，特别是不受控的 Pod 驱逐、节点异常等。

2. **核心组件日志**
   - 通过日志收集平台（如 ELK/Loki）或直接查看容器日志，搜索 Error/Fatal。

3. **监控系统状态**
   - 确认 Prometheus 目标全部 UP，Grafana 面板无异常。
   - 关键指标：API Server 请求延迟、etcd 磁盘同步延迟、调度延迟、节点资源水位。

4. **告警规则检查**
   - 确认告警规则处于活动状态，没有因配置变更而失效。

---

### 九、备份与灾难恢复

1. **etcd 备份**
   - 检查 etcd 备份脚本是否定期执行，备份文件是否可用。
   - 尝试从备份恢复到一个临时 etcd 节点（演练）。

2. **应用数据备份**
   - 若有 Velero 等备份工具，检查其最近备份记录：
     ```bash
     velero backup get
     ```
   - 确认备份存储位置可访问，且备份完整性无误。

---

### 十、资源容量与配额

1. **节点容量**
   ```bash
   kubectl describe nodes | grep -A 5 "Allocated resources"
   ```
   观察 CPU/内存/临时存储的 **请求** 与 **限制** 占节点总容量的比例，避免过量分配。

2. **命名空间配额**
   ```bash
   kubectl get resourcequota -A
   ```
   检查各命名空间是否接近配额上限，可能导致创建被拒。

3. **水平自动伸缩**
   - HPA 状态：
     ```bash
     kubectl get hpa -A
     ```
     确保目标利用率计算正确，未出现 `AbleToScale` 为 False 的情况。
   - 若有 Cluster Autoscaler，检查节点组伸缩活动和状态。

---

### 十一、配置一致性

1. **版本一致性**
   ```bash
   kubectl get nodes -o custom-columns=NAME:.metadata.name,VERSION:.status.nodeInfo.kubeletVersion
   ```
   所有节点 kubelet 版本相差不宜过大（原则上与控制平面小版本一致）。

2. **API 弃用警告**
   - 升级前检查：
     ```bash
     kubectl api-versions
     # 结合 cluster 使用 `kubent`(Kube No Trouble) 工具扫描
     ```

3. **节点配置一致性**
   - 对比不同节点的 kubelet 参数、容器运行时版本、内核版本等，避免隐性差异。

---

### 十二、自动化检查工具推荐

手动逐项检查容易遗漏，建议结合专业工具生成自动化检查报告：

- **Sonobuoy**：官方认证集群一致性测试工具，可进行端到端验证。
- **kube-bench**：基于 CIS Kubernetes Benchmark 的安全审计。
- **Kube-hunter**：主动扫描集群安全弱点。
- **Popeye**：扫描集群资源并报告配置、资源使用、安全等问题。
- **kubeaudit**：审计集群 Manifest 安全问题。
- **Kubernetes Dashboard / Lens**：可视化查看集群整体健康状态。

---

以上检查项涵盖了从基础设施到应用层的方方面面，可根据你的集群实际规模和运维要求，形成周期性的检查表格。如果需要对某个类别进行更深入的专项排查，请随时追问。