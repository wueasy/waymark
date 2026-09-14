package api

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"waymark/internal/auth"
	"waymark/internal/event"
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

// handleListSubscribers 分页查询指定命名空间下的订阅端列表。
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

	all := s.hub.ListSubscribers()
	matched := make([]event.Subscriber, 0, len(all))
	for _, sub := range all {
		if sub.Namespace == namespace {
			matched = append(matched, sub)
		}
	}

	total := int64(len(matched))
	start := (pageNum - 1) * pageSize
	if start > len(matched) {
		start = len(matched)
	}
	end := start + pageSize
	if end > len(matched) {
		end = len(matched)
	}
	ok(c, gin.H{
		"list":     matched[start:end],
		"total":    total,
		"pageNum":  pageNum,
		"pageSize": pageSize,
	})
}

// streamEvents 以 SSE 协议持续推送变更事件；configKeys 为空表示不订阅配置，
// 为 "*" 表示订阅配置维度下的全部变更；instanceKey 为空表示订阅实例维度下的全部变更。
func (s *Server) streamEvents(c *gin.Context, namespace, group string, configKeys []string, instanceKey string) {
	heartbeat := time.Duration(s.cfg.SSE.Heartbeat) * time.Second
	if heartbeat <= 0 {
		heartbeat = 15 * time.Second
	}

	username := ""
	if p := auth.CurrentPrincipal(c); p != nil && p.User != nil {
		username = p.User.Username
	}
	ch, unsubscribe := s.hub.Subscribe(namespace, group, configKeys, instanceKey, c.ClientIP(), username)
	defer unsubscribe()

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	ctx := c.Request.Context()
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
			return true
		}
	})
}
