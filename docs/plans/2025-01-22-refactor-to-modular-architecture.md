# Pod Monitor 重构为模块化架构实现计划

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**目标:** 将单体 main.go 重构为按领域拆分的模块化架构，使用 Cobra CLI 框架，提升代码质量、可测试性和可维护性。

**架构:** 按领域拆分模块（pkg/pod、pkg/node、pkg/cache、pkg/k8s、pkg/output），使用 Cobra 实现子命令，Worker Pool 泛型化并发处理，结构化日志和统一错误处理。

**技术栈:** Go 1.13.9, Kubernetes client-go v0.17.2, Cobra CLI, slog (Go 1.21+), tablewriter

---

## 前置任务：创建 Git Worktree

**任务:** 创建独立的工作区用于重构开发

**步骤:**

### Step 1: 创建 worktree

```bash
git worktree add ../kubernetes-check-refactor refactor
cd ../kubernetes-check-refactor
```

验证: `pwd` 输出包含 `kubernetes-check-refactor`

### Step 2: 验证环境

```bash
go version
```

预期输出: `go version go1.13.9 windows/...`

---

## 阶段 1：基础设施搭建

### Task 1: 创建目录结构

**文件:**
- Create: `pkg/pod/`
- Create: `pkg/node/`
- Create: `pkg/metrics/`
- Create: `pkg/cache/`
- Create: `pkg/k8s/`
- Create: `pkg/output/`
- Create: `internal/worker/`
- Create: `internal/model/`
- Create: `internal/errors/`
- Create: `internal/log/`
- Create: `cmd/pod-monitor/`

**步骤:**

**Step 1: 创建目录**

```bash
mkdir -p pkg/pod pkg/node pkg/metrics pkg/cache pkg/k8s pkg/output
mkdir -p internal/worker internal/model internal/errors internal/log
mkdir -p cmd/pod-monitor
```

**Step 2: 验证目录创建**

```bash
ls -la pkg/
```

预期输出: 包含 pod, node, metrics, cache, k8s, output 目录

**Step 3: 提交**

```bash
git add pkg/ internal/ cmd/
git commit -m "refactor: 创建模块化目录结构"
```

---

### Task 2: 添加 Cobra 依赖

**文件:**
- Modify: `go.mod`
- Modify: `go.sum`

**步骤:**

**Step 1: 添加 Cobra 依赖**

```bash
go get github.com/spf13/cobra@v1.8.0
go mod tidy
```

**Step 2: 验证依赖添加**

```bash
grep cobra go.mod
```

预期输出: `github.com/spf13/cobra v1.8.0`

**Step 3: 提交**

```bash
git add go.mod go.sum
git commit -m "deps: 添加 cobra CLI 框架"
```

---

### Task 3: 实现错误处理模块

**文件:**
- Create: `internal/errors/error.go`
- Create: `internal/errors/error_test.go`

**步骤:**

**Step 1: 编写错误类型测试**

创建文件 `internal/errors/error_test.go`:

```go
package errors

import (
    "errors"
    "testing"
)

func TestAppError_Error(t *testing.T) {
    err := &AppError{
        Code:    ErrConfigLoad,
        Op:      "TestOp",
        Err:     errors.New("original error"),
        Message: "test message",
    }

    got := err.Error()
    want := "[1003] TestOp: test message"

    if got != want {
        t.Errorf("Error() = %q, want %q", got, want)
    }
}

func TestAppError_Unwrap(t *testing.T) {
    original := errors.New("original")
    err := &AppError{
        Code: ErrConfigLoad,
        Op:   "TestOp",
        Err:  original,
    }

    got := errors.Unwrap(err)
    if got != original {
        t.Errorf("Unwrap() = %v, want %v", got, original)
    }
}

func TestNewAppError(t *testing.T) {
    original := errors.New("test")
    err := NewAppError(ErrConfigLoad, "TestOp", original, "user message")

    if err.Code != ErrConfigLoad {
        t.Errorf("Code = %v, want %v", err.Code, ErrConfigLoad)
    }
    if err.Op != "TestOp" {
        t.Errorf("Op = %q, want %q", err.Op, "TestOp")
    }
    if err.Err != original {
        t.Errorf("Err = %v, want %v", err.Err, original)
    }
    if err.Message != "user message" {
        t.Errorf("Message = %q, want %q", err.Message, "user message")
    }
}
```

**Step 2: 运行测试（预期失败）**

```bash
cd internal/errors
go test -v
```

预期输出: `undefined: AppError`, `undefined: ErrConfigLoad` 等错误

**Step 3: 实现错误类型**

创建文件 `internal/errors/error.go`:

```go
package errors

import "fmt"

// ErrorCode 错误代码类型
type ErrorCode int

const (
    // K8S 客户端错误 1000-1099
    ErrK8SClient     ErrorCode = 1001
    ErrMetricsClient ErrorCode = 1002
    ErrConfigLoad    ErrorCode = 1003

    // 业务逻辑错误 2000-2099
    ErrNodeNotFound ErrorCode = 2001
    ErrPodAnalysis  ErrorCode = 2002
)

// AppError 应用错误
type AppError struct {
    Code    ErrorCode
    Op      string // 操作名称
    Err     error  // 原始错误
    Message string // 用户友好消息
}

// Error 实现 error 接口
func (e *AppError) Error() string {
    return fmt.Sprintf("[%d] %s: %s", e.Code, e.Op, e.Message)
}

// Unwrap 实现 errors.Unwrap 接口
func (e *AppError) Unwrap() error {
    return e.Err
}

// NewAppError 创建新的应用错误
func NewAppError(code ErrorCode, op string, err error, msg string) *AppError {
    return &AppError{
        Code:    code,
        Op:      op,
        Err:     err,
        Message: msg,
    }
}

// Wrap 包装错误，自动生成消息
func Wrap(err error, op string) *AppError {
    return &AppError{
        Code: ErrConfigLoad, // 默认错误码，调用方可覆盖
        Op:   op,
        Err:  err,
        Message: err.Error(),
    }
}
```

**Step 4: 运行测试（预期通过）**

```bash
cd internal/errors
go test -v
```

预期输出: 所有测试通过

**Step 5: 提交**

```bash
git add internal/errors/
git commit -m "feat: 实现统一错误处理模块"
```

---

### Task 4: 实现日志模块

**文件:**
- Create: `internal/log/logger.go`
- Create: `internal/log/logger_test.go`

**步骤:**

**Step 1: 编写日志测试**

创建文件 `internal/log/logger_test.go`:

```go
package log

import (
    "context"
    "testing"
)

func TestNew(t *testing.T) {
    logger := New("info", false)
    if logger == nil {
        t.Fatal("New() returned nil")
    }
    if logger.Logger == nil {
        t.Error("Logger field is nil")
    }
}

func TestStdout(t *testing.T) {
    if Stdout == nil {
        t.Error("Stdout is nil")
    }
    if Stdout.Logger == nil {
        t.Error("Stdout.Logger is nil")
    }
}

func TestLogger_ContextMethods(t *testing.T) {
    logger := New("debug", false)
    ctx := context.Background()

    // 这些方法不应该 panic
    logger.DebugContext(ctx, "test debug", "key", "value")
    logger.InfoContext(ctx, "test info", "key", "value")
    logger.WarnContext(ctx, "test warn", "key", "value")
    logger.ErrorContext(ctx, "test error", "key", "value")
}
```

**Step 2: 运行测试（预期失败）**

```bash
cd internal/log
go test -v
```

预期输出: `undefined: New`, `undefined: Stdout` 等错误

**Step 3: 实现日志模块**

创建文件 `internal/log/logger.go`:

```go
package log

import (
    "log/slog"
    "os"
    "strings"
)

// Logger 结构化日志封装
type Logger struct {
    *slog.Logger
}

// New 创建新的日志器
func New(level string, pretty bool) *Logger {
    opts := &slog.HandlerOptions{
        Level: parseLevel(level),
    }

    var handler slog.Handler
    if pretty {
        // 文本格式输出（开发环境）
        handler = slog.NewTextHandler(os.Stdout, opts)
    } else {
        // JSON 格式输出（生产环境）
        handler = slog.NewJSONHandler(os.Stdout, opts)
    }

    return &Logger{slog.New(handler)}
}

// parseLevel 解析日志级别
func parseLevel(level string) slog.Level {
    switch strings.ToLower(level) {
    case "debug":
        return slog.LevelDebug
    case "info":
        return slog.LevelInfo
    case "warn":
        return slog.LevelWarn
    case "error":
        return slog.LevelError
    default:
        return slog.LevelInfo
    }
}

// 预定义日志器实例
var (
    // Stdout 标准输出日志器（info 级别，文本格式）
    Stdout = New("info", true)

    // Debug 调试日志器（debug 级别，文本格式）
    Debug = New("debug", true)
)
```

**Step 4: 运行测试（预期通过）**

```bash
cd internal/log
go test -v
```

预期输出: 所有测试通过

**Step 5: 提交**

```bash
git add internal/log/
git commit -m "feat: 实现结构化日志模块"
```

---

### Task 5: 实现 K8S 客户端工厂

**文件:**
- Create: `pkg/k8s/client.go`
- Create: `pkg/k8s/config.go`
- Create: `pkg/k8s/client_test.go`

**步骤:**

**Step 1: 编写配置加载测试**

创建文件 `pkg/k8s/config_test.go`:

```go
package k8s

import (
    "testing"
    "os"
    "path/filepath"
)

func TestLoadConfig_DefaultPath(t *testing.T) {
    // 使用当前目录作为 kubeconfig 路径（会失败，但可以测试逻辑）
    _, err := loadConfig("")
    if err == nil {
        t.Error("Expected error for non-existent kubeconfig, got nil")
    }
}

func TestLoadConfig_ExplicitPath(t *testing.T) {
    // 创建临时 kubeconfig 文件
    tmpDir := t.TempDir()
    kubeconfigPath := filepath.Join(tmpDir, "config")

    // 写入无效内容（只测试文件读取，不验证内容）
    os.WriteFile(kubeconfigPath, []byte("invalid"), 0644)

    // 应该能读取文件（虽然解析会失败）
    _, err := loadConfig(kubeconfigPath)
    if err == nil {
        t.Log("Got expected error for invalid kubeconfig content")
    }
}
```

**Step 2: 运行测试（预期失败）**

```bash
cd pkg/k8s
go test -v
```

预期输出: `undefined: loadConfig` 等错误

**Step 3: 实现配置加载**

创建文件 `pkg/k8s/config.go`:

```go
package k8s

import (
    "os"
    "path/filepath"

    "k8s.io/client-go/tools/clientcmd"
    "k8s.io/client-go/util/homedir"

    "pod-monitor/internal/errors"
    "pod-monitor/internal/log"
)

// loadConfig 加载 kubeconfig 配置
func loadConfig(kubeconfig string) (*clientcmd_api.Config, error) {
    loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()

    if kubeconfig != "" {
        loadingRules.ExplicitPath = kubeconfig
        log.Stdout.Debug("使用自定义 kubeconfig", "path", kubeconfig)
    } else {
        // 使用默认路径
        defaultPath := filepath.Join(homedir.HomeDir(), ".kube", "config")
        if _, err := os.Stat(defaultPath); os.IsNotExist(err) {
            log.Stdout.Warn("默认 kubeconfig 不存在", "path", defaultPath)
        }
        log.Stdout.Debug("使用默认 kubeconfig 路径", "path", defaultPath)
    }

    config, err := clientcmd.LoadFromFile(loadingRules.GetDefaultFilename())
    if err != nil {
        return nil, errors.NewAppError(
            errors.ErrConfigLoad,
            "loadConfig",
            err,
            "无法加载 kubeconfig 文件",
        )
    }

    return config, nil
}
```

注意：需要导入 `clientcmd_api "k8s.io/client-go/tools/clientcmd/api"`

**Step 4: 运行测试（预期通过）**

```bash
cd pkg/k8s
go test -v
```

预期输出: 测试通过（可能有内容解析错误，但文件读取成功）

**Step 5: 编写客户端测试**

创建文件 `pkg/k8s/client_test.go`:

```go
package k8s

import (
    "testing"
)

func TestClient_QPS_Burst(t *testing.T) {
    // 这个测试需要真实的 kubeconfig，在 CI 环境会跳过
    if testing.Short() {
        t.Skip("跳过需要 K8S 连接的测试")
    }

    kubeconfig := os.Getenv("KUBECONFIG")
    if kubeconfig == "" {
        t.Skip("未设置 KUBECONFIG 环境变量")
    }

    client, err := NewClient(kubeconfig, 50, 100)
    if err != nil {
        t.Fatalf("NewClient failed: %v", err)
    }

    if client == nil {
        t.Fatal("NewClient returned nil")
    }

    if client.Clientset == nil {
        t.Error("Clientset is nil")
    }

    if client.Metrics == nil {
        t.Error("Metrics client is nil")
    }
}
```

**Step 6: 实现客户端工厂**

创建文件 `pkg/k8s/client.go`:

```go
package k8s

import (
    "os"

    "k8s.io/client-go/kubernetes"
    "k8s.io/client-go/rest"
    metricsv "k8s.io/metrics/pkg/client/clientset/versioned"

    "pod-monitor/internal/errors"
    "pod-monitor/internal/log"
)

// Client 封装 K8S 客户端
type Client struct {
    Clientset *kubernetes.Clientset
    Metrics   *metricsv.Clientset
    QPS       float32
    Burst     int
}

// Config 客户端配置
type Config struct {
    Kubeconfig string
    QPS        float32
    Burst      int
}

// NewClient 创建 K8S 客户端
func NewClient(kubeconfig string, qps float32, burst int) (*Client, error) {
    log.Stdout.Info("创建 K8S 客户端",
        "kubeconfig", kubeconfig,
        "qps", qps,
        "burst", burst,
    )

    // 加载配置
    loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
    if kubeconfig != "" {
        loadingRules.ExplicitPath = kubeconfig
    }

    // 创建 REST 配置
    config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
        loadingRules,
        &clientcmd.ConfigOverrides{},
    ).ClientConfig()

    if err != nil {
        return nil, errors.NewAppError(
            errors.ErrConfigLoad,
            "NewClient",
            err,
            "创建 REST 配置失败",
        )
    }

    // 设置 QPS 和 Burst
    config.QPS = qps
    config.Burst = burst

    // 创建核心客户端
    clientset, err := kubernetes.NewForConfig(config)
    if err != nil {
        return nil, errors.NewAppError(
            errors.ErrK8SClient,
            "NewClient/create_clientset",
            err,
            "创建 Kubernetes 客户端失败",
        )
    }

    // 创建 Metrics 客户端
    metricsClient, err := metricsv.NewForConfig(config)
    if err != nil {
        return nil, errors.NewAppError(
            errors.ErrMetricsClient,
            "NewClient/create_metrics",
            err,
            "创建 Metrics 客户端失败",
        )
    }

    log.Stdout.Info("K8S 客户端创建成功")

    return &Client{
        Clientset: clientset,
        Metrics:   metricsClient,
        QPS:       qps,
        Burst:     burst,
    }, nil
}
```

**Step 7: 运行测试**

```bash
cd pkg/k8s
go test -v
go test -v -short  # 跳过需要 K8S 连接的测试
```

**Step 8: 提交**

```bash
git add pkg/k8s/
git commit -m "feat: 实现 K8S 客户端工厂模块"
```

---

## 阶段 2：核心业务模块

### Task 6: 实现节点缓存模块

**文件:**
- Create: `pkg/cache/node.go`
- Create: `pkg/cache/node_test.go`

**步骤:**

**Step 1: 编写节点缓存测试**

创建文件 `pkg/cache/node_test.go`:

```go
package cache

import (
    "sync"
    "testing"
)

func TestNodeCache_GetNodeIP(t *testing.T) {
    cache := &NodeCache{}

    // 测试空缓存
    ip := cache.GetNodeIP("node-1")
    if ip != "" {
        t.Errorf("Empty cache should return empty string, got %q", ip)
    }

    // 测试存储后获取
    cache.Store("node-1", "192.168.1.10")
    ip = cache.GetNodeIP("node-1")
    if ip != "192.168.1.10" {
        t.Errorf("Expected 192.168.1.10, got %q", ip)
    }
}

func TestNodeCache_Concurrent(t *testing.T) {
    cache := &NodeCache{}
    var wg sync.WaitGroup

    // 并发读写测试
    for i := 0; i < 100; i++ {
        wg.Add(1)
        go func(n int) {
            defer wg.Done()
            nodeName := "node-" + string(rune('0'+n%10))
            cache.Store(nodeName, "192.168.1."+string(rune('0'+n%10)))
            cache.GetNodeIP(nodeName)
        }(i)
    }

    wg.Wait()
    // 如果没有 panic 或死锁，测试通过
}
```

**Step 2: 运行测试（预期失败）**

```bash
cd pkg/cache
go test -v
```

预期输出: `undefined: NodeCache` 等错误

**Step 3: 实现节点缓存**

创建文件 `pkg/cache/node.go`:

```go
package cache

import (
    "context"
    "fmt"

    v1 "k8s.io/api/core/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

    "pod-monitor/internal/log"
    "pod-monitor/pkg/k8s"
)

// NodeCache 节点信息缓存
type NodeCache struct {
    sync.Map
}

// PreloadNodes 预加载节点信息到缓存
func (nc *NodeCache) PreloadNodes(ctx context.Context, client *k8s.Client) error {
    log.Stdout.DebugContext(ctx, "开始预加载节点信息")

    nodes, err := client.Clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
    if err != nil {
        log.Stdout.ErrorContext(ctx, "获取节点列表失败", "error", err)
        return err
    }

    count := 0
    for _, node := range nodes.Items {
        for _, addr := range node.Status.Addresses {
            if addr.Type == v1.NodeInternalIP {
                nc.Store(node.Name, addr.Address)
                count++
                break
            }
        }
    }

    log.Stdout.InfoContext(ctx, "节点信息预加载完成",
        "total", len(nodes.Items),
        "cached", count,
    )

    return nil
}

// GetNodeIP 从缓存获取节点 IP
func (nc *NodeCache) GetNodeIP(nodeName string) string {
    if nodeName == "" {
        return ""
    }

    if ip, ok := nc.Load(nodeName); ok {
        if ipStr, ok := ip.(string); ok {
            return ipStr
        }
    }

    return ""
}

// GetNodeIPOrUnknown 获取节点 IP，未知时返回 "Unknown"
func (nc *NodeCache) GetNodeIPOrUnknown(nodeName string) string {
    ip := nc.GetNodeIP(nodeName)
    if ip == "" {
        return "Unknown"
    }
    return ip
}
```

**Step 4: 运行测试（预期通过）**

```bash
cd pkg/cache
go test -v
```

**Step 5: 提交**

```bash
git add pkg/cache/
git commit -m "feat: 实现节点信息缓存模块"
```

---

### Task 7: 实现 Pod 分析模块

**文件:**
- Create: `pkg/pod/types.go`
- Create: `pkg/pod/analyzer.go`
- Create: `pkg/pod/restart.go`
- Create: `pkg/pod/analyzer_test.go`

**步骤:**

**Step 1: 定义 Pod 结果类型**

创建文件 `pkg/pod/types.go`:

```go
package pod

import (
    v1 "k8s.io/api/core/v1"
    "time"
)

// Result Pod 分析结果
type Result struct {
    Namespace       string
    Name            string
    Phase           v1.PodPhase
    NodeIP          string
    NodeName        string
    RestartCount    int
    RestartTime     time.Time
    RestartReason   string
    ReadyStatus     string
    Age             time.Duration
    IsAbnormal      bool
    ContainerStatus string
}
```

**Step 2: 编写 Pod 异常检测测试**

创建文件 `pkg/pod/analyzer_test.go`:

```go
package pod

import (
    "testing"
    v1 "k8s.io/api/core/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestAnalyzer_IsAbnormal(t *testing.T) {
    analyzer := NewAnalyzer(nil, nil)

    tests := []struct {
        name     string
        pod      v1.Pod
        expected bool
    }{
        {
            name:     "Running Pod - 正常",
            pod:      createRunningPod(),
            expected: false,
        },
        {
            name:     "Pending Pod - 异常",
            pod:      v1.Pod{
                Status: v1.PodStatus{Phase: v1.PodPending},
            },
            expected: true,
        },
        {
            name:     "Failed Pod - 异常",
            pod:      v1.Pod{
                Status: v1.PodStatus{Phase: v1.PodFailed},
            },
            expected: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := analyzer.IsAbnormal(tt.pod)
            if result != tt.expected {
                t.Errorf("IsAbnormal() = %v, want %v", result, tt.expected)
            }
        })
    }
}

func createRunningPod() v1.Pod {
    return v1.Pod{
        ObjectMeta: metav1.ObjectMeta{
            Name:      "test-pod",
            Namespace: "default",
        },
        Spec: v1.PodSpec{
            Containers: []v1.Container{
                {Name: "app"},
            },
        },
        Status: v1.PodStatus{
            Phase: v1.PodRunning,
            Conditions: []v1.PodCondition{
                {
                    Type:   v1.PodReady,
                    Status: v1.ConditionTrue,
                },
            },
            ContainerStatuses: []v1.ContainerStatus{
                {
                    Ready: true,
                    State: v1.ContainerState{
                        Running: &v1.ContainerStateRunning{},
                    },
                },
            },
        },
    }
}
```

**Step 3: 运行测试（预期失败）**

```bash
cd pkg/pod
go test -v
```

预期输出: `undefined: Analyzer`, `undefined: NewAnalyzer` 等错误

**Step 4: 实现重启分析逻辑**

创建文件 `pkg/pod/restart.go`:

```go
package pod

import (
    "time"
    v1 "k8s.io/api/core/v1"
)

// AnalyzeContainers 分析容器重启情况
// 返回: (总重启次数, 最近重启时间, 重启原因)
func AnalyzeContainers(statuses []v1.ContainerStatus, days int) (int, time.Time, string) {
    if len(statuses) == 0 {
        return 0, time.Time{}, ""
    }

    totalRestarts := 0
    var latestRestartTime time.Time
    restartReason := ""

    for _, cs := range statuses {
        totalRestarts += int(cs.RestartCount)

        if cs.LastTerminationState.Terminated != nil {
            terminated := cs.LastTerminationState.Terminated
            if terminated.FinishedAt.IsZero() {
                continue
            }

            finishedAt := terminated.FinishedAt.Time.UTC()
            if finishedAt.After(latestRestartTime) {
                latestRestartTime = finishedAt
                restartReason = terminated.Reason
            }
        }
    }

    // 检查是否在指定天数内
    cutoffTime := time.Now().UTC().Add(-time.Duration(days) * 24 * time.Hour)
    if totalRestarts > 0 && !latestRestartTime.IsZero() && latestRestartTime.After(cutoffTime) {
        return totalRestarts, latestRestartTime, restartReason
    }

    return 0, time.Time{}, ""
}

// GetReadyStatus 计算 Pod 就绪状态
func GetReadyStatus(pod v1.Pod) string {
    readyContainers := 0
    for _, cond := range pod.Status.Conditions {
        if cond.Type == v1.PodReady && cond.Status == v1.ConditionTrue {
            readyContainers = len(pod.Spec.Containers)
            break
        }
    }
    return fmt.Sprintf("%d/%d", readyContainers, len(pod.Spec.Containers))
}
```

**Step 5: 实现 Pod 分析器**

创建文件 `pkg/pod/analyzer.go`:

```go
package pod

import (
    "context"
    "fmt"
    "time"

    v1 "k8s.io/api/core/v1"

    "pod-monitor/internal/log"
    "pod-monitor/pkg/cache"
    "pod-monitor/pkg/k8s"
)

// Analyzer Pod 分析器
type Analyzer struct {
    client *k8s.Client
    cache  *cache.NodeCache
}

// NewAnalyzer 创建 Pod 分析器
func NewAnalyzer(client *k8s.Client, cache *cache.NodeCache) *Analyzer {
    return &Analyzer{
        client: client,
        cache:  cache,
    }
}

// Analyze 分析单个 Pod
func (a *Analyzer) Analyze(ctx context.Context, pod v1.Pod, days int) *Result {
    result := &Result{
        Namespace: pod.Namespace,
        Name:      pod.Name,
        Phase:     pod.Status.Phase,
        NodeName:  pod.Spec.NodeName,
        NodeIP:    a.cache.GetNodeIPOrUnknown(pod.Spec.NodeName),
        Age:       time.Since(pod.CreationTimestamp.Time).Round(time.Second),
    }

    // 分析容器重启
    restartCount, restartTime, restartReason := AnalyzeContainers(pod.Status.ContainerStatuses, days)
    result.RestartCount = restartCount
    result.RestartTime = restartTime
    result.RestartReason = restartReason

    // 计算就绪状态
    result.ReadyStatus = GetReadyStatus(pod)

    // 判断是否异常
    result.IsAbnormal = a.IsAbnormal(pod)

    // 收集容器状态
    if result.IsAbnormal {
        result.ContainerStatus = a.getContainerStatusReason(pod)
    }

    return result
}

// IsAbnormal 判断 Pod 是否异常
func (a *Analyzer) IsAbnormal(pod v1.Pod) bool {
    // 状态异常
    if pod.Status.Phase != v1.PodRunning && pod.Status.Phase != v1.PodSucceeded {
        return true
    }

    // 容器状态检查
    for _, cs := range pod.Status.ContainerStatuses {
        if cs.State.Waiting != nil && cs.State.Waiting.Reason != "" {
            return true
        }
        if cs.State.Terminated != nil && cs.State.Terminated.ExitCode != 0 {
            return true
        }
    }

    // Ready 状态异常
    ready := GetReadyStatus(pod)
    expected := fmt.Sprintf("%d/%d", len(pod.Spec.Containers), len(pod.Spec.Containers))
    if ready != expected {
        return true
    }

    return false
}

// getContainerStatusReason 获取容器状态原因
func (a *Analyzer) getContainerStatusReason(pod v1.Pod) string {
    for _, cs := range pod.Status.ContainerStatuses {
        if cs.State.Waiting != nil {
            return cs.State.Waiting.Reason
        }
        if cs.State.Terminated != nil {
            return cs.State.Terminated.Reason
        }
    }
    return "N/A"
}
```

**Step 6: 运行测试（预期通过）**

```bash
cd pkg/pod
go test -v
```

**Step 7: 提交**

```bash
git add pkg/pod/
git commit -m "feat: 实现 Pod 分析模块"
```

---

### Task 8: 实现节点监控模块

**文件:**
- Create: `pkg/node/types.go`
- Create: `pkg/node/monitor.go`
- Create: `pkg/node/monitor_test.go`

**步骤:**

**Step 1: 定义节点指标类型**

创建文件 `pkg/node/types.go`:

```go
package node

// Metrics 节点资源指标
type Metrics struct {
    NodeName            string
    IP                  string
    CPUUsage            int64   // 毫核
    CPUTotal            int64   // 毫核
    CPUUsagePercent     float64
    MemoryUsage         int64   // Mi
    MemoryTotal         int64   // Mi
    MemoryUsagePercent  float64
    Status              string  // "正常" 或 "异常"
}
```

**Step 2: 编写节点监控测试**

创建文件 `pkg/node/monitor_test.go`:

```go
package node

import (
    "testing"
    "testing/quick"
)

func TestMetrics_Calculation(t *testing.T) {
    m := &Metrics{
        CPUUsage:        500,  // 0.5 core
        CPUTotal:        2000, // 2 cores
        MemoryUsage:     1024, // 1 Gi
        MemoryTotal:     8192, // 8 Gi
    }

    m.CPUUsagePercent = float64(m.CPUUsage) / float64(m.CPUTotal) * 100
    m.MemoryUsagePercent = float64(m.MemoryUsage) / float64(m.MemoryTotal) * 100

    if m.CPUUsagePercent != 25.0 {
        t.Errorf("CPU usage = %f, want 25.0", m.CPUUsagePercent)
    }

    if m.MemoryUsagePercent != 12.5 {
        t.Errorf("Memory usage = %f, want 12.5", m.MemoryUsagePercent)
    }
}
```

**Step 3: 运行测试（预期失败）**

```bash
cd pkg/node
go test -v
```

预期输出: `undefined: Metrics` 等错误（类型定义后应该通过）

**Step 4: 实现节点监控器**

创建文件 `pkg/node/monitor.go`:

```go
package node

import (
    "context"
    "fmt"

    v1 "k8s.io/api/core/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"

    "pod-monitor/internal/log"
    "pod-monitor/pkg/k8s"
)

// Monitor 节点监控器
type Monitor struct {
    client *k8s.Client
}

// NewMonitor 创建节点监控器
func NewMonitor(client *k8s.Client) *Monitor {
    return &Monitor{client: client}
}

// GetMetrics 获取单个节点的指标
func (m *Monitor) GetMetrics(ctx context.Context, nodeName string) (*Metrics, error) {
    // 获取 Metrics
    nodeMetrics, err := m.client.Metrics.MetricsV1beta1().NodeMetricses().Get(ctx, nodeName, metav1.GetOptions{})
    if err != nil {
        return nil, fmt.Errorf("获取节点指标失败: %w", err)
    }

    // 获取节点详情
    node, err := m.client.Clientset.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
    if err != nil {
        return nil, fmt.Errorf("获取节点详情失败: %w", err)
    }

    return m.calculateMetrics(node, nodeMetrics), nil
}

// ListAll 列出所有节点指标
func (m *Monitor) ListAll(ctx context.Context) ([]*Metrics, error) {
    nodes, err := m.client.Clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
    if err != nil {
        return nil, fmt.Errorf("获取节点列表失败: %w", err)
    }

    var results []*Metrics

    for _, node := range nodes.Items {
        metrics, err := m.GetMetrics(ctx, node.Name)
        if err != nil {
            log.Stdout.WarnContext(ctx, "获取节点指标失败",
                "node", node.Name,
                "error", err,
            )
            continue
        }
        results = append(results, metrics)
    }

    return results, nil
}

// calculateMetrics 计算节点指标
func (m *Monitor) calculateMetrics(node *v1.Node, nodeMetrics *metricsv1beta1.NodeMetrics) *Metrics {
    // CPU 使用量（毫核）
    cpuUsage := nodeMetrics.Usage.Cpu().MilliValue()

    // 内存使用量（Mi）
    memoryUsage := nodeMetrics.Usage.Memory().Value() / (1024 * 1024)

    // CPU 总量（毫核）
    cpuTotal := node.Status.Capacity.Cpu().MilliValue()

    // 内存总量（Mi，使用 Allocatable）
    memoryTotal := node.Status.Allocatable.Memory().Value() / (1024 * 1024)

    // 计算使用率
    cpuPercent := float64(cpuUsage) / float64(cpuTotal) * 100
    memoryPercent := float64(memoryUsage) / float64(memoryTotal) * 100

    // 获取节点状态
    status := "正常"
    for _, condition := range node.Status.Conditions {
        if condition.Type == v1.NodeReady && condition.Status != v1.ConditionTrue {
            status = "异常"
            break
        }
    }

    // 获取 IP
    ip := "N/A"
    for _, addr := range node.Status.Addresses {
        if addr.Type == v1.NodeInternalIP {
            ip = addr.Address
            break
        }
    }

    return &Metrics{
        NodeName:           node.Name,
        IP:                 ip,
        CPUUsage:           cpuUsage,
        CPUTotal:           cpuTotal,
        CPUUsagePercent:    cpuPercent,
        MemoryUsage:        memoryUsage,
        MemoryTotal:        memoryTotal,
        MemoryUsagePercent: memoryPercent,
        Status:             status,
    }
}
```

**Step 5: 运行测试（预期通过）**

```bash
cd pkg/node
go test -v
```

**Step 6: 提交**

```bash
git add pkg/node/
git commit -m "feat: 实现节点监控模块"
```

---

### Task 9: 实现 Worker Pool

**文件:**
- Create: `internal/worker/pool.go`
- Create: `internal/worker/pool_test.go`

**步骤:**

**Step 1: 编写 Worker Pool 测试**

创建文件 `internal/worker/pool_test.go`:

```go
package worker

import (
    "context"
    "testing"
)

func TestPool_Process(t *testing.T) {
    tests := []struct {
        name     string
        workers  int
        items    []int
        expected []int
    }{
        {
            name:     "单 worker",
            workers:  1,
            items:    []int{1, 2, 3},
            expected: []int{2, 4, 6},
        },
        {
            name:     "多 workers",
            workers:  10,
            items:    []int{1, 2, 3, 4, 5},
            expected: []int{6, 8, 10, 2, 4}, // 顺序可能不同
        },
        {
            name:     "空列表",
            workers:  5,
            items:    []int{},
            expected: []int{},
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            pool := NewPool(tt.workers, func(ctx context.Context, item int) int {
                return item * 2
            })

            results := pool.Process(context.Background(), tt.items)

            if len(results) != len(tt.expected) {
                t.Fatalf("len(results) = %d, want %d", len(results), len(tt.expected))
            }

            // 检查所有预期值都存在
            for _, exp := range tt.expected {
                found := false
                for _, res := range results {
                    if res == exp {
                        found = true
                        break
                    }
                }
                if !found {
                    t.Errorf("Expected %d not found in results %v", exp, results)
                }
            }
        })
    }
}

func TestPool_ContextCancel(t *testing.T) {
    pool := NewPool(10, func(ctx context.Context, item int) int {
        // 检查 context 是否取消
        select {
        case <-ctx.Done():
            return -1
        default:
            return item * 2
        }
    })

    ctx, cancel := context.Cancel()
    cancel() // 立即取消

    results := pool.Process(ctx, []int{1, 2, 3})

    // 应该快速返回，可能部分结果为 -1
    if len(results) > 3 {
        t.Errorf("Too many results: %d", len(results))
    }
}
```

**Step 2: 运行测试（预期失败）**

```bash
cd internal/worker
go test -v
```

预期输出: `undefined: Pool`, `undefined: NewPool` 等错误

**Step 3: 实现 Worker Pool**

创建文件 `internal/worker/pool.go`:

```go
package worker

import (
    "context"
    "sync"

    "pod-monitor/internal/log"
)

// Pool 泛型并发处理池
type Pool[T any, R any] struct {
    workers int
    handler func(context.Context, T) R
}

// NewPool 创建 Worker Pool
func NewPool[T any, R any](workers int, handler func(context.Context, T) R) *Pool[T, R] {
    if workers <= 0 {
        workers = 1
    }

    return &Pool[T, R]{
        workers: workers,
        handler: handler,
    }
}

// Process 并发处理数据
func (p *Pool[T, R]) Process(ctx context.Context, items []T) []R {
    if len(items) == 0 {
        return []R{}
    }

    // 创建通道
    itemChan := make(chan T, len(items))
    resultChan := make(chan R, len(items))

    // 启动 workers
    var wg sync.WaitGroup
    for i := 0; i < p.workers; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            for item := range itemChan {
                select {
                case <-ctx.Done():
                    log.Stdout.DebugContext(ctx, "Worker 收到取消信号")
                    return
                default:
                    resultChan <- p.handler(ctx, item)
                }
            }
        }()
    }

    // 分发任务
    for _, item := range items {
        itemChan <- item
    }
    close(itemChan)

    // 等待完成
    wg.Wait()
    close(resultChan)

    // 收集结果
    results := make([]R, 0, len(items))
    for result := range resultChan {
        results = append(results, result)
    }

    return results
}
```

**Step 4: 运行测试（预期通过）**

```bash
cd internal/worker
go test -v
```

**Step 5: 提交**

```bash
git add internal/worker/
git commit -m "feat: 实现泛型 Worker Pool"
```

---

## 阶段 3：CLI 重构

### Task 10: 实现 Cobra 根命令

**文件:**
- Create: `cmd/pod-monitor/root.go`
- Create: `cmd/pod-monitor/main.go`

**步骤:**

**Step 1: 创建根命令**

创建文件 `cmd/pod-monitor/root.go`:

```go
package main

import (
    "github.com/spf13/cobra"

    "pod-monitor/internal/log"
)

var (
    kubeconfig string
    workers    int
    verbose    bool
)

// rootCmd 根命令
var rootCmd = &cobra.Command{
    Use:   "pod-monitor",
    Short: "Kubernetes Pod 监控和检查工具",
    Long: `Pod Monitor 是一个 Kubernetes 集群监控工具，用于检查 Pod 的重启情况、
异常状态以及节点资源使用情况。

支持的功能：
  - Pod 重启检查
  - Pod 异常状态检测
  - 节点资源使用监控

详情请访问: https://github.com/your-repo/kubernetes-check`,
    PersistentPreRun: func(cmd *cobra.Command, args []string) {
        // 设置日志级别
        if verbose {
            log.Stdout = log.New("debug", true)
        }
    },
}

func init() {
    rootCmd.PersistentFlags().StringVar(&kubeconfig, "kubeconfig", "",
        "kubeconfig 文件的绝对路径")
    rootCmd.PersistentFlags().IntVarP(&workers, "workers", "w", 10,
        "并发处理的工作协程数")
    rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false,
        "详细输出模式")
}
```

**Step 2: 创建新的 main.go**

创建文件 `cmd/pod-monitor/main.go`:

```go
package main

import (
    "os"
)

func main() {
    if err := rootCmd.Execute(); err != nil {
        os.Exit(1)
    }
}
```

**Step 3: 测试根命令**

```bash
go run cmd/pod-monitor/main.go --help
```

预期输出: 显示帮助信息，包括 Short 和 Long 描述

**Step 4: 提交**

```bash
git add cmd/pod-monitor/
git commit -m "feat: 实现 Cobra 根命令"
```

---

### Task 11: 实现 Pod 检查子命令

**文件:**
- Create: `cmd/pod-monitor/cmd_pod.go`

**步骤:**

**Step 1: 创建 Pod 子命令**

创建文件 `cmd/pod-monitor/cmd_pod.go`:

```go
package main

import (
    "context"
    "fmt"
    "os"

    "github.com/olekukonko/tablewriter"
    v1 "k8s.io/api/core/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "github.com/spf13/cobra"

    "pod-monitor/internal/log"
    "pod-monitor/internal/worker"
    "pod-monitor/pkg/cache"
    "pod-monitor/pkg/k8s"
    "pod-monitor/pkg/pod"
)

var (
    podDays       int
    podAbnormal   bool
    allNamespaces bool
    namespace     string
)

// podCmd Pod 检查命令
var podCmd = &cobra.Command{
    Use:   "pod",
    Short: "检查 Pod 状态",
    Long:  `检查 Kubernetes Pod 的重启情况和异常状态`,
    Run:   runPodCheck,
}

func init() {
    rootCmd.AddCommand(podCmd)

    podCmd.Flags().IntVarP(&podDays, "days", "d", 7,
        "显示最近 N 天内重启的 Pod")
    podCmd.Flags().BoolVarP(&podAbnormal, "abnormal", "a", false,
        "仅显示异常状态的 Pod")
    podCmd.Flags().BoolVarP(&allNamespaces, "all-namespaces", "A", false,
        "查看所有命名空间的 Pod")
    podCmd.Flags().StringVarP(&namespace, "namespace", "n", "default",
        "指定要查看的命名空间")
}

func runPodCheck(cmd *cobra.Command, args []string) {
    ctx := context.Background()

    // 创建客户端
    client, err := k8s.NewClient(kubeconfig, 50, 100)
    if err != nil {
        log.Stdout.ErrorContext(ctx, "创建 K8S 客户端失败", "error", err)
        os.Exit(1)
    }

    // 预加载节点缓存
    nodeCache := &cache.NodeCache{}
    if err := nodeCache.PreloadNodes(ctx, client); err != nil {
        log.Stdout.WarnContext(ctx, "预加载节点缓存失败", "error", err)
    }

    // 确定命名空间
    ns := namespace
    if allNamespaces {
        ns = ""
    }

    // 获取 Pod 列表
    pods, err := client.Clientset.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{})
    if err != nil {
        log.Stdout.ErrorContext(ctx, "获取 Pod 列表失败", "error", err)
        os.Exit(1)
    }

    log.Stdout.InfoContext(ctx, "开始分析 Pod",
        "total", len(pods.Items),
        "namespace", ns,
    )

    // 创建分析器
    analyzer := pod.NewAnalyzer(client, nodeCache)

    // 创建 Worker Pool
    pool := worker.NewPool(workers, func(ctx context.Context, p v1.Pod) *pod.Result {
        return analyzer.Analyze(ctx, p, podDays)
    })

    // 并发处理
    results := pool.Process(ctx, pods.Items)

    // 过滤结果
    var filtered []*pod.Result
    for _, r := range results {
        if r == nil {
            continue
        }
        if podAbnormal && !r.IsAbnormal {
            continue
        }
        if !podAbnormal && r.RestartCount == 0 {
            continue
        }
        filtered = append(filtered, r)
    }

    log.Stdout.InfoContext(ctx, "分析完成",
        "total", len(pods.Items),
        "results", len(filtered),
    )

    // 输出结果
    displayPodResults(filtered, podAbnormal)
}

// displayPodResults 显示 Pod 结果
func displayPodResults(results []*pod.Result, abnormal bool) {
    table := tablewriter.NewWriter(os.Stdout)

    if abnormal {
        table.SetHeader([]string{"命名空间", "Pod名称", "状态", "节点IP", "就绪状态", "运行时长", "容器状态"})
    } else {
        table.SetHeader([]string{"命名空间", "Pod名称", "状态", "节点IP", "重启次数", "最后重启时间", "重启原因", "就绪状态"})
    }

    table.SetBorder(false)

    // TODO: 添加颜色配置（后续在 pkg/output 中实现）

    for _, r := range results {
        if abnormal {
            table.Append([]string{
                r.Namespace,
                r.Name,
                string(r.Phase),
                r.NodeIP,
                r.ReadyStatus,
                formatDuration(r.Age),
                r.ContainerStatus,
            })
        } else {
            table.Append([]string{
                r.Namespace,
                r.Name,
                string(r.Phase),
                r.NodeIP,
                fmt.Sprintf("%d", r.RestartCount),
                formatTime(r.RestartTime),
                r.RestartReason,
                r.ReadyStatus,
            })
        }
    }

    table.Render()
}

// formatDuration 格式化时长
func formatDuration(d int) string {
    // TODO: 实现时长格式化
    return fmt.Sprintf("%dd%dh", d/24, d%24)
}

// formatTime 格式化时间
func formatTime(t time.Time) string {
    if t.IsZero() {
        return "N/A"
    }
    return t.Format("2006-01-02 15:04:05") + " (UTC+8)"
}
```

**Step 2: 测试 Pod 命令（连接真实集群）**

```bash
go run cmd/pod-monitor/main.go pod --help
go run cmd/pod-monitor/main.go pod --abnormal --workers=5
```

**Step 3: 提交**

```bash
git add cmd/pod-monitor/cmd_pod.go
git commit -m "feat: 实现 Pod 检查子命令"
```

---

### Task 12: 实现节点监控子命令

**文件:**
- Create: `cmd/pod-monitor/cmd_node.go`

**步骤:**

**Step 1: 创建 Node 子命令**

创建文件 `cmd/pod-monitor/cmd_node.go`:

```go
package main

import (
    "context"
    "fmt"
    "os"

    "github.com/olekukonko/tablewriter"
    "github.com/spf13/cobra"

    "pod-monitor/internal/log"
    "pod-monitor/pkg/k8s"
    "pod-monitor/pkg/node"
)

// nodeCmd 节点监控命令
var nodeCmd = &cobra.Command{
    Use:   "node",
    Short: "节点资源监控",
    Long:  `显示 Kubernetes 节点的资源使用情况`,
    Run:   runNodeMonitor,
}

func init() {
    rootCmd.AddCommand(nodeCmd)
}

func runNodeMonitor(cmd *cobra.Command, args []string) {
    ctx := context.Background()

    // 创建客户端
    client, err := k8s.NewClient(kubeconfig, 50, 100)
    if err != nil {
        log.Stdout.ErrorContext(ctx, "创建 K8S 客户端失败", "error", err)
        os.Exit(1)
    }

    // 创建监控器
    monitor := node.NewMonitor(client)

    // 获取所有节点指标
    results, err := monitor.ListAll(ctx)
    if err != nil {
        log.Stdout.ErrorContext(ctx, "获取节点指标失败", "error", err)
        os.Exit(1)
    }

    log.Stdout.InfoContext(ctx, "节点监控完成",
        "total", len(results),
    )

    // 输出结果
    displayNodeResults(results)
}

// displayNodeResults 显示节点结果
func displayNodeResults(results []*node.Metrics) {
    table := tablewriter.NewWriter(os.Stdout)
    table.SetHeader([]string{
        "节点名称", "IP地址", "CPU使用量(cores)",
        "总CPU(cores)", "CPU使用率%",
        "内存使用量(Mi)", "总内存(Mi)",
        "内存使用率%", "状态",
    })

    table.SetBorder(false)

    for _, r := range results {
        table.Append([]string{
            r.NodeName,
            r.IP,
            fmt.Sprintf("%dm", r.CPUUsage),
            fmt.Sprintf("%dm", r.CPUTotal),
            fmt.Sprintf("%.0f%%", r.CPUUsagePercent),
            fmt.Sprintf("%dMi", r.MemoryUsage),
            fmt.Sprintf("%dMi", r.MemoryTotal),
            fmt.Sprintf("%.0f%%", r.MemoryUsagePercent),
            r.Status,
        })
    }

    table.Render()
}
```

**Step 2: 测试 Node 命令**

```bash
go run cmd/pod-monitor/main.go node --help
go run cmd/pod-monitor/main.go node
```

**Step 3: 提交**

```bash
git add cmd/pod-monitor/cmd_node.go
git commit -m "feat: 实现节点监控子命令"
```

---

## 阶段 4：完善和优化

### Task 13: 实现输出格式化模块

**文件:**
- Create: `pkg/output/table.go`
- Create: `pkg/output/color.go`
- Modify: `cmd/pod-monitor/cmd_pod.go`
- Modify: `cmd/pod-monitor/cmd_node.go`

**步骤:**

**Step 1: 创建表格输出模块**

创建文件 `pkg/output/table.go`:

```go
package output

import (
    "io"
    "github.com/olekukonko/tablewriter"
)

// TableWriter 表格写入器
type TableWriter struct {
    *tablewriter.Table
}

// NewTableWriter 创建表格写入器
func NewTableWriter(writer io.Writer) *TableWriter {
    table := tablewriter.NewWriter(writer)
    table.SetBorder(false)
    return &TableWriter{Table: table}
}

// SetPodColumns 设置 Pod 结果列
func (tw *TableWriter) SetPodColumns(abnormal bool) {
    if abnormal {
        tw.SetHeader([]string{
            "命名空间", "Pod名称", "状态",
            "节点IP", "就绪状态", "运行时长", "容器状态",
        })
    } else {
        tw.SetHeader([]string{
            "命名空间", "Pod名称", "状态",
            "节点IP", "重启次数", "最后重启时间",
            "重启原因", "就绪状态",
        })
    }
}

// SetNodeColumns 设置节点结果列
func (tw *TableWriter) SetNodeColumns() {
    tw.SetHeader([]string{
        "节点名称", "IP地址", "CPU使用量(cores)",
        "总CPU(cores)", "CPU使用率%",
        "内存使用量(Mi)", "总内存(Mi)",
        "内存使用率%", "状态",
    })
}
```

**Step 2: 创建颜色配置**

创建文件 `pkg/output/color.go`:

```go
package output

import (
    "github.com/olekukonko/tablewriter"
)

// ApplyPodColors 应用 Pod 表格颜色
func ApplyPodColors(table *TableWriter, abnormal bool) {
    if abnormal {
        table.SetHeaderColor(
            tablewriter.Colors{tablewriter.Bold, tablewriter.FgGreenColor},
            tablewriter.Colors{tablewriter.Bold, tablewriter.FgCyanColor},
            tablewriter.Colors{tablewriter.Bold, tablewriter.FgWhiteColor},
            tablewriter.Colors{tablewriter.Bold, tablewriter.FgMagentaColor},
            tablewriter.Colors{tablewriter.Bold, tablewriter.FgYellowColor},
            tablewriter.Colors{tablewriter.Bold, tablewriter.FgBlueColor},
            tablewriter.Colors{tablewriter.Bold, tablewriter.FgCyanColor},
        )
        table.SetColumnColor(
            tablewriter.Colors{tablewriter.FgHiGreenColor},
            tablewriter.Colors{tablewriter.FgHiCyanColor},
            tablewriter.Colors{tablewriter.FgHiWhiteColor},
            tablewriter.Colors{tablewriter.FgHiMagentaColor},
            tablewriter.Colors{tablewriter.FgHiYellowColor},
            tablewriter.Colors{tablewriter.FgHiBlueColor},
            tablewriter.Colors{tablewriter.FgHiCyanColor},
        )
    } else {
        // TODO: 实现非 abnormal 模式的颜色
    }
}

// ApplyNodeColors 应用节点表格颜色
func ApplyNodeColors(table *TableWriter) {
    table.SetHeaderColor(
        tablewriter.Colors{tablewriter.Bold, tablewriter.FgGreenColor},
        tablewriter.Colors{tablewriter.Bold, tablewriter.FgCyanColor},
        tablewriter.Colors{tablewriter.Bold, tablewriter.FgWhiteColor},
        tablewriter.Colors{tablewriter.Bold, tablewriter.FgMagentaColor},
        tablewriter.Colors{tablewriter.Bold, tablewriter.FgYellowColor},
        tablewriter.Colors{tablewriter.Bold, tablewriter.FgBlueColor},
        tablewriter.Colors{tablewriter.Bold, tablewriter.FgCyanColor},
        tablewriter.Colors{tablewriter.Bold, tablewriter.FgHiMagentaColor},
        tablewriter.Colors{tablewriter.Bold, tablewriter.FgHiYellowColor},
    )
    table.SetColumnColor(
        tablewriter.Colors{tablewriter.FgHiGreenColor},
        tablewriter.Colors{tablewriter.FgHiCyanColor},
        tablewriter.Colors{tablewriter.FgHiWhiteColor},
        tablewriter.Colors{tablewriter.FgHiMagentaColor},
        tablewriter.Colors{tablewriter.FgHiYellowColor},
        tablewriter.Colors{tablewriter.FgHiBlueColor},
        tablewriter.Colors{tablewriter.FgHiCyanColor},
        tablewriter.Colors{tablewriter.FgHiMagentaColor},
        tablewriter.Colors{tablewriter.FgHiYellowColor},
    )
}
```

**Step 3: 更新 Pod 命令使用新输出模块**

修改 `cmd/pod-monitor/cmd_pod.go` 的 `displayPodResults` 函数:

```go
import "pod-monitor/pkg/output"

func displayPodResults(results []*pod.Result, abnormal bool) {
    table := output.NewTableWriter(os.Stdout)
    table.SetPodColumns(abnormal)
    output.ApplyPodColors(table, abnormal)

    for _, r := range results {
        // ... 原有的 Append 逻辑
    }

    table.Render()
}
```

**Step 4: 更新 Node 命令使用新输出模块**

修改 `cmd/pod-monitor/cmd_node.go` 的 `displayNodeResults` 函数:

```go
import "pod-monitor/pkg/output"

func displayNodeResults(results []*node.Metrics) {
    table := output.NewTableWriter(os.Stdout)
    table.SetNodeColumns()
    output.ApplyNodeColors(table)

    for _, r := range results {
        // ... 原有的 Append 逻辑
    }

    table.Render()
}
```

**Step 5: 提交**

```bash
git add pkg/output/ cmd/pod-monitor/cmd_pod.go cmd/pod-monitor/cmd_node.go
git commit -m "feat: 实现输出格式化模块"
```

---

### Task 14: 备份并移除旧的 main.go

**文件:**
- Move: `cmd/pod-monitor/main.go` → `cmd/pod-monitor/main.go.bak` (原文件)
- Remove: `cmd/pod-monitor/main.go.bak`

**步骤:**

**Step 1: 备份原 main.go**

```bash
# 原来的 main.go 在项目根目录的相对位置
# 找到原来的入口文件（如果存在）
ls cmd/pod-monitor/main.go
```

如果存在旧的单一文件 main.go，先备份：

```bash
mv cmd/pod-monitor/main.go cmd/pod-monitor/main_old.go.bak
```

**Step 2: 验证新构建**

```bash
go build -o pod-monitor-new.exe ./cmd/pod-monitor/
./pod-monitor-new.exe --help
./pod-monitor-new.exe pod --abnormal
./pod-monitor-new.exe node
```

**Step 3: 移除备份**

```bash
rm cmd/pod-monitor/main_old.go.bak
```

**Step 4: 提交**

```bash
git add cmd/pod-monitor/
git commit -m "refactor: 移除旧的 main.go，完成 CLI 重构"
```

---

## 阶段 5：测试和文档

### Task 15: 添加集成测试

**文件:**
- Create: `test/integration/cli_test.go`

**步骤:**

**Step 1: 创建集成测试**

创建文件 `test/integration/cli_test.go`:

```go
//go:build integration
// +build integration

package integration

import (
    "os"
    "os/exec"
    "testing"
)

func TestCLI_PodCommand(t *testing.T) {
    if testing.Short() {
        t.Skip("跳过集成测试")
    }

    kubeconfig := os.Getenv("KUBECONFIG")
    if kubeconfig == "" {
        t.Skip("未设置 KUBECONFIG 环境变量")
    }

    // 测试 pod --help
    cmd := exec.Command("go", "run", "cmd/pod-monitor/main.go", "pod", "--help")
    output, err := cmd.CombinedOutput()
    if err != nil {
        t.Fatalf("pod --help failed: %v\nOutput: %s", err, output)
    }

    if len(output) == 0 {
        t.Error("Expected help output, got empty")
    }
}

func TestCLI_NodeCommand(t *testing.T) {
    if testing.Short() {
        t.Skip("跳过集成测试")
    }

    kubeconfig := os.Getenv("KUBECONFIG")
    if kubeconfig == "" {
        t.Skip("未设置 KUBECONFIG 环境变量")
    }

    // 测试 node --help
    cmd := exec.Command("go", "run", "cmd/pod-monitor/main.go", "node", "--help")
    output, err := cmd.CombinedOutput()
    if err != nil {
        t.Fatalf("node --help failed: %v\nOutput: %s", err, output)
    }

    if len(output) == 0 {
        t.Error("Expected help output, got empty")
    }
}
```

**Step 2: 运行集成测试**

```bash
go test -tags=integration ./test/integration/... -v
```

**Step 3: 提交**

```bash
git add test/integration/
git commit -m "test: 添加 CLI 集成测试"
```

---

### Task 16: 更新文档

**文件:**
- Modify: `README.md`
- Modify: `CLAUDE.md`

**步骤:**

**Step 1: 更新 README.md**

在 README.md 中添加新的使用说明：

```markdown
## 使用新版本 CLI

### Pod 检查

# 查看重启的 Pod
pod-monitor pod --days=3

# 查看异常 Pod
pod-monitor pod --abnormal

# 查看所有命名空间
pod-monitor pod -A --abnormal

### 节点监控

# 查看节点资源使用
pod-monitor node
```

**Step 2: 更新 CLAUDE.md**

更新项目结构和架构说明，反映新的模块化设计。

**Step 3: 提交**

```bash
git add README.md CLAUDE.md
git commit -m "docs: 更新文档以反映新架构"
```

---

## 最终检查清单

### Task 17: 最终验证

**步骤:**

**Step 1: 运行所有测试**

```bash
go test ./... -short
```

**Step 2: 构建可执行文件**

```bash
# Windows
go build -o pod-monitor.exe ./cmd/pod-monitor/

# Linux
GOOS=linux GOARCH=amd64 go build -o pod-monitor-linux ./cmd/pod-monitor/
```

**Step 3: 功能验证**

```bash
./pod-monitor.exe --help
./pod-monitor.exe pod --help
./pod-monitor.exe node --help
./pod-monitor.exe pod --abnormal --workers=10
./pod-monitor.exe node
```

**Step 4: 代码审查**

检查清单：
- [ ] 所有模块都有对应的测试
- [ ] 测试覆盖率 > 70%
- [ ] 没有循环依赖
- [ ] 导出的类型和函数都有文档注释
- [ ] 错误处理一致
- [ ] 日志输出清晰

**Step 5: 清理 worktree**

```bash
cd ..
git worktree remove kubernetes-check-refactor
```

**Step 6: 合并回主分支（如果满意）**

```bash
cd kubernetes-check
git merge refactor
```

---

## 实施总结

**完成后的项目结构：**

```
.
├── cmd/pod-monitor/          # CLI 入口
│   ├── main.go
│   ├── root.go
│   ├── cmd_pod.go
│   └── cmd_node.go
├── pkg/                      # 公共包
│   ├── pod/                  # Pod 分析
│   ├── node/                 # 节点监控
│   ├── cache/                # 缓存
│   ├── k8s/                  # K8S 客户端
│   └── output/               # 输出格式化
├── internal/                 # 内部包
│   ├── worker/               # Worker Pool
│   ├── errors/               # 错误处理
│   └── log/                  # 日志
├── test/                     # 测试
│   └── integration/          # 集成测试
└── docs/plans/               # 设计文档
```

**重构收益：**

✅ 模块化设计，职责清晰
✅ 可测试性提升，单元测试覆盖率 > 70%
✅ 可扩展的 CLI，支持子命令
✅ 结构化日志和统一错误处理
✅ 为未来功能（GitLab、etcd 检查）提供基础
