#!/bin/bash
set -x
set -e
source .env
source tools.sh
source utils.sh

cd $PWD_DIR

migrate() {
# init op-geth-seq and op-geth-rpc
export OP_DATA_DIR=./data/op-geth-seq \
export OP_GENESIS_PATH=/data1/op-geth/test-pp-op/config-op/genesis-op-after-number.json \
export ERIGON_CHAINDATA_DIR=/data1/op-geth/test-pp-op/data/rpc/chaindata/ \
export ERIGON_SMTDATA_DIR=/data1/op-geth/test-pp-op/data/rpc/smt/ \
export GETH_CMD=/data1/op-geth/build/bin/geth
${GETH_CMD} --datadir=${OP_DATA_DIR} --gcmode=archive migrate --state.scheme=hash --ignore-addresses=0x000000000000000000000000000000005ca1ab1e --chaindata=${ERIGON_CHAINDATA_DIR} --smt-db-path=${ERIGON_SMTDATA_DIR} ${OP_GENESIS_PATH} 2>&1 | tee migrate.log

sleep 10
NEW_BLOCK_HASH=$(grep 'Successfully wrote genesis state' migrate.log | tail -1 | sed -n 's/.*hash=\(0x[0-9a-fA-F]\{64\}\).*/\1/p')
echo "NEW_BLOCK_HASH"

ROLLUP_CONTENT=$(jq ".genesis.l2.hash = \"$NEW_BLOCK_HASH\"" config-op/rollup.json)
echo $ROLLUP_CONTENT | jq > config-op/rollup.json


## TODO: update block number and hash into rollup.json
echo "finished migrate op-geth"
}

migrate