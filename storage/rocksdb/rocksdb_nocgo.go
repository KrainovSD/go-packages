//go:build !cgo

package rocksdb

import (
	"context"
	"iter"
)

type DB struct{}

func NewRocksDB(ctx context.Context, opts DatabaseOptions) (*DB, error) {
	return nil, ErrCgoRequired
}

func (db *DB) Close() {}

func (db *DB) Read(cf uint8, reader uint8, key []byte) (*SingleData, error) {
	return nil, ErrCgoRequired
}

func (db *DB) ReadAll(opts *ReadOptions) (*MultipleData, error) {
	return nil, ErrCgoRequired
}

func (db *DB) ReadBatchSingle(opts *ReadBatchSingleOptions) (*MultipleData, error) {
	return nil, ErrCgoRequired
}

func (db *DB) ReadBatchMultiple(opts *ReadBatchMultipleOptions) (*MultipleCFData, error) {
	return nil, ErrCgoRequired
}

func (db *DB) Write(cf uint8, writer uint8, operation Operation, key []byte, value []byte) error {
	return ErrCgoRequired
}

func (db *DB) WriteBatch(writer uint8, opts []WriteBatchOptions) error {
	return ErrCgoRequired
}

func (db *DB) ReadDump(reader uint8) ([]byte, error) {
	return nil, ErrCgoRequired
}

func (db *DB) WriteDump(writer uint8, dump []byte, force bool) error {
	return ErrCgoRequired
}

func (db *DB) AutoIncrement(cf uint8, reader uint8, writer uint8, key []byte) (uint64, error) {
	return 0, ErrCgoRequired
}

func (db *DB) DropAndRecreateCF(cf uint8) error {
	return ErrCgoRequired
}

func (db *DB) Stats() (*Stats, error) {
	return nil, ErrCgoRequired
}

func (db *DB) Metrics() (*Metrics, error) {
	return nil, ErrCgoRequired
}

type SingleData struct{}

func (d *SingleData) Close() {}

func (d *SingleData) Read() []byte {
	return nil
}

type MultipleData struct{}

func (d *MultipleData) Close() {}

func (d *MultipleData) Iter() iter.Seq2[[]byte, []byte] {
	return func(yield func([]byte, []byte) bool) {}
}

type MultipleCFData struct{}

func (d *MultipleCFData) Close() {}

func (d *MultipleCFData) Iter() iter.Seq2[[]byte, CFData] {
	return func(yield func([]byte, CFData) bool) {}
}

type CFData struct {
	Data []byte
	CF   uint8
}
