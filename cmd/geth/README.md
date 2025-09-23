# migration
## run
```
export OP_DATA_DIR=/data/xlayer_uploads/op_geth_data
export OP_GENESIS_PATH=/data/xlayer_uploads/genesis-op-raw.json
export ERIGON_CHAINDATA_DIR=/data/xlayer_uploads/erigon-data/chaindata
./build/bin/geth  --datadir=${OP_DATA_DIR} --gcmode=archive migrate --state.scheme=hash --ignore-addresses=0x000000000000000000000000000000005ca1ab1e --chaindata=${ERIGON_CHAINDATA_DIR} ${OP_GENESIS_PATH}
```

## unit test
```
go test ./core -cover -run TestMigration -coverprofile=coverage.out
go tool cover -html=coverage.out -o coverage.html
open coverage.html
```

