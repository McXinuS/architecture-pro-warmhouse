package main

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Reading struct {
	DeviceID   string    `json:"-"`
	DeviceType string    `json:"-"`
	Metric     string    `json:"metric"`
	Value      float64   `json:"value"`
	Unit       string    `json:"unit"`
	Ts         time.Time `json:"ts"`
}

// Store keeps telemetry readings in the service's own database.
type Store struct {
	pool *pgxpool.Pool
}

func NewStore(ctx context.Context, url string) (*Store, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	_, err = pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS telemetry (
			id BIGSERIAL PRIMARY KEY,
			device_id UUID NOT NULL,
			device_type VARCHAR(50) NOT NULL,
			metric VARCHAR(50) NOT NULL,
			value DOUBLE PRECISION NOT NULL,
			unit VARCHAR(20),
			ts TIMESTAMPTZ NOT NULL,
			received_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		CREATE INDEX IF NOT EXISTS idx_telemetry_device_ts ON telemetry (device_id, ts DESC);
	`)
	if err != nil {
		pool.Close()
		return nil, err
	}
	return &Store{pool: pool}, nil
}

func (s *Store) Close() {
	s.pool.Close()
}

func (s *Store) Save(ctx context.Context, readings []Reading) error {
	for _, r := range readings {
		_, err := s.pool.Exec(ctx,
			`INSERT INTO telemetry (device_id, device_type, metric, value, unit, ts) VALUES ($1, $2, $3, $4, $5, $6)`,
			r.DeviceID, r.DeviceType, r.Metric, r.Value, r.Unit, r.Ts)
		if err != nil {
			return err
		}
	}
	return nil
}

// Latest returns the most recent reading of every metric of the device.
func (s *Store) Latest(ctx context.Context, deviceID string) ([]Reading, error) {
	return s.query(ctx, `
		SELECT DISTINCT ON (metric) device_id::text, device_type, metric, value, unit, ts
		FROM telemetry WHERE device_id = $1
		ORDER BY metric, ts DESC`, deviceID)
}

// Point is a raw reading (Value) or an aggregate over an interval (Avg, Min, Max).
type Point struct {
	Ts    time.Time `json:"ts"`
	Value *float64  `json:"value,omitempty"`
	Avg   *float64  `json:"avg,omitempty"`
	Min   *float64  `json:"min,omitempty"`
	Max   *float64  `json:"max,omitempty"`
}

// truncUnits maps API intervals to PostgreSQL date_trunc units.
var truncUnits = map[string]string{"1m": "minute", "1h": "hour", "1d": "day"}

const maxRawPoints = 10000

// History returns the metric of the device in [from, to), oldest first, and its unit.
func (s *Store) History(ctx context.Context, deviceID, metric string, from, to time.Time, interval string) ([]Point, string, error) {
	var unit string
	err := s.pool.QueryRow(ctx, `
		SELECT COALESCE(unit, '') FROM telemetry
		WHERE device_id = $1 AND metric = $2 ORDER BY ts DESC LIMIT 1`, deviceID, metric).Scan(&unit)
	if err != nil && err != pgx.ErrNoRows {
		return nil, "", err
	}

	var rows pgx.Rows
	if interval == "raw" {
		rows, err = s.pool.Query(ctx, `
			SELECT ts, value FROM telemetry
			WHERE device_id = $1 AND metric = $2 AND ts >= $3 AND ts < $4
			ORDER BY ts LIMIT $5`, deviceID, metric, from, to, maxRawPoints)
	} else {
		rows, err = s.pool.Query(ctx, `
			SELECT date_trunc($5, ts) AS bucket, avg(value), min(value), max(value) FROM telemetry
			WHERE device_id = $1 AND metric = $2 AND ts >= $3 AND ts < $4
			GROUP BY bucket ORDER BY bucket`, deviceID, metric, from, to, truncUnits[interval])
	}
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	points := []Point{}
	for rows.Next() {
		var p Point
		if interval == "raw" {
			p.Value = new(float64)
			err = rows.Scan(&p.Ts, p.Value)
		} else {
			p.Avg, p.Min, p.Max = new(float64), new(float64), new(float64)
			err = rows.Scan(&p.Ts, p.Avg, p.Min, p.Max)
		}
		if err != nil {
			return nil, "", err
		}
		points = append(points, p)
	}
	return points, unit, rows.Err()
}

func (s *Store) query(ctx context.Context, sql string, args ...any) ([]Reading, error) {
	rows, err := s.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	readings := []Reading{}
	for rows.Next() {
		var r Reading
		var unit *string
		if err := rows.Scan(&r.DeviceID, &r.DeviceType, &r.Metric, &r.Value, &unit, &r.Ts); err != nil {
			return nil, err
		}
		if unit != nil {
			r.Unit = *unit
		}
		readings = append(readings, r)
	}
	return readings, rows.Err()
}
