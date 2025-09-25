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
  git clone -b feat/cliff/op-migration-test https://github.com/okx/xlayer-erigon.git

  cd $PWD_DIR
}