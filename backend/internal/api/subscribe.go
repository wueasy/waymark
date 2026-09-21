package api

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	wlog "github.com/wueasy/wueasy-go-tools/log"

	"waymark/internal/auth"
	"waymark/internal/store"
)

// handleSubscribe 订阅变更（SSE 长连接）：一条连接同时订阅配置与实例变更。
// dataId 可重复出现以订阅多个配置文件；dataId=* 表示订阅该分组下全部配置变更，
// 传具体 dataId 表示精确订阅，不传 dataId 表示不订阅配置；serviceName 为空表示订阅该分组下全部服务变更。
func (s *Server) handleSubscribe(c *gin.Context) {
	namespace := resolveNamespace(c.Query("namespace"))
	if !s.authorizeNamespace(c, namespace, false) {
		return
	}
	group := resolveGroup(c.Query("groupName"))
	configKeys := resolveKeys(c.QueryArray("dataId"))
	instanceKey := strings.TrimSpace(c.Query("serviceName"))
	s.streamEvents(c, namespace, group, configKeys, instanceKey)
}

// resolveKeys 归一化订阅键：去除空白、忽略空值并去重，保持原有顺序。
func resolveKeys(values []string) []string {
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		key := strings.TrimSpace(value)
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, key)
	}
	return out
}

// subscriberView 订阅端列表项：在会话记录基础上把订阅键展开为数组。
type subscriberView struct {
	store.SubscriberSession
	ConfigKeys []string `json:"configKeys"`
}

// handleListSubscribers 分页查询指定命名空间下的订阅端列表。
// 数据取自 subscriber_session 表，因此展示的是集群内全部节点的 SSE 连接，而非仅当前节点。
func (s *Server) handleListSubscribers(c *gin.Context) {
	namespace := resolveNamespace(c.Query("namespace"))
	if !s.authorizeNamespace(c, namespace, false) {
		return
	}
	pageNum := parsePositiveInt(c.Query("pageNum"), 1)
	pageSize := parsePositiveInt(c.Query("pageSize"), 20)
	if pageSize > 200 {
		pageSize = 200
	}

	filter := store.SubscriberFilter{
		NodeId:    strings.TrimSpace(c.Query("nodeId")),
		GroupName: strings.TrimSpace(c.Query("groupName")),
		Keyword:   strings.TrimSpace(c.Query("keyword")),
	}

	list, total, err := s.store.ListSubscribers(namespace, filter, pageNum, pageSize)
	if err != nil {
		fail(c, "查询订阅端失败: "+err.Error())
		return
	}

	views := make([]subscriberView, 0, len(list))
	for _, sess := range list {
		views = append(views, subscriberView{SubscriberSession: sess, ConfigKeys: decodeConfigKeys(sess.ConfigKeys)})
	}

	ok(c, gin.H{
		"list":     views,
		"total":    total,
		"pageNum":  pageNum,
		"pageSize": pageSize,
	})
}

// encodeConfigKeys 将订阅键序列化为 JSON 存库，nil 统一存为空数组。
func encodeConfigKeys(keys []string) string {
	if len(keys) == 0 {
		return "[]"
	}
	payload, err := json.Marshal(keys)
	if err != nil {
		return "[]"
	}
	return string(payload)
}

// decodeConfigKeys 解析会话中存储的订阅键，异常数据退化为空数组。
func decodeConfigKeys(raw string) []string {
	keys := make([]string, 0)
	if raw == "" {
		return keys
	}
	if err := json.Unmarshal([]byte(raw), &keys); err != nil {
		return []string{}
	}
	return keys
}

// streamEvents 以 SSE 协议持续推送变更事件；configKeys 为空表示不订阅配置，
// 为 "*" 表示订阅配置维度下的全部变更；instanceKey 为空表示订阅实例维度下的全部变更。
func (s *Server) streamEvents(c *gin.Context, namespace, group string, configKeys []string, instanceKey string) {
	heartbeat := time.Duration(s.cfg.SSE.Heartbeat) * time.Second
	if heartbeat <= 0 {
		heartbeat = 15 * time.Second
	}

	ctx := c.Request.Context()
	username := ""
	if p := auth.CurrentPrincipal(c); p != nil && p.User != nil {
		username = p.User.Username
	}
	ch, unsubscribe := s.hub.Subscribe(namespace, group, configKeys, instanceKey)
	defer unsubscribe()

	// 会话落库，使订阅列表在集群内可见；连接断开（含客户端异常掉线）时删除本行。
	sess := &store.SubscriberSession{
		NodeId:      s.cluster.NodeId(),
		Namespace:   namespace,
		GroupName:   group,
		ConfigKeys:  encodeConfigKeys(configKeys),
		InstanceKey: instanceKey,
		ClientIp:    c.ClientIP(),
		Username:    username,
	}
	if err := s.store.SaveSubscriber(sess); err != nil {
		wlog.Ctx(ctx).Warnf("[subscribe] 记录订阅会话失败: %v", err)
	}
	defer func() {
		if sess.Id > 0 {
			if err := s.store.DeleteSubscriber(sess.Id); err != nil {
				wlog.Ctx(ctx).Warnf("[subscribe] 清理订阅会话失败: %v", err)
			}
		}
	}()

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	ticker := time.NewTicker(heartbeat)
	defer ticker.Stop()

	_, _ = fmt.Fprint(c.Writer, ": connected\n\n")
	c.Writer.Flush()

	c.Stream(func(w io.Writer) bool {
		select {
		case <-ctx.Done():
			return false
		case ev, open := <-ch:
			if !open {
				return false
			}
			payload, err := json.Marshal(ev)
			if err != nil {
				return true
			}
			if _, err := fmt.Fprintf(w, "event: change\ndata: %s\n\n", payload); err != nil {
				return false
			}
			return true
		case <-ticker.C:
			if _, err := fmt.Fprint(w, ": ping\n\n"); err != nil {
				return false
			}
			// 心跳同时刷新落库时间，订阅列表据此展示连接的最后心跳。
			if sess.Id > 0 {
				if err := s.store.UpdateSubscriberHeartbeat(sess.Id); err != nil {
					wlog.Ctx(ctx).Warnf("[subscribe] 更新订阅会话心跳失败: %v", err)
				}
			}
			return true
		}
	})
}
