package auth

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	wlog "github.com/wueasy/wueasy-go-tools/log"
	"github.com/wueasy/wueasy-go-tools/result"
	"golang.org/x/crypto/bcrypt"

	"waymark/internal/store"
)

// ctxPrincipalKey 登录主体在 gin.Context 中的键。
const ctxPrincipalKey = "waymark.principal"

// Claims JWT 负载，仅承载身份标识，权限每次请求实时解析。
type Claims struct {
	UserId   int64  `json:"userId"`
	Username string `json:"username"`
	jwt.RegisteredClaims
}

// Principal 当前登录用户及其有效权限。
type Principal struct {
	User    *store.User
	Roles   []store.Role
	IsAdmin bool
	// perms 命名空间 -> 权限等级；管理员不受此限制。
	perms map[string]string
}

// CanAccess 判断是否可访问指定命名空间。管理员可访问全部命名空间。
func (p *Principal) CanAccess(namespace string) bool {
	if p.IsAdmin {
		return true
	}
	if namespace == "" {
		return false
	}
	_, ok := p.perms[namespace]
	return ok
}

// CanWrite 判断是否可在指定命名空间写入。
func (p *Principal) CanWrite(namespace string) bool {
	if p.IsAdmin {
		return true
	}
	return p.perms[namespace] == store.PermissionWrite
}

// CanWriteAny 判断是否在任一命名空间可写。
func (p *Principal) CanWriteAny() bool {
	if p.IsAdmin {
		return true
	}
	for _, perm := range p.perms {
		if perm == store.PermissionWrite {
			return true
		}
	}
	return false
}

// Permissions 返回命名空间授权副本。
func (p *Principal) Permissions() map[string]string {
	out := make(map[string]string, len(p.perms))
	for namespace, perm := range p.perms {
		out[namespace] = perm
	}
	return out
}

// Authenticator 认证与鉴权。
type Authenticator struct {
	secret []byte
	ttl    time.Duration
	store  *store.Store
}

// New 创建认证器。
func New(st *store.Store, secret string, ttlSeconds int64) *Authenticator {
	if ttlSeconds <= 0 {
		ttlSeconds = 7200
	}
	return &Authenticator{
		secret: []byte(secret),
		ttl:    time.Duration(ttlSeconds) * time.Second,
		store:  st,
	}
}

// IsInitialized 判断系统是否已初始化（是否存在用户）。
func (a *Authenticator) IsInitialized() (bool, error) {
	cnt, err := a.store.CountUsers()
	if err != nil {
		return false, err
	}
	return cnt > 0, nil
}

// InitAdmin 初始化首个管理员账号，仅在系统未初始化时可用。
func (a *Authenticator) InitAdmin(username, password, nickname string) error {
	if username == "" || password == "" {
		return errors.New("用户名和密码不能为空")
	}
	initialized, err := a.IsInitialized()
	if err != nil {
		return err
	}
	if initialized {
		return errors.New("系统已初始化，无法重复执行")
	}
	hash, err := HashPassword(password)
	if err != nil {
		return err
	}
	if nickname == "" {
		nickname = username
	}
	u := &store.User{
		Username: username,
		Password: hash,
		Nickname: nickname,
		Status:   1,
	}
	if err := a.store.CreateUser(u); err != nil {
		return err
	}
	admin, err := a.store.GetRoleByCode(store.RoleCodeAdmin)
	if err != nil {
		return fmt.Errorf("内置管理员角色缺失: %w", err)
	}
	return a.store.SetUserRoles(u.Id, []int64{admin.Id})
}

// Login 校验账号密码并签发令牌。
func (a *Authenticator) Login(username, password string) (*store.User, string, int64, error) {
	u, err := a.store.GetUserByUsername(username)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, "", 0, errors.New("用户名或密码错误")
		}
		return nil, "", 0, err
	}
	if u.Status != 1 {
		return nil, "", 0, errors.New("账号已被禁用")
	}
	if !CheckPassword(u.Password, password) {
		return nil, "", 0, errors.New("用户名或密码错误")
	}
	token, expiresIn, err := a.GenerateToken(u)
	if err != nil {
		return nil, "", 0, err
	}
	return u, token, expiresIn, nil
}

// GenerateToken 生成 JWT 令牌。
func (a *Authenticator) GenerateToken(u *store.User) (string, int64, error) {
	now := time.Now()
	claims := Claims{
		UserId:   u.Id,
		Username: u.Username,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(a.ttl)),
			Subject:   strconv.FormatInt(u.Id, 10),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(a.secret)
	if err != nil {
		return "", 0, err
	}
	return signed, int64(a.ttl.Seconds()), nil
}

// ParseToken 解析并校验令牌。
func (a *Authenticator) ParseToken(tokenStr string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("不支持的签名算法: %v", t.Header["alg"])
		}
		return a.secret, nil
	})
	if err != nil || !token.Valid {
		return nil, errors.New("令牌无效或已过期")
	}
	return claims, nil
}

// Resolve 依据用户 id 实时解析角色与命名空间权限。
func (a *Authenticator) Resolve(userId int64) (*Principal, error) {
	u, err := a.store.GetUserById(userId)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, errors.New("用户不存在")
		}
		return nil, err
	}
	if u.Status != 1 {
		return nil, errors.New("账号已被禁用")
	}
	roles, err := a.store.ListRolesByUserId(userId)
	if err != nil {
		return nil, err
	}
	p := &Principal{User: u, Roles: roles, perms: map[string]string{}}
	for _, r := range roles {
		if r.Code == store.RoleCodeAdmin {
			p.IsAdmin = true
			continue
		}
		for _, namespace := range r.Namespaces {
			// 多角色叠加时同一命名空间取最高权限。
			if _, ok := p.perms[namespace]; !ok || r.Permission == store.PermissionWrite {
				p.perms[namespace] = r.Permission
			}
		}
	}
	return p, nil
}

// CurrentPrincipal 获取当前登录主体。
func CurrentPrincipal(c *gin.Context) *Principal {
	if v, ok := c.Get(ctxPrincipalKey); ok {
		if p, ok := v.(*Principal); ok {
			return p
		}
	}
	return nil
}

// RequireAuth 认证中间件，校验 Bearer 令牌或 token 查询参数（SSE 场景），并实时加载权限。
func (a *Authenticator) RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenStr := extractToken(c)
		if tokenStr == "" {
			abortUnauthorized(c, "未提供认证令牌")
			return
		}
		claims, err := a.ParseToken(tokenStr)
		if err != nil {
			abortUnauthorized(c, err.Error())
			return
		}
		p, err := a.Resolve(claims.UserId)
		if err != nil {
			abortUnauthorized(c, err.Error())
			return
		}
		c.Set(ctxPrincipalKey, p)
		c.Next()
	}
}

// RequireAdmin 管理员中间件，需在 RequireAuth 之后使用。
func (a *Authenticator) RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		p := CurrentPrincipal(c)
		if p == nil {
			abortUnauthorized(c, "未登录")
			return
		}
		if !p.IsAdmin {
			wlog.Ctx(c.Request.Context()).Warnf("越权访问 user=%s ip=%s path=%s", p.User.Username, c.ClientIP(), c.Request.URL.Path)
			c.AbortWithStatusJSON(http.StatusForbidden, result.Fail(403, "需要管理员权限"))
			return
		}
		c.Next()
	}
}

func extractToken(c *gin.Context) string {
	header := c.GetHeader("Authorization")
	if header != "" {
		if strings.HasPrefix(header, "Bearer ") {
			return strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
		}
		return strings.TrimSpace(header)
	}
	return c.Query("token")
}

func abortUnauthorized(c *gin.Context, msg string) {
	wlog.Ctx(c.Request.Context()).Warnf("认证失败 ip=%s path=%s msg=%s", c.ClientIP(), c.Request.URL.Path, msg)
	c.AbortWithStatusJSON(http.StatusUnauthorized, result.Fail(401, msg))
}

// HashPassword 使用 bcrypt 生成密码哈希。
func HashPassword(password string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// CheckPassword 校验明文密码与哈希是否匹配。
func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}
