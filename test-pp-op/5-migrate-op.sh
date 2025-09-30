#!/bin/bash
set -x
set -e
source .env
source tools.sh
source utils.sh

cd $PWD_DIR


#DATA_DIR="data"
#if [ $CHECK_TYPE = "mainnet" ]; then
#    DATA_DIR="mainnet"
#fi

prepare() {
  # Check required files exist
    if [ ! -f "./config-op/genesis.json" ]; then
      echo "❌ ERROR: ./config-op/genesis.json not found!"
      exit 1
    fi

    if [ ! -f "./config-op/rollup.json" ]; then
      echo "❌ ERROR: ./config-op/rollup.json not found!"
      exit 1
    fi

  cp ./config-op/genesis.json ./config-op/genesis-op-raw.json

  #if [ $CHECK_REGENESIS_STRESS_TEST = "true" ]; then
  #  ./scripts/regenesis-with-stress-test.sh $DATA_DIR
  #else
  #  ./scripts/regenesis.sh $DATA_DIR
  #fi

  cp ./config-op/genesis.json ./config-op/genesis-op-before-number.json

  # modify genesis.json & rollup.json for fork block number and parent hash
  sed_inplace 's/"number": 0/"number": '"$FORK_BLOCK"'/' ./config-op/genesis.json
  sed_inplace 's/"parentHash": "0x0000000000000000000000000000000000000000000000000000000000000000"/"parentHash": "'"$PARENT_HASH"'"/' ./config-op/genesis.json
  # FORK_BLOCK_HEX=$(printf "0x%x" "$FORK_BLOCK")
  # sed_inplace 's/"number": "0x0"/"number": "'"$FORK_BLOCK_HEX"'"/' ./config-op/genesis.json
  jq '.genesis.l2.number = '"$FORK_BLOCK" ./config-op/rollup.json > temp_rollup.json && mv temp_rollup.json ./config-op/rollup.json

  cp ./config-op/genesis.json ./config-op/genesis-op-after-number.json

  # Extract contract addresses from state.json and update .env file
  echo "🔧 Extracting contract addresses from state.json..."
  STATE_JSON="$PWD_DIR/config-op/state.json"

  if [ -f "$STATE_JSON" ]; then
      # Extract contract addresses from state.json
      DEPLOYMENTS_TYPE=$(jq -r 'type' "$STATE_JSON")
      if [ "$DEPLOYMENTS_TYPE" = "object" ]; then
          OPCD_TYPE=$(jq -r '.opChainDeployments | type' "$STATE_JSON" 2>/dev/null)
          if [ "$OPCD_TYPE" = "object" ]; then
              DISPUTE_GAME_FACTORY_ADDRESS=$(jq -r '.opChainDeployments.DisputeGameFactoryProxy // empty' "$STATE_JSON")
              L2OO_ADDRESS=$(jq -r '.opChainDeployments.L2OutputOracleProxy // empty' "$STATE_JSON")
              OPCM_IMPL_ADDRESS=$(jq -r '.appliedIntent.opcmAddress // empty' "$STATE_JSON")
              SYSTEM_CONFIG_PROXY_ADDRESS=$(jq -r '.opChainDeployments.SystemConfigProxy // empty' "$STATE_JSON")
              PROXY_ADMIN=$(jq -r '.superchainContracts.SuperchainProxyAdminImpl // empty' "$STATE_JSON")
          elif [ "$OPCD_TYPE" = "array" ]; then
              DISPUTE_GAME_FACTORY_ADDRESS=$(jq -r '.opChainDeployments[0].DisputeGameFactoryProxy // empty' "$STATE_JSON")
              L2OO_ADDRESS=$(jq -r '.opChainDeployments[0].L2OutputOracleProxy // empty' "$STATE_JSON")
              OPCM_IMPL_ADDRESS=$(jq -r '.appliedIntent.opcmAddress // empty' "$STATE_JSON")
              SYSTEM_CONFIG_PROXY_ADDRESS=$(jq -r '.opChainDeployments[0].SystemConfigProxy // empty' "$STATE_JSON")
              PROXY_ADMIN=$(jq -r '.superchainContracts.SuperchainProxyAdminImpl // empty' "$STATE_JSON")
          else
              DISPUTE_GAME_FACTORY_ADDRESS=""
              L2OO_ADDRESS=""
              OPCM_IMPL_ADDRESS=""
              SYSTEM_CONFIG_PROXY_ADDRESS=""
              PROXY_ADMIN=""
          fi

          # Update .env if found
          if [ -n "$DISPUTE_GAME_FACTORY_ADDRESS" ]; then
              echo "✅ Found DisputeGameFactoryProxy address: $DISPUTE_GAME_FACTORY_ADDRESS"
              sed_inplace "s/DISPUTE_GAME_FACTORY_ADDRESS=.*/DISPUTE_GAME_FACTORY_ADDRESS=$DISPUTE_GAME_FACTORY_ADDRESS/" .env
          else
              echo "⚠️  DisputeGameFactoryProxy address not found in opChainDeployments"
          fi

          if [ -n "$L2OO_ADDRESS" ]; then
              echo "✅ Found L2OutputOracleProxy address: $L2OO_ADDRESS"
              sed_inplace "s/L2OO_ADDRESS=.*/L2OO_ADDRESS=$L2OO_ADDRESS/" .env
          else
              echo "⚠️  L2OutputOracleProxy address not found in opChainDeployments"
          fi

          if [ -n "$OPCM_IMPL_ADDRESS" ]; then
              echo "✅ Found opcmAddress address: $OPCM_IMPL_ADDRESS"
              sed_inplace "s/OPCM_IMPL_ADDRESS=.*/OPCM_IMPL_ADDRESS=$OPCM_IMPL_ADDRESS/" .env
          else
              echo "⚠️  opcmAddress address not found in opChainDeployments"
          fi

          if [ -n "$SYSTEM_CONFIG_PROXY_ADDRESS" ]; then
              echo "✅ Found SystemConfigProxy address: $SYSTEM_CONFIG_PROXY_ADDRESS"
              sed_inplace "s/SYSTEM_CONFIG_PROXY_ADDRESS=.*/SYSTEM_CONFIG_PROXY_ADDRESS=$SYSTEM_CONFIG_PROXY_ADDRESS/" .env
          else
              echo "⚠️  SystemConfigProxy address not found in opChainDeployments"
          fi

          if [ -n "$PROXY_ADMIN" ]; then
              echo "✅ Found ProxyAdmin address: $PROXY_ADMIN"
              sed_inplace "s/PROXY_ADMIN=.*/PROXY_ADMIN=$PROXY_ADMIN/" .env
          else
              echo "⚠️  ProxyAdmin address not found in opChainDeployments"
          fi

          # Show summary
          echo "📄 Contract addresses updated in .env:"
          echo "   DISPUTE_GAME_FACTORY_ADDRESS=$DISPUTE_GAME_FACTORY_ADDRESS"
          echo "   L2OO_ADDRESS=$L2OO_ADDRESS"
          echo "   OPCM_IMPL_ADDRESS=$OPCM_IMPL_ADDRESS"
          echo "   SYSTEM_CONFIG_PROXY_ADDRESS=$SYSTEM_CONFIG_PROXY_ADDRESS"
          echo "   PROXY_ADMIN=$PROXY_ADMIN"
      else
          echo "❌ $STATE_JSON is not a valid JSON object"
      fi
  else
      echo "❌ state.json not found at $STATE_JSON"
  fi
}

migrate() {
# init op-geth-seq and op-geth-rpc
export OP_DATA_DIR=/tmp/data/op_geth_data \
export OP_GENESIS_PATH=/data1/op-geth/test-pp-op/config-op/genesis-op-after-number.json \
export ERIGON_CHAINDATA_DIR=/data1/op-geth/test-pp-op/data/rpc/chaindata/ \
export ERIGON_SMTDATA_DIR=/data1/op-geth/test-pp-op/data/rpc/smt/ \
export GETH_CMD=/data1/op-geth/build/bin/geth
${GETH_CMD} --datadir=${OP_DATA_DIR} --gcmode=archive migrate --state.scheme=hash --ignore-addresses=0x000000000000000000000000000000005ca1ab1e --chaindata=${ERIGON_CHAINDATA_DIR} --smt-db-path=${ERIGON_SMTDATA_DIR} ${OP_GENESIS_PATH} 2>&1 | tee migrate.log
}

post_migrate() {

NEW_BLOCK_HASH=$(grep 'Successfully wrote genesis state' migrate.log | tail -1 | sed -n 's/.*hash=\(0x[0-9a-fA-F]\{64\}\).*/\1/p')
echo "NEW_BLOCK_HASH"

ROLLUP_CONTENT=$(jq ".genesis.l2.hash = \"$NEW_BLOCK_HASH\"" config-op/rollup.json)
echo $ROLLUP_CONTENT | jq > config-op/rollup.json

echo "finished migrate op-geth"

$MD5SUM_CMD config-op/genesis.json
# genesis.json is too large to embed in go, so we compress it now and decompress it in go code
gzip -c config-op/genesis.json > config-op/genesis.gz.json

# Ensure prestate files exist and devnetL1.json is consistent before deploying contracts
EXPORT_DIR="$PWD_DIR/data/cannon-data"
rm -rf $EXPORT_DIR
mkdir -p $EXPORT_DIR

# Set network based on ENV
if [ "$ENV" = "local" ]; then
    DOCKER_NETWORK_ARG="--network ${DOCKER_NETWORK}"
else
    DOCKER_NETWORK_ARG="--network host"
fi

ROOTLESS_DOCKER=$(docker info -f "{{println .SecurityOptions}}" | grep rootless || true)
if ! [ -z "$ROOTLESS_DOCKER" ]; then
docker run --rm --privileged \
    -v "$(pwd)/scripts:/scripts" \
    -v "$(pwd)/config-op/rollup.json:/app/op-program/chainconfig/configs/195-rollup.json" \
    -v "$(pwd)/config-op/genesis.gz.json:/app/op-program/chainconfig/configs/195-genesis-l2.json" \
    -v "$EXPORT_DIR:/app/op-program/bin" \
    -w /app \
    ${DOCKER_NETWORK_ARG} \
    "${OP_STACK_IMAGE_TAG}" \
    bash -c "
        echo '📊 Verifying Docker connection:'
        /scripts/dind-install-start.sh
        docker --version
        docker ps --format 'table {{.Names}}\t{{.Status}}' | head -3

        echo '🚀 Running make reproducible-prestate...'
        make reproducible-prestate

        echo '📁 Checking contents of op-program/bin:'
        ls -la /app/op-program/bin/ || echo 'Directory is empty or does not exist'
    "
else
docker run --rm \
    -v /var/run/docker.sock:/var/run/docker.sock \
    -v "$(pwd)/config-op/rollup.json:/app/op-program/chainconfig/configs/195-rollup.json" \
    -v "$(pwd)/config-op/genesis.gz.json:/app/op-program/chainconfig/configs/195-genesis-l2.json" \
    -v "$EXPORT_DIR:/app/op-program/bin" \
    -w /app \
    ${DOCKER_NETWORK_ARG} \
    -e DOCKER_HOST=unix:///var/run/docker.sock \
    "${OP_STACK_IMAGE_TAG}" \
    bash -c "
        echo '📊 Verifying Docker connection:'
        apt-get update
        apt-get install docker.io -y
        docker --version
        docker ps --format 'table {{.Names}}\t{{.Status}}' | head -3

        echo '🚀 Running make reproducible-prestate...'
        make reproducible-prestate

        echo '📁 Checking contents of op-program/bin:'
        ls -la /app/op-program/bin/ || echo 'Directory is empty or does not exist'
    "
fi
}


#prepare
migrate
#post_migrate
