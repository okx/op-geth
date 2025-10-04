#!/bin/bash
set -e
set -x

# init-erigon.sh runs outside the container.
ROOT_DIR=$(which git &>/dev/null && git rev-parse --show-toplevel || echo "/data")
PWD_DIR="$(pwd)"
TMP_DIR="$PWD_DIR/tmp"

sed_inplace() {
  if [[ "$OSTYPE" == "darwin"* ]]; then
    sed -i '' "$@"
  else
    sed -i "$@"
  fi
}