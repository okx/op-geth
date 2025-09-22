# Database Migration Tool

The op-geth database migration tool (`dbmigrate`) allows you to migrate blockchain databases between different storage backends (Pebble, RocksDB, LevelDB) while preserving all blockchain state data.

## Overview

This tool is specifically designed for migrating op-geth databases between storage engines. It handles:

- **Trie nodes**: All state trie data including accounts, storage, and receipts
- **Block data**: Headers, bodies, and transaction data
- **Metadata**: Chain configuration, sync status, and other operational data
- **Ancient data**: Optionally includes freezer/ancient data migration

## Supported Migrations

| Source | Target | Status |
|--------|--------|--------|
| Pebble | RocksDB | ✅ Fully Supported |
| Pebble | LevelDB | 🚧 Planned |
| RocksDB | Pebble | ✅ Fully Supported |
| RocksDB | LevelDB | 🚧 Planned |
| LevelDB | Pebble | 🚧 Planned |
| LevelDB | RocksDB | ✅ Fully Supported |

## Quick Start

### Basic Migration (Pebble to RocksDB)

```bash
# Stop your op-geth node first
dbmigrate \
  --datadir /path/to/datadir \
  --from pebble \
  --to rocksdb \
  --verify
```

### LevelDB to RocksDB Migration

```bash
# Stop your op-geth node first
dbmigrate \
  --datadir /path/to/datadir \
  --from leveldb \
  --to rocksdb \
  --verify
```

### Custom Target Directory

```bash
dbmigrate \
  --datadir /path/to/datadir \
  --from pebble \
  --to rocksdb \
  --target-dir /path/to/new/database \
  --verify
```

### Performance Optimized Migration

```bash
dbmigrate \
  --datadir /path/to/datadir \
  --from pebble \
  --to rocksdb \
  --batch-size 10485760 \  # 10MB batches
  --batch-count 1000 \      # 1000 entries per batch
  --verify
```

## Command Reference

```
dbmigrate [OPTIONS]

DESCRIPTION:
   Migrate database from one backend to another

FLAGS:
   --datadir value          Data directory for the databases (default: ~/.ethereum)
   --from value            Source database type (pebble, rocksdb, leveldb) (default: "pebble")
   --to value              Target database type (pebble, rocksdb, leveldb) (default: "rocksdb")
   --target-dir value      Target directory for migrated database (defaults to datadir/chaindata_migrated)
   --force                 Force migration even if target directory exists (default: false)
   --verify                Verify migration by comparing source and target databases (default: false)
   --batch-size value      Migration batch size in bytes (default: 10485760)
   --batch-count value     Number of entries per batch (default: 1000)
   --include-ancient       Include ancient/freezer data in migration (default: false)
   --help, -h              Show help
```

## Performance Tuning

### Batch Size Optimization

The migration tool uses batching for optimal performance. Adjust these parameters based on your system:

- **`--batch-size`**: Controls memory usage per batch (default: 10MB)
- **`--batch-count`**: Controls number of entries per batch (default: 1000)

**Memory-constrained systems:**
```bash
--batch-size 1048576    # 1MB batches
--batch-count 100       # 100 entries per batch
```

**High-memory systems:**
```bash
--batch-size 67108864   # 64MB batches
--batch-count 5000      # 5000 entries per batch
```

### Expected Performance

Performance varies based on hardware and database size:

| System Type | Typical Speed | Notes |
|-------------|---------------|-------|
| SSD + 16GB RAM | 1000-5000 entries/sec | Recommended minimum |
| NVMe + 32GB RAM | 5000-15000 entries/sec | Optimal performance |
| HDD | 100-500 entries/sec | Not recommended for large databases |

## Migration Process

The migration follows these steps:

1. **Validation**: Checks source/target types and directories
2. **Size Estimation**: Scans source database to estimate total work
3. **Database Opening**: Opens source (read-only) and target (read-write) databases
4. **Batch Migration**: Iterates through all key-value pairs in batches
5. **Verification** (optional): Compares source and target databases
6. **Completion**: Reports statistics and closes databases

### Progress Reporting

The tool provides detailed progress information:

```
INFO Migration progress    progress=45.2% (1,234,567/2,731,891 keys, 1.2GB/2.8GB bytes) |
                          speed=2,150 keys/s, 5.2MB/s |
                          batches=123 | errors=0 | duration=9m32s
```

## Verification

Use `--verify` to ensure migration integrity:

```bash
dbmigrate --verify [other options]
```

Verification process:
- Compares every key-value pair between source and target
- Reports any missing or mismatched data
- Adds ~30-50% to total migration time
- **Highly recommended** for production migrations

## Error Handling

The migration tool is designed to be robust:

### Automatic Recovery
- Continues on individual key failures
- Reports all errors in logs
- Provides final error count in statistics

### Common Issues

**"database closed" errors:**
- Ensure source database is not being used by another process
- Stop op-geth node before migration

**Memory errors:**
- Reduce `--batch-size` and `--batch-count`
- Ensure sufficient system memory

**Permission errors:**
- Verify read permissions on source directory
- Verify write permissions on target directory

## Production Usage

### Pre-Migration Checklist

1. **Stop op-geth node** completely
2. **Backup your database** (copy entire datadir)
3. **Verify disk space** (target needs ~100% of source size)
4. **Check system resources** (RAM, CPU availability)
5. **Test on smaller dataset** if possible

### Migration Steps

```bash
# 1. Stop op-geth
sudo systemctl stop op-geth

# 2. Backup (recommended)
cp -r /var/lib/op-geth/geth/chaindata /backup/chaindata-backup

# 3. Run migration with verification
dbmigrate \
  --datadir /var/lib/op-geth \
  --from pebble \
  --to rocksdb \
  --verify \
  --batch-size 10485760

# 4. Update op-geth configuration
# Edit config to use --db.engine=rocksdb

# 5. Start op-geth with new database
sudo systemctl start op-geth
```

### Post-Migration

1. **Verify node startup** with new database
2. **Check sync status** (should continue from where it left off)
3. **Monitor performance** for first few hours
4. **Remove old database** after confirming stability

## Troubleshooting

### Migration Fails to Start

**Error: "unsupported database type"**
- Check `--from` and `--to` flags
- Ensure database type is supported

**Error: "target directory already exists"**
- Use `--force` to overwrite
- Or choose different `--target-dir`

### Migration Performance Issues

**Very slow migration:**
- Increase `--batch-size` if you have more RAM
- Check disk I/O (use `iostat -x 1`)
- Ensure source database is on fast storage

**High memory usage:**
- Decrease `--batch-size` and `--batch-count`
- Monitor with `top` or `htop`

### Verification Failures

**Key missing errors:**
- Check source database integrity
- Retry migration with smaller batches

**Value mismatch errors:**
- May indicate hardware issues (RAM/storage)
- Run filesystem check (`fsck`)

## Database Formats

### Pebble Database
- **Location**: `datadir/geth/chaindata/`
- **Files**: `*.sst`, `MANIFEST-*`, `CURRENT`, etc.
- **Characteristics**: High performance, LSM-tree based

### RocksDB Database
- **Location**: `datadir/geth/chaindata/`
- **Files**: `*.sst`, `MANIFEST-*`, `CURRENT`, etc.
- **Characteristics**: Mature, highly tunable, LSM-tree based

### Migration Safety

Both Pebble and RocksDB use similar LSM-tree architectures, making migration safe:
- Key-value data is preserved exactly
- No data transformation or conversion
- Byte-for-byte identical content (verified with `--verify`)

## Advanced Usage

### Scripted Migration

```bash
#!/bin/bash
set -e

DATADIR="/var/lib/op-geth"
BACKUP_DIR="/backup/migration-$(date +%Y%m%d-%H%M%S)"
TARGET_DIR="$DATADIR/chaindata_rocksdb"

echo "Starting op-geth database migration..."

# Create backup
echo "Creating backup..."
mkdir -p "$BACKUP_DIR"
cp -r "$DATADIR/geth/chaindata" "$BACKUP_DIR/"

# Stop service
echo "Stopping op-geth..."
sudo systemctl stop op-geth

# Run migration
echo "Running migration..."
dbmigrate \
  --datadir "$DATADIR" \
  --from pebble \
  --to rocksdb \
  --target-dir "$TARGET_DIR" \
  --verify \
  --batch-size 10485760

# Replace old database
echo "Replacing database..."
mv "$DATADIR/geth/chaindata" "$DATADIR/geth/chaindata_pebble_old"
mv "$TARGET_DIR" "$DATADIR/geth/chaindata"

# Update configuration (implement based on your setup)
# sed -i 's/db\.engine=pebble/db.engine=rocksdb/' /etc/op-geth/config.toml

echo "Migration complete. Starting op-geth..."
sudo systemctl start op-geth

echo "Monitoring startup..."
sleep 30
sudo systemctl status op-geth
```

### Monitoring Migration

```bash
# Monitor in separate terminal
watch -n 5 'tail -n 20 migration.log | grep "Migration progress"'

# Check system resources
htop

# Monitor disk I/O
iostat -x 1
```

## FAQ

**Q: How long does migration take?**
A: Depends on database size and hardware. Typical rates:
- Small database (< 10GB): 10-30 minutes
- Medium database (10-100GB): 1-5 hours
- Large database (> 100GB): 5-24+ hours

**Q: Can I run migration while op-geth is running?**
A: No. Always stop op-geth before migration to avoid corruption.

**Q: What happens if migration is interrupted?**
A: Migration can be restarted safely. Use `--force` to overwrite partial target database.

**Q: Does migration affect blockchain sync?**
A: No. After migration, op-geth continues from the same block height.

**Q: Can I migrate back to the original database?**
A: Yes. Keep the original database until you're confident the migration worked.

**Q: What about ancient/freezer data?**
A: Currently not migrated by default. Use `--include-ancient` flag (experimental).

**Q: Are there any data format differences?**
A: No. The migration preserves exact key-value data. Only the storage engine changes.

## Support

For issues or questions:

1. Check this documentation
2. Review op-geth logs for specific errors
3. Search existing GitHub issues
4. Create new issue with:
   - Migration command used
   - Error messages
   - System specifications
   - Database size
