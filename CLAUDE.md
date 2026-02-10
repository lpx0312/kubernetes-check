# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 项目概述

这是一个 Kubernetes Pod 监控和检查工具，用于检查 K8S 集群中 Pod 的重启情况、异常状态以及节点资源使用情况。

## 开发环境设置

```bash
# 设置 Go 代理（中国国内环境）
go env -w GOPROXY=https://mirrors.aliyun.com/goproxy/,direct

# 或者使用多级代理
go env -w GOPROXY=https://goproxy.cn,https://goproxy.io,https://mirrors.aliyun.com/goproxy/,direct

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
# 基本运行 - 查看7天内重启的Pod（默认）
go run cmd/pod-monitor/main.go --kubeconfig=C:\Users\lipanx\.kube\config

# 使用并发处理加速（推荐用于大规模集群）
go run cmd/pod-monitor/main.go --workers=20 --kubeconfig=$env:USERPROFILE\.kube\config

# 查看异常状态的Pod
go run cmd/pod-monitor/main.go --abnormal --kubeconfig=$env:USERPROFILE\.kube\config

# 查看所有命名空间
go run cmd/pod-monitor/main.go -A --kubeconfig=$env:USERPROFILE\.kube\config

# 查看特定命名空间
go run cmd/pod-monitor/main.go -n kube-system --kubeconfig=$env:USERPROFILE\.kube\config

# 显示节点资源使用情况
go run cmd/pod-monitor/main.go --node-metrics --kubeconfig=$env:USERPROFILE\.kube\config
```

### 构建可执行文件

```powershell
# Windows 构建
$env:GOOS = "windows"
$env:GOARCH = "amd64"
go build -o pod-monitor.exe ./cmd/pod-monitor/

# Linux 构建
$env:GOOS = "linux"
$env:GOARCH = "amd64"
go build -o pod-monitor-linux ./cmd/pod-monitor/
```

### 构建后的使用

```bash
# 获取重启的pod（默认3天内）
./pod-monitor-linux --days=3

# 获取异常的pod
./pod-monitor-linux --abnormal
```

## 架构和代码结构

### 项目结构

```
.
├── cmd/
│   └── pod-monitor/
│       └── main.go          # 主程序入口
├── deploy/
│   └── deployment.yaml      # K8S 部署配置
├── Dockerfile               # Docker 构建文件
├── go.mod                   # Go 模块依赖
└── README.md                # 项目文档
```

### 核心架构

程序采用**并发 Worker Pool 模式**处理大量 Pod：

1. **K8S 客户端初始化**
   - 使用 `client-go` 库连接 K8S API
   - 创建两个客户端：`clientset`（核心 API）和 `metricsClient`（Metrics API）
   - 配置提升的 API 请求速率限制（QPS: 50, Burst: 100）

2. **节点预加载缓存** (`preloadNodeCache`)
   - 在处理 Pod 前，预先加载所有节点信息到 `sync.Map` 缓存
   - 避免在处理每个 Pod 时重复查询节点 API
   - 缓存节点名称到 IP 地址的映射

3. **并发处理流水线**
   - **分发阶段**：将 Pod 列表发送到 `podChan` 通道
   - **处理阶段**：Worker Pool（默认 10 个协程）从通道读取 Pod 并行处理
   - **收集阶段**：处理结果发送到 `resultChan` 通道，最终渲染为表格

4. **核心处理函数** (`processPod`)
   - 根据命令行参数执行不同的检查模式：
     - **重启检查模式**：分析容器重启次数、时间、原因
     - **异常检查模式**：检查 Pod 状态、容器等待/终止状态、就绪状态

5. **节点监控模式** (`displayNodeMetrics`)
   - 通过 Metrics API 获取节点 CPU/内存使用量
   - 计算使用率百分比（使用 Allocatable 而非 Capacity）
   - 显示节点 Ready 状态

### 关键优化点

1. **节点信息缓存**：使用 `sync.Map` 缓存节点 IP，减少 API 调用
2. **并发处理**：Worker Pool 模式通过 `--workers` 参数控制并发度
3. **流水线架构**：数据获取、处理、展示分离，各阶段独立运行
4. **速率限制提升**：提高 QPS 和 Burst 以处理大规模集群

### 命令行参数

- `--version`: 显示版本信息
- `--kubeconfig`: 指定 kubeconfig 文件路径
- `--days`: 显示最近 N 天内重启的 Pod（默认 7 天）
- `--workers`: 并发处理的工作协程数（默认 10）
- `--abnormal`: 仅显示异常状态 Pod
- `-A`: 查看所有命名空间的 Pod
- `-n`: 指定要查看的命名空间（默认 default）
- `--node-metrics`: 显示节点资源使用情况

### K8S 版本兼容性

项目使用 Kubernetes 1.17.2 客户端库（`k8s.io/client-go v0.17.2`），这是为了兼容旧版本的 K8S 集群。

### 输出格式

使用 `tablewriter` 库生成格式化的终端表格输出，支持彩色显示。

## 依赖项

- `k8s.io/client-go v0.17.2`: Kubernetes Go 客户端
- `k8s.io/metrics v0.17.2`: Kubernetes Metrics API 客户端
- `github.com/olekukonko/tablewriter`: 表格格式化输出

## 开发注意事项

1. **时区处理**：所有时间显示转换为 UTC+8（北京时间）
2. **错误处理**：使用 `log.Fatalf` 处理致命错误，`log.Printf` 记录警告
3. **资源计算**：
   - CPU 使用量单位：毫核（milli-core）
   - 内存使用量单位：Mi（MiB）
   - 使用 `Allocatable` 而非 `Capacity` 计算内存使用率，更准确反映可用资源

## 未来计划

根据 README.md 中提到的优化方向，项目计划扩展以下功能：

- GitLab 接口检查
- etcd/MySQL/Redis 检查
- 备份文件检查
- 平台组件检查（前端/后端/数据库/中间件）
- 集群健康状态检查（API Server、控制平面组件）
- 资源配额监控
- 网络检查（CoreDNS、节点间延迟）
- 存储检查（PV/PVC、StorageClass）
- 安全审计（RBAC、证书、Pod 安全策略）
