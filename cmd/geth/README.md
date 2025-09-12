## init
```azure
~/go/bin/geth --datadir=/tmp/op_geth --gcmode=archive init --state.scheme=hash /Users/yangweitao/dev/okx/op-geth/random_genesis.json
geth --datadir=/mnt/ramdisk_op/op_geth_data --gcmode=archive init --ignore-addresses=0x000000000000000000000000000000005ca1ab1e /mnt/ramdisk_op/genesis.json 
geth --datadir=/mnt/ramdisk_op/op_geth_data verify-genesis --ignore-addresses=0x000000000000000000000000000000005ca1ab1e /mnt/ramdisk_op/genesis.json
    
~/go/bin/geth --datadir=/Volumes/RAMDisk/geth --gcmode=archive verify-genesis --state.scheme=hash /Users/yangweitao/dev/okx/op-geth/random_genesis.json --db-cache=2048 --db-handles=1000
```
~/go/bin/geth --datadir=/Volumes/RAMDisk/geth --gcmode=archive init --state.scheme=hash --no-verify /Users/yangweitao/dev/okx/op-geth/random_genesis.json --db-cache=2048 --db-handles=1000


~/go/bin/geth --datadir=/Volumes/RAMDisk/op_geth_data --gcmode=archive init --state.scheme=hash --db-cache=2048 --db-handles=1000 --no-verify /Users/yangweitao/dev/okx/op-geth/random_genesis.json

./build/bin/geth --datadir=/tmp/op_geth_data --gcmode=archive migrate --state.scheme=hash --chaindata= -db-handles=1000 op-genesis.json


diskutil erasevolume HFS+ "RAMDisk" $(hdiutil attach -nomount ram://67108864)


./build/bin/geth --datadir=/tmp/op_geth_data --gcmode=archive migrate --state.scheme=hash --ignore-smt-verify --no-verify --ignore-addresses=0x000000000000000000000000000000005ca1ab1e --chaindata=/mnt/ramdisk_op/xlayer_chaindata/ op-genesis.json

./build/bin/geth --datadir=/tmp/op_geth_data --gcmode=archive migrate --state.scheme=hash --ignore-smt-verify --no-verify --ignore-addresses=0x000000000000000000000000000000005ca1ab1e --chaindata=/data/xlayer_uploads/rpc-bak0820/chaindata/ op-genesis.json
