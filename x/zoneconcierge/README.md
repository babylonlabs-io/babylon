# ZoneConcierge

The Zone Concierge module is responsible for providing BTC staking integration
functionalities for other Bitcoin Supercharged Networks (BSNs).  
It leverages the IBC protocol to receive consumer chains' headers, and propagate BTC timestamps of those headers
and information associated with the BTC staking protocol (e.g., finality providers, BTC stakes, and more).  
The Zone Concierge module synchronises the following information with consumer
chains via IBC packets:

- **BTC Headers:** Babylon Genesis forwards BTC headers to consumer chains to keep their
  BTC light clients in sync with Babylon's BTC light client. This allows
  consumer chains to independently verify BTC timestamps.
- **BTC Timestamps:** When a Babylon epoch is finalised, Babylon sends BTC
  timestamps to consumer chains. Each BTC timestamp contains:
  - The latest consumer chain header that was checkpointed in the finalised
    epoch
  - Recent BTC headers that extend the consumer's BTC light client
  - The finalised epoch's metadata and raw checkpoint
  - Proofs that the consumer header was included in the epoch and the epoch was
    properly sealed and submitted to Bitcoin
- **BTC Staking:** Babylon enables trustless Bitcoin staking for consumer chains
  by synchronising staking-related information between Bitcoin, Babylon and
  consumer chains. This allows BTC holders to stake their BTC to secure consumer
  chains without requiring any custodial solutions.

## Table of contents

- [ZoneConcierge](#zoneconcierge)
  - [Table of contents](#table-of-contents)
  - [State](#state)
    - [Parameters](#parameters)
    - [ChainInfo](#chaininfo)
    - [EpochChainInfo](#epochchaininfo)
    - [CanonicalChain](#canonicalchain)
    - [Fork](#fork)
    - [Params](#params)
  - [PostHandler for intercepting IBC headers](#posthandler-for-intercepting-ibc-headers)
  - [Hooks](#hooks)
    - [Indexing headers upon `AfterEpochEnds`](#indexing-headers-upon-afterepochends)
    - [Sending BTC timestamps upon `AfterRawCheckpointFinalized`](#sending-btc-timestamps-upon-afterrawcheckpointfinalized)
  - [Messages and Queries](#messages-and-queries)
  - [Consumer Chain Integration](#consumer-chain-integration)
    - [IBC Communication Protocol](#ibc-communication-protocol)
    - [Relaying BTC Headers](#relaying-btc-headers)
      - [Broadcasting headers](#broadcasting-headers)
      - [Selecting headers to be broadcast](#selecting-headers-to-be-broadcast)
    - [Relaying BTC Timestamps](#relaying-btc-timestamps)
      - [Triggering timestamp relay](#triggering-timestamp-relay)
      - [Broadcasting timestamps](#broadcasting-timestamps)
      - [Assembling timestamps](#assembling-timestamps)
      - [Cryptographic Proofs](#cryptographic-proofs)
    - [Propagating BTC Staking Events](#propagating-btc-staking-events)
      - [Broadcasting staking events](#broadcasting-staking-events)
      - [Processing event flow](#processing-event-flow)
      - [Registering consumers](#registering-consumers)
      - [Handling slashing](#handling-slashing)

<!-- TODO: concept section for describing BTC staking integration -->

## State

The Zone Concierge module keeps handling IBC headers of PoS blockchains, and
maintains the following KV stores.

### Parameters

The [parameter storage](./keeper/params.go) maintains the Zone Concierge
module's parameters. The Zone Concierge module's parameters are represented as a
`Params` [object](../../proto/babylon/zoneconcierge/v1/params.proto) defined as
follows:

```protobuf
// Params defines the parameters for the module.
message Params {
  option (gogoproto.equal) = true;
  
  // ibc_packet_timeout_seconds is the time period after which an unrelayed 
  // IBC packet becomes timeout, measured in seconds
  uint32 ibc_packet_timeout_seconds = 1
      [ (gogoproto.moretags) = "yaml:\"ibc_packet_timeout_seconds\"" ];
}
```

### ChainInfo

The [chain info storage](./keeper/chain_info_indexer.go) maintains `ChainInfo`
for each PoS blockchain. The key is the PoS blockchain's `ConsumerID`, which is
the ID of the IBC light client. The value is a `ChainInfo` object. The
`ChainInfo` is a structure storing the information of a PoS blockchain that
checkpoints to Babylon.

```protobuf
// ChainInfo is the information of a Consumer
message ChainInfo {
  // consumer_id is the ID of the consumer
  string consumer_id = 1;
  // latest_header is the latest header in the Consumer's canonical chain
  IndexedHeader latest_header = 2;
  // latest_forks is the latest forks, formed as a series of IndexedHeader (from
  // low to high)
  Forks latest_forks = 3;
  // timestamped_headers_count is the number of timestamped headers in the Consumer's
  // canonical chain
  uint64 timestamped_headers_count = 4;
}
```

### EpochChainInfo

The [epoch chain info storage](./keeper/epoch_chain_info_indexer.go) maintains
`ChainInfo` at the end of each Babylon epoch for each PoS blockchain. The key is
the PoS blockchain's `ConsumerID` plus the epoch number, and the value is a
`ChainInfo` object.

### CanonicalChain

The [canonical chain storage](./keeper/canonical_chain_indexer.go) maintains the
metadata of canonical IBC headers of a PoS blockchain. The key is the consumer
chain's `ConsumerID` plus the height, and the value is a `IndexedHeader` object.
`IndexedHeader` is a structure storing IBC header's metadata.

```protobuf
// IndexedHeader is the metadata of a Consumer header
message IndexedHeader {
  // consumer_id is the unique ID of the consumer
  string consumer_id = 1;
  // hash is the hash of this header
  bytes hash = 2;
  // height is the height of this header on the Consumer's ledger
  // (hash, height) jointly provides the position of the header on the Consumer's ledger
  uint64 height = 3;
  // time is the timestamp of this header on the Consumer's ledger.
  // It is needed for the Consumer to unbond all mature validators/delegations
  // before this timestamp when this header is BTC-finalised
  google.protobuf.Timestamp time = 4 [ (gogoproto.stdtime) = true ];
  // babylon_header_hash is the hash of the babylon block that includes this
  // Consumer header
  bytes babylon_header_hash = 5;
  // babylon_header_height is the height of the babylon block that includes this
  // Consumer header
  uint64 babylon_header_height = 6;
  // epoch is the epoch number of this header on Babylon ledger
  uint64 babylon_epoch = 7;
  // babylon_tx_hash is the hash of the tx that includes this header
  // (babylon_block_height, babylon_tx_hash) jointly provides the position of
  // the header on Babylon ledger
  bytes babylon_tx_hash = 8;
}
```

### Fork

The [fork storage](./keeper/fork_indexer.go) maintains the metadata of canonical
IBC headers of a PoS blockchain. The key is the PoS blockchain's `ConsumerID`
plus the height, and the value is a list of `IndexedHeader` objects, which
represent fork headers at that height.

### Params

The [parameter storage](./keeper/params.go) maintains the parameters for the
Zone Concierge module.

```protobuf
// Params defines the parameters for the module.
message Params {
  option (gogoproto.equal) = true;
  
  // ibc_packet_timeout_seconds is the time period after which an unrelayed 
  // IBC packet becomes timeout, measured in seconds
  uint32 ibc_packet_timeout_seconds = 1
      [ (gogoproto.moretags) = "yaml:\"ibc_packet_timeout_seconds\"" ];
}
```

## PostHandler for intercepting IBC headers

The Zone Concierge module implements a
[PostHandler](https://docs.cosmos.network/v0.50/learn/advanced/baseapp#runtx-antehandler-runmsgs-posthandler)
`IBCHeaderDecorator` to intercept headers sent to the [IBC client
module](https://github.com/cosmos/ibc-go/tree/v8.0.0/modules/core/02-client).
The `IBCHeaderDecorator` PostHandler is defined at
[x/zoneconcierge/keeper/header_handler.go](./keeper/header_handler.go), and
works as follows.

1. If the PoS blockchain hosting the header is not known to Babylon, initialize
   `ChainInfo` storage for the PoS blockchain.
2. If the header is on a fork, insert the header to the fork storage and update
   `ChainInfo`.
3. If the header is canonical, insert the header to the canonical chain storage
   and update `ChainInfo`.

## Hooks

The Zone Concierge module subscribes to the Epoching module's `AfterEpochEnds`
[hook](../epoching/types/hooks.go) for indexing the epochs when receiving
headers from consumer chains, and the Checkpointing module's
`AfterRawCheckpointFinalized` [hook](../checkpointing/types/hooks.go) for
sending BTC timestamps to consumer chains.

### Indexing headers upon `AfterEpochEnds`

The `AfterEpochEnds` hook is triggered when an epoch is ended, i.e., the last
block in this epoch has been committed by CometBFT. Upon `AfterEpochEnds`, the
Zone Concierge will save the current `ChainInfo` to the `EpochChainInfo` storage
for each consumer chain.

### Sending BTC timestamps upon `AfterRawCheckpointFinalized`

The `AfterRawCheckpointFinalized` hook is triggered upon a checkpoint becoming
*finalised*, i.e., Bitcoin transactions of the checkpoint become `w`-deep in
Bitcoin's canonical chain, where `w` is the `checkpoint_finalization_timeout`
[parameter](../../proto/babylon/btccheckpoint/v1/params.proto) in the
[BTCCheckpoint](../btccheckpoint/) module.

Upon `AfterRawCheckpointFinalized`, the Zone Concierge module will prepare and
send a BTC timestamp to each consumer chain.  
The [BTCTimestamp](../../proto/babylon/zoneconcierge/v1/packet.proto) structure  
includes a header and a set of proofs that the header is checkpointed by
Bitcoin.

<!-- TODO: diagram depicting BTC timestamp -->

```protobuf
// BTCTimestamp is a BTC timestamp that carries information of a BTC-finalized epoch
// It includes a number of BTC headers, a raw checkpoint, an epoch metadata, and 
// a Consumer header if there exists Consumer headers checkpointed to this epoch.
// Upon a newly finalized epoch in Babylon, Babylon will send a BTC timestamp to each
// PoS blockchain that has phase-2 integration with Babylon via IBC.
message BTCTimestamp {
  // header is the last Consumer header in the finalized Babylon epoch
  babylon.zoneconcierge.v1.IndexedHeader header = 1;

  /*
    Data for BTC light client
  */
  // btc_headers is BTC headers between
  // - the block AFTER the common ancestor of BTC tip at epoch `lastFinalizedEpoch-1` and BTC tip at epoch `lastFinalizedEpoch`
  // - BTC tip at epoch `lastFinalizedEpoch`
  // where `lastFinalizedEpoch` is the last finalized epoch in Babylon
  repeated babylon.btclightclient.v1.BTCHeaderInfo btc_headers = 2;

  /*
    Data for Babylon epoch chain
  */
  // epoch_info is the metadata of the sealed epoch
  babylon.epoching.v1.Epoch epoch_info = 3;
  // raw_checkpoint is the raw checkpoint that seals this epoch
  babylon.checkpointing.v1.RawCheckpoint raw_checkpoint = 4;
  // btc_submission_key is position of two BTC txs that include the raw checkpoint of this epoch
  babylon.btccheckpoint.v1.SubmissionKey btc_submission_key = 5;

  /* 
    Proofs that the header is finalized
  */
  babylon.zoneconcierge.v1.ProofFinalizedChainInfo proof = 6;
}

// ProofFinalizedChainInfo is a set of proofs that attest a chain info is
// BTC-finalized
message ProofFinalizedChainInfo {
  /*
    The following fields include proofs that attest the chain info is
    BTC-finalized
  */
  // proof_consumer_header_in_epoch is the proof that the Consumer header is timestamped
  // within a certain epoch
  tendermint.crypto.ProofOps proof_consumer_header_in_epoch = 1;
  // proof_epoch_sealed is the proof that the epoch is sealed
  babylon.zoneconcierge.v1.ProofEpochSealed proof_epoch_sealed = 2;
  // proof_epoch_submitted is the proof that the epoch's checkpoint is included
  // in BTC ledger It is the two TransactionInfo in the best (i.e., earliest)
  // checkpoint submission
  repeated babylon.btccheckpoint.v1.TransactionInfo proof_epoch_submitted = 3;
}
```

When `AfterRawCheckpointFinalized` is triggered, the Zone Concierge module will
send an IBC packet including a `BTCTimestamp` to each consumer chain. The logic
is defined at [x/zoneconcierge/keeper/hooks.go](./keeper/hooks.go) and works as
follows:

1. Find all open IBC channels with Babylon's Zone Concierge module. The
   counterparty at each IBC channel is a PoS blockchain.
2. Get all BTC headers to be sent in BTC timestamps. Specifically,
   1. Find the segment of BTC headers sent upon the last time
      `AfterRawCheckpointFinalized` is triggered.
   2. If all BTC headers in the segment are no longer canonical, the BTC headers
      to be sent will be the last `w+1` ones in the BTC light client, where `w`
      is the `checkpoint_finalization_timeout`
      [parameter](../../proto/babylon/btccheckpoint/v1/params.proto) in the
      [BTCCheckpoint](../btccheckpoint/) module.
   3. Otherwise, the BTC headers to be sent will be from the latest header that
      is still canonical in the segment to the current tip of the BTC light
      client.
3. For each of these IBC channels:
   1. Find the `ConsumerID` of the counterparty chain (i.e., the PoS blockchain)
      in the IBC channel.
   2. Get the `ChainInfo` of the `ConsumerID` at the last finalised epoch.
   3. Get the metadata of the last finalised epoch and its corresponding raw
      checkpoint.
   4. Generate the proof that the last PoS blockchain's canonical header is
      committed to the epoch's metadata.
        5. Generate the proof that the epoch is sealed, i.e., receives a BLS
        multisignature generated by validators with >2/3 total voting power at
        the last finalised epoch.
   6. Generate the proof that the epoch's checkpoint is submitted, i.e., encoded
      in transactions on Bitcoin.
   7. Assemble all the above and the BTC headers obtained in step 2 as
      `BTCTimestamp`, and send it to the IBC channel in an IBC packet.


## Messages and Queries

The Zone Concierge module only has one message `MsgUpdateParams` for updating
the module parameters via a governance proposal.

It provides a set of queries about the status of checkpointed PoS blockchains,
listed at
[docs.babylonlabs.io](https://docs.babylonlabs.io/docs/developer-guides/grpcrestapi#tag/ZoneConcierge).

## Consumer Chain Integration

The Zone Concierge module connects Babylon and consumer chains, relaying three
types of information through IBC: BTC headers, BTC timestamps, and BTC staking
events.

### IBC Communication Protocol

Channel Configuration:
- Port: `zoneconcierge`
- Ordering: `ORDERED`
- Version: `zoneconcierge-1`

Packet Types:
- Outbound: `BTCHeaders`, `BTCTimestamp`, `BTCStakingConsumerEvent`
- Inbound: `ConsumerSlashingIBCPacket`

### Relaying BTC Headers

Zone Concierge relays BTC headers from Babylon's BTC light client to consumer
chains to keep their BTC light clients synchronised with Bitcoin's canonical
chain.

#### Broadcasting headers

The `BroadcastBTCHeaders` function broadcasts BTC headers to all open IBC
channels.

#### Selecting headers to be broadcast

- If no headers have been sent previously: Send the last `w+1` BTC headers from
  the tip, where `w` is the checkpoint finalisation timeout
- If headers have been sent previously:
  - If the last sent segment is still valid (no Bitcoin reorg): Send headers
    from the last sent header to the current tip
  - If the last sent segment is invalid (Bitcoin reorg occurred): Send the last
    `w+1` headers from the current tip

### Relaying BTC Timestamps

Zone Concierge sends BTC timestamps to consumer chains when a Babylon epoch
becomes BTC-finalised.

#### Triggering timestamp relay

The `AfterRawCheckpointFinalized` hook is called when an epoch's checkpoint
becomes `w`-deep in Bitcoin's canonical chain.

#### Broadcasting timestamps

The `BroadcastBTCTimestamps` function creates and sends BTC timestamps to all
open IBC channels.

#### Assembling timestamps

1. `getFinalizedInfo` collects shared finalisation data:
   - Epoch metadata and raw checkpoint
   - BTC submission key and proofs
   - Proof that the epoch was sealed by validators
   - Proof that the epoch's checkpoint was submitted to Bitcoin
2. `createBTCTimestamp` constructs individual timestamps for each consumer:
   - If the channel is uninitialized: Include Bitcoin headers from tip to
     `(w+1+len(headersToBroadcast))`-deep
   - If the channel is initialized: Include only the headers from
     `headersToBroadcast`
   - If the consumer has a header checkpointed in the finalised epoch: Include
     the consumer header and proof

#### Cryptographic Proofs

- `ProofEpochSealed`: Proves >2/3 validators signed the epoch
- `ProofEpochSubmitted`: Proves the checkpoint was submitted to Bitcoin
- `ProofConsumerHeaderInEpoch`: Proves the consumer header was timestamped in
  the epoch (if applicable)

### Propagating BTC Staking Events

Zone Concierge propagates BTC staking events from Babylon to consumer chains to
enable trustless BTC staking.

#### Broadcasting staking events

The `BroadcastBTCStakingConsumerEvents` function sends staking events to
relevant consumer chains.

#### Processing event flow

1. Retrieve all pending consumer events from the BTC staking module via
   `GetAllBTCStakingConsumerIBCPackets`
2. Map consumer IDs to their corresponding open IBC channels
3. Send each consumer's events to all their open channels
4. Delete sent events from the store via `DeleteBTCStakingConsumerIBCPacket`

#### Registering consumers

- `HandleIBCChannelCreation` validates consumer registration during IBC
  handshake
- Consumer must be registered in the BTC staking module with a valid
  `ConsumerRegister`
- Channel ID is stored in the consumer's metadata upon successful handshake

#### Handling slashing

- `HandleConsumerSlashing` processes slashing reports from consumer chains
- Validates the slashing evidence and finality provider association
- Updates the finality provider's status and propagates slashing to other
  consumers
- Emits `EventSlashedFinalityProvider` for external slashing mechanisms
