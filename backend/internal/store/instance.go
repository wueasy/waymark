package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// UpsertInstance 注册或更新实例（按唯一键判断新增/更新），返回是否为新建。
func (s *Store) UpsertInstance(inst *Instance) (bool, error) {
	now, err := s.NowMillis()
	if err != nil {
		return false, err
	}
	inst.LastHeartbeat = now
	inst.UpdateTime = now
	if inst.Metadata == "" {
		inst.Metadata = "{}"
	}

	var id int64
	err = s.db.Get(&id, "SELECT id FROM service_instance WHERE namespace = ? AND group_name = ? AND service_name = ? AND cluster_name = ? AND ip = ? AND port = ?",
		inst.Namespace, inst.GroupName, inst.ServiceName, inst.ClusterName, inst.Ip, inst.Port)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		inst.CreateTime = now
		res, insertErr := s.db.Exec("INSERT INTO service_instance (namespace, group_name, service_name, cluster_name, ip, port, weight, healthy, ephemeral, metadata, last_heartbeat, create_time, update_time) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
			inst.Namespace, inst.GroupName, inst.ServiceName, inst.ClusterName, inst.Ip, inst.Port,
			inst.Weight, inst.Healthy, inst.Ephemeral, inst.Metadata, inst.LastHeartbeat, inst.CreateTime, inst.UpdateTime)
		if insertErr != nil {
			return false, insertErr
		}
		inst.Id, _ = res.LastInsertId()
		return true, nil

	case err != nil:
		return false, err

	default:
		inst.Id = id
		_, err = s.db.Exec("UPDATE service_instance SET weight = ?, healthy = ?, ephemeral = ?, metadata = ?, last_heartbeat = ?, update_time = ? WHERE id = ?",
			inst.Weight, inst.Healthy, inst.Ephemeral, inst.Metadata, inst.LastHeartbeat, inst.UpdateTime, id)
		if err != nil {
			return false, err
		}
		return false, nil
	}
}

// UpdateInstance 更新实例的可变属性。
func (s *Store) UpdateInstance(inst *Instance) error {
	now, err := s.NowMillis()
	if err != nil {
		return err
	}
	res, err := s.db.Exec("UPDATE service_instance SET weight = ?, healthy = ?, ephemeral = ?, metadata = ?, update_time = ? WHERE namespace = ? AND group_name = ? AND service_name = ? AND cluster_name = ? AND ip = ? AND port = ?",
		inst.Weight, inst.Healthy, inst.Ephemeral, inst.Metadata, now,
		inst.Namespace, inst.GroupName, inst.ServiceName, inst.ClusterName, inst.Ip, inst.Port)
	if err != nil {
		return err
	}
	return ensureAffected(res)
}

// DeleteInstance 注销实例，返回是否存在被删除的记录。
func (s *Store) DeleteInstance(namespace, group, service, ip string, port int) (bool, error) {
	res, err := s.db.Exec("DELETE FROM service_instance WHERE namespace = ? AND group_name = ? AND service_name = ? AND ip = ? AND port = ?",
		namespace, group, service, ip, port)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// BeatInstance 更新实例心跳时间。
func (s *Store) BeatInstance(namespace, group, service, ip string, port int) (bool, error) {
	now, err := s.NowMillis()
	if err != nil {
		return false, err
	}
	res, err := s.db.Exec("UPDATE service_instance SET last_heartbeat = ?, update_time = ? WHERE namespace = ? AND group_name = ? AND service_name = ? AND ip = ? AND port = ?",
		now, now, namespace, group, service, ip, port)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// ListInstances 查询服务实例列表。
func (s *Store) ListInstances(namespace, group, service string) ([]Instance, error) {
	list := make([]Instance, 0)
	sqlStr := "SELECT * FROM service_instance WHERE namespace = ?"
	args := []any{namespace}
	if group != "" {
		sqlStr += " AND group_name = ?"
		args = append(args, group)
	}
	if service != "" {
		sqlStr += " AND service_name = ?"
		args = append(args, service)
	}
	sqlStr += " ORDER BY service_name ASC, ip ASC, port ASC"
	err := s.db.Select(&list, sqlStr, args...)
	return list, err
}

// ListServices 分页查询服务概览（服务名 + 实例数 + 健康数），返回列表与总数。
func (s *Store) ListServices(namespace, group string, pageNum, pageSize int) ([]ServiceSummary, int64, error) {
	where := " WHERE namespace = ?"
	args := []any{namespace}
	if group != "" {
		where += " AND group_name = ?"
		args = append(args, group)
	}

	var total int64
	countSql := "SELECT COUNT(1) FROM (SELECT 1 FROM service_instance" + where + " GROUP BY group_name, service_name) AS t"
	if err := s.db.Get(&total, countSql, args...); err != nil {
		return nil, 0, err
	}

	list := make([]ServiceSummary, 0)
	query := "SELECT namespace, group_name, service_name, COUNT(1) AS instance_count, COUNT(CASE WHEN healthy = 1 THEN 1 END) AS healthy_count FROM service_instance" + where +
		" GROUP BY namespace, group_name, service_name ORDER BY group_name ASC, service_name ASC LIMIT ? OFFSET ?"
	queryArgs := append(append([]any{}, args...), pageSize, (pageNum-1)*pageSize)
	if err := s.db.Select(&list, query, queryArgs...); err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

// InstanceKey 服务标识，用于变更通知定位。
type InstanceKey struct {
	Namespace   string `db:"namespace" json:"namespace"`
	GroupName   string `db:"group_name" json:"groupName"`
	ServiceName string `db:"service_name" json:"serviceName"`
}

// ListExpiredServices 查询存在心跳超时实例的服务列表。超时阈值按数据库时钟计算，避免跨机器时钟漂移误判。
func (s *Store) ListExpiredServices(timeout time.Duration) ([]InstanceKey, error) {
	now, err := s.NowMillis()
	if err != nil {
		return nil, err
	}
	list := make([]InstanceKey, 0)
	err = s.db.Select(&list, "SELECT DISTINCT namespace, group_name, service_name FROM service_instance WHERE last_heartbeat < ?",
		now-timeout.Milliseconds())
	return list, err
}

// SweepExpiredInstances 淘汰心跳超时实例：临时实例删除，持久实例标记不健康。
// 超时阈值按数据库时钟计算，避免跨机器时钟漂移误删健康实例。
// 返回 (删除数量, 标记不健康数量)。
func (s *Store) SweepExpiredInstances(timeout time.Duration) (int64, int64, error) {
	now, err := s.NowMillis()
	if err != nil {
		return 0, 0, err
	}
	threshold := now - timeout.Milliseconds()

	res, err := s.db.Exec("DELETE FROM service_instance WHERE ephemeral = 1 AND last_heartbeat < ?", threshold)
	if err != nil {
		return 0, 0, fmt.Errorf("删除过期临时实例失败: %w", err)
	}
	deleted, _ := res.RowsAffected()

	res, err = s.db.Exec("UPDATE service_instance SET healthy = 0, update_time = ? WHERE ephemeral = 0 AND healthy = 1 AND last_heartbeat < ?",
		now, threshold)
	if err != nil {
		return deleted, 0, fmt.Errorf("标记过期持久实例失败: %w", err)
	}
	marked, _ := res.RowsAffected()

	return deleted, marked, nil
}
