-- Schema for the cdc integration test (cdc/mysqlsink/sink_integration_test.go).
-- Debezium reads cdc_source; mysqlsink writes the same table name into cdc_target.

CREATE DATABASE IF NOT EXISTS cdc_source;
CREATE DATABASE IF NOT EXISTS cdc_target;

CREATE TABLE cdc_source.orders (
  id          BIGINT       NOT NULL PRIMARY KEY,
  name        VARCHAR(255) NOT NULL,
  status      VARCHAR(32)  NOT NULL DEFAULT 'pending',
  secret_note VARCHAR(255) NULL,
  updated_at  DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3)
);

-- secret_note exists on the target on purpose: it must stay NULL because the
-- test's TableConfig.Include does not list it.
CREATE TABLE cdc_target.orders (
  id          BIGINT       NOT NULL PRIMARY KEY,
  name        VARCHAR(255) NOT NULL,
  status      VARCHAR(32)  NOT NULL DEFAULT 'pending',
  secret_note VARCHAR(255) NULL,
  updated_at  DATETIME(3)  NULL
);

-- Seeded before Debezium starts, so its initial snapshot emits this row (op=r).
INSERT INTO cdc_source.orders (id, name, status, secret_note) VALUES (1, 'initial', 'pending', 'do-not-copy');
