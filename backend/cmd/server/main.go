package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	wconfig "github.com/wueasy/wueasy-go-tools/config"
	wlog "github.com/wueasy/wueasy-go-tools/log"
	startup "github.com/wueasy/wueasy-go-tools/startup-parameter"
	systemService "github.com/wueasy/wueasy-go-tools/system-service"
	"github.com/wueasy/wueasy-go-tools/utils"

	"waymark/internal/api"
	appconfig "waymark/internal/config"
	"waymark/internal/store"
	"waymark/internal/web"
)

var (
	shutdownOnce sync.Once
	shutdownCh   = make(chan struct{})
)

func main() {
	sp := startup.GetStartupParameter()

	cfg, err := appconfig.Load(sp.ConfigPath)
	if err != nil {
		// 此时日志尚未初始化，只能输出到标准错误。
		log.Fatalf("加载配置失败: %v", err)
	}

	serviceConfig := wconfig.SystemServiceConfig{
		Version:     "0.1.0",
		Name:        "waymark",
		DisplayName: "Waymark 注册配置中心",
		Description: "Waymark 注册中心与配置中心后端服务",
		EnvRootPath: "WAYMARK_ROOT",
	}

	rootPath := utils.GetRootPath(serviceConfig.EnvRootPath)

	wlog.Init(rootPath, cfg.Log)

	// 使用 wueasy-go-tools 的系统服务启动能力：
	// 默认直接运行，也支持 install/uninstall/start/stop/restart/status 等系统服务命令。
	systemService.Run(sp, rootPath, serviceConfig, "", "", func() {
		runServer(cfg)
	}, func() {
		shutdownOnce.Do(func() { close(shutdownCh) })
	})
}

func runServer(cfg *appconfig.Config) {
	logger := wlog.Ctx(context.Background())

	if cfg.Auth.JWTSecret == "" {
		if cfg.Cluster.Enabled {
			logger.Fatal("集群模式必须配置 auth.jwt-secret，且各节点保持一致")
		}
		cfg.Auth.JWTSecret = randomHex(32)
		logger.Warn("未配置 auth.jwt-secret，已为单机模式生成随机密钥（重启后登录态失效）")
	}

	if cfg.Demo.Enabled {
		logger.Warnf("演示环境已启用，以下账号将被冻结（禁止删除/改密/禁用/改角色）: %v", cfg.Demo.ProtectedUsers)
	}

	st, err := store.Open(&cfg.DB)
	if err != nil {
		logger.Fatalf("初始化数据库失败: %v", err)
	}
	if err := st.Init(); err != nil {
		logger.Fatalf("初始化数据库表结构失败: %v", err)
	}

	srv := api.NewServer(cfg, st, cfg.Auth.JWTSecret)
	srv.Start()
	defer srv.Close()

	dist, err := fs.Sub(web.Dist, "dist")
	if err != nil {
		logger.Fatalf("加载前端资源失败: %v", err)
	}

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(api.CORS())
	r.Use(api.RequestLogger())
	r.Use(wlog.GinRecovery())
	if cfg.Server.Gzip {
		r.Use(api.Gzip())
	}
	srv.Register(r)
	r.NoRoute(spaHandler(dist))

	httpServer := &http.Server{
		Addr:    cfg.Addr(),
		Handler: r,
	}

	go func() {
		logger.Infof("Waymark 已启动: http://%s  模式: %s  数据库: %s", cfg.Addr(), mode(cfg), cfg.DB.Type)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatalf("服务启动失败: %v", err)
		}
	}()

	// 前台运行时监听系统信号，系统服务模式下由 stopCallback 触发。
	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		<-sig
		shutdownOnce.Do(func() { close(shutdownCh) })
	}()

	<-shutdownCh

	logger.Info("正在关闭服务...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = httpServer.Shutdown(ctx)
	wlog.Sync()
}

func mode(cfg *appconfig.Config) string {
	if cfg.Cluster.Enabled {
		return "集群模式"
	}
	return "单机模式"
}

func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "waymark-default-secret-please-change"
	}
	return hex.EncodeToString(b)
}

// spaHandler 提供静态资源并回退到 index.html。
func spaHandler(dist fs.FS) gin.HandlerFunc {
	fileServer := http.FileServer(http.FS(dist))
	return func(c *gin.Context) {
		p := c.Request.URL.Path
		if p != "/" {
			if _, err := fs.Stat(dist, p[1:]); err != nil {
				c.Request.URL.Path = "/"
			}
		}
		fileServer.ServeHTTP(c.Writer, c.Request)
	}
}
