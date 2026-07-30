# 更新日志

本项目遵循 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/) 规范，
版本号遵循 [语义化版本](https://semver.org/lang/zh-CN/)。

## [Unreleased]

### Added
- （待发布的新功能写在这里）

---

## [v1.0.0] - 2026-07-30

首个正式版本。Kubernetes 集群巡检 CLI，支持节点资源、Pod 状态、PVC 存储检查，并可生成 HTML 巡检报告。

### Added
- **Cobra 子命令架构**：`node`/`restart`/`abnormal`/`storage`/`report`/`version`/`completion`
  - 全局选项 `-k`/`-A`/`-n`/`-w` 长短参数，所有子命令继承
  - `restart` 支持 `-d/--days` 回溯天数，`report` 支持 `-o/--output` 输出路径
- **节点资源检查**：CPU/内存使用率、Ready 状态（需 metrics-server）
- **Pod 重启检查**：近 N 天内重启的 Pod，含重启次数/时间/原因
- **Pod 异常检查**：Pending/CrashLoopBackOff/ImagePullBackOff/未就绪等异常状态
- **存储检查**：
  - PVC 绑定状态（Bound/Pending/Lost）与使用量（kubelet `/stats/summary`）
  - 孤儿 PV（Released/Failed，含 ReclaimPolicy/AccessModes/节点亲和性）
  - 未挂载 PVC 显示"未挂载"，与"卷为空"明确区分
- **HTML 全量巡检报告**：
  - 自包含单文件（CSS 内联），浏览器/邮件直接打开
  - 巡检摘要（健康度/节点/Pod/PVC 统计）+ 4 个检查章节
  - 健康度判定：严重（异常节点/Lost PVC/Failed PV/使用率≥95%）/ 警告 / 健康
- **Shell 自动补全**：bash/zsh/fish/powershell（`k8s-patrol completion <shell>`）
- **三层架构**：Collector（采集）→ Report（数据契约）→ Renderer（终端表格 + HTML）
- **CI/CD**：GitHub Actions（push 触发 CI 检查 + tag 触发 Release 发布）+ Makefile

### Fixed
- client-go 从 v0.17.2 升级到 v0.31.14，匹配 K8s 1.31 集群
- 修复颜色列数不匹配导致的 panic（abnormal 7 列 / restart 8 列）
- 修复多容器 Pod 部分就绪时误报 0/n（`getReadyStatus` 改遍历 ContainerStatuses）
- 修复正常调度中的 Pending Pod 被误报为异常
- 节点监控消除 N+1 查询，CPU/内存统一使用 Allocatable
- metrics-server 缺失时友好降级提示，不再直接崩溃

---

[Unreleased]: https://github.com/lpx0312/kubernetes-check/compare/v1.0.0...HEAD
[v1.0.0]: https://github.com/lpx0312/kubernetes-check/releases/tag/v1.0.0
