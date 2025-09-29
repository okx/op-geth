#!/bin/bash
set -e
set -x

if ! [ -f .env ]; then
  cp example.env .env
fi

source .env



setup_xlayer_erigon() {
  PWD_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

  if [ ! -d "$PWD_DIR/tmp/xlayer-erigon" ]; then
    echo "Cloning xlayer-erigon repository..."
    mkdir -p $PWD_DIR/tmp
    cd $PWD_DIR/tmp/
    git clone -b feat/cliff/op-migration-test https://github.com/okx/xlayer-erigon.git
  else
    echo "xlayer-erigon directory already exists, skipping clone"
  fi

  cd $PWD_DIR/tmp/xlayer-erigon/test-pp-op

  if ! [ -f .env ]; then
    cp example.env .env
  fi

  echo "${ENV}"
  if [ "$ENV" = "mainnet" ]; then
    make mainnet
  else
    echo "Making local"
    make local
  fi
  cd $PWD_DIR
}

setup_xlayer_erigon

#if [ "$CHECK_TYPE" == "mainnet" ]; then
#  make mainnet
#  TMPSTR=""
#  while [ -z "$TMPSTR" ]; do
#    echo "Waiting for mainnet to be ready..."
#    sleep 1
#    docker logs xlayer-seq > tmp.log 2>&1
#    TMPSTR=$(grep "Waiting for txs from the pool" tmp.log || true)
#  done
#  rm -f tmp.log
#else
#  make run
#fi
