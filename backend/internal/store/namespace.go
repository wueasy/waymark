package store

import (
	"database/sql"
	"errors"
	"time"
)

// ListNamespaces 查询全部命名空间。
func (s *Store) ListNamespaces() ([]Namespace, error) {
	list := make([]Namespace, 0)
	err := s.db.Select(&list, "SELECT * FROM namespace ORDER BY id ASC")
	return list, err
}

// ListNamespacesByUserRole 查询用户通过角色获权的命名空间。
func (s *Store) ListNamespacesByUserRole(userId int64) ([]Namespace, error) {
	list := make([]Namespace, 0)
	err := s.db.Select(&list,
		"SELECT DISTINCT n.* FROM namespace n"+
			" JOIN role_namespace rn ON rn.namespace = n.namespace"+
			" JOIN user_role ur ON ur.role_id = rn.role_id"+
			" WHERE ur.user_id = ? ORDER BY n.id ASC",
		userId)
	return list, err
}

// GetNamespace 按标识查询命名空间。
func (s *Store) GetNamespace(namespace string) (*Namespace, error) {
	var n Namespace
	err := s.db.Get(&n, "SELECT * FROM namespace WHERE namespace = ?", namespace)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &n, err
}

// CreateNamespace 新增命名空间。
func (s *Store) CreateNamespace(n *Namespace) error {
	now := time.Now().UnixMilli()
	n.CreateTime = now
	n.UpdateTime = now
	res, err := s.db.Exec("INSERT INTO namespace (namespace, name, description, create_time, update_time) VALUES (?, ?, ?, ?, ?)",
		n.Namespace, n.Name, n.Description, n.CreateTime, n.UpdateTime)
	if err != nil {
		return err
	}
	n.Id, _ = res.LastInsertId()
	return nil
}

// UpdateNamespace 更新命名空间显示信息，标识本身不可修改。
func (s *Store) UpdateNamespace(n *Namespace) error {
	res, err := s.db.Exec("UPDATE namespace SET name = ?, description = ?, update_time = ? WHERE namespace = ?",
		n.Name, n.Description, time.Now().UnixMilli(), n.Namespace)
	if err != nil {
		return err
	}
	return ensureAffected(res)
}

// DeleteNamespace 删除命名空间及其角色授权关系。
func (s *Store) DeleteNamespace(namespace string) error {
	tx, err := s.db.Beginx()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec("DELETE FROM role_namespace WHERE namespace = ?", namespace); err != nil {
		return err
	}
	res, err := tx.Exec("DELETE FROM namespace WHERE namespace = ?", namespace)
	if err != nil {
		return err
	}
	if err := ensureAffected(res); err != nil {
		return err
	}
	return tx.Commit()
}
