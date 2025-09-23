// Copyright 2018 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

//go:build !js && !wasip1 && rocksdb
// +build !js,!wasip1,rocksdb

// Package rocksdb implements the key-value database layer based on RocksDB.
package rocksdb

import (
	"bytes"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/metrics"
	"github.com/linxGnu/grocksdb"
)

var (
	// errRocksDBClosed is returned if a rocksdb database was already closed at the
	// invocation of a data access operation.
	errRocksDBClosed = errors.New("database closed")
)

const (
	// degradationWarnInterval specifies how often warning should be printed if the
	// rocksdb database cannot keep up with requested writes.
	degradationWarnInterval = time.Minute

	// minCache is the minimum amount of memory in megabytes to allocate to rocksdb
	// read and write caching, split half and half.
	minCache = 16

	// minHandles is the minimum number of files handles to allocate to the open
	// database files.
	minHandles = 16

	// metricsGatheringInterval specifies the interval to retrieve rocksdb database
	// compaction, io and pause stats to report to the user.
	metricsGatheringInterval = 3 * time.Second
)

// Database is a persistent key-value store. Apart from basic data storage
// functionality it also supports batch writes and iterating over the keyspace in
// binary-alphabetical order.
type Database struct {
	fn string       // filename for reporting
	db *grocksdb.DB // RocksDB instance
	ro *grocksdb.ReadOptions
	wo *grocksdb.WriteOptions

	compTimeMeter       *metrics.Meter // Meter for measuring the total time spent in database compaction
	compReadMeter       *metrics.Meter // Meter for measuring the data read during compaction
	compWriteMeter      *metrics.Meter // Meter for measuring the data written during compaction
	writeDelayNMeter    *metrics.Meter // Meter for measuring the write delay number due to database compaction
	writeDelayMeter     *metrics.Meter // Meter for measuring the write delay duration due to database compaction
	diskSizeGauge       *metrics.Gauge // Gauge for tracking the size of all the levels in the database
	diskReadMeter       *metrics.Meter // Meter for measuring the effective amount of data read
	diskWriteMeter      *metrics.Meter // Meter for measuring the effective amount of data written
	memCompGauge        *metrics.Gauge // Gauge for tracking the number of memory compaction
	level0CompGauge     *metrics.Gauge // Gauge for tracking the number of table compaction in level0
	nonlevel0CompGauge  *metrics.Gauge // Gauge for tracking the number of table compaction in non0 level
	seekCompGauge       *metrics.Gauge // Gauge for tracking the number of table compaction caused by read opt
	manualMemAllocGauge *metrics.Gauge // Gauge to track the amount of memory that has been manually allocated (not a part of runtime/GC)

	levelsGauge []*metrics.Gauge // Gauge for tracking the number of tables in levels

	quitLock sync.Mutex      // Mutex protecting the quit channel access
	quitChan chan chan error // Quit channel to stop the metrics collection before closing the database

	closeLock sync.RWMutex // Mutex protecting the closed flag
	closed    bool         // Flag indicating if the database is closed

	log log.Logger // Contextual logger tracking the database path
}

// New returns a wrapped RocksDB object. The namespace is the prefix that the
// metrics reporting should use for surfacing internal stats.
func New(file string, cache int, handles int, namespace string, readonly bool) (*Database, error) {
	return NewCustom(file, namespace, func(opts *grocksdb.Options) {
		// Ensure we have some minimal caching and file guarantees
		if cache < minCache {
			cache = minCache
		}
		if handles < minHandles {
			handles = minHandles
		}

		// Set default options
		opts.SetMaxOpenFiles(handles)

		// Set cache sizes (split between block cache and write buffer)
		blockCache := grocksdb.NewLRUCache(uint64(cache / 2 * 1024 * 1024))
		bbto := grocksdb.NewDefaultBlockBasedTableOptions()
		bbto.SetBlockCache(blockCache)
		opts.SetBlockBasedTableFactory(bbto)

		// Write buffer size - RocksDB uses this for memtable
		opts.SetWriteBufferSize(uint64(cache / 4 * 1024 * 1024))
		opts.SetMaxWriteBufferNumber(3) // Similar to LevelDB's behavior

		// Performance tuning
		opts.SetMaxBackgroundCompactions(4)
		opts.SetMaxBackgroundFlushes(2)

		// Bloom filter for better read performance
		bbto.SetFilterPolicy(grocksdb.NewBloomFilter(10))

		if readonly {
			// For readonly mode, we don't need write buffers
			opts.SetWriteBufferSize(0)
		}
	})
}

// NewCustom returns a wrapped RocksDB object. The namespace is the prefix that the
// metrics reporting should use for surfacing internal stats.
// The customize function allows the caller to modify the rocksdb options.
func NewCustom(file string, namespace string, customize func(options *grocksdb.Options)) (*Database, error) {
	options := configureOptions(customize)
	logger := log.New("database", file)

	// Log configuration details
	usedCache := options.GetWriteBufferSize() * uint64(options.GetMaxWriteBufferNumber())
	logCtx := []interface{}{"cache", common.StorageSize(usedCache), "handles", options.GetMaxOpenFiles()}
	logger.Info("Allocated cache and file handles", logCtx...)

	// Open the db
	db, err := grocksdb.OpenDb(options, file)
	if err != nil {
		return nil, err
	}

	// Create read and write options
	ro := grocksdb.NewDefaultReadOptions()
	wo := grocksdb.NewDefaultWriteOptions()

	// Assemble the wrapper with all the registered metrics
	rdb := &Database{
		fn:       file,
		db:       db,
		ro:       ro,
		wo:       wo,
		log:      logger,
		quitChan: make(chan chan error),
	}

	rdb.compTimeMeter = metrics.NewRegisteredMeter(namespace+"compact/time", nil)
	rdb.compReadMeter = metrics.NewRegisteredMeter(namespace+"compact/input", nil)
	rdb.compWriteMeter = metrics.NewRegisteredMeter(namespace+"compact/output", nil)
	rdb.diskSizeGauge = metrics.NewRegisteredGauge(namespace+"disk/size", nil)
	rdb.diskReadMeter = metrics.NewRegisteredMeter(namespace+"disk/read", nil)
	rdb.diskWriteMeter = metrics.NewRegisteredMeter(namespace+"disk/write", nil)
	rdb.writeDelayMeter = metrics.NewRegisteredMeter(namespace+"compact/writedelay/duration", nil)
	rdb.writeDelayNMeter = metrics.NewRegisteredMeter(namespace+"compact/writedelay/counter", nil)
	rdb.memCompGauge = metrics.NewRegisteredGauge(namespace+"compact/memory", nil)
	rdb.level0CompGauge = metrics.NewRegisteredGauge(namespace+"compact/level0", nil)
	rdb.nonlevel0CompGauge = metrics.NewRegisteredGauge(namespace+"compact/nonlevel0", nil)
	rdb.seekCompGauge = metrics.NewRegisteredGauge(namespace+"compact/seek", nil)
	rdb.manualMemAllocGauge = metrics.NewRegisteredGauge(namespace+"memory/manualalloc", nil)

	// Start up the metrics gathering and return
	go rdb.meter(metricsGatheringInterval, namespace)

	return rdb, nil
}

// configureOptions sets some default options, then runs the provided setter.
func configureOptions(customizeFn func(*grocksdb.Options)) *grocksdb.Options {
	// Set default options
	options := grocksdb.NewDefaultOptions()

	// Create directories if needed
	options.SetCreateIfMissing(true)
	options.SetCreateIfMissingColumnFamilies(true)

	// Allow caller to make custom modifications to the options
	if customizeFn != nil {
		customizeFn(options)
	}
	return options
}

// Close stops the metrics collection, flushes any pending data to disk and closes
// all io accesses to the underlying key-value store.
func (db *Database) Close() error {
	db.closeLock.Lock()
	defer db.closeLock.Unlock()

	if db.closed {
		return nil
	}

	db.quitLock.Lock()
	if db.quitChan != nil {
		errc := make(chan error)
		db.quitChan <- errc
		if err := <-errc; err != nil {
			db.log.Error("Metrics collection failed", "err", err)
		}
		db.quitChan = nil
	}
	db.quitLock.Unlock()

	// Close options and database
	if db.ro != nil {
		db.ro.Destroy()
		db.ro = nil
	}
	if db.wo != nil {
		db.wo.Destroy()
		db.wo = nil
	}
	if db.db != nil {
		db.db.Close()
		db.db = nil
	}

	db.closed = true
	return nil
}

// Has retrieves if a key is present in the key-value store.
func (db *Database) Has(key []byte) (bool, error) {
	db.closeLock.RLock()
	defer db.closeLock.RUnlock()

	if db.closed {
		return false, errRocksDBClosed
	}

	data, err := db.db.Get(db.ro, key)
	if err != nil {
		return false, err
	}
	defer data.Free()
	return data.Data() != nil, nil
}

// Get retrieves the given key if it's present in the key-value store.
func (db *Database) Get(key []byte) ([]byte, error) {
	db.closeLock.RLock()
	defer db.closeLock.RUnlock()

	if db.closed {
		return nil, errRocksDBClosed
	}

	data, err := db.db.Get(db.ro, key)
	if err != nil {
		return nil, err
	}
	defer data.Free()

	if data.Data() == nil {
		return nil, fmt.Errorf("key not found")
	}

	// Copy the data since we're freeing the slice
	result := make([]byte, len(data.Data()))
	copy(result, data.Data())
	return result, nil
}

// Put inserts the given value into the key-value store.
func (db *Database) Put(key []byte, value []byte) error {
	db.closeLock.RLock()
	defer db.closeLock.RUnlock()

	if db.closed {
		return errRocksDBClosed
	}

	return db.db.Put(db.wo, key, value)
}

// Delete removes the key from the key-value store.
func (db *Database) Delete(key []byte) error {
	db.closeLock.RLock()
	defer db.closeLock.RUnlock()

	if db.closed {
		return errRocksDBClosed
	}

	return db.db.Delete(db.wo, key)
}

// DeleteRange deletes all of the keys (and values) in the range [start,end)
// (inclusive on start, exclusive on end).
// Note: This is a fallback implementation as grocksdb may not expose DeleteRange.
// It iterates through keys and deletes them individually.
func (db *Database) DeleteRange(start, end []byte) error {
	batch := db.NewBatch()
	it := db.NewIterator(nil, start)
	defer it.Release()

	var count int
	for it.Next() && bytes.Compare(end, it.Key()) > 0 {
		count++
		if count > 10000 { // should not block for more than a second
			if err := batch.Write(); err != nil {
				return err
			}
			return ethdb.ErrTooManyKeys
		}
		if err := batch.Delete(it.Key()); err != nil {
			return err
		}
	}
	return batch.Write()
}

// NewBatch creates a write-only key-value store that buffers changes to its host
// database until a final write is called.
func (db *Database) NewBatch() ethdb.Batch {
	return &batch{
		parent: db,
		db:     db.db,
		wo:     db.wo,
		b:      grocksdb.NewWriteBatch(),
	}
}

// NewBatchWithSize creates a write-only database batch with pre-allocated buffer.
func (db *Database) NewBatchWithSize(size int) ethdb.Batch {
	// RocksDB WriteBatch doesn't have size pre-allocation in grocksdb
	wb := grocksdb.NewWriteBatch()
	return &batch{
		parent: db,
		db:     db.db,
		wo:     db.wo,
		b:      wb,
	}
}

// NewIterator creates a binary-alphabetical iterator over a subset
// of database content with a particular key prefix, starting at a particular
// initial key (or after, if it does not exist).
func (db *Database) NewIterator(prefix []byte, start []byte) ethdb.Iterator {
	ro := grocksdb.NewDefaultReadOptions()

	// Set up iteration bounds
	startKey := append(prefix, start...)
	endKey := rocksdbPrefixRange(prefix)

	if endKey != nil {
		ro.SetIterateUpperBound(endKey)
	}

	iter := db.db.NewIterator(ro)
	iter.Seek(startKey)

	return &rocksdbIterator{
		iter:   iter,
		ro:     ro,
		prefix: prefix,
		first:  true,
	}
}

// Stat returns the statistic data of the database.
func (db *Database) Stat() (string, error) {
	stats := db.db.GetProperty("rocksdb.stats")
	if stats == "" {
		return "No stats available", nil
	}
	return stats, nil
}

// Compact flattens the underlying data store for the given key range. In essence,
// deleted and overwritten versions are discarded, and the data is rearranged to
// reduce the cost of operations needed to access them.
//
// A nil start is treated as a key before all keys in the data store; a nil limit
// is treated as a key after all keys in the data store. If both is nil then it
// will compact entire data store.
func (db *Database) Compact(start []byte, limit []byte) error {
	// RocksDB CompactRange
	db.db.CompactRange(grocksdb.Range{Start: start, Limit: limit})
	return nil
}

// Path returns the path to the database directory.
func (db *Database) Path() string {
	return db.fn
}

// meter periodically retrieves internal rocksdb counters and reports them to
// the metrics subsystem.
func (db *Database) meter(refresh time.Duration, namespace string) {
	var errc chan error
	timer := time.NewTimer(refresh)
	defer timer.Stop()

	// Iterate ad infinitum and collect the stats
	for errc == nil {
		select {
		case errc = <-db.quitChan:
			// Quit requesting, stop hammering the database
		case <-timer.C:
			timer.Reset(refresh)
			// Timeout, gather a new set of stats

			// Get various RocksDB statistics
			if stats := db.db.GetProperty("rocksdb.stats"); stats != "" {
				// Update basic metrics that we can get from RocksDB
				if totalSize := db.db.GetProperty("rocksdb.total-sst-files-size"); totalSize != "" {
					// Parse and update size metrics - simplified for now
					db.diskSizeGauge.Update(0)
				}

				if level0Files := db.db.GetProperty("rocksdb.num-files-at-level0"); level0Files != "" {
					// Parse and update level metrics - simplified for now
					db.level0CompGauge.Update(0)
				}

				if memTableSize := db.db.GetProperty("rocksdb.cur-size-all-mem-tables"); memTableSize != "" {
					// Parse and update memory metrics - simplified for now
					db.manualMemAllocGauge.Update(0)
				}
			}
		}
	}

	errc <- nil
}

// batch is a write-only rocksdb batch that commits changes to its host database
// when Write is called. A batch cannot be used concurrently.
type batch struct {
	parent *Database // Reference to parent database for closed checks
	db     *grocksdb.DB
	wo     *grocksdb.WriteOptions
	b      *grocksdb.WriteBatch
	size   int

	// Track operations for replay functionality
	ops []batchOp
}

// batchOp represents an operation in the batch
type batchOp struct {
	opType int // 0 = Put, 1 = Delete
	key    []byte
	value  []byte
}

// Put inserts the given value into the batch for later committing.
func (b *batch) Put(key, value []byte) error {
	b.b.Put(key, value)
	b.size += len(key) + len(value)

	// Store operation for replay
	keyCopy := make([]byte, len(key))
	copy(keyCopy, key)
	valueCopy := make([]byte, len(value))
	copy(valueCopy, value)
	b.ops = append(b.ops, batchOp{
		opType: 0, // Put operation
		key:    keyCopy,
		value:  valueCopy,
	})

	return nil
}

// Delete inserts the key removal into the batch for later committing.
func (b *batch) Delete(key []byte) error {
	b.b.Delete(key)
	b.size += len(key)

	// Store operation for replay
	keyCopy := make([]byte, len(key))
	copy(keyCopy, key)
	b.ops = append(b.ops, batchOp{
		opType: 1, // Delete operation
		key:    keyCopy,
		value:  nil,
	})

	return nil
}

// ValueSize retrieves the amount of data queued up for writing.
func (b *batch) ValueSize() int {
	return b.size
}

// Write flushes any accumulated data to disk.
func (b *batch) Write() error {
	// Check if the parent database is closed
	b.parent.closeLock.RLock()
	defer b.parent.closeLock.RUnlock()

	if b.parent.closed {
		return errRocksDBClosed
	}

	return b.db.Write(b.wo, b.b)
}

// Reset resets the batch for reuse.
func (b *batch) Reset() {
	b.b.Clear()
	b.size = 0
	b.ops = b.ops[:0] // Clear operations slice but keep capacity
}

// Replay replays the batch contents.
func (b *batch) Replay(w ethdb.KeyValueWriter) error {
	for _, op := range b.ops {
		if op.opType == 0 { // Put operation
			if err := w.Put(op.key, op.value); err != nil {
				return err
			}
		} else if op.opType == 1 { // Delete operation
			if err := w.Delete(op.key); err != nil {
				return err
			}
		}
	}
	return nil
}

// rocksdbIterator wraps the RocksDB iterator to implement the ethdb.Iterator interface
type rocksdbIterator struct {
	iter   *grocksdb.Iterator
	ro     *grocksdb.ReadOptions
	prefix []byte
	first  bool
}

// Next moves the iterator to the next key/value pair. It returns whether the
// iterator is exhausted.
func (it *rocksdbIterator) Next() bool {
	if it.first {
		it.first = false
		return it.Valid()
	}
	if !it.Valid() {
		return false
	}
	it.iter.Next()
	return it.Valid()
}

// Error returns any accumulated error. Exhausting all the key/value pairs
// is not considered to be an error.
func (it *rocksdbIterator) Error() error {
	// if iter is nil, it means the iterator is exhausted and released => no error
	if it.iter == nil {
		return nil
	}
	return it.iter.Err()
}

// Key returns the key of the current key/value pair, or nil if done.
func (it *rocksdbIterator) Key() []byte {
	if !it.Valid() {
		return nil
	}
	key := it.iter.Key()
	defer key.Free()
	result := make([]byte, len(key.Data()))
	copy(result, key.Data())
	return result
}

// Value returns the value of the current key/value pair, or nil if done.
func (it *rocksdbIterator) Value() []byte {
	if !it.Valid() {
		return nil
	}
	value := it.iter.Value()
	defer value.Free()
	result := make([]byte, len(value.Data()))
	copy(result, value.Data())
	return result
}

// Valid returns whether the iterator is positioned at a valid key/value pair.
func (it *rocksdbIterator) Valid() bool {
	if !it.iter.Valid() {
		return false
	}

	// Check if we're still within the prefix
	key := it.iter.Key()
	defer key.Free()
	return bytes.HasPrefix(key.Data(), it.prefix)
}

// Release releases associated resources. Release should always succeed and can
// be called multiple times without causing error.
func (it *rocksdbIterator) Release() {
	if it.iter != nil {
		it.iter.Close()
		it.iter = nil
	}
	if it.ro != nil {
		it.ro.Destroy()
		it.ro = nil
	}
}

// rocksdbPrefixRange returns the upper bound for the given prefix
func rocksdbPrefixRange(prefix []byte) []byte {
	if prefix == nil {
		return nil
	}

	for i := len(prefix) - 1; i >= 0; i-- {
		c := prefix[i]
		if c == 0xff {
			continue
		}
		limit := make([]byte, i+1)
		copy(limit, prefix)
		limit[i] = c + 1
		return limit
	}
	return nil
}
