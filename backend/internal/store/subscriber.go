package store

// SubscriberFilter 订阅会话查询条件，字段为空表示不限制。
type SubscriberFilter struct {
	NodeId    string
	GroupName string
	Keyword   string
}

// SaveSubscriber 记录一条 SSE 订阅会话，返回自增主键写入 sess.Id。
// 会话落库后集群内任意节点均可查询到，从而实现订阅列表的集群视角。
// 连接时间取数据库时钟，与集群其余时间口径一致，避免跨机器时钟漂移。
func (s *Store) SaveSubscriber(sess *SubscriberSession) error {
	now, err := s.NowMillis()
	if err != nil {
		return err
	}
	sess.ConnectedAt = now
	sess.LastHeartbeat = now
	res, err := s.db.Exec("INSERT INTO subscriber_session (node_id, namespace, group_name, config_keys, instance_key, client_ip, username, connected_at, last_heartbeat) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
		sess.NodeId, sess.Namespace, sess.GroupName, sess.ConfigKeys, sess.InstanceKey, sess.ClientIp, sess.Username, sess.ConnectedAt, sess.LastHeartbeat)
	if err != nil {
		return err
	}
	sess.Id, err = res.LastInsertId()
	return err
}

// UpdateSubscriberHeartbeat 刷新订阅会话的心跳时间（SSE keep-alive 时调用），
// 使订阅列表能够展示连接最后一次心跳距今多久，据此判断连接是否仍然活跃。
func (s *Store) UpdateSubscriberHeartbeat(id int64) error {
	now, err := s.NowMillis()
	if err != nil {
		return err
	}
	_, err = s.db.Exec("UPDATE subscriber_session SET last_heartbeat = ? WHERE id = ?", now, id)
	return err
}

// DeleteSubscriber 删除指定订阅会话（连接断开时调用）。
func (s *Store) DeleteSubscriber(id int64) error {
	_, err := s.db.Exec("DELETE FROM subscriber_session WHERE id = ?", id)
	return err
}

// DeleteSubscribersByNode 删除指定节点的全部订阅会话。
// 节点启动与优雅退出时调用，避免遗留记录出现在订阅列表中。
func (s *Store) DeleteSubscribersByNode(nodeId string) (int64, error) {
	res, err := s.db.Exec("DELETE FROM subscriber_session WHERE node_id = ?", nodeId)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// DeleteOrphanSubscribers 删除所属节点记录已不存在的订阅会话，返回删除数量。
// 仅在节点长时间无心跳、cluster_node 记录被 DeleteStaleNodes 移除后才会命中，
// 避免节点仅短暂心跳失败（进程与 SSE 连接仍存活）时会话被误删。
func (s *Store) DeleteOrphanSubscribers() (int64, error) {
	res, err := s.db.Exec("DELETE FROM subscriber_session WHERE node_id NOT IN (SELECT node_id FROM cluster_node)")
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// ListSubscribers 分页查询指定命名空间下的订阅会话（集群范围），按连接时间倒序，返回列表与总数。
func (s *Store) ListSubscribers(namespace string, filter SubscriberFilter, pageNum, pageSize int) ([]SubscriberSession, int64, error) {
	where := "namespace = ?"
	args := []any{namespace}
	if filter.NodeId != "" {
		where += " AND node_id = ?"
		args = append(args, filter.NodeId)
	}
	if filter.GroupName != "" {
		where += " AND group_name = ?"
		args = append(args, filter.GroupName)
	}
	if filter.Keyword != "" {
		where += " AND (instance_key LIKE ? OR client_ip LIKE ? OR username LIKE ?)"
		like := "%" + filter.Keyword + "%"
		args = append(args, like, like, like)
	}

	var total int64
	if err := s.db.Get(&total, "SELECT COUNT(1) FROM subscriber_session WHERE "+where, args...); err != nil {
		return nil, 0, err
	}

	list := make([]SubscriberSession, 0)
	queryArgs := append(append([]any{}, args...), pageSize, (pageNum-1)*pageSize)
	err := s.db.Select(&list, "SELECT * FROM subscriber_session WHERE "+where+" ORDER BY connected_at DESC, id DESC LIMIT ? OFFSET ?", queryArgs...)
	if err != nil {
		return nil, 0, err
	}
	return list, total, nil
}
