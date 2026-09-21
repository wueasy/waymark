package store

// mysqlTables MySQL 表清单，用于启动时校验表结构是否已就绪。
var mysqlTables = []string{
	"user",
	"role",
	"user_role",
	"role_namespace",
	"namespace",
	"service_instance",
	"config_info",
	"config_draft",
	"config_history",
	"change_log",
	"subscriber_session",
	"cluster_node",
	"cluster_leader",
}

// sqliteSchema SQLite 建表语句（TEXT 无长度限制，可承载大字段）。
// SQLite 为内置数据库，启动时自动执行；MySQL 表结构不在代码中创建，
// 需由 sql/mysql.sql 脚本预先执行。
var sqliteSchema = []string{
	"CREATE TABLE IF NOT EXISTS user (" +
		"id INTEGER PRIMARY KEY AUTOINCREMENT," +
		"username TEXT NOT NULL," +
		"password TEXT NOT NULL," +
		"nickname TEXT NOT NULL DEFAULT ''," +
		"status INTEGER NOT NULL DEFAULT 1," +
		"create_time INTEGER NOT NULL," +
		"update_time INTEGER NOT NULL," +
		"UNIQUE (username)" +
		")",

	"CREATE TABLE IF NOT EXISTS role (" +
		"id INTEGER PRIMARY KEY AUTOINCREMENT," +
		"code TEXT NOT NULL," +
		"name TEXT NOT NULL," +
		"description TEXT NOT NULL DEFAULT ''," +
		"permission TEXT NOT NULL," +
		"builtin INTEGER NOT NULL DEFAULT 0," +
		"create_time INTEGER NOT NULL," +
		"update_time INTEGER NOT NULL," +
		"UNIQUE (code)" +
		")",

	"CREATE TABLE IF NOT EXISTS user_role (" +
		"id INTEGER PRIMARY KEY AUTOINCREMENT," +
		"user_id INTEGER NOT NULL," +
		"role_id INTEGER NOT NULL," +
		"create_time INTEGER NOT NULL," +
		"UNIQUE (user_id, role_id)" +
		")",

	"CREATE TABLE IF NOT EXISTS role_namespace (" +
		"id INTEGER PRIMARY KEY AUTOINCREMENT," +
		"role_id INTEGER NOT NULL," +
		"namespace TEXT NOT NULL," +
		"create_time INTEGER NOT NULL," +
		"UNIQUE (role_id, namespace)" +
		")",

	"CREATE TABLE IF NOT EXISTS namespace (" +
		"id INTEGER PRIMARY KEY AUTOINCREMENT," +
		"namespace TEXT NOT NULL," +
		"name TEXT NOT NULL," +
		"description TEXT NOT NULL DEFAULT ''," +
		"create_time INTEGER NOT NULL," +
		"update_time INTEGER NOT NULL," +
		"UNIQUE (namespace)" +
		")",

	"CREATE TABLE IF NOT EXISTS service_instance (" +
		"id INTEGER PRIMARY KEY AUTOINCREMENT," +
		"namespace TEXT NOT NULL DEFAULT 'public'," +
		"group_name TEXT NOT NULL DEFAULT 'DEFAULT_GROUP'," +
		"service_name TEXT NOT NULL," +
		"cluster_name TEXT NOT NULL DEFAULT 'DEFAULT'," +
		"ip TEXT NOT NULL," +
		"port INTEGER NOT NULL," +
		"weight REAL NOT NULL DEFAULT 1," +
		"healthy INTEGER NOT NULL DEFAULT 1," +
		"ephemeral INTEGER NOT NULL DEFAULT 1," +
		"metadata TEXT NOT NULL DEFAULT ''," +
		"last_heartbeat INTEGER NOT NULL," +
		"create_time INTEGER NOT NULL," +
		"update_time INTEGER NOT NULL," +
		"UNIQUE (namespace, group_name, service_name, cluster_name, ip, port)" +
		")",

	"CREATE TABLE IF NOT EXISTS config_info (" +
		"id INTEGER PRIMARY KEY AUTOINCREMENT," +
		"namespace TEXT NOT NULL DEFAULT 'public'," +
		"group_name TEXT NOT NULL DEFAULT 'DEFAULT_GROUP'," +
		"data_id TEXT NOT NULL," +
		"content TEXT NOT NULL," +
		"md5 TEXT NOT NULL," +
		"type TEXT NOT NULL DEFAULT 'text'," +
		"create_time INTEGER NOT NULL," +
		"update_time INTEGER NOT NULL," +
		"UNIQUE (namespace, group_name, data_id)" +
		")",

	"CREATE TABLE IF NOT EXISTS config_draft (" +
		"id INTEGER PRIMARY KEY AUTOINCREMENT," +
		"namespace TEXT NOT NULL DEFAULT 'public'," +
		"group_name TEXT NOT NULL DEFAULT 'DEFAULT_GROUP'," +
		"data_id TEXT NOT NULL," +
		"content TEXT NOT NULL," +
		"md5 TEXT NOT NULL," +
		"type TEXT NOT NULL DEFAULT 'text'," +
		"based_md5 TEXT NOT NULL DEFAULT ''," +
		"operator TEXT NOT NULL DEFAULT ''," +
		"create_time INTEGER NOT NULL," +
		"update_time INTEGER NOT NULL," +
		"UNIQUE (namespace, group_name, data_id)" +
		")",

	"CREATE TABLE IF NOT EXISTS config_history (" +
		"id INTEGER PRIMARY KEY AUTOINCREMENT," +
		"namespace TEXT NOT NULL," +
		"group_name TEXT NOT NULL," +
		"data_id TEXT NOT NULL," +
		"content TEXT NOT NULL," +
		"md5 TEXT NOT NULL," +
		"type TEXT NOT NULL DEFAULT 'text'," +
		"create_time INTEGER NOT NULL" +
		")",

	"CREATE TABLE IF NOT EXISTS change_log (" +
		"seq INTEGER PRIMARY KEY AUTOINCREMENT," +
		"event_type TEXT NOT NULL," +
		"namespace TEXT NOT NULL," +
		"group_name TEXT NOT NULL," +
		"watch_key TEXT NOT NULL," +
		"md5 TEXT NOT NULL DEFAULT ''," +
		"change_time INTEGER NOT NULL" +
		")",

	"CREATE TABLE IF NOT EXISTS subscriber_session (" +
		"id INTEGER PRIMARY KEY AUTOINCREMENT," +
		"node_id TEXT NOT NULL," +
		"namespace TEXT NOT NULL DEFAULT 'public'," +
		"group_name TEXT NOT NULL DEFAULT 'DEFAULT_GROUP'," +
		"config_keys TEXT NOT NULL DEFAULT '[]'," +
		"instance_key TEXT NOT NULL DEFAULT ''," +
		"client_ip TEXT NOT NULL DEFAULT ''," +
		"username TEXT NOT NULL DEFAULT ''," +
		"connected_at INTEGER NOT NULL," +
		"last_heartbeat INTEGER NOT NULL DEFAULT 0" +
		")",

	"CREATE INDEX IF NOT EXISTS idx_subscriber_namespace ON subscriber_session (namespace)",

	"CREATE INDEX IF NOT EXISTS idx_subscriber_node ON subscriber_session (node_id)",

	"CREATE TABLE IF NOT EXISTS cluster_node (" +
		"id INTEGER PRIMARY KEY AUTOINCREMENT," +
		"node_id TEXT NOT NULL," +
		"address TEXT NOT NULL," +
		"status TEXT NOT NULL DEFAULT 'UP'," +
		"last_heartbeat INTEGER NOT NULL," +
		"create_time INTEGER NOT NULL," +
		"UNIQUE (node_id)" +
		")",

	"CREATE TABLE IF NOT EXISTS cluster_leader (" +
		"leader_key TEXT PRIMARY KEY," +
		"node_id TEXT NOT NULL," +
		"lease_until INTEGER NOT NULL," +
		"update_time INTEGER NOT NULL" +
		")",
}
