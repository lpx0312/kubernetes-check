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
# 优化7 ： K8S集群的相关检查 


```
# 获取重启的pod 默认是3天内的
./pod-monitor-linux -restart

# 获取异常的pod
./pod-monitor-linux -abnormal
```

