package renderer

// reportHTMLTemplate 是巡检报告的 HTML 模板。
// 样式全部内联到 <style>，生成的报告单文件自包含，浏览器/邮件打开即用，无外部依赖。
//
// 模板字段说明（对应 renderer.htmlReportData）：
//   .Cluster .GeneratedAt          - 报告头
//   .Summary.*                     - 摘要卡片
//   .HasNodes .NodeRows            - 节点章节
//   .HasAbnormal .AbnormalRows     - 异常 Pod 章节
//   .HasRestart .RestartRows       - 重启 Pod 章节
//   .Notes                         - 收集提示
//
// 阈值告警：CPU/内存超阈值的单元格由 Go 侧预计算 class（cell-warn/cell-severe），
// 模板只负责输出 class，不在模板里做数值比较（保持模板简单）。
const reportHTMLTemplate = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>k8s-patrol 集群巡检报告 - {{.GeneratedAt}}</title>
<style>
  * { box-sizing: border-box; margin: 0; padding: 0; }
  body {
    font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", "PingFang SC",
                 "Microsoft YaHei", sans-serif;
    background: #f5f7fa; color: #2c3e50; line-height: 1.6;
    padding: 24px; font-size: 14px;
  }
  .container { max-width: 1100px; margin: 0 auto; }

  /* 报告头 */
  .header {
    background: linear-gradient(135deg, #326ce5 0%, #1e4db7 100%);
    color: #fff; padding: 28px 32px; border-radius: 8px 8px 0 0;
  }
  .header h1 { font-size: 24px; font-weight: 600; margin-bottom: 6px; }
  .header .meta { font-size: 13px; opacity: 0.9; }

  /* 健康度徽章 */
  .health-badge {
    display: inline-block; padding: 4px 14px; border-radius: 14px;
    font-weight: 600; font-size: 13px; margin-top: 10px;
  }
  .health-健康 { background: #d4edda; color: #155724; }
  .health-警告 { background: #fff3cd; color: #856404; }
  .health-严重 { background: #f8d7da; color: #721c24; }

  /* 摘要卡片 */
  .summary { display: flex; gap: 16px; padding: 24px 32px; background: #fff; }
  .stat-card {
    flex: 1; padding: 18px; border-radius: 6px; text-align: center;
    border: 1px solid #e8ecef;
  }
  .stat-card .num { font-size: 28px; font-weight: 700; }
  .stat-card .label { font-size: 12px; color: #7f8c8d; margin-top: 4px; }
  .num-ok { color: #27ae60; }
  .num-warn { color: #e67e22; }
  .num-severe { color: #e74c3c; }
  .num-neutral { color: #3498db; }

  /* 章节 */
  .section { background: #fff; padding: 8px 32px 20px; margin-top: 2px; }
  .section:last-child { border-radius: 0 0 8px 8px; }
  .section h2 {
    font-size: 16px; font-weight: 600; padding: 16px 0 12px;
    border-bottom: 1px solid #eee; margin-bottom: 14px; color: #2c3e50;
  }
  .count { color: #95a5a6; font-weight: 400; font-size: 14px; }

  /* 表格 */
  table { width: 100%; border-collapse: collapse; font-size: 13px; }
  th {
    background: #f8f9fb; text-align: left; padding: 10px 12px;
    color: #5a6c7d; font-weight: 600; border-bottom: 2px solid #e8ecef;
    white-space: nowrap;
  }
  td { padding: 9px 12px; border-bottom: 1px solid #f0f2f5; }
  tr:nth-child(even) td { background: #fafbfc; }
  tr:hover td { background: #f0f6ff; }

  /* 状态/阈值告警单元格 */
  .cell-ok { color: #27ae60; font-weight: 600; }
  .cell-warn { color: #e67e22; font-weight: 600; }
  .cell-severe { color: #e74c3c; font-weight: 600; }

  /* 空数据提示 */
  .empty {
    text-align: center; padding: 24px; color: #27ae60;
    background: #f0fdf4; border-radius: 4px;
  }

  /* 提示信息 */
  .notes { padding: 16px 32px; background: #fff3cd; color: #856404; font-size: 13px; }
  .notes div { margin: 2px 0; }
  .footer { text-align: center; padding: 20px; color: #95a5a6; font-size: 12px; }
</style>
</head>
<body>
<div class="container">

  <!-- 报告头 -->
  <div class="header">
    <h1>k8s-patrol 集群巡检报告</h1>
    <div class="meta">
      生成时间: {{.GeneratedAt}} (UTC+8) ｜ 集群: {{.Cluster}}<br>
      健康度: <span class="health-badge health-{{.Summary.OverallHealth}}">{{.Summary.OverallHealth}}</span>
    </div>
  </div>

  <!-- 摘要卡片 -->
  <div class="summary">
    <div class="stat-card">
      <div class="num num-neutral">{{.Summary.TotalNodes}}</div>
      <div class="label">节点总数</div>
    </div>
    <div class="stat-card">
      <div class="num {{if gt .Summary.AbnormalNodes 0}}num-severe{{else}}num-ok{{end}}">{{.Summary.AbnormalNodes}}</div>
      <div class="label">异常节点</div>
    </div>
    <div class="stat-card">
      <div class="num num-neutral">{{.Summary.TotalPods}}</div>
      <div class="label">Pod 总数</div>
    </div>
    <div class="stat-card">
      <div class="num {{if gt .Summary.AbnormalPods 0}}num-warn{{else}}num-ok{{end}}">{{.Summary.AbnormalPods}}</div>
      <div class="label">异常 Pod</div>
    </div>
    <div class="stat-card">
      <div class="num {{if gt .Summary.RestartedPods 0}}num-warn{{else}}num-ok{{end}}">{{.Summary.RestartedPods}}</div>
      <div class="label">近期重启 Pod</div>
    </div>
    <div class="stat-card">
      <div class="num {{if gt .Summary.AbnormalPVC 0}}num-warn{{else}}num-ok{{end}}">{{.Summary.AbnormalPVC}}</div>
      <div class="label">异常 PVC</div>
    </div>
    <div class="stat-card">
      <div class="num {{if gt .Summary.OrphanPV 0}}num-warn{{else}}num-ok{{end}}">{{.Summary.OrphanPV}}</div>
      <div class="label">孤儿 PV</div>
    </div>
  </div>

  {{range .Notes}}<div class="notes">{{.}}</div>{{end}}

  <!-- 节点资源 -->
  {{if .HasNodes}}
  <div class="section">
    <h2>🖥️ 节点资源 <span class="count">({{len .NodeRows}})</span></h2>
    <table>
      <tr>
        <th>节点名称</th><th>IP 地址</th>
        <th>CPU 使用</th><th>CPU 使用率</th>
        <th>内存使用</th><th>内存使用率</th><th>状态</th>
      </tr>
      {{range .NodeRows}}
      <tr>
        <td>{{.NodeName}}</td>
        <td>{{.IP}}</td>
        <td>{{.CPUDisplay}} / {{.TotalCPUDisplay}}</td>
        <td class="{{.CPUClass}}">{{printf "%.1f%%" .CPUUsage}}</td>
        <td>{{.MemDisplay}} / {{.TotalMemDisplay}}</td>
        <td class="{{.MemClass}}">{{printf "%.1f%%" .MemoryUsage}}</td>
        <td class="{{if eq .Status "正常"}}cell-ok{{else}}cell-severe{{end}}">{{.Status}}</td>
      </tr>
      {{end}}
    </table>
  </div>
  {{end}}

  <!-- 存储与卷：PVC 表 + 孤儿 PV 表 -->
  <div class="section">
    <h2>💾 存储与卷 <span class="count">(PVC {{len .StorageRows}} / 孤儿PV {{len .OrphanPVRows}})</span></h2>
    {{if .HasStorage}}
    <table>
      <tr>
        <th>命名空间</th><th>PVC 名称</th><th>PVC 状态</th>
        <th>StorageClass</th><th>请求量</th><th>已用</th>
        <th>使用率</th><th>PV 名称</th><th>PV 状态</th>
      </tr>
      {{range .StorageRows}}
      <tr>
        <td>{{.Namespace}}</td>
        <td>{{.Name}}</td>
        <td class="{{.PhaseClass}}">{{.Phase}}</td>
        <td>{{.StorageClass}}</td>
        <td>{{.RequestedDisplay}}</td>
        <td>{{.UsedDisplay}}</td>
        <td class="{{.UsageClass}}">{{.UsageDisplay}}</td>
        <td>{{.PVName}}</td>
        <td class="{{if eq .PVPhase "Bound"}}cell-ok{{else if eq .PVPhase "Available"}}cell-ok{{else if eq .PVPhase ""}}-{{else}}cell-severe{{end}}">{{if eq .PVPhase ""}}-{{else}}{{.PVPhase}}{{end}}</td>
      </tr>
      {{end}}
    </table>
    {{else}}
    <div class="empty">✓ 无持久化存储使用</div>
    {{end}}

    {{if .HasOrphanPV}}
    <h2 style="margin-top:20px">🗑️ 孤儿 PV <span class="count">({{len .OrphanPVRows}})</span></h2>
    <table>
      <tr>
        <th>PV 名称</th><th>状态</th><th>回收策略</th>
        <th>访问模式</th><th>绑定节点</th><th>容量</th>
      </tr>
      {{range .OrphanPVRows}}
      <tr>
        <td>{{.Name}}</td>
        <td class="{{.PhaseClass}}">{{.Phase}}</td>
        <td>{{.ReclaimPolicy}}</td>
        <td>{{.AccessModes}}</td>
        <td>{{.BoundNode}}</td>
        <td>{{.CapacityDisplay}}</td>
      </tr>
      {{end}}
    </table>
    {{end}}
  </div>

  <!-- 异常 Pod -->
  <div class="section">
    <h2>⚠️ 异常 Pod <span class="count">({{len .AbnormalRows}})</span></h2>
    {{if .HasAbnormal}}
    <table>
      <tr>
        <th>命名空间</th><th>Pod 名称</th><th>状态</th>
        <th>节点 IP</th><th>就绪</th><th>运行时长</th><th>容器状态</th>
      </tr>
      {{range .AbnormalRows}}
      <tr>
        <td>{{.Namespace}}</td>
        <td>{{.Name}}</td>
        <td class="cell-warn">{{.Phase}}</td>
        <td>{{.NodeIP}}</td>
        <td>{{.Ready}}</td>
        <td>{{.AgeDisplay}}</td>
        <td class="cell-severe">{{.ContainerStatus}}</td>
      </tr>
      {{end}}
    </table>
    {{else}}
    <div class="empty">✓ 所有 Pod 运行正常，无异常</div>
    {{end}}
  </div>

  <!-- 重启 Pod -->
  <div class="section">
    <h2>🔄 近期重启 Pod <span class="count">({{len .RestartRows}})</span></h2>
    {{if .HasRestart}}
    <table>
      <tr>
        <th>命名空间</th><th>Pod 名称</th><th>状态</th>
        <th>节点 IP</th><th>重启次数</th><th>最后重启时间</th><th>重启原因</th><th>就绪</th>
      </tr>
      {{range .RestartRows}}
      <tr>
        <td>{{.Namespace}}</td>
        <td>{{.Name}}</td>
        <td>{{.Phase}}</td>
        <td>{{.NodeIP}}</td>
        <td class="cell-warn">{{.RestartCount}}</td>
        <td>{{.RestartTimeDisplay}}</td>
        <td class="cell-severe">{{.Reason}}</td>
        <td>{{.Ready}}</td>
      </tr>
      {{end}}
    </table>
    {{else}}
    <div class="empty">✓ 无近期重启的 Pod</div>
    {{end}}
  </div>

  <div class="footer">由 k8s-patrol 自动生成 · {{.GeneratedAt}}</div>
</div>
</body>
</html>`
