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

type StatisticsLevel int

const (
	STATISTICS_LEVEL_DISABLE_ALL             StatisticsLevel = 0
	STATISTICS_LEVEL_EXCEPT_HISTOGRAM_TIMERS StatisticsLevel = 1
	STATISTICS_LEVEL_EXCEPT_TIMERS           StatisticsLevel = 2
	STATISTICS_LEVEL_EXCEPT_DETAILED_TIMERS  StatisticsLevel = 3
	STATISTICS_LEVEL_EXCEPT_TIME_FOR_MUTEX   StatisticsLevel = 4
	STATISTICS_LEVEL_ALL                     StatisticsLevel = 5
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
	StatisticsLevel                 *StatisticsLevel
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
	opts.StatisticsLevel = valueOrDefault(opts.StatisticsLevel, STATISTICS_LEVEL_EXCEPT_DETAILED_TIMERS)
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

var ErrMetricsDisabled = errors.New("rocksdb: metrics is disabled")

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

type Stats struct {
	DB             DBStats
	Memory         MemoryUsage
	ColumnFamilies []CFStats
}

type MemoryUsage struct {
	MemTableTotal     uint64
	MemTableUnflushed uint64
	TableReaders      uint64
	CacheTotal        uint64
}

type DBStats struct {
	BackgroundErrors          uint64
	NumRunningCompactions     uint64
	NumRunningFlushes         uint64
	NumUnscheduledCompactions uint64
	IsWriteStopped            bool
	NumSnapshots              uint64
	OldestSnapshotTime        uint64
}

type CFStats struct {
	EstimateNumKeys                uint64
	NumEntriesActiveMemTable       uint64
	NumEntriesImmMemTables         uint64
	NumDeletesActiveMemTable       uint64
	NumDeletesImmMemTables         uint64
	CurSizeActiveMemTable          uint64
	CurSizeAllMemTables            uint64
	SizeAllMemTables               uint64
	NumImmutableMemTable           uint64
	MemTableFlushPending           bool
	CompactionPending              bool
	TotalSSTFilesSize              uint64
	LiveSSTFilesSize               uint64
	ObsoleteSSTFilesSize           uint64
	NumFilesAtLevel                [7]uint64
	EstimateTableReadersMem        uint64
	EstimatePendingCompactionBytes uint64
	EstimateLiveDataSize           uint64
	BaseLevel                      uint64
	BlockCacheUsage                uint64
	BlockCachePinnedUsage          uint64
	BlockCacheCapacity             uint64
	NumLiveVersions                uint64
}

type Metrics struct {
	Tickers    [TICKERS_COUNT]uint64
	Histograms [HISTOGRAMS_COUNT]HistogramData
}

type HistogramData struct {
	Count   uint64
	Sum     uint64
	Min     float64
	Max     float64
	Median  float64
	P95     float64
	P99     float64
	Average float64
	StdDev  float64
}

const (
	TICKERS_COUNT    = 267
	HISTOGRAMS_COUNT = 80
)

type TickerID uint32

type HistogramID uint32

const (
	TICKER_BLOCK_CACHE_MISS                  TickerID = 0
	TICKER_BLOCK_CACHE_HIT                   TickerID = 1
	TICKER_BLOCK_CACHE_ADD                   TickerID = 2
	TICKER_BLOCK_CACHE_INDEX_MISS            TickerID = 4
	TICKER_BLOCK_CACHE_INDEX_HIT             TickerID = 5
	TICKER_BLOCK_CACHE_FILTER_MISS           TickerID = 8
	TICKER_BLOCK_CACHE_FILTER_HIT            TickerID = 9
	TICKER_BLOCK_CACHE_DATA_MISS             TickerID = 12
	TICKER_BLOCK_CACHE_DATA_HIT              TickerID = 13
	TICKER_BLOCK_CACHE_BYTES_READ            TickerID = 16
	TICKER_BLOCK_CACHE_BYTES_WRITE           TickerID = 17
	TICKER_BLOOM_FILTER_USEFUL               TickerID = 35
	TICKER_BLOOM_FILTER_FULL_POSITIVE        TickerID = 36
	TICKER_BLOOM_FILTER_FULL_TRUE_POSITIVE   TickerID = 37
	TICKER_BLOOM_FILTER_PREFIX_CHECKED       TickerID = 38
	TICKER_BLOOM_FILTER_PREFIX_USEFUL        TickerID = 39
	TICKER_BLOOM_FILTER_PREFIX_TRUE_POSITIVE TickerID = 40
	TICKER_MEMTABLE_HIT                      TickerID = 45
	TICKER_MEMTABLE_MISS                     TickerID = 46
	TICKER_GET_HIT_L0                        TickerID = 47
	TICKER_GET_HIT_L1                        TickerID = 48
	TICKER_GET_HIT_L2_AND_UP                 TickerID = 49
	TICKER_NUMBER_KEYS_WRITTEN               TickerID = 58
	TICKER_NUMBER_KEYS_READ                  TickerID = 59
	TICKER_NUMBER_KEYS_UPDATED               TickerID = 60
	TICKER_BYTES_WRITTEN                     TickerID = 61
	TICKER_BYTES_READ                        TickerID = 62
	TICKER_NUMBER_DB_SEEK                    TickerID = 63
	TICKER_NUMBER_DB_NEXT                    TickerID = 64
	TICKER_NUMBER_DB_PREV                    TickerID = 65
	TICKER_ITER_BYTES_READ                   TickerID = 69
	TICKER_NUMBER_OF_RESEEKS_IN_ITERATION    TickerID = 71
	TICKER_STALL_MICROS                      TickerID = 76
	TICKER_NUMBER_MULTIGET_CALLS             TickerID = 78
	TICKER_NUMBER_MULTIGET_KEYS_READ         TickerID = 79
	TICKER_NUMBER_MULTIGET_BYTES_READ        TickerID = 80
	TICKER_NUMBER_MULTIGET_KEYS_FOUND        TickerID = 81
	TICKER_WAL_FILE_SYNCED                   TickerID = 84
	TICKER_WAL_FILE_BYTES                    TickerID = 85
	TICKER_WRITE_DONE_BY_SELF                TickerID = 91
	TICKER_WRITE_DONE_BY_OTHER               TickerID = 92
	TICKER_WRITE_WITH_WAL                    TickerID = 93
	TICKER_COMPACT_READ_BYTES                TickerID = 94
	TICKER_COMPACT_WRITE_BYTES               TickerID = 95
	TICKER_FLUSH_WRITE_BYTES                 TickerID = 96
	TICKER_NUMBER_BLOCK_COMPRESSED           TickerID = 107
	TICKER_NUMBER_BLOCK_DECOMPRESSED         TickerID = 108
	TICKER_BYTES_COMPRESSED_FROM             TickerID = 109
	TICKER_BYTES_COMPRESSED_TO               TickerID = 110
	TICKER_BYTES_COMPRESSION_BYPASSED        TickerID = 111
	TICKER_BYTES_COMPRESSION_REJECTED        TickerID = 112
	TICKER_ERROR_HANDLER_BG_ERROR_COUNT      TickerID = 177
	TICKER_MEMTABLE_PAYLOAD_BYTES_AT_FLUSH   TickerID = 183
	TICKER_MEMTABLE_GARBAGE_BYTES_AT_FLUSH   TickerID = 184
)

const (
	HISTOGRAM_DB_GET                       HistogramID = 0
	HISTOGRAM_DB_WRITE                     HistogramID = 1
	HISTOGRAM_COMPACTION_TIME              HistogramID = 2
	HISTOGRAM_WAL_FILE_SYNC_MICROS         HistogramID = 7
	HISTOGRAM_TABLE_OPEN_IO_MICROS         HistogramID = 9
	HISTOGRAM_DB_MULTIGET                  HistogramID = 10
	HISTOGRAM_READ_BLOCK_COMPACTION_MICROS HistogramID = 11
	HISTOGRAM_READ_BLOCK_GET_MICROS        HistogramID = 12
	HISTOGRAM_WRITE_STALL                  HistogramID = 16
	HISTOGRAM_SST_READ_MICROS              HistogramID = 17
	HISTOGRAM_BYTES_PER_READ               HistogramID = 31
	HISTOGRAM_BYTES_PER_WRITE              HistogramID = 32
	HISTOGRAM_BYTES_PER_MULTIGET           HistogramID = 33
	HISTOGRAM_FLUSH_TIME                   HistogramID = 50
)
