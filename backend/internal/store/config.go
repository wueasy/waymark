package store

import (
	"database/sql"
	"errors"
	"time"

	"github.com/jmoiron/sqlx"
)

// AppendChangeLog 直接追加变更日志（非事务场景，如实例注册）。
func (s *Store) AppendChangeLog(eventType, namespace, group, watchKey, md5 string) error {
	return s.appendChangeLogDB(s.db, eventType, namespace, group, watchKey, md5, time.Now().UnixMilli())
}

// UpsertConfig 发布或更新配置：写主表 + 历史 + 变更日志（同一事务）。
func (s *Store) UpsertConfig(item *ConfigItem) error {
	tx, err := s.db.Beginx()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if err = s.upsertConfigTx(tx, item, time.Now().UnixMilli()); err != nil {
		return err
	}
	return tx.Commit()
}

// PublishDraft 发布草稿：发布配置的同时删除草稿（同一事务）。clearDraft 为 true 时删除草稿记录，
// 强制发布（草稿基于的版本已被他人覆盖）时也会清理草稿。
func (s *Store) PublishDraft(item *ConfigItem, clearDraft bool) error {
	tx, err := s.db.Beginx()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if err = s.upsertConfigTx(tx, item, time.Now().UnixMilli()); err != nil {
		return err
	}
	if clearDraft {
		if _, err = tx.Exec("DELETE FROM config_draft WHERE namespace = ? AND group_name = ? AND data_id = ?",
			item.Namespace, item.GroupName, item.DataId); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// upsertConfigTx 在事务内写入配置主表、历史版本与变更日志。
func (s *Store) upsertConfigTx(tx *sqlx.Tx, item *ConfigItem, now int64) error {
	var id int64
	err := tx.Get(&id, "SELECT id FROM config_info WHERE namespace = ? AND group_name = ? AND data_id = ?",
		item.Namespace, item.GroupName, item.DataId)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		item.CreateTime = now
		item.UpdateTime = now
		if _, err = tx.Exec("INSERT INTO config_info (namespace, group_name, data_id, content, md5, type, create_time, update_time) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
			item.Namespace, item.GroupName, item.DataId, item.Content, item.Md5, item.Type, item.CreateTime, item.UpdateTime); err != nil {
			return err
		}
	case err != nil:
		return err
	default:
		item.UpdateTime = now
		if _, err = tx.Exec("UPDATE config_info SET content = ?, md5 = ?, type = ?, update_time = ? WHERE id = ?",
			item.Content, item.Md5, item.Type, item.UpdateTime, id); err != nil {
			return err
		}
	}

	if _, err = tx.Exec("INSERT INTO config_history (namespace, group_name, data_id, content, md5, type, create_time) VALUES (?, ?, ?, ?, ?, ?, ?)",
		item.Namespace, item.GroupName, item.DataId, item.Content, item.Md5, item.Type, now); err != nil {
		return err
	}

	return s.appendChangeLog(tx, "CONFIG", item.Namespace, item.GroupName, item.DataId, item.Md5, now)
}

// SaveDraft 保存配置草稿（不存在则创建，存在则更新），草稿不写变更日志，不影响下游。
func (s *Store) SaveDraft(draft *ConfigDraft) error {
	now := time.Now().UnixMilli()
	var id int64
	err := s.db.Get(&id, "SELECT id FROM config_draft WHERE namespace = ? AND group_name = ? AND data_id = ?",
		draft.Namespace, draft.GroupName, draft.DataId)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		draft.CreateTime = now
		draft.UpdateTime = now
		res, err := s.db.Exec("INSERT INTO config_draft (namespace, group_name, data_id, content, md5, type, based_md5, operator, create_time, update_time) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
			draft.Namespace, draft.GroupName, draft.DataId, draft.Content, draft.Md5, draft.Type, draft.BasedMd5, draft.Operator, draft.CreateTime, draft.UpdateTime)
		if err != nil {
			return err
		}
		draft.Id, _ = res.LastInsertId()
		return nil
	case err != nil:
		return err
	default:
		draft.Id = id
		draft.UpdateTime = now
		_, err = s.db.Exec("UPDATE config_draft SET content = ?, md5 = ?, type = ?, based_md5 = ?, operator = ?, update_time = ? WHERE id = ?",
			draft.Content, draft.Md5, draft.Type, draft.BasedMd5, draft.Operator, draft.UpdateTime, id)
		return err
	}
}

// GetDraft 查询配置草稿。
func (s *Store) GetDraft(namespace, group, dataId string) (*ConfigDraft, error) {
	var draft ConfigDraft
	err := s.db.Get(&draft, "SELECT * FROM config_draft WHERE namespace = ? AND group_name = ? AND data_id = ?", namespace, group, dataId)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &draft, err
}

// DeleteDraft 删除配置草稿。
func (s *Store) DeleteDraft(namespace, group, dataId string) error {
	_, err := s.db.Exec("DELETE FROM config_draft WHERE namespace = ? AND group_name = ? AND data_id = ?", namespace, group, dataId)
	return err
}

// GetConfig 查询配置项。
func (s *Store) GetConfig(namespace, group, dataId string) (*ConfigItem, error) {
	var item ConfigItem
	err := s.db.Get(&item, "SELECT * FROM config_info WHERE namespace = ? AND group_name = ? AND data_id = ?", namespace, group, dataId)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &item, err
}

// ListConfigs 分页查询配置列表，返回列表与总数；同时联查草稿信息（hasDraft/draftMd5/draftUpdateTime/draftOperator）。
func (s *Store) ListConfigs(namespace, group, dataId string, pageNum, pageSize int) ([]ConfigItem, int64, error) {
	where := " WHERE c.namespace = ?"
	args := []any{namespace}
	if group != "" {
		where += " AND c.group_name = ?"
		args = append(args, group)
	}
	if dataId != "" {
		where += " AND c.data_id LIKE ?"
		args = append(args, "%"+dataId+"%")
	}

	var total int64
	if err := s.db.Get(&total, "SELECT COUNT(1) FROM config_info c"+where, args...); err != nil {
		return nil, 0, err
	}

	list := make([]ConfigItem, 0)
	query := `SELECT c.*,
	CASE WHEN d.id IS NULL THEN 0 ELSE 1 END AS has_draft,
	COALESCE(d.md5, '') AS draft_md5,
	COALESCE(d.update_time, 0) AS draft_update_time,
	COALESCE(d.operator, '') AS draft_operator
	FROM config_info c
	LEFT JOIN config_draft d ON d.namespace = c.namespace AND d.group_name = c.group_name AND d.data_id = c.data_id` +
		where + " ORDER BY c.group_name ASC, c.data_id ASC LIMIT ? OFFSET ?"
	queryArgs := append(append([]any{}, args...), pageSize, (pageNum-1)*pageSize)
	if err := s.db.Select(&list, query, queryArgs...); err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

// DeleteConfig 删除配置并写入变更日志。
func (s *Store) DeleteConfig(namespace, group, dataId string) error {
	now := time.Now().UnixMilli()
	tx, err := s.db.Beginx()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.Exec("DELETE FROM config_info WHERE namespace = ? AND group_name = ? AND data_id = ?", namespace, group, dataId)
	if err != nil {
		return err
	}
	if err := ensureAffected(res); err != nil {
		return err
	}
	// 配置已删除，其草稿失去意义，一并清理避免残留孤儿草稿。
	if _, err := tx.Exec("DELETE FROM config_draft WHERE namespace = ? AND group_name = ? AND data_id = ?",
		namespace, group, dataId); err != nil {
		return err
	}
	if err := s.appendChangeLog(tx, "CONFIG", namespace, group, dataId, "", now); err != nil {
		return err
	}
	return tx.Commit()
}

// ListConfigHistory 分页查询配置历史版本，返回列表与总数。
func (s *Store) ListConfigHistory(namespace, group, dataId string, pageNum, pageSize int) ([]ConfigHistory, int64, error) {
	var total int64
	if err := s.db.Get(&total, "SELECT COUNT(1) FROM config_history WHERE namespace = ? AND group_name = ? AND data_id = ?",
		namespace, group, dataId); err != nil {
		return nil, 0, err
	}

	list := make([]ConfigHistory, 0)
	err := s.db.Select(&list, "SELECT * FROM config_history WHERE namespace = ? AND group_name = ? AND data_id = ? ORDER BY id DESC LIMIT ? OFFSET ?",
		namespace, group, dataId, pageSize, (pageNum-1)*pageSize)
	if err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

// GetConfigHistory 按 id 查询单个历史版本。
func (s *Store) GetConfigHistory(id int64) (*ConfigHistory, error) {
	var item ConfigHistory
	err := s.db.Get(&item, "SELECT * FROM config_history WHERE id = ?", id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &item, err
}

// execer 抽象 *sqlx.DB 与 *sqlx.Tx 的 Exec 能力。
type execer interface {
	Exec(query string, args ...any) (sql.Result, error)
}

// appendChangeLog 在事务内追加变更日志。
func (s *Store) appendChangeLog(tx *sqlx.Tx, eventType, namespace, group, watchKey, md5 string, now int64) error {
	return s.appendChangeLogDB(tx, eventType, namespace, group, watchKey, md5, now)
}

// appendChangeLogDB 追加变更日志，供 SSE 推送与跨节点同步使用。
func (s *Store) appendChangeLogDB(e execer, eventType, namespace, group, watchKey, md5 string, now int64) error {
	_, err := e.Exec("INSERT INTO change_log (event_type, namespace, group_name, watch_key, md5, change_time) VALUES (?, ?, ?, ?, ?, ?)",
		eventType, namespace, group, watchKey, md5, now)
	return err
}

// ListChangeLogSince 查询指定游标之后的变更日志。
func (s *Store) ListChangeLogSince(seq int64, limit int) ([]ChangeLog, error) {
	list := make([]ChangeLog, 0)
	err := s.db.Select(&list, "SELECT * FROM change_log WHERE seq > ? ORDER BY seq ASC LIMIT ?", seq, limit)
	return list, err
}

// MaxChangeLogSeq 返回当前最大变更日志序号。
func (s *Store) MaxChangeLogSeq() (int64, error) {
	var seq sql.NullInt64
	if err := s.db.Get(&seq, "SELECT MAX(seq) FROM change_log"); err != nil {
		return 0, err
	}
	return seq.Int64, nil
}

// CleanChangeLog 清理超出保留期的变更日志。保留期按数据库时钟计算，避免跨机器时钟漂移。
func (s *Store) CleanChangeLog(retention time.Duration) (int64, error) {
	now, err := s.NowMillis()
	if err != nil {
		return 0, err
	}
	res, err := s.db.Exec("DELETE FROM change_log WHERE change_time < ?", now-retention.Milliseconds())
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
