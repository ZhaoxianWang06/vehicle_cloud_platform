package domain

import (
	"errors"
	"strings"
	"time"

	"vehicle-cloud/platform"
)

var ErrNotFound = errors.New("not found")

// Record 是一条车况。云端以 reported_at 作为车辆时钟，ingested_at 由数据库生成。
type Record struct {
	VIN        string    `json:"vin"`
	SpeedKmh   float64   `json:"speed_kmh"`
	BatteryPct float64   `json:"battery_pct"`
	Lng        float64   `json:"lng"`
	Lat        float64   `json:"lat"`
	OdometerKm float64   `json:"odometer_km"`
	ReportedAt time.Time `json:"reported_at"`
}

type Latest struct {
	Record
	Source string `json:"source"`
}

func Normalize(in Record) (Record, error) {
	in.VIN = strings.ToUpper(strings.TrimSpace(in.VIN))
	if err := platform.ValidateVIN(in.VIN); err != nil {
		return Record{}, err
	}
	if in.SpeedKmh < 0 || in.SpeedKmh > 300 {
		return Record{}, errors.New("speed_kmh 必须在 0 到 300 之间")
	}
	if in.BatteryPct < 0 || in.BatteryPct > 100 {
		return Record{}, errors.New("battery_pct 必须在 0 到 100 之间")
	}
	if in.Lng < -180 || in.Lng > 180 {
		return Record{}, errors.New("lng 超出范围")
	}
	if in.Lat < -90 || in.Lat > 90 {
		return Record{}, errors.New("lat 超出范围")
	}
	if in.OdometerKm < 0 || in.OdometerKm > 10_000_000 {
		return Record{}, errors.New("odometer_km 超出范围")
	}
	if in.ReportedAt.IsZero() {
		in.ReportedAt = time.Now().UTC()
	} else {
		in.ReportedAt = in.ReportedAt.UTC()
	}
	return in, nil
}
