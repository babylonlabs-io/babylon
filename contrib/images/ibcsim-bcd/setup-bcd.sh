#!/bin/bash

display_usage() {
	echo "Missing parameters. Please check if all parameters were specified."
	echo "Usage: setup-bcd.sh [CHAIN_ID] [CHAIN_DIR] [RPC_PORT] [P2P_PORT] [PROFILING_PORT] [GRPC_PORT] [BABYLON_CONTRACT_CODE_FILE] [BTCSTAKING_CONTRACT_CODE_FILE] [BTCFINALITY_CONTRACT_CODE_FILE] [INSTANTIATING_CFG]"
	echo "Example: setup-bcd.sh test-chain-id ./data 26657 26656 6060 9090 ./babylon_contract.wasm '{"btc_confirmation_depth":1,"checkpoint_finalization_timeout":2,"network":"Regtest","babylon_tag":"bbn0", "notify_cosmos_zone":false, "btc_staking_code_id":2}'"
	exit 1
}

BINARY=bcd
DENOM=stake
BASEDENOM=ustake
KEYRING=--keyring-backend="test"
SILENT=1

redirect() {
	if [ "$SILENT" -eq 1 ]; then
		"$@" >/dev/null 2>&1
	else
		"$@"
	fi
}

if [ "$#" -lt "9" ]; then
	display_usage
	exit 1
fi

CHAINID=$1
CHAINDIR=$2
RPCPORT=$3
P2PPORT=$4
PROFPORT=$5
GRPCPORT=$6
BABYLON_CONTRACT_CODE_FILE=$7
BTCSTAKING_CONTRACT_CODE_FILE=$8
BTCFINALITY_CONTRACT_CODE_FILE=$9
INSTANTIATING_CFG=${10}

# ensure the binary exists
if ! command -v $BINARY &>/dev/null; then
	echo "$BINARY could not be found"
	exit
fi

# Delete chain data from old runs
echo "Deleting $CHAINDIR/$CHAINID folders..."
rm -rf $CHAINDIR/$CHAINID &>/dev/null
rm $CHAINDIR/$CHAINID.log &>/dev/null

echo "Creating $BINARY instance: home=$CHAINDIR | chain-id=$CHAINID | p2p=:$P2PPORT | rpc=:$RPCPORT | profiling=:$PROFPORT | grpc=:$GRPCPORT"

# Add dir for chain, exit if error
if ! mkdir -p $CHAINDIR/$CHAINID 2>/dev/null; then
	echo "Failed to create chain folder. Aborting..."
	exit 1
fi
# Build genesis file incl account for passed address
coins="100000000000$DENOM,100000000000$BASEDENOM"
delegate="100000000000$DENOM"

redirect $BINARY --home $CHAINDIR/$CHAINID --chain-id $CHAINID init $CHAINID
sleep 1
$BINARY --home $CHAINDIR/$CHAINID keys add validator $KEYRING --output json > $CHAINDIR/$CHAINID/validator_seed.json 2>&1
sleep 1
$BINARY --home $CHAINDIR/$CHAINID keys add user $KEYRING --output json > $CHAINDIR/$CHAINID/key_seed.json 2>&1
sleep 1
redirect $BINARY --home $CHAINDIR/$CHAINID genesis add-genesis-account $($BINARY --home $CHAINDIR/$CHAINID keys $KEYRING show user -a) $coins
sleep 1
redirect $BINARY --home $CHAINDIR/$CHAINID genesis add-genesis-account $($BINARY --home $CHAINDIR/$CHAINID keys $KEYRING show validator -a) $coins
sleep 1
redirect $BINARY --home $CHAINDIR/$CHAINID genesis gentx validator $delegate $KEYRING --chain-id $CHAINID
sleep 1
redirect $BINARY --home $CHAINDIR/$CHAINID genesis collect-gentxs
sleep 1

# Set proper defaults and change ports
echo "Change settings in config.toml and genesis.json files..."
sed -i 's#"tcp://127.0.0.1:26657"#"tcp://0.0.0.0:'"$RPCPORT"'"#g' $CHAINDIR/$CHAINID/config/config.toml
sed -i 's#"tcp://0.0.0.0:26656"#"tcp://0.0.0.0:'"$P2PPORT"'"#g' $CHAINDIR/$CHAINID/config/config.toml
sed -i 's#"localhost:6060"#"localhost:'"$PROFPORT"'"#g' $CHAINDIR/$CHAINID/config/config.toml
sed -i 's/timeout_commit = "5s"/timeout_commit = "1s"/g' $CHAINDIR/$CHAINID/config/config.toml
sed -i 's/max_body_bytes = 1000000/max_body_bytes = 1000000000/g' $CHAINDIR/$CHAINID/config/config.toml
sed -i 's/minimum-gas-prices = ""/minimum-gas-prices = "0.00001ustake"/g' $CHAINDIR/$CHAINID/config/app.toml
sed -i 's/timeout_propose = "3s"/timeout_propose = "1s"/g' $CHAINDIR/$CHAINID/config/config.toml
sed -i 's/index_all_keys = false/index_all_keys = true/g' $CHAINDIR/$CHAINID/config/config.toml
sed -i 's#"tcp://0.0.0.0:1317"#"tcp://0.0.0.0:1318"#g' $CHAINDIR/$CHAINID/config/app.toml # ensure port is not conflicted with Babylon
sed -i 's/"bond_denom": "stake"/"bond_denom": "'"$DENOM"'"/g' $CHAINDIR/$CHAINID/config/genesis.json
# sed -i '' 's#index-events = \[\]#index-events = \["message.action","send_packet.packet_src_channel","send_packet.packet_sequence"\]#g' $CHAINDIR/$CHAINID/config/app.toml

## Script for getting contract addresses
## TODO(euphrates): pass a gov prop on setting the Babylon / BTC staking contract addresses
# babylonContractAddr=$(bcd query wasm list-contract-by-code 1 -o json | jq -r '.contracts[0]')
# btcStakingContractAddr=$(bcd query wasm list-contract-by-code 2 -o json | jq -r '.contracts[0]')
# echo "babylonContractAddr is $babylonContractAddr"
# echo "btcStakingContractAddr is $btcStakingContractAddr"

# update contract address in genesis
babylonContractAddr=bbnc14hj2tavq8fpesdwxxcu44rty3hh90vhujrvcmstl4zr3txmfvw9syx25zf
btcStakingContractAddr=bbnc1nc5tatafv6eyq7llkr2gv50ff9e22mnf70qgjlv737ktmt4eswrqgn0kq0
btcFinalityContractAddr=bbnc17p9rzwnnfxcjp32un9ug7yhhzgtkhvl9jfksztgw5uh69wac2pgssg3nft
sed -i 's/"babylon_contract_address": ""/"babylon_contract_address": "'"$babylonContractAddr"'"/g' $CHAINDIR/$CHAINID/config/genesis.json
sed -i 's/"btc_staking_contract_address": ""/"btc_staking_contract_address": "'"$btcStakingContractAddr"'"/g' $CHAINDIR/$CHAINID/config/genesis.json
sed -i 's/"btc_finality_contract_address": ""/"btc_finality_contract_address": "'"$btcFinalityContractAddr"'"/g' $CHAINDIR/$CHAINID/config/genesis.json

# Start
echo "Starting $BINARY..."
$BINARY --home $CHAINDIR/$CHAINID start --pruning=nothing --grpc-web.enable=false --grpc.address="0.0.0.0:$GRPCPORT" --log_level trace --trace --log_format 'plain' 2>&1 | tee $CHAINDIR/$CHAINID.log &
sleep 20

# upload contract code
echo "Uploading babylon contract code $BABYLON_CONTRACT_CODE_FILE..."
$BINARY --home $CHAINDIR/$CHAINID tx wasm store "$BABYLON_CONTRACT_CODE_FILE" $KEYRING --from user --chain-id $CHAINID --gas 20000000000 --gas-prices 0.01ustake --node http://localhost:$RPCPORT -y
sleep 10

# upload contract code
echo "Uploading btcstaking contract code $BTCSTAKING_CONTRACT_CODE_FILE..."
$BINARY --home $CHAINDIR/$CHAINID tx wasm store "$BTCSTAKING_CONTRACT_CODE_FILE" $KEYRING --from user --chain-id $CHAINID --gas 20000000000 --gas-prices 0.01ustake --node http://localhost:$RPCPORT -y
sleep 10

# upload contract code
echo "Uploading btcfinality contract code $BTCFINALITY_CONTRACT_CODE_FILE..."
$BINARY --home $CHAINDIR/$CHAINID tx wasm store "$BTCFINALITY_CONTRACT_CODE_FILE" $KEYRING --from user --chain-id $CHAINID --gas 20000000000 --gas-prices 0.01ustake --node http://localhost:$RPCPORT -y
sleep 10

# Echo the command with expanded variables
echo "Instantiating contract $BABYLON_CONTRACT_CODE_FILE..."
$BINARY --home $CHAINDIR/$CHAINID tx wasm instantiate 1 "$INSTANTIATING_CFG" --admin=$(bcd --home $CHAINDIR/$CHAINID keys show user --keyring-backend test -a) --label "v0.0.1" $KEYRING --from user --chain-id $CHAINID --gas 20000000000 --gas-prices 0.001ustake --node http://localhost:$RPCPORT -y --amount 100000stake
