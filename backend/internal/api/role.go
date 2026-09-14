package api

import (
	"errors"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"

	"waymark/internal/store"
)

var roleCodePattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{2,32}$`)

type roleRequest struct {
	Code        string   `json:"code"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Permission  string   `json:"permission"`
	Namespaces  []string `json:"namespaces"`
}

// handleListRoles 角色列表。
func (s *Server) handleListRoles(c *gin.Context) {
	list, err := s.store.ListRoles()
	if err != nil {
		fail(c, "查询角色列表失败: "+err.Error())
		return
	}
	ok(c, list)
}

// handleCreateRole 新建角色。
func (s *Server) handleCreateRole(c *gin.Context) {
	var req roleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, "参数错误: "+err.Error())
		return
	}
	req.Code = strings.TrimSpace(req.Code)
	req.Name = strings.TrimSpace(req.Name)
	if !roleCodePattern.MatchString(req.Code) {
		fail(c, "角色标识仅支持 2-32 位字母、数字、下划线和中划线")
		return
	}
	if req.Code == store.RoleCodeAdmin {
		fail(c, "角色标识 admin 为内置保留标识")
		return
	}
	if req.Name == "" {
		fail(c, "角色名称不能为空")
		return
	}
	if !isValidPermission(req.Permission) {
		fail(c, "权限等级不合法，可选 read / write")
		return
	}
	if _, err := s.store.GetRoleByCode(req.Code); err == nil {
		fail(c, "角色标识已存在: "+req.Code)
		return
	}
	if err := s.store.CreateRole(&store.Role{
		Code:        req.Code,
		Name:        req.Name,
		Description: req.Description,
		Permission:  req.Permission,
		Namespaces:  req.Namespaces,
	}); err != nil {
		fail(c, "创建角色失败: "+err.Error())
		return
	}
	auditf(c, "创建角色 code=%s name=%s permission=%s", req.Code, req.Name, req.Permission)
	okNull(c)
}

// handleUpdateRole 更新角色，内置角色不可修改，角色标识不可变更。
func (s *Server) handleUpdateRole(c *gin.Context) {
	id, okId := parseIdParam(c)
	if !okId {
		return
	}
	var req roleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, "参数错误: "+err.Error())
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		fail(c, "角色名称不能为空")
		return
	}
	if !isValidPermission(req.Permission) {
		fail(c, "权限等级不合法，可选 read / write")
		return
	}
	existing, err := s.store.GetRoleById(id)
	if err != nil {
		fail(c, "角色不存在")
		return
	}
	if existing.Builtin == store.BuiltinYes {
		fail(c, "内置角色不允许修改")
		return
	}
	existing.Name = req.Name
	existing.Description = req.Description
	existing.Permission = req.Permission
	existing.Namespaces = req.Namespaces
	if err := s.store.UpdateRole(existing); err != nil {
		fail(c, "更新角色失败: "+err.Error())
		return
	}
	auditf(c, "更新角色 code=%s name=%s permission=%s", existing.Code, existing.Name, existing.Permission)
	okNull(c)
}

// handleDeleteRole 删除角色，内置角色不可删除。
func (s *Server) handleDeleteRole(c *gin.Context) {
	id, okId := parseIdParam(c)
	if !okId {
		return
	}
	existing, err := s.store.GetRoleById(id)
	if err != nil {
		fail(c, "角色不存在")
		return
	}
	if existing.Builtin == store.BuiltinYes {
		fail(c, "内置角色不允许删除")
		return
	}
	if err := s.store.DeleteRole(id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			fail(c, "角色不存在")
			return
		}
		fail(c, "删除角色失败: "+err.Error())
		return
	}
	auditf(c, "删除角色 code=%s name=%s", existing.Code, existing.Name)
	okNull(c)
}

func isValidPermission(permission string) bool {
	return permission == store.PermissionRead || permission == store.PermissionWrite
}
