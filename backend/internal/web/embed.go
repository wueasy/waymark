package web

import "embed"

// Dist 内嵌前端构建产物（frontend/dist 打包脚本会同步到 dist 目录）。
//
//go:embed all:dist
var Dist embed.FS
