package cluster

import (
	"context"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	wlog "github.com/wueasy/wueasy-go-tools/log"

	"waymark/internal/config"
	"waymark/internal/registry"
	"waymark/internal/store"
)

// 心跳检查间隔下限，避免配置过小造成数据库压力。
const minHeartbeatInterval = 2 * time.Second

// Cluster 集群节点管理：节点注册与心跳、Leader 选举、后台维护任务。
type Cluster struct {
	cfg      *config.Config
	store    *store.Store
	registry *registry.Service
	nodeId   string
	address  string
	leader   atomic.Bool
}

// New 创建集群实例。
// 节点 IP 与端口优先取 cluster.ip / cluster.port，未配置时分别回退到本机探测 IP 与 server.port。
func New(cfg *config.Config, st *store.Store, reg *registry.Service) *Cluster {
	port := cfg.Port()
	if cfg.Cluster.Port > 0 {
		port = cfg.Cluster.Port
	}
	ip := cfg.Cluster.IP
	if ip == "" {
		ip = localIP()
	}
	address := net.JoinHostPort(ip, strconv.Itoa(port))

	nodeId := cfg.Cluster.NodeID
	if nodeId == "" {
		nodeId = defaultNodeID(port)
	}
	return &Cluster{
		cfg:      cfg,
		store:    st,
		registry: reg,
		nodeId:   nodeId,
		address:  address,
	}
}

// NodeId 当前节点标识。
func (c *Cluster) NodeId() string { return c.nodeId }

// Address 当前节点地址。
func (c *Cluster) Address() string { return c.address }

// IsLeader 当前节点是否为 Leader。
func (c *Cluster) IsLeader() bool { return c.leader.Load() }

// Start 注册节点并启动心跳、选举与维护循环。
func (c *Cluster) Start(ctx context.Context) {
	if err := c.store.UpsertNode(c.nodeId, c.address); err != nil {
		wlog.Ctx(ctx).Warnf("[cluster] 注册节点失败: %v", err)
	}
	if !c.cfg.Cluster.Enabled {
		c.leader.Store(true)
		wlog.Ctx(ctx).Infof("[cluster] 单机模式，当前节点承担全部维护任务，nodeId=%s", c.nodeId)
	} else {
		wlog.Ctx(ctx).Infof("[cluster] 集群模式启动，nodeId=%s address=%s", c.nodeId, c.address)
		c.electionOnce(ctx)
	}

	go c.heartbeatLoop(ctx)
	go c.electionLoop(ctx)
	go c.maintenanceLoop(ctx)
}

// heartbeatLoop 周期刷新节点心跳。使用 UpsertNode，节点记录被清理后仍能自动重新注册。
func (c *Cluster) heartbeatLoop(ctx context.Context) {
	interval := time.Duration(c.cfg.Cluster.NodeTimeout) * time.Second / 3
	if interval < minHeartbeatInterval {
		interval = minHeartbeatInterval
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := c.store.UpsertNode(c.nodeId, c.address); err != nil {
				wlog.Ctx(ctx).Warnf("[cluster] 节点心跳失败: %v", err)
			}
		}
	}
}

// electionLoop 周期执行 Leader 租约抢占或续租。
func (c *Cluster) electionLoop(ctx context.Context) {
	if !c.cfg.Cluster.Enabled {
		return
	}
	interval := time.Duration(c.cfg.Cluster.ElectionInterval) * time.Second
	if interval <= 0 {
		interval = 3 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.electionOnce(ctx)
		}
	}
}

// electionOnce 执行一次选举：租约过期则抢占，仍持有则续租。
// 租约时间统一由数据库时钟生成，避免各机器本地时钟漂移导致提前抢占或续租失败。
func (c *Cluster) electionOnce(ctx context.Context) {
	ttl := time.Duration(c.cfg.Cluster.LeaseTTL) * time.Second
	if ttl <= 0 {
		ttl = 10 * time.Second
	}

	acquired, err := c.store.TryAcquireLeader(c.nodeId, ttl)
	if err != nil {
		wlog.Ctx(ctx).Warnf("[cluster] 抢占 Leader 租约失败: %v", err)
		return
	}
	if acquired {
		c.setLeader(ctx, true)
		return
	}

	renewed, err := c.store.RenewLeader(c.nodeId, ttl)
	if err != nil {
		wlog.Ctx(ctx).Warnf("[cluster] 续租 Leader 租约失败: %v", err)
		return
	}
	c.setLeader(ctx, renewed)
}

// setLeader 更新本地 Leader 状态，仅在状态变化时输出日志。
func (c *Cluster) setLeader(ctx context.Context, leader bool) {
	if c.leader.Swap(leader) != leader {
		if leader {
			wlog.Ctx(ctx).Infof("[cluster] 当前节点成为 Leader: %s", c.nodeId)
		} else {
			wlog.Ctx(ctx).Infof("[cluster] 当前节点失去 Leader 状态: %s", c.nodeId)
		}
	}
}

// maintenanceLoop 仅由 Leader 执行的后台维护任务。
func (c *Cluster) maintenanceLoop(ctx context.Context) {
	interval := time.Duration(c.cfg.Registry.SweepInterval) * time.Second
	if interval <= 0 {
		interval = 10 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !c.leader.Load() {
				continue
			}
			c.maintain(ctx)
		}
	}
}

func (c *Cluster) maintain(ctx context.Context) {
	timeout := time.Duration(c.cfg.Registry.HeartbeatTimeout) * time.Second
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	c.registry.Sweep(ctx, timeout)

	nodeTimeout := time.Duration(c.cfg.Cluster.NodeTimeout) * time.Second
	if nodeTimeout <= 0 {
		nodeTimeout = 30 * time.Second
	}
	if n, err := c.store.MarkNodesDown(nodeTimeout); err != nil {
		wlog.Ctx(ctx).Warnf("[cluster] 标记下线节点失败: %v", err)
	} else if n > 0 {
		wlog.Ctx(ctx).Infof("[cluster] 标记 %d 个节点为 DOWN", n)
	}

	nodeRetain := time.Duration(c.cfg.Cluster.NodeRetain) * time.Second
	if nodeRetain <= 0 {
		nodeRetain = 5 * time.Minute
	}
	if n, err := c.store.DeleteStaleNodes(nodeRetain); err != nil {
		wlog.Ctx(ctx).Warnf("[cluster] 移除失效节点失败: %v", err)
	} else if n > 0 {
		wlog.Ctx(ctx).Infof("[cluster] 移除 %d 个长时间无心跳节点", n)
	}

	retention := time.Duration(c.cfg.SSE.LogRetention) * time.Second
	if retention > 0 {
		if n, err := c.store.CleanChangeLog(retention); err != nil {
			wlog.Ctx(ctx).Warnf("[cluster] 清理变更日志失败: %v", err)
		} else if n > 0 {
			wlog.Ctx(ctx).Infof("[cluster] 清理过期变更日志 %d 条", n)
		}
	}
}

// defaultNodeID 生成默认节点标识：主机名-网卡 MAC-端口。
// 跨机器部署时不同机器的网卡 MAC 不同，可避免仅用「主机名-端口」在机器同名时节点标识冲突，
// 进而导致 cluster_node 唯一键撞行、甚至同 node_id 互相续租造成双 Leader。
func defaultNodeID(port int) string {
	host, err := os.Hostname()
	if err != nil || host == "" {
		host = "waymark"
	}
	id := host
	if mac := primaryMAC(); mac != "" {
		id += "-" + mac
	}
	return id + "-" + strconv.Itoa(port)
}

// primaryMAC 返回首个已启用物理网卡的硬件地址（去掉分隔符），失败返回空串。
func primaryMAC() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}
		if len(iface.HardwareAddr) == 0 {
			continue
		}
		return strings.ToUpper(strings.ReplaceAll(iface.HardwareAddr.String(), ":", ""))
	}
	return ""
}

// localIP 探测本机首个非回环 IPv4 地址。
func localIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "127.0.0.1"
	}
	for _, addr := range addrs {
		ipNet, ok := addr.(*net.IPNet)
		if !ok || ipNet.IP.IsLoopback() {
			continue
		}
		if v4 := ipNet.IP.To4(); v4 != nil {
			return v4.String()
		}
	}
	return "127.0.0.1"
}

// LeaderInfo 当前 Leader 概要信息。
type LeaderInfo struct {
	NodeId     string `json:"nodeId"`
	LeaseUntil int64  `json:"leaseUntil"`
	IsSelf     bool   `json:"isSelf"`
}

// Leader 返回当前 Leader 信息。单机模式下当前节点即为 Leader。
func (c *Cluster) Leader() (*LeaderInfo, error) {
	l, err := c.store.GetLeader()
	if err != nil {
		return nil, fmt.Errorf("查询 Leader 失败: %w", err)
	}
	if !c.cfg.Cluster.Enabled {
		return &LeaderInfo{NodeId: c.nodeId, LeaseUntil: l.LeaseUntil, IsSelf: true}, nil
	}
	return &LeaderInfo{NodeId: l.NodeId, LeaseUntil: l.LeaseUntil, IsSelf: l.NodeId == c.nodeId}, nil
}
