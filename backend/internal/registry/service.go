package registry

import (
	"context"
	"encoding/json"
	"time"

	wlog "github.com/wueasy/wueasy-go-tools/log"

	"waymark/internal/event"
	"waymark/internal/store"
)

// Service 注册中心：实例注册、心跳、注销、查询与变更通知。
type Service struct {
	store *store.Store
}

// New 创建注册中心服务。
func New(st *store.Store) *Service {
	return &Service{store: st}
}

// Register 注册或更新实例，返回是否为新建。
func (s *Service) Register(inst *store.Instance) (bool, error) {
	created, err := s.store.UpsertInstance(inst)
	if err != nil {
		return false, err
	}
	s.notify(inst.Namespace, inst.GroupName, inst.ServiceName)
	return created, nil
}

// Update 更新实例可变属性。
func (s *Service) Update(inst *store.Instance) error {
	if err := s.store.UpdateInstance(inst); err != nil {
		return err
	}
	s.notify(inst.Namespace, inst.GroupName, inst.ServiceName)
	return nil
}

// Deregister 注销实例。
func (s *Service) Deregister(namespace, group, service, ip string, port int) error {
	removed, err := s.store.DeleteInstance(namespace, group, service, ip, port)
	if err != nil {
		return err
	}
	if removed {
		s.notify(namespace, group, service)
	}
	return nil
}

// Beat 更新实例心跳。
func (s *Service) Beat(namespace, group, service, ip string, port int) (bool, error) {
	return s.store.BeatInstance(namespace, group, service, ip, port)
}

// ListInstances 查询实例列表。
func (s *Service) ListInstances(namespace, group, service string) ([]store.Instance, error) {
	return s.store.ListInstances(namespace, group, service)
}

// ListServices 分页查询服务概览，返回列表与总数。
func (s *Service) ListServices(namespace, group string, pageNum, pageSize int) ([]store.ServiceSummary, int64, error) {
	return s.store.ListServices(namespace, group, pageNum, pageSize)
}

// Sweep 淘汰心跳超时实例，并通知受影响的服务（由 Leader 节点执行）。
func (s *Service) Sweep(ctx context.Context, timeout time.Duration) {
	affected, err := s.store.ListExpiredServices(timeout)
	if err != nil {
		wlog.Ctx(ctx).Warnf("[registry] 查询超时实例失败: %v", err)
		return
	}
	if len(affected) == 0 {
		return
	}
	deleted, marked, err := s.store.SweepExpiredInstances(timeout)
	if err != nil {
		wlog.Ctx(ctx).Warnf("[registry] 淘汰超时实例失败: %v", err)
		return
	}
	if deleted == 0 && marked == 0 {
		return
	}
	for _, k := range affected {
		s.notify(k.Namespace, k.GroupName, k.ServiceName)
	}
	wlog.Ctx(ctx).Infof("[registry] 淘汰超时实例完成，删除 %d 个，标记不健康 %d 个", deleted, marked)
}

// EncodeMetadata 将元数据编码为 JSON 字符串。
func EncodeMetadata(m map[string]string) string {
	if len(m) == 0 {
		return "{}"
	}
	b, err := json.Marshal(m)
	if err != nil {
		return "{}"
	}
	return string(b)
}

// DecodeMetadata 将元数据 JSON 字符串解码为 map。
func DecodeMetadata(s string) map[string]string {
	if s == "" {
		return map[string]string{}
	}
	m := make(map[string]string)
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		return map[string]string{}
	}
	return m
}

func (s *Service) notify(namespace, group, service string) {
	if err := s.store.AppendChangeLog(event.TypeInstance, namespace, group, service, ""); err != nil {
		wlog.Ctx(context.Background()).Warnf("[registry] 写入变更日志失败: %v", err)
	}
}
