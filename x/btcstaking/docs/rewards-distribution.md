# BSN Rewards Distribution on Babylon Genesis

1. [Introduction](#1-introduction)
    1. [What is BSN Rewards Distribution?](#11-what-is-bsn-rewards-distribution)
    2. [Who Gets Rewards?](#12-who-gets-rewards)
    3. [Key Terminology](#13-key-terminology)
    4. [Rewards Distribution Flow](#14-rewards-distribution-flow)
    5. [Module Overview](#15-module-overview)
2. [Rewards Protocol](#2-rewards-protocol)
    1. [Core Distribution Function](#21-core-distribution-function)
    2. [Babylon Genesis Fee Collection and Storage](#22-babylon-genesis-fee-collection-and-storage)
    3. [Querying, withdrawing rewards](#23-querying-withdrawing-rewards)
    4. [Submitting rewards through transactions](#24-submitting-rewards-through-msgaddbsnrewards)
    5. [Submitting rewards through IBC](#25-submitting-rewards-through-ibc)
3. [Rollup BSN Consumers](#3-rollup-bsn-consumers)
    1. [Bridge funds, transfer using tx](#31-bridge-funds-transfer-using-tx)
    2. [Cosmos BSN Consumers](#32-cosmos-bsn-consumers)

## 1. Introduction

### 1.1. What is BSN Rewards Distribution?

When you stake Bitcoin through Babylon, you're helping secure other
networks by delegating your Bitcoin to finality providers. In return for this
service, these consumer BSNs pay rewards to participants in the staking
ecosystem.

The rewards distribution process connects BSN consumers (like rollups or
Cosmos networks) with Bitcoin stakers and finality providers, ensuring all
participants receive proportional compensation for their contributions to
network security.

### 1.2. Who Gets Rewards?

The rewards distribution involves several participants:

- **Bitcoin Stakers**: Individuals who have staked their Bitcoin through Babylon
    earn rewards proportional to their stake
- **Finality Providers**: Operators who run infrastructure and validate on
   behalf of consumer BSNs, earning both commission from their delegators and
   proportional rewards
- **Babylon Genesis**: The protocol itself earns a commission on all reward
  distributions to fund operations and development

### 1.3. Key Terminology

Below are a list of key terms regarding rewards distribution

- **F1 Fee Distribution Algorithm**: A proven mathematical algorithm (also
  used by Cosmos SDK) that calculates how rewards should be
  distributed proportionally among participants. It ensures accurate reward
  calculations even when participants join or leave at different times.
- **BSN Consumers**: Blockchain networks (like rollups or Cosmos
  networks) that register with Babylon Genesis to receive Bitcoin staking
  security services. These BSN consumers pay rewards in exchange for the security
  provided by Bitcoin stakers.
- **Multi-Chain Support**: The ability to handle reward distributions from
  many different consumer BSNs through a single, unified system on Babylon Genesis.
- **Flexible Invocation**: Rewards can be submitted and processed through
  multiple methods - either direct blockchain transactions or
  IBC transfers with special instructions.

### 1.4. Rewards Distribution Flow

The following diagram illustrates how rewards flow through Babylon Genesis.
![Rewards](./static/rewards.png)

**Flow Explanation:**

1. BSN consumers (rollups or Cosmos networks) accumulate rewards for
   Bitcoin staking security services
2. BSN consumers submit rewards through either:
   - **Direct transactions** (`MsgAddBsnRewards`) for rollups
   - **IBC transfers** with callback memos for Cosmos networks
3. The `x/btcstaking` module processes rewards, validates consumer
   registration, and transfers funds to the `x/incentive` module
4. Babylon Genesis automatically deducts its commission percentage before
   distribution
5. The F1 algorithm distributes remaining rewards proportionally among
   finality providers and their Bitcoin staker delegators

### 1.5. Module overview

The rewards distribution is handled by the `x/btcstaking` module and
integrates with several other Babylon modules:

- `x/btcstaking`: Core module managing finality providers, delegations,
  and reward distribution
- `x/incentive`: Handles the F1 fee distribution algorithm and reward
  calculation logic
- `x/btcstkconsumer`: Manages BSN consumer registrations and
  commission rates
- `IBC Callback Middleware`: Processes IBC transfers with reward
  distribution instructions

**Target Audience**: This document is intended as a technical reference for
developers implementing BSN reward distribution systems. This includes BSN
developers building reward distribution mechanisms, protocol integrators
connecting to Babylon Genesis, and finality provider operators implementing
automated reward processing.

## 2. Rewards Protocol

### 2.1. Core Distribution Function

The `AddBsnRewards` function is the core of the rewards distribution process
and is responsible for distributing rewards to the finality providers of a
specific BSN consumer. It can be triggered either by a direct `MsgAddBsnRewards`
message or through
an IBC transfer.

The [`AddBsnRewards` function](../keeper/msg_server.go)
coordinates the entire reward distribution
process:

1. Verifies the sender has sufficient balance and all finality
   providers are registered with active delegations
2. Moves total rewards from sender to the `incentive` module
   account for processing
3. Automatically deducts Babylon Genesis commission and transfers it to the
    protocol's collection account
4. Distributes remaining rewards among finality providers and their BTC
    stakers using the F1 distribution algorithm

The function can be triggered by either a direct `MsgAddBsnRewards` 
transaction or an IBC transfer with callback parameters, ensuring consistent 
processing regardless of the submission method.

> **⚡ Important:** Babylon does not enforce how the `FpRatios` must be
> calculated. It is up to the message `Sender` to calculate the correct 
> distribution based on their reward distribution logic.

### 2.2. Babylon Genesis Fee Collection and Storage

As part of the rewards distribution process, Babylon Genesis automatically
calculates and deducts a commission from the total rewards before distributing
them to finality providers and their stakers. The following steps occur in
sequence during each reward distribution:

The commission rate is set when a BSN consumer registers on Babylon Genesis.
During registration, the consumer specifies their `BabylonRewardsCommission`
rate, which is stored in the consumer registration record.

The Babylon commission is calculated using the formula: `Total Rewards × 
Commission Rate` and automatically transferred to the `commission_collector_bsn` 
module account before reward distribution. This account is managed by the 
`x/incentive` module and controlled by the protocol.

The remaining rewards are distributed to finality providers and their BTC 
stakers through the F1 distribution algorithm implemented in the `x/incentive` 
module.

### 2.3. Querying, withdrawing rewards

The BTC staking module handles BSN rewards distribution by transferring funds to
the `x/incentive` module, which then manages the actual reward distribution,
tracking, and withdrawal for BTC stakers and finality providers.

BSN rewards are processed through the `AddBsnRewards` function (detailed in 
section 2.1) and then managed by the `x/incentive` module for tracking and 
withdrawal.

The rewards distribution leverages the existing incentive module infrastructure
for:
- Reward gauge management
- Historical reward tracking
- Withdrawal functionality

#### Querying Rewards

Since rewards are managed by the `x/incentive` module, the incentive module's
query endpoints should be used to check their reward status. The BTC
staking module focuses on the distribution mechanism rather than reward
tracking.

#### Withdrawing Rewards

Reward withdrawal is handled through the `x/incentive` module's withdrawal
system. The BTC staking module ensures that:

- Finality provider commissions are properly allocated to their reward gauges
- BTC delegator rewards are distributed proportionally based on their stake
- All rewards follow the F1 distribution algorithm for accurate tracking

*For detailed information about querying and withdrawing rewards, please refer
to the [`x/incentive` module documentation](../../incentive/README.md), as
the actual reward management is handled there.*

### 2.4. Submitting rewards through `MsgAddBsnRewards`

BSN consumers can distribute rewards to their finality providers and BTC stakers
by submitting `MsgAddBsnRewards` transactions directly to Babylon Genesis.
The following steps occur in sequence during transaction-based
reward distribution:

#### 1. Reward Distribution Calculation
The sender (typically a BSN consumer or authorised entity) calculates how
rewards should be distributed among finality providers based on their
performance, voting power, and stake contributions. This calculation
determines the `FpRatios` field in the message.

#### 2. Token Bridging
The sender bridges the reward tokens from their source chain to Babylon Genesis.
This ensures the tokens are available on Babylon Genesis for distribution
through the protocol's reward system.

#### 3. Message Construction and Submission
The sender constructs and submits the `MsgAddBsnRewards` message with the following structure:

```go
type MsgAddBsnRewards struct {
	// Sender is the babylon address which will pay for the rewards
	Sender string `protobuf:"bytes,1,opt,name=sender,proto3" json:"sender,omitempty"`
	// BsnConsumerId is the ID of the BSN consumer
	BsnConsumerId string `protobuf:"bytes,2,opt,name=bsn_consumer_id,json=bsnConsumerId,proto3" json:"bsn_consumer_id,omitempty"`
	// TotalRewards is the total amount of rewards to be distributed among finality providers
	TotalRewards github_com_cosmos_cosmos_sdk_types.Coins `protobuf:"bytes,3,rep,name=total_rewards,json=totalRewards,proto3,castrepeated=github.com/cosmos/cosmos-sdk/types.Coins" json:"total_rewards"`
	// FpRatios is a list of finality providers and their respective reward distribution ratios
	FpRatios []FpRatio `protobuf:"bytes,4,rep,name=fp_ratios,json=fpRatios,proto3" json:"fp_ratios"`
}
```

**Field Explanations:**
- `Sender`: Babylon address (bbn...) that will pay for the rewards and must
    have sufficient balance
- `BsnConsumerId`: For Cosmos SDK networks, this is the IBC client ID; for
    rollups, this is the rollup ID
- `TotalRewards`: Total reward amount to be distributed according to the
    specified ratios
- `FpRatios`: List specifying how rewards should be distributed among finality
    providers (ratios should sum to 1.0)

> **⚡ Important:** Before sending the message, the `Sender` must have enough
> coins to cover the amount declared in the `TotalRewards` field.
>
> All finality providers in the `FpRatios` list must
> already be
> registered on the Babylon chain and have active delegations. Otherwise, an
> error will be returned to the caller.
>
> The BSN consumer identified by `BsnConsumerId` must exist on
> Babylon Genesis.

#### 4. Automatic Processing
Once received, Babylon Genesis processes the transaction through the 
`AddBsnRewards` function described in section 2.1.

> **⚡ Important:** The message should be sent as soon as possible after
> calculating the distribution to ensure timely reward processing.

### 2.5. Submitting rewards through IBC

Cosmos SDK-based BSN consumers can distribute rewards using IBC transfers with
specially formatted memo fields. This method leverages Inter-Blockchain
Communication to trigger reward distribution through callback mechanisms.
The following steps occur in sequence during IBC-based reward distribution:

#### 1. IBC Transfer Preparation
The sender prepares an ICS20 token transfer to Babylon Genesis, including the
reward tokens and a specially formatted memo field that contains callback
instructions for reward distribution.

#### 2. Memo Field Construction
The sender constructs a JSON memo field with the following structure for IBC
callback processing:

```go
// CallbackMemo defines the structure for callback memo in IBC transfers
type CallbackMemo struct {
	Action string `json:"action,omitempty"`
	DestCallback *CallbackInfo `json:"dest_callback,omitempty"`
}

// CallbackInfo contains the callback information
type CallbackInfo struct {
	Address string `json:"address"`
	AddBsnRewards *CallbackAddBsnRewards `json:"add_bsn_rewards,omitempty"`
}

// CallbackAddBsnRewards specifies BSN reward distribution parameters
type CallbackAddBsnRewards struct {
	BsnConsumerID string `json:"bsn_consumer_id"`
	FpRatios []FpRatio `json:"fp_ratios"`
}
```

**Field Explanations:**
- `Action`: Must be set to `"add_bsn_rewards"` to trigger reward distribution
- `Address`: Required field for callback mechanism (can be placeholder)
- `BsnConsumerID`: Identifies which BSN consumer the rewards are for
- `FpRatios`: Specifies reward distribution ratios among finality providers

#### 3. IBC Transfer Execution
The sender executes the IBC transfer with the constructed memo field.
The transfer amount represents the total rewards to be distributed according
to the specified ratios.

#### 4. Callback Processing
When Babylon Genesis receives the IBC transfer, it parses the memo field and 
triggers the same `AddBsnRewards` processing as direct transactions.

> **⚡ Important:**
> IBC-based reward distribution follows the same validation rules and processing
> logic as direct `MsgAddBsnRewards` transactions, ensuring consistent
> behavior across both submission methods.

## 3. Rollup BSN Consumers

### 3.1. Bridge funds, transfer using tx

Rollup BSN consumers follow a transaction-based reward distribution model that 
requires
external bridging infrastructure and direct message submission to Babylon
Genesis.

Rollup BSN Consumers register on Babylon Genesis by submitting a
`MsgRegisterConsumer` transaction. This message includes:

- `ConsumerId`: The rollup ID that will be used as `BsnConsumerId` in
  reward transactions
- `ConsumerName` and `ConsumerDescription`: Human-readable information about
  the rollup
- `RollupFinalityContractAddress`: The address of the finality contract
  deployed on Babylon Genesis (this field distinguishes rollup BSN from
  Cosmos BSN consumers)
- `BabylonRewardsCommission`: The commission rate that Babylon Genesis will
  automatically deduct from reward distributions

Babylon Genesis validates that the finality contract exists on-chain
and stores the rollup's metadata, including the commission rate and contract
address.

The rollup must implement its own bridge infrastructure to transfer
reward tokens from the rollup to Babylon Genesis. Babylon Genesis doesn't
provide bridging mechanisms - it simply requires that reward tokens be
available in the sender's account before transaction submission. The bridge
must handle cross-chain communication, finality requirements, and security
measures like multi-signature schemes.

Once tokens are bridged, the rollup submits a `MsgAddBsnRewards` transaction 
using their registered rollup ID as the `BsnConsumerId`, following the 
same process as described in section 2.4.

### 3.2. Cosmos BSN Consumers

Cosmos SDK-based BSN consumers use IBC transfers with specialised callback 
mechanisms
to distribute rewards without requiring external bridge infrastructure.

Cosmos BSN consumer register by providing an IBC client ID as their 
`ConsumerId`.
Babylon Genesis validates that the corresponding IBC light client exists
and stores the consumer commission rate.

Instead of external bridging, Cosmos BSN consumers use standard IBC transfers 
with
with the `memo` field (as seen below) to trigger reward distribution. When
Babylon Genesis receives an IBC transfer, the callback system
processes the memo field to extract reward distribution parameters.

The memo must contain a JSON structure with `action: "add_bsn_rewards"` and
include the BSN consumer ID and finality provider ratios.

Here's an example memo field:

```json
{
  "action": "add_bsn_rewards",
  "dest_callback": {
    "address": "bbn1a2cghwg94u6n5qpjahecv7rtdn0ygx8ugqf46e",
    "add_bsn_rewards": {
      "bsn_consumer_id": "07-tendermint-0",
      "fp_ratios": [
        {
          "btc_pk": "04d4436af9ab1cebd15296cd68ecdb20e48b9c190df4eed1cb3c0a2cf45514d9",
          "ratio": "0.700000000000000000"
        },
        {
          "btc_pk": "4ced6bba09417a58d14fb68528f27c8d25318a5c9e4b1af95415b2a4554403a2",
          "ratio": "0.300000000000000000"
        }
      ]
    }
  }
}
```

The entire amount sent with the ICS20 transfer will be used as rewards for 
distribution, processed through the same reward distribution system as direct 
transactions.

