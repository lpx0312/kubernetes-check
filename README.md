# k8s-patrol

Kubernetes 集群巡检 CLI 工具。检查节点资源、Pod 状态、PVC 存储使用情况，并可一键生成 HTML 巡检报告。

## 功能

- **节点资源检查** — CPU/内存使用率、Ready 状态（需 metrics-server）
- **Pod 重启检查** — 近 N 天内重启过的 Pod，含重启次数/时间/原因
- **Pod 异常检查** — 当前状态异常的 Pod（Pending/CrashLoopBackOff/未就绪）
- **存储检查** — PVC 绑定状态、使用量、孤儿 PV（Released/Failed）
- **HTML 巡检报告** — 一次命令生成带健康度摘要的全量报告（自包含单文件）

## 快速开始

### 安装

从源码构建（需 Go 1.22+）：

```bash
# 国内环境建议先设代理
go env -w GOPROXY=https://goproxy.cn,direct

go build -o k8s-patrol ./cmd/k8s-patrol/
```

交叉编译（生成多平台二进制）：

```bash
# Linux amd64
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o k8s-patrol-linux-amd64 ./cmd/k8s-patrol/

# Linux arm64
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags="-s -w" -o k8s-patrol-linux-arm64 ./cmd/k8s-patrol/
```

### 基本用法

```bash
# 查看所有命令
k8s-patrol --help

# 检查节点资源
k8s-patrol node --kubeconfig=/root/.kube/config

# 检查近 7 天重启的 Pod（所有命名空间）
k8s-patrol restart -d 7 -A --kubeconfig=/root/.kube/config

# 检查当前异常的 Pod
k8s-patrol abnormal -A --kubeconfig=/root/.kube/config

# 检查 PVC 存储使用情况
k8s-patrol storage -A --kubeconfig=/root/.kube/config

# 生成全量 HTML 巡检报告
k8s-patrol report -o weekly.html --kubeconfig=/root/.kube/config
```

## 命令参考

```
k8s-patrol --help
Kubernetes 集群巡检工具

可用命令:
  node        检查节点资源使用情况
  restart     检查近期重启的 Pod
  abnormal    检查当前异常的 Pod
  storage     检查 PVC 绑定状态与使用量
  report      生成全量 HTML 巡检报告
  version     显示版本信息
  completion  生成 shell 自动补全脚本

全局选项（所有子命令继承）:
  -k, --kubeconfig string   kubeconfig 文件路径
  -A, --all-namespaces      查询所有命名空间
  -n, --namespace string    指定命名空间（默认 default）
  -w, --workers int         并发处理数（默认 10）
```

各子命令专属参数：

| 命令 | 参数 | 说明 |
|------|------|------|
| `restart` | `-d, --days int` | 回溯天数（默认 7） |
| `report` | `-o, --output string` | 输出文件路径（默认 `report-YYYYMMDD.html`） |

使用 `k8s-patrol <命令> --help` 查看各命令详细说明。

## Shell 自动补全（completion）

k8s-patrol 支持 bash/zsh/fish/powershell 的 Tab 自动补全，输入命令时按 Tab 可自动补全子命令和参数。

> **注意**：自动补全需要三个条件：① 系统装了 `bash-completion` 包；② 补全脚本放到正确目录；③ 当前 shell 已加载。只运行 `k8s-patrol completion bash` 只是把脚本打印到屏幕，并不会自动生效。

### Bash

```bash
# 1. 安装 bash-completion 基础包（openEuler/CentOS 默认不装）
yum install -y bash-completion      # 或 apt install -y bash-completion

# 2. 安装 k8s-patrol 补全脚本
k8s-patrol completion bash > /etc/bash_completion.d/k8s-patrol

# 3. 让当前终端立即生效（或重新 SSH 登录）
source /etc/profile.d/bash_completion.sh

# 4. 测试：输入 k8s-patrol 后按 Tab
k8s-patrol <Tab>
# 弹出: abnormal completion help node report restart storage version
```

### Zsh

```bash
# 1. 启用 compinit（若未启用，加到 ~/.zshrc）
echo "autoload -U compinit; compinit" >> ~/.zshrc

# 2. 安装补全脚本到 fpath
k8s-patrol completion zsh > "${fpath[1]}/_k8s-patrol"

# 3. 重新加载
source ~/.zshrc
```

### Fish / PowerShell

```bash
# Fish
k8s-patrol completion fish > ~/.config/fish/completions/k8s-patrol.fish

# PowerShell（Windows）
k8s-patrol completion powershell | Out-String | Invoke-Expression
```

## 架构

三层架构，职责清晰：

```
cmd/k8s-patrol/        Cobra 子命令编排（CLI 入口）
internal/collector/    数据采集层（节点/Pod/存储），产出 report.Report
internal/report/       巡检结果数据结构（Collector 与 Renderer 的契约）
internal/renderer/     渲染层（终端表格 + HTML 报告）
```

- **Collector** 通过 worker pool 并发采集，节点 IP 用 sync.Map 缓存避免重复查询
- **Renderer** 与采集逻辑解耦，终端表格和 HTML 报告各自独立，互不影响

详细设计见 [CLAUDE.md](CLAUDE.md)。

## 依赖

- Go 1.22+
- Kubernetes 1.31.x（client-go v0.31.14）
- 集群需安装 [metrics-server](docs/metrics-server-deploy-guide.md)（节点资源检查依赖）
- kubelet `/stats/summary` API 可达（PVC 使用量检查依赖）

## 构建

```bash
go build -o k8s-patrol ./cmd/k8s-patrol/

# 生产构建（裁剪调试信息，体积小约 30%）
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o k8s-patrol ./cmd/k8s-patrol/
```

## 开发

```bash
# 运行测试
go test ./internal/...

# 下载依赖
go mod tidy
```
