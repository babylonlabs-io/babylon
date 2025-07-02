# Incentive

## Table of Contents

1. [Distribution for BTC Stakers and Finality Providers](#1-distribution-for-btc-stakers-and-finality-providers)
   1. [Overview of x/distribution logic](#overview-of-xdistribution-logic)
   2. [Reward Distribution for BABY Stakers](#reward-distribution-for-baby-stakers)
   3. [What is the inflation mechanism and how are rewards distributed](#what-is-the-inflation-mechanism-and-how-are-rewards-distributed)
2. [States](#2-states)
   1. [Parameters](#21-parameters)
   2. [Gauge](#22-gauge)
   3. [Reward Gauge](#23-reward-gauge)
   4. [FinalityProviderHistoricalRewards](#24-finalityproviderhistoricalrewards)
   5. [FinalityProviderCurrentRewards](#25-finalityprovidercurrentrewards)
   6. [BTCDelegationRewardsTracker](#26-btcdelegationrewardstracker)
3. [Messages](#3-messages)
   1. [MsgUpdateParams](#31-msgupdateparams)
   2. [MsgSetWithdrawAddress](#32-msgsetwithdrawaddress)
   3. [MsgWithdrawReward](#33-msgwithdrawreward)
4. [BeginBlocker and EndBlocker](#4-beginblocker-and-endblocker)
5. [Queries](#5-queries)
6. [AnteHandler for refundable transactions](#6-antehandler-decorator-for-refundable-transactions)

## 1. Distribution for BTC Stakers and Finality Providers

The `x/incentive` module is responsible for distributing rewards to both
finality providers and BTC stakers. Initially, rewards are calculated
based on the voting power of the finality provider, with the remaining rewards
distributed to BTC delegators according to their voting power share.

> ⚡ The reward distribution is triggered at the beginning of each
> block during the [BeginBlocker](https://docs.cosmos.network/main/build/building-modules/beginblock-endblock#beginblocker-and-endblocker-1)
> phase for finality providers and BTC delegators.

The module is designed to manage the distribution of rewards for both BTC
stakers and finality providers. Rewards are allocated based on the voting
power of the finality provider and the BTC delegations. This module works
in collaboration with the `x/distribution`. For native stakers and validators,
rewards are distributed through the `x/distribution` module after the initial
distribution to BTC stakers and finality providers.

Within the module, there is a critical object called the `rewards gauge`, which
serves as a gauge accumulating rewards for both finality providers and BTC stakers.
This gauge is a key component of the module, acting as a ledger that tracks
rewards allocated but not yet withdrawn by stakeholders.

```protobuf
// RewardGauge is an object that stores rewards distributed to a BTC staking
// stakeholder code adapted from
// https://github.com/osmosis-labs/osmosis/blob/v18.0.0/proto/osmosis/incentives/gauge.proto
message RewardGauge {
  // coins are coins that have been in the gauge
  // Can have multiple coin denoms
  repeated cosmos.base.v1beta1.Coin coins = 1 [
    (gogoproto.nullable) = false,
    (gogoproto.castrepeated) = "github.com/cosmos/cosmos-sdk/types.Coins"
  ];
  // withdrawn_coins are coins that have been withdrawn by the stakeholder
  // already
  repeated cosmos.base.v1beta1.Coin withdrawn_coins = 2 [
    (gogoproto.nullable) = false,
    (gogoproto.castrepeated) = "github.com/cosmos/cosmos-sdk/types.Coins"
  ];
}
```

There are 2 messages available through CLI

- `MsgWithdrawRewards` - withdraw rewards for a delegator or finality provider
- `MsgSetWithdrawAddress` - set a new withdraw address for a delegator or
  finality provider

### Overview of x/distribution logic

Once the rewards have been distributed by `x/incentive` to BTC stakers
and finality providers, the `x/distribution` module is called in `app.go`. The
`x/distribution` module is designed to passively distribute rewards between
delegators and validators by rewards accumulating in global pool until withdrawals
(e.g. `MsgWithdrawDelegatorReward`) or a delegation state change such as bonding,
unbonding or re-delegation and this would trigger a full reward withdrawal
for that delegation before the state change.

So, while rewards continuously accrue, they are actually distributed
(when transferred out of the pool) only when a withdrawal is executed
or when a change in delegation triggers a withdrawal.

### Reward Distribution for BABY Stakers

Upon `BeginBlock`, following the reward distribution for BTC stakers and finality
providers, the [`x/distribution`](https://docs.cosmos.network/main/build/modules/distribution)
module will then distribute the rest to native stakers and validators. However,
note that rewards are not actively “pushed” out to accounts at this time instead,
they are recorded and remain in the pool until a withdrawal event.

### What is the inflation mechanism and how are rewards distributed

The `x/mint` module defines the inflation mechanism, which is calculated
annually and holds the parameters for the logic that that determines
inflation and the minting of new coins. The logic is based on the current year,
depending on the number of years since genesis. The inflation schedule,
found [here](../mint/README.md) outlines how the inflation rate is expected to
change over time, typically decreasing to a target rate to ensure a controlled
increase in the coin supply.

Each block, there is a check to update the minter's inflation rate if the
provisions (total number of tokens to be minted that year) are zero, which is
expected at genesis. The inflation rate and annual provisions are recalculated
based on the total supply of the staking token. Additionally there is a check
for the number of coins that are minted for that block, this is based on annual
provisions and the precise time elapsed since the previous block.

## 2. States

The Checkpointing module maintains the following KV stores.

### Prefixes

- `DelegatorWithdrawAddrPrefix`: Used for storing the withdraw address for each
  delegator.
- `RefundableMsgKeySetPrefix`: Used for storing the set of hashes of messages
  that can be refunded.
- `FinalityProviderCurrentRewardsKeyPrefix`: Used for storing the Current
  rewards of finality provider by address.
- `BTCDelegationRewardsTrackerKeyPrefix`: Used for for BTC delegation rewards
  tracker info
- `FinalityProviderHistoricalRewardsKeyPrefix`: Used for storing the Historical
  rewards of finality provider by address and period.

### Keys

- `ParamsKey`: Used for storing the parameters of the Incentive module.
- `BTCStakingGaugeKey`: Used for storing the BTC staking gauge at each block
  height.
- `RewardGaugeKey`: Used for storing the reward gauge for a given stakeholder
  in a given type.
- `BTCDelegatorToFPKey`: Used for storing the map reference from a delegator
  to a finality provider.

### 2.1. Parameters

The [parameter management](./keeper/params.go) maintains the
incentive module's parameters. The BTC Checkpoint module's
parameters are represented as a `Params` object
[here](../proto/babylon/incentive/params.proto)
defined as follows:

```protobuf
// Params defines the parameters for the module, including portions of rewards
// distributed to each type of stakeholder. Note that sum of the portions should
// be strictly less than 1 so that the rest will go to Comet
// validators/delegations adapted from
// https://github.com/cosmos/cosmos-sdk/blob/release/v0.47.x/proto/cosmos/distribution/v1beta1/distribution.proto
message Params {
  option (gogoproto.goproto_stringer) = false;

  // btc_staking_portion is the portion of rewards that goes to Finality
  // Providers/delegations NOTE: the portion of each Finality
  // Provider/delegation is calculated by using its voting power and finality
  // provider's commission
  string btc_staking_portion = 1 [
    (cosmos_proto.scalar) = "cosmos.Dec",
    (gogoproto.customtype) = "cosmossdk.io/math.LegacyDec",
    (gogoproto.nullable) = false
  ];
}
```

### 2.2. Gauge

This object stores rewards that are to be distributed, It can hold multiple
denominations of coins and is managed by
[reward gauge management](./keeper/btc_staking_gauge.go).

```protobuf
// Gauge is an object that stores rewards to be distributed
// code adapted from
// https://github.com/osmosis-labs/osmosis/blob/v18.0.0/proto/osmosis/incentives/gauge.proto
message Gauge {
  // coins are coins that have been in the gauge
  // Can have multiple coin denoms
  repeated cosmos.base.v1beta1.Coin coins = 1 [
    (gogoproto.nullable) = false,
    (gogoproto.castrepeated) = "github.com/cosmos/cosmos-sdk/types.Coins"
  ];
}
```

### 2.3. Reward Gauge

The `RewardGauge` is coordinated by the [reward gauge management]
(./keeper/reward_gauge.go) and is used to track rewards distributed to a BTC
staking stakeholder. It includes both the accumulated rewards and the
withdrawn rewards, allowing the system to manage the lifecycle of rewards for
each stakeholder. It is important to note that the amount of rewards available
to withdraw is calculated as the difference between the total coins and the
`withdrawn_coins`.

```protobuf
// RewardGauge is an object that stores rewards distributed to a BTC staking
// stakeholder code adapted from
// https://github.com/osmosis-labs/osmosis/blob/v18.0.0/proto/osmosis/incentives/gauge.proto
message RewardGauge {
  // coins are coins that have been in the gauge
  // Can have multiple coin denoms
  repeated cosmos.base.v1beta1.Coin coins = 1 [
    (gogoproto.nullable) = false,
    (gogoproto.castrepeated) = "github.com/cosmos/cosmos-sdk/types.Coins"
  ];
  // withdrawn_coins are coins that have been withdrawn by the stakeholder
  // already
  repeated cosmos.base.v1beta1.Coin withdrawn_coins = 2 [
    (gogoproto.nullable) = false,
    (gogoproto.castrepeated) = "github.com/cosmos/cosmos-sdk/types.Coins"
  ];
}
```

### 2.4. FinalityProviderHistoricalRewards

This object tracks the cumulative rewards ratio of a finality provider per
satoshi for a given period. It is used to maintain a historical record of
rewards, allowing the system to calculate the difference in rewards
between periods for accurate distribution. Both the
[reward tracker store](./keeper/reward_tracker_store.go)
and [reward tracker](./keeper/reward_tracker.go) are focused on the management
of rewards. It is important to note that the finality provider key and period
is used to store this structure.

```protobuf
// FinalityProviderHistoricalRewards represents the cumulative rewards ratio of
// the finality provider per sat in that period. The period is ommited here and
// should be part of the key used to store this structure. Key: Prefix +
// Finality provider bech32 address + Period.
message FinalityProviderHistoricalRewards {
  // The cumulative rewards of that finality provider per sat until that period
  // This coins will aways increase the value, never be reduced due to keep
  // acumulation and when the cumulative rewards will be used to distribute
  // rewards, 2 periods will be loaded, calculate the difference and multiplied
  // by the total sat amount delegated
  // https://github.com/cosmos/cosmos-sdk/blob/e76102f885b71fd6e1c1efb692052173c4b3c3a3/x/distribution/keeper/delegation.go#L47
  repeated cosmos.base.v1beta1.Coin cumulative_rewards_per_sat = 1 [
    (gogoproto.nullable) = false,
    (gogoproto.castrepeated) = "github.com/cosmos/cosmos-sdk/types.Coins"
  ];
}
```

### 2.5. FinalityProviderCurrentRewards

This message tracks the current rewards for a finality provider that have not
yet been stored in `FinalityProviderHistoricalRewards`. Managed by the
[reward tracker store](./keeper/reward_tracker_store.go)

```protobuf
// FinalityProviderCurrentRewards represents the current rewards of the pool of
// BTC delegations that delegated for this finality provider is entitled to.
// Note: This rewards are for the BTC delegators that delegated to this FP
// the FP itself is not the owner or can withdraw this rewards.
// If a slash event happens with this finality provider, all the delegations
// need to withdraw to the RewardGauge and the related scrutures should be
// deleted. Key: Prefix + Finality provider bech32 address.
message FinalityProviderCurrentRewards {
  // CurrentRewards is the current rewards that the finality provider have and
  // it was not yet stored inside the FinalityProviderHistoricalRewards. Once
  // something happens that modifies the amount of satoshis delegated to this
  // finality provider or the delegators starting period (activation, unbonding
  // or btc rewards withdraw) a new period must be created, accumulate this
  // rewards to FinalityProviderHistoricalRewards with a new period and zero out
  // the Current Rewards.
  repeated cosmos.base.v1beta1.Coin current_rewards = 1 [
    (gogoproto.nullable) = false,
    (gogoproto.castrepeated) = "github.com/cosmos/cosmos-sdk/types.Coins"
  ];
  // Period stores the current period that serves as a reference for
  // creating new historical rewards and correlate with
  // BTCDelegationRewardsTracker StartPeriodCumulativeReward.
  uint64 period = 2;
  // TotalActiveSat is the total amount of active satoshi delegated
  // to this finality provider.
  bytes total_active_sat = 3 [
    (cosmos_proto.scalar) = "cosmos.Int",
    (gogoproto.customtype) = "cosmossdk.io/math.Int",
    (gogoproto.nullable) = false
  ];
}
```

### 2.6. BTCDelegationRewardsTracker

This message tracks the rewards for a BTC delegator, including the starting
period of the last reward withdrawal or modification and the total active
satoshis delegated to a specific finality provider. Managed by the
[reward tracker store](./keeper/reward_tracker_store.go)

```protobuf
// BTCDelegationRewardsTracker represents the structure that holds information
// from the last time this BTC delegator withdraw the rewards or modified his
// active staked amount to one finality provider.
// The finality provider address is ommitted here but should be part of the
// key used to store this structure together with the BTC delegator address.
message BTCDelegationRewardsTracker {
  // StartPeriodCumulativeReward the starting period the the BTC delegator
  // made his last withdraw of rewards or modified his active staking amount
  // of satoshis.
  uint64 start_period_cumulative_reward = 1;
  // TotalActiveSat is the total amount of active satoshi delegated
  // to one specific finality provider.
  bytes total_active_sat = 2 [
    (cosmos_proto.scalar) = "cosmos.Int",
    (gogoproto.customtype) = "cosmossdk.io/math.Int",
    (gogoproto.nullable) = false
  ];
}
```

## 3. Messages

### 3.1. MsgUpdateParams

The `MsgUpdateParams` message is used to update the parameters of the incentive module.
This should only be executable through governance proposals.

```protobuf
message MsgUpdateParams {
  option (cosmos.msg.v1.signer) = "authority";
  string authority = 1;
  Params params = 2;
}
```

Upon receiving `MsgUpdateParams`, a Babylon node will execute as follows:

1. Verify that the sender is authorized to update the parameters.
2. Validate the new parameters to ensure they are within acceptable ranges.
   - Update the module's parameters with the new values.
3. Emit an event indicating the successful update of parameters.

### 3.2. MsgSetWithdrawAddress

The `MsgSetWithdrawAddress` message is used to set a new withdraw address for a
delegator. This address is where the rewards will be sent when they are withdrawn.

```protobuf
message MsgSetWithdrawAddress {
  option (cosmos.msg.v1.signer) = "delegator_address";
  string delegator_address = 1;
  string withdraw_address = 2;
}
```

Upon receiving `MsgSetWithdrawAddress`, a Babylon node will execute as follows:

1. Verify that the sender is the delegator or has the authority to set the
   withdraw address.
2. Update the withdraw address for the delegator in the state.

### 3.3. MsgWithdrawReward

The `MsgWithdrawReward` message is used to withdraw rewards for a delegator.
This message triggers the distribution of accumulated rewards to the specified
withdraw address.

When a user submits a `MsgWithdrawReward` transaction, the system
initiates a process to transfer all rewards that have been accumulated in
the user's account gauge to their account. The gauge acts as a ledger,
tracking the rewards due to the user. Upon processing this message, the
system calculates the total rewards in the gauge and transfers this amount
to the user's account balance. This involves deducting the reward amount
from the gauge and crediting it to the user's account, effectively updating
the blockchain's state to reflect these changes.

```protobuf
message MsgWithdrawReward {
  option (cosmos.msg.v1.signer) = "delegator_address";
  string delegator_address = 1;
}
```

Upon receiving `MsgWithdrawReward`, a Babylon node will execute as follows:

1. Verify that the sender is the delegator or has the authority to withdraw
   rewards.
2. Calculate the rewards owed to the delegator based on the current state.
3. Transfer the calculated rewards to the delegator's withdraw address.

## 4. BeginBlocker and EndBlocker

Upon `BeginBlocker`, the `HandleCoinsInFeeCollector` is called to ensure that
rewards are distributed and available at the start of each block.
The `HandleCoinsInFeeCollector` intercepts a portion of coins in the fee
collector and distributes them to the `x/incentive` module account of the
current height. Whilst this is happening, the `x/incentive` module creates a
new BTC staking gauge of the current height. The portion of coins is also
recorded in the BTC staking gauge that was created. This logic is defined
[here](./abci.go).

Upon `EndBlocker` of the `x/finality` module, we check first to see if the BTC
staking protocol is activated and there exists a height where a finality provider
has voting power. If it exists and the BTC staking protocol is active, we index
the current block and then tally all non-finalized blocks. `k.IndexBlock(ctx)`
and `k.TallyBlocks(ctx)` are called to manage block indexing and tallying.
The function next calculates `heightToExamine` to determine which blocks need
to be examined for liveness and rewarding. If `heightToExamine` is more than or
equal to 1, then we handle liveness checks (`HandleLiveness`) for finality
providers and handle the rewarding process (`HandleRewarding`). `HandleRewarding`
is responsible for processing rewards for finalized blocks. It calls
`rewardBTCStaking`, which in turn calls `RewardBTCStaking` from the
`IncentiveKeeper` to distribute rewards based on the voting power distribution
cache. This logic is defined [here](../finality/abci.go).

In contrast, `BtcDelegationActivated` is triggered when a new BTC delegation
is activated. This function updates the reward tracker KV store to reflect
the new state of the delegation, initiates the creation of a new period,
initializes the finality provider, creates historical reward trackers,
and withdraws BTC delegation rewards to the gauge. While
`BtcDelegationActivated` is not directly invoked by the `EndBlocker`,
it is closely linked to it. The invocation of `BtcDelegationActivated` is
event-driven, responding to specific transactions related to BTC staking,
such as when a user delegates BTC to a finality provider.

## 5. Queries

| **Endpoint**                                                            | **Description**                                                          |
| ----------------------------------------------------------------------- | ------------------------------------------------------------------------ |
| `/babylon/incentive/v1/params`                                          | Queries the current parameters of the Incentive module.                  |
| `/babylon/incentive/v1/btc_staking_gauge/{height}`                      | Retrieves the BTC staking gauge information for a specific block height. |
| `/babylon/incentive/v1/reward_gauges`                                   | Retrieves the reward gauges for all stakeholders.                        |
| `/babylon/incentive/v1/delegators/{delegator_address}/withdraw_address` | Queries the withdraw address of a specific delegator.                    |

## 6. AnteHandler Decorator for Refundable Transactions

In order to prevent misuse of fee grants on refundable transactions,
the module includes a custom `AnteHandler` decorator.
This decorator detects refundable messages and forbids the use of fee grants for those transactions,
ensuring that transaction fees are paid directly by the sender and not by a grantor.
After the transaction is processed, the fees are refunded to the fee payer according to the `PostHandler` logic.

This is necessary to avoid over-complicating the refund logic.

### Logic Summary

- The decorator inspects messages in a transaction.
- If the transaction is marked as _refundable_, the decorator rejects the use of a fee grant.
- This behavior applies at the ante-handling phase, before the message is processed by the application.

At the moment, a transaction is considered refundable if **all** messages it contains are within these:

- `btclctypes.MsgInsertHeaders`
- `btcctypes.MsgInsertBTCSpvProof`
- `bstypes.MsgAddCovenantSigs`
- `bstypes.MsgBTCUndelegate`
- `bstypes.MsgSelectiveSlashingEvidence`
- `bstypes.MsgAddBTCDelegationInclusionProof`
- `ftypes.MsgAddFinalitySig`
