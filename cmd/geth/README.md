## init
```azure
~/go/bin/geth --datadir=/tmp/op_geth --gcmode=archive init --state.scheme=hash /Users/yangweitao/dev/okx/op-geth/random_genesis.json
geth --datadir=/mnt/ramdisk_op/op_geth_data --gcmode=archive init --ignore-addresses=0x000000000000000000000000000000005ca1ab1e /mnt/ramdisk_op/genesis.json 
geth --datadir=/mnt/ramdisk_op/op_geth_data verify-genesis --ignore-addresses=0x000000000000000000000000000000005ca1ab1e /mnt/ramdisk_op/genesis.json
    
~/go/bin/geth --datadir=/Volumes/RAMDisk/geth --gcmode=archive verify-genesis --state.scheme=hash /Users/yangweitao/dev/okx/op-geth/random_genesis.json --db-cache=2048 --db-handles=1000
```
~/go/bin/geth --datadir=/Volumes/RAMDisk/geth --gcmode=archive init --state.scheme=hash --no-verify /Users/yangweitao/dev/okx/op-geth/random_genesis.json --db-cache=2048 --db-handles=1000


~/go/bin/geth --datadir=/Volumes/RAMDisk/op_geth_data --gcmode=archive init --state.scheme=hash --db-cache=2048 --db-handles=1000 --no-verify /Users/yangweitao/dev/okx/op-geth/random_genesis.json

./build/bin/geth --datadir=/tmp/op_geth_data --gcmode=archive migrate --state.scheme=hash -db-handles=1000 /mnt/ramdisk_op/genesis.json

diskutil erasevolume HFS+ "RAMDisk" $(hdiutil attach -nomount ram://67108864)


export PEBBLE_CACHE=2048
export PEBBLE_MEMTABLE_SIZE=128
export PEBBLE_COMPRESSION=none
geth init genesis.json