## migrate
```
## unit test
go test ./core -cover -run TestMigration -coverprofile=coverage.out
go tool cover -html=coverage.out -o coverage.html
open coverage.html

## run
geth --datadir=/tmp/op_geth_data --gcmode=archive migrate --state.scheme=hash --ignore-smt-verify --no-verify --chaindata=/mnt/ramdisk_op/xlayer_chaindata/ op-genesis.json
```