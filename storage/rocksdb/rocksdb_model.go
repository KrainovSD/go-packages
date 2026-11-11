package rocksdb

import (
	"errors"
	"runtime"
	"unsafe"
)

type ComparatorType uint8

const (
	COMPARATOR_U64_LE ComparatorType = iota + 1
)

type Compression int

const (
	COMPRESSION_NO Compression = iota
	COMPRESSION_SNAPPY
	COMPRESSION_ZLIB
	COMPRESSION_BZ2
	COMPRESSION_LZ4
	COMPRESSION_LZ4HC
	COMPRESSION_XPRESS
	COMPRESSION_ZSTD
)

type CompactionType int

const (
	COMPACTION_LEVEL CompactionType = iota
	COMPACTION_UNIVERSAL
	COMPACTION_FIFO
)

type RocksDBOptions struct {
	ColumnFamilies []ColumnFamilyOptions
	Writers        []WriterOptions
	Readers        []ReaderOptions

	CompactionType                  *CompactionType
	MemtableMemoryBudget            *uint64
	MaxBackgroundJobs               *int
	MaxTotalWalSize                 *uint64
	WriteBufferSize                 *uint64
	AvoidUnnecessaryBlockingIO      *bool
	KeepLogFileNum                  *uint64
	MaxLogFileSize                  *uint64
	StatsDumpPeriodSec              *uint32
	DeleteObsoleteFilesPeriodMicros *uint64
	Compression                     *Compression
	CompressionLevels               []Compression
	MaxOpenFiles                    *int
	BlockCacheSize                  *uint64
}

type ColumnFamilyOptions struct {
	Name           string
	PrefixLen      uint64
	PrefixOffset   uint64
	ComparatorType ComparatorType
	MergeOperator  unsafe.Pointer

	CompactionType                   *CompactionType
	MemtableMemoryBudget             *uint64
	MaxBackgroundJobs                *int
	WriteBufferSize                  *uint64
	MinWriteBufferNumberToMerge      *int
	MaxWriteBufferNumber             *int
	TargetFileSizeBase               *uint64
	TargetFileSizeMultiplier         *int
	MaxBytesForLevelBase             *uint64
	MaxBytesForLevelMultiplier       *float64
	LevelCompactionDynamicLevelBytes *bool
	Level0FileNumCompactionTrigger   *int
	Level0SlowdownWritesTrigger      *int
	Level0StopWritesTrigger          *int
	Compression                      *Compression
	CompressionLevels                []Compression
	MaxOpenFiles                     *int
	TTL                              *uint64

	Table *TableOptions
}

type TableOptions struct {
	BloomBitsPerKey        *float64
	BloomWholeKeyFiltering *bool
	BlockCacheSize         *uint64
}

type WriterOptions struct {
	Sync       *bool
	DisableWal *bool
	NoSlowdown *bool
	LowPri     *bool
}

type ReaderOptions struct {
	FillCache         *bool
	VerifyChecksums   *bool
	TotalOrderSeek    *bool
	PrefixSameAsStart *bool
}

func (opts RocksDBOptions) WithDefaults() *RocksDBOptions {
	if opts.CompactionType == nil && opts.Compression == nil && len(opts.CompressionLevels) == 0 {
		opts.CompressionLevels = []Compression{COMPRESSION_NO, COMPRESSION_LZ4, COMPRESSION_ZSTD}
	}
	opts.MaxBackgroundJobs = valueOrDefault(opts.MaxBackgroundJobs, runtime.NumCPU())
	opts.MaxTotalWalSize = valueOrDefault(opts.MaxTotalWalSize, uint64(64<<20))
	opts.AvoidUnnecessaryBlockingIO = valueOrDefault(opts.AvoidUnnecessaryBlockingIO, true)
	opts.KeepLogFileNum = valueOrDefault(opts.KeepLogFileNum, uint64(3))
	opts.MaxLogFileSize = valueOrDefault(opts.MaxLogFileSize, uint64(5<<20))
	opts.DeleteObsoleteFilesPeriodMicros = valueOrDefault(opts.DeleteObsoleteFilesPeriodMicros, uint64(6*60*60*1_000_000))
	opts.MaxOpenFiles = valueOrDefault(opts.MaxOpenFiles, 1024)
	if len(opts.ColumnFamilies) > 0 {
		opts.BlockCacheSize = valueOrDefault(opts.BlockCacheSize, uint64((len(opts.ColumnFamilies)*8)<<20))
	}
	return &opts
}

func (opts ColumnFamilyOptions) WithDefaults() ColumnFamilyOptions {
	if opts.CompactionType == nil && opts.Compression == nil && len(opts.CompressionLevels) == 0 {
		opts.CompressionLevels = []Compression{COMPRESSION_NO, COMPRESSION_LZ4, COMPRESSION_ZSTD}
	}
	opts.MaxBackgroundJobs = valueOrDefault(opts.MaxBackgroundJobs, runtime.NumCPU())
	opts.WriteBufferSize = valueOrDefault(opts.WriteBufferSize, uint64(4<<20))
	opts.MinWriteBufferNumberToMerge = valueOrDefault(opts.MinWriteBufferNumberToMerge, 2)
	opts.MaxWriteBufferNumber = valueOrDefault(opts.MaxWriteBufferNumber, 4)
	opts.TargetFileSizeBase = valueOrDefault(opts.TargetFileSizeBase, uint64(8<<20))
	opts.TargetFileSizeMultiplier = valueOrDefault(opts.TargetFileSizeMultiplier, 1)
	opts.MaxBytesForLevelBase = valueOrDefault(opts.MaxBytesForLevelBase, uint64(64<<20))
	opts.MaxBytesForLevelMultiplier = valueOrDefault(opts.MaxBytesForLevelMultiplier, 10.0)
	opts.LevelCompactionDynamicLevelBytes = valueOrDefault(opts.LevelCompactionDynamicLevelBytes, true)
	opts.Level0FileNumCompactionTrigger = valueOrDefault(opts.Level0FileNumCompactionTrigger, 4)
	opts.Level0SlowdownWritesTrigger = valueOrDefault(opts.Level0SlowdownWritesTrigger, 20)
	opts.Level0StopWritesTrigger = valueOrDefault(opts.Level0StopWritesTrigger, 36)
	opts.MaxOpenFiles = valueOrDefault(opts.MaxOpenFiles, -1)
	var table TableOptions
	if opts.Table != nil {
		table = *opts.Table
	}
	table.BloomBitsPerKey = valueOrDefault(table.BloomBitsPerKey, 10.0)
	table.BloomWholeKeyFiltering = valueOrDefault(table.BloomWholeKeyFiltering, true)
	opts.Table = &table
	return opts
}

var DefaultReader = ReaderOptions{
	FillCache:       new(true),
	VerifyChecksums: new(true),
}

var DefaultPrefixReader = ReaderOptions{
	FillCache:         new(true),
	VerifyChecksums:   new(true),
	PrefixSameAsStart: new(true),
}

var DefaultWriter = WriterOptions{
	Sync: new(true),
}

type Operation uint8

const (
	OPERATION_PUT Operation = iota + 1
	OPERATION_MERGE
	OPERATION_DELETE
)

func valueOrDefault[T any](v *T, def T) *T {
	if v != nil {
		return v
	}
	return new(def)
}

var ErrCgoRequired = errors.New("rocksdb: cgo is required")

type DatabaseOptions struct {
	Path              string
	ArenaIntitialSize uint64
	ArenaTTL          uint8
	Rocks             *RocksDBOptions
}

func (o *DatabaseOptions) validate() error {
	if o.Path == "" {
		return errors.New("path can't be empty")
	}
	return nil
}

type ReadOptions struct {
	Prefix     []byte
	PrefixLen  uint64
	CF         uint8
	Reader     uint8
	Limit      uint64
	Offset     uint64
	Cursor     []byte
	Reverse    bool
	ReturnKeys bool
}

type ReadBatchSingleOptions struct {
	Keys       [][]byte
	Cf         uint8
	Reader     uint8
	SortedKeys bool
}

type ReadBatchMultipleOptions struct {
	Keys   [][]byte
	CFs    []uint8
	Reader uint8
}

type WriteBatchOptions struct {
	Key       []byte
	Value     []byte
	CF        uint8
	Reader    *uint8
	Operation Operation
}

type Dump struct {
	CF     uint8
	Values [][]byte
	Keys   [][]byte
}
