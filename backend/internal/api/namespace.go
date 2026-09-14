package api

import (
	"errors"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"

	"waymark/internal/auth"
	"waymark/internal/store"
)

var namespacePattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{2,64}$`)

type namespaceRequest struct {
	Namespace   string `json:"namespace"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// handleListNamespaces 命名空间列表：管理员返回全部，其他用户返回通过角色获权的列表。
func (s *Server) handleListNamespaces(c *gin.Context) {
	p := auth.CurrentPrincipal(c)
	if p == nil {
		unauthorized(c, "未登录")
		return
	}
	var (
		list []store.Namespace
		err  error
	)
	if p.IsAdmin {
		list, err = s.store.ListNamespaces()
	} else {
		list, err = s.store.ListNamespacesByUserRole(p.User.Id)
	}
	if err != nil {
		fail(c, "查询命名空间失败: "+err.Error())
		return
	}
	ok(c, list)
}

// handleCreateNamespace 创建命名空间。
func (s *Server) handleCreateNamespace(c *gin.Context) {
	var req namespaceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, "参数错误: "+err.Error())
		return
	}
	req.Namespace = strings.TrimSpace(req.Namespace)
	if !namespacePattern.MatchString(req.Namespace) {
		fail(c, "命名空间标识仅支持 2-64 位字母、数字、下划线和中划线")
		return
	}
	if _, err := s.store.GetNamespace(req.Namespace); err == nil {
		fail(c, "命名空间已存在: "+req.Namespace)
		return
	}
	if req.Name == "" {
		req.Name = req.Namespace
	}
	if err := s.store.CreateNamespace(&store.Namespace{
		Namespace:   req.Namespace,
		Name:        req.Name,
		Description: req.Description,
	}); err != nil {
		fail(c, "创建命名空间失败: "+err.Error())
		return
	}
	auditf(c, "创建命名空间 namespace=%s name=%s", req.Namespace, req.Name)
	okNull(c)
}

// handleUpdateNamespace 更新命名空间显示信息。
func (s *Server) handleUpdateNamespace(c *gin.Context) {
	name := c.Param("name")
	var req namespaceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, "参数错误: "+err.Error())
		return
	}
	if req.Name == "" {
		req.Name = name
	}
	if err := s.store.UpdateNamespace(&store.Namespace{
		Namespace:   name,
		Name:        req.Name,
		Description: req.Description,
	}); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			fail(c, "命名空间不存在: "+name)
			return
		}
		fail(c, "更新命名空间失败: "+err.Error())
		return
	}
	auditf(c, "更新命名空间 namespace=%s name=%s", name, req.Name)
	okNull(c)
}

// handleDeleteNamespace 删除命名空间。
func (s *Server) handleDeleteNamespace(c *gin.Context) {
	name := c.Param("name")
	if name == store.DefaultNamespace {
		fail(c, "默认命名空间 public 不允许删除")
		return
	}
	if err := s.store.DeleteNamespace(name); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			fail(c, "命名空间不存在: "+name)
			return
		}
		fail(c, "删除命名空间失败: "+err.Error())
		return
	}
	auditf(c, "删除命名空间 namespace=%s", name)
	okNull(c)
}
