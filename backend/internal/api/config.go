package api

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"waymark/internal/configcenter"
	"waymark/internal/store"
)

// maxImportSize 导入 zip 压缩包大小上限。
const maxImportSize = 20 << 20 // 20MB

type configRequest struct {
	Namespace string `json:"namespace"`
	GroupName string `json:"groupName"`
	DataId    string `json:"dataId"`
	Content   string `json:"content"`
	Type      string `json:"type"`
}

// handleListConfigs 分页查询配置列表。
func (s *Server) handleListConfigs(c *gin.Context) {
	namespace := resolveNamespace(c.Query("namespace"))
	if !s.authorizeNamespace(c, namespace, false) {
		return
	}
	pageNum := parsePositiveInt(c.Query("pageNum"), 1)
	pageSize := parsePositiveInt(c.Query("pageSize"), 20)
	if pageSize > 200 {
		pageSize = 200
	}
	list, total, err := s.configCenter.List(namespace, c.Query("groupName"), c.Query("dataId"), pageNum, pageSize)
	if err != nil {
		fail(c, "查询配置列表失败: "+err.Error())
		return
	}
	ok(c, gin.H{
		"list":     list,
		"total":    total,
		"pageNum":  pageNum,
		"pageSize": pageSize,
	})
}

// handleGetConfig 查询配置详情。
func (s *Server) handleGetConfig(c *gin.Context) {
	namespace := resolveNamespace(c.Query("namespace"))
	group := resolveGroup(c.Query("groupName"))
	dataId := strings.TrimSpace(c.Query("dataId"))
	if dataId == "" {
		fail(c, "dataId 不能为空")
		return
	}
	if !s.authorizeNamespace(c, namespace, false) {
		return
	}
	item, err := s.configCenter.Get(namespace, group, dataId)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			fail(c, "配置不存在: "+dataId)
			return
		}
		fail(c, "查询配置失败: "+err.Error())
		return
	}
	ok(c, item)
}

// handlePublishConfig 发布或更新配置。
func (s *Server) handlePublishConfig(c *gin.Context) {
	var req configRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, "参数错误: "+err.Error())
		return
	}
	namespace := resolveNamespace(req.Namespace)
	dataId := strings.TrimSpace(req.DataId)
	if dataId == "" {
		fail(c, "dataId 不能为空")
		return
	}
	if !s.authorizeNamespace(c, namespace, true) {
		return
	}
	if !s.ensureNamespaceExists(c, namespace) {
		return
	}
	if err := s.configCenter.Publish(namespace, resolveGroup(req.GroupName), dataId, req.Content, req.Type); err != nil {
		fail(c, "发布配置失败: "+err.Error())
		return
	}
	auditf(c, "发布配置 namespace=%s groupName=%s dataId=%s type=%s", namespace, resolveGroup(req.GroupName), dataId, req.Type)
	okNull(c)
}

// handleDeleteConfig 删除配置。
func (s *Server) handleDeleteConfig(c *gin.Context) {
	namespace := resolveNamespace(c.Query("namespace"))
	group := resolveGroup(c.Query("groupName"))
	dataId := strings.TrimSpace(c.Query("dataId"))
	if dataId == "" {
		fail(c, "dataId 不能为空")
		return
	}
	if !s.authorizeNamespace(c, namespace, true) {
		return
	}
	if err := s.configCenter.Delete(namespace, group, dataId); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			fail(c, "配置不存在: "+dataId)
			return
		}
		fail(c, "删除配置失败: "+err.Error())
		return
	}
	auditf(c, "删除配置 namespace=%s groupName=%s dataId=%s", namespace, group, dataId)
	okNull(c)
}

// handleConfigHistory 分页查询配置历史版本。
func (s *Server) handleConfigHistory(c *gin.Context) {
	namespace := resolveNamespace(c.Query("namespace"))
	group := resolveGroup(c.Query("groupName"))
	dataId := strings.TrimSpace(c.Query("dataId"))
	if dataId == "" {
		fail(c, "dataId 不能为空")
		return
	}
	if !s.authorizeNamespace(c, namespace, false) {
		return
	}
	pageNum := parsePositiveInt(c.Query("pageNum"), 1)
	pageSize := parsePositiveInt(c.Query("pageSize"), 20)
	if pageSize > 200 {
		pageSize = 200
	}
	list, total, err := s.configCenter.History(namespace, group, dataId, pageNum, pageSize)
	if err != nil {
		fail(c, "查询配置历史失败: "+err.Error())
		return
	}
	ok(c, gin.H{
		"list":     list,
		"total":    total,
		"pageNum":  pageNum,
		"pageSize": pageSize,
	})
}

type configRestoreRequest struct {
	Namespace string `json:"namespace"`
	GroupName string `json:"groupName"`
	DataId    string `json:"dataId"`
	HistoryId int64  `json:"historyId"`
}

// handleRestoreConfig 将指定历史版本还原为当前配置。
func (s *Server) handleRestoreConfig(c *gin.Context) {
	var req configRestoreRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, "参数错误: "+err.Error())
		return
	}
	namespace := resolveNamespace(req.Namespace)
	dataId := strings.TrimSpace(req.DataId)
	if dataId == "" {
		fail(c, "dataId 不能为空")
		return
	}
	if req.HistoryId <= 0 {
		fail(c, "historyId 不能为空")
		return
	}
	if !s.authorizeNamespace(c, namespace, true) {
		return
	}
	if err := s.configCenter.Restore(namespace, resolveGroup(req.GroupName), dataId, req.HistoryId); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			fail(c, "历史版本不存在")
			return
		}
		fail(c, "还原配置失败: "+err.Error())
		return
	}
	auditf(c, "还原配置 namespace=%s groupName=%s dataId=%s historyId=%d", namespace, resolveGroup(req.GroupName), dataId, req.HistoryId)
	okNull(c)
}

type configExportItem struct {
	GroupName string `json:"groupName"`
	DataId    string `json:"dataId"`
}

type configExportRequest struct {
	Namespace string             `json:"namespace"`
	GroupName string             `json:"groupName"`
	DataId    string             `json:"dataId"`
	Items     []configExportItem `json:"items"`
}

// handleExportConfigs 导出配置为 zip 压缩包（支持多选导出）。
func (s *Server) handleExportConfigs(c *gin.Context) {
	var req configExportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, "参数错误: "+err.Error())
		return
	}
	namespace := resolveNamespace(req.Namespace)
	if !s.authorizeNamespace(c, namespace, false) {
		return
	}

	items := make([]configcenter.ExportItem, 0, len(req.Items))
	for _, it := range req.Items {
		dataId := strings.TrimSpace(it.DataId)
		if dataId == "" {
			continue
		}
		items = append(items, configcenter.ExportItem{GroupName: resolveGroup(it.GroupName), DataId: dataId})
	}

	// 未指定具体配置时，按当前筛选条件导出（空分组表示全部）。
	data, err := s.configCenter.Export(namespace, strings.TrimSpace(req.GroupName), strings.TrimSpace(req.DataId), items)
	if err != nil {
		if errors.Is(err, configcenter.ErrNothingToExport) {
			fail(c, "没有可导出的配置")
			return
		}
		fail(c, "导出配置失败: "+err.Error())
		return
	}

	filename := fmt.Sprintf("%s-configs.zip", namespace)
	auditf(c, "导出配置 namespace=%s groupName=%s dataId=%s items=%d", namespace, strings.TrimSpace(req.GroupName), strings.TrimSpace(req.DataId), len(items))
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="configs.zip"; filename*=UTF-8''%s`, url.QueryEscape(filename)))
	c.Data(http.StatusOK, "application/zip", data)
}

// handleImportConfigs 从 zip 压缩包导入配置，可指定目标分组。
func (s *Server) handleImportConfigs(c *gin.Context) {
	namespace := resolveNamespace(c.PostForm("namespace"))
	group := strings.TrimSpace(c.PostForm("groupName"))
	if !s.authorizeNamespace(c, namespace, true) {
		return
	}
	if !s.ensureNamespaceExists(c, namespace) {
		return
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		fail(c, "请选择要导入的 zip 文件")
		return
	}
	if fileHeader.Size <= 0 {
		fail(c, "导入文件为空")
		return
	}
	if fileHeader.Size > maxImportSize {
		fail(c, "导入文件过大，最大支持 20MB")
		return
	}

	f, err := fileHeader.Open()
	if err != nil {
		fail(c, "读取上传文件失败: "+err.Error())
		return
	}
	defer func() { _ = f.Close() }()
	data, err := io.ReadAll(f)
	if err != nil {
		fail(c, "读取上传文件失败: "+err.Error())
		return
	}

	result, err := s.configCenter.Import(namespace, group, data)
	if err != nil {
		fail(c, "导入配置失败: "+err.Error())
		return
	}
	auditf(c, "导入配置 namespace=%s groupName=%s imported=%d failed=%d", namespace, group, result.Imported, len(result.Failed))
	ok(c, result)
}

func parsePositiveInt(raw string, def int) int {
	if raw == "" {
		return def
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v <= 0 {
		return def
	}
	return v
}
