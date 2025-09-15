## init
```azure
-- geth --datadir=op_geth --gcmode=archive init --state.scheme=hash /mnt/ramdisk_op/genesis.json
geth --datadir=/mnt/ramdisk_op/op_geth_data --gcmode=archive init --ignore-addresses=0x000000000000000000000000000000005ca1ab1e /mnt/ramdisk_op/genesis.json 
geth --datadir=/mnt/ramdisk_op/op_geth_data verify-genesis --ignore-addresses=0x000000000000000000000000000000005ca1ab1e /mnt/ramdisk_op/genesis.json
```