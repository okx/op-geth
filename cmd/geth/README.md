# migration


## preparation
1. build migration tool
```
docker run --rm --privileged \
    -v "$(pwd)/x:/x" \
    -w /app \
    "op-geth" \
    bash -c "
    
    "
```
2. stop rpc
3. prune https://okg-block.sg.larksuite.com/wiki/RnMIw1So6ivru4k33NQlTm61gfh
4. mount rpc
```
mkdir -p /mnt/ramdisk_op
mount -t tmpfs -o size=128g tmpfs /mnt/ramdisk_op
df -hT /mnt/ramdisk_op
```
5. cp data to mounted disk
4. restart rpc, on the mounted data
5. wait rpc sync to the latest head
6. prepare jwt for op-geth
```
openssl rand -hex 32 > /tmp/jwt.txt
```
7. upload op genesis.json


## migration
1. pause seq (pause api)
2. stop rpc
3. run migration
```
export OP_DATA_DIR=/mnt/ramdisk_op/op_geth_data \
export OP_GENESIS_PATH=/mnt/genesis-op-raw.json \
export ERIGON_CHAINDATA_DIR=/data/xlayer_uploads/erigon-data-split/chaindata/ \
export ERIGON_SMTDATA_DIR=/data/xlayer_uploads/erigon-data-split/smt/
nohup ./build/bin/geth --datadir=${OP_DATA_DIR} --gcmode=archive migrate --state.scheme=hash --ignore-addresses=0x000000000000000000000000000000005ca1ab1e --chaindata=${ERIGON_CHAINDATA_DIR} --smt-db-path=${ERIGON_SMTDATA_DIR} ${OP_GENESIS_PATH} > migrate.log 2>&1 &
```
4. cp data to disk



## start op
1. start op-geth
```
geth \
--datadir ./.ogeth \
--http \
--http.corsdomain="*" \
--http.vhosts="*" \
--http.port=7547 \
--http.addr=0.0.0.0 \
--http.api=web3,debug,eth,txpool,net,engine \
--ws \
--ws.addr=0.0.0.0 \
--ws.port=7546 \
--ws.origins="*" \
--ws.api=debug,eth,txpool,net,engine \
--syncmode=full \
--gcmode=archive \
--nodiscover \
--maxpeers=0 \
--networkid=901 \
--authrpc.vhosts="*" \
--authrpc.addr=0.0.0.0 \
--authrpc.port=8552 \
--authrpc.jwtsecret=/tmp/jwt.txt \
--rollup.disabletxpoolgossip=true
```






## drop cache (when necessary)
```
# flush dirty pages to disk
echo 3 | sudo tee /proc/sys/vm/drop_caches
```

## run


## unit test
```
go test ./core -cover -run TestMigration -coverprofile=coverage.out
go tool cover -html=coverage.out -o coverage.html
open coverage.html
```


