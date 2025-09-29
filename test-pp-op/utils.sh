#!/bin/bash
set -e
set -x

source .env

ROOT_DIR="$(dirname "$PWD_DIR")"


setup_xlayer_erigon() {
  PWD_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
  rm -rf $PWD_DIR/tmp/xlayer-erigon
  mkdir -p $PWD_DIR/tmp
  cd $PWD_DIR/tmp/
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