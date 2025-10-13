#!/bin/bash
set -x
set -e
source .env
source tools.sh

PWD_DIR=$(pwd)

if [ -z "$CHAIN_ID" ]; then
  echo "❌ ERROR: CHAIN_ID is not set. Set it explicitly or derive it from intent.toml before proceeding."
  exit 1
fi
if ! [[ "$CHAIN_ID" =~ ^[0-9]+$ ]]; then
  echo "❌ ERROR: CHAIN_ID must be a numeric value, got: '$CHAIN_ID'"
  exit 1
fi
# Function to show usage
show_usage() {
    echo "Usage: $0 [OPTIONS]"
    echo "Options:"
    echo "  -a, --arch ARCH    Target architecture (x86|arm|amd64|arm64)"
    echo "                     Default: auto-detect from OSTYPE"
    echo "  -h, --help         Show this help message"
    echo ""
    echo "Architecture mapping:"
    echo "  x86/amd64 -> linux/amd64"
    echo "  arm/arm64 -> linux/arm64"
    echo ""
    echo "Current OSTYPE: $OSTYPE"
}

# Default architecture detection based on OSTYPE
detect_arch() {
    case "$OSTYPE" in
        *darwin*)
            if [[ $(uname -m) == "arm64" ]]; then
                echo "linux/arm64"
            else
                echo "linux/amd64"
            fi
            ;;
        *linux*)
            if [[ $(uname -m) == "aarch64" ]] || [[ $(uname -m) == "arm64" ]]; then
                echo "linux/arm64"
            else
                echo "linux/amd64"
            fi
            ;;
        *)
            echo "linux/amd64"  # Default fallback
            ;;
    esac
}

# Parse command line arguments
DOCKER_PLATFORM=""
while [[ $# -gt 0 ]]; do
    case $1 in
        -a|--arch)
            case "$2" in
                x86|amd64)
                    DOCKER_PLATFORM="linux/amd64"
                    ;;
                arm|arm64)
                    DOCKER_PLATFORM="linux/arm64"
                    ;;
                *)
                    echo "Error: Invalid architecture '$2'"
                    echo "Valid options: x86, amd64, arm, arm64"
                    exit 1
                    ;;
            esac
            shift 2
            ;;
        -h|--help)
            show_usage
            exit 0
            ;;
        *)
            echo "Error: Unknown option '$1'"
            show_usage
            exit 1
            ;;
    esac
done

# Set default platform if not specified
if [ -z "$DOCKER_PLATFORM" ]; then
    DOCKER_PLATFORM=$(detect_arch)
    echo "Auto-detected platform: $DOCKER_PLATFORM"
else
    echo "Using specified platform: $DOCKER_PLATFORM"
fi

post_migrate() {
    # Check if genesis.json exists, panic if it doesn't
    if [ ! -f "merged.genesis.json" ]; then
        echo "ERROR: merged.genesis.json does not exist!"
        echo "Please ensure the genesis.json file is present before running this script."
        exit 1
    fi

    $MD5SUM_CMD merged.genesis.json
    # genesis.json is too large to embed in go, so we compress it now and decompress it in go code
    gzip -c merged.genesis.json > config-op/merged.genesis.gz.json

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
            --platform $DOCKER_PLATFORM \
            -v "$(pwd)/scripts:/scripts" \
            -v "$(pwd)/config-op/rollup.json:/app/op-program/chainconfig/configs/${CHAIN_ID}-rollup.json" \
            -v "$(pwd)/config-op/merged.genesis.gz.json:/app/op-program/chainconfig/configs/${CHAIN_ID}-genesis-l2.json" \
            -v "$EXPORT_DIR:/app/op-program/bin" \
            --name op-program \
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
            --platform $DOCKER_PLATFORM \
            -v /var/run/docker.sock:/var/run/docker.sock \
            -v "$(pwd)/config-op/rollup.json:/app/op-program/chainconfig/configs/${CHAIN_ID}-rollup.json" \
            -v "$(pwd)/config-op/merged.genesis.gz.json:/app/op-program/chainconfig/configs/${CHAIN_ID}-genesis-l2.json" \
            -v "$EXPORT_DIR:/app/op-program/bin" \
            --name op-program \
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