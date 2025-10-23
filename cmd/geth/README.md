# migration

## mount
```
mkdir -p /mnt/ramdisk_op
mount -t tmpfs -o size=128g tmpfs /mnt/ramdisk_op
df -hT /mnt/ramdisk_op
```

## drop cache (when necessary)
```
# flush dirty pages to disk
echo 3 | sudo tee /proc/sys/vm/drop_caches
```

## run
```
export OP_DATA_DIR=/mnt/ramdisk_op/op_geth_data \
export OP_GENESIS_PATH=/mnt/genesis-op-raw.json \
export ERIGON_CHAINDATA_DIR=/data/xlayer_uploads/erigon-data-split/chaindata/ \
export ERIGON_SMTDATA_DIR=/data/xlayer_uploads/erigon-data-split/smt/
nohup ./build/bin/geth --datadir=${OP_DATA_DIR} --gcmode=archive migrate --state.scheme=hash --ignore-addresses=0x000000000000000000000000000000005ca1ab1e --chaindata=${ERIGON_CHAINDATA_DIR} --smt-db-path=${ERIGON_SMTDATA_DIR} ${OP_GENESIS_PATH} > migrate.log 2>&1 &
```

## verifyMigration
```
geth verifyMigrate --chaindata=${erigon_chaindata_path} --datadir=${op_data_dir} --standalone-smt=true
```

## unit test
```
go test ./core -cover -run TestMigration -coverprofile=coverage.out
go tool cover -html=coverage.out -o coverage.html
open coverage.html
```
