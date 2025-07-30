# BSN Rewards Distribution on Babylon Genesis

1. [Introduction](#1-introduction)
2. [Rewards Protocol](#2-rewards-protocol)
    1. [Core Distribution Function](#21-core-distribution-function)
    2. [Babylon Genesis Fee Collection and Storage](#22-babylon-genesis-fee-collection-and-storage)
    3. [Querying, withdrawing rewards](#23-querying-withdrawing-rewards)
    4. [Submitting rewards through transactions](#24-submitting-rewards-through-transactions)
    5. [Submitting rewards through IBC](#25-submitting-rewards-through-ibc)
3. [Rollup BSNs](#3-rollup-bsns)
    1. [Bridge funds, transfer using tx](#31-bridge-funds-transfer-using-tx)
    2. [Cosmos BSNs](#32-cosmos-bsns)

## 1. Introduction

The BSN rewards distribution on the Babylon Genesis chain is handled by the
`btcstaking` module.

There are two distinct flows for initiating the rewards distribution, each
relevant for different types of consumers:

1.  **Direct Message Invocation**: This flow is triggered by processing the
    `MsgAddBsnRewards` message. See
    [Submitting rewards through transactions](#submitting-rewards-through-transactions).
2.  **IBC Transfer**: This flow is initiated when an IBC transfer with a
    specific memo field is received. See
    [Submitting rewards through IBC](#submitting-rewards-through-ibc).

**Target Audience**: This document is intended as a technical reference for
developers implementing BSN reward distribution systems. This includes BSN
developers building reward distribution mechanisms, protocol integrators
connecting to Babylon Genesis, and finality provider operators implementing
automated reward processing.

## 2. Rewards Protocol

### 2.1. Core Distribution Function

The `AddBsnRewards` function is the core of the rewards distribution process.
It is located in the `x/btcstaking/keeper/rewards.go` file and is responsible
for distributing rewards to the finality providers of a specific BSN.
It can be triggered either by a direct `MsgAddBsnRewards` message or through
an IBC transfer.

The `AddBsnRewards` function performs the following actions:

1. Verifies that the account initiating the rewards
    distribution has a sufficient balance to cover the total rewards transfer.
2. Transfers the total rewards from the sender's account to
    the `incentive` module account. This account acts as a holding area for the
    rewards before they are distributed.
3. Calculates the Babylon commission based on the
    commission rate defined for the BSN. The commission is then sent
    to a predefined module account.
4. After deducting the Babylon commission, the remaining rewards are
    distributed among the finality providers and their
    corresponding BTC stakers. The distribution is based on the voting power of
    the stakers and the commission rates of the finality providers, using the F1
    algorithm implemented in the `x/incentive` module.

When Babylon Genesis receives a `MsgAddBsnRewards` message, it extracts all
relevant fields and delegates processing to the `AddBsnRewards` function
described earlier.

> **⚡ Important** Babylon does not enforce how the `FpRatios` must be
> calculated. It is up to the message `Sender` to calculate the correct
> distribution.

An overview of the process of distributing rewards through `MsgAddBsnRewards` is
as follows:

1. The `Sender` calculates how the rewards should be distributed among the
   finality providers.
2. The `Sender` bridges the reward token to the Babylon Genesis chain.
3. The `Sender` sends the `MsgAddBsnRewards` message to Babylon Genesis.

> **⚡ Important**
> It is important that the `MsgAddBsnRewards` message is sent to Babylon Genesis
> as soon as possible after calculating the distribution.

### 2.2. Babylon Genesis Fee Collection and Storage

As part of the rewards distribution process, Babylon Genesis automatically
calculates and deducts a commission from the total rewards before distributing
them to finality providers and their stakers. The following steps occur in
sequence during each reward distribution:

#### 1. Commission Rate Definition
The commission rate is set when a BSN consumer registers on Babylon Genesis.
During registration, the consumer specifies their `BabylonRewardsCommission`
rate, which is stored in the consumer registration record.

#### 2. Commission Calculation
The Babylon commission is calculated using the formula: `Total Rewards ×
Commission Rate`. The remaining rewards after commission deduction are then
distributed among finality providers and their BTC stakers using the F1
distribution algorithm implemented in the `x/incentive` module.

#### 3. Commission Storage
The calculated commission is stored in a dedicated module account called
`commission_collector_bsn`. This is a special account managed by the
`x/incentive` module that is controlled by the protocol and not accessible
to external parties without governance.

#### 4. Commission Transfer Process
The commission transfer happens automatically through the system's banking functions:
1. Funds move from the `incentive` module account
    (where all rewards initially arrive)
2. The calculated commission amount is transferred to the
    `commission_collector_bsn` module account (managed by the `x/incentive` module)
3. This transfer occurs before any rewards are distributed to
    finality providers or stakers

The commission collection is completely automatic and ensures Babylon Genesis
receives its predetermined cut from every reward distribution.

The remaining rewards (total rewards minus Babylon commission) are then
distributed among the finality providers and their corresponding BTC stakers
according to their voting power and commission rates.

### 2.3. Querying, withdrawing rewards

The BTC staking module handles BSN rewards distribution by transferring funds to
the `x/incentive` module, which then manages the actual reward distribution,
tracking, and withdrawal for BTC stakers and finality providers.

#### Reward Distribution Process

When BSN rewards are distributed through the BTC staking module:

1. Rewards are sent via `MsgAddBsnRewards` or IBC transfer
2. Babylon Genesis automatically deducts its commission
3. For each finality provider:
   - FP commission is calculated and allocated
   - Remaining rewards are distributed to BTC delegators
4. Rewards are handled by the `x/incentive` module using its F1 distribution
    algorithm

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
to the [`x/incentive` module documentation](../incentive/README.md), as
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

```protobuf
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
- `BsnConsumerId`: For Cosmos SDK chains, this is the IBC client ID; for
    rollup chains, this is the rollup chain ID
- `TotalRewards`: Total reward amount to be distributed according to the
    specified ratios
- `FpRatios`: List specifying how rewards should be distributed among finality
    providers (ratios should sum to 1.0)

> **⚡ Important** Before sending the message, the `Sender` must have enough
> coins to cover the amount declared in the `TotalRewards` field.

> **⚡ Important** All finality providers in the `FpRatios` list must already be
> registered on the Babylon chain and have active delegations. Otherwise, an
> error will be returned to the caller.

> **⚡ Important** The consumer identified by `BsnConsumerId` must exist on
> Babylon Genesis.

#### 4. Automatic Processing
Once received, Babylon Genesis automatically processes the transaction by
deducting its commission and distributing the remaining rewards to finality
providers and their stakers using the F1 distribution algorithm.

> **⚡ Important**
> The `MsgAddBsnRewards` message should be sent to Babylon Genesis as soon as
possible after calculating the distribution to ensure timely reward processing.

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
- `BsnConsumerID`: Identifies which BSN the rewards are for
- `FpRatios`: Specifies reward distribution ratios among finality providers

#### 3. IBC Transfer Execution
The sender executes the IBC transfer with the constructed memo field.
The transfer amount represents the total rewards to be distributed according
to the specified ratios.

#### 4. Callback Processing
When Babylon Genesis receives the IBC transfer, it automatically:
1. Parses the memo field to extract reward distribution parameters
2. Validates the BSN ID and finality provider ratios
3. Triggers the same `AddBsnRewards` processing as direct transactions
4. Deducts Babylon commission and distributes remaining rewards using the F1
    algorithm

> **⚡ Important**
> IBC-based reward distribution follows the same validation rules and processing
> logic as direct `MsgAddBsnRewards` transactions, ensuring consistent
> behavior across both submission methods.

## 3. Rollup BSNs

### 3.1. Bridge funds, transfer using tx

Rollup BSNs follow a transaction-based reward distribution model that requires
external bridging infrastructure and direct message submission to Babylon
Genesis.

Rollup consumers register on Babylon Genesis by submitting a
`MsgRegisterConsumer` transaction. This message includes:

- `ConsumerId`: The rollup chain ID that will be used as `BsnConsumerId` in
  reward transactions
- `ConsumerName` and `ConsumerDescription`: Human-readable information about
  the rollup
- `RollupFinalityContractAddress`: The address of the finality contract
  deployed on Babylon Genesis (this field distinguishes rollup consumers from
  Cosmos consumers)
- `BabylonRewardsCommission`: The commission rate that Babylon Genesis will
  automatically deduct from reward distributions

Babylon Genesis validates that the finality contract exists on-chain
and stores the rollup's metadata, including the commission rate and contract
address.

The rollup chain must implement its own bridge infrastructure to transfer
reward tokens from the rollup to Babylon Genesis. Babylon Genesis doesn't
provide bridging mechanisms - it simply requires that reward tokens be
available in the sender's account before transaction submission. The bridge
must handle cross-chain communication, finality requirements, and security
measures like multi-signature schemes.

Once tokens are bridged, the rollup submits a `MsgAddBsnRewards` transaction
using their registered rollup chain ID as the `BsnConsumerId`. The transaction
follows the same validation rules as described in section 2.4: the sender
must have sufficient balance, all finality providers must be registered with
active delegations, and the consumer must exist in the registry.

Babylon Genesis processes rollup rewards using the same `AddBsnRewards`
function as other types.

### 3.2. Cosmos BSNs

Cosmos SDK-based BSNs use IBC transfers with specialised callback mechanisms
to distribute rewards without requiring external bridge infrastructure.

Cosmos consumers register by providing an IBC client ID as their `ConsumerId`.
Babylon Genesis validates that the corresponding IBC light client exists
and stores the consumer's commission rate.

Instead of external bridging, Cosmos BSNs use standard IBC transfers with
with the `memo` field (as seen below) to trigger reward distribution. When
Babylon Genesis receives an IBC transfer, the callback system
processes the memo field to extract reward distribution parameters.

The memo must contain a JSON structure with `action: "add_bsn_rewards"` and
include the BSN consumer ID and finality provider ratios.

```go
type CallbackMemo struct {
    Action       string        `json:"action,omitempty"`
    DestCallback *CallbackInfo `json:"dest_callback,omitempty"`
}

type CallbackInfo struct {
    Address       string                   `json:"address"`
    AddBsnRewards *CallbackAddBsnRewards   `json:"add_bsn_rewards,omitempty"`
}

type CallbackAddBsnRewards struct {
    BsnConsumerID string    `json:"bsn_consumer_id"`
    FpRatios      []FpRatio `json:"fp_ratios"`
}
```

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
distribution. The callback processing validates the IBC transfer was
successful, parses the transfer data, extracts the memo parameters, and
calculates the proper IBC denomination for the transferred tokens.

Once processed, the system calls the same `AddBsnRewards` function used by
direct transactions, ensuring identical validation, commission deduction and
reward distribution.

