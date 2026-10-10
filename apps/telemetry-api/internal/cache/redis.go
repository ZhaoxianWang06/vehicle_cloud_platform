package cache

import (
	"context"
	"crypto/tls"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"

	"vehicle-cloud/telemetry-api/internal/domain"
)

const (
	statusPrefix = "veh:status:"
	onlineKey    = "veh:online"
	statusTTL    = 24 * time.Hour
	onlineWindow = 60 * time.Second
)

// Redis 只放“当前状态”。历史以 MySQL 为准，缓存写失败时上报接口仍可成功。
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
		PoolSize:     50,
	}
	if useTLS {
		opts.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	}
	return &Redis{client: redis.NewClient(opts)}
}

func (c *Redis) Close() error { return c.client.Close() }

func (c *Redis) Ping(ctx context.Context) error { return c.client.Ping(ctx).Err() }

func (c *Redis) PutLatest(ctx context.Context, rec domain.Record) error {
	key := statusPrefix + rec.VIN
	pipe := c.client.Pipeline()
	pipe.HSet(ctx, key, map[string]any{
		"vin":         rec.VIN,
		"speed_kmh":   strconv.FormatFloat(rec.SpeedKmh, 'f', 2, 64),
		"battery_pct": strconv.FormatFloat(rec.BatteryPct, 'f', 2, 64),
		"lng":         strconv.FormatFloat(rec.Lng, 'f', 6, 64),
		"lat":         strconv.FormatFloat(rec.Lat, 'f', 6, 64),
		"odometer_km": strconv.FormatFloat(rec.OdometerKm, 'f', 1, 64),
		"reported_at": rec.ReportedAt.Format(time.RFC3339Nano),
	})
	pipe.Expire(ctx, key, statusTTL)
	// 在线车辆用 ZSET 的分数表示最后心跳。过期清理放到后台，避免把 SCAN 放进上报热路径。
	pipe.ZAdd(ctx, onlineKey, redis.Z{Score: float64(time.Now().Unix()), Member: rec.VIN})
	_, err := pipe.Exec(ctx)
	return err
}

func (c *Redis) GetLatest(ctx context.Context, vin string) (domain.Record, bool, error) {
	vals, err := c.client.HGetAll(ctx, statusPrefix+vin).Result()
	if err != nil {
		return domain.Record{}, false, err
	}
	if len(vals) == 0 {
		return domain.Record{}, false, nil
	}
	rec, err := recordFromHash(vals)
	if err != nil {
		return domain.Record{}, false, err
	}
	return rec, true, nil
}

func (c *Redis) OnlineCount(ctx context.Context) (int64, error) {
	cutoff := strconv.FormatInt(time.Now().Add(-onlineWindow).Unix(), 10)
	if err := c.client.ZRemRangeByScore(ctx, onlineKey, "-inf", cutoff).Err(); err != nil {
		return 0, err
	}
	return c.client.ZCard(ctx, onlineKey).Result()
}

func recordFromHash(vals map[string]string) (domain.Record, error) {
	speed, err := strconv.ParseFloat(vals["speed_kmh"], 64)
	if err != nil {
		return domain.Record{}, fmt.Errorf("speed_kmh: %w", err)
	}
	battery, err := strconv.ParseFloat(vals["battery_pct"], 64)
	if err != nil {
		return domain.Record{}, fmt.Errorf("battery_pct: %w", err)
	}
	lng, err := strconv.ParseFloat(vals["lng"], 64)
	if err != nil {
		return domain.Record{}, fmt.Errorf("lng: %w", err)
	}
	lat, err := strconv.ParseFloat(vals["lat"], 64)
	if err != nil {
		return domain.Record{}, fmt.Errorf("lat: %w", err)
	}
	odo, err := strconv.ParseFloat(vals["odometer_km"], 64)
	if err != nil {
		return domain.Record{}, fmt.Errorf("odometer_km: %w", err)
	}
	reported, err := time.Parse(time.RFC3339Nano, vals["reported_at"])
	if err != nil {
		return domain.Record{}, fmt.Errorf("reported_at: %w", err)
	}
	return domain.Record{
		VIN:        vals["vin"],
		SpeedKmh:   speed,
		BatteryPct: battery,
		Lng:        lng,
		Lat:        lat,
		OdometerKm: odo,
		ReportedAt: reported,
	}, nil
}
