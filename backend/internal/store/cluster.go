package store

import (
	"database/sql"
	"errors"
	"time"
)

// UpsertNode 注册节点并刷新心跳。记录不存在时插入，因此节点被清理后再次心跳会自动重新注册。
func (s *Store) UpsertNode(nodeId, address string) error {
	now, err := s.NowMillis()
	if err != nil {
		return err
	}
	stmt := s.insertIgnoreKeyword() + " INTO cluster_node (node_id, address, status, last_heartbeat, create_time) VALUES (?, ?, ?, ?, ?)"
	if _, err := s.db.Exec(stmt, nodeId, address, NodeStatusUp, now, now); err != nil {
		return err
	}
	_, err = s.db.Exec("UPDATE cluster_node SET address = ?, status = ?, last_heartbeat = ? WHERE node_id = ?",
		address, NodeStatusUp, now, nodeId)
	return err
}

// ListNodes 查询全部节点。
func (s *Store) ListNodes() ([]ClusterNode, error) {
	list := make([]ClusterNode, 0)
	err := s.db.Select(&list, "SELECT * FROM cluster_node ORDER BY id ASC")
	return list, err
}

// MarkNodesDown 将心跳超时的节点标记为 DOWN。超时阈值按数据库时钟计算，避免跨机器时钟漂移误判。
func (s *Store) MarkNodesDown(timeout time.Duration) (int64, error) {
	now, err := s.NowMillis()
	if err != nil {
		return 0, err
	}
	threshold := now - timeout.Milliseconds()
	res, err := s.db.Exec("UPDATE cluster_node SET status = ? WHERE status = ? AND last_heartbeat < ?",
		NodeStatusDown, NodeStatusUp, threshold)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// DeleteStaleNodes 移除长时间无心跳的节点记录，返回删除数量。
// 保留时长应显著大于 node-timeout：节点先被标记 DOWN，超过保留期后才移除。
// 超时阈值按数据库时钟计算，避免跨机器时钟漂移误删。
func (s *Store) DeleteStaleNodes(timeout time.Duration) (int64, error) {
	now, err := s.NowMillis()
	if err != nil {
		return 0, err
	}
	res, err := s.db.Exec("DELETE FROM cluster_node WHERE last_heartbeat < ?", now-timeout.Milliseconds())
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// TryAcquireLeader 尝试抢占已过期的 Leader 租约，返回是否抢占成功。
// 时间以数据库时钟为准，避免机器时钟超前时提前抢占未过期租约。
func (s *Store) TryAcquireLeader(nodeId string, ttl time.Duration) (bool, error) {
	now, err := s.NowMillis()
	if err != nil {
		return false, err
	}
	res, err := s.db.Exec("UPDATE cluster_leader SET node_id = ?, lease_until = ?, update_time = ? WHERE leader_key = ? AND lease_until < ?",
		nodeId, now+ttl.Milliseconds(), now, "default", now)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// RenewLeader 续租（仍持有租约时）。返回是否续租成功。
func (s *Store) RenewLeader(nodeId string, ttl time.Duration) (bool, error) {
	now, err := s.NowMillis()
	if err != nil {
		return false, err
	}
	res, err := s.db.Exec("UPDATE cluster_leader SET lease_until = ?, update_time = ? WHERE leader_key = ? AND node_id = ?",
		now+ttl.Milliseconds(), now, "default", nodeId)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// GetLeader 查询当前 Leader 租约。
func (s *Store) GetLeader() (*ClusterLeader, error) {
	var l ClusterLeader
	err := s.db.Get(&l, "SELECT * FROM cluster_leader WHERE leader_key = ?", "default")
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &l, err
}
