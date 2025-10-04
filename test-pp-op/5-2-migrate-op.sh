#!/bin/bash
set -x
set -e
source .env
source tools.sh
source utils.sh

cd $PWD_DIR

migrate() {
  export OP_DATA_DIR=./data/op-geth-seq
  export OP_GENESIS_PATH=${PWD_DIR}/config-op/genesis-op-after-number.json

  if [ "$ENV" = "local" ]; then
    ERIGON_CHAINDATA_DIR=${PWD_DIR}/data/rpc/chaindata/
    ERIGON_SMTDATA_DIR=${PWD_DIR}/data/rpc/smt/
  else
     ERIGON_CHAINDATA_DIR=${PWD_DIR}/data/rpc/chaindata/
     ERIGON_SMTDATA_DIR=${PWD_DIR}/data/rpc/smt/
#    ERIGON_CHAINDATA_DIR=/data/erigon-data/chaindata/
#    ERIGON_SMTDATA_DIR=/data/erigon-data/smt/
  fi

  if [[ "$OSTYPE" == "darwin"* ]]; then
      export GETH_CMD=~/go/bin/geth

      if [ ! -f "$HOME/go/bin/geth" ]; then
          cd ../
          go install ./cmd/geth
          cd test-pp-op
      else
          echo "✅ geth found at ~/go/bin/geth"
      fi
  else
      export GETH_CMD=/data1/op-geth/build/bin/geth
      echo "✅ Using Linux geth path: $GETH_CMD"
  fi


  ${GETH_CMD} --datadir=${OP_DATA_DIR} --gcmode=archive migrate --state.scheme=hash --ignore-addresses=0x000000000000000000000000000000005ca1ab1e --chaindata=${ERIGON_CHAINDATA_DIR} --smt-db-path=${ERIGON_SMTDATA_DIR} --output merged.genesis.json ${OP_GENESIS_PATH} 2>&1 | tee migrate.log

  sleep 5
  #NEW_BLOCK_HASH=$(grep 'Successfully wrote genesis state' migrate.log | tail -1 | sed -n 's/.*hash=\(0x[0-9a-fA-F]\{64\}\).*/\1/p')
  #echo "NEW_BLOCK_HASH"
  #
  #ROLLUP_CONTENT=$(jq ".genesis.l2.hash = \"$NEW_BLOCK_HASH\"" config-op/rollup.json)
  #echo $ROLLUP_CONTENT | jq > config-op/rollup.json

  LOG_BLOCK=$(grep -A 5 "Update rollup.json file with the following information l2" migrate.log | tail -n 5)
  L2_NUMBER=$(echo "$LOG_BLOCK" | grep '"number"' | sed 's/[^0-9]*\([0-9]*\).*/\1/')
  L2_HASH=$(echo "$LOG_BLOCK" | grep '"hash"' | sed 's/.*"\(0x[0-9a-fA-F]*\)".*/\1/')
  echo "L2_NUMBER: $L2_NUMBER"
  echo "L2_HASH: $L2_HASH"

  jq --argjson num "$L2_NUMBER" --arg hash "$L2_HASH" \
     '.genesis.l2.number = $num | .genesis.l2.hash = $hash' \
     config-op/rollup.json > config-op/rollup.json.tmp && mv config-op/rollup.json.tmp config-op/rollup.json

  echo "finished migrate op-geth"
}

migrate