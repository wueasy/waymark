package api

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"waymark/internal/registry"
	"waymark/internal/store"
)

type instanceRequest struct {
	Namespace   string            `json:"namespace"`
	GroupName   string            `json:"groupName"`
	ServiceName string            `json:"serviceName"`
	ClusterName string            `json:"clusterName"`
	Ip          string            `json:"ip"`
	Port        int               `json:"port"`
	Weight      float64           `json:"weight"`
	Healthy     *int              `json:"healthy"`
	Ephemeral   *int              `json:"ephemeral"`
	Metadata    map[string]string `json:"metadata"`
}

type instanceVO struct {
	Id            int64             `json:"id"`
	Namespace     string            `json:"namespace"`
	GroupName     string            `json:"groupName"`
	ServiceName   string            `json:"serviceName"`
	ClusterName   string            `json:"clusterName"`
	Ip            string            `json:"ip"`
	Port          int               `json:"port"`
	Weight        float64           `json:"weight"`
	Healthy       int               `json:"healthy"`
	Ephemeral     int               `json:"ephemeral"`
	Metadata      map[string]string `json:"metadata"`
	LastHeartbeat int64             `json:"lastHeartbeat"`
	CreateTime    int64             `json:"createTime"`
	UpdateTime    int64             `json:"updateTime"`
}

// handleRegisterInstance 注册实例。
func (s *Server) handleRegisterInstance(c *gin.Context) {
	var req instanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, "参数错误: "+err.Error())
		return
	}
	inst, okReq := s.buildInstance(c, &req)
	if !okReq {
		return
	}
	if !s.authorizeNamespace(c, inst.Namespace, true) {
		return
	}
	created, err := s.registry.Register(inst)
	if err != nil {
		fail(c, "注册实例失败: "+err.Error())
		return
	}
	auditf(c, "注册实例 namespace=%s service=%s addr=%s:%d created=%v", inst.Namespace, inst.ServiceName, inst.Ip, inst.Port, created)
	ok(c, gin.H{"created": created, "instance": toInstanceVO(inst)})
}

// handleUpdateInstance 更新实例属性。
func (s *Server) handleUpdateInstance(c *gin.Context) {
	var req instanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, "参数错误: "+err.Error())
		return
	}
	inst, okReq := s.buildInstance(c, &req)
	if !okReq {
		return
	}
	if !s.authorizeNamespace(c, inst.Namespace, true) {
		return
	}
	if err := s.registry.Update(inst); err != nil {
		fail(c, "更新实例失败: "+err.Error())
		return
	}
	auditf(c, "更新实例 namespace=%s service=%s addr=%s:%d", inst.Namespace, inst.ServiceName, inst.Ip, inst.Port)
	okNull(c)
}

// handleDeregisterInstance 注销实例。
func (s *Server) handleDeregisterInstance(c *gin.Context) {
	namespace := resolveNamespace(c.Query("namespace"))
	group := resolveGroup(c.Query("groupName"))
	service := strings.TrimSpace(c.Query("serviceName"))
	ip := strings.TrimSpace(c.Query("ip"))
	port, err := strconv.Atoi(c.Query("port"))
	if service == "" || ip == "" || err != nil {
		fail(c, "serviceName、ip、port 不能为空")
		return
	}
	if !s.authorizeNamespace(c, namespace, true) {
		return
	}
	if err := s.registry.Deregister(namespace, group, service, ip, port); err != nil {
		fail(c, "注销实例失败: "+err.Error())
		return
	}
	auditf(c, "注销实例 namespace=%s service=%s addr=%s:%d", namespace, service, ip, port)
	okNull(c)
}

// handleBeat 实例心跳。
func (s *Server) handleBeat(c *gin.Context) {
	namespace := resolveNamespace(c.Query("namespace"))
	group := resolveGroup(c.Query("groupName"))
	service := strings.TrimSpace(c.Query("serviceName"))
	ip := strings.TrimSpace(c.Query("ip"))
	port, err := strconv.Atoi(c.Query("port"))
	if service == "" || ip == "" || err != nil {
		fail(c, "serviceName、ip、port 不能为空")
		return
	}
	if !s.authorizeNamespace(c, namespace, true) {
		return
	}
	exists, err := s.registry.Beat(namespace, group, service, ip, port)
	if err != nil {
		fail(c, "心跳失败: "+err.Error())
		return
	}
	if !exists {
		fail(c, "实例不存在，请先注册")
		return
	}
	okNull(c)
}

// handleListInstances 查询实例列表。
func (s *Server) handleListInstances(c *gin.Context) {
	namespace := resolveNamespace(c.Query("namespace"))
	group := c.Query("groupName")
	service := c.Query("serviceName")
	if !s.authorizeNamespace(c, namespace, false) {
		return
	}
	list, err := s.registry.ListInstances(namespace, group, service)
	if err != nil {
		fail(c, "查询实例失败: "+err.Error())
		return
	}
	voList := make([]instanceVO, 0, len(list))
	for i := range list {
		voList = append(voList, toInstanceVO(&list[i]))
	}
	ok(c, voList)
}

// handleListServices 分页查询服务概览。
func (s *Server) handleListServices(c *gin.Context) {
	namespace := resolveNamespace(c.Query("namespace"))
	group := c.Query("groupName")
	if !s.authorizeNamespace(c, namespace, false) {
		return
	}
	pageNum := parsePositiveInt(c.Query("pageNum"), 1)
	pageSize := parsePositiveInt(c.Query("pageSize"), 20)
	if pageSize > 200 {
		pageSize = 200
	}
	list, total, err := s.registry.ListServices(namespace, group, pageNum, pageSize)
	if err != nil {
		fail(c, "查询服务列表失败: "+err.Error())
		return
	}
	ok(c, gin.H{
		"list":     list,
		"total":    total,
		"pageNum":  pageNum,
		"pageSize": pageSize,
	})
}

// buildInstance 校验并构建实例对象。
func (s *Server) buildInstance(c *gin.Context, req *instanceRequest) (*store.Instance, bool) {
	req.ServiceName = strings.TrimSpace(req.ServiceName)
	req.Ip = strings.TrimSpace(req.Ip)
	if req.ServiceName == "" || req.Ip == "" || req.Port <= 0 {
		fail(c, "serviceName、ip、port 不能为空")
		return nil, false
	}
	namespace := resolveNamespace(req.Namespace)
	if !s.ensureNamespaceExists(c, namespace) {
		return nil, false
	}

	weight := req.Weight
	if weight <= 0 {
		weight = 1
	}
	healthy := 1
	if req.Healthy != nil {
		healthy = *req.Healthy
	}
	ephemeral := 1
	if req.Ephemeral != nil {
		ephemeral = *req.Ephemeral
	}

	return &store.Instance{
		Namespace:   namespace,
		GroupName:   resolveGroup(req.GroupName),
		ServiceName: req.ServiceName,
		ClusterName: resolveCluster(req.ClusterName),
		Ip:          req.Ip,
		Port:        req.Port,
		Weight:      weight,
		Healthy:     healthy,
		Ephemeral:   ephemeral,
		Metadata:    registry.EncodeMetadata(req.Metadata),
	}, true
}

func toInstanceVO(inst *store.Instance) instanceVO {
	return instanceVO{
		Id:            inst.Id,
		Namespace:     inst.Namespace,
		GroupName:     inst.GroupName,
		ServiceName:   inst.ServiceName,
		ClusterName:   inst.ClusterName,
		Ip:            inst.Ip,
		Port:          inst.Port,
		Weight:        inst.Weight,
		Healthy:       inst.Healthy,
		Ephemeral:     inst.Ephemeral,
		Metadata:      registry.DecodeMetadata(inst.Metadata),
		LastHeartbeat: inst.LastHeartbeat,
		CreateTime:    inst.CreateTime,
		UpdateTime:    inst.UpdateTime,
	}
}

func resolveNamespace(namespace string) string {
	namespace = strings.TrimSpace(namespace)
	if namespace == "" {
		return store.DefaultNamespace
	}
	return namespace
}

func resolveGroup(group string) string {
	group = strings.TrimSpace(group)
	if group == "" {
		return store.DefaultGroup
	}
	return group
}

func resolveCluster(cluster string) string {
	cluster = strings.TrimSpace(cluster)
	if cluster == "" {
		return store.DefaultCluster
	}
	return cluster
}
