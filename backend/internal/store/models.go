package store

// 内置角色标识。内置 admin 角色固定拥有全部命名空间读写权限，不可编辑或删除。
const RoleCodeAdmin = "admin"

// 角色权限等级。
const (
	PermissionRead  = "read"  // 只读
	PermissionWrite = "write" // 读写
)

// 角色内置标记。
const (
	BuiltinYes = 1
	BuiltinNo  = 0
)

// 节点状态。
const (
	NodeStatusUp   = "UP"
	NodeStatusDown = "DOWN"
)

// 默认命名空间与分组。
const (
	DefaultNamespace = "public"
	DefaultGroup     = "DEFAULT_GROUP"
	DefaultCluster   = "DEFAULT"
)

// User 用户。角色通过 user_role 关联，一个用户可拥有多个角色。
type User struct {
	Id         int64  `db:"id" json:"id"`
	Username   string `db:"username" json:"username"`
	Password   string `db:"password" json:"-"`
	Nickname   string `db:"nickname" json:"nickname"`
	Status     int    `db:"status" json:"status"`
	CreateTime int64  `db:"create_time" json:"createTime"`
	UpdateTime int64  `db:"update_time" json:"updateTime"`
}

// Role 角色。角色持有一个权限等级，并授权若干命名空间。
type Role struct {
	Id          int64    `db:"id" json:"id"`
	Code        string   `db:"code" json:"code"`
	Name        string   `db:"name" json:"name"`
	Description string   `db:"description" json:"description"`
	Permission  string   `db:"permission" json:"permission"`
	Builtin     int      `db:"builtin" json:"builtin"`
	CreateTime  int64    `db:"create_time" json:"createTime"`
	UpdateTime  int64    `db:"update_time" json:"updateTime"`
	Namespaces  []string `db:"-" json:"namespaces"`
}

// Namespace 命名空间。
type Namespace struct {
	Id          int64  `db:"id" json:"id"`
	Namespace   string `db:"namespace" json:"namespace"`
	Name        string `db:"name" json:"name"`
	Description string `db:"description" json:"description"`
	CreateTime  int64  `db:"create_time" json:"createTime"`
	UpdateTime  int64  `db:"update_time" json:"updateTime"`
}

// Instance 服务实例。
type Instance struct {
	Id            int64   `db:"id" json:"id"`
	Namespace     string  `db:"namespace" json:"namespace"`
	GroupName     string  `db:"group_name" json:"groupName"`
	ServiceName   string  `db:"service_name" json:"serviceName"`
	ClusterName   string  `db:"cluster_name" json:"clusterName"`
	Ip            string  `db:"ip" json:"ip"`
	Port          int     `db:"port" json:"port"`
	Weight        float64 `db:"weight" json:"weight"`
	Healthy       int     `db:"healthy" json:"healthy"`
	Ephemeral     int     `db:"ephemeral" json:"ephemeral"`
	Metadata      string  `db:"metadata" json:"metadata"`
	LastHeartbeat int64   `db:"last_heartbeat" json:"lastHeartbeat"`
	CreateTime    int64   `db:"create_time" json:"createTime"`
	UpdateTime    int64   `db:"update_time" json:"updateTime"`
}

// ConfigItem 配置项。
type ConfigItem struct {
	Id         int64  `db:"id" json:"id"`
	Namespace  string `db:"namespace" json:"namespace"`
	GroupName  string `db:"group_name" json:"groupName"`
	DataId     string `db:"data_id" json:"dataId"`
	Content    string `db:"content" json:"content"`
	Md5        string `db:"md5" json:"md5"`
	Type       string `db:"type" json:"type"`
	CreateTime int64  `db:"create_time" json:"createTime"`
	UpdateTime int64  `db:"update_time" json:"updateTime"`

	// 以下字段仅在列表联查草稿时填充（不落 config_info 表）。
	HasDraft        int    `db:"has_draft" json:"hasDraft"`
	DraftMd5        string `db:"draft_md5" json:"draftMd5"`
	DraftUpdateTime int64  `db:"draft_update_time" json:"draftUpdateTime"`
	DraftOperator   string `db:"draft_operator" json:"draftOperator"`
}

// ConfigDraft 配置草稿。编辑态数据，发布前不影响下游（不写变更日志）。
type ConfigDraft struct {
	Id         int64  `db:"id" json:"id"`
	Namespace  string `db:"namespace" json:"namespace"`
	GroupName  string `db:"group_name" json:"groupName"`
	DataId     string `db:"data_id" json:"dataId"`
	Content    string `db:"content" json:"content"`
	Md5        string `db:"md5" json:"md5"`
	Type       string `db:"type" json:"type"`
	BasedMd5   string `db:"based_md5" json:"basedMd5"` // 草稿基于的已发布版本摘要，用于冲突检测
	Operator   string `db:"operator" json:"operator"`
	CreateTime int64  `db:"create_time" json:"createTime"`
	UpdateTime int64  `db:"update_time" json:"updateTime"`
}

// ConfigHistory 配置历史版本。
type ConfigHistory struct {
	Id         int64  `db:"id" json:"id"`
	Namespace  string `db:"namespace" json:"namespace"`
	GroupName  string `db:"group_name" json:"groupName"`
	DataId     string `db:"data_id" json:"dataId"`
	Content    string `db:"content" json:"content"`
	Md5        string `db:"md5" json:"md5"`
	Type       string `db:"type" json:"type"`
	CreateTime int64  `db:"create_time" json:"createTime"`
}

// ChangeLog 变更日志，用于 SSE 推送与跨节点同步。
type ChangeLog struct {
	Seq        int64  `db:"seq" json:"seq"`
	EventType  string `db:"event_type" json:"eventType"`
	Namespace  string `db:"namespace" json:"namespace"`
	GroupName  string `db:"group_name" json:"groupName"`
	WatchKey   string `db:"watch_key" json:"watchKey"`
	Md5        string `db:"md5" json:"md5"`
	ChangeTime int64  `db:"change_time" json:"changeTime"`
}

// SubscriberSession SSE 订阅会话。一条连接一行，落库后集群内各节点均可查询，
// 连接断开时删除；节点异常退出遗留的记录由 Leader 依据节点状态清理。
type SubscriberSession struct {
	Id          int64  `db:"id" json:"id"`
	NodeId      string `db:"node_id" json:"nodeId"`
	Namespace   string `db:"namespace" json:"namespace"`
	GroupName   string `db:"group_name" json:"group"`
	ConfigKeys  string `db:"config_keys" json:"-"` // JSON 数组文本，响应时转换为 []string
	InstanceKey string `db:"instance_key" json:"instanceKey"`
	ClientIp    string `db:"client_ip" json:"clientIp"`
	Username    string `db:"username" json:"username"`
	ConnectedAt int64  `db:"connected_at" json:"connectedAt"`
	// LastHeartbeat 最后一次 SSE keep-alive 心跳时间（毫秒），用于判断连接是否仍然活跃。
	LastHeartbeat int64 `db:"last_heartbeat" json:"lastHeartbeat"`
}

// ClusterNode 集群节点。
type ClusterNode struct {
	Id            int64  `db:"id" json:"id"`
	NodeId        string `db:"node_id" json:"nodeId"`
	Address       string `db:"address" json:"address"`
	Status        string `db:"status" json:"status"`
	LastHeartbeat int64  `db:"last_heartbeat" json:"lastHeartbeat"`
	CreateTime    int64  `db:"create_time" json:"createTime"`
}

// ClusterLeader Leader 选举租约。
type ClusterLeader struct {
	LeaderKey  string `db:"leader_key" json:"leaderKey"`
	NodeId     string `db:"node_id" json:"nodeId"`
	LeaseUntil int64  `db:"lease_until" json:"leaseUntil"`
	UpdateTime int64  `db:"update_time" json:"updateTime"`
}

// ServiceSummary 服务概览。
type ServiceSummary struct {
	Namespace     string `db:"namespace" json:"namespace"`
	GroupName     string `db:"group_name" json:"groupName"`
	ServiceName   string `db:"service_name" json:"serviceName"`
	InstanceCount int    `db:"instance_count" json:"instanceCount"`
	HealthyCount  int    `db:"healthy_count" json:"healthyCount"`
}
