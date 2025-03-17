# Incentive

## Table of Contents

- [1. Introduction](#1-introduction)
- [2. States](#2-states)
  - [2.1. Parameters](#21-parameters)
  - [2.2. Gauge](#22-gauge)
  - [2.3. Reward Gauge](#23-reward-gauge)
  - [2.4. FinalityProviderHistoricalRewards](#24-finalityproviderhistoricalrewards)
  - [2.5. FinalityProviderCurrentRewards](#25-finalityprovidercurrentrewards)
  - [2.6. BTCDelegationRewardsTracker](#26-btcdelegationrewardstracker)
- [3. Messages](#3-messages)
  - [3.1. MsgUpdateParams](#31-msgupdateparams)
  - [3.2. MsgSetWithdrawAddress](#32-msgsetwithdrawaddress)
  - [3.3. MsgWithdrawReward](#33-msgwithdrawreward)
- [4. BeginBlocker](#4-beginblocker)
- [5. Queries](#5-queries)

## 1. Introduction

The Incentive module is responsible for determining the eligibility of rewards
for both finality providers and BTC stakers. Initially, rewards are calculated
based on the voting power of the finality provider, with the remaining rewards
distributed to BTC delegations according to their voting power share.

Rewards and distributions are triggered at the beginning of each
block during the [BeginBlocker](https://docs.cosmos.network/main/build/building-modules/beginblock-endblock#beginblocker-and-endblocker-1)
phase, ensuring immediate availability.

The module is designed to manage the distribution of rewards for both BTC
stakers and finality providers. Rewards are allocated based on the voting
power of the finality provider and the BTC delegations. This module works
in collaboration with the `x/distribution`. For native stakers, rewards are
distributed through the `x/distribution` module after the initial distribution
to BTC stakers and finality providers. Details on this process can be found in
the genesis file [here](../proto/babylon/incentive/genesis.proto).

Within the module, there is an object called the `rewards gauge`, which serves
as a pool accumulating rewards for both finality providers and BTC stakers.
This gauge is a key component of the module, acting as a ledger that tracks
rewards allocated but not yet withdrawn by stakeholders.

There are 2 messages available through CLI

- `MsgWithdrawRewards` - withdraw rewards for a delegator or finality provider
- `MsgSetWithdrawAddress` - set a new withdraw address for a delegator or
    finality provider

### What is the inflation mechanism and how are rewards distributed

The minter module defines the inflation mechanism, which is calculated
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

### Reward Distribution for BABY Stakers

Upon `BeginBlock`, following the reward distribution for BTC stakers and finality
providers, the [`x/distribution`](https://docs.cosmos.network/main/build/modules/distribution)
module will then distribute the rest to native stakers and validators.

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
message Params {
    option (gogoproto.goproto_stringer) = false;
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
message Gauge {
    repeated cosmos.base.v1beta1.Coin coins = 1 [
        (gogoproto.nullable) = false,
        (gogoproto.castrepeated) = "github.com/cosmos/cosmos-sdk/types.Coins"
    ];
}
```

### 2.3. Reward Gauge

The `RewardGauge` is  managed by [reward gauge management](./keeper/reward_gauge.go)
and is used to track rewards distributed to a BTC staking stakeholder. It
includes both the accumulated rewards and the withdrawn rewards, allowing the
 system to manage the lifecycle of rewards for each stakeholder.

```protobuf
message RewardGauge {
repeated cosmos.base.v1beta1.Coin coins = 1 [
        (gogoproto.nullable) = false,
        (gogoproto.castrepeated) = "github.com/cosmos/cosmos-sdk/types.Coins"
    ];
    repeated cosmos.base.v1beta1.Coin withdrawn_coins = 2 [
        (gogoproto.nullable) = false,
        (gogoproto.castrepeated) = "github.com/cosmos/cosmos-sdk/types.Coins"
    ];
}
```

### 2.4. FinalityProviderHistoricalRewards

This message tracks the cumulative rewards ratio of a finality provider per
satoshi for a given period. It is used to maintain a historical record of
rewards, allowing the system to calculate the difference in rewards
between periods for accurate distribution. Both the
[reward tracker store](./keeper/reward_tracker_store.go)
and [reward tracker](./keeper/reward_tracker.go) are focused on the management
of rewards.

```protobuf
message FinalityProviderHistoricalRewards {
    repeated cosmos.base.v1beta1.Coin cumulative_rewards_per_sat = 1 [
        (gogoproto.nullable) = false,
        (gogoproto.castrepeated) = "github.com/cosmos/cosmos-sdk/types.Coins"
    ];
}

```

### 2.5. FinalityProviderCurrentRewards

This message tracks the current rewards for a finality provider that have not
yet been stored in `FinalityProviderHistoricalRewards`. Managed by the
[reward_tracker_store.go](./keeper/reward_tracker_store.go)

```protobuf
message FinalityProviderCurrentRewards {
    repeated cosmos.base.v1beta1.Coin current_rewards = 1 [
        (gogoproto.nullable) = false,
        (gogoproto.castrepeated) = "github.com/cosmos/cosmos-sdk/types.Coins"
    ];
    uint64 period = 2;
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
[reward_tracker_store.go](./keeper/reward_tracker_store.go)

```protobuf
message BTCDelegationRewardsTracker {
uint64 start_period_cumulative_reward = 1;
    bytes total_active_sat = 2 [
        (cosmos_proto.scalar) = "cosmos.Int",
        (gogoproto.customtype) = "cosmossdk.io/math.Int",
        (gogoproto.nullable) = false
    ];
}
```

## 3. Messages

### 3.1. MsgUpdateParams

The `MsgUpdateParams` message is used to update the parameters of the Incentive module. This message is typically submitted by an authorized entity, such as a governance module, to modify the operational parameters of the module.

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
3. Emit an event indicating the successful update of the withdraw address.

### 3.3. MsgWithdrawReward

The `MsgWithdrawReward` message is used to withdraw rewards for a delegator.
This message triggers the distribution of accumulated rewards to the specified
withdraw address.

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
    - Transfer the calculated rewards to the delegator's withdraw address.
3. Emit an event indicating the successful withdrawal of rewards.

## 4. BeginBlocker

Upon `BeginBlocker` the `HandleCoinsInFeeCollector` is called to ensure that
rewards are distributed and available at the start of each block. This logic is
defined [here](./abci.go)

## 5. Queries

### Parameters

Endpoint: `/babylon/incentive/v1/params`
Description: Queries the current parameters of the Incentive module.

#### BTC Staking Gauge

Endpoint: `/babylon/incentive/v1/btc_staking_gauge/{height}`
Description: Retrieves the BTC staking gauge information for a specific block height.

#### Reward Gauges

Endpoint: `/babylon/incentive/v1/reward_gauges`
Description: Retrieves the reward gauges for all stakeholders.

#### Delegator Withdraw Address

Endpoint: `/babylon/incentive/v1/delegators/{delegator_address}/withdraw_address`
Description: Queries the withdraw address of a specific delegator.
