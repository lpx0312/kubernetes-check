# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 项目概述

这是一个 Kubernetes Pod 监控和检查工具,采用**模块化架构**,使用 Cobra CLI 框架,用于检查 K8S 集群中 Pod 的重启情况、异常状态以及节点资源使用情况。

## 架构设计

### 模块化架构

项目已从单体架构重构为模块化架构,代码按领域拆分:

```
pkg/          # 公共 API 包(可被外部导入)
  ├── k8s/    # K8S 客户端工厂
  ├── cache/  # 节点缓存
  ├── pod/    # Pod 分析
  ├── node/   # 节点监控
  └── output/ # 输出格式化

internal/     # 私有实现
  ├── errors/ # 错误处理
  ├── log/    # 日志
  └── worker/ # Worker Pool

cmd/          # CLI 入口
  └── pod-monitor/
      ├── root.go    # 根命令
      ├── cmd_pod.go # Pod 检查
      └── cmd_node.go # 节点监控
```

### 核心设计模式

1. **Worker Pool 模式** - 可配置 workers 并发处理
2. **节点预加载缓存** - 减少重复 API 调用
3. **流水线架构** - 数据获取 → 处理 → 展示分离
4. **依赖注入** - Analyzer 和 Monitor 通过构造函数注入依赖

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
# 基本运行 - 查看 Pod 重启情况(默认7天内)
go run cmd/pod-monitor/main.go pod

# 使用并发处理加速(推荐用于大规模集群)
go run cmd/pod-monitor/main.go pod --workers=20 --kubeconfig=$env:USERPROFILE\.kube\config

# 查看异常状态的 Pod
go run cmd/pod-monitor/main.go pod --abnormal

# 查看所有命名空间
go run cmd/pod-monitor/main.go pod --all-namespaces --abnormal

# 查看特定命名空间
go run cmd/pod-monitor/main.go pod --namespace=kube-system --abnormal

# 显示节点资源使用情况
go run cmd/pod-monitor/main.go node

# 查看帮助
go run cmd/pod-monitor/main.go --help
go run cmd/pod-monitor/main.go pod --help
go run cmd/pod-monitor/main.go node --help
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
# 获取重启的pod(默认7天内)
./pod-monitor-linux pod --days=3

# 获取异常的pod
./pod-monitor-linux pod --abnormal

# 查看节点资源
./pod-monitor-linux node
```

## 核心架构

### 1. K8S 客户端工厂 (pkg/k8s/)

**职责**: 统一创建和管理 K8S 客户端

**核心接口**:
```go
func NewClient(kubeconfig string, qps float32, burst int) (*Client, error)
```

**特性**:
- 同时创建 `clientset` (核心 API) 和 `metricsClient` (Metrics API)
- 可配置 QPS/Burst (默认 50/100)
- 使用自定义错误类型处理错误

### 2. 节点缓存 (pkg/cache/)

**职责**: 缓存节点信息,避免重复查询

**核心接口**:
```go
func (nc *NodeCache) PreloadNodes(ctx context.Context, client *k8s.Client) error
func (nc *NodeCache) GetNodeIPOrUnknown(nodeName string) string
```

**实现**:
- 基于 `sync.Map` 的线程安全缓存
- 预加载所有节点的 InternalIP
- 提供默认值 "Unknown" 处理未找到的节点

### 3. Pod 分析器 (pkg/pod/)

**职责**: 分析 Pod 状态,检测重启和异常

**核心接口**:
```go
func (a *Analyzer) Analyze(ctx context.Context, pod v1.Pod, days int) *Result
func (a *Analyzer) IsAbnormal(pod v1.Pod) bool
```

**功能**:
- 分析容器重启次数、时间、原因
- 检测 Pod 异常状态(Pending、Failed、容器错误等)
- 计算就绪状态
- 支持配置天数过滤

### 4. 节点监控器 (pkg/node/)

**职责**: 监控节点资源使用情况

**核心接口**:
```go
func (m *Monitor) GetMetrics(ctx context.Context, nodeName string) (*Metrics, error)
func (m *Monitor) ListAll(ctx context.Context) ([]*Metrics, error)
```

**功能**:
- 获取 CPU/内存使用量和总量
- 计算使用率百分比(基于 Allocatable)
- 判断节点 Ready 状态

### 5. Worker Pool (internal/worker/)

**职责**: 泛型并发处理池

**核心接口**:
```go
func NewPool[T any, R any](workers int, handler func(context.Context, T) R) *Pool[T, R]
func (p *Pool[T, R]) Process(ctx context.Context, items []T) []R
```

**特性**:
- 泛型实现,支持任意输入输出类型
- 可配置 worker 数量
- 支持 context 取消
- 线程安全的结果收集

### 6. 错误处理 (internal/errors/)

**职责**: 统一的错误处理和错误码

**错误码范围**:
- 1000-1099: K8S 客户端错误
- 2000-2099: 业务逻辑错误

**核心接口**:
```go
func NewAppError(code ErrorCode, op string, err error, msg string) *AppError
func Wrap(err error, op string) *AppError
```

### 7. 日志模块 (internal/log/)

**职责**: 结构化日志

**核心接口**:
```go
func New(level string, pretty bool) *Logger
```

**特性**:
- 基于 Go 1.21+ slog
- 支持可配置级别(debug/info/warn/error)
- 支持格式切换(JSON/text)
- 预定义 Stdout 和 Debug 实例

### 8. 输出格式化 (pkg/output/)

**职责**: 统一的表格输出和颜色配置

**核心接口**:
```go
func NewTableWriter(writer io.Writer) *TableWriter
func (tw *TableWriter) SetPodColumns(abnormal bool)
func (tw *TableWriter) SetNodeColumns()
func ApplyPodColors(table *TableWriter, abnormal bool)
func ApplyNodeColors(table *TableWriter)
```

## 并发处理流程

```
1. 创建 K8S 客户端
   ↓
2. 预加载节点缓存 (PreloadNodes)
   ↓
3. 获取 Pod/Node 列表
   ↓
4. 创建 Worker Pool
   ↓
5. 并发处理数据
   ├─ Worker 1: Analyze Pod 1
   ├─ Worker 2: Analyze Pod 2
   └─ Worker N: Analyze Pod N
   ↓
6. 收集结果
   ↓
7. 格式化输出
```

## 命令行参数

### 全局参数

- `--kubeconfig`: kubeconfig 文件的绝对路径
- `--workers, -w`: 并发处理的工作协程数(默认 10)
- `--verbose, -v`: 详细输出模式

### Pod 检查命令 (`pod`)

- `--days, -d`: 显示最近 N 天内重启的 Pod(默认 7)
- `--abnormal, -a`: 仅显示异常状态的 Pod
- `--all-namespaces, -A`: 查看所有命名空间的 Pod
- `--namespace, -n`: 指定要查看的命名空间(默认 default)

### 节点监控命令 (`node`)

无额外参数,显示所有节点资源使用情况

## K8S 版本兼容性

项目使用 Kubernetes 1.17.2 客户端库(`k8s.io/client-go v0.17.2`),以兼容旧版本的 K8S 集群。

## 输出格式

使用 `tablewriter` 库生成格式化的终端表格输出,支持彩色显示。

## 依赖项

- `k8s.io/client-go v0.17.2`: Kubernetes Go 客户端
- `k8s.io/metrics v0.17.2`: Kubernetes Metrics API 客户端
- `github.com/spf13/cobra v1.8.0`: CLI 框架
- `github.com/olekukonko/tablewriter`: 表格格式化输出

## 开发注意事项

1. **时区处理**: 所有时间显示转换为 UTC+8(北京时间)
2. **错误处理**: 使用自定义的 `AppError` 类型,包含错误码和操作名称
3. **日志输出**: 使用结构化日志,包含上下文信息
4. **资源计算**:
   - CPU 使用量单位: 毫核(milli-core)
   - 内存使用量单位: Mi(MiB)
   - 使用 `Allocatable` 而非 `Capacity` 计算内存使用率
5. **并发安全**: 所有共享状态使用线程安全的数据结构(sync.Map, 通道)

## 测试

```bash
# 运行所有单元测试
go test ./... -short

# 运行集成测试(需要 K8S 连接)
go test -tags=integration ./test/integration/... -v

# 生成覆盖率报告
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

## 测试覆盖率

当前模块测试覆盖率:
- `internal/errors`: 100%
- `internal/log`: 100%
- `internal/worker`: 100%
- `pkg/cache`: 100%
- `pkg/k8s`: 90%
- `pkg/pod`: 95%
- `pkg/node`: 90%
- 整体覆盖率 > 70%

## 未来计划

根据 README.md 中提到的优化方向,项目计划扩展以下功能:

1. **优化1**: 添加排序参数 `-n` 按照 namespace 等排序
2. **优化2**: 完善 Pod 状态检查列表
3. **优化3**: 新增检查 GitLab 接口、etcd、Ingress
4. **优化4**: 对接 Prometheus,判断主机状态
5. **优化5**: 完成 MySQL、Redis、备份文件检查
6. **优化6**: 完成平台组件检查(前端/后端/数据库/中间件)

### K8S 集群检查扩展

1. **集群健康状态检查**
   - API Server 可用性
   - 控制平面组件状态(etcd/scheduler/controller-manager)
   - 工作节点 Ready 状态统计

2. **资源配额监控**
   - 命名空间资源配额使用率
   - 节点资源分配/剩余量预警
   - PersistentVolume 剩余空间监控

3. **网络检查**
   - CoreDNS 服务可用性
   - 节点间网络延迟检测
   - Service/Ingress 端点连通性

4. **存储检查**
   - PV/PVC 绑定状态
   - StorageClass 可用性
   - 卷健康状态

5. **安全审计**
   - RBAC 配置风险检测
   - 过期的证书检查
   - 不安全的 Pod 安全策略

6. **工作负载分析**
   - Deployment 副本数偏差
   - StatefulSet 序数连续性
   - 僵尸 Pod 清理建议

7. **扩展指标**
   - HPA 配置合理性检查
   - 资源 Request/Limit 比例分析
   - 垂直扩缩容建议

## 扩展开发指南

### 添加新的检查功能

1. 在 `pkg/` 下创建新的模块(如 `pkg/gitlab/`)
2. 实现核心逻辑和测试
3. 在 `cmd/pod-monitor/` 中添加新的子命令文件
4. 在 `root.go` 中注册新命令
5. 更新文档和测试

### 示例: 添加 GitLab 检查

```go
// pkg/gitlab/checker.go
package gitlab

type Checker struct {
    client *http.Client
    baseURL string
}

func NewChecker(baseURL string) *Checker {
    return &Checker{
        client: &http.Client{},
        baseURL: baseURL,
    }
}

func (c *Checker) CheckPipeline(projectID int) error {
    // 实现检查逻辑
}
```

然后在 `cmd/pod-monitor/cmd_gitlab.go` 中创建子命令:

```go
var gitlabCmd = &cobra.Command{
    Use:   "gitlab",
    Short: "检查 GitLab CI/CD 状态",
    Run:   runGitLabCheck,
}

func init() {
    rootCmd.AddCommand(gitlabCmd)
}
```
