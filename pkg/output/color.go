package output

import "github.com/olekukonko/tablewriter"

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
