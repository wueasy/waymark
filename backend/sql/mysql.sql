-- Waymark 注册配置中心 MySQL 建表脚本
-- 说明：MySQL 表结构不在程序代码中自动创建，首次部署时需先执行本脚本。
--      基础数据（默认命名空间 public、Leader 租约行）由服务启动时自动写入，无需手工插入。
-- 字符集：utf8mb4；大字段统一使用 LONGTEXT。

CREATE TABLE IF NOT EXISTS `user` (
  `id`          BIGINT       NOT NULL AUTO_INCREMENT,
  `username`    VARCHAR(64)  NOT NULL,
  `password`    VARCHAR(128) NOT NULL,
  `nickname`    VARCHAR(64)  NOT NULL DEFAULT '',
  `status`      TINYINT      NOT NULL DEFAULT 1,
  `create_time` BIGINT       NOT NULL,
  `update_time` BIGINT       NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_user` (`username`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS `role` (
  `id`          BIGINT       NOT NULL AUTO_INCREMENT,
  `code`        VARCHAR(32)  NOT NULL,
  `name`        VARCHAR(64)  NOT NULL,
  `description` VARCHAR(255) NOT NULL DEFAULT '',
  `permission`  VARCHAR(16)  NOT NULL DEFAULT 'read',
  `builtin`     TINYINT      NOT NULL DEFAULT 0,
  `create_time` BIGINT       NOT NULL,
  `update_time` BIGINT       NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_role_code` (`code`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS `user_role` (
  `id`          BIGINT NOT NULL AUTO_INCREMENT,
  `user_id`     BIGINT NOT NULL,
  `role_id`     BIGINT NOT NULL,
  `create_time` BIGINT NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_user_role` (`user_id`, `role_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS `namespace` (
  `id`          BIGINT       NOT NULL AUTO_INCREMENT,
  `namespace`   VARCHAR(64)  NOT NULL,
  `name`        VARCHAR(128) NOT NULL,
  `description` VARCHAR(255) NOT NULL DEFAULT '',
  `create_time` BIGINT       NOT NULL,
  `update_time` BIGINT       NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_namespace` (`namespace`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS `role_namespace` (
  `id`          BIGINT      NOT NULL AUTO_INCREMENT,
  `role_id`     BIGINT      NOT NULL,
  `namespace`   VARCHAR(64) NOT NULL,
  `create_time` BIGINT      NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_role_ns` (`role_id`, `namespace`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS `service_instance` (
  `id`             BIGINT       NOT NULL AUTO_INCREMENT,
  `namespace`      VARCHAR(64)  NOT NULL DEFAULT 'public',
  `group_name`     VARCHAR(64)  NOT NULL DEFAULT 'DEFAULT_GROUP',
  `service_name`   VARCHAR(128) NOT NULL,
  `cluster_name`   VARCHAR(64)  NOT NULL DEFAULT 'DEFAULT',
  `ip`             VARCHAR(64)  NOT NULL,
  `port`           INT          NOT NULL,
  `weight`         DOUBLE       NOT NULL DEFAULT 1,
  `healthy`        TINYINT      NOT NULL DEFAULT 1,
  `ephemeral`      TINYINT      NOT NULL DEFAULT 1,
  `metadata`       LONGTEXT     NOT NULL,
  `last_heartbeat` BIGINT       NOT NULL,
  `create_time`    BIGINT       NOT NULL,
  `update_time`    BIGINT       NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_instance` (`namespace`, `group_name`, `service_name`, `cluster_name`, `ip`, `port`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS `config_info` (
  `id`          BIGINT       NOT NULL AUTO_INCREMENT,
  `namespace`   VARCHAR(64)  NOT NULL DEFAULT 'public',
  `group_name`  VARCHAR(64)  NOT NULL DEFAULT 'DEFAULT_GROUP',
  `data_id`     VARCHAR(128) NOT NULL,
  `content`     LONGTEXT     NOT NULL,
  `md5`         VARCHAR(32)  NOT NULL,
  `type`        VARCHAR(32)  NOT NULL DEFAULT 'text',
  `create_time` BIGINT       NOT NULL,
  `update_time` BIGINT       NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_config` (`namespace`, `group_name`, `data_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS `config_draft` (
  `id`          BIGINT       NOT NULL AUTO_INCREMENT,
  `namespace`   VARCHAR(64)  NOT NULL DEFAULT 'public',
  `group_name`  VARCHAR(64)  NOT NULL DEFAULT 'DEFAULT_GROUP',
  `data_id`     VARCHAR(128) NOT NULL,
  `content`     LONGTEXT     NOT NULL,
  `md5`         VARCHAR(32)  NOT NULL,
  `type`        VARCHAR(32)  NOT NULL DEFAULT 'text',
  `based_md5`   VARCHAR(32)  NOT NULL DEFAULT '',
  `operator`    VARCHAR(64)  NOT NULL DEFAULT '',
  `create_time` BIGINT       NOT NULL,
  `update_time` BIGINT       NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_config_draft` (`namespace`, `group_name`, `data_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS `config_history` (
  `id`          BIGINT       NOT NULL AUTO_INCREMENT,
  `namespace`   VARCHAR(64)  NOT NULL,
  `group_name`  VARCHAR(64)  NOT NULL,
  `data_id`     VARCHAR(128) NOT NULL,
  `content`     LONGTEXT     NOT NULL,
  `md5`         VARCHAR(32)  NOT NULL,
  `type`        VARCHAR(32)  NOT NULL DEFAULT 'text',
  `create_time` BIGINT       NOT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_history` (`namespace`, `group_name`, `data_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS `change_log` (
  `seq`         BIGINT       NOT NULL AUTO_INCREMENT,
  `event_type`  VARCHAR(16)  NOT NULL,
  `namespace`   VARCHAR(64)  NOT NULL,
  `group_name`  VARCHAR(64)  NOT NULL,
  `watch_key`   VARCHAR(128) NOT NULL,
  `md5`         VARCHAR(32)  NOT NULL DEFAULT '',
  `change_time` BIGINT       NOT NULL,
  PRIMARY KEY (`seq`),
  KEY `idx_change_time` (`change_time`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS `subscriber_session` (
  `id`           BIGINT       NOT NULL AUTO_INCREMENT,
  `node_id`      VARCHAR(64)  NOT NULL,
  `namespace`    VARCHAR(64)  NOT NULL DEFAULT 'public',
  `group_name`   VARCHAR(64)  NOT NULL DEFAULT 'DEFAULT_GROUP',
  `config_keys`  TEXT         NOT NULL,
  `instance_key` VARCHAR(128) NOT NULL DEFAULT '',
  `client_ip`    VARCHAR(64)  NOT NULL DEFAULT '',
  `username`     VARCHAR(64)  NOT NULL DEFAULT '',
  `connected_at` BIGINT       NOT NULL,
  `last_heartbeat` BIGINT     NOT NULL DEFAULT 0 COMMENT '最后一次 SSE keep-alive 心跳时间（毫秒）',
  PRIMARY KEY (`id`),
  KEY `idx_subscriber_ns` (`namespace`),
  KEY `idx_subscriber_node` (`node_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS `cluster_node` (
  `id`             BIGINT       NOT NULL AUTO_INCREMENT,
  `node_id`        VARCHAR(64)  NOT NULL,
  `address`        VARCHAR(128) NOT NULL,
  `status`         VARCHAR(16)  NOT NULL DEFAULT 'UP',
  `last_heartbeat` BIGINT       NOT NULL,
  `create_time`    BIGINT       NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_node` (`node_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS `cluster_leader` (
  `leader_key`  VARCHAR(64) NOT NULL,
  `node_id`     VARCHAR(64) NOT NULL,
  `lease_until` BIGINT      NOT NULL,
  `update_time` BIGINT      NOT NULL,
  PRIMARY KEY (`leader_key`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
