# Kubernetes Pod 监控工具

一个强大的 Kubernetes 集群监控工具,用于检查 Pod 的重启情况、异常状态以及节点资源使用情况。

## 特性

- ✅ **Pod 重启检查** - 查看指定时间范围内重启的 Pod
- ✅ **Pod 异常检测** - 自动识别异常状态的 Pod
- ✅ **节点资源监控** - 实时查看节点 CPU/内存使用率
- ✅ **并发处理** - 支持可配置的并发度,加速大规模集群检查
- ✅ **模块化架构** - 清晰的代码结构,易于扩展和维护

## 快速开始

### 环境准备

```bash
# 设置 Go 代理(中国国内环境)
go env -w GOPROXY=https://mirrors.aliyun.com/goproxy/,direct

# 下载依赖
go mod download

# 如果遇到依赖问题,清理并重新下载
go clean -modcache
go mod verify
go mod download
```

### 运行程序

#### 使用新的模块化 CLI

```bash
# 查看 Pod 重启情况(默认7天内)
go run cmd/pod-monitor/main.go pod

# 查看最近3天内重启的 Pod
go run cmd/pod-monitor/main.go pod --days=3

# 查看异常状态的 Pod
go run cmd/pod-monitor/main.go pod --abnormal

# 查看所有命名空间的异常 Pod
go run cmd/pod-monitor/main.go pod --abnormal --all-namespaces

# 查看特定命名空间
go run cmd/pod-monitor/main.go pod -n kube-system --abnormal

# 使用20个并发worker加速处理
go run cmd/pod-monitor/main.go pod --workers=20 --abnormal

# 查看节点资源使用情况
go run cmd/pod-monitor/main.go node

# 查看帮助信息
go run cmd/pod-monitor/main.go --help
go run cmd/pod-monitor/main.go pod --help
go run cmd/pod-monitor/main.go node --help
```

### 构建可执行文件

#### Windows 构建

```powershell
$env:GOOS = "windows"
$env:GOARCH = "amd64"
go build -o pod-monitor.exe ./cmd/pod-monitor/
```

#### Linux 构建

```bash
GOOS=linux GOARCH=amd64 go build -o pod-monitor-linux ./cmd/pod-monitor/
```

### 使用构建后的程序

```bash
# 获取重启的pod(默认7天内)
./pod-monitor-linux pod --days=3

# 获取异常的pod
./pod-monitor-linux pod --abnormal

# 查看节点资源
./pod-monitor-linux node

# 使用高并发加速
./pod-monitor-linux pod --workers=20 --abnormal
```

## 性能优化说明

程序通过以下优化实现加速:

1. **节点信息预加载** - 通过 `PreloadNodes` 函数提前加载所有节点信息到内存缓存
2. **并行处理** - 使用 Worker Pool 模式并发处理 Pod,通过 `--workers` 参数控制并发数
3. **缓存机制** - 使用 `sync.Map` 缓存节点 IP 信息,减少 API 调用次数
4. **流水线处理** - 分离数据获取、处理和展示阶段
5. **API 速率限制优化** - 提升 QPS 和 Burst 限制(默认 50/100)

## 命令行参数

### 全局参数

- `--kubeconfig`: 指定 kubeconfig 文件路径
- `--workers, -w`: 并发处理的工作协程数(默认 10)
- `--verbose, -v`: 详细输出模式

### Pod 检查命令 (`pod`)

- `--days, -d`: 显示最近 N 天内重启的 Pod(默认 7)
- `--abnormal, -a`: 仅显示异常状态的 Pod
- `--all-namespaces, -A`: 查看所有命名空间的 Pod
- `--namespace, -n`: 指定要查看的命名空间(默认 default)

### 节点监控命令 (`node`)

- 显示所有节点的 CPU/内存使用率和状态

## 项目架构

### 目录结构

```
.
├── cmd/pod-monitor/          # CLI 入口
│   ├── main.go              # 主程序入口
│   ├── root.go              # 根命令定义
│   ├── cmd_pod.go           # Pod 检查子命令
│   └── cmd_node.go          # 节点监控子命令
├── pkg/                      # 公共 API 包
│   ├── k8s/                 # K8S 客户端工厂
│   ├── cache/               # 节点信息缓存
│   ├── pod/                 # Pod 分析逻辑
│   ├── node/                # 节点监控逻辑
│   └── output/              # 输出格式化
├── internal/                 # 私有实现
│   ├── errors/              # 错误处理
│   ├── log/                 # 结构化日志
│   └── worker/              # Worker Pool
├── test/                     # 测试
│   └── integration/         # 集成测试
└── docs/plans/               # 设计文档
```

### 核心模块

- **pkg/k8s**: K8S 客户端工厂,统一创建 Clientset 和 MetricsClient
- **pkg/cache**: 基于 `sync.Map` 的线程安全节点缓存
- **pkg/pod**: Pod 分析器,检测重启和异常状态
- **pkg/node**: 节点监控器,计算资源使用率
- **internal/worker**: 泛型 Worker Pool,支持并发处理
- **internal/log**: 基于 Go 1.21+ slog 的结构化日志
- **internal/errors**: 统一的错误处理和错误码

## 技术栈

- **语言**: Go 1.23.0
- **K8S 客户端**: client-go v0.17.2
- **Metrics API**: k8s.io/metrics v0.17.2
- **CLI 框架**: Cobra v1.8.0
- **日志**: Go 1.21+ slog
- **测试**: Go testing 包
- **输出**: tablewriter

## 开发

### 运行测试

```bash
# 运行所有单元测试
go test ./... -short

# 运行集成测试(需要 K8S 连接)
go test -tags=integration ./test/integration/... -v
```

### 代码覆盖率

```bash
# 生成覆盖率报告
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

## 未来计划

根据需求文档,计划扩展以下功能:

1. **优化1**: 添加排序参数 `-n` 按照 namespace 等排序
2. **优化2**: 完善Pod状态检查列表
3. **优化3**: 新增检查 GitLab 接口、etcd、Ingress
4. **优化4**: 对接 Prometheus,判断主机状态
5. **优化5**: 完成 MySQL、Redis、备份文件检查
6. **优化6**: 完成平台组件检查(前端/后端/数据库/中间件)

### K8S 集群检查

1. **集群健康状态检查**
   - API Server 可用性
   - 控制平面组件状态
   - 工作节点 Ready 状态统计

2. **资源配额监控**
   - 命名空间资源配额使用率
   - 节点资源分配/剩余量预警

3. **网络检查**
   - CoreDNS 服务可用性
   - 节点间网络延迟检测

4. **存储检查**
   - PV/PVC 绑定状态
   - StorageClass 可用性

5. **安全审计**
   - RBAC 配置风险检测
   - 过期的证书检查

## 贡献

欢迎提交 Issue 和 Pull Request!

## 许可证

MIT License
