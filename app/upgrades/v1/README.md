# Signet Launch

This folder contains a software upgrade for testing purposes.
DO NOT USE IN PRODUCTION!

## Compile signet launch upgrade

This upgrade loads 5 JSONs from strings in different files.

- BTC Headers at `./data_btc_headers.go`
- Finality Providers signed messages at`./data_signed_fps.go`
- Tokens distribution at `./data_token_distribution.go`
- BTC Staking Parameters `./btcstaking_params.go`
- Finality Parameters `./finality_params.go`

### BTC Headers

This upgrade accepts insertion of multiple
[`btclighttypes.BTCHeaderInfo`](../../../x/btclightclient/types/btclightclient.pb.go#36)
due to Babylon Phase-1 and Phase-2 launch will be a few months appart, so
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
