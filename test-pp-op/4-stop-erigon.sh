#!/bin/bash
set -x
set -e
source .env
source tools.sh
source utils.sh

cd $PWD_DIR

## Stop X Layer services
${DOCKER_COMPOSE_CMD} stop xlayer-seq
${DOCKER_COMPOSE_CMD} stop xlayer-rpc
#
#docker compose stop xlayer-bridge-service
#docker compose stop xlayer-bridge-ui
#docker compose stop xlayer-agg-sender

# Get fork block number and parent hash
LOG_OUTPUT=$(docker logs xlayer-seq 2>&1)
FORK_BLOCK=$(echo "$LOG_OUTPUT" | grep "Finish block" | tail -1 | sed -n 's/.*Finish block \([0-9]*\) with.*/\1/p')
# Add 1 to FORK_BLOCK
#FORK_BLOCK=$((FORK_BLOCK + 1))
echo "FORK_BLOCK=$FORK_BLOCK"


sed_inplace "s/FORK_BLOCK=.*/FORK_BLOCK=$FORK_BLOCK/" .env
PARENT_HASH=$(echo "$LOG_OUTPUT" | grep "RPC Daemon notified of new headers" | tail -1 | sed -n 's/.*hash=\([0-9a-fx]*\) .*/\1/p')
echo "PARENT_HASH=$PARENT_HASH"
sed_inplace "s/PARENT_HASH=.*/PARENT_HASH=$PARENT_HASH/" .env