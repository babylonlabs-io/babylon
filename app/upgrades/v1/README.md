# Upgrade V1

Babylon launched as Phase-1 without a cosmos chain running
to collect BTC staking prior to decentralize the finality provider
set of operators. The first upgrade of Babylon chain to start
receiving BTC delegations will include the BTC headers created
during Phase-1 and upgrade, finality providers registered in the
dashboard, tokens distribution for the active users and operators
that participated and need to finish their actions and update of
parameters for `x/finality` and `x/btcstaking` modules.

## Testnet vs Mainnet

Babylon upgrade data will be different for mainnet and testnet,
finality providers should not use the same keys for mainnet and testnet.
So to register himself and test, the finality providers will use two
different registrations one for mainnet and another for testnet. The
BTC Headers also are different as the Bitcoin mainnet and signet produces
different block headers. So, the upgrade data will be divided into 2
`app/upgrades/v1`:

- `app/upgrades/v1/mainnet` contains the files with JSON string for mainnet.
- `app/upgrades/v1/testnet` contains the files with JSON string for testnet.

## Devnets

Devnets that are only for internal testing should just replace the upgrade
data files in testnet and build the binary with `make build-testnet`. No need
to push the devenet data into the github repository.

## Upgrade data as string

This upgrade loads 5 JSONs from strings in different files.

- BTC Headers at `./data_btc_headers.go`
- Finality Providers signed messages at`./data_signed_fps.go`
- Tokens distribution at `./data_token_distribution.go`
- BTC Staking Parameters `./btcstaking_params.go`
- Finality Parameters `./finality_params.go`

### BTC Headers

This upgrade accepts insertion of multiple
[`btclighttypes.BTCHeaderInfo`](../../../x/btclightclient/types/btclightclient.pb.go#36)
due to Babylon Phase-1 and Phase-2 launch will be a few months apart, so
during Phase-1 Babylon accepts BTC delegations without Babylonchain running.
At the time of launching the Babylonchain it is needed all the BTC block
headers that has passed since babylon started to accept BTC staking messages,
and to avoid giving too much work for
[vigilante](https://github.com/babylonlabs-io/vigilante)
to submit all of those missing headers.

To generate this BTC headers there is a specific command in
[staking-indexer](https://github.com/babylonlabs-io/staking-indexer)
that query BTC for all the BTC headers and outputs it as json file
`sid btc-headers [from-block-height] [to-block-height]` and then
it is needed to recreate the golang file `./data_btc_headers.go`
with some simple bash script:

```shell
GO_BTC_HEADERS_PATH="signetlaunch/data_btc_headers.go"
EXPORT_TO="./btc-headers.json"
# export the btc headers to a file
$SID_BIN btc-headers 1 1000 --output $EXPORT_TO
btcHeadersJson=$(cat $EXPORT_TO)

# writes the headers to babylon as go file
echo "package signetlaunch

const NewBtcHeadersStr = \`$btcHeadersJson\`" > $GO_BTC_HEADERS_PATH
```

### Signed Create Finality Provider

For BTC stakers to stake during Phase-1 it is needed to have finality
providers. Babylon created a repository to publicly store this information
inside [networks](https://github.com/babylonlabs-io/networks) repository.
Inside the bbn-1 mainnet all the finality providers that wanted to be available
for BTC staking since the beginning would need to
[register](https://github.com/babylonlabs-io/networks/blob/main/bbn-1/finality-providers/README.md)
theirselves in the registry.
For the transition from Phase-1 to Phase-2, registered finality providers in
Phase-1 will need to provider a signed
[MsgCreateFinalityProvider](../../../x/btcstaking/types/tx.pb.go#38) as a
json file message inside the networks repository registry.

### Tokens distribution

During the upgrade, some tokens will be distributed so users and operators can
finish their actions, by example:

- BTC stakers to finalize their BTC delegation
- Finality providers to submit pub rand and finality
- New Cosmos-SDK validators to decentralize after the upgrade
- Vigilantes
- Covenant Emulators

> This data for token distribution will be built accordingly with the
data collected during Phase-1.

## Building with Upgrade

Upgrade plan is included based on the build tags.
By default the mainnet data is included with the upgrade plan,
so running `make build` already adds the mainnet build tag and
includes the upgrade plan with the mainnet data. If `make build-testnet`
is run, it includes the `testnet` build tag and only includes the
data for testnet in the upgrade plan.
