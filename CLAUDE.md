# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 项目概述

k8s-patrol 是一个 Kubernetes 集群巡检 CLI 工具，采用 Cobra 子命令 + 三层架构（Collector → Report → Renderer）。检查节点资源、Pod 重启/异常状态、PVC 存储使用情况，并可生成自包含的 HTML 巡检报告。

## 开发环境设置

```bash
# 设置 Go 代理（中国国内环境）
go env -w GOPROXY=https://goproxy.cn,direct

# 下载依赖
go mod download

# 如果遇到依赖问题，清理并重新下载
go clean -modcache
go mod verify
go mod download
```

## 构建和运行

### 运行程序

```bash
# 查看所有命令
go run ./cmd/k8s-patrol --help

# 检查近7天重启的Pod（所有命名空间）
go run ./cmd/k8s-patrol restart -d 7 -A --kubeconfig=$env:USERPROFILE\.kube\config

# 检查当前异常的Pod
go run ./cmd/k8s-patrol abnormal -A --kubeconfig=$env:USERPROFILE\.kube\config

# 检查节点资源
go run ./cmd/k8s-patrol node --kubeconfig=$env:USERPROFILE\.kube\config

# 检查PVC存储
go run ./cmd/k8s-patrol storage -A --kubeconfig=$env:USERPROFILE\.kube\config

# 生成全量HTML巡检报告
go run ./cmd/k8s-patrol report -o report.html --kubeconfig=$env:USERPROFILE\.kube\config
```

### 构建可执行文件

```bash
# 本地构建
go build -o k8s-patrol ./cmd/k8s-patrol/

# 交叉编译（生产构建，裁剪调试信息）
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o k8s-patrol-linux-amd64 ./cmd/k8s-patrol/
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags="-s -w" -o k8s-patrol-linux-arm64 ./cmd/k8s-patrol/
```

### 构建后的使用

```bash
# 检查近3天重启的pod
./k8s-patrol restart -d 3

# 检查异常的pod
./k8s-patrol abnormal -A

# 检查存储
./k8s-patrol storage -A

# 生成巡检报告
./k8s-patrol report -o weekly.html
```

## 架构和代码结构

### 项目结构

```
.
├── cmd/k8s-patrol/           # CLI 入口（Cobra 子命令）
│   ├── main.go               # 仅调 rootCmd.Execute()
│   ├── root.go               # 根命令 + 全局 flag + version 子命令
│   ├── cmd_node.go           # node 子命令（节点资源）
│   ├── cmd_restart.go        # restart 子命令（Pod重启，-d/--days）
│   ├── cmd_abnormal.go       # abnormal 子命令（Pod异常）
│   ├── cmd_storage.go        # storage 子命令（PVC/孤儿PV）
│   └── cmd_report.go         # report 子命令（HTML全量报告，-o/--output）
├── internal/
│   ├── collector/            # 数据采集层
│   │   ├── collector.go      # Collector 结构体 + Collect() 入口 + worker pool
│   │   ├── pod.go            # Pod 采集（重启/异常判定）
│   │   ├── node.go           # 节点采集（metrics API + 节点缓存）
│   │   └── storage.go        # 存储采集（PVC/PV + kubelet stats）
│   ├── report/
│   │   └── types.go          # 巡检结果数据结构（三层架构的契约）
│   └── renderer/
│       ├── table.go          # 终端表格渲染
│       ├── html.go           # HTML 报告渲染 + view model
│       └── report_template.go# HTML 模板 + 内联 CSS
├── docs/                     # 文档
├── go.mod                    # Go 模块依赖
└── README.md
```

### 三层架构

```
CLI（cmd/）  →  Collector（internal/collector/）  →  Report（internal/report/）  →  Renderer（internal/renderer/）
  Cobra子命令      数据采集，产出 report.Report        巡检结果数据结构              终端表格 + HTML 报告
```

- **Collector**：封装 K8s 客户端构造、worker pool 并发采集、节点缓存。所有采集失败降级为 Note 提示，不崩溃。
- **Report**：纯数据结构，是 Collector 与 Renderer 之间的契约，无业务逻辑。
- **Renderer**：消费 Report，与采集逻辑彻底解耦。终端表格和 HTML 报告各自独立。

### 核心机制

1. **K8S 客户端初始化**（collector.go `New()`）
   - 加载 kubeconfig，提取 context 名用于报告头
   - 创建 clientset（核心 API）、metricsClient（Metrics API）、restClient（kubelet proxy）
   - 提升速率限制（QPS: 50, Burst: 100）

2. **节点缓存**（node.go `preloadNodeCache`）
   - 预加载所有节点信息到 `sync.Map`（节点名 → 内网 IP）
   - worker pool 并发查 IP 时命中缓存，未命中才回退 API 查询

3. **Pod 并发处理**（collector.go `collectPods`）
   - worker pool 模式，`--workers` 控制并发度
   - 重启检查和异常检查是两个独立分支（abnormal=true/false）

4. **存储检查**（storage.go `collectStorage`）
   - PVC 绑定状态：K8s API 列 PVC + PV 关联
   - PVC 使用量：kubelet `/stats/summary` API（有 pvcRef 直接关联，零 SSH）
   - 孤儿 PV：筛 Released/Failed 且无 PVC 引用的 PV
   - 自定义 kubelet stats 最小类型（零依赖，不引 k8s.io/kubelet）

5. **健康度判定**（collector.go `computeSummary`）
   - 严重：异常节点 / metrics-server 缺失 / Lost PVC / Failed PV / PVC 使用率≥95%
   - 警告：异常 Pod / 重启 Pod / Pending PVC / Released PV / PVC 使用率 85%-95%

6. **HTML 报告渲染**（renderer/html.go）
   - 自包含单文件（CSS 内联），浏览器/邮件直接打开
   - view model 模式：模板只展示，所有格式化和阈值判定在 Go 侧预计算

### 命令行参数

Cobra 子命令架构，全局选项所有子命令继承：

**全局选项：**
- `-k, --kubeconfig`：kubeconfig 文件路径
- `-A, --all-namespaces`：查询所有命名空间
- `-n, --namespace`：指定命名空间（默认 default）
- `-w, --workers`：并发处理数（默认 10）

**子命令：**
- `node`：检查节点资源使用情况
- `restart`：检查近期重启的 Pod（`-d, --days` 回溯天数，默认 7）
- `abnormal`：检查当前异常的 Pod
- `storage`：检查 PVC 绑定状态与使用量
- `report`：生成全量 HTML 巡检报告（`-o, --output` 默认 report-YYYYMMDD.html）
- `version`：显示版本信息
- `completion`：生成 shell 自动补全脚本（bash/zsh/fish/powershell）

### K8S 版本兼容性

项目使用 Kubernetes 1.31.x 客户端库（`k8s.io/client-go v0.31.14`），匹配 K8s 1.31 集群。

### 依赖项

- `k8s.io/client-go v0.31.14`：Kubernetes Go 客户端
- `k8s.io/metrics v0.31.14`：Metrics API 客户端
- `github.com/spf13/cobra`：CLI 子命令框架
- `github.com/olekukonko/tablewriter`：终端表格格式化

## 开发注意事项

1. **时区处理**：所有时间显示转换为 UTC+8（北京时间）
2. **错误处理**：CLI 层用 `RunE` 返回 error；采集层失败降级为 `rep.AddNote()`，不崩溃
3. **资源计算口径**：
   - CPU 使用量单位：毫核（milli-core）
   - 内存使用量单位：Mi（MiB）
   - 节点 CPU/内存总量统一使用 `Allocatable`（非 Capacity），反映真实可分配资源
   - 存储容量单位：Gi（GiB），使用率 = usedBytes / capacityBytes
4. **PVC 使用量盲区**：kubelet stats 只统计被 Pod 挂载的卷；未挂载的 PVC 显示"未挂载"，其卷内可能仍有数据，使用量暂不可知
5. **渲染解耦**：新增检查项只需加 collector 采集 + report 数据结构 + renderer 渲染三处，互不影响

## 未来计划

- 事件检查（Warning Event 根因定位）
- 工作负载检查（Deployment 副本偏差、StatefulSet 序数连续性）
- 网络检查（CoreDNS、Service/Endpoint 连通性）
- PDF 报告输出（基于 HTML + 无头浏览器）
