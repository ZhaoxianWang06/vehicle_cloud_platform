package cache

import (
	"context"
	"crypto/tls"
	"time"

	"github.com/redis/go-redis/v9"

	"vehicle-cloud/control-api/internal/domain"
)

// Redis 承担两件事：指令状态的热读取，以及按车辆分开的下行 Stream。
// T-Box 消费 cmd:stream:{vin}。MySQL 仍然是指令是否受理的事实来源。
type Redis struct {
	client *redis.Client
}

func Open(addr, password string, db int, useTLS bool) *Redis {
	opts := &redis.Options{
		Addr:         addr,
		Password:     password,
		DB:           db,
		DialTimeout:  2 * time.Second,
		ReadTimeout:  2 * time.Second,
		WriteTimeout: 2 * time.Second,
		PoolSize:     30,
	}
	if useTLS {
		opts.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	}
	return &Redis{client: redis.NewClient(opts)}
}

func (c *Redis) Close() error { return c.client.Close() }

func (c *Redis) Ping(ctx context.Context) error { return c.client.Ping(ctx).Err() }

func (c *Redis) Enqueue(ctx context.Context, cmd domain.Command) error {
	params := string(cmd.Params)
	if params == "" {
		params = "{}"
	}
	fields := commandFields(cmd)
	fields["status"] = domain.StatusDispatched
	pipe := c.client.TxPipeline()
	pipe.HSet(ctx, statusKey(cmd.ID), fields)
	pipe.Expire(ctx, statusKey(cmd.ID), 24*time.Hour)
	pipe.Set(ctx, idemKey(cmd.VIN, cmd.IdempotencyKey), cmd.ID, 24*time.Hour)
	pipe.XAdd(ctx, &redis.XAddArgs{
		Stream: streamKey(cmd.VIN),
		MaxLen: 1000,
		Approx: true,
		Values: map[string]any{
			"id":      cmd.ID,
			"command": cmd.Name,
			"params":  params,
		},
	})
	_, err := pipe.Exec(ctx)
	return err
}

func (c *Redis) PutStatus(ctx context.Context, cmd domain.Command) error {
	err := c.client.HSet(ctx, statusKey(cmd.ID), commandFields(cmd)).Err()
	if err != nil {
		return err
	}
	return c.client.Expire(ctx, statusKey(cmd.ID), 24*time.Hour).Err()
}

func (c *Redis) Get(ctx context.Context, id string) (domain.Command, bool, error) {
	vals, err := c.client.HGetAll(ctx, statusKey(id)).Result()
	if err != nil {
		return domain.Command{}, false, err
	}
	if len(vals) == 0 {
		return domain.Command{}, false, nil
	}
	cmd := domain.Command{
		ID:             vals["id"],
		VIN:            vals["vin"],
		Name:           vals["command"],
		IdempotencyKey: vals["idempotency_key"],
		Status:         vals["status"],
		Message:        vals["message"],
	}
	if raw := vals["created_at"]; raw != "" {
		if parsed, err := time.Parse(time.RFC3339Nano, raw); err == nil {
			cmd.CreatedAt = parsed
		}
	}
	if raw := vals["acked_at"]; raw != "" {
		if parsed, err := time.Parse(time.RFC3339Nano, raw); err == nil {
			cmd.AckedAt = &parsed
		}
	}
	return cmd, true, nil
}

func commandFields(cmd domain.Command) map[string]any {
	fields := map[string]any{
		"id":              cmd.ID,
		"vin":             cmd.VIN,
		"command":         cmd.Name,
		"status":          cmd.Status,
		"idempotency_key": cmd.IdempotencyKey,
		"message":         cmd.Message,
	}
	if !cmd.CreatedAt.IsZero() {
		fields["created_at"] = cmd.CreatedAt.UTC().Format(time.RFC3339Nano)
	}
	if cmd.AckedAt != nil && !cmd.AckedAt.IsZero() {
		fields["acked_at"] = cmd.AckedAt.UTC().Format(time.RFC3339Nano)
	}
	return fields
}

func statusKey(id string) string { return "cmd:" + id }

func idemKey(vin, key string) string { return "cmd:idem:" + vin + ":" + key }

func streamKey(vin string) string { return "cmd:stream:" + vin }
