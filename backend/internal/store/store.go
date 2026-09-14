package store

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite"

	"waymark/internal/config"
)

// 数据库方言。
const (
	DialectMySQL  = "mysql"
	DialectSQLite = "sqlite"
)

// ErrNotFound 记录不存在。
var ErrNotFound = errors.New("记录不存在")

// Store 数据访问层，屏蔽 MySQL / SQLite 方言差异。
type Store struct {
	db      *sqlx.DB
	dialect string
}

// Open 根据配置打开数据库连接。
func Open(cfg *config.DBConfig) (*Store, error) {
	switch cfg.Type {
	case DialectMySQL:
		db, err := sqlx.Connect(DialectMySQL, cfg.MySQL.DSN)
		if err != nil {
			return nil, fmt.Errorf("连接 mysql 失败: %w", err)
		}
		if cfg.MySQL.MaxOpenConns > 0 {
			db.SetMaxOpenConns(cfg.MySQL.MaxOpenConns)
		}
		if cfg.MySQL.MaxIdleConns > 0 {
			db.SetMaxIdleConns(cfg.MySQL.MaxIdleConns)
		}
		return &Store{db: db, dialect: DialectMySQL}, nil

	case DialectSQLite:
		path := cfg.SQLite.Path
		if dir := filepath.Dir(path); dir != "" && dir != "." {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return nil, fmt.Errorf("创建 sqlite 目录失败: %w", err)
			}
		}
		db, err := sqlx.Connect(DialectSQLite, sqliteDSN(path))
		if err != nil {
			return nil, fmt.Errorf("连接 sqlite 失败: %w", err)
		}
		// SQLite 单文件写入串行化，限制单连接可彻底避免 database is locked。
		db.SetMaxOpenConns(1)
		db.SetMaxIdleConns(1)
		return &Store{db: db, dialect: DialectSQLite}, nil

	default:
		return nil, fmt.Errorf("不支持的数据库类型: %s", cfg.Type)
	}
}

// sqliteDSN 拼接 SQLite 连接串，启用 busy_timeout 与 WAL 提升并发写入稳定性。
func sqliteDSN(path string) string {
	dsn := path
	if !strings.HasPrefix(dsn, "file:") {
		dsn = "file:" + dsn
	}
	sep := "?"
	if strings.Contains(dsn, "?") {
		sep = "&"
	}
	return dsn + sep + "_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"
}

// Dialect 返回当前数据库方言。
func (s *Store) Dialect() string { return s.dialect }

// DB 返回底层连接，供特殊场景使用。
func (s *Store) DB() *sqlx.DB { return s.db }

// Close 关闭连接。
func (s *Store) Close() error { return s.db.Close() }

// Init 初始化数据库。
// SQLite 为内置数据库，自动建表并写入基础数据；
// MySQL 表结构不在代码中创建，需先执行 sql/mysql.sql，此处仅校验表结构并写入基础数据。
func (s *Store) Init() error {
	if s.dialect == DialectSQLite {
		for _, stmt := range sqliteSchema {
			if _, err := s.db.Exec(stmt); err != nil {
				return fmt.Errorf("初始化表结构失败: %w", err)
			}
		}
	} else if err := s.checkTables(mysqlTables); err != nil {
		return err
	}
	if err := s.seed(); err != nil {
		return err
	}
	if s.dialect == DialectSQLite {
		return s.migrateLegacySQLite()
	}
	return nil
}

// migrateLegacySQLite 将旧版「用户单角色 + 用户命名空间授权」平滑迁移到角色模型（幂等）。
// 新版结构由 sqliteSchema 建立，此处仅处理历史库中遗留的 user.role 列与 user_namespace 表。
func (s *Store) migrateLegacySQLite() error {
	hasRole, err := s.sqliteHasColumn("user", "role")
	if err != nil {
		return err
	}
	if hasRole {
		now := time.Now().UnixMilli()
		if _, err := s.db.Exec(
			"INSERT OR IGNORE INTO user_role (user_id, role_id, create_time)"+
				" SELECT u.id, r.id, ? FROM user u JOIN role r ON r.code = u.role", now); err != nil {
			return fmt.Errorf("迁移用户角色失败: %w", err)
		}
		if _, err := s.db.Exec("ALTER TABLE user DROP COLUMN role"); err != nil {
			return fmt.Errorf("移除 user.role 列失败: %w", err)
		}
	}
	legacy, err := s.sqliteTableExists("user_namespace")
	if err != nil {
		return err
	}
	if legacy {
		now := time.Now().UnixMilli()
		if _, err := s.db.Exec(
			"INSERT OR IGNORE INTO role_namespace (role_id, namespace, create_time)"+
				" SELECT DISTINCT ur.role_id, un.namespace, ? FROM user_namespace un"+
				" JOIN user_role ur ON ur.user_id = un.user_id", now); err != nil {
			return fmt.Errorf("迁移命名空间授权失败: %w", err)
		}
		if _, err := s.db.Exec("DROP TABLE user_namespace"); err != nil {
			return fmt.Errorf("删除 user_namespace 表失败: %w", err)
		}
	}
	return nil
}

// sqliteHasColumn 判断表是否包含指定列。
func (s *Store) sqliteHasColumn(table, column string) (bool, error) {
	var cnt int
	if err := s.db.Get(&cnt, "SELECT COUNT(1) FROM pragma_table_info(?) WHERE name = ?", table, column); err != nil {
		return false, err
	}
	return cnt > 0, nil
}

// sqliteTableExists 判断表是否存在。
func (s *Store) sqliteTableExists(table string) (bool, error) {
	var cnt int
	if err := s.db.Get(&cnt, "SELECT COUNT(1) FROM sqlite_master WHERE type = 'table' AND name = ?", table); err != nil {
		return false, err
	}
	return cnt > 0, nil
}

// checkTables 校验表是否存在，提示 MySQL 用户先执行建表脚本。
func (s *Store) checkTables(tables []string) error {
	for _, table := range tables {
		if _, err := s.db.Exec("SELECT 1 FROM " + table + " LIMIT 0"); err != nil {
			return fmt.Errorf("数据表 %s 不可用，请先执行 sql/mysql.sql 建表脚本: %w", table, err)
		}
	}
	return nil
}

// insertIgnoreKeyword 返回两种方言的「主键/唯一冲突忽略」插入关键字。
func (s *Store) insertIgnoreKeyword() string {
	if s.dialect == DialectSQLite {
		return "INSERT OR IGNORE"
	}
	return "INSERT IGNORE"
}

// NowMillis 返回数据库服务器当前时间的毫秒时间戳。
// 集群多机部署时统一以数据库时钟为准，避免各机器本地时钟漂移导致心跳误判、租约误抢占。
func (s *Store) NowMillis() (int64, error) {
	var now int64
	if err := s.db.Get(&now, "SELECT "+s.dbNowExpr()); err != nil {
		return 0, err
	}
	return now, nil
}

// dbNowExpr 返回两种方言下「数据库当前时间毫秒时间戳」的 SQL 表达式。
func (s *Store) dbNowExpr() string {
	if s.dialect == DialectSQLite {
		return "CAST((julianday('now') - 2440587.5) * 86400000 AS INTEGER)"
	}
	return "CAST(UNIX_TIMESTAMP(NOW(3)) * 1000 AS SIGNED)"
}

// initialRoles 初始化角色。admin 为内置角色不可编辑删除；editor / viewer 仅作初始便利角色，可自由增删改。
var initialRoles = []Role{
	{Code: RoleCodeAdmin, Name: "管理员", Description: "拥有全部命名空间的读写权限", Permission: PermissionWrite, Builtin: BuiltinYes},
	{Code: "editor", Name: "编辑", Description: "被授权命名空间内可读写", Permission: PermissionWrite, Builtin: BuiltinNo},
	{Code: "viewer", Name: "只读", Description: "被授权命名空间内只读", Permission: PermissionRead, Builtin: BuiltinNo},
}

// seed 初始化公共命名空间、内置角色与 Leader 租约行（幂等）。
func (s *Store) seed() error {
	now := time.Now().UnixMilli()
	stmt := s.insertIgnoreKeyword() +
		" INTO namespace (namespace, name, description, create_time, update_time) VALUES (?, ?, ?, ?, ?)"
	if _, err := s.db.Exec(stmt, DefaultNamespace, "公共命名空间", "默认命名空间", now, now); err != nil {
		return fmt.Errorf("初始化默认命名空间失败: %w", err)
	}
	stmt = s.insertIgnoreKeyword() +
		" INTO cluster_leader (leader_key, node_id, lease_until, update_time) VALUES (?, ?, ?, ?)"
	if _, err := s.db.Exec(stmt, "default", "", int64(0), now); err != nil {
		return fmt.Errorf("初始化 Leader 租约失败: %w", err)
	}
	stmt = s.insertIgnoreKeyword() +
		" INTO role (code, name, description, permission, builtin, create_time, update_time) VALUES (?, ?, ?, ?, ?, ?, ?)"
	for _, r := range initialRoles {
		if _, err := s.db.Exec(stmt, r.Code, r.Name, r.Description, r.Permission, r.Builtin, now, now); err != nil {
			return fmt.Errorf("初始化角色 %s 失败: %w", r.Code, err)
		}
	}
	return nil
}
