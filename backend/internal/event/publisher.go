package event

import (
	"context"
	"time"

	wlog "github.com/wueasy/wueasy-go-tools/log"

	"waymark/internal/store"
)

// Publisher 轮询变更日志并推送本地 SSE 订阅者，实现跨节点变更的最终一致。
type Publisher struct {
	store    *store.Store
	hub      *Hub
	interval time.Duration
	cursor   int64
}

// NewPublisher 创建 publisher，pollMs 为扫描周期（毫秒）。
func NewPublisher(st *store.Store, hub *Hub, pollMs int) *Publisher {
	if pollMs <= 0 {
		pollMs = 500
	}
	return &Publisher{
		store:    st,
		hub:      hub,
		interval: time.Duration(pollMs) * time.Millisecond,
	}
}

// Start 初始化游标并启动后台扫描。
func (p *Publisher) Start(ctx context.Context) {
	seq, err := p.store.MaxChangeLogSeq()
	if err != nil {
		wlog.Ctx(ctx).Warnf("[event] 初始化变更日志游标失败: %v", err)
	} else {
		p.cursor = seq
	}
	go p.run(ctx)
}

func (p *Publisher) run(ctx context.Context) {
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.poll(ctx)
		}
	}
}

const pollBatchSize = 200

func (p *Publisher) poll(ctx context.Context) {
	for {
		if ctx.Err() != nil {
			return
		}
		logs, err := p.store.ListChangeLogSince(p.cursor, pollBatchSize)
		if err != nil {
			wlog.Ctx(ctx).Warnf("[event] 查询变更日志失败: %v", err)
			return
		}
		if len(logs) == 0 {
			return
		}
		for _, l := range logs {
			p.hub.Publish(Event{
				EventType: l.EventType,
				Namespace: l.Namespace,
				Group:     l.GroupName,
				WatchKey:  l.WatchKey,
				Md5:       l.Md5,
			})
			p.cursor = l.Seq
		}
		if len(logs) < pollBatchSize {
			return
		}
	}
}
