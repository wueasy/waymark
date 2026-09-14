package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	wconfig "github.com/wueasy/wueasy-go-tools/config"
	"gopkg.in/yaml.v3"
)

// Config 应用配置，对应 config.yaml。
type Config struct {
	Server   ServerConfig      `yaml:"server"`
	Log      wconfig.LogConfig `yaml:"log"`
	Auth     AuthConfig        `yaml:"auth"`
	Demo     DemoConfig        `yaml:"demo"`
	DB       DBConfig          `yaml:"db"`
	Cluster  ClusterConfig     `yaml:"cluster"`
	Registry RegistryConfig    `yaml:"registry"`
	SSE      SSEConfig         `yaml:"sse"`
}

// ServerConfig HTTP 服务配置。服务固定监听 0.0.0.0，不提供 address 配置。
type ServerConfig struct {
	Port int  `yaml:"port"`
	Gzip bool `yaml:"gzip"`
}

// AuthConfig 认证配置。
type AuthConfig struct {
	JWTSecret string `yaml:"jwt-secret"` // 集群各节点需保持一致
	TokenTTL  int64  `yaml:"token-ttl"`  // 秒
}

// DemoConfig 演示环境配置。
// 启用后，protected-users 中列出的账号将被冻结：禁止删除、重置密码、修改本人密码、
// 禁用/启用、修改昵称与角色分配，避免演示账号被改动或删除后无法继续演示。
type DemoConfig struct {
	Enabled        bool     `yaml:"enabled"`         // 演示环境开关
	ProtectedUsers []string `yaml:"protected-users"` // 受保护账号用户名列表
}

// DBConfig 数据库配置。
type DBConfig struct {
	Type   string       `yaml:"type"` // mysql | sqlite
	MySQL  MySQLConfig  `yaml:"mysql"`
	SQLite SQLiteConfig `yaml:"sqlite"`
}

// MySQLConfig MySQL 连接配置。
type MySQLConfig struct {
	DSN          string `yaml:"dsn"`
	MaxOpenConns int    `yaml:"max-open-conns"`
	MaxIdleConns int    `yaml:"max-idle-conns"`
}

// SQLiteConfig SQLite 连接配置。
type SQLiteConfig struct {
	Path string `yaml:"path"`
}

// ClusterConfig 集群配置。
type ClusterConfig struct {
	Enabled          bool   `yaml:"enabled"`           // 集群开关，sqlite 模式下强制 false
	NodeID           string `yaml:"node-id"`           // 空则自动生成
	IP               string `yaml:"ip"`                // 空则自动探测本机 IP
	Port             int    `yaml:"port"`              // 0 则使用 server.port
	LeaseTTL         int    `yaml:"lease-ttl"`         // 秒，Leader 租约时长
	ElectionInterval int    `yaml:"election-interval"` // 秒，选举/续租周期
	NodeTimeout      int    `yaml:"node-timeout"`      // 秒，节点心跳超时（标记 DOWN）
	NodeRetain       int    `yaml:"node-retain"`       // 秒，节点无心跳多久后移除记录
}

// RegistryConfig 注册中心配置。
type RegistryConfig struct {
	HeartbeatTimeout int `yaml:"heartbeat-timeout"` // 秒，实例心跳超时
	SweepInterval    int `yaml:"sweep-interval"`    // 秒，淘汰扫描周期（Leader 执行）
}

// SSEConfig 变更推送配置。
type SSEConfig struct {
	Heartbeat    int `yaml:"heartbeat"`     // 秒，keep-alive 间隔
	PollMs       int `yaml:"poll-ms"`       // 毫秒，变更日志扫描周期
	LogRetention int `yaml:"log-retention"` // 秒，变更日志保留期
}

// Load 读取并解析 config.yaml。path 为空时使用默认文件名。
func Load(path string) (*Config, error) {
	if path == "" {
		path = "config.yaml"
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}
	cfg := defaultConfig()
	if err := yaml.Unmarshal(b, cfg); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Addr 返回监听地址，服务端固定监听所有网卡。
func (c *Config) Addr() string {
	return "0.0.0.0:" + strconv.Itoa(c.Port())
}

// Port 返回服务端口，未配置（<=0）时返回默认端口。
func (c *Config) Port() int {
	if c.Server.Port <= 0 {
		return 9868
	}
	return c.Server.Port
}

// IsDemoProtected 判断用户名在演示环境下是否为受保护账号（忽略大小写与首尾空格）。
func (c *Config) IsDemoProtected(username string) bool {
	if !c.Demo.Enabled || username == "" {
		return false
	}
	username = strings.TrimSpace(username)
	for _, u := range c.Demo.ProtectedUsers {
		if strings.EqualFold(strings.TrimSpace(u), username) {
			return true
		}
	}
	return false
}

// Validate 校验配置合法性。
func (c *Config) Validate() error {
	switch c.DB.Type {
	case "mysql":
	case "sqlite":
		if c.Cluster.Enabled {
			return errors.New("sqlite 模式不支持集群，请将 cluster.enabled 设为 false")
		}
	default:
		return fmt.Errorf("不支持的 db.type: %q，仅支持 mysql 或 sqlite", c.DB.Type)
	}
	if c.DB.Type == "sqlite" && c.DB.SQLite.Path == "" {
		return errors.New("未配置 db.sqlite.path")
	}
	if c.DB.Type == "mysql" && c.DB.MySQL.DSN == "" {
		return errors.New("未配置 db.mysql.dsn")
	}
	if c.Auth.TokenTTL <= 0 {
		c.Auth.TokenTTL = 7200
	}
	if c.Server.Port < 0 || c.Server.Port > 65535 {
		return fmt.Errorf("server.port 非法: %d，取值范围 1-65535", c.Server.Port)
	}
	if c.Cluster.Port < 0 || c.Cluster.Port > 65535 {
		return fmt.Errorf("cluster.port 非法: %d，取值范围 1-65535", c.Cluster.Port)
	}
	return nil
}

func defaultConfig() *Config {
	return &Config{
		Server: ServerConfig{Port: 9868, Gzip: true},
		Auth:   AuthConfig{TokenTTL: 7200},
		DB: DBConfig{
			Type:   "sqlite",
			MySQL:  MySQLConfig{MaxOpenConns: 50, MaxIdleConns: 10},
			SQLite: SQLiteConfig{Path: "./data/waymark.db"},
		},
		Cluster: ClusterConfig{
			Enabled:          false,
			LeaseTTL:         10,
			ElectionInterval: 3,
			NodeTimeout:      30,
			NodeRetain:       300,
		},
		Registry: RegistryConfig{HeartbeatTimeout: 15, SweepInterval: 10},
		SSE:      SSEConfig{Heartbeat: 15, PollMs: 500, LogRetention: 604800},
	}
}
