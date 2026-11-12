//go:build cgo

package rocksdb

import (
	"context"
	"fmt"
	"testing"
)

func TestStats(t *testing.T) {
	var path = t.TempDir()
	var opts = testDatabaseOptions(t)
	opts.Path = path
	var db, err = NewRocksDB(context.Background(), opts)
	if err != nil {
		t.Fatalf("new rocksdb: %v", err)
	}
	var stats *Stats
	if stats, err = db.Stats(); err != nil {
		t.Fatalf("stats: %v", err)
	}
	if len(stats.ColumnFamilies) != len(opts.Rocks.ColumnFamilies)+1 {
		t.Fatalf("cf stats len = %d, want %d", len(stats.ColumnFamilies), len(opts.Rocks.ColumnFamilies)+1)
	}
	if stats.DB.BackgroundErrors != 0 {
		t.Fatalf("background errors = %d, want 0", stats.DB.BackgroundErrors)
	}
	if stats.DB.IsWriteStopped {
		t.Fatal("write stopped = true, want false")
	}
	for i := range 5 {
		if err = db.Write(3, 0, OPERATION_PUT, fmt.Appendf(nil, "k%d", i), []byte("value")); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	if stats, err = db.Stats(); err != nil {
		t.Fatalf("stats after writes: %v", err)
	}
	var cfStats = stats.ColumnFamilies[3]
	if cfStats.NumEntriesActiveMemTable < 5 {
		t.Fatalf("active memtable entries = %d, want >= 5", cfStats.NumEntriesActiveMemTable)
	}
	if cfStats.EstimateNumKeys < 5 {
		t.Fatalf("estimate num keys = %d, want >= 5", cfStats.EstimateNumKeys)
	}
	if cfStats.CurSizeActiveMemTable == 0 {
		t.Fatal("active memtable size = 0")
	}
	if cfStats.BlockCacheCapacity == 0 {
		t.Fatal("block cache capacity = 0")
	}
	if stats.ColumnFamilies[0].EstimateNumKeys != 0 {
		t.Fatalf("default cf estimate num keys = %d, want 0", stats.ColumnFamilies[0].EstimateNumKeys)
	}
	db.Close()

	opts = testDatabaseOptions(t)
	opts.Path = path
	var reopened *DB
	if reopened, err = NewRocksDB(context.Background(), opts); err != nil {
		t.Fatalf("reopen rocksdb: %v", err)
	}
	if stats, err = reopened.Stats(); err != nil {
		t.Fatalf("stats after reopen: %v", err)
	}
	cfStats = stats.ColumnFamilies[3]
	if cfStats.LiveSSTFilesSize == 0 {
		t.Fatal("live sst files size = 0 after flush on close")
	}
	var files uint64
	for _, level := range cfStats.NumFilesAtLevel {
		files += level
	}
	if files == 0 {
		t.Fatal("num files at levels = 0 after flush on close")
	}
	reopened.Close()
}

const (
	statsSharedCacheSize = uint64(16 << 20)
	statsOwnCacheSize    = uint64(8 << 20)
)

func TestStatsMemoryUsageAllShared(t *testing.T) {
	var opts = testDatabaseOptions(t)
	opts.Rocks.BlockCacheSize = new(uint64(statsSharedCacheSize))
	var stats = reopenWithStats(t, opts)
	checkMemoryUsage(t, stats)
	var usage uint64
	for i, cfStats := range stats.ColumnFamilies {
		if cfStats.BlockCacheCapacity != statsSharedCacheSize {
			t.Fatalf("cf %d capacity = %d, want %d", i, cfStats.BlockCacheCapacity, statsSharedCacheSize)
		}
		usage = cfStats.BlockCacheUsage
	}
	if usage == 0 {
		t.Fatal("shared cache usage = 0")
	}
	for i, cfStats := range stats.ColumnFamilies {
		if cfStats.BlockCacheUsage != usage {
			t.Fatalf("cf %d usage = %d, want %d (same shared cache)", i, cfStats.BlockCacheUsage, usage)
		}
	}
	if stats.Memory.CacheTotal != usage {
		t.Fatalf("cache total = %d, want single shared usage %d", stats.Memory.CacheTotal, usage)
	}
}

func TestStatsMemoryUsageMixed(t *testing.T) {
	var opts = testDatabaseOptions(t)
	opts.Rocks.BlockCacheSize = new(uint64(statsSharedCacheSize))
	opts.Rocks.ColumnFamilies[3].Table.BlockCacheSize = new(uint64(statsOwnCacheSize))
	var stats = reopenWithStats(t, opts)
	checkMemoryUsage(t, stats)
	var ownUsage = stats.ColumnFamilies[4].BlockCacheUsage
	var sharedUsage uint64
	for i, cfStats := range stats.ColumnFamilies {
		var want = statsSharedCacheSize
		if i == 4 {
			want = statsOwnCacheSize
		}
		if cfStats.BlockCacheCapacity != want {
			t.Fatalf("cf %d capacity = %d, want %d", i, cfStats.BlockCacheCapacity, want)
		}
		if i != 4 {
			sharedUsage = cfStats.BlockCacheUsage
		}
	}
	if ownUsage == 0 {
		t.Fatal("own cache usage = 0")
	}
	if sharedUsage == 0 {
		t.Fatal("shared cache usage = 0")
	}
	if stats.Memory.CacheTotal != ownUsage+sharedUsage {
		t.Fatalf("cache total = %d, want %d", stats.Memory.CacheTotal, ownUsage+sharedUsage)
	}
}

func TestStatsMemoryUsageAllOwn(t *testing.T) {
	var opts = testDatabaseOptions(t)
	opts.Rocks.BlockCacheSize = nil
	for i := range opts.Rocks.ColumnFamilies {
		opts.Rocks.ColumnFamilies[i].Table.BlockCacheSize = new(uint64(statsOwnCacheSize))
	}
	var stats = reopenWithStats(t, opts)
	checkMemoryUsage(t, stats)
	var total uint64
	for i, cfStats := range stats.ColumnFamilies {
		if i > 0 && cfStats.BlockCacheCapacity != statsOwnCacheSize {
			t.Fatalf("cf %d capacity = %d, want %d", i, cfStats.BlockCacheCapacity, statsOwnCacheSize)
		}
		total += cfStats.BlockCacheUsage
	}
	if stats.ColumnFamilies[3].BlockCacheUsage == 0 || stats.ColumnFamilies[4].BlockCacheUsage == 0 {
		t.Fatal("own cache usage of written cfs = 0")
	}
	if stats.Memory.CacheTotal != total {
		t.Fatalf("cache total = %d, want %d", stats.Memory.CacheTotal, total)
	}
}

func reopenWithStats(t *testing.T, opts DatabaseOptions) *Stats {
	t.Helper()
	var db, err = NewRocksDB(context.Background(), opts)
	if err != nil {
		t.Fatalf("new rocksdb: %v", err)
	}
	for i := range 3 {
		var key = fmt.Appendf(nil, "k%d", i)
		if err = db.Write(3, 0, OPERATION_PUT, key, []byte("value")); err != nil {
			t.Fatalf("write cf 3: %v", err)
		}
		if err = db.Write(4, 0, OPERATION_PUT, key, []byte("value")); err != nil {
			t.Fatalf("write cf 4: %v", err)
		}
	}
	db.Close()
	if db, err = NewRocksDB(context.Background(), opts); err != nil {
		t.Fatalf("reopen rocksdb: %v", err)
	}
	var stats *Stats
	if stats, err = db.Stats(); err != nil {
		t.Fatalf("stats: %v", err)
	}
	db.Close()
	return stats
}

func checkMemoryUsage(t *testing.T, stats *Stats) {
	t.Helper()
	if stats.Memory.MemTableTotal == 0 {
		t.Fatal("mem table total = 0")
	}
	if stats.Memory.MemTableUnflushed == 0 {
		t.Fatal("mem table unflushed = 0")
	}
	if stats.Memory.TableReaders == 0 {
		t.Fatal("table readers = 0")
	}
	if stats.Memory.CacheTotal == 0 {
		t.Fatal("cache total = 0")
	}
}
