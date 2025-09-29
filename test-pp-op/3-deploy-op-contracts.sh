set -e
set -x
source .env
source tools.sh
source utils.sh

cd $PWD_DIR

deploy() {

# Extract contract address from deployment output
TRANSACTOR_ADDRESS=$(echo "$TRANSACTOR_DEPLOY_OUTPUT" | jq -r '.contractAddress // empty')
if [ -z "$TRANSACTOR_ADDRESS" ] || [ "$TRANSACTOR_ADDRESS" = "null" ]; then
  echo "❌ Failed to extract Transactor contract address from deployment output"
  echo "Deployment output: $TRANSACTOR_DEPLOY_OUTPUT"
  exit 1
fi

echo "✅ Transactor contract deployed at: $TRANSACTOR_ADDRESS"

# Update .env file with Transactor address
sed_inplace "s/TRANSACTOR=.*/TRANSACTOR=$TRANSACTOR_ADDRESS/" .env
source .env
echo "✅ Updated TRANSACTOR address in .env: $TRANSACTOR_ADDRESS"

echo "🔧 Bootstrapping superchain with op-deployer..."

docker run \
  --network "$DOCKER_NETWORK" \
  -v "$(pwd)/$CONFIG_DIR:/deployments" \
  -w /app \
  "${OP_CONTRACTS_IMAGE_TAG}" \
  bash -c "
    set -e
    /app/op-deployer/bin/op-deployer bootstrap superchain \
      --l1-rpc-url $L1_RPC_URL_IN_DOCKER \
      --private-key $DEPLOYER_PRIVATE_KEY \
      --artifacts-locator file:///app/packages/contracts-bedrock/forge-artifacts \
      --superchain-proxy-admin-owner $TRANSACTOR_ADDRESS \
      --protocol-versions-owner $ADMIN_OWNER_ADDRESS \
      --guardian $ADMIN_OWNER_ADDRESS \
      --outfile /deployments/superchain.json
  "

echo "🔧 Bootstrapping implementations with op-deployer..."

SUPERCHAIN_JSON="$CONFIG_DIR/superchain.json"
PROTOCOL_VERSIONS_PROXY=$(jq -r '.protocolVersionsProxyAddress' "$SUPERCHAIN_JSON")
SUPERCHAIN_CONFIG_PROXY=$(jq -r '.superchainConfigProxyAddress' "$SUPERCHAIN_JSON")
PROXY_ADMIN=$(jq -r '.proxyAdminAddress' "$SUPERCHAIN_JSON")

docker run \
  --network "$DOCKER_NETWORK" \
  -v "$(pwd)/$CONFIG_DIR:/deployments" \
  -w /app \
  "${OP_CONTRACTS_IMAGE_TAG}" \
  bash -c "
    set -e
    /app/op-deployer/bin/op-deployer bootstrap implementations \
      --artifacts-locator file:///app/packages/contracts-bedrock/forge-artifacts \
      --l1-rpc-url $L1_RPC_URL_IN_DOCKER \
      --outfile /deployments/implementations.json \
      --mips-version "7" \
      --private-key $DEPLOYER_PRIVATE_KEY \
      --protocol-versions-proxy $PROTOCOL_VERSIONS_PROXY \
      --superchain-config-proxy $SUPERCHAIN_CONFIG_PROXY \
      --superchain-proxy-admin $PROXY_ADMIN \
      --upgrade-controller $ADMIN_OWNER_ADDRESS \
      --challenge-period-seconds $CHALLENGE_PERIOD_SECONDS \
      --withdrawal-delay-seconds $WITHDRAWAL_DELAY_SECONDS \
      --proof-maturity-delay-seconds $WITHDRAWAL_DELAY_SECONDS \
      --dispute-game-finality-delay-seconds $DISPUTE_GAME_FINALITY_DELAY_SECONDS
  "

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

# deploy contracts, TODO, should we need to modify source code to deploy contracts?
docker run \
  --network "$DOCKER_NETWORK" \
  -v "$(pwd)/$CONFIG_DIR:/deployments" \
  -w /app \
  "${OP_CONTRACTS_IMAGE_TAG}" \
  bash -c "
    set -e
    echo '🔧 Starting contract deployment with op-deployer...'

    # Deploy using op-deployer, wait for completion before proceeding
    /app/op-deployer/bin/op-deployer apply \
      --workdir /deployments \
      --private-key $DEPLOYER_PRIVATE_KEY \
      --l1-rpc-url $L1_RPC_URL_IN_DOCKER

    echo '📄 Generating L2 genesis and rollup config...'

    # Generate L2 genesis using op-deployer
    /app/op-deployer/bin/op-deployer inspect genesis \
      --workdir /deployments \
      195 > /deployments/genesis.json

    # Generate L2 rollup using op-node
    /app/op-deployer/bin/op-deployer inspect rollup \
      --workdir /deployments \
      195 > /deployments/rollup.json

    echo '✅ Contract deployment completed successfully'
  "

echo "genesis.json and rollup.json are generated in deployments folder"

echo "🎉 OP Stack deployment preparation completed!"
}

deploy_transactor_contract() {
  # Deploy Transactor contract first
  echo "🔧 Deploying Transactor contract..."
  TRANSACTOR_DEPLOY_OUTPUT=$(docker run \
    --network "$DOCKER_NETWORK" \
    -v "$(pwd)/$CONFIG_DIR:/deployments" \
    -w /app \
    "${OP_CONTRACTS_IMAGE_TAG}" \
    bash -c "
      set -e
      cd /app/packages/contracts-bedrock
      cast send --rpc-url $L1_RPC_URL --private-key $DEPLOYER_PRIVATE_KEY --create \"\$(forge inspect src/periphery/Transactor.sol:Transactor bytecode)\$(cast abi-encode 'constructor(address)' $ADMIN_OWNER_ADDRESS | sed 's/0x//')\" --json
    ")

}

deploy_transactor_contract

