```
# 运行
go env -w GOPROXY=https://mirrors.aliyun.com/goproxy/,direct
go mod download
go run cmd/pod-monitor/main.go --days=3 --kubeconfig=C:\Users\lipanx\.kube\config 

#### 去清理并重新下载依赖
# 清理旧缓存
go clean -modcache
# 强制校验依赖
go mod verify
# 下载依赖
go mod download

# 设置多级代理（按顺序尝试）
go env -w GOPROXY=https://goproxy.cn,https://goproxy.io,https://mirrors.aliyun.com/goproxy/,direct
# 关闭模块校验（临时方案）
go env -w GOSUMDB=off
# 强制刷新依赖
go mod tidy -v


### 构建

# 设置Windows编译环境变量
$env:GOOS = "windows"
$env:GOARCH = "amd64"
go build -o pod-monitor.exe ./cmd/pod-monitor/

# 设置linux编译环境变量
$env:GOOS = "linux"
$env:GOARCH = "amd64"
go build -o pod-monitor-linux ./cmd/pod-monitor/
```

# 2025.2.20 明日任务
# 完善了解 ， 为啥能加速
 go run cmd/pod-monitor/main.go --workers=20 --kubeconfig=$env:USERPROFILE\.kube\config
优化点说明: 
1. 节点信息预加载：通过preloadNodeCache函数提前加载所有节点信息到内存缓存
2. 并行处理：使用worker pool模式并发处理Pod，通过--workers参数控制并发数
3. 缓存机制：使用sync.Map缓存节点IP信息，减少API调用次数
4. 流水线处理：分离数据获取、处理和展示阶段

# 优化0： 新增-h 参数 可以查看帮助
# 优化1: 加排序参数 -n 可以按照 namespace 等排序
    go run cmd/pod-monitor/main.go --workers=20 --kubeconfig=$env:USERPROFILE\.kube\config --sort=namespace

# 优化2: 新增检查Pod状态的列表./
# 优化3: 新增检查Gitlab的接口
         新增检查etcd的检查
         新增ingress的检查
# 优化4:   对接prometheus, 判断主机的状态，直接装有node-export的机器的就行了， 就不用专门写agent来获取状态了，单独虚拟机还是需要agent的
# 优化5 ： 完成etcd的检查，mysql， redis的检查， 备份文件的检查
# 优化6 ： 完成平台组件：前端：后端：数据库：中间件，等检查 

```
# 获取重启的pod 默认是3天内的
./pod-monitor-linux -restart

# 获取异常的pod
./pod-monitor-linux -abnormal
```

# 优化7 ： 新增集群检查
# K8S集群的相关检查
1. **集群健康状态检查**
   - API Server可用性
   - 控制平面组件状态（etcd/scheduler/controller-manager）
   - 工作节点Ready状态统计
   
2. **资源配额监控**
   - 命名空间资源配额使用率
   - 节点资源分配/剩余量预警
   - PersistentVolume剩余空间监控

3. **网络检查**
   - CoreDNS服务可用性
   - 节点间网络延迟检测
   - Service/Ingress端点连通性

4. **存储检查**
   - PV/PVC绑定状态
   - StorageClass可用性
   - 卷健康状态

5. **安全审计**
   - RBAC配置风险检测
   - 过期的证书检查
   - 不安全的Pod安全策略

6. **工作负载分析**
   - Deployment副本数偏差
   - StatefulSet序数连续性
   - 僵尸Pod清理建议

7. **扩展指标**
   - HPA配置合理性检查
   - 资源Request/Limit比例分析
   - 垂直扩缩容建议

实现建议：
- 新增`cluster_check.go`模块集中处理集群级检查
- 复用现有的metricsClient获取扩展指标
- 通过`--cluster-check`命令行参数触发专项检查模式
        