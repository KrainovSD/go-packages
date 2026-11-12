//go:build cgo

package rocksdb

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestMetrics(t *testing.T) {
	var opts = testDatabaseOptions(t)
	if opts.Rocks.StatisticsLevel == nil || *opts.Rocks.StatisticsLevel != STATISTICS_LEVEL_EXCEPT_DETAILED_TIMERS {
		t.Fatal("statistics level default is not level 3")
	}
	var db, err = NewRocksDB(context.Background(), opts)
	if err != nil {
		t.Fatalf("new rocksdb: %v", err)
	}
	for i := range 3 {
		var key = fmt.Appendf(nil, "k%d", i)
		if err = db.Write(3, 0, OPERATION_PUT, key, []byte("value")); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	var metrics *Metrics
	if metrics, err = db.Metrics(); err != nil {
		t.Fatalf("metrics: %v", err)
	}
	if metrics.Tickers[TICKER_NUMBER_KEYS_WRITTEN] < 3 {
		t.Fatalf("number keys written = %d, want >= 3",
			metrics.Tickers[TICKER_NUMBER_KEYS_WRITTEN])
	}
	if metrics.Tickers[TICKER_BYTES_WRITTEN] == 0 {
		t.Fatal("bytes written = 0")
	}
	var writeHg = metrics.Histograms[HISTOGRAM_DB_WRITE]
	if writeHg.Count < 3 {
		t.Fatalf("db write histogram count = %d, want >= 3", writeHg.Count)
	}
	if writeHg.Sum == 0 || writeHg.Max == 0 {
		t.Fatalf("db write histogram sum = %d, max = %f, want nonzero",
			writeHg.Sum, writeHg.Max)
	}
	db.Close()

	if db, err = NewRocksDB(context.Background(), opts); err != nil {
		t.Fatalf("reopen rocksdb: %v", err)
	}
	if metrics, err = db.Metrics(); err != nil {
		t.Fatalf("metrics after reopen: %v", err)
	}
	if metrics.Tickers[TICKER_NUMBER_KEYS_WRITTEN] != 0 {
		t.Fatalf("number keys written after reopen = %d, want 0: metrics must not survive reopen",
			metrics.Tickers[TICKER_NUMBER_KEYS_WRITTEN])
	}
	var data *SingleData
	if data, err = db.Read(3, 0, []byte("k0")); err != nil {
		t.Fatalf("read: %v", err)
	}
	data.Close()
	if metrics, err = db.Metrics(); err != nil {
		t.Fatalf("metrics after read: %v", err)
	}
	var cacheOps = metrics.Tickers[TICKER_BLOCK_CACHE_MISS] +
		metrics.Tickers[TICKER_BLOCK_CACHE_HIT] +
		metrics.Tickers[TICKER_BLOCK_CACHE_ADD]
	if cacheOps == 0 {
		t.Fatal("block cache tickers = 0 after sst read")
	}
	if metrics.Tickers[TICKER_NUMBER_KEYS_READ] == 0 {
		t.Fatal("number keys read = 0 after sst read")
	}
	db.Close()
}

func TestMetricsDisabled(t *testing.T) {
	var opts = testDatabaseOptions(t)
	opts.Rocks.StatisticsLevel = new(STATISTICS_LEVEL_DISABLE_ALL)
	var db, err = NewRocksDB(context.Background(), opts)
	if err != nil {
		t.Fatalf("new rocksdb: %v", err)
	}
	defer db.Close()
	if err = db.Write(3, 0, OPERATION_PUT, []byte("k0"), []byte("value")); err != nil {
		t.Fatalf("write: %v", err)
	}
	var metrics *Metrics
	if metrics, err = db.Metrics(); err == nil {
		t.Fatal("metrics with disabled statistics: no error")
	}
	if !errors.Is(err, ErrMetricsDisabled) {
		t.Fatalf("error = %v, want ErrMetricsDisabled", err)
	}
	if metrics != nil {
		t.Fatal("metrics = non-nil, want nil")
	}
}

func TestMetricsInvalidLevel(t *testing.T) {
	var opts = testDatabaseOptions(t)
	opts.Rocks.StatisticsLevel = new(StatisticsLevel(6))
	var _, err = NewRocksDB(context.Background(), opts)
	if err == nil {
		t.Fatal("new rocksdb with level 6: no error")
	}
	if !strings.Contains(err.Error(), "statistics level") {
		t.Fatalf("error = %q, want statistics level range message", err)
	}
}
