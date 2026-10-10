-- 实验库。口令只用于本地和集群内的学习环境，不能拿去连生产。
CREATE DATABASE IF NOT EXISTS vehicle_cloud
  DEFAULT CHARACTER SET utf8mb4
  DEFAULT COLLATE utf8mb4_0900_ai_ci;
USE vehicle_cloud;

-- 车辆主数据。上报和车控都会补一行，避免演示时先注册。
CREATE TABLE IF NOT EXISTS vehicles (
  vin        CHAR(17)  NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (vin)
) ENGINE=InnoDB;

-- 车况历史。实时状态在 Redis，这里只回答“过去发生了什么”。
CREATE TABLE IF NOT EXISTS telemetry (
  id           BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  vin          CHAR(17)        NOT NULL,
  speed_kmh    DOUBLE          NOT NULL,
  battery_pct  DOUBLE          NOT NULL,
  lng          DOUBLE          NOT NULL,
  lat          DOUBLE          NOT NULL,
  odometer_km  DOUBLE          NOT NULL,
  reported_at  DATETIME(3)     NOT NULL,
  ingested_at  DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  KEY idx_telemetry_vin_reported (vin, reported_at),
  CONSTRAINT fk_telemetry_vehicle FOREIGN KEY (vin) REFERENCES vehicles (vin)
) ENGINE=InnoDB;

-- 车控指令。幂等键挡住网关重试；(vin, idempotency_key) 是事实来源。
CREATE TABLE IF NOT EXISTS control_commands (
  id               CHAR(36)     NOT NULL,
  vin              CHAR(17)     NOT NULL,
  command          VARCHAR(32)  NOT NULL,
  params_json      JSON         NULL,
  idempotency_key  VARCHAR(64)  NOT NULL,
  status           VARCHAR(16)  NOT NULL,
  result_message   VARCHAR(255) NOT NULL DEFAULT '',
  created_at       DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at       DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  acked_at         DATETIME(3)  NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uk_command_idem (vin, idempotency_key),
  KEY idx_command_vin_created (vin, created_at),
  CONSTRAINT fk_command_vehicle FOREIGN KEY (vin) REFERENCES vehicles (vin)
) ENGINE=InnoDB;

CREATE USER IF NOT EXISTS 'exporter'@'%' IDENTIFIED BY 'exporter-lab-pass';
GRANT PROCESS, REPLICATION CLIENT, SELECT ON *.* TO 'exporter'@'%';
FLUSH PRIVILEGES;
