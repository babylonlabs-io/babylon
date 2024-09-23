#!/bin/sh

# Create new directory that will hold node and services' configuration
mkdir -p ibcsim-bcd/.testnets && chmod -R 777 ibcsim-bcd/.testnets
echo "Creating and configuring testnet directory..."
docker run --rm -v $(pwd)/ibcsim-bcd/.testnets:/data babylonlabs-io/babylond \
    babylond testnet init-files --v 2 -o /data \
    --starting-ip-address 192.168.10.2 --keyring-backend=test \
    --chain-id chain-test --epoch-interval 10 \
    --btc-finalization-timeout 2 --btc-confirmation-depth 1 \
    --minimum-gas-prices 0.000006ubbn \
    --btc-base-header 0100000000000000000000000000000000000000000000000000000000000000000000003ba3edfd7a7b12b27ac72c3e67768f617fc81bc3888a51323a9fb8aa4b1e5e4adae5494dffff7f2002000000 \
    --btc-network regtest --additional-sender-account \
    --slashing-pk-script "76a914010101010101010101010101010101010101010188ab" \
    --slashing-rate 0.1 \
    --min-commission-rate 0.05 \
    --covenant-quorum 1 \
    --covenant-pks "bb50e2d89a4ed70663d080659fe0ad4b9bc3e06c17a227433966cb59ceee020d" # should be updated if `covenant-keyring` dir is changed`

# Create separate subpaths for each component and copy relevant configuration
chmod -R 777 ibcsim-bcd/.testnets
echo "Testnet directory created and configured successfully."
