package main

import (
    "flag"
    "fmt"
    "log"
    "os"
    "sync"
    "time"
    "github.com/olekukonko/tablewriter"
    v1 "k8s.io/api/core/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/client-go/kubernetes"
    "k8s.io/client-go/tools/clientcmd"
    metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
    metrics "k8s.io/metrics/pkg/client/clientset/versioned"
)

var (
    versionFlag    = flag.Bool("version", false, "显示版本信息")
    kubeconfigFlag = flag.String("kubeconfig", "", "绝对路径到kubeconfig文件")
    daysFlag       = flag.Int("days", 7, "显示最近N天内重启的Pod (默认7天)")
    workersFlag    = flag.Int("workers", 10, "并发处理的工作协程数 (默认10)")
    abnormalFlag   = flag.Bool("abnormal", false, "仅显示异常状态Pod")
    allNamespaces  = flag.Bool("A", false, "查看所有命名空间的Pod")
    namespace      = flag.String("n", "default", "指定要查看的命名空间")
    nodeMetricsFlag = flag.Bool("node-metrics", false, "显示节点资源使用情况")
)

// NodeMetrics 存储节点资源使用情况
type NodeMetrics struct {
    NodeName    string
    CPU         int64
    Memory      int64
    TotalCPU    int64
    TotalMemory int64
    MemoryUsage float64
    CPUUsage    float64
    Raw         *metricsv1beta1.NodeMetrics // 添加原始指标数据
}

const (
    timeFormat = "2006-01-02 15:04:05"
    beijingTZ  = "Asia/Shanghai"
)

func main() {
    // 命令行参数解析
    flag.Parse()
    if *versionFlag {
        fmt.Println("Pod Monitor v1.0 (Kubernetes 1.17.2 compatible)")
        return
    }

    // kubeconfig路径处理
    if *kubeconfigFlag != "" {
        log.Printf("使用自定义kubeconfig: %s", *kubeconfigFlag)
    } else {
        log.Printf("使用默认kubeconfig路径: %s", clientcmd.RecommendedHomeFile)
    }

    // 加载kubeconfig配置
    loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
    if *kubeconfigFlag != "" {
        loadingRules.ExplicitPath = *kubeconfigFlag
    }
    
    // 检查默认配置文件是否存在
    if _, err := os.Stat(loadingRules.GetDefaultFilename()); os.IsNotExist(err) {
        log.Printf("警告: 默认kubeconfig文件不存在于 %s", loadingRules.GetDefaultFilename())
    }
    // 创建客户端配置
    config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
        loadingRules,
        &clientcmd.ConfigOverrides{},
    ).ClientConfig()
    
    // 提升API请求速率限制
    config.QPS = 50   // 默认5
    config.Burst = 100 // 默认10
    
    // 创建Kubernetes客户端
    clientset, err := kubernetes.NewForConfig(config)
    if err != nil {
        log.Fatalf("创建客户端失败: %v", err)
    }

    // 创建Metrics客户端
    metricsClient, err := metrics.NewForConfig(config)
    if err != nil {
        log.Fatalf("创建Metrics客户端失败: %v", err)
    }

    // 如果指定了节点监控标志，则显示节点资源使用情况
    if *nodeMetricsFlag {
        displayNodeMetrics(clientset, metricsClient)
        return
    }

    // 获取Pod列表
    var pods *v1.PodList
    var targetNamespace string

    if *allNamespaces {
        targetNamespace = ""
        log.Printf("正在查看所有命名空间的Pod")
    } else {
        targetNamespace = *namespace
        log.Printf("正在查看命名空间 %s 的Pod", targetNamespace)
    }

    pods, err = clientset.CoreV1().Pods(targetNamespace).List(metav1.ListOptions{})
    if err != nil {
        log.Fatalf("获取Pod列表失败: %v", err)
    }

    // 预加载节点信息缓存（提升查询效率）
    nodeCache := sync.Map{}
    preloadNodeCache(clientset, &nodeCache)

    // 创建工作管道
    podChan := make(chan v1.Pod, len(pods.Items))
    resultChan := make(chan []string, len(pods.Items))

    // 启动工作协程池（并发处理Pod）
    var wg sync.WaitGroup
    for i := 0; i < *workersFlag; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            for pod := range podChan {
                processPod(pod, clientset, &nodeCache, daysFlag, resultChan)
            }
        }()
    }

    // 分发任务
    for _, pod := range pods.Items {
        podChan <- pod
    }
    close(podChan)
    wg.Wait()
    close(resultChan)

    log.Printf("共处理 %d 个Pod，发现 %d 个异常Pod", len(pods.Items), len(resultChan))

    table := tablewriter.NewWriter(os.Stdout)
    if *abnormalFlag {
        table.SetHeader([]string{"命名空间", "Pod名称", "状态", "节点IP", "就绪状态", "运行时长", "容器状态"})
        table.SetColumnAlignment([]int{
            tablewriter.ALIGN_LEFT,  // Namespace
            tablewriter.ALIGN_LEFT,  // PodName
            tablewriter.ALIGN_CENTER,// PodStatus
            tablewriter.ALIGN_LEFT,  // Node
            tablewriter.ALIGN_CENTER,// READY
            tablewriter.ALIGN_RIGHT, // AGE
            tablewriter.ALIGN_LEFT,  // ContainerStatus
        })
    } else {
        table.SetHeader([]string{"命名空间", "Pod名称", "状态", "节点IP", "重启次数", "最后重启时间", "重启原因", "就绪状态"})
        table.SetColumnAlignment([]int{
            tablewriter.ALIGN_LEFT,  // Namespace
            tablewriter.ALIGN_LEFT,  // PodName
            tablewriter.ALIGN_CENTER,// PodStatus
            tablewriter.ALIGN_LEFT,  // Node
            tablewriter.ALIGN_CENTER,// Restart
            tablewriter.ALIGN_LEFT,  // RestartTime
            tablewriter.ALIGN_LEFT,  // RestartReason
            tablewriter.ALIGN_CENTER,// READY
        })
    }

    // 添加表格样式配置
    table.SetBorder(false)
    if *abnormalFlag {
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
            tablewriter.Colors{tablewriter.FgHiGreenColor},     // 节点名称
            tablewriter.Colors{tablewriter.FgHiCyanColor},      // IP地址
            tablewriter.Colors{tablewriter.FgHiWhiteColor},     // CPU使用量
            tablewriter.Colors{tablewriter.FgHiMagentaColor},   // 总CPU
            tablewriter.Colors{tablewriter.FgHiYellowColor},    // CPU使用率%
            tablewriter.Colors{tablewriter.FgHiBlueColor},      // 内存使用量
            tablewriter.Colors{tablewriter.FgHiCyanColor},      // 总内存
            tablewriter.Colors{tablewriter.FgHiMagentaColor},   // 内存使用率%
            tablewriter.Colors{tablewriter.FgHiYellowColor},    // 状态
        )
    } else {
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
            tablewriter.Colors{tablewriter.FgHiMagentaColor},  // 添加逗号
            tablewriter.Colors{tablewriter.FgHiYellowColor},
        )
    }

    // 收集结果
    for row := range resultChan {
        table.Append(row)
    }
    table.Render()
}

func analyzeContainers(statuses []v1.ContainerStatus, days int) (int, time.Time, string) {
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

    cutoffTime := time.Now().UTC().Add(-time.Duration(days)*24*time.Hour)
    if totalRestarts > 0 && !latestRestartTime.IsZero() && latestRestartTime.After(cutoffTime) {
        loc, _ := time.LoadLocation(beijingTZ)
        latestRestartTime = latestRestartTime.In(loc)
        return totalRestarts, latestRestartTime, restartReason
    }
    return 0, time.Time{}, ""
}

func getNodeIP(clientset *kubernetes.Clientset, nodeName string) string {
    if nodeName == "" {
        return "N/A"
    }

    node, err := clientset.CoreV1().Nodes().Get(nodeName, metav1.GetOptions{})
    if err != nil {
        return "Unknown"
    }

    for _, addr := range node.Status.Addresses {
        if addr.Type == v1.NodeInternalIP {
            return addr.Address
        }
    }
    return "N/A"
}

func getReadyStatus(pod v1.Pod) string {
    readyContainers := 0
    for _, cond := range pod.Status.Conditions {
        if cond.Type == v1.PodReady && cond.Status == v1.ConditionTrue {
            readyContainers = len(pod.Spec.Containers)
            break
        }
    }
    return fmt.Sprintf("%d/%d", readyContainers, len(pod.Spec.Containers))
}

// 新增预加载节点缓存函数
func preloadNodeCache(clientset *kubernetes.Clientset, cache *sync.Map) {
    nodes, err := clientset.CoreV1().Nodes().List(metav1.ListOptions{})
    if err != nil {
        log.Printf("预加载节点信息失败: %v", err)
        return
    }

    for _, node := range nodes.Items {
        for _, addr := range node.Status.Addresses {
            if addr.Type == v1.NodeInternalIP {
                cache.Store(node.Name, addr.Address)
                break
            }
        }
    }
}

// 修改异常状态判断函数
func isPodAbnormal(pod v1.Pod) bool {
    // 状态异常判断
    if pod.Status.Phase != v1.PodRunning && 
       pod.Status.Phase != v1.PodSucceeded {
        return true
    }
    
    // 新增容器状态检查
    for _, cs := range pod.Status.ContainerStatuses {
        if cs.State.Waiting != nil && cs.State.Waiting.Reason != "" {
            return true // 容器处于等待状态且有原因
        }
        if cs.State.Terminated != nil && cs.State.Terminated.ExitCode != 0 {
            return true // 容器异常退出 
        }
    }
    
    // Ready状态异常判断
    ready := getReadyStatus(pod)
    if ready != fmt.Sprintf("%d/%d", len(pod.Spec.Containers), len(pod.Spec.Containers)) {
        return true
    }
    
    return false
}

// 修改processPod函数，添加异常状态处理
func processPod(pod v1.Pod, clientset *kubernetes.Clientset, nodeCache *sync.Map, days *int, resultChan chan<- []string) {
    // log.Printf("正在检查Pod: %s/%s 状态: %s", pod.Namespace, pod.Name, pod.Status.Phase)

    if *abnormalFlag {
        if isPodAbnormal(pod) {
            // 新增容器状态原因收集
            containerStatus := "N/A"
            for _, cs := range pod.Status.ContainerStatuses {
                if cs.State.Waiting != nil {
                    containerStatus = cs.State.Waiting.Reason
                    break
                }
                if cs.State.Terminated != nil {
                    containerStatus = cs.State.Terminated.Reason
                    break
                }
            }

            age := time.Since(pod.CreationTimestamp.Time).Round(time.Second)
            
            var nodeIP string
            if cached, ok := nodeCache.Load(pod.Spec.NodeName); ok {
                nodeIP = cached.(string)
            } else {
                nodeIP = "Unknown"
            }

            resultChan <- []string{
                pod.Namespace,
                pod.Name,
                string(pod.Status.Phase),
                nodeIP,
                getReadyStatus(pod),
                fmt.Sprintf("%dd%dh", int(age.Hours()/24), int(age.Hours())%24),
                containerStatus, // 新增容器状态列
            }
        }
        return
    }

    // 原有重启检查逻辑保持不变
    restartCount, restartTime, restartReason := analyzeContainers(pod.Status.ContainerStatuses, *days)
    
    var nodeIP string
    if cached, ok := nodeCache.Load(pod.Spec.NodeName); ok {
        nodeIP = cached.(string)
    } else {
        nodeIP = "Unknown"
    }

    readyStatus := getReadyStatus(pod)

    if restartCount > 0 {
        resultChan <- []string{
            pod.Namespace,
            pod.Name,
            string(pod.Status.Phase),
            nodeIP,
            fmt.Sprintf("%d", restartCount),
            restartTime.Format(timeFormat + " (UTC+8)"),
            restartReason,
            readyStatus,
        }
    }
}

// 获取节点资源使用情况
func getNodeMetrics(nodeName string, metricsClient *metrics.Clientset, clientset *kubernetes.Clientset) (*NodeMetrics, error) {
    metrics, err := metricsClient.MetricsV1beta1().NodeMetricses().Get(nodeName, metav1.GetOptions{})
    if err != nil {
        return nil, err
    }

    // 获取节点详情以获取总资源容量
    node, err := clientset.CoreV1().Nodes().Get(nodeName, metav1.GetOptions{})
    if err != nil {
        return nil, err
    }

    // 获取CPU使用量（单位：毫核）
    cpuUsage := metrics.Usage.Cpu().MilliValue()
    
    // 获取内存使用量（单位：Mi）
    memoryUsage := metrics.Usage.Memory().Value() / (1024 * 1024)
    
    // 获取节点总CPU和内存容量
    totalCPU := node.Status.Capacity.Cpu().MilliValue()
    totalMemory := node.Status.Allocatable.Memory().Value() / (1024 * 1024)  // 使用Allocatable而不是Capacity
    
    // 计算CPU使用率
    cpuUsagePercent := float64(cpuUsage) / float64(totalCPU) * 100
    // 计算内存使用率（使用实际使用量除以可分配内存）
    memoryUsagePercent := float64(memoryUsage) / float64(totalMemory) * 100

    return &NodeMetrics{
        NodeName:    nodeName,
        CPU:         cpuUsage,
        Memory:      memoryUsage,
        TotalCPU:    totalCPU,
        TotalMemory: totalMemory,
        CPUUsage:    cpuUsagePercent,
        MemoryUsage: memoryUsagePercent,
        Raw:         metrics,
    }, nil
}

// 显示节点资源使用情况
func displayNodeMetrics(clientset *kubernetes.Clientset, metricsClient *metrics.Clientset) {
    nodes, err := clientset.CoreV1().Nodes().List(metav1.ListOptions{})
    if err != nil {
        log.Fatalf("获取节点列表失败: %v", err)
    }

    table := tablewriter.NewWriter(os.Stdout)
    table.SetHeader([]string{"节点名称", "IP地址", "CPU使用量(cores)", "总CPU(cores)", "CPU使用率%", "内存使用量(Mi)", "总内存(Mi)", "内存使用率%", "状态"})
    table.SetColumnAlignment([]int{
        tablewriter.ALIGN_LEFT,   // 节点名称
        tablewriter.ALIGN_LEFT,   // IP地址
        tablewriter.ALIGN_RIGHT,  // CPU使用量
        tablewriter.ALIGN_RIGHT,  // CPU总量
        tablewriter.ALIGN_RIGHT,  // CPU使用率
        tablewriter.ALIGN_RIGHT,  // 内存使用量
        tablewriter.ALIGN_RIGHT,  // 内存总量
        tablewriter.ALIGN_RIGHT,  // 内存使用率
        tablewriter.ALIGN_CENTER, // 状态
    })

    // 设置表格样式，与abnormal模式保持一致
    table.SetBorder(false)
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
        tablewriter.Colors{tablewriter.FgHiGreenColor},     // 节点名称
        tablewriter.Colors{tablewriter.FgHiCyanColor},      // IP地址
        tablewriter.Colors{tablewriter.FgHiWhiteColor},     // CPU使用量
        tablewriter.Colors{tablewriter.FgHiMagentaColor},   // 总CPU
        tablewriter.Colors{tablewriter.FgHiYellowColor},    // CPU使用率%
        tablewriter.Colors{tablewriter.FgHiBlueColor},      // 内存使用量
        tablewriter.Colors{tablewriter.FgHiCyanColor},      // 总内存
        tablewriter.Colors{tablewriter.FgHiMagentaColor},   // 内存使用率%
        tablewriter.Colors{tablewriter.FgHiYellowColor},    // 状态
    )

    for _, node := range nodes.Items {
        metrics, err := getNodeMetrics(node.Name, metricsClient, clientset)
        if err != nil {
            log.Printf("获取节点 %s 的资源指标失败: %v", node.Name, err)
            continue
        }

        // 获取节点状态
        status := "正常"
        for _, condition := range node.Status.Conditions {
            if condition.Type == v1.NodeReady && condition.Status != v1.ConditionTrue {
                status = "异常"
                break
            }
        }

        table.Append([]string{
            node.Name,
            getNodeIP(clientset, node.Name),  // 新增IP地址
            fmt.Sprintf("%dm", metrics.CPU),
            fmt.Sprintf("%dm", metrics.TotalCPU),
            fmt.Sprintf("%.0f%%", metrics.CPUUsage),
            fmt.Sprintf("%dMi", metrics.Memory),
            fmt.Sprintf("%dMi", metrics.TotalMemory),
            fmt.Sprintf("%.0f%%", metrics.MemoryUsage),
            status,
        })
    }
    table.Render()
}