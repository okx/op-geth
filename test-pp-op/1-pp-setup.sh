#!/bin/bash
set -e
set -x

if ! [ -f .env ]; then
  echo "need to provide .env file"
fi

source .env
source tools.sh

if [ "$ENV" = "local" ]; then
    COMPOSE_FILE="docker-compose-local.yml"
else
    COMPOSE_FILE="docker-compose.yml"
fi

DOCKER_COMPOSE=$(shell docker compose version >/dev/null 2>&1 && echo "docker compose" || echo "docker-compose")
DOCKER_COMPOSE_CMD="${DOCKER_COMPOSE} -f ${COMPOSE_FILE}"
echo ${DOCKER_COMPOSE_CMD}

# 1. setup l1
# 2. run & init erigon pp
start_local_xlayer_erigon() {
  export ENV=local
  make run_erigon
}

#setup_xlayer_erigon() {
#  PWD_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
#
#  if [ ! -d "$PWD_DIR/tmp/xlayer-erigon" ]; then
#    echo "Cloning xlayer-erigon repository..."
#    mkdir -p $PWD_DIR/tmp
#    cd $PWD_DIR/tmp/
#    git clone -b feat/cliff/op-migration-test https://github.com/okx/xlayer-erigon.git
#  else
#    echo "xlayer-erigon directory already exists, skipping clone"
#  fi
#
#  cd $PWD_DIR/tmp/xlayer-erigon/test-pp-op
#
#  if ! [ -f .env ]; then
#    cp example.env .env
#  fi
#
#  echo "${ENV}"
#  if [ "$ENV" = "mainnet" ]; then
#    make mainnet
#  else
#    echo "Making local"
#    make local
#  fi
#  cd $PWD_DIR
#}
#

setup_xlayer_erigon() {
if [ "$ENV" = "local" ]; then
    echo "Starting local environment setup..."
    start_local_xlayer_erigon
elif [ "$ENV" = "testnet" ]; then
    echo "Starting testnet environment setup..."
    setup_testnet_xlayer_erigon
else
    echo "Starting ${ENV} environment setup..."
    echo "not implemented"
    exit 1
fi
}
setup_xlayer_erigon
