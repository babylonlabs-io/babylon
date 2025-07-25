# BSN Rewards Distribution on Babylon Genesis

1. [Introduction](#introduction)
2. [Main distribution function](#main-distribution-function)
3. [Distribution via `MsgAddBsnRewards` Message](#distribution-via-msgaddbsnrewards-message)
4. [Distribution via IBC Transfer](#distribution-via-ibc-transfer)

## Introduction

The BSN rewards distribution on the Babylon Genesis chain is handled by the
`btcstaking` module.

There are two distinct flows for initiating the rewards distribution, each
relevant for different types of consumers:

1.  **Direct Message Invocation**: This flow is triggered by processing the
    `MsgAddBsnRewards` message.
2.  **IBC Transfer**: This flow is initiated when an IBC transfer with a
    specific memo field is received.

This document details both of these flows.

## Main distribution function

The `AddBsnRewards` function is the core of the rewards distribution process.
It is located in the `x/btcstaking/keeper/rewards.go` file and is responsible
for distributing rewards to the finality providers of a specific BSN consumer.
It can be triggered either by a direct `MsgAddBsnRewards` message or through
an IBC transfer.

The function performs the following actions:

*   **Balance Check**: Verifies that the account initiating the rewards
    distribution has a sufficient balance to cover the total rewards transfer.
*   **Fund Transfer**: Transfers the total rewards from the sender's account to
    the `incentive` module account. This account acts as a holding area for the
    rewards before they are distributed.
*   **Babylon Commission**: Calculates the Babylon commission based on the
    commission rate defined for the BSN consumer. The commission is then sent
    to a predefined module account.
*   **Reward Distribution**: After deducting the Babylon commission, the
    remaining rewards are distributed among the finality providers and their
    corresponding BTC stakers. The distribution is based on the voting power of
    the stakers and the commission rates of the finality providers, using the F1
    algorithm implemented in the `x/incentive` module.

## Distribution via `MsgAddBsnRewards` Message

The message used to trigger rewards distribution is defined as follows:

```protobuf
type MsgAddBsnRewards struct {
	// Sender is the babylon address which will pay for the rewards
	Sender string `protobuf:"bytes,1,opt,name=sender,proto3" json:"sender,omitempty"`
	// BsnConsumerId is the ID of the BSN consumer
	// - for Cosmos SDK chains, the consumer ID will be the IBC client ID
	// - for rollup chains, the consumer ID will be the chain ID of the rollup
	//   chain
	BsnConsumerId string `protobuf:"bytes,2,opt,name=bsn_consumer_id,json=bsnConsumerId,proto3" json:"bsn_consumer_id,omitempty"`
	// TotalRewards is the total amount of rewards to be distributed among finality providers.
	// This amount will be distributed according to the ratios specified in fp_ratios.
	TotalRewards github_com_cosmos_cosmos_sdk_types.Coins `protobuf:"bytes,3,rep,name=total_rewards,json=totalRewards,proto3,castrepeated=github.com/cosmos/cosmos-sdk/types.Coins" json:"total_rewards"`
	// FpRatios is a list of finality providers and their respective reward distribution ratios.
	// The ratios should sum to 1.0 to distribute the entire total_rewards amount.
	FpRatios []FpRatio `protobuf:"bytes,4,rep,name=fp_ratios,json=fpRatios,proto3" json:"fp_ratios"`
}
```

> **⚡ Important** Before sending the message, the `Sender` must have enough
> coins to cover the amount declared in the `TotalRewards` field.

> **⚡ Important** All finality providers in the `FpRatios` list must already be
> registered on the Babylon chain and have active delegations. Otherwise, an
> error will be returned to the caller.

> **⚡ Important** The consumer identified by `BsnConsumerId` must exist on
> Babylon Genesis.

When Babylon Genesis receives a `MsgAddBsnRewards` message, it extracts all
relevant fields and delegates processing to the `AddBsnRewards` function
described earlier.

> **⚡ Important** Babylon does not enforce how the `FpRatios` must be
> calculated. It is up to the message `Sender` to calculate the correct
> distribution.

The process of distributing rewards through `MsgAddBsnRewards` is as follows:

1. The `Sender` calculates how the rewards should be distributed among the
   finality providers.
2. The `Sender` bridges the reward token to the Babylon Genesis chain.
3. The `Sender` sends the `MsgAddBsnRewards` message to Babylon Genesis.

> **⚡ Important**
> It is important that the `MsgAddBsnRewards` message is sent to Babylon Genesis
> as soon as possible after calculating the distribution.

## Distribution via IBC Transfer

To distribute rewards through IBC, the distributing entity must send an ICS20
transfer to Babylon with the `memo` field defined as follows:

```go
const (
	// CallbackActionAddBsnRewardsMemo is the memo string indicating BSN reward distribution
	CallbackActionAddBsnRewardsMemo = "add_bsn_rewards"
)

// CallbackMemo defines the structure for callback memo in IBC transfers
type CallbackMemo struct {
	// Action defines which action to be called and uses checks the field in memo
	Action string `json:"action,omitempty"`
	// DestCallback mandatory dest_callback wrapper to call contract callbacks
	DestCallback *CallbackInfo `json:"dest_callback,omitempty"`
}

// CallbackInfo contains the callback information
type CallbackInfo struct {
	// Address mandatory address to call callbacks, but unused
	Address string `json:"address"`
	// AddBsnRewards fill out this field to call the action to give out
	// rewards to BSN using IBC callback
	AddBsnRewards *CallbackAddBsnRewards `json:"add_bsn_rewards,omitempty"`
}

// CallbackAddBsnRewards callback memo information wrapper to
// add BSN rewards.
type CallbackAddBsnRewards struct {
	// BsnConsumerID specifies which BSN to send the rewards to
	BsnConsumerID string `json:"bsn_consumer_id"`
	// FpRatios splits the rewards between the given FPs ratios
	FpRatios []FpRatio `json:"fp_ratios"`
}
```

The rules of processing those fields are the same as when processing `MsgAddBsnRewards`
message.

When Babylon Genesis receives such a transfer, it will:

1. Parse all necessary data from the `memo` field.
2. Use the `AddBsnRewards` function to distribute rewards to the finality
   providers defined in the `FpRatios` field and their stakers.


