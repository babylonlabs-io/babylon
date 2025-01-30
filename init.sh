#!/bin/bash

CHAINID="test_6901-1"
BASE_DENOM="ubbn"
MONIKER="test"
KEYRING="test"
KEY="dev0"
LOGLEVEL="info"
BTC_NETWORK="regtest"
HOMEDIR="./.testnet/node0/babylond"

# validate dependencies are installed
command -v jq > /dev/null 2>&1 || { echo >&2 "jq not installed. More info: https://stedolan.github.io/jq/download/"; exit 1; }

# Build the binary
echo "Building babylond..."
make install && make build

# Clean up any previous data
rm -rf ./.testnet

# Create testnet
echo "Creating Babylon EVM enabled testnet..."
./build/babylond testnet \
    --v 1 \
    --output-dir ./.testnet \
    --starting-ip-address 127.0.0.1 \
    --keyring-backend $KEYRING \
    --chain-id $CHAINID \
    --minimum-gas-prices "0.0001ubbn" \
    --btc-network $BTC_NETWORK \
    --node-daemon-home babylond \
    --node-dir-prefix "node" \
    --genesis-time $(date +%s)

# Configure genesis parameters
GENESIS_FILE="./.testnet/node0/babylond/config/genesis.json"

# Update EVM configuration in genesis
jq '.app_state["evm"] = {
  "accounts": [],
  "params": {
    "evm_denom": "ubbn",
    "enable_create": true,
    "enable_call": true,
    "extra_eips": [],
    "chain_config": {
      "homestead_block": "0",
      "dao_fork_block": "0",
      "dao_fork_support": true,
      "eip150_block": "0",
      "eip150_hash": "0x0000000000000000000000000000000000000000000000000000000000000000",
      "eip155_block": "0",
      "eip158_block": "0",
      "byzantium_block": "0",
      "constantinople_block": "0",
      "petersburg_block": "0",
      "istanbul_block": "0",
      "muir_glacier_block": "0",
      "berlin_block": "0",
      "london_block": "0",
      "arrow_glacier_block": "0",
      "gray_glacier_block": "0",
      "merge_netsplit_block": "0",
      "shanghai_time": "0",
      "cancun_time": "0",
      "prague_time": "0"
    },
    "allow_unprotected_txs": false
  }
}' "$GENESIS_FILE" > temp.json && mv temp.json "$GENESIS_FILE"

# Configure JSON-RPC and API settings
if [[ "$OSTYPE" == "darwin"* ]]; then
    # macOS specific sed commands
    sed -i '' 's/enable = false/enable = true/' ./.testnet/node0/babylond/config/app.toml
    sed -i '' 's/api = "eth,net,web3"/api = "eth,net,web3,txpool"/' ./.testnet/node0/babylond/config/app.toml
    sed -i '' 's/enabled-unsafe-cors = false/enabled-unsafe-cors = true/' ./.testnet/node0/babylond/config/app.toml
else
    # Linux specific sed commands
    sed -i 's/enable = false/enable = true/' ./.testnet/node0/babylond/config/app.toml
    sed -i 's/api = "eth,net,web3"/api = "eth,net,web3,txpool"/' ./.testnet/node0/babylond/config/app.toml
    sed -i 's/enabled-unsafe-cors = false/enabled-unsafe-cors = true/' ./.testnet/node0/babylond/config/app.toml
fi

# Start the node
echo "Starting node..."
./build/babylond start \
    --home ./.testnet/node0/babylond \
    --log_level=$LOGLEVEL \
    --minimum-gas-prices "0.0001ubbn" \
    --json-rpc.enable=true \
    --json-rpc.api="eth,txpool,personal,net,debug,web3,miner" \
    --json-rpc.address="0.0.0.0:8545" \
    --grpc.enable=true \
    --api.enable=true \
    --api.enabled-unsafe-cors=true