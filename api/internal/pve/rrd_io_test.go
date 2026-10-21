package pve

import "testing"

func TestMergeStorageIO(t *testing.T) {
	disk, iops := MergeStorageIO([][]RRDPoint{
		{
			{"time": 100.0, "rd_bytes": 10.0, "wr_bytes": 2.0, "rd_requests": 4.0, "wr_requests": 1.0},
			{"time": 200.0, "rd_bytes": 20.0, "wr_bytes": 3.0, "rd_requests": 5.0, "wr_requests": 2.0},
		},
		{
			{"time": 100.0, "diskread": 5.0, "diskwrite": 1.0, "rd_requests": 1.0, "wr_requests": 1.0},
		},
	})
	if len(disk) != 2 || disk[0]["diskread"] != 15.0 || disk[0]["diskwrite"] != 3.0 {
		t.Fatalf("disk merge: %+v", disk)
	}
	if len(iops) != 2 || iops[0]["rd_requests"] != 5.0 || iops[0]["wr_requests"] != 2.0 {
		t.Fatalf("iops merge: %+v", iops)
	}
}

func TestMergeStorageIOGuestOps(t *testing.T) {
	_, iops := MergeStorageIO([][]RRDPoint{
		{{"time": 100.0, "diskread": 8.0, "diskwrite": 2.0, "ioread": 3.0, "iowrite": 1.0}},
	})
	if len(iops) != 1 || iops[0]["rd_requests"] != 3.0 || iops[0]["wr_requests"] != 1.0 {
		t.Fatalf("guest iops: %+v", iops)
	}
}
