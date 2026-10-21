package store

import (
	"context"
	"time"
)

type MetricSample struct {
	Time    time.Time `json:"time"`
	CPU     float64   `json:"cpu"`
	Mem     float64   `json:"mem"`
	MaxMem  float64   `json:"maxmem"`
	Disk    float64   `json:"disk"`
	MaxDisk float64   `json:"maxdisk"`
	NetIn   float64   `json:"netin"`
	NetOut  float64   `json:"netout"`
}

func (s *Store) WriteMetricSample(ctx context.Context, connectionID, resourceID string, ts time.Time, cpu, mem, maxmem, disk, maxdisk, netin, netout float64) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO metric_history (connection_id, resource_id, ts, cpu, mem, maxmem, disk, maxdisk, netin, netout)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (connection_id, resource_id, ts) DO UPDATE SET
			cpu = EXCLUDED.cpu, mem = EXCLUDED.mem, maxmem = EXCLUDED.maxmem,
			disk = EXCLUDED.disk, maxdisk = EXCLUDED.maxdisk, netin = EXCLUDED.netin, netout = EXCLUDED.netout
	`, connectionID, resourceID, ts.UTC().Truncate(time.Minute), cpu, mem, maxmem, disk, maxdisk, netin, netout)
	return err
}

func (s *Store) QueryMetricHistory(ctx context.Context, connectionID, resourceID string, from, to time.Time) ([]MetricSample, error) {
	if to.IsZero() {
		to = time.Now()
	}
	if from.IsZero() {
		from = to.Add(-24 * time.Hour)
	}
	rows, err := s.pool.Query(ctx, `
		SELECT ts, cpu, mem, maxmem, disk, maxdisk, netin, netout
		FROM metric_history
		WHERE connection_id = $1 AND resource_id = $2 AND ts >= $3 AND ts < $4
		ORDER BY ts
	`, connectionID, resourceID, from.UTC(), to.UTC())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MetricSample
	for rows.Next() {
		var p MetricSample
		if err := rows.Scan(&p.Time, &p.CPU, &p.Mem, &p.MaxMem, &p.Disk, &p.MaxDisk, &p.NetIn, &p.NetOut); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) PruneMetricHistory(ctx context.Context, olderThan time.Time) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM metric_history WHERE ts < $1`, olderThan.UTC())
	return err
}
