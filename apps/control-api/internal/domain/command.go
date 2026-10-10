package domain

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"vehicle-cloud/platform"
)

const (
	StatusPending    = "pending"
	StatusDispatched = "dispatched"
	StatusAcked      = "acked"
	StatusFailed     = "failed"
)

var ErrNotFound = errors.New("not found")

var allowedCommands = map[string]struct{}{
	"door_lock":   {},
	"door_unlock": {},
	"ac_on":       {},
	"ac_off":      {},
	"horn":        {},
	"find_car":    {},
}

// Command 是一条下行车控。幂等键由调用方生成，云端用它挡住网关重试。
type Command struct {
	ID             string          `json:"id"`
	VIN            string          `json:"vin"`
	Name           string          `json:"command"`
	Params         json.RawMessage `json:"params,omitempty"`
	IdempotencyKey string          `json:"idempotency_key"`
	Status         string          `json:"status"`
	Message        string          `json:"message,omitempty"`
	CreatedAt      time.Time       `json:"created_at,omitempty"`
	AckedAt        *time.Time      `json:"acked_at,omitempty"`
}

type CreateInput struct {
	VIN            string          `json:"vin"`
	Command        string          `json:"command"`
	Params         json.RawMessage `json:"params"`
	IdempotencyKey string          `json:"idempotency_key"`
}

type AckInput struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

func NewID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:]), nil
}

func NormalizeCreate(in CreateInput) (Command, error) {
	vin := strings.ToUpper(strings.TrimSpace(in.VIN))
	if err := platform.ValidateVIN(vin); err != nil {
		return Command{}, err
	}
	name := strings.ToLower(strings.TrimSpace(in.Command))
	if _, ok := allowedCommands[name]; !ok {
		return Command{}, fmt.Errorf("不支持的车控指令 %q", in.Command)
	}
	if err := validateKey(in.IdempotencyKey); err != nil {
		return Command{}, err
	}
	params := in.Params
	if len(params) == 0 || string(params) == "null" {
		params = nil
	} else if !json.Valid(params) {
		return Command{}, errors.New("params 不是合法 JSON")
	}
	id, err := NewID()
	if err != nil {
		return Command{}, err
	}
	return Command{
		ID:             id,
		VIN:            vin,
		Name:           name,
		Params:         params,
		IdempotencyKey: in.IdempotencyKey,
		Status:         StatusPending,
	}, nil
}

func validateKey(key string) error {
	if len(key) < 8 || len(key) > 64 {
		return errors.New("idempotency_key 长度必须在 8 到 64 之间")
	}
	for _, r := range key {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_', r == '-':
		default:
			return errors.New("idempotency_key 只能包含字母、数字、下划线和中划线")
		}
	}
	return nil
}

func ValidID(id string) bool {
	if len(id) != 36 {
		return false
	}
	for i, r := range id {
		switch i {
		case 8, 13, 18, 23:
			if r != '-' {
				return false
			}
		default:
			if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) {
				return false
			}
		}
	}
	return true
}
