package event

import (
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// 事件类型。
const (
	TypeConfig   = "CONFIG"
	TypeInstance = "INSTANCE"
)

// Event 变更事件（轻量信号，不含大内容，客户端收到后自行拉取最新数据）。
type Event struct {
	EventType string `json:"eventType"`
	Namespace string `json:"namespace"`
	Group     string `json:"group"`
	WatchKey  string `json:"watchKey"`
	Md5       string `json:"md5"`
}

// Subscriber 订阅端信息（一条 SSE 连接对应一个订阅者，同时订阅配置与实例变更）。
type Subscriber struct {
	Id          int64    `json:"id"`
	Namespace   string   `json:"namespace"`
	Group       string   `json:"group"`
	ConfigKeys  []string `json:"configKeys"`
	InstanceKey string   `json:"instanceKey"`
	ClientIp    string   `json:"clientIp"`
	Username    string   `json:"username"`
	ConnectedAt int64    `json:"connectedAt"`
}

type subscriber struct {
	ch   chan Event
	keys []string
	meta Subscriber
}

// Hub 本地 SSE 订阅者管理。
type Hub struct {
	mu   sync.RWMutex
	subs map[string]map[*subscriber]struct{}
	seq  atomic.Int64
}

// NewHub 创建订阅中心。
func NewHub() *Hub {
	return &Hub{subs: make(map[string]map[*subscriber]struct{})}
}

func subKey(eventType, namespace, group, watchKey string) string {
	return eventType + "|" + namespace + "|" + group + "|" + watchKey
}

// Subscribe 订阅指定维度下的配置与实例变更。configKeys 为要订阅的 dataId 列表：
// 传 "*" 表示订阅该分组下全部配置变更，传具体 dataId 表示精确订阅，不传（空）表示不订阅配置；
// instanceKey 为空表示该分组下全部服务变更。一条连接同时订阅两类变更。
// clientIp 与 username 用于展示订阅端信息。返回事件通道与取消订阅函数。
func (h *Hub) Subscribe(namespace, group string, configKeys []string, instanceKey, clientIp, username string) (<-chan Event, func()) {
	if configKeys == nil {
		configKeys = []string{}
	}
	keys := make([]string, 0, len(configKeys)+1)
	for _, configKey := range configKeys {
		if configKey == "*" {
			// 显式通配：订阅该分组下全部配置变更。
			keys = append(keys, subKey(TypeConfig, namespace, group, ""))
			continue
		}
		keys = append(keys, subKey(TypeConfig, namespace, group, configKey))
	}
	keys = append(keys, subKey(TypeInstance, namespace, group, instanceKey))

	s := &subscriber{
		ch:   make(chan Event, 32),
		keys: keys,
		meta: Subscriber{
			Id:          h.seq.Add(1),
			Namespace:   namespace,
			Group:       group,
			ConfigKeys:  configKeys,
			InstanceKey: instanceKey,
			ClientIp:    clientIp,
			Username:    username,
			ConnectedAt: time.Now().UnixMilli(),
		},
	}
	h.mu.Lock()
	for _, key := range s.keys {
		if h.subs[key] == nil {
			h.subs[key] = make(map[*subscriber]struct{})
		}
		h.subs[key][s] = struct{}{}
	}
	h.mu.Unlock()

	return s.ch, func() { h.unsubscribe(s) }
}

// ListSubscribers 返回当前全部订阅端信息的快照，按连接先后排序。
// 同一连接注册在配置与实例两个维度下，按连接去重后返回。
func (h *Hub) ListSubscribers() []Subscriber {
	h.mu.RLock()
	defer h.mu.RUnlock()
	seen := make(map[*subscriber]struct{})
	out := make([]Subscriber, 0)
	for _, set := range h.subs {
		for s := range set {
			if _, ok := seen[s]; ok {
				continue
			}
			seen[s] = struct{}{}
			out = append(out, s.meta)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Id < out[j].Id })
	return out
}

func (h *Hub) unsubscribe(s *subscriber) {
	h.mu.Lock()
	defer h.mu.Unlock()
	removed := false
	for _, key := range s.keys {
		set, ok := h.subs[key]
		if !ok {
			continue
		}
		if _, exist := set[s]; !exist {
			continue
		}
		delete(set, s)
		removed = true
		if len(set) == 0 {
			delete(h.subs, key)
		}
	}
	if removed {
		close(s.ch)
	}
}

// Publish 推送事件：同时匹配精确订阅者与通配订阅者（watchKey 为空）。
func (h *Hub) Publish(ev Event) {
	keys := []string{subKey(ev.EventType, ev.Namespace, ev.Group, ev.WatchKey)}
	if ev.WatchKey != "" {
		keys = append(keys, subKey(ev.EventType, ev.Namespace, ev.Group, ""))
	}

	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, k := range keys {
		for s := range h.subs[k] {
			select {
			case s.ch <- ev:
			default:
				// 订阅者消费过慢时丢弃事件，客户端会在后续事件或重连后重新拉取。
			}
		}
	}
}
