package store

import (
	"database/sql"
	"errors"
	"time"

	"github.com/jmoiron/sqlx"
)

// ListRoles 查询全部角色，并填充各自授权的命名空间。
func (s *Store) ListRoles() ([]Role, error) {
	list := make([]Role, 0)
	if err := s.db.Select(&list, "SELECT * FROM role ORDER BY builtin DESC, id ASC"); err != nil {
		return nil, err
	}
	return s.fillRoleNamespaces(list)
}

// ListRolesByUserId 查询用户拥有的角色。
func (s *Store) ListRolesByUserId(userId int64) ([]Role, error) {
	list := make([]Role, 0)
	err := s.db.Select(&list,
		"SELECT r.* FROM role r JOIN user_role ur ON ur.role_id = r.id WHERE ur.user_id = ? ORDER BY r.builtin DESC, r.id ASC",
		userId)
	if err != nil {
		return nil, err
	}
	return s.fillRoleNamespaces(list)
}

// GetRoleById 按主键查询角色。
func (s *Store) GetRoleById(id int64) (*Role, error) {
	var r Role
	err := s.db.Get(&r, "SELECT * FROM role WHERE id = ?", id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return s.getRoleNamespaces(&r)
}

// GetRoleByCode 按标识查询角色。
func (s *Store) GetRoleByCode(code string) (*Role, error) {
	var r Role
	err := s.db.Get(&r, "SELECT * FROM role WHERE code = ?", code)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return s.getRoleNamespaces(&r)
}

// CreateRole 新增角色并写入命名空间授权。
func (s *Store) CreateRole(r *Role) error {
	now := time.Now().UnixMilli()
	r.CreateTime = now
	r.UpdateTime = now
	tx, err := s.db.Beginx()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.Exec(
		"INSERT INTO role (code, name, description, permission, builtin, create_time, update_time) VALUES (?, ?, ?, ?, ?, ?, ?)",
		r.Code, r.Name, r.Description, r.Permission, BuiltinNo, r.CreateTime, r.UpdateTime)
	if err != nil {
		return err
	}
	r.Id, _ = res.LastInsertId()
	if err := replaceRoleNamespaces(tx, r.Id, r.Namespaces); err != nil {
		return err
	}
	return tx.Commit()
}

// UpdateRole 更新角色名称、描述、权限等级与命名空间授权，角色标识不可修改。
func (s *Store) UpdateRole(r *Role) error {
	tx, err := s.db.Beginx()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.Exec("UPDATE role SET name = ?, description = ?, permission = ?, update_time = ? WHERE id = ?",
		r.Name, r.Description, r.Permission, time.Now().UnixMilli(), r.Id)
	if err != nil {
		return err
	}
	if err := ensureAffected(res); err != nil {
		return err
	}
	if err := replaceRoleNamespaces(tx, r.Id, r.Namespaces); err != nil {
		return err
	}
	return tx.Commit()
}

// DeleteRole 删除角色及其命名空间授权与用户关联。
func (s *Store) DeleteRole(id int64) error {
	tx, err := s.db.Beginx()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	for _, stmt := range []string{
		"DELETE FROM role_namespace WHERE role_id = ?",
		"DELETE FROM user_role WHERE role_id = ?",
	} {
		if _, err := tx.Exec(stmt, id); err != nil {
			return err
		}
	}
	res, err := tx.Exec("DELETE FROM role WHERE id = ?", id)
	if err != nil {
		return err
	}
	if err := ensureAffected(res); err != nil {
		return err
	}
	return tx.Commit()
}

// ListUserRoleIds 查询用户已分配的角色 id。
func (s *Store) ListUserRoleIds(userId int64) ([]int64, error) {
	list := make([]int64, 0)
	err := s.db.Select(&list, "SELECT role_id FROM user_role WHERE user_id = ? ORDER BY role_id ASC", userId)
	return list, err
}

// SetUserRoles 覆盖式设置用户的角色。
func (s *Store) SetUserRoles(userId int64, roleIds []int64) error {
	tx, err := s.db.Beginx()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec("DELETE FROM user_role WHERE user_id = ?", userId); err != nil {
		return err
	}
	now := time.Now().UnixMilli()
	seen := make(map[int64]struct{}, len(roleIds))
	for _, roleId := range roleIds {
		if roleId <= 0 {
			continue
		}
		if _, ok := seen[roleId]; ok {
			continue
		}
		seen[roleId] = struct{}{}
		if _, err := tx.Exec("INSERT INTO user_role (user_id, role_id, create_time) VALUES (?, ?, ?)", userId, roleId, now); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// NamespacePermissions 汇总用户各角色在命名空间上的有效权限，同一命名空间取最高等级（write > read）。
func (s *Store) NamespacePermissions(userId int64) (map[string]string, error) {
	rows, err := s.db.Queryx(
		"SELECT rn.namespace, r.permission FROM role_namespace rn"+
			" JOIN role r ON r.id = rn.role_id"+
			" JOIN user_role ur ON ur.role_id = r.id"+
			" WHERE ur.user_id = ?", userId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	perms := make(map[string]string)
	for rows.Next() {
		var namespace, permission string
		if err := rows.Scan(&namespace, &permission); err != nil {
			return nil, err
		}
		if perms[namespace] == PermissionWrite {
			continue
		}
		perms[namespace] = permission
	}
	return perms, rows.Err()
}

// getRoleNamespaces 为单个角色填充命名空间授权。
func (s *Store) getRoleNamespaces(r *Role) (*Role, error) {
	list, err := s.fillRoleNamespaces([]Role{*r})
	if err != nil {
		return nil, err
	}
	return &list[0], nil
}

// fillRoleNamespaces 批量填充角色的命名空间授权，避免逐条查询。
func (s *Store) fillRoleNamespaces(list []Role) ([]Role, error) {
	if len(list) == 0 {
		return list, nil
	}
	index := make(map[int64]int, len(list))
	for i := range list {
		list[i].Namespaces = make([]string, 0)
		index[list[i].Id] = i
	}
	rows, err := s.db.Queryx("SELECT role_id, namespace FROM role_namespace ORDER BY namespace ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var roleId int64
		var namespace string
		if err := rows.Scan(&roleId, &namespace); err != nil {
			return nil, err
		}
		if i, ok := index[roleId]; ok {
			list[i].Namespaces = append(list[i].Namespaces, namespace)
		}
	}
	return list, rows.Err()
}

// replaceRoleNamespaces 覆盖式重写角色的命名空间授权。
func replaceRoleNamespaces(tx *sqlx.Tx, roleId int64, namespaces []string) error {
	if _, err := tx.Exec("DELETE FROM role_namespace WHERE role_id = ?", roleId); err != nil {
		return err
	}
	now := time.Now().UnixMilli()
	seen := make(map[string]struct{}, len(namespaces))
	for _, namespace := range namespaces {
		if namespace == "" {
			continue
		}
		if _, ok := seen[namespace]; ok {
			continue
		}
		seen[namespace] = struct{}{}
		if _, err := tx.Exec("INSERT INTO role_namespace (role_id, namespace, create_time) VALUES (?, ?, ?)", roleId, namespace, now); err != nil {
			return err
		}
	}
	return nil
}
