package api

import (
	"github.com/gin-gonic/gin"
)

// handleClusterNodes 查询集群节点列表。
func (s *Server) handleClusterNodes(c *gin.Context) {
	list, err := s.store.ListNodes()
	if err != nil {
		fail(c, "查询节点列表失败: "+err.Error())
		return
	}
	ok(c, gin.H{
		"mode":    s.mode(),
		"nodeId":  s.cluster.NodeId(),
		"nodes":   list,
	})
}

// handleClusterLeader 查询当前 Leader。
func (s *Server) handleClusterLeader(c *gin.Context) {
	leader, err := s.cluster.Leader()
	if err != nil {
		fail(c, "查询 Leader 失败: "+err.Error())
		return
	}
	ok(c, gin.H{
		"mode":     s.mode(),
		"nodeId":   s.cluster.NodeId(),
		"address":  s.cluster.Address(),
		"isLeader": s.cluster.IsLeader(),
		"leader":   leader,
	})
}

func (s *Server) mode() string {
	if s.cfg.Cluster.Enabled {
		return "cluster"
	}
	return "standalone"
}
