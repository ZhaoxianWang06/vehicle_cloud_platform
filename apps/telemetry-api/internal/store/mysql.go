package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"vehicle-cloud/telemetry-api/internal/domain"
)

// MySQL 保存车况历史。连接池按上报接口的并发来设，而不是按默认无限制来用。
type MySQL struct {
	db *sql.DB
}

func OpenMySQL(dsn string) (*MySQL, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(50)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(30 * time.Minute)
	return &MySQL{db: db}, nil
}

func (s *MySQL) Close() error { return s.db.Close() }

func (s *MySQL) Ping(ctx context.Context) error { return s.db.PingContext(ctx) }

func (s *MySQL) SaveTelemetry(ctx context.Context, rec domain.Record) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO vehicles (vin) VALUES (?) ON DUPLICATE KEY UPDATE vin = vin`,
		rec.VIN,
	); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO telemetry (vin, speed_kmh, battery_pct, lng, lat, odometer_km, reported_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		rec.VIN, rec.SpeedKmh, rec.BatteryPct, rec.Lng, rec.Lat, rec.OdometerKm, rec.ReportedAt,
	); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *MySQL) Latest(ctx context.Context, vin string) (domain.Record, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT vin, speed_kmh, battery_pct, lng, lat, odometer_km, reported_at
		FROM telemetry
		WHERE vin = ?
		ORDER BY reported_at DESC, id DESC
		LIMIT 1`, vin)
	rec, err := scanRecord(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Record{}, domain.ErrNotFound
	}
	return rec, err
}

func (s *MySQL) History(ctx context.Context, vin string, limit int) ([]domain.Record, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT vin, speed_kmh, battery_pct, lng, lat, odometer_km, reported_at
		FROM telemetry
		WHERE vin = ?
		ORDER BY reported_at DESC, id DESC
		LIMIT ?`, vin, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]domain.Record, 0, limit)
	for rows.Next() {
		rec, err := scanRecord(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}

type scanner interface {
	Scan(dest ...any) error
}

func scanRecord(row scanner) (domain.Record, error) {
	var rec domain.Record
	err := row.Scan(&rec.VIN, &rec.SpeedKmh, &rec.BatteryPct, &rec.Lng, &rec.Lat, &rec.OdometerKm, &rec.ReportedAt)
	return rec, err
}
