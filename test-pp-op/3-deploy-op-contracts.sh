#!/bin/bash
set -e
set -x

source .env
source tools.sh
source utils.sh

## TODO: get chainId from intent.toml
if [ "$ENV" = "local" ]; then
    CHAIN_ID=195
    DOCKER_NETWORK_ARG="$DOCKER_NETWORK"
else
    CHAIN_ID=196
    DOCKER_NETWORK_ARG="host"
fi

cd $PWD_DIR

deploy_transactor_contract() {
  # Deploy Transactor contract first
  echo "🔧 Deploying Transactor contract..."
  
  # Debug: Show environment variables
  echo "ENV: $ENV"
  echo "DOCKER_NETWORK: $DOCKER_NETWORK"
  echo "L1_RPC_URL_IN_DOCKER: $L1_RPC_URL_IN_DOCKER"
  echo "DEPLOYER_PRIVATE_KEY: ${DEPLOYER_PRIVATE_KEY:0:10}..."
  echo "ADMIN_OWNER_ADDRESS: $ADMIN_OWNER_ADDRESS"
  echo "OP_CONTRACTS_IMAGE_TAG: $OP_CONTRACTS_IMAGE_TAG"
  
  # Build docker run command with conditional network flag
  DOCKER_ARGS=()
  DOCKER_ARGS+=("-v" "$(pwd)/$CONFIG_DIR:/deployments")
  DOCKER_ARGS+=("-w" "/app")
  DOCKER_ARGS+=("-e" "CURL_CA_BUNDLE=")
  DOCKER_ARGS+=("-e" "GIT_SSL_NO_VERIFY=true")
  DOCKER_ARGS+=("-e" "NODE_TLS_REJECT_UNAUTHORIZED=0")
  DOCKER_ARGS+=("-e" "GODEBUG=x509ignoreCN=1")
  DOCKER_ARGS+=("--network" "$DOCKER_NETWORK_ARG")
  
  DOCKER_ARGS+=("$OP_CONTRACTS_IMAGE_TAG")
  
  # Create the bash command with SSL verification disabled
  BASH_CMD="set -e && cd /app/packages/contracts-bedrock && export CURL_CA_BUNDLE= && export GIT_SSL_NO_VERIFY=true && cast send --rpc-url $L1_RPC_URL_IN_DOCKER --private-key $DEPLOYER_PRIVATE_KEY --create \"\$(forge inspect src/periphery/Transactor.sol:Transactor bytecode)\$(cast abi-encode 'constructor(address)' $ADMIN_OWNER_ADDRESS | sed 's/0x//')\" --json"
  
  echo "🔧 Executing Docker command..."
  echo "Command: docker run ${DOCKER_ARGS[*]} bash -c \"$BASH_CMD\""
  
  TRANSACTOR_DEPLOY_OUTPUT=$(docker run "${DOCKER_ARGS[@]}" bash -c "$BASH_CMD")
  
  echo "Raw deployment output:"
  echo "$TRANSACTOR_DEPLOY_OUTPUT"
  echo "--- End of raw output ---"

  # Extract contract address from deployment output
  TRANSACTOR_ADDRESS=$(echo "$TRANSACTOR_DEPLOY_OUTPUT" | jq -r '.contractAddress // empty' 2>/dev/null || echo "")
  if [ -z "$TRANSACTOR_ADDRESS" ] || [ "$TRANSACTOR_ADDRESS" = "null" ]; then
    echo "❌ Failed to extract Transactor contract address from deployment output"
    echo "Deployment output: $TRANSACTOR_DEPLOY_OUTPUT"
    echo "Trying to extract address manually..."
    
    # Try alternative extraction methods
    TRANSACTOR_ADDRESS=$(echo "$TRANSACTOR_DEPLOY_OUTPUT" | grep -o '"contractAddress":"[^"]*"' | cut -d'"' -f4 || echo "")
    if [ -z "$TRANSACTOR_ADDRESS" ]; then
      TRANSACTOR_ADDRESS=$(echo "$TRANSACTOR_DEPLOY_OUTPUT" | grep -o '0x[a-fA-F0-9]\{40\}' | head -1 || echo "")
    fi
    
    if [ -z "$TRANSACTOR_ADDRESS" ]; then
      echo "❌ Still failed to extract contract address"
      exit 1
    else
      echo "✅ Extracted address manually: $TRANSACTOR_ADDRESS"
    fi
  fi

  echo "✅ Transactor contract deployed at: $TRANSACTOR_ADDRESS"

  # Update .env file with Transactor address
  sed_inplace "s/TRANSACTOR=.*/TRANSACTOR=$TRANSACTOR_ADDRESS/" .env
  source .env
  echo "✅ Updated TRANSACTOR address in .env: $TRANSACTOR_ADDRESS"
}

# Bootstrapping superchain with op-deployer
# Output: after deploy, it will output `superchain.json` under config-op
# e.g. {
#  "protocolVersionsImplAddress": "0x37e15e4d6dffa9e5e320ee1ec036922e563cb76c",
#  "protocolVersionsProxyAddress": "0xfb5a7622e23e0f807b97a8ed608d50d56d202688",
#  "superchainConfigImplAddress": "0xce28685eb204186b557133766eca00334eb441e4",
#  "superchainConfigProxyAddress": "0x8c15b9d397b5bf29e114aebff0663fdd34976756",
#  "proxyAdminAddress": "0x210879bec4c74c7e4e6df5e919f9525d75e15183"
# }
deploy_op_stack_bootstrap_superchain() {
  source .env
  TRANSACTOR_ADDRESS=${TRANSACTOR}
  echo "🔧 Bootstrapping superchain with op-deployer..."

  # Build docker run command with conditional network flag
  DOCKER_ARGS=()
  DOCKER_ARGS+=("-v" "$(pwd)/$CONFIG_DIR:/deployments")
  DOCKER_ARGS+=("-w" "/app")
  DOCKER_ARGS+=("-e" "CURL_CA_BUNDLE=")
  DOCKER_ARGS+=("-e" "GIT_SSL_NO_VERIFY=true")
  DOCKER_ARGS+=("-e" "NODE_TLS_REJECT_UNAUTHORIZED=0")
  DOCKER_ARGS+=("-e" "GODEBUG=x509ignoreCN=1,x509ignoreUnknownCA=1,x509ignoreSystemRoots=1")
  DOCKER_ARGS+=("--network" "$DOCKER_NETWORK_ARG")

  DOCKER_ARGS+=("$OP_CONTRACTS_IMAGE_TAG")

  BASH_CMD="set -e && /app/op-deployer/bin/op-deployer bootstrap superchain --l1-rpc-url $L1_RPC_URL_IN_DOCKER --private-key $DEPLOYER_PRIVATE_KEY --artifacts-locator file:///app/packages/contracts-bedrock/forge-artifacts --superchain-proxy-admin-owner $TRANSACTOR_ADDRESS --protocol-versions-owner $ADMIN_OWNER_ADDRESS --guardian $ADMIN_OWNER_ADDRESS --outfile /deployments/superchain.json"

  docker run "${DOCKER_ARGS[@]}" bash -c "$BASH_CMD"

  echo "🔧 Bootstrapping implementations with op-deployer..."

  SUPERCHAIN_JSON="$CONFIG_DIR/superchain.json"
  PROTOCOL_VERSIONS_PROXY=$(jq -r '.protocolVersionsProxyAddress' "$SUPERCHAIN_JSON")
  SUPERCHAIN_CONFIG_PROXY=$(jq -r '.superchainConfigProxyAddress' "$SUPERCHAIN_JSON")
  PROXY_ADMIN=$(jq -r '.proxyAdminAddress' "$SUPERCHAIN_JSON")
}

deploy_op_stack_bootstrap_implementations() {
  # Build docker run command with conditional network flag
  DOCKER_ARGS=()
  DOCKER_ARGS+=("-v" "$(pwd)/$CONFIG_DIR:/deployments")
  DOCKER_ARGS+=("-w" "/app")
  DOCKER_ARGS+=("-e" "CURL_CA_BUNDLE=")
  DOCKER_ARGS+=("-e" "GIT_SSL_NO_VERIFY=true")
  DOCKER_ARGS+=("-e" "NODE_TLS_REJECT_UNAUTHORIZED=0")
  DOCKER_ARGS+=("--network" "$DOCKER_NETWORK_ARG")

  DOCKER_ARGS+=("$OP_CONTRACTS_IMAGE_TAG")

  BASH_CMD="set -e && export CURL_CA_BUNDLE= && export GIT_SSL_NO_VERIFY=true && /app/op-deployer/bin/op-deployer bootstrap implementations --artifacts-locator file:///app/packages/contracts-bedrock/forge-artifacts --l1-rpc-url $L1_RPC_URL_IN_DOCKER --outfile /deployments/implementations.json --mips-version \"7\" --private-key $DEPLOYER_PRIVATE_KEY --protocol-versions-proxy $PROTOCOL_VERSIONS_PROXY --superchain-config-proxy $SUPERCHAIN_CONFIG_PROXY --superchain-proxy-admin $PROXY_ADMIN --upgrade-controller $ADMIN_OWNER_ADDRESS --challenge-period-seconds $CHALLENGE_PERIOD_SECONDS --withdrawal-delay-seconds $WITHDRAWAL_DELAY_SECONDS --proof-maturity-delay-seconds $WITHDRAWAL_DELAY_SECONDS --dispute-game-finality-delay-seconds $DISPUTE_GAME_FINALITY_DELAY_SECONDS"

  docker run "${DOCKER_ARGS[@]}" bash -c "$BASH_CMD"

  # Update intent.toml with Transactor address for l1ProxyAdminOwner
  sed_inplace "s/l1ProxyAdminOwner = \".*\"/l1ProxyAdminOwner = \"$TRANSACTOR_ADDRESS\"/" ./config-op/intent.toml
  echo "✅ Updated l1ProxyAdminOwner in intent.toml: $TRANSACTOR_ADDRESS"

  # Read opcmAddress from implementations.json and write it into intent.toml
  OPCM_ADDRESS=$(jq -r '.opcmAddress' ./config-op/implementations.json)
  if [ -z "$OPCM_ADDRESS" ] || [ "$OPCM_ADDRESS" = "null" ]; then
    echo "❌ Failed to read opcmAddress from implementations.json"
    exit 1
  fi

  # Replace the opcmAddress field in intent.toml with the new value
  sed_inplace "s/^opcmAddress = \".*\"/opcmAddress = \"$OPCM_ADDRESS\"/" ./config-op/intent.toml
  echo "✅ Updated opcmAddress ($OPCM_ADDRESS) in intent.toml"
}

deploy_op_stack_contracts() {
  # Deploy contracts, TODO: should we need to modify source code to deploy contracts?
  # Build docker run command with conditional network flag
  DOCKER_ARGS=()
  DOCKER_ARGS+=("-v" "$(pwd)/$CONFIG_DIR:/deployments")
  DOCKER_ARGS+=("-w" "/app")
  DOCKER_ARGS+=("-e" "CURL_CA_BUNDLE=")
  DOCKER_ARGS+=("-e" "GIT_SSL_NO_VERIFY=true")
  DOCKER_ARGS+=("-e" "NODE_TLS_REJECT_UNAUTHORIZED=0")
  DOCKER_ARGS+=("--network" "$DOCKER_NETWORK_ARG")

  DOCKER_ARGS+=("$OP_CONTRACTS_IMAGE_TAG")

  BASH_CMD="set -e && export CURL_CA_BUNDLE= && export GIT_SSL_NO_VERIFY=true && echo '🔧 Starting contract deployment with op-deployer...' && echo '' && echo 'Deploy using op-deployer, wait for completion before proceeding' && /app/op-deployer/bin/op-deployer apply --workdir /deployments --private-key $DEPLOYER_PRIVATE_KEY --l1-rpc-url $L1_RPC_URL_IN_DOCKER && echo '' && echo '📄 Generating L2 genesis and rollup config...' && echo '' && echo 'Generate L2 genesis using op-deployer' && /app/op-deployer/bin/op-deployer inspect genesis --workdir /deployments $CHAIN_ID > /deployments/genesis.json && echo '' && echo 'Generate L2 rollup using op-node' && /app/op-deployer/bin/op-deployer inspect rollup --workdir /deployments $CHAIN_ID > /deployments/rollup.json && echo '' && echo '✅ Contract deployment completed successfully'"

  docker run "${DOCKER_ARGS[@]}" bash -c "$BASH_CMD"

  echo "genesis.json and rollup.json are generated in deployments folder"
  echo "🎉 OP Stack deployment preparation completed!"
}



cp ./config-op/intent.${ENV}.toml.bak ./config-op/intent.toml
cp ./config-op/state.json.bak ./config-op/state.json

deploy_transactor_contract
deploy_op_stack_bootstrap_superchain
deploy_op_stack_bootstrap_implementations
deploy_op_stack_contracts