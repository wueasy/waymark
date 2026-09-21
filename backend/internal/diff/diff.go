// Package diff 提供配置内容的行级差异计算（基于 LCS），用于发布前的变更预览。
package diff

import "strings"

// 行操作类型。
const (
	OpEqual = "equal" // 未变更
	OpAdd   = "add"   // 新增行
	OpDel   = "del"   // 删除行
)

// maxCells LCS 动态规划的最大单元格数，超出时退化为整段替换，避免占用过多内存。
const maxCells = 4_000_000

// Line 单行差异。OldNo/NewNo 为对应内容中的行号（从 1 开始），不存在的一侧为 0。
type Line struct {
	Op    string `json:"op"`
	OldNo int    `json:"oldNo"`
	NewNo int    `json:"newNo"`
	Text  string `json:"text"`
}

// Stats 差异统计。
type Stats struct {
	Added   int `json:"added"`
	Removed int `json:"removed"`
}

// Result 差异计算结果。内容一致时 Changed 为 false 且 Lines 为空。
type Result struct {
	Changed bool   `json:"changed"`
	Lines   []Line `json:"lines"`
	Stats   Stats  `json:"stats"`
}

// Compare 计算原内容与新内容的行级差异，输出统一 diff 形式（含未变更行）。
func Compare(oldContent, newContent string) Result {
	if oldContent == newContent {
		return Result{Changed: false, Lines: []Line{}, Stats: Stats{}}
	}

	oldLines := splitLines(oldContent)
	newLines := splitLines(newContent)

	result := Result{Lines: make([]Line, 0, len(oldLines)+len(newLines))}
	if len(oldLines)*len(newLines) > maxCells {
		// 内容过大时不做 LCS，直接按整段替换呈现。
		for i, text := range oldLines {
			result.Lines = append(result.Lines, Line{Op: OpDel, OldNo: i + 1, Text: text})
		}
		for i, text := range newLines {
			result.Lines = append(result.Lines, Line{Op: OpAdd, NewNo: i + 1, Text: text})
		}
	} else {
		result.Lines = append(result.Lines, lcsLines(oldLines, newLines)...)
	}

	for _, line := range result.Lines {
		switch line.Op {
		case OpAdd:
			result.Stats.Added++
		case OpDel:
			result.Stats.Removed++
		}
	}
	result.Changed = result.Stats.Added > 0 || result.Stats.Removed > 0
	return result
}

// lcsLines 基于最长公共子序列生成行级差异。
func lcsLines(oldLines, newLines []string) []Line {
	n, m := len(oldLines), len(newLines)
	// dp[i][j] 表示 oldLines[i:] 与 newLines[j:] 的最长公共子序列长度。
	dp := make([][]int, n+1)
	for i := range dp {
		dp[i] = make([]int, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			switch {
			case oldLines[i] == newLines[j]:
				dp[i][j] = dp[i+1][j+1] + 1
			case dp[i+1][j] >= dp[i][j+1]:
				dp[i][j] = dp[i+1][j]
			default:
				dp[i][j] = dp[i][j+1]
			}
		}
	}

	lines := make([]Line, 0, n+m)
	for i, j := 0, 0; i < n || j < m; {
		switch {
		case i < n && j < m && oldLines[i] == newLines[j]:
			lines = append(lines, Line{Op: OpEqual, OldNo: i + 1, NewNo: j + 1, Text: oldLines[i]})
			i++
			j++
		case j < m && (i == n || dp[i+1][j] < dp[i][j+1]):
			lines = append(lines, Line{Op: OpAdd, NewNo: j + 1, Text: newLines[j]})
			j++
		default:
			lines = append(lines, Line{Op: OpDel, OldNo: i + 1, Text: oldLines[i]})
			i++
		}
	}
	return lines
}

// splitLines 按行切分内容，忽略行尾的 \r 以便展示；末尾换行不产生多余空行。
func splitLines(content string) []string {
	if content == "" {
		return nil
	}
	lines := strings.Split(content, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	for i, line := range lines {
		lines[i] = strings.TrimSuffix(line, "\r")
	}
	return lines
}
