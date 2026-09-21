package api

import (
	"compress/gzip"
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	wlog "github.com/wueasy/wueasy-go-tools/log"

	"waymark/internal/auth"
	"waymark/internal/cluster"
	"waymark/internal/config"
	"waymark/internal/configcenter"
	"waymark/internal/event"
	"waymark/internal/registry"
	"waymark/internal/store"
)

// Server HTTP 服务端，聚合各业务模块。
type Server struct {
	cfg          *config.Config
	store        *store.Store
	auth         *auth.Authenticator
	hub          *event.Hub
	registry     *registry.Service
	configCenter *configcenter.Service
	cluster      *cluster.Cluster
	publisher    *event.Publisher
	cancel       context.CancelFunc
}

// NewServer 创建服务端。
func NewServer(cfg *config.Config, st *store.Store, jwtSecret string) *Server {
	hub := event.NewHub()
	reg := registry.New(st)
	s := &Server{
		cfg:          cfg,
		store:        st,
		auth:         auth.New(st, jwtSecret, cfg.Auth.TokenTTL),
		hub:          hub,
		registry:     reg,
		configCenter: configcenter.New(st),
		cluster:      cluster.New(cfg, st, reg),
		publisher:    event.NewPublisher(st, hub, cfg.SSE.PollMs),
	}
	return s
}

// Start 启动后台任务（变更推送、集群心跳与选举）。
func (s *Server) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	s.publisher.Start(ctx)
	s.cluster.Start(ctx)
}

// Close 释放资源。退出前清理本节点订阅会话并将节点标记为 DOWN，
// 使订阅列表与集群节点状态立即反映下线，无需等待心跳超时。
func (s *Server) Close() {
	if s.cancel != nil {
		s.cancel()
	}
	if s.store == nil {
		return
	}
	ctx := wlog.Ctx(context.Background())
	nodeId := s.cluster.NodeId()
	if n, err := s.store.DeleteSubscribersByNode(nodeId); err != nil {
		ctx.Warnf("[server] 清理本节点订阅会话失败: %v", err)
	} else if n > 0 {
		ctx.Infof("[server] 清理本节点订阅会话 %d 条", n)
	}
	if err := s.store.MarkNodeDown(nodeId); err != nil {
		ctx.Warnf("[server] 标记本节点下线失败: %v", err)
	}
	_ = s.store.Close()
}

// Register 将 API 路由注册到 gin 引擎。
func (s *Server) Register(r *gin.Engine) {
	r.GET("/api/health", s.handleHealth)

	api := r.Group("/api")

	// 无需认证
	api.GET("/auth/init-status", s.handleInitStatus)
	api.POST("/auth/init", s.handleInit)
	api.POST("/auth/login", s.handleLogin)

	// 需认证
	authed := api.Group("")
	authed.Use(s.auth.RequireAuth())
	authed.GET("/auth/profile", s.handleProfile)
	authed.PUT("/auth/password", s.handleChangePassword)

	// 命名空间：列表按用户授权过滤，增删改为管理员
	authed.GET("/namespaces", s.handleListNamespaces)

	// 注册中心
	authed.GET("/registry/instances", s.handleListInstances)
	authed.GET("/registry/services", s.handleListServices)
	authed.POST("/registry/instance", s.handleRegisterInstance)
	authed.PUT("/registry/instance", s.handleUpdateInstance)
	authed.DELETE("/registry/instance", s.handleDeregisterInstance)
	authed.PUT("/registry/beat", s.handleBeat)

	// 配置中心
	authed.GET("/configs", s.handleListConfigs)
	authed.GET("/configs/detail", s.handleGetConfig)
	authed.GET("/configs/history", s.handleConfigHistory)
	authed.POST("/configs", s.handlePublishConfig)
	authed.PUT("/configs/draft", s.handleSaveDraft)
	authed.GET("/configs/draft", s.handleGetDraft)
	authed.DELETE("/configs/draft", s.handleDiscardDraft)
	authed.GET("/configs/draft/diff", s.handlePreviewDraftDiff)
	authed.POST("/configs/publish", s.handlePublishDraft)
	authed.POST("/configs/restore", s.handleRestoreConfig)
	authed.POST("/configs/export", s.handleExportConfigs)
	authed.POST("/configs/import", s.handleImportConfigs)
	authed.DELETE("/configs", s.handleDeleteConfig)

	// 变更订阅（SSE）：一条连接同时订阅配置与实例变更
	authed.GET("/subscribe", s.handleSubscribe)

	// 订阅端信息（按命名空间权限过滤）
	authed.GET("/subscribe/subscribers", s.handleListSubscribers)

	// 集群信息：运行模式对所有登录用户可见（前端头部需要展示），节点明细仅管理员可见
	authed.GET("/cluster/leader", s.handleClusterLeader)

	// 管理员接口
	admin := api.Group("")
	admin.Use(s.auth.RequireAuth(), s.auth.RequireAdmin())
	admin.GET("/users", s.handleListUsers)
	admin.POST("/users", s.handleCreateUser)
	admin.PUT("/users/:id", s.handleUpdateUser)
	admin.DELETE("/users/:id", s.handleDeleteUser)
	admin.PUT("/users/:id/password", s.handleResetPassword)

	// 角色管理
	admin.GET("/roles", s.handleListRoles)
	admin.POST("/roles", s.handleCreateRole)
	admin.PUT("/roles/:id", s.handleUpdateRole)
	admin.DELETE("/roles/:id", s.handleDeleteRole)

	admin.POST("/namespaces", s.handleCreateNamespace)
	admin.PUT("/namespaces/:name", s.handleUpdateNamespace)
	admin.DELETE("/namespaces/:name", s.handleDeleteNamespace)

	// 集群节点明细
	admin.GET("/cluster/nodes", s.handleClusterNodes)
}

// handleHealth 健康检查，无需认证。
func (s *Server) handleHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// authorizeNamespace 校验当前用户对目标命名空间的访问权限。
// 校验失败时已写入响应，返回 false。
func (s *Server) authorizeNamespace(c *gin.Context, namespace string, write bool) bool {
	p := auth.CurrentPrincipal(c)
	if p == nil {
		unauthorized(c, "未登录")
		return false
	}
	if !p.CanAccess(namespace) {
		forbidden(c, "无权访问命名空间: "+namespace)
		return false
	}
	if write && !p.CanWrite(namespace) {
		forbidden(c, "无命名空间 "+namespace+" 的写权限")
		return false
	}
	return true
}

// ensureNamespaceExists 校验命名空间是否存在。
func (s *Server) ensureNamespaceExists(c *gin.Context, namespace string) bool {
	if _, err := s.store.GetNamespace(namespace); err != nil {
		fail(c, "命名空间不存在: "+namespace)
		return false
	}
	return true
}

// quietPaths 高频接口不记录请求开始/结束日志，避免日志噪音。
// 这些接口的失败仍会通过 fail/unauthorized/forbidden 记录告警日志。
var quietPaths = map[string]struct{}{
	"/api/registry/beat": {},
}

// RequestLogger 记录请求开始与结束；对高频接口仅注入 traceId，不产生请求日志。
func RequestLogger() gin.HandlerFunc {
	base := wlog.GinLogger()
	return func(c *gin.Context) {
		if _, quiet := quietPaths[c.Request.URL.Path]; !quiet {
			base(c)
			return
		}
		// 复用 wlog 的 traceId 机制，保证失败日志仍带请求 ID。
		requestId := c.GetHeader("wueasy-request-id")
		if requestId == "" {
			requestId = uuid.NewString()
		}
		c.Request = c.Request.WithContext(wlog.NewContext(c.Request.Context(), requestId))
		c.Next()
	}
}

// CORS 允许前端开发服务器跨域访问。
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// Gzip 在客户端支持 gzip 时对响应体进行压缩。
// SSE 长连接与 zip 导出不做压缩，避免缓冲导致事件无法实时下发或重复压缩。
func Gzip() gin.HandlerFunc {
	return func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/subscribe") ||
			c.Request.URL.Path == "/api/configs/export" ||
			!strings.Contains(c.GetHeader("Accept-Encoding"), "gzip") {
			c.Next()
			return
		}
		gz := gzip.NewWriter(c.Writer)
		defer gz.Close()
		c.Header("Content-Encoding", "gzip")
		c.Header("Vary", "Accept-Encoding")
		c.Writer = &gzipResponseWriter{ResponseWriter: c.Writer, gz: gz}
		c.Next()
	}
}

type gzipResponseWriter struct {
	gin.ResponseWriter
	gz *gzip.Writer
}

func (g *gzipResponseWriter) WriteHeader(code int) {
	g.Header().Del("Content-Length")
	g.ResponseWriter.WriteHeader(code)
}

func (g *gzipResponseWriter) Write(b []byte) (int, error) {
	g.Header().Del("Content-Length")
	return g.gz.Write(b)
}

func (g *gzipResponseWriter) WriteString(s string) (int, error) {
	g.Header().Del("Content-Length")
	return g.gz.Write([]byte(s))
}
