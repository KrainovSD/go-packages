//go:build cgo

package rocksdb

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"sync"
	"testing"
)

func TestOpenClose(t *testing.T) {
	var db, err = NewRocksDB(context.Background(), testDatabaseOptions(t))
	if err != nil {
		t.Fatalf("new rocksdb: %v", err)
	}
	db.Close()
	var reopened *DB
	if reopened, err = NewRocksDB(context.Background(), testDatabaseOptions(t)); err != nil {
		t.Fatalf("reopen rocksdb: %v", err)
	}
	reopened.Close()
}

func TestWritePutReadDelete(t *testing.T) {
	var db = testDatabase(t)
	var err error
	if err = db.Write(3, 0, OPERATION_PUT, []byte("k1"), []byte("v1")); err != nil {
		t.Fatalf("write put: %v", err)
	}
	var single *SingleData
	if single, err = db.Read(3, 0, []byte("k1")); err != nil {
		t.Fatalf("read: %v", err)
	}
	if got := single.Read(); !bytes.Equal(got, []byte("v1")) {
		t.Fatalf("value = %q, want v1", got)
	}
	single.Close()

	if single, err = db.Read(3, 0, []byte("missing")); err != nil {
		t.Fatalf("read missing: %v", err)
	}
	if got := single.Read(); got != nil {
		t.Fatalf("missing value = %q, want nil", got)
	}
	single.Close()

	if err = db.Write(3, 0, OPERATION_DELETE, []byte("k1"), nil); err != nil {
		t.Fatalf("write delete: %v", err)
	}
	if single, err = db.Read(3, 0, []byte("k1")); err != nil {
		t.Fatalf("read after delete: %v", err)
	}
	if got := single.Read(); got != nil {
		t.Fatalf("deleted value = %q, want nil", got)
	}
	single.Close()
}

func TestWriteBatchReadAllDelete(t *testing.T) {
	var db = testDatabase(t)
	var err error
	if err = db.WriteBatch(0, []WriteBatchOptions{
		{Key: []byte("k1"), Value: []byte("v1"), CF: 3, Operation: OPERATION_PUT},
		{Key: []byte("k2"), Value: []byte("v2"), CF: 3, Operation: OPERATION_PUT},
		{Key: []byte("k3"), Value: []byte("v3"), CF: 3, Operation: OPERATION_PUT},
	}); err != nil {
		t.Fatalf("write batch put: %v", err)
	}
	var keys, values = readAllKV(t, db, &ReadOptions{CF: 3, ReturnKeys: true})
	assertKV(t, keys, values,
		[][]byte{[]byte("k1"), []byte("k2"), []byte("k3")},
		[][]byte{[]byte("v1"), []byte("v2"), []byte("v3")})

	if err = db.WriteBatch(0, []WriteBatchOptions{
		{Key: []byte("k1"), CF: 3, Operation: OPERATION_DELETE},
		{Key: []byte("k4"), Value: []byte("v4"), CF: 3, Operation: OPERATION_PUT},
	}); err != nil {
		t.Fatalf("write batch delete + put: %v", err)
	}
	keys, values = readAllKV(t, db, &ReadOptions{CF: 3, ReturnKeys: true})
	assertKV(t, keys, values,
		[][]byte{[]byte("k2"), []byte("k3"), []byte("k4")},
		[][]byte{[]byte("v2"), []byte("v3"), []byte("v4")})
}

func TestReadAllOffset(t *testing.T) {
	var db = testDatabase(t)
	seedWriteBatchKeys(t, db, 3)
	var keys, values = readAllKV(t, db, &ReadOptions{CF: 3, ReturnKeys: true, Offset: 2})
	assertKV(t, keys, values,
		[][]byte{[]byte("k3"), []byte("k4"), []byte("k5")},
		[][]byte{[]byte("v3"), []byte("v4"), []byte("v5")})
}

func TestReadAllLimit(t *testing.T) {
	var db = testDatabase(t)
	seedWriteBatchKeys(t, db, 3)
	var keys, values = readAllKV(t, db, &ReadOptions{CF: 3, ReturnKeys: true, Limit: 2})
	assertKV(t, keys, values,
		[][]byte{[]byte("k1"), []byte("k2")},
		[][]byte{[]byte("v1"), []byte("v2")})
}

func TestReadAllCursor(t *testing.T) {
	var db = testDatabase(t)
	seedWriteBatchKeys(t, db, 3)
	var keys, values = readAllKV(t, db, &ReadOptions{CF: 3, ReturnKeys: true, Cursor: []byte("k2")})
	assertKV(t, keys, values,
		[][]byte{[]byte("k3"), []byte("k4"), []byte("k5")},
		[][]byte{[]byte("v3"), []byte("v4"), []byte("v5")})

	keys, values = readAllKV(t, db, &ReadOptions{
		CF:         3,
		ReturnKeys: true,
		Cursor:     []byte("k1"),
		Offset:     1,
		Limit:      2,
	})
	assertKV(t, keys, values,
		[][]byte{[]byte("k3"), []byte("k4")},
		[][]byte{[]byte("v3"), []byte("v4")})
}

func TestReadAllReverse(t *testing.T) {
	var db = testDatabase(t)
	seedWriteBatchKeys(t, db, 3)
	var keys, values = readAllKV(t, db, &ReadOptions{CF: 3, ReturnKeys: true, Reverse: true})
	assertKV(t, keys, values,
		[][]byte{[]byte("k5"), []byte("k4"), []byte("k3"), []byte("k2"), []byte("k1")},
		[][]byte{[]byte("v5"), []byte("v4"), []byte("v3"), []byte("v2"), []byte("v1")})
}

func TestWriteBatchPrefixSpread(t *testing.T) {
	var db = testDatabase(t)
	var err error
	if err = db.WriteBatch(0, []WriteBatchOptions{
		{Key: []byte("p:1"), Value: []byte("A"), CF: 5, Operation: OPERATION_PUT},
		{Key: []byte("p:2"), Value: []byte("B"), CF: 5, Operation: OPERATION_PUT},
	}); err != nil {
		t.Fatalf("write batch: %v", err)
	}
	var prefixReader uint8 = 1
	if err = db.WriteBatch(0, []WriteBatchOptions{
		{Key: []byte("p:"), Value: []byte("X"), CF: 5, Reader: &prefixReader, Operation: OPERATION_PUT},
	}); err != nil {
		t.Fatalf("write batch spread: %v", err)
	}
	var all *MultipleData
	if all, err = db.ReadAll(&ReadOptions{CF: 5, ReturnKeys: true, Limit: 100}); err != nil {
		t.Fatalf("read all: %v", err)
	}
	for key, value := range all.Iter() {
		if !bytes.Equal(value, []byte("X")) {
			t.Fatalf("key %q value = %q, want X", key, value)
		}
	}
	all.Close()
	var batch *MultipleData
	if batch, err = db.ReadBatchSingle(&ReadBatchSingleOptions{
		Keys: [][]byte{[]byte("p:1"), []byte("p:2")},
		Cf:   5,
	}); err != nil {
		t.Fatalf("read batch single: %v", err)
	}
	for key, value := range batch.Iter() {
		if !bytes.Equal(value, []byte("X")) {
			t.Fatalf("key %q value = %q, want X", key, value)
		}
	}
	batch.Close()
}

func TestWriteBatchDeletePrefix(t *testing.T) {
	var db = testDatabase(t)
	var err error
	if err = db.WriteBatch(0, []WriteBatchOptions{
		{Key: []byte("p:1"), Value: []byte("v1"), CF: 5, Operation: OPERATION_PUT},
		{Key: []byte("p:2"), Value: []byte("v2"), CF: 5, Operation: OPERATION_PUT},
		{Key: []byte("other"), Value: []byte("keep"), CF: 5, Operation: OPERATION_PUT},
	}); err != nil {
		t.Fatalf("write batch put: %v", err)
	}
	var prefixReader uint8 = 1
	if err = db.WriteBatch(0, []WriteBatchOptions{
		{Key: []byte("p:"), CF: 5, Reader: &prefixReader, Operation: OPERATION_DELETE},
	}); err != nil {
		t.Fatalf("write batch delete prefix: %v", err)
	}
	var keys, values = readAllKV(t, db, &ReadOptions{CF: 5, ReturnKeys: true})
	assertKV(t, keys, values, [][]byte{[]byte("other")}, [][]byte{[]byte("keep")})
}

func TestReadBatchSingle(t *testing.T) {
	var db = testDatabase(t)
	var err error
	if err = db.WriteBatch(0, []WriteBatchOptions{
		{Key: []byte("a"), Value: []byte("va"), CF: 3, Operation: OPERATION_PUT},
		{Key: []byte("b"), Value: []byte("vb"), CF: 3, Operation: OPERATION_PUT},
		{Key: []byte("c"), Value: []byte("vc"), CF: 3, Operation: OPERATION_PUT},
	}); err != nil {
		t.Fatalf("write batch put: %v", err)
	}
	var sorted *MultipleData
	if sorted, err = db.ReadBatchSingle(&ReadBatchSingleOptions{
		Keys:       [][]byte{[]byte("a"), []byte("c")},
		Cf:         3,
		SortedKeys: true,
	}); err != nil {
		t.Fatalf("read batch single sorted: %v", err)
	}
	var keys, values [][]byte
	for key, value := range sorted.Iter() {
		keys = append(keys, key)
		values = append(values, value)
	}
	sorted.Close()
	assertKV(t, keys, values, [][]byte{[]byte("a"), []byte("c")}, [][]byte{[]byte("va"), []byte("vc")})

	var unsorted *MultipleData
	if unsorted, err = db.ReadBatchSingle(&ReadBatchSingleOptions{
		Keys: [][]byte{[]byte("c"), []byte("a")},
		Cf:   3,
	}); err != nil {
		t.Fatalf("read batch single unsorted: %v", err)
	}
	keys, values = nil, nil
	for key, value := range unsorted.Iter() {
		keys = append(keys, key)
		values = append(values, value)
	}
	unsorted.Close()
	assertKV(t, keys, values, [][]byte{[]byte("c"), []byte("a")}, [][]byte{[]byte("vc"), []byte("va")})

	var withMissing *MultipleData
	if withMissing, err = db.ReadBatchSingle(&ReadBatchSingleOptions{
		Keys: [][]byte{[]byte("a"), []byte("zz")},
		Cf:   3,
	}); err != nil {
		t.Fatalf("read batch single with missing: %v", err)
	}
	values = nil
	for _, value := range withMissing.Iter() {
		values = append(values, value)
	}
	withMissing.Close()
	if len(values) != 2 {
		t.Fatalf("values = %d, want 2", len(values))
	}
	if !bytes.Equal(values[0], []byte("va")) {
		t.Fatalf("value a = %q, want va", values[0])
	}
	if values[1] != nil {
		t.Fatalf("missing value = %q, want nil", values[1])
	}
}

func TestReadBatchMultiple(t *testing.T) {
	var db = testDatabase(t)
	var err error
	if err = db.WriteBatch(0, []WriteBatchOptions{
		{Key: []byte("k1"), Value: []byte("v1"), CF: 3, Operation: OPERATION_PUT},
		{Key: []byte("k2"), Value: []byte("v2"), CF: 4, Operation: OPERATION_PUT},
	}); err != nil {
		t.Fatalf("write batch put: %v", err)
	}
	var multi *MultipleCFData
	if multi, err = db.ReadBatchMultiple(&ReadBatchMultipleOptions{
		Keys: [][]byte{[]byte("k1"), []byte("k2"), []byte("k3")},
		CFs:  []uint8{3, 4, 4},
	}); err != nil {
		t.Fatalf("read batch multi: %v", err)
	}
	var got []CFData
	for _, data := range multi.Iter() {
		got = append(got, data)
	}
	multi.Close()
	if len(got) != 3 {
		t.Fatalf("batch multi values = %d, want 3", len(got))
	}
	if !bytes.Equal(got[0].Data, []byte("v1")) || got[0].CF != 3 {
		t.Fatalf("k1 = %q cf %d, want v1 cf 3", got[0].Data, got[0].CF)
	}
	if !bytes.Equal(got[1].Data, []byte("v2")) || got[1].CF != 4 {
		t.Fatalf("k2 = %q cf %d, want v2 cf 4", got[1].Data, got[1].CF)
	}
	if got[2].Data != nil {
		t.Fatalf("missing k3 = %q, want nil", got[2].Data)
	}
}

func TestReadDumpWriteDump(t *testing.T) {
	var db = testDatabase(t)
	var err error
	if err = db.WriteBatch(0, []WriteBatchOptions{
		{Key: []byte("d1"), Value: []byte("default"), CF: 0, Operation: OPERATION_PUT},
		{Key: []byte("k1"), Value: []byte("v1"), CF: 3, Operation: OPERATION_PUT},
		{Key: []byte("k2"), Value: []byte("v2"), CF: 3, Operation: OPERATION_PUT},
		{Key: []byte("k1"), Value: []byte("other"), CF: 4, Operation: OPERATION_PUT},
	}); err != nil {
		t.Fatalf("seed write batch: %v", err)
	}

	var wg sync.WaitGroup
	var noiseWriter = func(id int) {
		defer wg.Done()
		var err error
		for i := range 50 {
			if err = db.Write(5, 0, OPERATION_PUT, fmt.Appendf(nil, "w%d-%d", id, i), []byte("noise")); err != nil {
				t.Errorf("noise write: %v", err)
				return
			}
		}
	}
	var seedWriter = func() {
		defer wg.Done()
		var err error
		for range 50 {
			if err = db.WriteBatch(0, []WriteBatchOptions{
				{Key: []byte("d1"), Value: []byte("default"), CF: 0, Operation: OPERATION_PUT},
				{Key: []byte("k1"), Value: []byte("v1"), CF: 3, Operation: OPERATION_PUT},
				{Key: []byte("k2"), Value: []byte("v2"), CF: 3, Operation: OPERATION_PUT},
				{Key: []byte("k1"), Value: []byte("other"), CF: 4, Operation: OPERATION_PUT},
			}); err != nil {
				t.Errorf("seed rewrite: %v", err)
				return
			}
		}
	}
	var reader = func() {
		defer wg.Done()
		var err error
		for range 50 {
			var single *SingleData
			if single, err = db.Read(3, 0, []byte("k1")); err != nil {
				t.Errorf("read k1: %v", err)
				return
			}
			if got := single.Read(); !bytes.Equal(got, []byte("v1")) {
				t.Errorf("read k1 = %q, want v1", got)
				single.Close()
				return
			}
			single.Close()
			var all *MultipleData
			if all, err = db.ReadAll(&ReadOptions{CF: 4, ReturnKeys: true}); err != nil {
				t.Errorf("read all cf4: %v", err)
				return
			}
			all.Close()
		}
	}

	wg.Add(6)
	for id := range 2 {
		go noiseWriter(id)
	}
	for range 4 {
		go reader()
	}
	var dump []byte
	if dump, err = db.ReadDump(0); err != nil {
		t.Fatalf("read dump: %v", err)
	}
	wg.Wait()

	wg.Add(6)
	for range 2 {
		go seedWriter()
	}
	for range 4 {
		go reader()
	}
	if err = db.WriteDump(0, dump, true); err != nil {
		t.Fatalf("write dump force: %v", err)
	}
	wg.Wait()

	var restored []byte
	if restored, err = db.ReadDump(0); err != nil {
		t.Fatalf("read restored dump: %v", err)
	}
	if !bytes.Equal(dump, restored) {
		t.Fatalf("dump after force restore mismatch:\nfirst:  %s\nsecond: %s", dump, restored)
	}
	var single *SingleData
	if single, err = db.Read(4, 0, []byte("k1")); err != nil {
		t.Fatalf("read cf4 after restore: %v", err)
	}
	if got := single.Read(); !bytes.Equal(got, []byte("other")) {
		t.Fatalf("cf4 value after restore = %q, want other", got)
	}
	single.Close()

	var fresh = testDatabase(t)
	if err = fresh.WriteDump(0, dump, false); err != nil {
		t.Fatalf("write dump to fresh db: %v", err)
	}
	var freshDump []byte
	if freshDump, err = fresh.ReadDump(0); err != nil {
		t.Fatalf("read fresh dump: %v", err)
	}
	if !bytes.Equal(dump, freshDump) {
		t.Fatalf("fresh dump mismatch:\nfirst:  %s\nsecond: %s", dump, freshDump)
	}
	if single, err = fresh.Read(3, 0, []byte("k2")); err != nil {
		t.Fatalf("read fresh k2: %v", err)
	}
	if got := single.Read(); !bytes.Equal(got, []byte("v2")) {
		t.Fatalf("fresh k2 value = %q, want v2", got)
	}
	single.Close()
}

func TestDropAndRecreateCF(t *testing.T) {
	var db = testDatabase(t)
	var err error
	if err = db.Write(3, 0, OPERATION_PUT, []byte("k1"), []byte("v1")); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err = db.Write(4, 0, OPERATION_PUT, []byte("k1"), []byte("other")); err != nil {
		t.Fatalf("write cf4: %v", err)
	}

	if err = db.DropAndRecreateCF(3); err != nil {
		t.Fatalf("drop and recreate cf: %v", err)
	}

	var single *SingleData
	if single, err = db.Read(3, 0, []byte("k1")); err != nil {
		t.Fatalf("read after drop: %v", err)
	}
	if got := single.Read(); got != nil {
		t.Fatalf("value after drop = %q, want nil", got)
	}
	single.Close()

	if single, err = db.Read(4, 0, []byte("k1")); err != nil {
		t.Fatalf("read cf4: %v", err)
	}
	if got := single.Read(); !bytes.Equal(got, []byte("other")) {
		t.Fatalf("cf4 value = %q, want other", got)
	}
	single.Close()

	if err = db.Write(3, 0, OPERATION_PUT, []byte("k2"), []byte("v2")); err != nil {
		t.Fatalf("write after drop: %v", err)
	}
	if single, err = db.Read(3, 0, []byte("k2")); err != nil {
		t.Fatalf("read k2: %v", err)
	}
	if got := single.Read(); !bytes.Equal(got, []byte("v2")) {
		t.Fatalf("k2 value = %q, want v2", got)
	}
	single.Close()

	if err = db.DropAndRecreateCF(0); err == nil {
		t.Fatalf("drop default cf: expected error, got nil")
	}
}

func TestDropAndRecreateCFConcurrent(t *testing.T) {
	var db = testDatabase(t)
	var wg sync.WaitGroup
	var write = func(id int) {
		defer wg.Done()
		var err error
		for range 200 {
			if err = db.Write(3, 0, OPERATION_PUT, fmt.Appendf(nil, "k%d", id), []byte("v")); err != nil {
				t.Errorf("write: %v", err)
				return
			}
		}
	}
	wg.Add(4)
	for id := range 4 {
		go write(id)
	}
	var err error
	for range 10 {
		if err = db.DropAndRecreateCF(3); err != nil {
			t.Errorf("drop and recreate: %v", err)
			break
		}
	}
	wg.Wait()
}

func TestAutoIncrement(t *testing.T) {
	var db = testDatabase(t)
	var key1 = make([]byte, 8)
	binary.LittleEndian.PutUint64(key1, 1)
	var key2 = make([]byte, 8)
	binary.LittleEndian.PutUint64(key2, 2)
	var id uint64
	var err error
	for _, want := range []uint64{1, 2, 3} {
		if id, err = db.AutoIncrement(2, 0, 0, key1); err != nil {
			t.Fatalf("auto increment key1: %v", err)
		}
		if id != want {
			t.Fatalf("auto increment key1 = %d, want %d", id, want)
		}
	}
	for _, want := range []uint64{1, 2} {
		if id, err = db.AutoIncrement(2, 0, 0, key2); err != nil {
			t.Fatalf("auto increment key2: %v", err)
		}
		if id != want {
			t.Fatalf("auto increment key2 = %d, want %d", id, want)
		}
	}
	var single *SingleData
	if single, err = db.Read(2, 0, key1); err != nil {
		t.Fatalf("read counter: %v", err)
	}
	var raw = single.Read()
	single.Close()
	if len(raw) != 8 || binary.LittleEndian.Uint64(raw) != 3 {
		t.Fatalf("stored counter = %v, want little endian 3", raw)
	}
}

func TestAutoIncrementConcurrent(t *testing.T) {
	var db = testDatabase(t)
	const goroutinesCount = 8
	const incrementsPerGoroutine = 100
	var key = make([]byte, 8)
	binary.LittleEndian.PutUint64(key, 1)
	var ids = make(chan uint64, goroutinesCount*incrementsPerGoroutine)
	var wg sync.WaitGroup
	var increment = func() {
		defer wg.Done()
		var id uint64
		var err error
		for range incrementsPerGoroutine {
			if id, err = db.AutoIncrement(2, 0, 0, key); err != nil {
				t.Errorf("auto increment: %v", err)
				return
			}
			ids <- id
		}
	}
	wg.Add(goroutinesCount)
	for range goroutinesCount {
		go increment()
	}
	wg.Wait()
	close(ids)
	var unique = make(map[uint64]struct{}, goroutinesCount*incrementsPerGoroutine)
	for id := range ids {
		if _, ok := unique[id]; ok {
			t.Fatalf("duplicated id %d", id)
		}
		if id == 0 || id > goroutinesCount*incrementsPerGoroutine {
			t.Fatalf("id %d out of range 1..%d", id, goroutinesCount*incrementsPerGoroutine)
		}
		unique[id] = struct{}{}
	}
	if len(unique) != goroutinesCount*incrementsPerGoroutine {
		t.Fatalf("unique ids = %d, want %d", len(unique), goroutinesCount*incrementsPerGoroutine)
	}
	var single, err = db.Read(2, 0, key)
	if err != nil {
		t.Fatalf("read counter: %v", err)
	}
	var raw = single.Read()
	single.Close()
	if len(raw) != 8 || binary.LittleEndian.Uint64(raw) != goroutinesCount*incrementsPerGoroutine {
		t.Fatalf("stored counter = %v, want little endian %d", raw, goroutinesCount*incrementsPerGoroutine)
	}
}

func TestConcurrentReadWrite(t *testing.T) {
	var db = testDatabase(t)
	var err error
	var seed = make([]WriteBatchOptions, 0, 100)
	for i := range 50 {
		seed = append(seed,
			WriteBatchOptions{Key: fmt.Appendf(nil, "k%02d", i), Value: fmt.Appendf(nil, "v%d", i), CF: 3, Operation: OPERATION_PUT},
			WriteBatchOptions{Key: fmt.Appendf(nil, "k%02d", i), Value: fmt.Appendf(nil, "v%d", i), CF: 4, Operation: OPERATION_PUT},
		)
	}
	if err = db.WriteBatch(0, seed); err != nil {
		t.Fatalf("seed write batch: %v", err)
	}

	var wg sync.WaitGroup
	var write = func(id int) {
		defer wg.Done()
		var err error
		for i := range 100 {
			var cf uint8 = 3
			if (id+i)%2 == 0 {
				cf = 4
			}
			if err = db.WriteBatch(0, []WriteBatchOptions{{
				Key:       fmt.Appendf(nil, "k%02d", i%50),
				Value:     fmt.Appendf(nil, "v-%d-%d", id, i),
				CF:        cf,
				Operation: OPERATION_PUT,
			}}); err != nil {
				t.Errorf("write batch: %v", err)
				return
			}
		}
	}
	var read = func() {
		defer wg.Done()
		var err error
		for i := range 100 {
			var single *SingleData
			if single, err = db.Read(3, 0, fmt.Appendf(nil, "k%02d", i%50)); err != nil {
				t.Errorf("read: %v", err)
				return
			}
			single.Close()
			if i%10 == 0 {
				var all *MultipleData
				if all, err = db.ReadAll(&ReadOptions{CF: 4, Limit: 10}); err != nil {
					t.Errorf("read all: %v", err)
					return
				}
				all.Close()
			}
		}
	}
	wg.Add(8)
	for id := range 4 {
		go write(id)
	}
	for range 4 {
		go read()
	}
	wg.Wait()
}

func TestErrorPaths(t *testing.T) {
	var db = testDatabase(t)
	var err error

	if _, err = NewRocksDB(context.Background(), DatabaseOptions{}); err == nil {
		t.Fatalf("new rocksdb empty path: expected error, got nil")
	}
	if _, err = db.Read(99, 0, []byte("k1")); err == nil {
		t.Fatalf("read invalid cf: expected error, got nil")
	}
	if _, err = db.Read(3, 99, []byte("k1")); err == nil {
		t.Fatalf("read invalid reader: expected error, got nil")
	}
	if _, err = db.ReadAll(&ReadOptions{CF: 99}); err == nil {
		t.Fatalf("read all invalid cf: expected error, got nil")
	}
	if _, err = db.ReadAll(&ReadOptions{CF: 3, Reader: 99}); err == nil {
		t.Fatalf("read all invalid reader: expected error, got nil")
	}
	if _, err = db.ReadBatchSingle(&ReadBatchSingleOptions{Keys: [][]byte{[]byte("k1")}, Cf: 99}); err == nil {
		t.Fatalf("read batch single invalid cf: expected error, got nil")
	}
	if _, err = db.ReadBatchSingle(&ReadBatchSingleOptions{Keys: [][]byte{[]byte("k1")}, Cf: 3, Reader: 99}); err == nil {
		t.Fatalf("read batch single invalid reader: expected error, got nil")
	}
	if _, err = db.ReadBatchMultiple(&ReadBatchMultipleOptions{Keys: [][]byte{[]byte("k1")}, CFs: []uint8{99}}); err == nil {
		t.Fatalf("read batch multiple invalid cf: expected error, got nil")
	}
	if _, err = db.ReadBatchMultiple(&ReadBatchMultipleOptions{Keys: [][]byte{[]byte("k1")}, CFs: []uint8{3}, Reader: 99}); err == nil {
		t.Fatalf("read batch multiple invalid reader: expected error, got nil")
	}
	if err = db.Write(99, 0, OPERATION_PUT, []byte("k1"), []byte("v1")); err == nil {
		t.Fatalf("write invalid cf: expected error, got nil")
	}
	if err = db.Write(3, 99, OPERATION_PUT, []byte("k1"), []byte("v1")); err == nil {
		t.Fatalf("write invalid writer: expected error, got nil")
	}
	if err = db.WriteBatch(99, []WriteBatchOptions{
		{Key: []byte("k1"), Value: []byte("v1"), CF: 3, Operation: OPERATION_PUT},
	}); err == nil {
		t.Fatalf("write batch invalid writer: expected error, got nil")
	}
	if err = db.WriteBatch(0, []WriteBatchOptions{
		{Key: []byte("k1"), Value: []byte("v1"), CF: 99, Operation: OPERATION_PUT},
	}); err == nil {
		t.Fatalf("write batch invalid cf: expected error, got nil")
	}
	if _, err = db.AutoIncrement(99, 0, 0, []byte("ids")); err == nil {
		t.Fatalf("auto increment invalid cf: expected error, got nil")
	}
	if err = db.DropAndRecreateCF(99); err == nil {
		t.Fatalf("drop and recreate invalid cf: expected error, got nil")
	}

	if err = db.Write(3, 0, OPERATION_PUT, []byte("k1"), []byte("v1")); err != nil {
		t.Fatalf("write after errors: %v", err)
	}
	var single *SingleData
	if single, err = db.Read(3, 0, []byte("k1")); err != nil {
		t.Fatalf("read after errors: %v", err)
	}
	if got := single.Read(); !bytes.Equal(got, []byte("v1")) {
		t.Fatalf("value after errors = %q, want v1", got)
	}
	single.Close()
}

func TestReadAllCursorReverse(t *testing.T) {
	var db = testDatabase(t)
	seedWriteBatchKeys(t, db, 3)
	var keys, values = readAllKV(t, db, &ReadOptions{CF: 3, ReturnKeys: true, Reverse: true, Cursor: []byte("k4")})
	assertKV(t, keys, values,
		[][]byte{[]byte("k3"), []byte("k2"), []byte("k1")},
		[][]byte{[]byte("v3"), []byte("v2"), []byte("v1")})

	keys, values = readAllKV(t, db, &ReadOptions{CF: 3, ReturnKeys: true, Reverse: true, Cursor: []byte("k4"), Limit: 2})
	assertKV(t, keys, values,
		[][]byte{[]byte("k3"), []byte("k2")},
		[][]byte{[]byte("v3"), []byte("v2")})

	keys, values = readAllKV(t, db, &ReadOptions{CF: 3, ReturnKeys: true, Reverse: true, Cursor: []byte("k4"), Offset: 1})
	assertKV(t, keys, values,
		[][]byte{[]byte("k2"), []byte("k1")},
		[][]byte{[]byte("v2"), []byte("v1")})

	keys, values = readAllKV(t, db, &ReadOptions{CF: 3, ReturnKeys: true, Reverse: true, Cursor: []byte("zz")})
	if len(keys) != 0 || len(values) != 0 {
		t.Fatalf("reverse with missing cursor: got %q / %q, want empty", keys, values)
	}
}

func testDatabase(t *testing.T) *DB {
	t.Helper()
	var db, err = NewRocksDB(context.Background(), testDatabaseOptions(t))
	if err != nil {
		t.Fatalf("new rocksdb: %v", err)
	}
	t.Cleanup(db.Close)
	return db
}

func readAllKV(t *testing.T, db *DB, opts *ReadOptions) ([][]byte, [][]byte) {
	t.Helper()
	var data, err = db.ReadAll(opts)
	if err != nil {
		t.Fatalf("read all: %v", err)
	}
	defer data.Close()
	var keys, values [][]byte
	for key, value := range data.Iter() {
		keys = append(keys, append([]byte(nil), key...))
		values = append(values, append([]byte(nil), value...))
	}
	return keys, values
}

func assertKV(t *testing.T, keys, values, wantKeys, wantValues [][]byte) {
	t.Helper()
	if len(keys) != len(wantKeys) || len(values) != len(wantValues) {
		t.Fatalf("got %d pairs (%q / %q), want %d (%q / %q)",
			len(keys), keys, values, len(wantKeys), wantKeys, wantValues)
	}
	for i := range wantKeys {
		if !bytes.Equal(keys[i], wantKeys[i]) {
			t.Fatalf("key %d = %q, want %q", i, keys[i], wantKeys[i])
		}
		if !bytes.Equal(values[i], wantValues[i]) {
			t.Fatalf("value %d = %q, want %q", i, values[i], wantValues[i])
		}
	}
}

func testDatabaseOptions(t *testing.T) DatabaseOptions {
	t.Helper()
	return DatabaseOptions{
		Path:              t.TempDir(),
		ArenaIntitialSize: 4 << 20,
		ArenaTTL:          5,
		Rocks: RocksDBOptions{
			Writers: []WriterOptions{DefaultWriter},
			Readers: []ReaderOptions{DefaultReader, DefaultPrefixReader},
			ColumnFamilies: []ColumnFamilyOptions{
				ColumnFamilyOptions{Name: "migrations", ComparatorType: COMPARATOR_U64_LE}.WithDefaults(),
				ColumnFamilyOptions{Name: "next_ids", ComparatorType: COMPARATOR_U64_LE}.WithDefaults(),
				ColumnFamilyOptions{Name: "tests1"}.WithDefaults(),
				ColumnFamilyOptions{Name: "tests2"}.WithDefaults(),
				ColumnFamilyOptions{Name: "tests3"}.WithDefaults(),
			},
		}.WithDefaults(),
	}
}

func seedWriteBatchKeys(t *testing.T, db *DB, cf uint8) {
	t.Helper()
	var opts = make([]WriteBatchOptions, 0, 5)
	for i := range 5 {
		opts = append(opts, WriteBatchOptions{
			Key:       fmt.Appendf(nil, "k%d", i+1),
			Value:     fmt.Appendf(nil, "v%d", i+1),
			CF:        cf,
			Operation: OPERATION_PUT,
		})
	}
	var err error
	if err = db.WriteBatch(0, opts); err != nil {
		t.Fatalf("seed write batch: %v", err)
	}
}
