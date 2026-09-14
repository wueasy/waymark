# Waymark 注册中心 & 配置中心

> 类 Nacos 但更简洁的注册/配置中心。核心原则：**共享数据库做事实源、DB 租约做 Leader 选举、SSE 长连接做变更推送**，不引入 Raft/Paxos、不做节点间数据对拷。

---

## 1. 简介

Waymark 提供以下能力：

- **注册中心**：服务实例的注册、心跳、注销、查询与变更订阅。
- **配置中心**：配置的发布、查询、删除、历史与变更订阅。
- **用户认证与权限**：登录认证、角色管理（自定义角色 + 权限等级）、用户角色分配、基于角色的命名空间授权。
- **命名空间管理**：命名空间的创建、修改、删除。

支持两种运行模式（可通过配置切换）：

| 模式 | 数据库 | 集群 | 用途 |
|------|--------|------|------|
| 单机模式 | SQLite | 否 | 轻量、免安装，适合开发/内网/嵌入式 |
| 集群模式 | MySQL | 是 | 多节点共享同一 MySQL，支持 Leader 选举与高可用 |

运行模式由 `db.type` 与 `cluster.enabled` 共同决定，非法组合（sqlite + 集群）在启动时校验并报错。

---

## 2. 总体架构

```
                      ┌──────────────────────────────────────┐
 客户端 / SDK / 控制台 ──►  HTTP (JSON + SSE 长连接) ──►      │  Waymark 节点 (Gin)      │
                      └──────────────────────────────────────┘
                                      │  store 抽象层
                         ┌────────────┴─────────────┐
                         │      MySQL  /  SQLite     │
                         └──────────────────────────┘
```

- 每个节点独立提供 HTTP 读写服务，共享同一数据库。
- 客户端通过 **SSE 长连接** 订阅配置/服务变更，收到通知后再拉取最新数据。
- 集群通过 **DB 租约选举** 产生 Leader，Leader 负责后台维护任务，避免多节点重复执行。
- 服务端始终监听 `0.0.0.0`，不提供 `address` 配置。

---

## 3. 集群与 Leader 选举

### 3.1 选举模型（基于数据库租约）

不引入额外一致性协议，用 `cluster_leader` 表实现「租约式选举」：

```
每个节点周期执行（周期 = leaseTTL / 3）：
  1) UPDATE cluster_leader
        SET node_id = :me, lease_until = :now + leaseTTL
      WHERE leader_key = 'default' AND lease_until < :now
     → 影响行数 = 1：抢到租约，成为 Leader
     → 影响行数 = 0：未抢到，进入步骤 2
  2) SELECT node_id FROM cluster_leader WHERE leader_key = 'default'
     → node_id == :me：仍是 Leader，续租（延长 lease_until）
     → node_id != :me：是 Follower，等待下个周期
```

- 租约到期后（Leader 崩溃/无响应），任意节点可在下一周期抢占，实现自动切换。
- **时间统一以数据库服务器时钟为准**（`NOW(3)` / SQLite `julianday('now')`）：节点心跳、实例心跳的写入与其超时判定、租约的抢占与续租均使用数据库时钟，避免跨机器本地时钟漂移导致误判下线或提前抢占租约。
- 单机模式下跳过选举，本节点即唯一节点并直接承担维护任务。

### 3.2 Leader 职责

Leader 负责**有且仅有一份**的后台维护任务（避免多节点重复执行）：

1. 淘汰过期临时实例（心跳超时）。
2. 集群节点健康检查（心跳超时标记 `DOWN`，超过 `node-retain` 未心跳则移除节点记录）。
3. 清理过期变更日志（保留期内日志保留，见 4.2）。

### 3.3 节点发现

- 节点启动时 upsert 自身到 `cluster_node` 并周期心跳，其他节点查表即可获取节点列表。
- 节点标识（`node-id`）优先取 `cluster.node-id`；未配置时自动生成为「主机名-网卡 MAC-端口」，保证跨机器唯一，避免同名机器撞 `cluster_node` 唯一键、甚至同 `node-id` 互相续租造成双 Leader。
- 节点地址（`ip:port`）优先取 `cluster.ip` / `cluster.port`；未配置时 IP 自动探测本机首个非回环 IPv4、端口取 `server.port`。多网卡或容器场景建议显式配置。节点地址仅用于展示，跨节点同步依赖共享变更日志，不做节点间 RPC。
- 心跳超时（如 30s）由 Leader 标记为 `DOWN`；超过 `node-retain`（如 300s）仍无心跳则从 `cluster_node` 移除。节点记录被移除后若节点仍在运行，下次心跳会自动重新注册。

---

## 4. 变更通知（SSE 长连接）

> 不使用 HTTP 长轮询。客户端与节点之间通过 **SSE（Server-Sent Events）** 保持长连接；节点之间通过**共享变更日志**感知跨节点变更。

### 4.1 订阅端点

```
GET /api/subscribe?dataId=&dataId=&serviceName=&groupName=&namespace=  # 单条连接同时订阅多个配置与实例变更
```

参数语义：

- `dataId`：要订阅的配置文件名，可重复出现以订阅多个文件；传 `*` 表示订阅该分组下**全部**配置变更，传具体 `dataId` 表示精确订阅，**不传表示不订阅配置**（仅订阅实例维度）。
- `serviceName`：为空表示订阅该分组下**全部**服务变更。

SSE 事件格式（`data:` 行内为 JSON，字段使用小驼峰）：

```
event: change
data: {"eventType":"CONFIG","namespace":"public","group":"DEFAULT_GROUP","watchKey":"app.yaml","md5":"a1b2c3..."}
```

- 客户端收到事件后，用普通 `GET` 拉取最新配置/实例列表（SSE 只传轻量变更信号，不传大内容）。
- 服务端每 15s 发送一次 `: ping` 注释行作为心跳，防止中间设备断开连接。
- 客户端在断开后采用指数退避重连。
- 认证：SSE 使用 `?token=` 查询参数传递令牌（浏览器 EventSource 无法自定义请求头）。

### 4.2 跨节点同步（变更日志）

每次写操作在落库事务内，同时向 `change_log` 追加一条事件：

```
写入事务：upsert 数据 + 追加 change_log → 提交
```

每个节点运行一个 **publisher** 后台协程：

```
每 pollMs 执行：
  SELECT ... FROM change_log WHERE seq > :cursor ORDER BY seq LIMIT 200
  对每条事件 → 匹配本地 SSE 订阅者 → 推送
  更新 cursor = 最后一条 seq
```

- `cursor` 为节点内存中的游标，启动时初始化为当前 `max(seq)`（只处理新变更）。
- 同节点写入 → 事件最终经 publisher 推送给本地订阅者（延迟 ≤ pollMs）。
- 跨节点写入 → 其他节点的 publisher 扫描到新事件后推送，实现最终一致。
- 变更日志由 Leader 定期清理（保留期建议 7 天，远大于 publisher 消费延迟）。

---

## 5. 快速开始

### 5.1 数据库准备

- **SQLite（单机）**：内置数据库，服务启动时自动建表，无需手工操作。
- **MySQL（集群）**：表结构**不在代码中自动创建**，首次部署需先手工执行 [`backend/sql/mysql.sql`](backend/sql/mysql.sql)，服务启动时仅校验表结构是否就绪。
- 基础数据（默认命名空间 `public`、Leader 租约行）由服务启动时幂等写入。

首次启动后 `user` 表为空，前端会进入初始化页，输入首个管理员账号（用户名 + 密码）即可创建 `admin` 并进入正常登录流程。

### 5.2 配置示例（config.yaml）

```yaml
server:
  port: 9868
  gzip: true                      # 是否对响应启用 gzip 压缩（SSE 长连接不做压缩）

log:
  level: info                     # 日志级别 debug/info/warn/error
  max-size: 100                   # 单个日志文件最大大小（MB）
  max-backups: 100                # 保留的旧日志文件数量
  max-age: 100                    # 日志保留天数
  sensitive:                      # 日志脱敏，避免密码等敏感信息写入日志
    max-length: 1000              # 单个字段值超过该长度则截断（上限 1000）
    field-rules:
      - field-names: ["password", "oldpassword", "newpassword", "confirmpassword", "password2"]
        type: password
      - field-names: ["token", "authorization", "secret", "jwt-secret", "jwtsecret"]
        type: password

auth:
  jwt-secret: ""                  # 集群模式必须配置且各节点一致；单机模式留空则自动生成随机密钥
  token-ttl: 7200                 # 秒，令牌有效期

demo:
  enabled: true                   # 演示环境开关：开启后，protected-users 中的账号将被冻结
  protected-users: ["admin"]      # 受保护账号：禁止删除、重置/修改密码、禁用启用、改昵称与角色分配

db:
  type: sqlite                    # mysql | sqlite
  mysql:
    dsn: "root:123456@tcp(127.0.0.1:3306)/waymark?charset=utf8mb4&parseTime=true&loc=Local"
    max-open-conns: 50
    max-idle-conns: 10
  sqlite:
    path: "./data/waymark.db"

cluster:
  enabled: false                  # 集群开关，sqlite 模式下必须为 false
  node-id: ""                     # 空则自动生成为 主机名-MAC-端口（跨机器唯一）
  ip: ""                          # 本节点注册 IP，空则自动探测本机 IP；配置后以此为准
  port: 0                         # 本节点注册端口，0 则使用 server.port；配置后以此为准
  lease-ttl: 10                   # 秒，Leader 租约时长
  election-interval: 3            # 秒，选举/续租周期
  node-timeout: 30                # 秒，节点心跳超时（标记为离线）
  node-retain: 300                # 秒，节点无心跳超过该时长后从节点列表移除

registry:
  heartbeat-timeout: 15           # 秒，实例心跳超时（超时后临时实例删除、持久实例标记不健康）
  sweep-interval: 10              # 秒，淘汰扫描周期（由 Leader 执行）

sse:
  heartbeat: 15                   # 秒，SSE keep-alive 间隔
  poll-ms: 500                    # 毫秒，变更日志扫描周期
  log-retention: 0                # 秒，变更日志保留期（0 表示不保留）
```

### 5.3 启动

- 后端：`backend/启动后端.bat`（或 `go run ./cmd/server`，工作目录为 `backend`）。
- 前端：`frontend/启动前端.bat`（或 `npm install && npm run dev`）。
