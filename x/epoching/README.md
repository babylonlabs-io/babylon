# Epoching

Babylon implements [epoched
staking](https://github.com/cosmos/cosmos-sdk/blob/main/docs/architecture/adr-039-epoched-staking.md)
to reduce and parameterize the frequency of updating the validator set in
Babylon. This reduces the frequency of Babylon sending checkpoints to Bitcoin,
thereby reducing Babylon's operation cost and its footprint on Bitcoin.

In the epoched staking design, the blockchain is divided into epochs, each of
which contains a fixed number of consecutive blocks. Messages that affect the
validator set's stake distribution are delayed to the end of each epoch for
execution, such that the validator set remains unchanged during each epoch. The
Epoching module is responsible for implementing the epoched staking design,
including:

- tracking the current epoch number of the blockchain;
- recording the metadata of each epoch;
- delaying the execution of messages that affect the validator set's stake
  distribution to the end of each epoch; and
- finishing all unbonding requests for epochs that have a Bitcoin checkpoint
  with sufficient confirmations.

## Table of contents

- [Table of contents](#table-of-contents)
- [Concepts](#concepts)
  - [Problem statement](#problem-statement)
  - [Babylon's Epoching module design](#babylons-epoching-module-design)
- [States](#states)
  - [Parameters](#parameters)
  - [Epochs](#epochs)
  - [Epoch message queue](#epoch-message-queue)
  - [Epoch validator set](#epoch-validator-set)
- [Messages](#messages)
  - [Disabling Staking module messages via AnteHandler](#disabling-staking-module-messages-via-antehandler)
  - [Epoched staking messages](#epoched-staking-messages)
  - [MsgUpdateParams](#msgupdateparams)
- [BeginBlocker and EndBlocker](#beginblocker-and-endblocker)
  - [Disabling Staking module's EndBlocker](#disabling-staking-modules-endblocker)
- [BeginBlocker](#beginblocker)
- [EndBlocker](#endblocker)
- [Hooks](#hooks)
  - [Hooks in the Epoching module](#hooks-in-the-epoching-module)
  - [Bitcoin-assisted unbonding via the `AfterRawCheckpointFinalized` hook](#bitcoin-assisted-unbonding-via-the-afterrawcheckpointfinalized-hook)
- [Events](#events)
- [Queries](#queries)

## Concepts

### Problem statement

In the Cosmos SDK, the validator set can change with every block, impacting
stake distribution through various staking-related actions (e.g., bond/unbond,
delegate/undelegate/redelegate, slash). This frequent updating poses challenges,
as

1. Babylon's Bitcoin Timestamping protocol requires checkpointing the validator
   set to Bitcoin upon every validator set update;
2. Bitcoin's 10-minute block interval makes checkpointing every new block
   impractical; and
3. frequent validator set updates complicate the implementation of threshold
   cryptography, light clients, fair leader election, and staking derivatives,
   as highlighted in
   [ADR-039](https://github.com/cosmos/cosmos-sdk/blob/main/docs/architecture/adr-039-epoched-staking.md).

We introduce the concept of epoching to reduce the frequency of validator set
update, thereby addressing the above challenges. In the epoching design, the
blockchain is divided into epochs, and updating the validator set once per
epoch. This approach, detailed in
[ADR-039](https://github.com/cosmos/cosmos-sdk/blob/main/docs/architecture/adr-039-epoched-staking.md),
has been pursued by multiple efforts (e.g.,
[here](https://github.com/cosmos/cosmos-sdk/pull/8829),
[here](https://github.com/cosmos/cosmos-sdk/pull/10132), and
[here](https://github.com/cosmos/cosmos-sdk/pull/10173)) but was not fully
implemented. Babylon has implemented the Epoching module, catering to specific
design goals such as checkpointing epochs. In addition, Babylon implements
*Bitcoin-assisted unbonding*, where unbonding requests in an epoch will be
finished when the epoch they were created in is checkpointed on Bitcoin with
sufficient confirmations.

### Babylon's Epoching module design

Babylon implements the Epoching module in order to reduce the frequency of
validator set updates, and thus the frequency of checkpointing to Bitcoin.
Specifically, the Epoching module is responsible for the following tasks:

- Dividing the blockchain into epochs.
- Disabling some functionalities of the Staking module.
- Disabling messages of the Staking module.
- Delaying staking-related messages till the end of the epoch.
- Bitcoin-assisted unbonding.

**Dividing the blockchain into epochs.** The epoching mechanism introduces the
concept of epochs. The blockchain is divided into epochs, each consisting of a
fixed number of consecutive blocks. The number of blocks in an epoch is called
epoch interval, which is a system parameter.

**Disabling functionalities of the Staking module.** Babylon disables two
functionalities of the Staking module, namely the validator set update mechanism
and the 21-day unbonding mechanism.

In Cosmos SDK, the Staking module handles staking-related messages and updates
the validator set upon every block. Consequently, the Staking module updates the
validator set upon every block. In order to reduce the frequency of validator
set updates to once per epoch, Babylon disables the validator set update
mechanism of the Staking module.

In addition, the Staking module enforces the "21-day unbonding rule": unbonding
validators and delegations will become unbonded after 21 days (in the default
case). The long unbonding period aims to circumvent the [long-range
attack](https://medium.com/babylonchain-io/why-are-unbonding-periods-so-long-on-proof-of-stake-d44e863c5cb8),
at the cost of capital efficiency. Babylon departs from Cosmos SDK by employing
Bitcoin-assisted unbonding, where unbonding validators and delegations become
unbonded once the corresponding epoch has been checkpointed on Bitcoin. Babylon
disables the 21-day unbonding mechanism to this end.

In order to disable the two functionalities, Babylon disables Staking module's
`EndBlocker` function that updates validator sets and unbonds mature validators
upon a block ends. Instead, upon an epoch that has ended, the Epoching module
will invoke the Staking module's functionality that updates the validator set.
In addition, upon an epoch that has been checkpointed to Bitcoin, the Epoching
module will invoke the Staking module's functionality that unbonds mature
validators.

**Disabling messages of the Staking module.** In order to keep the validator set
unchanged during each epoch, the routing of messages to Staking module is disabled.
Instead, of sending the messages to the Staking module, the Epoching and Checkpointing
modules defines wrapped versions of those messages and forwards their
unwrapped forms to the Staking module upon the end of an epoch. In the [Staking
module](https://github.com/cosmos/cosmos-sdk/blob/v0.50.3/proto/cosmos/staking/v1beta1/tx.proto),
these messages include

- `MsgCreateValidator` for creating a new validator
- `MsgDelegate` for delegating coins from a delegator to a validator
- `MsgBeginRedelegate` for redelegating coins from a delegator and source
  validator to a destination validator.
- `MsgUndelegate` for undelegating from a delegator and a validator.
- `MsgCancelUnbondingDelegation` for cancelling an unbonding delegation of a
  delegator.
- `MsgEditValidator` for editing validator descriptions and commission.
- `MsgUpdateParams` to update parameters of staking module.

The above messages were received by the Staking message server, which now
is being called at the end of an epoch. The routes of the above messages
were never registered by the app router so if anyone tries to send it
to a node it will error out. Since there is no message to be received
by the Staking module, all of commands under `babylond tx staking` were
removed.

**Staking module Migrations.** Since the staking module msg server
was not registered in the app routes, the function of staking `AppModule`
that register the migrations and the query server are in the
[`app.go`](https://github.com/babylonlabs-io/babylon/blob/cf6c0e3873133331d95267a58da06c04a3e2c601/app/app.go#L608). If a new
version of the cosmos-sdk is released and there is a new migration,
it is needed to register the migration there too.

**Delaying wrapped messages to the end of epochs.** The Epoching module
maintains a message queue for each epoch. Upon each wrapped message, the
Epoching module performs basic sanity checks, then enqueues the message to the
message queue. When the epoch ends, the Epoching module will forward queued
messages to the Staking module. Consequently, the Staking module receives and
handles staking-related messages, and performs validator set updates.

**Bitcoin-assisted Unbonding.** Babylon implements the Bitcoin-assisted
unbonding mechanism by invoking the Staking module upon a checkpointed epoch .
Specifically, the Staking module's `BlockValidatorUpdates`
[function](https://github.com/cosmos/cosmos-sdk/blob/7e6948f50cd4838a0161838a099f74e0b5b0213c/x/staking/keeper/val_state_change.go#L36-L102)
is responsible for identifying and unbonding mature validators and delegations
that have been unbonding for 21 days, and is invoked upon every block. Babylon
has disabled the invocation of `BlockValidatorUpdates` per block, and implements
the state management for epochs. When an epoch has a checkpoint that is
sufficiently deep in Bitcoin, the Epoching module will invoke
`BlockValidatorUpdates` to finish all unbonding validators and delegations.

## States

The Epoching module maintains the following KV stores.

### Parameters

The [parameter storage](./keeper/params.go) maintains the Epoching module's
parameters. The Epoching module's parameters are represented as a `Params`
[object](../../proto/babylon/epoching/v1/params.proto) defined as follows:

```protobuf
// Params defines the parameters for the module.
message Params {
  option (gogoproto.equal) = true;

  // epoch_interval is the number of consecutive blocks to form an epoch
  uint64 epoch_interval = 1
      [ (gogoproto.moretags) = "yaml:\"epoch_interval\"" ];
}
```

### Epochs

The [epoch storage](./keeper/params.go) maintains the metadata of each epoch.
The key is the epoch number, and the value is an `Epoch`
[object](../../proto/babylon/epoching/v1/epoching.proto) representing the epoch
metadata.

```protobuf
// Epoch is a structure that contains the metadata of an epoch
message Epoch {
  // epoch_number is the number of this epoch
  uint64 epoch_number = 1;
  // current_epoch_interval is the epoch interval at the time of this epoch
  uint64 current_epoch_interval = 2;
  // first_block_height is the height of the first block in this epoch
  uint64 first_block_height = 3;
  // last_block_time is the time of the last block in this epoch.
  // Babylon needs to remember the last header's time of each epoch to complete
  // unbonding validators/delegations when a previous epoch's checkpoint is
  // finalised. The last_block_time field is nil in the epoch's beginning, and
  // is set upon the end of this epoch.
  google.protobuf.Timestamp last_block_time = 4 [ (gogoproto.stdtime) = true ];
  // sealer is the last block of the sealed epoch
  // sealer_app_hash points to the sealer but stored in the 1st header
  // of the next epoch
  bytes sealer_app_hash = 5;
  // sealer_block_hash is the hash of the sealer
  // the validator set has generated a BLS multisig on the hash,
  // i.e., hash of the last block in the epoch
  bytes sealer_block_hash = 6;
}
```

### Epoch message queue

The Epoching module implements a message queue to delay the execution of
messages that affect the validator set's stake distribution to the end of each
epoch. This ensures that during an epoch, the validator set's stake distribution
will remain unchanged, except for slashed validators. The [epoch message queue
storage](./keeper/epoch_msg_queue.go) maintains the queue of these
staking-related messages. The key is the epoch number concatenated with the
index of the queued message, and the value is a `QueuedMessage`
[object](../../proto/babylon/epoching/v1/epoching.proto) representing this
queued message.

```protobuf
// QueuedMessage is a message that can change the validator set and is delayed
// to the end of an epoch
message QueuedMessage {
  // tx_id is the ID of the tx that contains the message
  bytes tx_id = 1;
  // msg_id is the original message ID, i.e., hash of the marshaled message
  bytes msg_id = 2;
  // block_height is the height when this msg is submitted to Babylon
  uint64 block_height = 3;
  // block_time is the timestamp when this msg is submitted to Babylon
  google.protobuf.Timestamp block_time = 4 [ (gogoproto.stdtime) = true ];
  // msg is the actual message that is sent by a user and is queued by the
  // epoching module
  oneof msg {
    cosmos.staking.v1beta1.MsgCreateValidator msg_create_validator = 5;
    cosmos.staking.v1beta1.MsgDelegate msg_delegate = 6;
    cosmos.staking.v1beta1.MsgUndelegate msg_undelegate = 7;
    cosmos.staking.v1beta1.MsgBeginRedelegate msg_begin_redelegate = 8;
    cosmos.staking.v1beta1.MsgCancelUnbondingDelegation msg_cancel_unbonding_delegation = 9;
    cosmos.staking.v1beta1.MsgEditValidator msg_edit_validator = 10;
    cosmos.staking.v1beta1.MsgUpdateParams msg_update_params = 11;
  }
}
```

In the Cosmos SDK, the `MsgCreateValidator`, `MsgDelegate`, `MsgUndelegate`,
`MsgBeginRedelegate`, `MsgCancelUnbondingDelegation`, `MsgEditValidator` and
`MsgUpdateParams` messages of the Staking. So here they are wrapped into
`QueuedMessage` objects. Their execution is delayed to the end of an epoch
for execution.

### Epoch validator set

The [epoch validator set storage](./keeper/epoch_val_set.go) maintains the
validator set at the beginning of each epoch. The validator set will remain the
same throughout the epoch, unless some validators get slashed during this epoch.
The key is the epoch number concatenated with the validator's address, and the
value is this validator's voting power (in `sdk.Int`) at this epoch.

### Disabling Staking module messages by not registering in the router

In Cosmos SDK, the routing of messages is registered by each `AppModule`,
to avoid bypassing the epoching validator set update by some special case
like CosmWasm or `x/authz` module `MsgExec` the entire routing of `x/staking`
is never registered, this is done at
[RegisterServicesWithoutStaking](https://github.com/babylonlabs-io/babylon/blob/b9ac09b85b904816167741039e2e27ddb876429d/app/app.go#L590).

### Epoched staking messages

The epoched staking messages in the Epoching module are defined at
[proto/babylon/epoching/v1/tx.proto](../../proto/babylon/epoching/v1/tx.proto).
They are simply wrappers of the corresponding messages in Cosmos SDK's Staking
module.

```proto
// MsgWrappedDelegate is the message for delegating stakes
message MsgWrappedDelegate {
  option (gogoproto.equal) = false;
  option (gogoproto.goproto_getters) = false;
  option (cosmos.msg.v1.signer) = "msg";

  cosmos.staking.v1beta1.MsgDelegate msg = 1;
}
// MsgWrappedUndelegate is the message for undelegating stakes
message MsgWrappedUndelegate {
  option (gogoproto.equal) = false;
  option (gogoproto.goproto_getters) = false;
  option (cosmos.msg.v1.signer) = "msg";

  cosmos.staking.v1beta1.MsgUndelegate msg = 1;
}
// MsgWrappedDelegate is the message for moving bonded stakes from a
// validator to another validator
message MsgWrappedBeginRedelegate {
  option (gogoproto.equal) = false;
  option (gogoproto.goproto_getters) = false;
  option (cosmos.msg.v1.signer) = "msg";

  cosmos.staking.v1beta1.MsgBeginRedelegate msg = 1;
}
// MsgWrappedCancelUnbondingDelegation is the message for cancelling
// an unbonding delegation
message MsgWrappedCancelUnbondingDelegation {
  option (gogoproto.equal) = false;
  option (gogoproto.goproto_getters) = false;
  option (cosmos.msg.v1.signer) = "msg";

  cosmos.staking.v1beta1.MsgCancelUnbondingDelegation msg = 1;
}
// MsgWrappedEditValidator defines a message for updating validator description
// and commission rate.
message MsgWrappedEditValidator {
  option (gogoproto.equal) = false;
  option (gogoproto.goproto_getters) = false;
  option (cosmos.msg.v1.signer) = "msg";

  cosmos.staking.v1beta1.MsgEditValidator msg = 1;
}
// MsgWrappedStakingUpdateParams defines a message for updating x/staking module parameters.
message MsgWrappedStakingUpdateParams {
  option (gogoproto.equal) = false;
  option (gogoproto.goproto_getters) = false;
  option (cosmos.msg.v1.signer) = "msg";

  cosmos.staking.v1beta1.MsgUpdateParams msg = 1;
}
```

The handlers of the epoched staking messages in the Epoching module are defined
at [x/epoching/keeper/msg_server.go](./keeper/msg_server.go). Each handler
performs the same [verification
logics](https://github.com/cosmos/cosmos-sdk/blob/v0.50.3/x/staking/keeper/msg_server.go)
of the corresponding message as the ones performed by the Cosmos SDK's Staking
module, and then inserts the message to the epoch message queue storage.

### MsgUpdateParams

The `MsgUpdateParams` message is used for updating the module parameters for the
Epoching module. It can only be executed via a governance proposal.

```protobuf
// MsgUpdateParams defines a message for updating Epoching module parameters.
message MsgUpdateParams {
  option (cosmos.msg.v1.signer) = "authority";

  // authority is the address of the governance account.
  // just FYI: cosmos.AddressString marks that this field should use type alias
  // for AddressString instead of string, but the functionality is not yet implemented
  // in cosmos-proto
  string authority = 1 [(cosmos_proto.scalar) = "cosmos.AddressString"];

  // params defines the epoching parameters to update.
  //
  // NOTE: All parameters must be supplied.
  Params params = 2 [(gogoproto.nullable) = false];
}
```

## BeginBlocker and EndBlocker

Babylon disables the Staking module's EndBlocker to avoid validator set updates
upon each block. The Epoching module implements `BeginBlocker` to initialize an
epoch upon the beginning of an epoch, and implements `EndBlocker` to execute all
messages and update the validator set upon the end of an epoch.

### Disabling Staking module's EndBlocker

Cosmos SDK's Staking module [updates the validator
set](https://github.com/cosmos/cosmos-sdk/blob/v0.50.3/x/staking/keeper/abci.go#L23C1-L24C1)
upon `EndBlocker` of every block. In order to implement the epoching mechanism,
Babylon disables the Staking module's `EndBlocker` [as
follows](../../app/app.go).

```go
// Babylon does not want EndBlock processing in staking
app.ModuleManager.OrderEndBlockers = append(app.ModuleManager.OrderEndBlockers[:2], app.ModuleManager.OrderEndBlockers[2+1:]...) // remove stakingtypes.ModuleName
```

## BeginBlocker

Upon `BeginBlocker`, the Epoching module of each Babylon node will [execute the
following](./abci.go):

1. If at the first block of the next epoch, then do the following:
   1. Enter a new epoch, i.e., create a new `Epoch` object and save it to the
      epoch metadata storage.
   2. Record the current `AppHash` as the *sealer Apphash* for the previous
      epoch. The entire `AppState` till the end of the last epoch commits to
      this `AppHash`, hence the name "sealer AppHash".
   3. Initialize the epoch message queue for the current epoch.
   4. Save the current validator set to the epoch validator set storage.
   5. Trigger hooks and emit events that the chain has entered a new epoch.
2. If at the last block of the current epoch, then record the current
   `BlockHash` as the *sealer BlockHash* for the current epoch. The entire
   blockchain so far commits to this `BlockHash` via a hash chain, hence the
   name "sealer BlockHash".

## EndBlocker

Upon `EndBlocker`, the Epoching module of each Babylon node will [execute the
following](./abci.go) *if at the last block of the current epoch*:

1. Get all queued messages of this epoch in the epoch message queue storage.
2. Forward each of the queued messages to the corresponding message handler in
   the Staking module.
3. Emit events about the execution results of the messages.
4. Invoke the Staking module to update the validator set.
5. Trigger hooks and emit events that the chain has ended the current epoch.

## Hooks

The Epoching module implements a set of hooks to notify other modules about
certain events, and utilizes the `AfterRawCheckpointFinalized`
[hook](../checkpointing/types/hooks.go) in the Checkpointing module for
Bitcoin-assisted unbonding.

### Hooks in the Epoching module

```go
// EpochingHooks event hooks for epoching validator object (noalias)
type EpochingHooks interface {
   AfterEpochBegins(ctx context.Context, epoch uint64)            // Must be called after an epoch begins
   AfterEpochEnds(ctx context.Context, epoch uint64)              // Must be called after an epoch ends
   BeforeSlashThreshold(ctx context.Context, valSet ValidatorSet) // Must be called before a certain threshold (1/3 or 2/3) of validators are slashed in a single epoch
}
```

### Bitcoin-assisted unbonding via the `AfterRawCheckpointFinalized` hook

The Epoching module subscribes to the Checkpointing module's
`AfterRawCheckpointFinalized` [hook](../checkpointing/types/hooks.go) for
Bitcoin-assisted unbonding. The `AfterRawCheckpointFinalized` hook is triggered
upon a checkpoint becoming *finalized*, i.e., Bitcoin transactions of the
checkpoint become `w`-deep in Bitcoin's canonical chain, where `w` is the
`checkpoint_finalization_timeout`
[parameter](../../proto/babylon/btccheckpoint/v1/params.proto) in the
BTCCheckpoint module. Upon `AfterRawCheckpointFinalized`, the Epoching module
will finish all unbonding validators and delegations till the epoch associated
with the finalized checkpoint, including [the following](./keeper/hooks.go):

1. Find the metadata `Epoch` of the epoch associated with the finalized
   checkpoint.
2. Find the timestamp of the last block of this epoch from `Epoch`.
3. Notify the Staking module to finish all unbonding validators and delegations
   before this timestamp.

## Events

The Epoching module defines a set of events about the state updates of epochs,
validators, and delegations.

```protobuf
// EventBeginEpoch is the event emitted when an epoch has started
message EventBeginEpoch { uint64 epoch_number = 1; }
// EventEndEpoch is the event emitted when an epoch has ended
message EventEndEpoch { uint64 epoch_number = 1; }
// EventHandleQueuedMsg is the event emitted when a queued message has been handled
message EventHandleQueuedMsg {
  string original_event_type = 1;
  uint64 epoch_number = 2;
  uint64 height = 3;
  bytes tx_id = 4;
  bytes msg_id = 5;
  repeated bytes original_attributes = 6
      [ (gogoproto.customtype) =
            "github.com/cometbft/cometbft/abci/types.EventAttribute" ];
  string error = 7;
}
// EventSlashThreshold is the event emitted when a set of validators have been slashed
message EventSlashThreshold {
  int64 slashed_voting_power = 1;
  int64 total_voting_power = 2;
  repeated bytes slashed_validators = 3;
}
// EventWrappedDelegate is the event emitted when a MsgWrappedDelegate has been queued
message EventWrappedDelegate {
  string delegator_address = 1;
  string validator_address = 2;
  uint64 amount = 3;
  string denom = 4;
  uint64 epoch_boundary = 5;
}

// EventWrappedUndelegate is the event emitted when a MsgWrappedUndelegate has been queued
message EventWrappedUndelegate {
  string delegator_address = 1;
  string validator_address = 2;
  uint64 amount = 3;
  string denom = 4;
  uint64 epoch_boundary = 5;
}
// EventWrappedBeginRedelegate is the event emitted when a MsgWrappedBeginRedelegate has been queued
message EventWrappedBeginRedelegate {
  string delegator_address = 1;
  string source_validator_address = 2;
  string destination_validator_address = 3;
  uint64 amount = 4;
  string denom = 5;
  uint64 epoch_boundary = 6;
}
// EventWrappedCancelUnbondingDelegation is the event emitted when a MsgWrappedCancelUnbondingDelegation has been queued
message EventWrappedCancelUnbondingDelegation {
  string delegator_address = 1;
  string validator_address = 2;
  uint64 amount = 3;
  int64 creation_height = 4;
  uint64 epoch_boundary = 5;
}
// EventWrappedEditValidator is the event emitted when a MsgWrappedEditValidator has been queued
message EventWrappedEditValidator {
  string validator_address = 1;
  uint64 epoch_boundary = 2;
}
// EventWrappedStakingUpdateParams is the event emitted when a MsgWrappedStakingUpdateParams has been queued
message EventWrappedStakingUpdateParams {
  // unbonding_time is the time duration of unbonding.
  string unbonding_time = 1;
  // max_validators is the maximum number of validators.
  uint32 max_validators = 2;
  // max_entries is the max entries for either unbonding delegation or redelegation (per pair/trio).
  uint32 max_entries = 3;
  // historical_entries is the number of historical entries to persist.
  uint32 historical_entries = 4;
  // bond_denom defines the bondable coin denomination.
  string bond_denom = 5;
  // min_commission_rate is the chain-wide minimum commission rate that a validator can charge their delegators
  string min_commission_rate = 6;
  uint64 epoch_boundary = 7;
}
```

## Queries

The Epoching module provides a set of queries about epochs, validators and
delegations, listed at
[docs.babylonlabs.io](https://docs.babylonlabs.io/docs/developer-guides/grpcrestapi#tag/Epoching).
<!-- TODO: update Babylon doc website -->
