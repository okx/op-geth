#!/bin/bash
set -e
set -x

## Show help
# ./build_images.sh --help

## usage
#./build_images.sh

## Build only op-geth
# ./build_images.sh --op-geth

## Build only op-stack images
# ./build_images.sh --op-stack

## Force rebuild all images
# ./build_images.sh --all --force

## Force rebuild only op-geth
# ./build_images.sh --op-geth --force


source .env

# Default values
BUILD_OP_GETH=false
BUILD_OP_STACK=false
BUILD_ALL=false
FORCE=false

# Parse command line arguments
while [[ $# -gt 0 ]]; do
  case $1 in
    --op-geth)
      BUILD_OP_GETH=true
      shift
      ;;
    --op-stack)
      BUILD_OP_STACK=true
      shift
      ;;
    --all)
      BUILD_ALL=true
      shift
      ;;
    --force)
      FORCE=true
      shift
      ;;
    -h|--help)
      echo "Usage: $0 [OPTIONS]"
      echo "Options:"
      echo "  --op-geth     Build op-geth image only"
      echo "  --op-stack    Build op-stack images only (contracts + opstack)"
      echo "  --all         Build all images (default if no options specified)"
      echo "  --force       Force rebuild even if images exist"
      echo "  -h, --help    Show this help message"
      exit 0
      ;;
    *)
      echo "Unknown option: $1"
      echo "Use --help for usage information"
      exit 1
      ;;
  esac
done

# If no specific options provided, build all
if [ "$BUILD_OP_GETH" = false ] && [ "$BUILD_OP_STACK" = false ] && [ "$BUILD_ALL" = false ]; then
  BUILD_ALL=true
fi

# If --all is specified, set both flags
if [ "$BUILD_ALL" = true ]; then
  BUILD_OP_GETH=true
  BUILD_OP_STACK=true
fi

build_patched_zkevm_bridge_service_image() {
  echo "build patched zkevm bridge service image"
  PWD_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
  rm -rf $PWD_DIR/tmp/zkevm-bridge-service
  mkdir -p $PWD_DIR/tmp
  cd $PWD_DIR/tmp/
  git clone -b v0.6.0-RC16 https://github.com/0xPolygon/zkevm-bridge-service.git
    # it has docker file
  cd zkevm-bridge-service

  # patch zkevm-bridge-service
  git apply $PWD_DIR/patch/xlayer-bridge-service-0001-support-sync-L2-block-at-given-number.patch
  git apply $PWD_DIR/patch/xlayer-bridge-service-0002-skip-reorg-check-after-regenesis.patch
  git apply $PWD_DIR/patch/xlayer-bridge-service-0003-skip-syncing-blocks-before-regenesis.patch

  docker build -t $XLAYER_BRIDGE_SERVICE_IMAGE_TAG .
  cd $PWD_DIR
}

build_aggkit_image() {
  echo "build aggkit image"
  PWD_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
  rm -rf $PWD_DIR/tmp/aggkit
  mkdir -p $PWD_DIR/tmp
  cd $PWD_DIR/tmp/

  echo "Cloning contract repository..."
  git clone -b feature/0.1.0 https://github.com/okx/aggkit.git
  cd ./aggkit
  echo "Cleaning and resting contract repository..."
  git reset --hard; git checkout feature/0.1.0;git pull
  make build-docker
  cd $PWD_DIR
}

build_op_stack_image() {
  echo "build op stack image"
  PWD_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
  rm -rf $PWD_DIR/tmp/optimism
  mkdir -p $PWD_DIR/tmp
  cd $PWD_DIR/tmp/
  echo "Cloning Optimism repository..."
  git clone --recurse-submodules -b dev https://github.com/okx/optimism.git

  # cp Transactor.sol to optimism, which is used for addGameType
  cp $PWD_DIR/contracts/Transactor.sol optimism/packages/contracts-bedrock/src/periphery/Transactor.sol

  cd optimism
  docker build -t $OP_CONTRACTS_IMAGE_TAG -f Dockerfile-contracts .
  docker build -t $OP_STACK_IMAGE_TAG -f Dockerfile-opstack .

  cd $PWD_DIR
}

build_op_geth_image() {
  echo "build op-geth image"
  PWD_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

  PROJECT_ROOT="$(git rev-parse --show-toplevel)"

  echo "PROJECT_ROOT: ${PROJECT_ROOT}"
  
#  # If tmp/optimism doesn't exist, clone it
#  if [ ! -d "$PWD_DIR/tmp/optimism" ]; then
#    rm -rf $PWD_DIR/tmp/optimism
#    mkdir -p $PWD_DIR/tmp
#    cd $PWD_DIR/tmp/
#    echo "Cloning Optimism repository..."
#    git clone --recurse-submodules -b dev https://github.com/okx/optimism.git
#    cd $PWD_DIR
#  fi
  
  cd $PROJECT_ROOT
  docker build -t $OP_GETH_IMAGE_TAG .
  cd $PWD_DIR
}

# Helper function to check if image exists
image_exists() {
  local image_tag=$1
  docker image inspect "$image_tag" >/dev/null 2>&1
}

# Helper function to build if needed
build_if_needed() {
  local image_tag=$1
  local build_function=$2
  local description=$3
  
  if [ "$FORCE" = true ] || ! image_exists "$image_tag"; then
    echo "Building $description..."
    $build_function
  else
    echo "Image $image_tag already exists (use --force to rebuild)"
  fi
}

# Build images based on selected options
if [ "$BUILD_OP_STACK" = true ]; then
  build_if_needed "$OP_CONTRACTS_IMAGE_TAG" "build_op_stack_image" "OP Stack contracts"
  build_if_needed "$OP_STACK_IMAGE_TAG" "build_op_stack_image" "OP Stack image"
fi

if [ "$BUILD_OP_GETH" = true ]; then
  build_if_needed "$OP_GETH_IMAGE_TAG" "build_op_geth_image" "OP-Geth image"
fi

# Always build these images (legacy support)
if [ "$FORCE" = true ] || ! image_exists "$XLAYER_BRIDGE_SERVICE_IMAGE_TAG"; then
  echo "Building patched zkevm bridge service image..."
  build_patched_zkevm_bridge_service_image
else
  echo "Image $XLAYER_BRIDGE_SERVICE_IMAGE_TAG already exists (use --force to rebuild)"
fi

if [ "$FORCE" = true ] || ! image_exists "aggkit:local"; then
  echo "Building aggkit image..."
  build_aggkit_image
else
  echo "Image aggkit:local already exists (use --force to rebuild)"
fi

echo "Build completed!"