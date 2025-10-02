#!/bin/bash
set -x
set -e
source .env
source tools.sh
source utils.sh

cd $PWD_DIR


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
  cp ./config-op/genesis.json ./config-op/genesis-op-before-number.json

  jq '.config.legacyXLayerBlock = '"$FORK_BLOCK" ./config-op/genesis.json > temp_genesis.json && mv temp_genesis.json ./config-op/genesis.json
  sed_inplace 's/"parentHash": "0x0000000000000000000000000000000000000000000000000000000000000000"/"parentHash": "'"$PARENT_HASH"'"/' ./config-op/genesis.json
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


prepare