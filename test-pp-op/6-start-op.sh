set -e
set -x
source .env
source utils.sh
source tools.sh


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
