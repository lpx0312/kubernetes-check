# 生产案例：kubelet rootFs 统计失败导致节点指标采集异常

> **节点**：`10.18.130.15-share`（virtio 虚拟磁盘 / LVM 卷环境）
> **集群规模**：28 节点，仅此 1 个节点异常
> **发现方式**：k8s-patrol 巡检报告提示「获取节点 xx 的资源指标失败」
> **业务影响**：无（MySQL/Nacos/Redis 等核心中间件已稳定运行 3 年）
> **处理结论**：维持现状不重启，待维护窗口处理

---

## 一、故障现象

### 1.1 k8s-patrol 巡检报告

运行全量巡检时，控制台输出告警：

```
获取节点 10.18.130.15-share 的资源指标失败:
  nodemetrics.metrics.k8s.io "10.18.130.15-share" not found
警告: 1 个节点的指标获取失败
```

节点资源监控表格中，该节点**直接消失**（28 节点只显示 27 个），其余节点数据正常。

### 1.2 metrics-server 日志

```
unable to fully collect metrics:
  unable to fully scrape metrics from source kubelet_summary:10.18.130.15-share:
    unable to fetch metrics from Kubelet 10.18.130.15-share (10.18.130.15):
      request failed - "500 Internal Server Error",
      response: "Internal Error: failed to get rootFs stats:
        failed to get rootFs info: failed to get device for dir "/var/lib/kubelet":
          could not find device with major: 253, minor: 65 in cached partitions map"
```

### 1.3 该节点运行的负载

节点上跑了多个**生产核心中间件**（均 Running 3 年以上）：

| 命名空间 | Pod | 中间件 | 运行时长 |
|---------|-----|--------|---------|
| afc-dragon-middleware-prd | afc-dragon-mysql-0 | MySQL | 3y234d |
| afc-dragon-middleware-prd | afc-dragon-mysql-2 | MySQL | 3y234d |
| afc-dragon-middleware-prd | afc-dragon-nacos-1 | Nacos | 3y236d |
| afc-dragon-middleware-prd | afc-dragon-redis-1 | Redis | 3y236d |
| afc-dragon-middleware-prd | afc-minio-prd-2 | MinIO | 3y46d |
| afc-dragon-middleware-prd | afc-mq-broker-0-1 | MQ | 2y45d |

**这是决定处理策略的关键因素**：不能贸然重启。

---

## 二、排查过程与根因

### 2.1 调用链定位

故障的传播路径（从上游到下游）：

```
kubelet 的 cadvisor 查 /var/lib/kubelet 底层块设备 → 找 253:65 → 缓存里没有
        ↓
kubelet /stats/summary 接口返回 500 Internal Server Error
        ↓
metrics-server 抓不到该节点数据 → "no metrics known for node"
        ↓
k8s-patrol 查 metrics API → not found → 降级跳过该节点
```

**根因在 kubelet 自身**，k8s-patrol 和 metrics-server 都只是"受害者"。

### 2.2 验证：直接调 kubelet 接口复现

```bash
kubectl --kubeconfig=/etc/kubernetes/admin.conf get --raw \
  "/api/v1/nodes/10.18.130.15-share/proxy/stats/summary"
```

返回与 metrics-server 日志完全一致的 500 错误，**确认故障源在 kubelet**。

### 2.3 验证：设备号真实情况

在 `10.18.130.15` 上排查：

```bash
# /var/lib/kubelet 的真实挂载（普通磁盘，非 dm）
$ df -h /var/lib/kubelet/
/dev/vde1   100G   33M   100G   1%   /var/lib/kubelet

$ findmnt -n -o SOURCE /var/lib/kubelet
/dev/vde1

# 253:65 确实存在于内核设备表（是 virtio 磁盘 vde1，不是 device-mapper）
$ cat /proc/partitions | grep "253"
 253        0  104857600 vda
 253       65  104856576 vde1      ← 内核里有

# 但 /dev/dm-* 的 major 是 252（device-mapper 是另一套）
$ ls -l /dev/dm-*
brw-rw---- root disk 252, 0  dm-0
```

### 2.4 根因结论

| 项 | 结论 |
|----|------|
| **故障类型** | kubelet 内嵌 cadvisor 的分区缓存失效 |
| **缺失设备** | `253:65`（virtio-blk 磁盘 vde1，承载 `/var/lib/kubelet`） |
| **根本原因** | kubelet 启动时读 `/proc/partitions` 构建设备缓存的时机，早于 virtio 设备 settle，导致 253:65 被漏读；之后缓存不再刷新 |
| **环境诱因** | virtio 虚拟磁盘 + LVM（`vg_middleware-disk--*`），这类环境下 kubelet 设备识别偶发问题 |
| **不是什么** | 不是 device-mapper 问题（dm 是 252，与 253 无关）；不是磁盘损坏（数据正常）；不是网络问题 |

**关键矛盾**：内核 `/proc/partitions` 明明有 253:65，但 kubelet 的内部缓存里没有 —— 典型的「缓存与实际状态不一致」。

---

## 三、影响评估

### 3.1 对业务的影响：无

- 容器进程（containerd fork 出来的）与 kubelet 相互独立，kubelet 的统计接口坏不影响容器运行
- MySQL/Nacos/Redis 已稳定运行 3 年，证明此故障**实际无害于业务**
- 文件系统本身健康，数据读写正常

### 3.2 对集群管理的影响：存在隐患

| 受影响组件 | 影响 | 风险等级 |
|-----------|------|---------|
| metrics-server | 拿不到该节点 CPU/内存指标 | 低（仅监控盲区） |
| **eviction_manager** | **拿不到磁盘统计，无法执行磁盘压力驱逐** | 中（盘满时无法自保） |
| kubectl top | 该节点无数据 | 低 |
| HPA（基于节点指标） | 可能受影响 | 视调度情况 |

**核心隐患**：eviction manager 失效。当前磁盘几乎为空（`33M/100G`），暂时安全；但若日志/数据暴涨，kubelet 无法自动驱逐 Pod 保护节点。

---

## 四、处理方案与决策

### 4.1 最终决策：维持现状，不重启

理由：
1. 该节点跑了 3 年核心中间件，贸然重启风险远大于收益
2. 故障 3 年未引发业务问题，证明实际无害
3. 有每日巡检 + 磁盘满告警兜底，eviction manager 失效的隐患可控
4. 排到下次维护窗口再处理

### 4.2 可选修复方案（供维护窗口参考）

#### 方案 A：重启 kubelet（推荐，非重启节点）

```bash
# 在 10.18.130.15 上，低峰期操作
# 0. 先看节点负载
kubectl get pods --all-namespaces --field-selector spec.nodeName=10.18.130.15-share

# 1. cordon，防止重启期间调度新 Pod
kubectl cordon 10.18.130.15-share --kubeconfig=/etc/kubernetes/admin.conf

# 2. 重启 kubelet（容器不停，只是 kubelet 进程重启）
systemctl restart kubelet
sleep 30
systemctl status kubelet | head -5

# 3. 验证 stats 接口恢复（关键验证）
kubectl --kubeconfig=/etc/kubernetes/admin.conf get --raw \
  "/api/v1/nodes/10.18.130.15-share/proxy/stats/summary" 2>&1 | head -5

# 4. 恢复调度
kubectl uncordon 10.18.130.15-share --kubeconfig=/etc/kubernetes/admin.conf

# 5. 确认节点上 Pod 都正常
kubectl get pods --all-namespaces --field-selector spec.nodeName=10.18.130.15-share
```

**预期**：kubelet 重启后会重读 `/proc/partitions`，重新加载 253:65，缓存重建成功，stats 接口恢复 200。

**风险**：kubelet 重启后会"重新对账"所有 Pod。99% 无害，有 1% 概率某个 Pod 状态不一致被"修正"（如重启容器）。所以必须在低峰期 + cordon 后操作。

**关键时间窗**：kubelet 重启需在 `pod-eviction-timeout`（默认 5 分钟）内完成，否则 Pod 会被驱逐。正常重启 30 秒，远低于 5 分钟。

#### 方案 B：重启节点（终极手段，不推荐在生产直接用）

```bash
reboot
```

100% 能解决（内核重新 settle 设备），但**节点上所有 Pod 会重启重调度**，对核心中间件影响大。仅作为方案 A 失败后的兜底。

#### 方案 C：热修复（本环境不可用）

通过 cadvisor 4194 端口强制刷新设备缓存。**但在该环境验证不可行**：

```bash
$ curl -kv 127.0.0.1:4194
Failed connect to 127.0.0.1:4194; 拒绝连接
$ netstat -tunlp | grep 4194
（无输出）
```

kubelet 1.x 默认不暴露 cadvisor 独立端口，热修复路径走不通。

### 4.3 不重启期间的替代监控

既然 kubelet 的 rootFs 统计坏了，可绕过 kubelet 用系统命令直接监控磁盘：

```bash
# 定期检查该节点磁盘真实状态
ssh 10.18.130.15 "df -h /var/lib/kubelet /var/log/containers"

# 重点监控中间件日志增长
ssh 10.18.130.15 "du -sh /var/log/containers/*afc-dragon* | sort -rh | head -10"
```

只要磁盘使用率没异常上涨，该节点可安全维持现状。

---

## 五、排查命令速查（下次遇到同类问题直接用）

```bash
# 1. 确认是否 kubelet stats 接口故障（最直接）
kubectl get --raw "/api/v1/nodes/<NODE>/proxy/stats/summary"

# 2. 看故障节点的真实设备挂载
df -h /var/lib/kubelet
findmnt -n -o SOURCE /var/lib/kubelet

# 3. 看内核设备表里报错的 major:minor 是否存在
cat /proc/partitions | grep "<major>"

# 4. 看 kubelet 日志的 rootFs 相关报错
journalctl -u kubelet --since "10 min ago" | grep -iE "rootFs|partition|device"

# 5. 看 device-mapper / virtio 设备情况
ls -l /dev/dm-*
ls -l /dev/mapper/
```

---

## 六、复盘要点

1. **k8s-patrol 的降级设计经受住了真实故障考验**：单节点指标失败没有导致工具崩溃，而是降级提示 + 跳过 + 继续生成报告。
2. **但降级策略有改进空间**：原实现把指标失败的节点「静默跳过」，导致报告里该节点消失。已改进为「降级行显示」——节点以「指标不可用」状态出现在报告里，健康度判定为「警告」（因节点本身 Ready）。
3. **生产环境的核心中间件节点，稳定性优先于"完美状态"**：跑了 3 年的节点，只要故障实际无害，维持现状优于冒险修复。
4. **监控盲区要用替代手段补位**：kubelet 统计坏掉期间，用 `df`/`du` 等系统命令直接监控磁盘，确保 eviction manager 失效的隐患可控。

---

*文档生成时间：2026-07-30 · 基于 k8s-patrol 巡检发现的实际生产故障整理*
