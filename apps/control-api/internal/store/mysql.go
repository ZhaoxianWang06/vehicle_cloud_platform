package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/go-sql-driver/mysql"

	"vehicle-cloud/control-api/internal/domain"
)

type MySQL struct {
	db *sql.DB
}

func OpenMySQL(dsn string) (*MySQL, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(30)
	db.SetMaxIdleConns(8)
	db.SetConnMaxLifetime(30 * time.Minute)
	return &MySQL{db: db}, nil
}

func (s *MySQL) Close() error { return s.db.Close() }

func (s *MySQL) Ping(ctx context.Context) error { return s.db.PingContext(ctx) }

// Insert 以 (vin, idempotency_key) 唯一约束作为幂等事实来源。
// 重复键返回已存在的指令，调用方不要再次当作新命令。
func (s *MySQL) Insert(ctx context.Context, cmd domain.Command) (domain.Command, bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.Command{}, false, err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO vehicles (vin) VALUES (?) ON DUPLICATE KEY UPDATE vin = vin`, cmd.VIN,
	); err != nil {
		return domain.Command{}, false, err
	}

	var params any
	if len(cmd.Params) > 0 {
		params = string(cmd.Params)
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO control_commands (id, vin, command, params_json, idempotency_key, status)
		VALUES (?, ?, ?, ?, ?, ?)`,
		cmd.ID, cmd.VIN, cmd.Name, params, cmd.IdempotencyKey, domain.StatusPending,
	)
	if err != nil {
		var me *mysql.MySQLError
		if errors.As(err, &me) && me.Number == 1062 {
			_ = tx.Rollback()
			existing, getErr := s.byIdempotency(ctx, cmd.VIN, cmd.IdempotencyKey)
			return existing, true, getErr
		}
		return domain.Command{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return domain.Command{}, false, err
	}
	stored, err := s.Get(ctx, cmd.ID)
	return stored, false, err
}

func (s *MySQL) MarkDispatched(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE control_commands SET status = ?
		WHERE id = ? AND status = ?`,
		domain.StatusDispatched, id, domain.StatusPending,
	)
	return err
}

func (s *MySQL) Ack(ctx context.Context, id, message string, success bool) (domain.Command, error) {
	status := domain.StatusAcked
	if !success {
		status = domain.StatusFailed
	}
	res, err := s.db.ExecContext(ctx, `
		UPDATE control_commands
		SET status = ?, result_message = ?, acked_at = CURRENT_TIMESTAMP(3)
		WHERE id = ? AND status IN (?, ?)`,
		status, message, id, domain.StatusPending, domain.StatusDispatched,
	)
	if err != nil {
		return domain.Command{}, err
	}
	cmd, err := s.Get(ctx, id)
	if err != nil {
		return domain.Command{}, err
	}
	n, _ := res.RowsAffected()
	if n == 0 && cmd.Status != domain.StatusAcked && cmd.Status != domain.StatusFailed {
		return domain.Command{}, domain.ErrNotFound
	}
	return cmd, nil
}

func (s *MySQL) Get(ctx context.Context, id string) (domain.Command, error) {
	row := s.db.QueryRowContext(ctx, selectSQL+` WHERE id = ?`, id)
	cmd, err := scanCommand(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Command{}, domain.ErrNotFound
	}
	return cmd, err
}

func (s *MySQL) ListByVIN(ctx context.Context, vin string, limit int) ([]domain.Command, error) {
	rows, err := s.db.QueryContext(ctx, selectSQL+` WHERE vin = ? ORDER BY created_at DESC LIMIT ?`, vin, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]domain.Command, 0, limit)
	for rows.Next() {
		cmd, err := scanCommand(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, cmd)
	}
	return out, rows.Err()
}

func (s *MySQL) byIdempotency(ctx context.Context, vin, key string) (domain.Command, error) {
	row := s.db.QueryRowContext(ctx, selectSQL+` WHERE vin = ? AND idempotency_key = ?`, vin, key)
	cmd, err := scanCommand(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Command{}, domain.ErrNotFound
	}
	return cmd, err
}

const selectSQL = `
	SELECT id, vin, command, params_json, idempotency_key, status, result_message, created_at, acked_at
	FROM control_commands`

type scanner interface {
	Scan(dest ...any) error
}

func scanCommand(row scanner) (domain.Command, error) {
	var (
		cmd     domain.Command
		params  sql.NullString
		message sql.NullString
		acked   sql.NullTime
	)
	err := row.Scan(&cmd.ID, &cmd.VIN, &cmd.Name, &params, &cmd.IdempotencyKey, &cmd.Status, &message, &cmd.CreatedAt, &acked)
	if err != nil {
		return domain.Command{}, err
	}
	if params.Valid && params.String != "" {
		cmd.Params = []byte(params.String)
	}
	if message.Valid {
		cmd.Message = message.String
	}
	if acked.Valid {
		t := acked.Time.UTC()
		cmd.AckedAt = &t
	}
	cmd.CreatedAt = cmd.CreatedAt.UTC()
	return cmd, nil
}
