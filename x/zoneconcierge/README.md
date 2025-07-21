# ZoneConcierge

The Zone Concierge module is responsible for providing BTC staking integration
functionalities for other Bitcoin Supercharged Networks (BSNs).  
It leverages the IBC protocol to receive BSNs' headers, and propagate BTC timestamps of those headers
and information associated with the BTC staking protocol (e.g., finality providers, BTC stakes, and more).  
The Zone Concierge module synchronises the following information with consumer
chains via IBC packets:

- **BTC Headers:** Babylon Genesis forwards BTC headers to BSNs to keep their
  BTC light clients in sync with Babylon's BTC light client. This allows
  BSNs to independently verify BTC timestamps.
- **BTC Timestamps:** When a Babylon epoch is finalised, Babylon sends BTC
  timestamps to BSNs. Each BTC timestamp contains:
  - The latest BSN header that was checkpointed in the finalised
    epoch
  - Recent BTC headers that extend the consumer's BTC light client
  - The finalised epoch's metadata and raw checkpoint
  - Proofs that the consumer header was included in the epoch and the epoch was
    properly sealed and submitted to Bitcoin
- **BTC Staking:** Babylon enables trustless Bitcoin staking for BSNs
  by synchronising staking-related information between Bitcoin, Babylon and
  BSNs. This allows BTC holders to stake their BTC to secure consumer
  chains without requiring any custodial solutions.

## Table of contents

<<<<<<< HEAD
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
  - [BSN Integration](#bsn-integration)
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
=======
- [Table of contents](#table-of-contents)
- [State](#state)
  - [Parameters](#parameters)
  - [LatestEpochHeaders](#latestepochheaders)
  - [FinalizedEpochHeaders](#finalizedepochheaders)
  - [BSNBTCState](#bsnbtcstate)
  - [Params](#params)
  - [Port](#port)
  - [LastSentBTCSegment](#lastsentbtcsegment)
  - [SealedEpochProof](#sealedepochproof)
- [PostHandler for intercepting IBC headers](#posthandler-for-intercepting-ibc-headers)
- [Hooks](#hooks)
  - [Indexing headers upon `AfterEpochEnds`](#indexing-headers-upon-afterepochends)
  - [Recording proofs upon `AfterRawCheckpointSealed`](#recording-proofs-upon-afterrawcheckpointsealed)
  - [Sending BTC timestamps upon `AfterRawCheckpointFinalized`](#sending-btc-timestamps-upon-afterrawcheckpointfinalized)
- [EndBlocker](#endblocker)
  - [Broadcasting BTC Headers](#broadcasting-btc-headers)
  - [Broadcasting BTC Staking Events](#broadcasting-btc-staking-events)
- [Handling Inbound IBC Packets](#handling-inbound-ibc-packets)
  - [Inbound IBC Packets](#inbound-ibc-packets)
  - [Processing Inbound IBC Packets](#processing-inbound-ibc-packets)
- [Messages and Queries](#messages-and-queries)
- [BSN Integration](#bsn-integration)
  - [IBC Communication Protocol](#ibc-communication-protocol)
  - [Relaying BTC Headers](#relaying-btc-headers)
  - [Relaying BTC Timestamps](#relaying-btc-timestamps)
  - [Relaying BTC Staking Events](#relaying-btc-staking-events)
>>>>>>> 4e54346 (fix(zoneconcierge): refactor the usage of `ChainInfo` and canonical chain indexer. (#1387))

<!-- TODO: concept section for describing BTC staking integration -->

## State

<<<<<<< HEAD
The Zone Concierge module keeps handling IBC headers of PoS blockchains, and
maintains the following KV stores.
=======
The Zone Concierge module maintains a simplified header indexing system with the following KV stores. Consumer registration is handled by the `btcstkconsumer` module.
>>>>>>> 4e54346 (fix(zoneconcierge): refactor the usage of `ChainInfo` and canonical chain indexer. (#1387))

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

### LatestEpochHeaders

<<<<<<< HEAD
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
=======
The [latest epoch headers storage](./keeper/epoch_header_indexer.go) maintains
the most recent header received from each BSN during the current epoch. The
key is the BSN's `ConsumerID`, and the value is an `IndexedHeader` object.
This storage is cleared at the end of each epoch when headers are moved to the
finalized storage.

### FinalizedEpochHeaders

The [finalized epoch headers storage](./keeper/epoch_header_indexer.go)
maintains headers that have been finalized for each BSN and epoch. The key
is the epoch number plus the BSN's `ConsumerID`, and the value is an
`IndexedHeaderWithProof` object. The `IndexedHeaderWithProof` contains both the
header metadata and the inclusion proof.

```protobuf
// IndexedHeader is the metadata of a BSN header
>>>>>>> 4e54346 (fix(zoneconcierge): refactor the usage of `ChainInfo` and canonical chain indexer. (#1387))
message IndexedHeader {
  // consumer_id is the unique ID of the BSN
  string consumer_id = 1;
  // hash is the hash of this header
  bytes hash = 2;
  // height is the height of this header on the BSN's ledger.
  // (hash, height) jointly provide the position of the header on the BSN ledger
  uint64 height = 3;
  // time is the timestamp of this header on the BSN's ledger.
  // It is needed for a BSN to unbond all mature validators/delegations before
  // this timestamp, when this header is BTC-finalised
  google.protobuf.Timestamp time = 4 [ (gogoproto.stdtime) = true ];
  // babylon_header_hash is the hash of the babylon block that includes this BSN
  // header
  bytes babylon_header_hash = 5;
  // babylon_header_height is the height of the babylon block that includes this
  // BSN header
  uint64 babylon_header_height = 6;
  // epoch is the epoch number of this header on Babylon ledger
  uint64 babylon_epoch = 7;
  // babylon_tx_hash is the hash of the tx that includes this header
  // (babylon_block_height, babylon_tx_hash) jointly provides the position of
  // the header on Babylon ledger
  bytes babylon_tx_hash = 8;
}

// IndexedHeaderWithProof is an indexed header with a proof that the header is
// included in the epoch
message IndexedHeaderWithProof {
  IndexedHeader header = 1;
  // proof is an inclusion proof that the header
  // is committed to the `app_hash` of the sealer header of header.babylon_epoch
  tendermint.crypto.ProofOps proof = 2;
}
```

### BSNBTCState

The [BSN BTC state storage](./keeper/consumer_btc_state.go) maintains
unified BTC synchronization state for each BSN. The key is the BSN's
`ConsumerID`, and the value is a `BSNBTCState` object that tracks the base
BTC header and last sent BTC header segment for each BSN.

```protobuf
// BSNBTCState stores per-BSN BTC synchronization state
// This includes both the base header and the last sent segment
message BSNBTCState {
  // base_header is the base BTC header for this BSN
  // This represents the starting point from which BTC headers are synchronized
  babylon.btclightclient.v1.BTCHeaderInfo base_header = 1;
  // last_sent_segment is the last segment of BTC headers sent to this BSN
  // This is used to determine the next headers to send and handle reorgs
  BTCChainSegment last_sent_segment = 2;
}
```

<<<<<<< HEAD
The [fork storage](./keeper/fork_indexer.go) maintains the metadata of canonical
IBC headers of a PoS blockchain. The key is the PoS blockchain's `ConsumerID`
plus the height, and the value is a list of `IndexedHeader` objects, which
represent fork headers at that height.
=======
>>>>>>> 4e54346 (fix(zoneconcierge): refactor the usage of `ChainInfo` and canonical chain indexer. (#1387))

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

<<<<<<< HEAD
1. If the PoS blockchain hosting the header is not known to Babylon, initialize
   `ChainInfo` storage for the PoS blockchain.
2. If the header is on a fork, insert the header to the fork storage and update
   `ChainInfo`.
3. If the header is canonical, insert the header to the canonical chain storage
   and update `ChainInfo`.
=======
For each IBC client update message in the transaction, the `PostHandler`
executes as follows:

1. Extract the header info and the client state from the message
2. Determine if the header is on a fork by checking if the client is frozen
3. Call `HandleHeaderWithValidCommit` to process the header
4. Check if the BSN is registered through the `btcstkconsumer` module and is a Cosmos BSN; if not, ignore the header
5. Create an `IndexedHeader` with the header metadata and Babylon context
6. If the header is not on a fork and is newer than the existing latest header,
   update the latest epoch header for the BSN
7. Log fork events for monitoring and debugging purposes
>>>>>>> 4e54346 (fix(zoneconcierge): refactor the usage of `ChainInfo` and canonical chain indexer. (#1387))

## Hooks

The Zone Concierge module subscribes to the Epoching module's `AfterEpochEnds`
<<<<<<< HEAD
[hook](../epoching/types/hooks.go) for indexing the epochs when receiving
headers from BSNs, and the Checkpointing module's
`AfterRawCheckpointFinalized` [hook](../checkpointing/types/hooks.go) for
sending BTC timestamps to BSNs.
=======
[hook](../epoching/types/hooks.go) for recording epoch headers, the
Checkpointing module's `AfterRawCheckpointSealed`
[hook](../checkpointing/types/hooks.go) for recording epoch header proofs, and
the Checkpointing module's `AfterRawCheckpointFinalized`
[hook](../checkpointing/types/hooks.go) for sending BTC timestamps to BSNs.
>>>>>>> 4e54346 (fix(zoneconcierge): refactor the usage of `ChainInfo` and canonical chain indexer. (#1387))

### Indexing headers upon `AfterEpochEnds`

The `AfterEpochEnds` hook is triggered when an epoch is ended, i.e., the last
block in this epoch has been committed by CometBFT. Upon `AfterEpochEnds`, the
Zone Concierge will:

1. Record all current latest epoch headers as finalized headers for the completed epoch
2. Clear the latest epoch headers to prepare for the next epoch

<<<<<<< HEAD
=======
### Recording proofs upon `AfterRawCheckpointSealed`

The `AfterRawCheckpointSealed` hook is triggered when an epoch's raw checkpoint
is sealed by validator signatures. Upon `AfterRawCheckpointSealed`, the Zone
Concierge will:

1. Generate inclusion proofs for all finalized headers in the sealed epoch
2. Generate and store the proof that the epoch is sealed

>>>>>>> 4e54346 (fix(zoneconcierge): refactor the usage of `ChainInfo` and canonical chain indexer. (#1387))
### Sending BTC timestamps upon `AfterRawCheckpointFinalized`

The `AfterRawCheckpointFinalized` hook is triggered upon a checkpoint becoming
*finalised*, i.e., Bitcoin transactions of the checkpoint become `w`-deep in
Bitcoin's canonical chain, where `w` is the `checkpoint_finalization_timeout`
[parameter](../../proto/babylon/btccheckpoint/v1/params.proto) in the
[BTCCheckpoint](../btccheckpoint/) module.

Upon `AfterRawCheckpointFinalized`, the Zone Concierge module will prepare and
send a BTC timestamp to each BSN.  
The [BTCTimestamp](../../proto/babylon/zoneconcierge/v1/packet.proto) structure  
includes a header and a set of proofs that the header is checkpointed by
Bitcoin.

<!-- TODO: diagram depicting BTC timestamp -->

```protobuf
// BTCTimestamp is a BTC timestamp that carries information of a BTC-finalized epoch
// It includes a number of BTC headers, a raw checkpoint, an epoch metadata, and 
// a BSN header if there exists BSN headers checkpointed to this epoch.
// Upon a newly finalized epoch in Babylon, Babylon will send a BTC timestamp to each
// BSN via IBC.
message BTCTimestamp {
  // header is the last BSN header in the finalized Babylon epoch
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
  babylon.zoneconcierge.v1.ProofFinalizedHeader proof = 6;
}

// ProofFinalizedHeader is a set of proofs that attest a header is
// BTC-finalized
message ProofFinalizedHeader {
  /*
    The following fields include proofs that attest the header is
    BTC-finalized
  */
  // proof_epoch_sealed is the proof that the epoch is sealed
  babylon.zoneconcierge.v1.ProofEpochSealed proof_epoch_sealed = 1;
  // proof_epoch_submitted is the proof that the epoch's checkpoint is included
  // in BTC ledger It is the two TransactionInfo in the best (i.e., earliest)
  // checkpoint submission
  repeated babylon.btccheckpoint.v1.TransactionInfo proof_epoch_submitted = 2;
  // proof_consumer_header_in_epoch is the proof that the BSN header is included in the epoch
  tendermint.crypto.ProofOps proof_consumer_header_in_epoch = 3;
}
```

When `AfterRawCheckpointFinalized` is triggered, the Zone Concierge module will
send an IBC packet including a `BTCTimestamp` to each BSN. The logic
is defined at [x/zoneconcierge/keeper/hooks.go](./keeper/hooks.go) and works as
follows:

<<<<<<< HEAD
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

=======
1. **Determine BTC headers to broadcast**: Get all BTC headers to be sent in BTC
   timestamps using the global broadcast strategy (fallback to the last `w+1`
   BTC headers from the current tip for compatibility)

2. **Broadcast BTC timestamps to all open channels**: For each open IBC channel
   with Babylon Genesis' Zone Concierge module:
   - Find the `ConsumerID` of the counterparty chain (i.e., the PoS system) in
     the IBC channel
   - Get the finalized header for the `ConsumerID` at the last finalised epoch
   - Get the metadata of the last finalised epoch and its corresponding raw
     checkpoint
   - Generate the proof that the last PoS system's canonical header is committed
     to the epoch's metadata (if applicable)
   - Generate the proof that the epoch is sealed, i.e., receives a BLS
     multisignature generated by validators with >2/3 total voting power at the
     last finalised epoch
   - Generate the proof that the epoch's checkpoint is submitted, i.e., encoded
     in transactions on Bitcoin
   - Assemble all the above and the BTC headers obtained in step 1 as
     `BTCTimestamp`, and send it to the IBC channel in an IBC packet

3. **Update last sent segment**: If headers were broadcast, update the last sent
   BTC segment for future reference

## EndBlocker

The Zone Concierge module implements an `EndBlocker` function that is executed
at the end of every block. The `EndBlocker` is defined at
[x/zoneconcierge/abci.go](./abci.go), and broadcasts BTC headers and BTC staking
related events.

### Broadcasting BTC Headers

The `EndBlocker` calls `BroadcastBTCHeaders` to send BTC headers to all open IBC
channels with BSNs. This ensures that BSNs' BTC light clients stay synchronized
with Babylon Genesis' BTC light client.

The header selection logic now uses per-BSN BTC state tracking with the
following enhanced rules:

- **BSN-specific BTC state**: Each BSN has its own `BSNBTCState`
  that tracks the base BTC header and last sent segment
- **No BSN base header**: Falls back to sending the last `w+1` BTC headers
  from the tip (where `w` is the confirmation depth parameter)
- **BSN base header exists but no headers sent**: Send headers from the
  BSN's base header to the current tip
- **Headers previously sent**: Send headers from the child of the most recent
  valid header in the last sent segment to the current tip
- **Reorg detection**: If no header from the last sent segment is still valid
  (indicating a Bitcoin reorg), fall back to sending from the BSN's base
  header to the current tip

This per-BSN approach provides better efficiency and reorg handling
compared to the previous global broadcast strategy.

### Broadcasting BTC Staking Events

After broadcasting BTC headers, the `EndBlocker` calls
`BroadcastBTCStakingConsumerEvents` to propagate BTC staking events to relevant
BSNs. This function handles the distribution of BTC staking-related events that
need to be communicated to BSNs. The process works as follows:

1. **Getting events**: Gets all pending events from `x/btcstaking` module's
   store

2. **Channel Mapping**: For each BSN that has events:
  - Retrieves all open IBC channels connected to that BSN's port
  - Maps the consumer ID (a BSN's identifier) to its corresponding active
    channels

3. **Event Distribution**:
   - Groups events by consumer ID
   - For each BSN:
     - Assembles its relevant events into an IBC packet
     - Sends the packet to the IBC channel with that BSN

4. **Cleanup and State Management**:
   - After successful transmission, removes sent events from the pending queue
   - Updates relevant indices and counters

This process ensures that all BTC staking events are reliably propagated to the
corresponding BSNs, maintaining consistency across the network and enabling
proper operation of the BTC staking system.

## Handling Inbound IBC Packets

The Zone Concierge module implements the `OnRecvPacket` function to handle
incoming IBC packets from BSNs. The packet handling is defined at
[x/zoneconcierge/module_ibc.go](./module_ibc.go) and processes different types
of inbound packets.

### Inbound IBC Packets

The [inbound packet structure](proto/babylon/zoneconcierge/v1/packet.proto) is
defined as follows. Currently, the Zone Concierge module handles one type of
incoming packet: `BSNSlashingIBCPacket`. This packet type allows BSNs to
report slashing evidence for finality providers.

```protobuf
// InboundPacket represents packets received by Babylon from other chains
message InboundPacket {
  // packet is the actual message carried in the IBC packet
  oneof packet {
    BSNSlashingIBCPacket bsn_slashing = 1;
  }
}

// BSNSlashingIBCPacket defines the slashing information that a BSN sends to Babylon's ZoneConcierge upon a
// BSN slashing event.
// It includes the FP public key, the BSN block height at the slashing event, and the double sign evidence.
message BSNSlashingIBCPacket {
  /// evidence is the FP slashing evidence that the BSN sends to Babylon
  babylon.finality.v1.Evidence evidence = 1;
}
```

### Processing Inbound IBC Packets

The `HandleConsumerSlashing` function (called upon
[OnRecvPacket](x/zoneconcierge/module_ibc.go)) processes slashing reports
received from BSNs through IBC packets, with the following workflow:

1. **Verifying Evidence**:
   - Validates that slashing evidence is present and well-formed
   - Extracts the BTC secret key from the evidence
   - Verifies that the finality provider's BTC public key matches the evidence
2. **Slashing Execution**:
   - Updates the BSN finality provider's slashed status
   - Sends power distribution update events to adjust the Babylon Genesis
     finality provider's voting power (necessary because all BTC stakes must
     delegate to a Babylon Genesis finality provider, so slashing affects their
     voting power)
   - Identifies all BTC delegations associated with the slashed finality
     provider
   - Identifies all affected BSNs, where "affected" means there exists a slashed
     BTC delegation that multi-stakes to a finality provider in this BSN
   - Creates slashed BTC delegation events for each affected BSN
   - Propagates the slashing event to each BSN such that the BSN will update the
     status of affected BTC delegations and update the voting power of affected
     BSN finality providers. (Note: The propagation timing depends on the IBC
     relayer's operation schedule and is not under direct control of this
     module)
3. **Event Emission**: Emits a `EventSlashedFinalityProvider` event for external
   slashing mechanisms (e.g., BTC slasher/vigilante)
>>>>>>> 4e54346 (fix(zoneconcierge): refactor the usage of `ChainInfo` and canonical chain indexer. (#1387))

## Messages and Queries

The Zone Concierge module only has one message `MsgUpdateParams` for updating
the module parameters via a governance proposal.

It provides a set of queries about the status of checkpointed PoS blockchains,
listed at
[docs.babylonlabs.io](https://docs.babylonlabs.io/docs/developer-guides/grpcrestapi#tag/ZoneConcierge).

## BSN Integration

The Zone Concierge module connects Babylon and BSNs, relaying three
types of information through IBC: BTC headers, BTC timestamps, and BTC staking
events.

### IBC Communication Protocol

Channel Configuration:
- Port: `zoneconcierge`
- Ordering: `ORDERED`
- Version: `zoneconcierge-1`

<<<<<<< HEAD
Packet Types:
- Outbound: `BTCHeaders`, `BTCTimestamp`, `BTCStakingConsumerEvent`
- Inbound: `ConsumerSlashingIBCPacket`
=======
| Packet Direction | Types |
|-----------------|-------|
| Outbound | `BTCHeaders`, `BTCTimestamp`, `BTCStakingConsumerEvent` |
| Inbound | `BSNSlashingIBCPacket` |
>>>>>>> 4e54346 (fix(zoneconcierge): refactor the usage of `ChainInfo` and canonical chain indexer. (#1387))

### Relaying BTC Headers

Zone Concierge relays BTC headers from Babylon's BTC light client to consumer
chains to keep their BTC light clients synchronised with Bitcoin's canonical
chain.

#### Broadcasting headers

<<<<<<< HEAD
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
=======
This ensures BSNs can keep their BTC light clients synchronized with Bitcoin's
canonical chain. The headers are sent through IBC packets to all open channels
between Babylon and the BSNs, with enhanced per-consumer tracking for improved
efficiency and reorg handling.
>>>>>>> 4e54346 (fix(zoneconcierge): refactor the usage of `ChainInfo` and canonical chain indexer. (#1387))

### Relaying BTC Timestamps

Zone Concierge sends BTC timestamps to BSNs when a Babylon epoch
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

Zone Concierge propagates BTC staking events from Babylon to BSNs to
enable trustless BTC staking.

#### Broadcasting staking events

The `BroadcastBTCStakingConsumerEvents` function sends staking events to
relevant BSNs.

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

- `HandleConsumerSlashing` processes slashing reports from BSNs
- Validates the slashing evidence and finality provider association
- Updates the finality provider's status and propagates slashing to other
  consumers
- Emits `EventSlashedFinalityProvider` for external slashing mechanisms
