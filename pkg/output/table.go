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
		tw.SetHeader([]string{"命名空间", "Pod名称", "状态", "节点IP", "就绪状态", "运行时长", "容器状态"})
	} else {
		tw.SetHeader([]string{"命名空间", "Pod名称", "状态", "节点IP", "重启次数", "最后重启时间", "重启原因", "就绪状态"})
	}
}

// SetNodeColumns 设置节点结果列
func (tw *TableWriter) SetNodeColumns() {
	tw.SetHeader([]string{"节点名称", "IP地址", "CPU使用量(cores)", "总CPU(cores)", "CPU使用率%", "内存使用量(Mi)", "总内存(Mi)", "内存使用率%", "状态"})
}
