# Finality

Babylon's BTC Staking protocol introduces an additional consensus round on
blocks produced by CometBFT, called the finality round. The participants of this
round are referred to as finality providers and their voting power stems from
staked bitcoins delegated to them.

The Finality module is responsible for handling finality votes, maintaining the
finalization status of blocks, and identifying equivocating finality providers
in the finalization rounds. This includes:

- handling requests for committing EOTS public randomness from finality
providers;
- handling requests for submitting finality votes from finality providers;
- maintaining the finalization status of blocks;
- identifying sluggish finality providers; and
- maintaining equivocation evidence of culpable finality providers.

## Table of contents

- [Table of contents](#table-of-contents)
- [Concepts](#concepts)
- [States](#states)
  - [Parameters](#parameters)
  - [Voting power table](#voting-power-table)
  - [Public randomness](#public-randomness)
  - [Finality votes](#finality-votes)
  - [Indexed blocks with finalization status](#indexed-blocks-with-finalization-status)
  - [Equivocation evidences](#equivocation-evidences)
  - [Signing info tracker](#signing-info-tracker)
- [Messages](#messages)
  - [MsgCommitPubRandList](#msgcommitpubrandlist)
  - [MsgAddFinalitySig](#msgaddfinalitysig)
  - [MsgUpdateParams](#msgupdateparams)
- [BeginBlocker](#beginblocker)
- [EndBlocker](#endblocker)
- [Events](#events)
- [Queries](#queries)

## Concepts

<!-- summary of BTC staking protocol and BTC staking module -->
**Babylon Bitcoin Staking.** Babylon's Bitcoin Staking protocol allows bitcoin
holders to *trustlessly* stake their bitcoins, in order to provide economic
security to the Babylon chain and other Proof-of-Stake (PoS) blockchains. The
protocol composes a PoS blockchain with an off-the-shelf *finality voting round*
run by a set of [finality
providers](https://github.com/babylonlabs-io/finality-provider) who receive *BTC
delegations* from [BTC stakers](https://github.com/babylonlabs-io/btc-staker). The
finality providers and BTC delegations are maintained by Babylon's [BTC Staking
module](../btcstaking/README.md), and the Finality module is responsible for
maintaining the finality voting round.

<!-- introducing finality voting round, Finality module -->
**Finality voting round.**  In the finality voting round, a block committed in
the CometBFT ledger receives *finality votes* from a set of finality providers.
A finality vote is a signature under the [*Extractable One-Time Signature
(EOTS)*
primitive](https://docs.babylonlabs.io/assets/files/btc_staking_litepaper-32bfea0c243773f0bfac63e148387aef.pdf).
A block is considered finalized if it receives a quorum, i.e., votes from
finality providers with more than 2/3 voting power at its height.

<!-- Babylon BTC staking security guarantee, i.e., slashable safety -->
**Slashable safety guarantee.** The finality voting round ensures the *slashable
safety* property of finalized blocks: upon a safety violation where a
conflicting block also receives a valid quorum, adversarial finality providers
with more than 1/3 total voting power will be provably identified by the
protocol and be slashed. The formal definition of slashable safety can be found
at [the S&P'23 paper](https://arxiv.org/pdf/2207.08392.pdf) and [the CCS'23
paper](https://arxiv.org/pdf/2305.07830.pdf). In Babylon's Bitcoin Staking
protocol, if a finality provider is slashed, then

- the secret key of the finality provider is revealed to the public,
- a parameterized amount of bitcoins of all BTC delegations under it will be
  burned *on the Bitcoin network*, and
- the finality provider's voting power will be zeroized.

In addition to the standard safety guarantee of CometBFT consensus, the
slashable safety guarantee disincentivizes safety offences launched by
adversarial finality providers.

<!-- user stories of finality provider and finality module -->
**Interaction between finality providers and the Finality module.** In order to
participate in the finality voting round, an active finality provider with BTC
delegations (as specified in the [BTC Staking module](../btcstaking/README.md))
needs to interact with Babylon as follows:

- **Committing EOTS public randomness.** The finality provider proactively sends
  a merkle-tree-based commit of a list of *EOTS public randomness* for future
  heights to the Finality module. EOTS ensures that given an EOTS public
  randomness, a signer can only sign a single message. Otherwise, anyone can
  extract the signer's secret key by using two EOTS signatures on different
  messages, the corresponding EOTS public randomness, and the signer's public
  key. A public randomness commit takes effect only after it is BTC-timestamped.
- **Submitting EOTS signatures.** Upon a new block, the finality provider
  submits an EOTS signature w.r.t. the derived public randomness at that height.
  The Finality module will verify the EOTS signature, and check if there are
  known EOTS signatures on conflicting blocks from this finality provider. If
  yes, then this constitutes an equivocation, and the Finality module will save
  the equivocation evidence, such that anyone can extract the finality
  provider's secret key and slash it.

Babylon has implemented a [BTC staking
tracker](https://github.com/babylonlabs-io/vigilante) daemon program that
subscribes to equivocation evidences in the Finality module, and slashes BTC
delegations under equivocating finality providers by sending their slashing
transactions to the Bitcoin network.

## States

The Finality module maintains the following KV stores.

### Parameters

The [parameter storage](./keeper/params.go) maintains the Finality module's
parameters. The Finality module's parameters are represented as a `Params`
[object](../../proto/babylon/finality/v1/params.proto) defined as follows:

```protobuf
// Params defines the parameters for the module.
message Params {
  option (gogoproto.goproto_stringer) = false;

  // max_active_finality_providers is the maximum number of active finality providers in the BTC staking protocol
  uint32 max_active_finality_providers = 1;
  // signed_blocks_window defines the size of the sliding window for tracking finality provider liveness
  int64 signed_blocks_window  = 2;
  // finality_sig_timeout defines how much time (in terms of blocks) finality providers have to cast a finality
  // vote before being judged as missing their voting turn on the given block
  int64 finality_sig_timeout = 3;
  // min_signed_per_window defines the minimum number of blocks that a finality provider is required to sign
  // within the sliding window to avoid being jailed
  bytes min_signed_per_window = 4 [
    (cosmos_proto.scalar)  = "cosmos.Dec",
    (gogoproto.customtype) = "cosmossdk.io/math.LegacyDec",
    (gogoproto.nullable)   = false,
    (amino.dont_omitempty) = true
  ];
  // min_pub_rand is the minimum number of public randomness each
  // message should commit
  uint64 min_pub_rand = 5;
  // jail_duration is the minimum period of time that a finality provider remains jailed
  google.protobuf.Duration jail_duration = 6
  [(gogoproto.nullable) = false, (amino.dont_omitempty) = true, (gogoproto.stdduration) = true];
  // finality_activation_height is the babylon block height which the finality module will
  // start to accept finality voting and the minimum allowed value for the public randomness
  // commit start height.
  uint64 finality_activation_height = 7;
}
```

### Voting power table

The [voting power table management](./keeper/voting_power_table.go) maintains
the voting power table of all finality providers at each height of the Babylon
chain. The key is the block height concatenated with the finality provider's
Bitcoin secp256k1 public key in BIP-340 format, and the value is the finality
provider's voting power quantified in Satoshis. Voting power is assigned to top
`N` (defined in parameters) finality providers that have BTC-timestamped public
randomness for the height, ranked by the total delegated value.

### Public randomness

The [public randomness storage](./keeper/public_randomness.go) maintains the
EOTS public randomness commit that each finality provider commits to Babylon.
The key is the finality provider's Bitcoin secp256k1 public key concatenated
with the block height, and the value is a merkle tree constructed by the list of
public randomness with starting height, and the number of public randomness. It
also stores the epoch number at which Babylon receives the commit.

```protobuf
// PubRandCommit is a commitment to a series of public randomness
// currently, the commitment is a root of a Merkle tree that includes
// a series of public randomness
message PubRandCommit {
    // start_height is the height of the first commitment
    uint64 start_height = 1;
    // num_pub_rand is the number of committed public randomness
    uint64 num_pub_rand = 2;
    // commitment is the value of the commitment
    // currently, it is the root of the merkle tree constructed by the public randomness
    bytes commitment = 3;
    // epoch_num defines the epoch number that the commit falls into
    uint64 epoch_num = 4;
}
```

### Finality votes

The [finality vote storage](./keeper/votes.go) maintains the finality votes of
finality providers on blocks. The key is the block height concatenated with the
finality provider's Bitcoin secp256k1 public key, and the value is a
`SchnorrEOTSSig` [object](../../types/btc_schnorr_eots.go) representing an EOTS
signature. Here, the EOTS signature is signed over a block's height and
`AppHash` by the finality provider, using the private randomness corresponding
to the EOTS public randomness derived using the block height. The EOTS signature
serves as a finality vote on this block from this finality provider. It is a
32-byte scalar and is defined as a 32-byte array in the implementation.

```go
type SchnorrEOTSSig []byte
const SchnorrEOTSSigLen = 32
```

### Indexed blocks with finalization status

The [indexed block storage](./keeper/indexed_blocks.go) maintains the necessary
metadata and finalization status of blocks. The key is the block height and the
value is an `IndexedBlock` object
[defined](../../proto/babylon/finality/v1/finality.proto) as follows.

```protobuf
// IndexedBlock is the necessary metadata and finalization status of a block
message IndexedBlock {
    // height is the height of the block
    uint64 height = 1;
    // app_hash is the AppHash of the block
    bytes app_hash = 2;
    // finalized indicates whether the IndexedBlock is finalised by 2/3
    // finality providers or not
    bool finalized = 3;
}
```

### Equivocation evidences

The [equivocation evidence storage](./keeper/evidence.go) maintains evidences of
equivocation offences committed by finality providers. The key is a finality
provider's Bitcoin secp256k1 public key concatenated with the block height, and
the value is an `Evidence`
[object](../../proto/babylon/finality/v1/finality.proto) representing the
evidence that this finality provider has equivocated at this height. Anyone
observing the `Evidence` object can extract the finality provider's Bitcoin
secp256k1 secret key, as per EOTS's extractability property.

```protobuf
// Evidence is the evidence that a finality provider has signed finality
// signatures with correct public randomness on two conflicting Babylon headers
message Evidence {
    // fp_btc_pk is the BTC PK of the finality provider that casts this vote
    bytes fp_btc_pk = 1 [ (gogoproto.customtype) = "github.com/babylonlabs-io/babylon/types.BIP340PubKey" ];
    // block_height is the height of the conflicting blocks
    uint64 block_height = 2;
    // pub_rand is the public randomness the finality provider has committed to
    bytes pub_rand = 3 [ (gogoproto.customtype) = "github.com/babylonlabs-io/babylon/types.SchnorrPubRand" ];
    // canonical_app_hash is the AppHash of the canonical block
    bytes canonical_app_hash = 4;
    // fork_app_hash is the AppHash of the fork block
    bytes fork_app_hash = 5;
    // canonical_finality_sig is the finality signature to the canonical block
    // where finality signature is an EOTS signature, i.e.,
    // the `s` in a Schnorr signature `(r, s)`
    // `r` is the public randomness that is already committed by the finality provider
    bytes canonical_finality_sig = 6 [ (gogoproto.customtype) = "github.com/babylonlabs-io/babylon/types.SchnorrEOTSSig" ];
    // fork_finality_sig is the finality signature to the fork block
    // where finality signature is an EOTS signature
    bytes fork_finality_sig = 7 [ (gogoproto.customtype) = "github.com/babylonlabs-io/babylon/types.SchnorrEOTSSig" ];
}
```

### Signing info tracker

Information about finality providers' voting histories is tracked through
`FinalityProviderSigningInfo`. It is indexed in the store as follows:

- `FinalityProviderSigningTracker: BTCPublicKey -> ProtoBuffer
  (FinalityProviderSigningInfo)`

- FinalityProviderMissedBlockBitmap: `BTCPublicKey -> VarInt(didMiss)` (varint
  is a number encoding format)

The first mapping allows us to easily look at the recent signing info for a
finality provider based on its public key, while the second mapping
(`MissedBlocksBitArray`) acts as a bit-array of size `SignedBlocksWindow` that
tells us if the finality provider missed the block for a given index in the
bit-array. The index in the bit-array is given as little-endian uint64. The
result is a varint that takes on 0 or 1, where 0 indicates the finality provider
did not miss (did sign) the corresponding block, and 1 indicates they missed the
block (did not sign).

Note that the `MissedBlocksBitArray` is not explicitly initialized up-front.
Keys are added as the first `SignedBlocksWindow` blocks for a newly active
finality provider. The `SignedBlocksWindow` parameter defines the size (number
of blocks) of the sliding window used to track finality provider liveness.

The information stored for tracking finality provider liveness is as follows:

```protobuf
// FinalityProviderSigningInfo defines a finality provider's signing info 
// for monitoring their liveness activity.
message FinalityProviderSigningInfo {
  // fp_btc_pk is the BTC PK of the finality provider that casts this finality
  // signature
  bytes fp_btc_pk = 1 [ (gogoproto.customtype) = "github.com/babylonlabs-io/babylon/types.BIP340PubKey" ];
  // start_height is the block height at which finality provider become active
  int64 start_height = 2;
  // missed_blocks_counter defines a counter to avoid unnecessary array reads.
  // Note that `Sum(MissedBlocksBitArray)` always equals `MissedBlocksCounter`.
  int64 missed_blocks_counter = 3;
}
```

Note that the value of `missed_blocks_counter` in the
`FinalityProviderSigningInfo` is the same as the summed value of the
corresponding missed block bitmap. This is to avoid unnecessary bitmap reads.
Also note that the judgement of whether a finality signature is `missed` or not
is irreversible.

The two maps will be updated upon `BeginBlock` which will be described in a
later section.

## Messages

The Finality module handles the following messages from finality providers. The
message formats are defined at
[proto/babylon/finality/v1/tx.proto](../../proto/babylon/finality/v1/tx.proto).
The message handlers are defined at
[x/finality/keeper/msg_server.go](./keeper/msg_server.go).

### MsgCommitPubRandList

The `MsgCommitPubRandList` message is used for committing a merkle tree
constructed by a list of EOTS public randomness that will be used by a finality
provider in the future. It is typically submitted by a finality provider via the
[finality provider](https://github.com/babylonlabs-io/finality-provider) program.

```protobuf
// MsgCommitPubRandList defines a message for committing a list of public randomness for EOTS
message MsgCommitPubRandList {
  option (cosmos.msg.v1.signer) = "signer";
  string signer = 1;
  // fp_btc_pk is the BTC PK of the finality provider that commits the public randomness
  bytes fp_btc_pk = 2 [ (gogoproto.customtype) = "github.com/babylonlabs-io/babylon/types.BIP340PubKey" ];
  // start_height is the start block height of the list of public randomness
  uint64 start_height = 3;
  // num_pub_rand is the number of public randomness committed
  uint64 num_pub_rand = 4;
  // commitment is the commitment of these public randomness
  // currently it's the root of the Merkle tree that includes these public randomness
  bytes commitment = 5;
  // sig is the signature on (start_height || num_pub_rand || commitment) signed by 
  // SK corresponding to fp_btc_pk. This prevents others to commit public
  // randomness on behalf of fp_btc_pk
  bytes sig = 6 [ (gogoproto.customtype) = "github.com/babylonlabs-io/babylon/types.BIP340Signature" ];
}
```

Upon `MsgCommitPubRandList`, a Babylon node will execute as follows:

1. Ensure the message contains at least `MinPubRand` number of EOTS public
   randomness, where `MinPubRand` is defined in the module parameters.
2. Ensure the finality provider has been registered in Babylon.
3. Ensure the list of EOTS public randomness does not overlap with existing EOTS
   public randomness that this finality provider previously committed before.
4. Verify the Schnorr signature over the list of public randomness signed by the
   finality provider.
5. Store the list of EOTS public randomness along with the current epoch number
   to the public randomness storage.

### MsgAddFinalitySig

The `MsgAddFinalitySig` message is used for submitting a finality vote, i.e., an
EOTS signature over a block signed by a finality provider. It is typically
submitted by a finality provider via the [finality
provider](https://github.com/babylonlabs-io/finality-provider) program.

```protobuf
// MsgAddFinalitySig defines a message for adding a finality vote
message MsgAddFinalitySig {
    option (cosmos.msg.v1.signer) = "signer";

    string signer = 1;
    // fp_btc_pk is the BTC PK of the finality provider that casts this vote
    bytes fp_btc_pk = 2 [ (gogoproto.customtype) = "github.com/babylonlabs-io/babylon/types.BIP340PubKey" ];
    // block_height is the height of the voted block
    uint64 block_height = 3;
    // block_app_hash is the AppHash of the voted block
    bytes block_app_hash = 4;
    // finality_sig is the finality signature to this block
    // where finality signature is an EOTS signature, i.e.,
    // the `s` in a Schnorr signature `(r, s)`
    // `r` is the public randomness that is already committed by the finality provider
    bytes finality_sig = 5 [ (gogoproto.customtype) = "github.com/babylonlabs-io/babylon/types.SchnorrEOTSSig" ];
}
```

Upon `MsgAddFinalitySig`, a Babylon node will execute as follows:

1. Ensure the finality provider has been registered in Babylon and is not
   slashed.
2. Ensure the epoch that the finality provider is registered has been finalized
   by BTC timestamping.
3. Ensure the finality provider has voting power at this height.
4. Ensure the finality provider has not previously casted the same vote.
5. Derive the EOTS public randomness using the committed EOTS master public
   randomness and the block height.
6. Verify the EOTS signature w.r.t. the derived EOTS public randomness.
7. If the voted block's `AppHash` is different from the canonical block at the
   same height known by the Babylon node, then this means the finality provider
   has voted for a fork. Babylon node buffers this finality vote to the evidence
   storage. If the finality provider has also voted for the block at the same
   height, then this finality provider is slashed, i.e., its voting power is
   removed, equivocation evidence is recorded, and a slashing event is emitted.
8. If the voted block's `AppHash` is same as that of the canonical block at the
   same height, then this means the finality provider has voted for the
   canonical block, and the Babylon node will store this finality vote to the
   finality vote storage. If the finality provider has also voted for a fork
   block at the same height, then this finality provider will be slashed.

### MsgUpdateParams

The `MsgUpdateParams` message is used for updating the module parameters for the
Finality module. It can only be executed via a govenance proposal.

```protobuf
// MsgUpdateParams defines a message for updating finality module parameters.
message MsgUpdateParams {
    option (cosmos.msg.v1.signer) = "authority";
  
    // authority is the address of the governance account.
    // just FYI: cosmos.AddressString marks that this field should use type alias
    // for AddressString instead of string, but the functionality is not yet implemented
    // in cosmos-proto
    string authority = 1 [(cosmos_proto.scalar) = "cosmos.AddressString"];
  
    // params defines the finality parameters to update.
    //
    // NOTE: All parameters must be supplied.
    Params params = 2 [(gogoproto.nullable) = false];
}
```

### MsgResumeFinalityProposal

The `MsgResumeFinalityProposal` message is used for resuming finality in case
of finality halting. It can only be executed via a governance proposal.

```protobuf
// MsgResumeFinalityProposal is a governance proposal to resume finality from halting
message MsgResumeFinalityProposal {
  option (cosmos.msg.v1.signer) = "authority";

  // authority is the address of the governance account.
  // just FYI: cosmos.AddressString marks that this field should use type alias
  // for AddressString instead of string, but the functionality is not yet implemented
  // in cosmos-proto
  string authority = 1 [(cosmos_proto.scalar) = "cosmos.AddressString"];
  // fp_pks_hex is a list of finality provider public keys to jail
  // the public key follows encoding in BIP-340 spec
  repeated string fp_pks_hex = 2;
  // halting_height is the height where the finality halting begins
  uint32 halting_height = 3;
}
```

## BeginBlocker

Upon `BeginBlocker`, the Finality module of each Babylon node will [execute the
following](./abci.go):

1. Record the voting power table at the current height, by reconciling the
   voting power table at the last height with all events that affect voting
   power distribution (including newly active BTC delegations, newly unbonded
   BTC delegations, and slashed finality providers). Note that the voting power
   is assigned to a finality provider if it (1) has BTC-timestamped public
   randomness, and (2) it is ranked at top `N` by the total delegated value.
2. If the BTC Staking protocol is activated, i.e., there exists at least 1
   active BTC delegation, then record the voting power distribution w.r.t. the
   active finality providers and active BTC delegations.

## EndBlocker

Upon `EndBlocker`, the Finality module of each Babylon node will [execute the
following](./abci.go) *if the BTC staking protocol is activated (i.e., there has
been >=1 active BTC delegations)*:

1. Index the current block, i.e., extract its height and `AppHash`, construct an
   `IndexedBlock` object, and save it to the indexed block storage.
2. Tally all non-finalized blocks as follows:
   1. Find the starting height that the Babylon node should start to finalize.
      This is the earliest height that is not finalize yet since the activation
      of BTC staking.
   2. For each `IndexedBlock` between the starting height and the current
      height, tally this block as follows:
      1. Find the set of active finality providers at this height.
      2. If the finality provider set is empty, then this block is not
         finalizable and the Babylon node will skip this block.
      3. If the finality provider set is not empty, then find all finality votes
         on this `IndexedBlock`, and check whether this `IndexedBlock` has
         received votes of more than 2/3 voting power from the active finality
         provider set. If yes, then finalize this block, i.e., set this
         `IndexedBlock` to be finalized in the indexed block storage and
         distribute rewards to the voted finality providers and their BTC
         delegations. Otherwise, none of the subsequent blocks shall be
         finalized and the loop breaks here.
3. Update the finality provider's voting history and label it to `sluggish` if
   the number of block it has missed has passed the parameterized threshold.

## Events

The Finality module defines the following events.

```protobuf
// EventSlashedFinalityProvider is the event emitted when a finality provider is slashed
// due to signing two conflicting blocks
message EventSlashedFinalityProvider {
    // evidence is the evidence that the finality provider double signs
    Evidence evidence = 1;
}

// EventSluggishFinalityProviderDetected is the event emitted when a finality provider is
// detected as sluggish
message EventSluggishFinalityProviderDetected {
// public_key is the BTC public key of the finality provider
string public_key = 1;
}

// EventSluggishFinalityProviderReverted is the event emitted when a sluggish finality
// provider is no longer considered sluggish
message EventSluggishFinalityProviderReverted {
// public_key is the BTC public key of the finality provider
string public_key = 1;
}

```

## Queries

The Finality module provides a set of queries about finality signatures on each
block, listed at
[docs.babylonlabs.io](https://docs.babylonlabs.io/docs/developer-guides/grpcrestapi#tag/Finality).
<!-- TODO: update Babylon doc website -->
