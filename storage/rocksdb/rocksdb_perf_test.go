//go:build cgo

package rocksdb

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

func TestWithPerfGet(t *testing.T) {
	var opts = testDatabaseOptions(t)
	var db, err = NewRocksDB(context.Background(), opts)
	if err != nil {
		t.Fatalf("new rocksdb: %v", err)
	}
	if err = db.Write(3, 0, OPERATION_PUT, []byte("k0"), []byte("value")); err != nil {
		t.Fatalf("write: %v", err)
	}
	db.Close()
	if db, err = NewRocksDB(context.Background(), opts); err != nil {
		t.Fatalf("reopen rocksdb: %v", err)
	}
	defer db.Close()
	var data *SingleData
	var perf *PerfMetrics
	if perf, err = db.WithPerf(PERF_LEVEL_ENABLE_TIME_EXCEPT_FOR_MUTEX, func() error {
		var err error
		if data, err = db.Read(3, 0, []byte("k0")); err != nil {
			return fmt.Errorf("read: %w", err)
		}
		return nil
	}); err != nil {
		t.Fatalf("with perf: %v", err)
	}
	data.Close()
	if perf.Values[PERF_USER_KEY_COMPARISON_COUNT] == 0 {
		t.Fatal("user key comparison count = 0 after sst read")
	}
	if perf.Values[PERF_BLOCK_READ_COUNT] == 0 {
		t.Fatal("block read count = 0 after sst read")
	}
	if perf.Values[PERF_GET_READ_BYTES] == 0 {
		t.Fatal("get read bytes = 0 after sst read")
	}
	if perf.Values[PERF_BLOCK_READ_TIME] == 0 {
		t.Fatal("block read time = 0 with time level")
	}
}

func TestWithPerfWrite(t *testing.T) {
	var opts = testDatabaseOptions(t)
	var db, err = NewRocksDB(context.Background(), opts)
	if err != nil {
		t.Fatalf("new rocksdb: %v", err)
	}
	defer db.Close()
	var perf *PerfMetrics
	if perf, err = db.WithPerf(PERF_LEVEL_ENABLE_TIME_EXCEPT_FOR_MUTEX, func() error {
		return db.Write(3, 0, OPERATION_PUT, []byte("k0"), []byte("value"))
	}); err != nil {
		t.Fatalf("with perf: %v", err)
	}
	if perf.Values[PERF_WRITE_WAL_TIME] == 0 {
		t.Fatal("write wal time = 0 with time level")
	}
	if perf.Values[PERF_WRITE_PRE_AND_POST_PROCESS_TIME] == 0 {
		t.Fatal("write pre and post process time = 0 with time level")
	}
}

func TestWithPerfReset(t *testing.T) {
	var opts = testDatabaseOptions(t)
	var db, err = NewRocksDB(context.Background(), opts)
	if err != nil {
		t.Fatalf("new rocksdb: %v", err)
	}
	defer db.Close()
	if err = db.Write(3, 0, OPERATION_PUT, []byte("k0"), []byte("value")); err != nil {
		t.Fatalf("write: %v", err)
	}
	var perf *PerfMetrics
	if perf, err = db.WithPerf(PERF_LEVEL_ENABLE_COUNT, func() error { return nil }); err != nil {
		t.Fatalf("with perf: %v", err)
	}
	for i := range perf.Values {
		if perf.Values[i] != 0 {
			t.Fatalf("perf metric %d = %d, want 0: context must be reset before fn",
				i, perf.Values[i])
		}
	}
}

func TestWithPerfFnError(t *testing.T) {
	var opts = testDatabaseOptions(t)
	var db, err = NewRocksDB(context.Background(), opts)
	if err != nil {
		t.Fatalf("new rocksdb: %v", err)
	}
	defer db.Close()
	var perf *PerfMetrics
	var fnErr = errors.New("boom")
	if perf, err = db.WithPerf(PERF_LEVEL_ENABLE_COUNT, func() error { return fnErr }); !errors.Is(err, fnErr) {
		t.Fatalf("error = %v, want fn error as-is", err)
	}
	if perf == nil {
		t.Fatal("perf metrics = nil, want collected despite fn error")
	}
}
