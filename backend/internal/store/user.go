package store

import (
	"database/sql"
	"errors"
	"time"
)

// userColumns 显式列清单，避免依赖表字段顺序。
const userColumns = "id, username, password, nickname, status, create_time, update_time"

// CountUsers 统计用户数量，用于判断是否需要初始化。
func (s *Store) CountUsers() (int, error) {
	var cnt int
	err := s.db.Get(&cnt, "SELECT COUNT(1) FROM user")
	return cnt, err
}

// GetUserByUsername 按用户名查询用户。
func (s *Store) GetUserByUsername(username string) (*User, error) {
	var u User
	err := s.db.Get(&u, "SELECT "+userColumns+" FROM user WHERE username = ?", username)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &u, err
}

// GetUserById 按主键查询用户。
func (s *Store) GetUserById(id int64) (*User, error) {
	var u User
	err := s.db.Get(&u, "SELECT "+userColumns+" FROM user WHERE id = ?", id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &u, err
}

// ListUsers 查询全部用户。
func (s *Store) ListUsers() ([]User, error) {
	list := make([]User, 0)
	err := s.db.Select(&list, "SELECT "+userColumns+" FROM user ORDER BY id ASC")
	return list, err
}

// CreateUser 新增用户。
func (s *Store) CreateUser(u *User) error {
	now := time.Now().UnixMilli()
	u.CreateTime = now
	u.UpdateTime = now
	res, err := s.db.Exec("INSERT INTO user (username, password, nickname, status, create_time, update_time) VALUES (?, ?, ?, ?, ?, ?)",
		u.Username, u.Password, u.Nickname, u.Status, u.CreateTime, u.UpdateTime)
	if err != nil {
		return err
	}
	u.Id, _ = res.LastInsertId()
	return nil
}

// UpdateUser 更新用户基础信息（不含密码与角色）。
func (s *Store) UpdateUser(u *User) error {
	res, err := s.db.Exec("UPDATE user SET nickname = ?, status = ?, update_time = ? WHERE id = ?",
		u.Nickname, u.Status, time.Now().UnixMilli(), u.Id)
	if err != nil {
		return err
	}
	return ensureAffected(res)
}

// UpdateUserPassword 更新密码。
func (s *Store) UpdateUserPassword(id int64, password string) error {
	res, err := s.db.Exec("UPDATE user SET password = ?, update_time = ? WHERE id = ?",
		password, time.Now().UnixMilli(), id)
	if err != nil {
		return err
	}
	return ensureAffected(res)
}

// DeleteUser 删除用户及其角色关联。
func (s *Store) DeleteUser(id int64) error {
	tx, err := s.db.Beginx()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec("DELETE FROM user_role WHERE user_id = ?", id); err != nil {
		return err
	}
	res, err := tx.Exec("DELETE FROM user WHERE id = ?", id)
	if err != nil {
		return err
	}
	if err := ensureAffected(res); err != nil {
		return err
	}
	return tx.Commit()
}

func ensureAffected(res sql.Result) error {
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
