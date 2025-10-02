#!/bin/bash
set -x
set -e
source .env
source tools.sh
source utils.sh

cd $PWD_DIR

post_migrate() {

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
docker run -it --privileged \
    --platform linux/amd64 \
    -v "$(pwd)/scripts:/scripts" \
    -v "$(pwd)/config-op/rollup.json:/app/op-program/chainconfig/configs/196-rollup.json" \
    -v "$(pwd)/config-op/genesis.gz.json:/app/op-program/chainconfig/configs/196-genesis-l2.json" \
    -v "$EXPORT_DIR:/app/op-program/bin" \
    --name my-op-temp \
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
docker run -it \
    --platform linux/amd64 \
    -v /var/run/docker.sock:/var/run/docker.sock \
    -v "$(pwd)/config-op/rollup.json:/app/op-program/chainconfig/configs/196-rollup.json" \
    -v "$(pwd)/config-op/genesis.gz.json:/app/op-program/chainconfig/configs/196-genesis-l2.json" \
    -v "$EXPORT_DIR:/app/op-program/bin" \
    --name my-op-temp \
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

post_migrate