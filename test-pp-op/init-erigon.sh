#!/bin/bash
set -e
set -x

# =============================================================================
# Init Erigon Chain
# =============================================================================
# The script is a comprehensive initialization script for setting up a local XLayer Erigon
# development environment. It handles contract deployment, configuration generation, and system setup for a ZK-EVM rollup test environment.
#



source .env
source utils.sh

git checkout config/*

if [ ! -d "tmp/xlayer-erigon" ]; then
    echo "ERROR: tmp/xlayer-erigon directory does not exist!"
    echo "Please ensure the xlayer-erigon directory is present before running this script."
    echo "the hack cmd is required"
    exit 1
fi

mkdir -p $TMP_DIR
cd $TMP_DIR
if [ -d "./xlayer-contracts" ]; then
    cd xlayer-contracts
    current_branch=$(git branch --show-current)
    if [ "$current_branch" != "zjg/v11.0.0-rc.0-op-v1" ]; then
        echo "Switching to correct branch: zjg/v11.0.0-rc.0-op-v1"
        git fetch origin
        git checkout zjg/v11.0.0-rc.0-op-v1
        git pull origin zjg/v11.0.0-rc.0-op-v1
    else
        echo "Updating existing repository..."
        git pull origin zjg/v11.0.0-rc.0-op-v1
    fi
    echo "Cleaning contract repository (selective)..."
    rm -rf artifacts cache .openzeppelin node_modules
    rm -f deployment/v2/deploy_output.json deployment/v2/create_rollup_output_*.json deployment/v2/genesis.json .env
    cd ..
else
    echo "Cloning contract repository..."
    git clone -b zjg/v11.0.0-rc.0-op-v1 https://github.com/okx/xlayer-contracts.git
fi

cd $TMP_DIR/xlayer-contracts

echo "Creating .env file..."
cat > .env << EOF
MNEMONIC="$DEPLOYER_MNEMONIC"
INFURA_PROJECT_ID="000"
ETHERSCAN_API_KEY="000"
EOF

cd deployment/v2

echo "Creating create_rollup_parameters.json..."
cat > create_rollup_parameters.json << EOF
{
    "adminZkEVM": "$DEPLOYER_ADDRESS",
    "chainID": 195,
    "consensusContract": "PolygonPessimisticConsensus",
    "dataAvailabilityProtocol": "PolygonDataCommittee",
    "deployerPvtKey": "",
    "description": "description",
    "forkID": 13,
    "gasTokenAddress":"$L1_OKB_ADDRESS",
    "maxFeePerGas": "",
    "maxPriorityFeePerGas": "",
    "multiplierGas": "",
    "networkName": "xlayer",
    "realVerifier": false,
    "trustedSequencer": "$SEQUENCER_ADDRESS",
    "trustedSequencerURL": "$L2_PP_SEQ_URL_IN_DOCKER",
    "trustedAggregator":"$AGGREGATOR_ADDRESS",
    "programVKey": "$PP_VKEY"
}
EOF

echo "Creating deploy_parameters.json..."
cat > deploy_parameters.json << EOF
{
    "admin": "$DEPLOYER_ADDRESS",
    "deployerPvtKey": "",
    "emergencyCouncilAddress": "$DEPLOYER_ADDRESS",
    "initialZkEVMDeployerOwner": "$DEPLOYER_ADDRESS",
    "maxFeePerGas": "",
    "maxPriorityFeePerGas": "",
    "minDelayTimelock": 60,
    "multiplierGas": "",
    "description": "description",
    "pendingStateTimeout": 604799,
    "polTokenAddress": "$L1_OKB_ADDRESS",
    "salt": "0x0000000000000000000000000000000000000000000000000000000000000001",
    "timelockAdminAddress": "$DEPLOYER_ADDRESS",
    "trustedSequencer": "$SEQUENCER_ADDRESS",
    "trustedSequencerURL": "$L2_PP_SEQ_URL_IN_DOCKER",
    "trustedAggregator": "$AGGREGATOR_ADDRESS",
    "trustedAggregatorTimeout": 604799,
    "forkID": 13,
    "test": true,
    "ppVKey": "$PP_VKEY",
    "ppVKeySelector": "0x00000001",
    "realVerifier": false,
    "defaultAdminAddress": "$DEPLOYER_ADDRESS",
    "aggchainDefaultVKeyRoleAddress": "$DEPLOYER_ADDRESS",
    "addRouteRoleAddress": "$DEPLOYER_ADDRESS",
    "freezeRouteRoleAddress": "$DEPLOYER_ADDRESS",
    "zkEVMDeployerAddress": "$DEPLOYER_ADDRESS"
}
EOF

echo "Compiling contracts..."
cd ../../
npm i
npm run deploy:v2:localhost

cd "$ROOT_DIR"
mkdir -p $TMP_DIR/pp-deployed
ROLLUP_OUTPUT_PATH=$(find $TMP_DIR/xlayer-contracts/deployment/v2 -name "create_rollup_output_*.json" | sort -r | head -n 1)
cp -rf $ROLLUP_OUTPUT_PATH $TMP_DIR/pp-deployed/create_rollup_output.json
cp -rf $TMP_DIR/xlayer-contracts/deployment/v2/create_rollup_parameters.json $TMP_DIR/pp-deployed/
cp -rf $TMP_DIR/xlayer-contracts/deployment/v2/deploy_parameters.json $TMP_DIR/pp-deployed/
cp -rf $TMP_DIR/xlayer-contracts/deployment/v2/deploy_output.json $TMP_DIR/pp-deployed/
cp -rf $TMP_DIR/xlayer-contracts/deployment/v2/genesis.json $TMP_DIR/pp-deployed/
ROLLUP_OUTPUT_PATH="$TMP_DIR/pp-deployed/create_rollup_output.json"
DEPLOY_OUTPUT_PATH="$TMP_DIR/pp-deployed/deploy_output.json"

# echo "Transferring ERC20 token to Sequencer..."
# cast send --legacy --from $SEQUENCER_ADDRESS --private-key $SEQUENCER_PRIVATE_KEY $TOKEN_ADDRESS "transfer(address,uint256)" $SEQUENCER_ADDRESS 1000

echo "Setting Trusted Sequencer URL..."
POE_ADDRESS=$(cat $ROLLUP_OUTPUT_PATH | grep -o '"rollupAddress": "[^"]*"' | cut -d'"' -f4)
BRIDGE_ADDRESS=$(cat $DEPLOY_OUTPUT_PATH | grep -o '"polygonZkEVMBridgeAddress": "[^"]*"' | cut -d'"' -f4)
GENESIS_VALUE=$(cat $ROLLUP_OUTPUT_PATH | grep -o '"genesis": "[^"]*"' | cut -d'"' -f4)
TIMESTAMP_VALUE=$(cat $ROLLUP_OUTPUT_PATH | grep -o '"timestamp": [0-9]*' | cut -d' ' -f2)
L1_FIRST_BLOCK=$(cat $DEPLOY_OUTPUT_PATH | grep -o '"upgradeToULxLyBlockNumber": [0-9]*' | cut -d' ' -f2)
L1_SECOND_BLOCK=$(cat $ROLLUP_OUTPUT_PATH | grep -o '"createRollupBlockNumber": [0-9]*' | cut -d' ' -f2)
ROLLUP_MANAGER_ADDRESS=$(grep -o '"polygonRollupManagerAddress": "[^"]*"' "$DEPLOY_OUTPUT_PATH" | cut -d'"' -f4)
GLOBAL_EXIT_ROOT_ADDRESS=$(grep -o '"polygonZkEVMGlobalExitRootAddress": "[^"]*"' "$DEPLOY_OUTPUT_PATH" | cut -d'"' -f4)
echo "Poe address from JSON: $POE_ADDRESS"
echo "Bridge address from JSON: $BRIDGE_ADDRESS"
echo "Genesis value from JSON: $GENESIS_VALUE"
echo "Timestamp value from JSON: $TIMESTAMP_VALUE"
echo "L1FirstBlock value from JSON: $L1_FIRST_BLOCK"
echo "L1SecondBlock value from JSON: $L1_SECOND_BLOCK"
echo "RollupManagerAddress value from JSON: $ROLLUP_MANAGER_ADDRESS"
echo "GlobalExitRootAddress value from JSON: $GLOBAL_EXIT_ROOT_ADDRESS"

echo "Using POE address from JSON: $POE_ADDRESS"
cast send --legacy --from $DEPLOYER_ADDRESS --private-key $DEPLOYER_PRIVATE_KEY $POE_ADDRESS "setTrustedSequencerURL(string)" "$L2_PP_RPC_URL_IN_DOCKER"

cast send --legacy --from $DEPLOYER_ADDRESS --private-key $DEPLOYER_PRIVATE_KEY $BRIDGE_ADDRESS 'function bridgeAsset(uint32 destinationNetwork, address destinationAddress, uint256 amount, address token, bool forceUpdateGlobalExitRoot, bytes permitData) returns()' 7 0x0000000000000000000000000000000000000000 0 0x0000000000000000000000000000000000000000 true 0x

echo "Generating configuration files..."

cd "${PWD_DIR}/tmp/xlayer-erigon"
go install ./cmd/hack/allocs
cd ${PWD_DIR}
which allocs
allocs $TMP_DIR/xlayer-contracts/deployment/v2/genesis.json
mv allocs.json $PWD_DIR/config/dynamic-mynetwork-allocs.json

cat > $PWD_DIR/config/dynamic-mynetwork-conf.json << EOF
{
  "root": "$GENESIS_VALUE",
  "timestamp": $TIMESTAMP_VALUE,
  "gasLimit": 0,
  "difficulty": 0
}
EOF
echo "dynamic-mynetwork-conf.json file updated"

echo "Updating test.erigon.seq.config.yaml file..."
ERIGON_SEQ_CONFIG_FILE="${PWD_DIR}/config/test.erigon.seq.config.yaml"
sed_inplace "s|zkevm.address-zkevm: \"[^\"]*\"|zkevm.address-zkevm: \"$POE_ADDRESS\"|g" "$ERIGON_SEQ_CONFIG_FILE"
sed_inplace "s|zkevm.address-rollup: \"[^\"]*\"|zkevm.address-rollup: \"$ROLLUP_MANAGER_ADDRESS\"|g" "$ERIGON_SEQ_CONFIG_FILE"
sed_inplace "s|zkevm.address-ger-manager: \"[^\"]*\"|zkevm.address-ger-manager: \"$GLOBAL_EXIT_ROOT_ADDRESS\"|g" "$ERIGON_SEQ_CONFIG_FILE"
sed_inplace "s|zkevm.l1-first-block: [0-9]*|zkevm.l1-first-block: $L1_FIRST_BLOCK|g" "$ERIGON_SEQ_CONFIG_FILE"

mkdir -p "$PWD_DIR/config"
jq '.firstBatchData' "$ROLLUP_OUTPUT_PATH" > "$PWD_DIR/config/first-batch-config.json"
echo "Successfully exported firstBatchData to $PWD_DIR/config/first-batch-config.json"

echo "Updating parameter in aggkit.toml..."
CONFIG_FILE="${PWD_DIR}/config/aggkit.toml"
sed_inplace "s|polygonBridgeAddr = \"[^\"]*\"|polygonBridgeAddr = \"$BRIDGE_ADDRESS\"|" "$CONFIG_FILE"
sed_inplace "s|BridgeAddr = \"[^\"]*\"|BridgeAddr = \"$BRIDGE_ADDRESS\"|" "$CONFIG_FILE"
sed_inplace "s|BridgeAddrL2 = \"[^\"]*\"|BridgeAddrL2 = \"$BRIDGE_ADDRESS\"|" "$CONFIG_FILE"
sed_inplace "s|rollupCreationBlockNumber = \"[^\"]*\"|rollupCreationBlockNumber = \"$L1_FIRST_BLOCK\"|" "$CONFIG_FILE"
sed_inplace "s|rollupManagerCreationBlockNumber = \"[^\"]*\"|rollupManagerCreationBlockNumber = \"$L1_SECOND_BLOCK\"|" "$CONFIG_FILE"
sed_inplace "s|genesisBlockNumber = \"[^\"]*\"|genesisBlockNumber = \"$L1_FIRST_BLOCK\"|" "$CONFIG_FILE"
sed_inplace "s|polygonRollupManagerAddress = \"[^\"]*\"|polygonRollupManagerAddress = \"$ROLLUP_MANAGER_ADDRESS\"|" "$CONFIG_FILE"
sed_inplace "s|polygonZkEVMGlobalExitRootAddress = \"[^\"]*\"|polygonZkEVMGlobalExitRootAddress = \"$GLOBAL_EXIT_ROOT_ADDRESS\"|" "$CONFIG_FILE"
sed_inplace "s|polygonZkEVMAddress = \"[^\"]*\"|polygonZkEVMAddress = \"$POE_ADDRESS\"|" "$CONFIG_FILE"
echo "Successfully updated contract address parameters in aggkit.toml"

echo "Updating contract address parameters in agglayer-config.toml..."
AGGLAYER_CONFIG_FILE="${PWD_DIR}/config/agglayer-config.toml"
sed_inplace "s|rollup-manager-contract = \"[^\"]*\"|rollup-manager-contract = \"$ROLLUP_MANAGER_ADDRESS\"|" "$AGGLAYER_CONFIG_FILE"
sed_inplace "s|polygon-zkevm-global-exit-root-v2-contract = \"[^\"]*\"|polygon-zkevm-global-exit-root-v2-contract = \"$GLOBAL_EXIT_ROOT_ADDRESS\"|" "$AGGLAYER_CONFIG_FILE"
GENESIS_CONFIG_FILE="${PWD_DIR}/config/test.genesis.config.json"
sed_inplace "s|\"genesisBlockNumber\": [0-9]*|\"genesisBlockNumber\": $L1_FIRST_BLOCK|" "$GENESIS_CONFIG_FILE"
sed_inplace "s|\"rollupCreationBlockNumber\": [0-9]*|\"rollupCreationBlockNumber\": $L1_SECOND_BLOCK|" "$GENESIS_CONFIG_FILE"
sed_inplace "s|\"rollupManagerCreationBlockNumber\": [0-9]*|\"rollupManagerCreationBlockNumber\": $L1_FIRST_BLOCK|" "$GENESIS_CONFIG_FILE"
AGGLAYER_CONFIG_FILE="${PWD_DIR}/config/agglayer-config.toml"
sed_inplace "s|polygon-zkevm-global-exit-root-v2-contract = \"[^\"]*\"|polygon-zkevm-global-exit-root-v2-contract = \"$GLOBAL_EXIT_ROOT_ADDRESS\"|" "$AGGLAYER_CONFIG_FILE"

echo "Initialization script completed!"