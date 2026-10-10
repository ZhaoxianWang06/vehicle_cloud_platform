package domain

import (
	"strings"
	"testing"
	"time"
)

func valid() Record {
	return Record{
		VIN:        "LSVAA4182E2123456",
		SpeedKmh:   60,
		BatteryPct: 80,
		Lng:        121.47,
		Lat:        31.23,
		OdometerKm: 15000,
		ReportedAt: time.Date(2026, 10, 9, 8, 0, 0, 0, time.UTC),
	}
}

func TestNormalizeOK(t *testing.T) {
	got, err := Normalize(valid())
	if err != nil {
		t.Fatal(err)
	}
	if got.VIN != "LSVAA4182E2123456" {
		t.Fatalf("vin = %s", got.VIN)
	}
}

func TestNormalizeRejects(t *testing.T) {
	cases := []func(*Record){
		func(r *Record) { r.VIN = "BAD" },
		func(r *Record) { r.SpeedKmh = 301 },
		func(r *Record) { r.BatteryPct = -1 },
		func(r *Record) { r.Lat = 91 },
		func(r *Record) { r.Lng = 181 },
		func(r *Record) { r.OdometerKm = -1 },
	}
	for _, mutate := range cases {
		in := valid()
		mutate(&in)
		if _, err := Normalize(in); err == nil {
			t.Fatalf("expected error for %+v", in)
		}
	}
}

func TestNormalizeUppercasesVINAndFillsTime(t *testing.T) {
	in := valid()
	in.VIN = strings.ToLower(in.VIN)
	in.ReportedAt = time.Time{}
	got, err := Normalize(in)
	if err != nil {
		t.Fatal(err)
	}
	if got.VIN != "LSVAA4182E2123456" || got.ReportedAt.IsZero() {
		t.Fatalf("%+v", got)
	}
}
