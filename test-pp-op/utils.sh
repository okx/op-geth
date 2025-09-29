#!/bin/bash
set -e
set -x

ROOT_DIR=$(git rev-parse --show-toplevel)
TEST_DIR="$ROOT_DIR/test-pp-op"
PWD_DIR="$TEST_DIR"
TMP_DIR="$TEST_DIR/tmp"

sed_inplace() {
  if [[ "$OSTYPE" == "darwin"* ]]; then
    sed -i '' "$@"
  else
    sed -i "$@"
  fi
}