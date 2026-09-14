package api

import (
	"errors"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	wlog "github.com/wueasy/wueasy-go-tools/log"

	"waymark/internal/auth"
	"waymark/internal/store"
)

// demoProtectedMsg 演示环境下受保护账号的拒绝提示。
const demoProtectedMsg = "该账号为演示环境受保护账号，禁止此操作"

type initRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Nickname string `json:"nickname"`
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// roleBrief 角色精简信息，用于用户列表与个人资料展示。
type roleBrief struct {
	Id   int64  `json:"id"`
	Code string `json:"code"`
	Name string `json:"name"`
}

// userProfile 当前登录用户的资料、角色与命名空间有效权限。
type userProfile struct {
	Id            int64             `json:"id"`
	Username      string            `json:"username"`
	Nickname      string            `json:"nickname"`
	Status        int               `json:"status"`
	IsAdmin       bool              `json:"isAdmin"`
	DemoProtected bool              `json:"demoProtected"`
	Roles         []roleBrief       `json:"roles"`
	Permissions   map[string]string `json:"permissions"`
}

// handleInitStatus 查询系统是否已初始化。
func (s *Server) handleInitStatus(c *gin.Context) {
	initialized, err := s.auth.IsInitialized()
	if err != nil {
		fail(c, "查询初始化状态失败: "+err.Error())
		return
	}
	ok(c, gin.H{"initialized": initialized})
}

// handleInit 初始化首个管理员账号。
func (s *Server) handleInit(c *gin.Context) {
	var req initRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, "参数错误: "+err.Error())
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	if err := s.auth.InitAdmin(req.Username, req.Password, req.Nickname); err != nil {
		fail(c, err.Error())
		return
	}
	auditf(c, "初始化管理员账号 username=%s", req.Username)
	okNull(c)
}

// handleLogin 登录并签发令牌。
func (s *Server) handleLogin(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, "参数错误: "+err.Error())
		return
	}
	u, token, expiresIn, err := s.auth.Login(strings.TrimSpace(req.Username), req.Password)
	if err != nil {
		wlog.Ctx(c.Request.Context()).Warnf("登录失败 username=%s ip=%s msg=%s", req.Username, c.ClientIP(), err.Error())
		failCode(c, codeUnauthorized, err.Error())
		return
	}
	p, err := s.auth.Resolve(u.Id)
	if err != nil {
		fail(c, "加载用户权限失败: "+err.Error())
		return
	}
	auditf(c, "登录成功 username=%s", u.Username)
	ok(c, gin.H{
		"token":     token,
		"expiresIn": expiresIn,
		"user":      s.buildProfile(p),
	})
}

// handleProfile 返回当前登录用户信息。
func (s *Server) handleProfile(c *gin.Context) {
	p := auth.CurrentPrincipal(c)
	if p == nil {
		unauthorized(c, "未登录")
		return
	}
	ok(c, s.buildProfile(p))
}

func (s *Server) buildProfile(p *auth.Principal) userProfile {
	return userProfile{
		Id:            p.User.Id,
		Username:      p.User.Username,
		Nickname:      p.User.Nickname,
		Status:        p.User.Status,
		IsAdmin:       p.IsAdmin,
		DemoProtected: s.cfg.IsDemoProtected(p.User.Username),
		Roles:         toRoleBriefs(p.Roles),
		Permissions:   p.Permissions(),
	}
}

func toRoleBriefs(roles []store.Role) []roleBrief {
	out := make([]roleBrief, 0, len(roles))
	for _, r := range roles {
		out = append(out, roleBrief{Id: r.Id, Code: r.Code, Name: r.Name})
	}
	return out
}

type createUserRequest struct {
	Username string  `json:"username"`
	Password string  `json:"password"`
	Nickname string  `json:"nickname"`
	RoleIds  []int64 `json:"roleIds"`
	Status   *int    `json:"status"`
}

type updateUserRequest struct {
	Nickname string  `json:"nickname"`
	RoleIds  []int64 `json:"roleIds"`
	Status   *int    `json:"status"`
}

type passwordRequest struct {
	Password string `json:"password"`
}

type changePasswordRequest struct {
	OldPassword string `json:"oldPassword"`
	NewPassword string `json:"newPassword"`
}

// userItem 用户列表项，附带已分配角色。
type userItem struct {
	Id            int64       `json:"id"`
	Username      string      `json:"username"`
	Nickname      string      `json:"nickname"`
	Status        int         `json:"status"`
	DemoProtected bool        `json:"demoProtected"`
	CreateTime    int64       `json:"createTime"`
	UpdateTime    int64       `json:"updateTime"`
	Roles         []roleBrief `json:"roles"`
}

// handleListUsers 用户列表。
func (s *Server) handleListUsers(c *gin.Context) {
	users, err := s.store.ListUsers()
	if err != nil {
		fail(c, "查询用户列表失败: "+err.Error())
		return
	}
	list := make([]userItem, 0, len(users))
	for _, u := range users {
		roles, err := s.store.ListRolesByUserId(u.Id)
		if err != nil {
			fail(c, "查询用户角色失败: "+err.Error())
			return
		}
		list = append(list, userItem{
			Id:            u.Id,
			Username:      u.Username,
			Nickname:      u.Nickname,
			Status:        u.Status,
			DemoProtected: s.cfg.IsDemoProtected(u.Username),
			CreateTime:    u.CreateTime,
			UpdateTime:    u.UpdateTime,
			Roles:         toRoleBriefs(roles),
		})
	}
	ok(c, list)
}

// handleCreateUser 创建用户并可同时分配角色。
func (s *Server) handleCreateUser(c *gin.Context) {
	var req createUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, "参数错误: "+err.Error())
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" || req.Password == "" {
		fail(c, "用户名和密码不能为空")
		return
	}
	if err := s.validateRoleIds(req.RoleIds); err != nil {
		fail(c, err.Error())
		return
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		fail(c, "密码加密失败: "+err.Error())
		return
	}
	u := &store.User{
		Username: req.Username,
		Password: hash,
		Nickname: req.Nickname,
		Status:   1,
	}
	if u.Nickname == "" {
		u.Nickname = req.Username
	}
	if req.Status != nil {
		u.Status = *req.Status
	}
	if err := s.store.CreateUser(u); err != nil {
		fail(c, "创建用户失败: "+err.Error())
		return
	}
	if len(req.RoleIds) > 0 {
		if err := s.store.SetUserRoles(u.Id, req.RoleIds); err != nil {
			fail(c, "分配角色失败: "+err.Error())
			return
		}
	}
	auditf(c, "创建用户 username=%s roles=%v", u.Username, req.RoleIds)
	ok(c, u)
}

// handleUpdateUser 更新用户基础信息并覆盖式分配角色。
func (s *Server) handleUpdateUser(c *gin.Context) {
	id, okId := parseIdParam(c)
	if !okId {
		return
	}
	var req updateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, "参数错误: "+err.Error())
		return
	}
	if err := s.validateRoleIds(req.RoleIds); err != nil {
		fail(c, err.Error())
		return
	}
	existing, err := s.store.GetUserById(id)
	if err != nil {
		fail(c, "用户不存在")
		return
	}
	if s.cfg.IsDemoProtected(existing.Username) {
		fail(c, demoProtectedMsg)
		return
	}
	status := existing.Status
	if req.Status != nil {
		status = *req.Status
	}
	p := auth.CurrentPrincipal(c)
	if p != nil && p.User.Id == id {
		if status != 1 {
			fail(c, "不能禁用当前登录账号")
			return
		}
		if req.RoleIds != nil {
			adminRole, err := s.store.GetRoleByCode(store.RoleCodeAdmin)
			if err != nil || !containsId(req.RoleIds, adminRole.Id) {
				fail(c, "不能移除当前登录账号的管理员角色")
				return
			}
		}
	}
	existing.Nickname = req.Nickname
	existing.Status = status
	if err := s.store.UpdateUser(existing); err != nil {
		fail(c, "更新用户失败: "+err.Error())
		return
	}
	if req.RoleIds != nil {
		if err := s.store.SetUserRoles(id, req.RoleIds); err != nil {
			fail(c, "分配角色失败: "+err.Error())
			return
		}
	}
	auditf(c, "更新用户 username=%s status=%d roles=%v", existing.Username, status, req.RoleIds)
	okNull(c)
}

// handleDeleteUser 删除用户。
func (s *Server) handleDeleteUser(c *gin.Context) {
	id, okId := parseIdParam(c)
	if !okId {
		return
	}
	p := auth.CurrentPrincipal(c)
	if p != nil && p.User.Id == id {
		fail(c, "不能删除当前登录账号")
		return
	}
	u, err := s.store.GetUserById(id)
	if err != nil {
		fail(c, "用户不存在")
		return
	}
	if s.cfg.IsDemoProtected(u.Username) {
		fail(c, demoProtectedMsg)
		return
	}
	if err := s.store.DeleteUser(id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			fail(c, "用户不存在")
			return
		}
		fail(c, "删除用户失败: "+err.Error())
		return
	}
	auditf(c, "删除用户 userId=%d", id)
	okNull(c)
}

// handleResetPassword 重置密码。
func (s *Server) handleResetPassword(c *gin.Context) {
	id, okId := parseIdParam(c)
	if !okId {
		return
	}
	var req passwordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, "参数错误: "+err.Error())
		return
	}
	if req.Password == "" {
		fail(c, "密码不能为空")
		return
	}
	u, err := s.store.GetUserById(id)
	if err != nil {
		fail(c, "用户不存在")
		return
	}
	if s.cfg.IsDemoProtected(u.Username) {
		fail(c, demoProtectedMsg)
		return
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		fail(c, "密码加密失败: "+err.Error())
		return
	}
	if err := s.store.UpdateUserPassword(id, hash); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			fail(c, "用户不存在")
			return
		}
		fail(c, "重置密码失败: "+err.Error())
		return
	}
	auditf(c, "重置用户密码 userId=%d", id)
	okNull(c)
}

// handleChangePassword 修改当前登录用户的密码，需校验原密码。
func (s *Server) handleChangePassword(c *gin.Context) {
	p := auth.CurrentPrincipal(c)
	if p == nil {
		unauthorized(c, "未登录")
		return
	}
	var req changePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, "参数错误: "+err.Error())
		return
	}
	if req.OldPassword == "" || req.NewPassword == "" {
		fail(c, "原密码和新密码不能为空")
		return
	}
	u, err := s.store.GetUserById(p.User.Id)
	if err != nil {
		fail(c, "用户不存在")
		return
	}
	if s.cfg.IsDemoProtected(u.Username) {
		fail(c, demoProtectedMsg)
		return
	}
	if !auth.CheckPassword(u.Password, req.OldPassword) {
		fail(c, "原密码错误")
		return
	}
	hash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		fail(c, "密码加密失败: "+err.Error())
		return
	}
	if err := s.store.UpdateUserPassword(u.Id, hash); err != nil {
		fail(c, "修改密码失败: "+err.Error())
		return
	}
	auditf(c, "修改密码 username=%s", u.Username)
	okNull(c)
}

// validateRoleIds 校验待分配的角色 id 均存在。
func (s *Server) validateRoleIds(roleIds []int64) error {
	if len(roleIds) == 0 {
		return nil
	}
	roles, err := s.store.ListRoles()
	if err != nil {
		return errors.New("查询角色失败: " + err.Error())
	}
	exist := make(map[int64]struct{}, len(roles))
	for _, r := range roles {
		exist[r.Id] = struct{}{}
	}
	for _, id := range roleIds {
		if _, ok := exist[id]; !ok {
			return errors.New("存在无效的角色 id")
		}
	}
	return nil
}

func containsId(ids []int64, id int64) bool {
	for _, v := range ids {
		if v == id {
			return true
		}
	}
	return false
}

func parseIdParam(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		fail(c, "非法的 id 参数")
		return 0, false
	}
	return id, true
}
