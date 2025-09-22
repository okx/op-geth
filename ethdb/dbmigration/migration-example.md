# Database Migration Example

This example demonstrates how to migrate an op-geth database from Pebble to RocksDB.

## Prerequisites

1. **Stop op-geth**: Ensure your op-geth node is completely stopped before migration
2. **Backup data**: Always backup your database before migration
3. **Check disk space**: Ensure you have sufficient disk space (>100% of source DB size)
4. **Build geth**: Make sure you have compiled geth with the migration tool

## Basic Migration Example

```bash
#!/bin/bash
# migrate-pebble-to-rocksdb.sh

set -e  # Exit on any error

DATADIR="/var/lib/op-geth"  # Adjust to your data directory
BACKUP_DIR="/backup/db-migration-$(date +%Y%m%d-%H%M%S)"

echo "=== Op-geth Database Migration: Pebble → RocksDB ==="

# Step 1: Create backup
echo "Creating backup..."
mkdir -p "$BACKUP_DIR"
cp -r "$DATADIR/geth/chaindata" "$BACKUP_DIR/chaindata-original"
echo "Backup created at: $BACKUP_DIR"

# Step 2: Stop op-geth (if running as systemd service)
echo "Stopping op-geth..."
sudo systemctl stop op-geth || echo "Service not running or not managed by systemd"

# Step 3: Run migration with verification
echo "Starting migration..."
./db-migrate \
  --datadir "$DATADIR" \
  --from pebble \
  --to rocksdb \
  --verify \
  --batch-size 10485760 \
  --batch-count 1000

if [ $? -eq 0 ]; then
    echo "✅ Migration completed successfully!"
else
    echo "❌ Migration failed! Check logs above."
    exit 1
fi

# Step 4: Update configuration (example - adjust for your setup)
echo "Updating op-geth configuration..."
# sed -i 's/--db\.engine=pebble/--db.engine=rocksdb/' /etc/systemd/system/op-geth.service
# systemctl daemon-reload

echo "Migration complete! You can now start op-geth with RocksDB backend."
echo "Backup is available at: $BACKUP_DIR"
```

## Performance-Optimized Migration

For large databases, use optimized settings:

```bash
./db-migrate \
  --datadir /var/lib/op-geth \
  --from pebble \
  --to rocksdb \
  --target-dir /nvme/rocksdb-chaindata \
  --batch-size 67108864 \    # 64MB batches
  --batch-count 5000 \       # 5000 entries per batch
  --verify
```

## Testing Migration (Small Dataset)

Create a test environment to verify the migration works:

```bash
# Create test directories
mkdir -p /tmp/migration-test/{source,target}

# Initialize a small test database (you would populate this with real data)
./geth init --datadir /tmp/migration-test/source genesis.json

# Run migration
./geth db-migrate \
  --datadir /tmp/migration-test/source \
  --from pebble \
  --to rocksdb \
  --target-dir /tmp/migration-test/target \
  --verify \
  --force

# Cleanup
rm -rf /tmp/migration-test
```

## Expected Output

```
INFO [01-15|10:30:15.123] Starting database migration             sourceType=pebble targetType=rocksdb sourceDir=/var/lib/op-geth/geth/chaindata targetDir=/var/lib/op-geth/chaindata_migrated batchSize=10MB batchCount=1000 verify=true
INFO [01-15|10:30:15.234] Estimating database size for progress tracking...
INFO [01-15|10:30:16.456] Database size estimation complete       totalKeys=2,731,891 totalBytes=2.8GB
INFO [01-15|10:30:16.789] Starting database migration
INFO [01-15|10:30:21.123] Migration progress                       progress=15.2% (415,367/2,731,891 keys, 435MB/2.8GB bytes) speed=2,150 keys/s throughput=5.2MB/s batches=123 errors=0 duration=4s
INFO [01-15|10:30:26.456] Migration progress                       progress=32.4% (885,432/2,731,891 keys, 921MB/2.8GB bytes) speed=2,200 keys/s throughput=5.4MB/s batches=267 errors=0 duration=9s
...
INFO [01-15|10:39:45.234] Migration completed successfully        stats="Progress: 100.00% (2,731,891/2,731,891 keys, 2.8GB/2.8GB bytes) | Speed: 2,180 keys/s, 5.3MB/s | Batches: 891 | Errors: 0 | Duration: 9m28s"
INFO [01-15|10:39:45.345] Starting migration verification
INFO [01-15|10:39:48.123] Verification progress                    progress=25.5% verified=697,632 errors=0 duration=2s
...
INFO [01-15|10:42:12.567] Migration verification completed successfully verifiedKeys=2,731,891 duration=2m27s
INFO [01-15|10:42:12.678] Database migration completed successfully totalKeys=2,731,891 totalBytes=2.8GB duration=11m55s avgSpeed=3,815 keys/s avgThroughput=4.0MB/s batchesWritten=891 errorCount=0
```

## Troubleshooting

### Common Issues

**Migration fails with "database closed":**
```bash
# Ensure geth is stopped
pkill -f geth
sleep 5
# Retry migration
```

**Out of memory errors:**
```bash
# Reduce batch size
./geth db-migrate --batch-size 1048576 --batch-count 100 ...
```

**Target directory exists:**
```bash
# Use --force to overwrite
./geth db-migrate --force ...
# Or choose different target
./geth db-migrate --target-dir /path/to/new/location ...
```

**Verification fails:**
```bash
# Check disk space and system health
df -h
# Check for hardware issues
dmesg | grep -i error
# Retry migration with smaller batches
./geth db-migrate --batch-size 1048576 --batch-count 50 ...
```

### Performance Tips

1. **Use NVMe storage** for both source and target
2. **Increase batch size** on high-memory systems
3. **Monitor system resources** during migration:
   ```bash
   # In another terminal
   watch -n 5 'htop'
   iostat -x 1
   ```
4. **Disable verification** for faster migration (not recommended for production)

### Rollback

If migration fails or you need to rollback:

```bash
# Restore from backup
rm -rf /var/lib/op-geth/geth/chaindata
cp -r /backup/chaindata-original /var/lib/op-geth/geth/chaindata

# Start op-geth with original configuration
sudo systemctl start op-geth
```

## Migration Checklist

- [ ] Stop op-geth completely
- [ ] Create full backup of datadir
- [ ] Check available disk space (>150% of source)
- [ ] Test migration on copy/smaller dataset first
- [ ] Run migration with verification enabled
- [ ] Update op-geth configuration for new DB engine
- [ ] Test op-geth startup with migrated database
- [ ] Monitor performance for first few hours
- [ ] Keep backup until confident migration successful
