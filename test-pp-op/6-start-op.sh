set -e
set -x
source .env
source utils.sh
source tools.sh

# Function to add game type via Transactor
add_game_type_via_transactor() {
    local GAME_TYPE=$1
    local IS_PERMISSIONED=$2
    local CLOCK_EXTENSION_VAL=$3
    local MAX_CLOCK_DURATION_VAL=$4
    local ABSOLUTE_PRESTATE_VAL=$5

    echo "=== Adding Game Type $GAME_TYPE via Transactor ==="
    echo "Game Type: $GAME_TYPE"
    echo "Is Permissioned: $IS_PERMISSIONED"
    echo "Clock Extension: $CLOCK_EXTENSION_VAL"
    echo "Max Clock Duration: $MAX_CLOCK_DURATION_VAL"
    echo ""

    docker run --rm \
      --network "$DOCKER_NETWORK" \
      -v "$(pwd)/$CONFIG_DIR:/deployments" \
      -w /app \
      "${OP_CONTRACTS_IMAGE_TAG}" \
      bash -c "
        set -e

        # Get addresses from environment
        RPC_URL=$L1_RPC_URL_IN_DOCKER
        TRANSACTOR_ADDRESS=$TRANSACTOR
        SENDER_ADDRESS=\$(cast wallet address --private-key $ADMIN_OWNER_PRIVATE_KEY)
        PRIVATE_KEY=$ADMIN_OWNER_PRIVATE_KEY

        # Get addresses from environment variables
        SYSTEM_CONFIG=$SYSTEM_CONFIG_PROXY_ADDRESS
        PROXY_ADMIN=$PROXY_ADMIN
        OPCM=$OPCM_IMPL_ADDRESS
        DISPUTE_GAME_FACTORY=\$(cast call --rpc-url \$RPC_URL \$SYSTEM_CONFIG 'disputeGameFactory()(address)')

        echo 'State JSON Path: '\$STATE_JSON_PATH
        echo 'Dispute Game Factory: '\$DISPUTE_GAME_FACTORY
        echo 'System Config: '\$SYSTEM_CONFIG
        echo 'Proxy Admin: '\$PROXY_ADMIN
        echo 'OPCM: '\$OPCM
        echo 'Transactor Address: '\$TRANSACTOR_ADDRESS
        echo 'RPC URL: '\$RPC_URL
        echo 'Sender Address: '\$SENDER_ADDRESS
        echo ''

        # Retrieve existing permissioned game implementation for parameters
        echo 'Retrieving permissioned game parameters...'
        PERMISSIONED_GAME=\$(cast call --rpc-url \$RPC_URL \$DISPUTE_GAME_FACTORY 'gameImpls(uint32)(address)' 1)
        echo 'Permissioned Game Implementation: '\$PERMISSIONED_GAME

        if [ \"\$PERMISSIONED_GAME\" == \"0x0000000000000000000000000000000000000000\" ]; then
            echo 'Error: No permissioned game found. Cannot retrieve parameters.'
            exit 1
        fi

        # Retrieve parameters from existing permissioned game
        ABSOLUTE_PRESTATE='$ABSOLUTE_PRESTATE_VAL'
        MAX_GAME_DEPTH=\$(cast call --rpc-url \$RPC_URL \$PERMISSIONED_GAME 'maxGameDepth()')
        SPLIT_DEPTH=\$(cast call --rpc-url \$RPC_URL \$PERMISSIONED_GAME 'splitDepth()')
        VM=\$(cast call --rpc-url \$RPC_URL \$PERMISSIONED_GAME 'vm()(address)')

        echo 'Retrieved parameters:'
        echo '  Absolute Prestate: '\$ABSOLUTE_PRESTATE
        echo '  Max Game Depth: '\$MAX_GAME_DEPTH
        echo '  Split Depth: '\$SPLIT_DEPTH
        echo '  Clock Extension: '$CLOCK_EXTENSION_VAL'
        echo '  Max Clock Duration: '$MAX_CLOCK_DURATION_VAL'
        echo '  VM: '\$VM
        echo ''

        # Set initial bond
        INITIAL_BOND='1000000000000000000'  # 1 ETH in wei

        # Create unique salt mixer
        SALT_MIXER='123'

        echo 'Creating addGameType calldata...'

        # Create calldata for addGameType function
        ADDGAMETYPE_CALLDATA=\$(cast calldata 'addGameType((string,address,address,address,uint32,bytes32,uint256,uint256,uint64,uint64,uint256,address,bool)[])' \
        \"[(\
        \\\"\$SALT_MIXER\\\",\
        \$SYSTEM_CONFIG,\
        \$PROXY_ADMIN,\
        0x0000000000000000000000000000000000000000,\
        $GAME_TYPE,\
        \$ABSOLUTE_PRESTATE,\
        \$MAX_GAME_DEPTH,\
        \$SPLIT_DEPTH,\
        $CLOCK_EXTENSION_VAL,\
        $MAX_CLOCK_DURATION_VAL,\
        \$INITIAL_BOND,\
        \$VM,\
        $IS_PERMISSIONED\
        )]\")

        echo 'AddGameType calldata: '\$ADDGAMETYPE_CALLDATA
        echo ''

        # Create calldata for Transactor's DELEGATECALL function
        echo 'Creating Transactor DELEGATECALL calldata...'
        TRANSACTOR_CALLDATA=\$(cast calldata 'DELEGATECALL(address,bytes)' \$OPCM \$ADDGAMETYPE_CALLDATA)

        echo 'Transactor calldata: '\$TRANSACTOR_CALLDATA
        echo ''

        # Execute the transaction through Transactor
        echo 'Executing transaction via Transactor...'
        echo 'Target: '\$TRANSACTOR_ADDRESS
        echo 'From: '\$SENDER_ADDRESS

        cast send \\
            --rpc-url \$RPC_URL \\
            --private-key \$PRIVATE_KEY \\
            --from \$SENDER_ADDRESS \\
            \$TRANSACTOR_ADDRESS \\
            \$TRANSACTOR_CALLDATA

        echo ''
        echo 'Transaction sent! Check the transaction hash above for confirmation.'
        echo ''

        # Verify the new game type was added
        echo 'Verifying new game type was added...'
        NEW_GAME_IMPL=\$(cast call --rpc-url \$RPC_URL \$DISPUTE_GAME_FACTORY 'gameImpls(uint32)(address)' $GAME_TYPE)

        if [ \"\$NEW_GAME_IMPL\" != \"0x0000000000000000000000000000000000000000\" ]; then
            echo '✅ Success! New game type $GAME_TYPE added.'
            echo 'Game Type $GAME_TYPE Implementation: '\$NEW_GAME_IMPL
        else
            echo '❌ Warning: Could not verify game type was added. Check transaction status.'
        fi

        echo '✅ AddGameType operations completed successfully'
      "
}

${DOCKER_COMPOSE_CMD} up -d op-batcher

sleep 10
# Check for L2 genesis hash mismatch
LOG_OUTPUT=$(${DOCKER_COMPOSE_CMD} logs op-seq 2>&1 | tail -20)
if echo "$LOG_OUTPUT" | grep -q "expected L2 genesis hash to match L2 block at genesis block number"; then
    echo "❌ L2 genesis hash mismatch detected!"
    echo "Error details:"
    echo "$LOG_OUTPUT" | grep "expected L2 genesis hash to match L2 block at genesis block number"
    exit 1
fi

${DOCKER_COMPOSE_CMD} up -d op-rpc

sleep 10
