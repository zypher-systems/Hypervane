package pve

import "time"

func SumBlockOps(stat map[string]BlockDeviceStats) (rd, wr int64, ok bool) {
	if len(stat) == 0 {
		return 0, 0, false
	}
	for _, d := range stat {
		rd += int64(d.RdOperations)
		wr += int64(d.WrOperations)
	}
	return rd, wr, true
}

func IOPSRate(prev, now int64, dt time.Duration) (float64, bool) {
	if dt <= 0 || dt > 90*time.Second || now < prev {
		return 0, false
	}
	return float64(now-prev) / dt.Seconds(), true
}

func SumBlockTimeNS(stat map[string]BlockDeviceStats) (int64, bool) {
	if len(stat) == 0 {
		return 0, false
	}
	var ns int64
	for _, d := range stat {
		ns += int64(d.RdTotalTimeNS + d.WrTotalTimeNS + d.FlushTotalTimeNS)
	}
	return ns, true
}

func IOWaitFrac(prev, now int64, dt time.Duration) (float64, bool) {
	if dt <= 0 || dt > 90*time.Second || now < prev {
		return 0, false
	}
	den := float64(dt.Nanoseconds())
	if den <= 0 {
		return 0, false
	}
	frac := float64(now-prev) / den
	if frac < 0 {
		return 0, false
	}
	if frac > 1 {
		frac = 1
	}
	return frac, true
}
