
PWD_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd $PWD_DIR
EXPORT_DIR="$PWD_DIR/data/cannon-data"
mkdir -p $EXPORT_DIR

echo "Adding game type to DisputeGameFactory via op-deployer..."

# Retrieve existing values from chain for reference
# Get permissioned game implementation
PERMISSIONED_GAME_RAW=$(cast call --rpc-url $L1_RPC_URL $DISPUTE_GAME_FACTORY_ADDRESS "gameImpls(uint32)" 1)
# Convert 32-byte hex to 20-byte address (last 40 hex chars, with 0x prefix)
PERMISSIONED_GAME="0x${PERMISSIONED_GAME_RAW: -40}"

# Get prestate value from prestate-proof-mt64.json
ABSOLUTE_PRESTATE=$(jq -r '.pre' "$EXPORT_DIR/prestate-proof-mt64.json")
MAX_GAME_DEPTH=$(cast call --rpc-url $L1_RPC_URL $PERMISSIONED_GAME "maxGameDepth()")
SPLIT_DEPTH=$(cast call --rpc-url $L1_RPC_URL $PERMISSIONED_GAME "splitDepth()")
VM_RAW=$(cast call --rpc-url $L1_RPC_URL $PERMISSIONED_GAME "vm()")
VM="0x${VM_RAW: -40}"
ANCHOR_STATE_REGISTRY=$(cast call --rpc-url $L1_RPC_URL $PERMISSIONED_GAME "anchorStateRegistry()")
L2_CHAIN_ID=$(cast call --rpc-url $L1_RPC_URL $PERMISSIONED_GAME "l2ChainId()")

# Call the function to add game type 1 (permissioned) via Transactor
add_game_type_via_transactor 1 true $TEMP_CLOCK_EXTENSION $TEMP_MAX_CLOCK_DURATION $ABSOLUTE_PRESTATE

export GAME_TYPE=1
${DOCKER_COMPOSE_CMD} up -d op-proposer

echo "Waiting for op-proposer to create a game..."
GAME_CREATED=false
MAX_WAIT_TIME=600  # 10 minutes timeout
WAIT_COUNT=0

while [ "$GAME_CREATED" = false ] && [ $WAIT_COUNT -lt $MAX_WAIT_TIME ]; do
    # Check if a game was created by op-proposer
    GAME_COUNT=$(cast call --rpc-url $L1_RPC_URL $DISPUTE_GAME_FACTORY_ADDRESS "gameCount()(uint256)")
    if [ "$GAME_COUNT" -gt 0 ]; then
        echo "✅ Game created! Game count: $GAME_COUNT"
        GAME_CREATED=true
    else
        echo "⏳ Waiting for game creation... ($WAIT_COUNT/$MAX_WAIT_TIME seconds)"
        sleep 1
        WAIT_COUNT=$((WAIT_COUNT + 1))
    fi
done

if [ "$GAME_CREATED" = false ]; then
    echo "❌ Timeout waiting for game creation"
    exit 1
fi

echo "🛑 Stopping op-proposer..."
${DOCKER_COMPOSE_CMD} stop op-proposer

echo "⏰ Sleeping for ($TEMP_MAX_CLOCK_DURATION seconds)..."
sleep $TEMP_MAX_CLOCK_DURATION

echo "🔧 Executing dispute resolution sequence using op-challenger..."

# Get the latest game address
LATEST_GAME_INDEX=$((GAME_COUNT - 1))
GAME_INFO=$(cast call --rpc-url $L1_RPC_URL $DISPUTE_GAME_FACTORY_ADDRESS "gameAtIndex(uint256)(uint256,uint256,address)" $LATEST_GAME_INDEX)
# Extract the third value (address) from the returned tuple - address is the last 40 hex chars
GAME_ADDRESS="0x${GAME_INFO: -40}"

echo "Latest game address: $GAME_ADDRESS"

# Execute the dispute resolution sequence using op-challenger commands
echo "1. Resolving claim (0,0) using op-challenger..."
docker run --rm \
  --network "$DOCKER_NETWORK" \
  -v "$(pwd)/data/cannon-data:/data" \
  -v "$(pwd)/config-op/rollup.json:/rollup.json" \
  -v "$(pwd)/config-op/genesis.json:/l2-genesis.json" \
  "${OP_STACK_IMAGE_TAG}" \
  /app/op-challenger/bin/op-challenger resolve-claim \
    --l1-eth-rpc=${L1_RPC_URL_IN_DOCKER} \
    --private-key=${OP_CHALLENGER_PRIVATE_KEY} \
    --game-address=$GAME_ADDRESS \
    --claim=0

echo "2. Resolving game using op-challenger..."
docker run --rm \
  --network "$DOCKER_NETWORK" \
  -v "$(pwd)/data/cannon-data:/data" \
  -v "$(pwd)/config-op/rollup.json:/rollup.json" \
  -v "$(pwd)/config-op/genesis.json:/l2-genesis.json" \
  "${OP_STACK_IMAGE_TAG}" \
  /app/op-challenger/bin/op-challenger resolve \
    --l1-eth-rpc=${L1_RPC_URL_IN_DOCKER} \
    --private-key=${OP_CHALLENGER_PRIVATE_KEY} \
    --game-address=$GAME_ADDRESS

sleep $DISPUTE_GAME_FINALITY_DELAY_SECONDS

echo "3. Claiming credit for proposer using cast command..."
docker run --rm \
  --network "$DOCKER_NETWORK" \
  "${OP_STACK_IMAGE_TAG}" \
  cast send \
    --rpc-url ${L1_RPC_URL_IN_DOCKER} \
    --private-key ${OP_CHALLENGER_PRIVATE_KEY} \
    $GAME_ADDRESS \
    "claimCredit(address)" \
    $PROPOSER_ADDRESS

echo "✅ Dispute resolution sequence completed using op-challenger commands!"

# Retrieve existing values from chain for reference
# Get permissioned game implementation
PERMISSIONED_GAME_RAW=$(cast call --rpc-url $L1_RPC_URL $DISPUTE_GAME_FACTORY_ADDRESS "gameImpls(uint32)" 1)
# Convert 32-byte hex to 20-byte address (last 40 hex chars, with 0x prefix)
PERMISSIONED_GAME="0x${PERMISSIONED_GAME_RAW: -40}"

ABSOLUTE_PRESTATE=$(cast call --rpc-url $L1_RPC_URL $PERMISSIONED_GAME "absolutePrestate()")
ANCHOR_STATE_REGISTRY=$(cast call --rpc-url $L1_RPC_URL $PERMISSIONED_GAME "anchorStateRegistry()")

# Call the function to add game type 0 (permissionless) via Transactor
add_game_type_via_transactor 0 false $CLOCK_EXTENSION $MAX_CLOCK_DURATION $ABSOLUTE_PRESTATE

docker run --rm \
  --network "$DOCKER_NETWORK" \
  -v "$(pwd)/$CONFIG_DIR:/deployments" \
  -w /app \
  "${OP_CONTRACTS_IMAGE_TAG}" \
  bash -c "
    set -e

    echo '📋 Gathering contract addresses and generating calldata...'
    DISPUTE_GAME_FACTORY_ADDR=\$(cast call --rpc-url $L1_RPC_URL_IN_DOCKER $SYSTEM_CONFIG_PROXY_ADDRESS 'disputeGameFactory()(address)')
    OPTIMISM_PORTAL_ADDR=\$(cast call --rpc-url $L1_RPC_URL_IN_DOCKER $SYSTEM_CONFIG_PROXY_ADDRESS 'optimismPortal()(address)')
    echo 'disputeGameFactory: '\$DISPUTE_GAME_FACTORY_ADDR
    echo 'optimismPortal: '\$OPTIMISM_PORTAL_ADDR

    # Get anchorStateRegistry address with proper return type specification
    ANCHOR_STATE_REGISTRY_ADDR=\$(cast call --rpc-url $L1_RPC_URL_IN_DOCKER \$OPTIMISM_PORTAL_ADDR 'anchorStateRegistry()(address)')
    echo 'anchorStateRegistry: '\$ANCHOR_STATE_REGISTRY_ADDR

    GAME_ADDR=\$(cast call --rpc-url $L1_RPC_URL_IN_DOCKER \$DISPUTE_GAME_FACTORY_ADDR 'gameImpls(uint32)(address)' 0)
    echo 'gameImpls(0): '\$GAME_ADDR

    cast send \$ANCHOR_STATE_REGISTRY_ADDR 'setRespectedGameType(uint32)' 0 --rpc-url $L1_RPC_URL_IN_DOCKER --private-key $ADMIN_OWNER_PRIVATE_KEY

    echo '✅ setRespectedGameType completed successfully'
  "

export GAME_TYPE=0

sleep $TEMP_GAME_WINDOW
${DOCKER_COMPOSE_CMD} up -d op-proposer op-challenger op-dispute-mon
#
## Run appropriate state check based on mode
#if [ "$CHECK_REGENESIS_STRESS_TEST" = "true" ]; then
#  echo "🔍 Running regenesis check (with stress test)..."
#  ./scripts/check-regenesis-with-stress-test.sh
#else
#  echo "🔍 Running regenesis check..."
#  ./scripts/check-regenesis.sh
#fi
