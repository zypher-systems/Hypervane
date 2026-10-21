package store

import (
	"context"
	"fmt"
	"time"
)

type IOPSPoint struct {
	Time time.Time
	Rd   float64
	Wr   float64
	Wait float64
}

func IOPSWindow(timeframe string) time.Duration {
	switch timeframe {
	case "day":
		return 24 * time.Hour
	case "week":
		return 7 * 24 * time.Hour
	case "month":
		return 32 * 24 * time.Hour
	case "year":
		return 366 * 24 * time.Hour
	default:
		return 70 * time.Minute
	}
}

func IOPSStep(timeframe string) time.Duration {
	switch timeframe {
	case "day":
		return time.Minute
	case "week":
		return 5 * time.Minute
	case "month":
		return 30 * time.Minute
	case "year":
		return 6 * time.Hour
	default:
		return 10 * time.Second
	}
}

func (s *Store) WriteNodeIOPS(ctx context.Context, connectionID, node string, ts time.Time, rd, wr float64) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO node_iops (connection_id, node, ts, rd, wr)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (connection_id, node, ts) DO UPDATE SET rd = EXCLUDED.rd, wr = EXCLUDED.wr
	`, connectionID, node, ts.UTC().Truncate(time.Second), rd, wr)
	return err
}

func (s *Store) QueryNodeIOPS(ctx context.Context, connectionID, node string, from, to time.Time, step time.Duration) ([]IOPSPoint, error) {
	if step <= 0 {
		step = 10 * time.Second
	}
	rows, err := s.pool.Query(ctx, `
		SELECT to_timestamp(floor(extract(epoch FROM ts) / $4) * $4) AS bucket,
		       avg(rd), avg(wr)
		FROM node_iops
		WHERE connection_id = $5 AND node = $1 AND ts >= $2 AND ts < $3
		GROUP BY 1
		ORDER BY 1
	`, node, from.UTC(), to.UTC(), step.Seconds(), connectionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []IOPSPoint
	for rows.Next() {
		var p IOPSPoint
		if err := rows.Scan(&p.Time, &p.Rd, &p.Wr); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) PruneNodeIOPS(ctx context.Context, olderThan time.Time) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM node_iops WHERE ts < $1`, olderThan.UTC())
	if err != nil {
		return fmt.Errorf("prune node_iops: %w", err)
	}
	return nil
}

func (s *Store) WriteGuestIOPS(ctx context.Context, connectionID string, vmid int, ts time.Time, rd, wr, wait float64) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO guest_iops (connection_id, vmid, ts, rd, wr, wait)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (connection_id, vmid, ts) DO UPDATE SET rd = EXCLUDED.rd, wr = EXCLUDED.wr, wait = EXCLUDED.wait
	`, connectionID, vmid, ts.UTC().Truncate(time.Second), rd, wr, wait)
	return err
}

func (s *Store) QueryGuestIOPS(ctx context.Context, connectionID string, vmid int, from, to time.Time, step time.Duration) ([]IOPSPoint, error) {
	if step <= 0 {
		step = 10 * time.Second
	}
	rows, err := s.pool.Query(ctx, `
		SELECT to_timestamp(floor(extract(epoch FROM ts) / $4) * $4) AS bucket,
		       avg(rd), avg(wr), avg(wait)
		FROM guest_iops
		WHERE connection_id = $5 AND vmid = $1 AND ts >= $2 AND ts < $3
		GROUP BY 1
		ORDER BY 1
	`, vmid, from.UTC(), to.UTC(), step.Seconds(), connectionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []IOPSPoint
	for rows.Next() {
		var p IOPSPoint
		if err := rows.Scan(&p.Time, &p.Rd, &p.Wr, &p.Wait); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

type GuestIOPSLatest struct {
	VMID int
	Rd   float64
	Wr   float64
	Wait float64
}

func (s *Store) LatestGuestIOPS(ctx context.Context, connectionID string) (map[int]GuestIOPSLatest, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT DISTINCT ON (vmid) vmid, rd, wr, wait
		FROM guest_iops
		WHERE connection_id = $1
		ORDER BY vmid, ts DESC
	`, connectionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int]GuestIOPSLatest{}
	for rows.Next() {
		var p GuestIOPSLatest
		if err := rows.Scan(&p.VMID, &p.Rd, &p.Wr, &p.Wait); err != nil {
			return nil, err
		}
		out[p.VMID] = p
	}
	return out, rows.Err()
}

func (s *Store) PruneGuestIOPS(ctx context.Context, olderThan time.Time) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM guest_iops WHERE ts < $1`, olderThan.UTC())
	if err != nil {
		return fmt.Errorf("prune guest_iops: %w", err)
	}
	return nil
}
