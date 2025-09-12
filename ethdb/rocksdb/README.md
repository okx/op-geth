# RocksDB Support for op-geth-okx

This package provides RocksDB support for the Ethereum client as an alternative to LevelDB and Pebble database backends.

## Prerequisites

To enable RocksDB support, you need to install the RocksDB C++ library on your system. We recommend building [RocksDB v10.4.2](https://github.com/facebook/rocksdb/releases/tag/v10.4.2) from sources by following the official [install guide](https://github.com/facebook/rocksdb/blob/master/INSTALL.md).

In summary, on Ubuntu:
```
git clone -b v10.4.2 https://github.com/facebook/rocksdb.git
cd rocksdb
sudo apt update
sudo apt install -y libsnappy-dev zlib1g-dev libbz2-dev liblz4-dev libzstd-dev libgflags-dev
make clean
make -j static_lib
sudo make install
```

## Building with RocksDB Support

Before running the commands below, run:
```
export CGO_CFLAGS="-I/usr/local/include"
export CGO_LDFLAGS="-L/usr/local/lib -lrocksdb -lstdc++ -lm -lz -lsnappy -llz4 -lzstd -lbz2 -luring"
```

### Method 1: Using Build Tags (Recommended)

Build the project with the `rocksdb` build tag:

```bash
# Build geth with RocksDB support
go build -tags rocksdb ./cmd/geth

# Build specific packages with RocksDB
go build -tags rocksdb ./ethdb/rocksdb
go test -tags rocksdb ./ethdb/rocksdb
```

### Method 2: Default Build (Stub Implementation)

If you build without the `rocksdb` tag, the stub implementation will be used, and attempts to use RocksDB will return an error:

```bash
# This will build successfully but RocksDB won't be functional
go build ./cmd/geth
```

## Usage

Once built with RocksDB support, you can use it by specifying the database engine:

### Command Line
```bash
# Use RocksDB as the database engine
./geth --db.engine=rocksdb

# Use RocksDB with custom cache and handles
./geth --db.engine=rocksdb --cache=1024 --db.handles=2048
```

### Configuration File
```toml
[Eth]
DatabaseEngine = "rocksdb"
DatabaseCache = 1024
DatabaseHandles = 2048
```

### Programmatic Usage
```go
import "github.com/ethereum/go-ethereum/ethdb/rocksdb"

// Basic usage
db, err := rocksdb.New("/path/to/db", 512, 1024, "metrics/", false)
if err != nil {
    // Handle error - may be ErrRocksDBNotSupported if not built with rocksdb tag
}

// Custom configuration
db, err := rocksdb.NewCustom("/path/to/db", "metrics/", func(opts *grocksdb.Options) {
    opts.SetMaxOpenFiles(2048)
    opts.SetWriteBufferSize(128 * 1024 * 1024) // 128MB
    opts.SetMaxWriteBufferNumber(4)
    opts.SetMaxBackgroundCompactions(8)
})
```

## Features

### ✅ Implemented Features
- **Complete KeyValueStore interface compliance**
- **Basic operations**: Get, Put, Delete, Has
- **Batch operations**: Atomic writes with batching
- **Iterators**: Prefix-based iteration with range support
- **Database compaction**: Manual compaction for performance
- **Statistics**: Integration with geth metrics system
- **Resource management**: Proper cleanup and resource handling
- **Build tag support**: Conditional compilation for optional dependency

### ⚠️ Limitations
- **Range deletion**: Implemented as fallback iteration (not native RocksDB DeleteRange)
- **Batch replay**: Limited support due to grocksdb API constraints
- **Compression**: Default compression only (specific algorithms not exposed)

### 🔧 Advanced Features
- **Configurable caching**: Block cache and write buffer tuning
- **Background operations**: Configurable compaction and flush threads
- **Bloom filters**: Optimized read performance
- **Memory management**: Tracked memory allocation metrics

## Performance Tuning

### Cache Configuration
```go
db, err := rocksdb.NewCustom(path, namespace, func(opts *grocksdb.Options) {
    // Set cache to 1GB total (512MB block cache + 256MB write buffer)
    blockCache := grocksdb.NewLRUCache(uint64(512 * 1024 * 1024))
    bbto := grocksdb.NewDefaultBlockBasedTableOptions()
    bbto.SetBlockCache(blockCache)
    opts.SetBlockBasedTableFactory(bbto)

    // Write buffer: 256MB
    opts.SetWriteBufferSize(uint64(256 * 1024 * 1024))
    opts.SetMaxWriteBufferNumber(3)
})
```

### Background Thread Configuration
```go
opts.SetMaxBackgroundCompactions(runtime.NumCPU()) // Use all CPU cores
opts.SetMaxBackgroundFlushes(2)                    // Dedicated flush threads
```

### File Handle Management
```go
opts.SetMaxOpenFiles(2048) // Adjust based on system limits
```

## Troubleshooting

### Build Issues

**Error: `could not determine what C.rocksdb_get_default_column_family_handle refers to`**
- **Solution**: Install RocksDB development libraries (see Prerequisites)
- **Alternative**: Build without `rocksdb` tag for stub implementation

**Error: `rocksdb support is not enabled`**
- **Solution**: Build with `-tags rocksdb` flag
- **Check**: Ensure RocksDB C++ library is properly installed

**Error: CGO compilation issues**
- **Solution**: Ensure CGO is enabled: `export CGO_ENABLED=1`
- **Check**: Verify gcc/g++ is installed and accessible

### Runtime Issues

**Database corruption errors**
- RocksDB has built-in corruption detection and recovery
- Use `geth removedb` to clear corrupted database (requires re-sync)

**Performance issues**
- Tune cache sizes based on available RAM
- Adjust background thread counts based on CPU cores
- Monitor disk I/O patterns and adjust accordingly

**High memory usage**
- Reduce block cache size in configuration
- Lower write buffer size and count
- Enable compression if CPU allows

## Migration from Other Databases

⚠️ **Important**: Different database engines are not compatible. You cannot directly migrate data between LevelDB, Pebble, and RocksDB.

### Migration Process
1. **Stop the node completely**
2. **Backup your data directory** (optional but recommended)
3. **Remove the old database directory**:
   ```bash
   rm -rf /path/to/datadir/geth/chaindata
   ```
4. **Start with RocksDB**:
   ```bash
   ./geth --db.engine=rocksdb
   ```
5. **Wait for full re-synchronization** (this will take time)

### Switching Between Databases
```bash
# Switch to RocksDB
./geth removedb && ./geth --db.engine=rocksdb

# Switch to Pebble
./geth removedb && ./geth --db.engine=pebble

# Switch to LevelDB
./geth removedb && ./geth --db.engine=leveldb
```

## Testing

Run the test suite with RocksDB support:

```bash
# Run all RocksDB tests
go test -tags rocksdb ./ethdb/rocksdb

# Run with verbose output
go test -tags rocksdb -v ./ethdb/rocksdb

# Run specific tests
go test -tags rocksdb -run TestRocksDBSuite ./ethdb/rocksdb
```

## Implementation Details

### Package Structure
```
ethdb/rocksdb/
├── rocksdb.go      # Main implementation (requires rocksdb tag)
├── rocksdb_test.go # Test suite (requires rocksdb tag)
├── rocksdb_stub.go # Stub implementation (used without rocksdb tag)
└── README.md       # This documentation
```

### Dependencies
- **Runtime**: `github.com/linxGnu/grocksdb` (Go bindings for RocksDB)
- **System**: RocksDB C++ library (librocksdb-dev)
- **Build**: CGO enabled, C/C++ compiler

### Build Tags
- **`rocksdb`**: Enables full RocksDB implementation
- **Default**: Uses stub implementation that returns errors

## Contributing

To improve RocksDB support:

1. **Performance optimizations**: Benchmark and tune default configurations
2. **Feature completeness**: Implement native DeleteRange and better batch replay
3. **Error handling**: Improve error messages and recovery mechanisms
4. **Documentation**: Add more usage examples and troubleshooting guides
5. **Testing**: Expand test coverage for edge cases and performance scenarios

### Development Setup
```bash
# Install RocksDB development libraries (see Prerequisites)
# Clone and setup project
git clone <repository>
cd op-geth-okx

# Build with RocksDB
go build -tags rocksdb ./ethdb/rocksdb

# Run tests
go test -tags rocksdb ./ethdb/rocksdb

# Build full project
go build -tags rocksdb ./...
```

## License

This RocksDB integration follows the same licensing as go-ethereum (GNU LGPL v3.0).
