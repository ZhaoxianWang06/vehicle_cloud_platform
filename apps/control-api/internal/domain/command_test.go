package domain

import "testing"

func TestNormalizeCreate(t *testing.T) {
	cmd, err := NormalizeCreate(CreateInput{
		VIN:            "lsvaa4182e2123456",
		Command:        "DOOR_LOCK",
		IdempotencyKey: "req-0001-lock",
	})
	if err != nil {
		t.Fatal(err)
	}
	if cmd.VIN != "LSVAA4182E2123456" || cmd.Name != "door_lock" || cmd.Status != StatusPending {
		t.Fatalf("%+v", cmd)
	}
	if !ValidID(cmd.ID) {
		t.Fatalf("id %s", cmd.ID)
	}
}

func TestNormalizeCreateRejects(t *testing.T) {
	_, err := NormalizeCreate(CreateInput{VIN: "LSVAA4182E2123456", Command: "explode", IdempotencyKey: "req-0001-lock"})
	if err == nil {
		t.Fatal("expected unsupported command")
	}
	_, err = NormalizeCreate(CreateInput{VIN: "LSVAA4182E2123456", Command: "horn", IdempotencyKey: "short"})
	if err == nil {
		t.Fatal("expected bad idempotency key")
	}
}
