# Signet Launch

This folder contains a software upgrade for testing purposes.
DO NOT USE IN PRODUCTION!

## Compile signet launch upgrade

This upgrade has two data points loaded from strings. BTC Headers from
`./data_btc_headers.go` and signed messages to create finality providers
`./data_signed_fps.go`.

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
btcBlockTipHeight=1000
EXPORT_TO="./btc-headers.json"
# export the btc headers to a file
$SID_BIN btc-headers 1 $btcBlockTipHeight --output $EXPORT_TO
btcHeadersJson=$(cat $EXPORT_TO)

# writes the headers to babylon as go file
echo "package signetlaunch

const NewBtcHeadersStr = \`$btcHeadersJson\`" > $GO_BTC_HEADERS_PATH
```

### Signed Create
