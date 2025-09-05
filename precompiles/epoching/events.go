package epoching

import (
	"bytes"
	"math/big"
	"reflect"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"

	cmn "github.com/cosmos/evm/precompiles/common"

	sdk "github.com/cosmos/cosmos-sdk/types"

	checkpointingtypes "github.com/babylonlabs-io/babylon/v4/x/checkpointing/types"
	epochingtypes "github.com/babylonlabs-io/babylon/v4/x/epoching/types"
)

const (
	// EventTypeWrappedCreateValidator defines the event type for the staking CreateValidator transaction.
	EventTypeWrappedCreateValidator = "WrappedCreateValidator"
	// EventTypeWrappedEditValidator defines the event type for the staking EditValidator transaction.
	EventTypeWrappedEditValidator = "WrappedEditValidator"
	// EventTypeWrappedDelegate defines the event type for the staking Delegate transaction.
	EventTypeWrappedDelegate = "WrappedDelegate"
	// EventTypeWrappedUnbond defines the event type for the staking Undelegate transaction.
	EventTypeWrappedUnbond = "WrappedUnbond"
	// EventTypeWrappedRedelegate defines the event type for the staking Redelegate transaction.
	EventTypeWrappedRedelegate = "WrappedRedelegate"
	// EventTypeWrappedCancelUnbondingDelegation defines the event type for the staking CancelUnbondingDelegation transaction.
	EventTypeWrappedCancelUnbondingDelegation = "WrappedCancelUnbondingDelegation"
)

// EmitWrappedCreateValidatorEvent creates a new create validator event emitted on a CreateValidator transaction.
func (p Precompile) EmitWrappedCreateValidatorEvent(ctx sdk.Context, stateDB vm.StateDB, msg *checkpointingtypes.MsgWrappedCreateValidator, validatorAddr common.Address) error {
	// Prepare the event topics
	event := p.Events[EventTypeWrappedCreateValidator]

	topics, err := p.createEditValidatorTxTopics(2, event, validatorAddr)
	if err != nil {
		return err
	}

	// Prepare the event data
	var b bytes.Buffer
	b.Write(cmn.PackNum(reflect.ValueOf(msg.MsgCreateValidator.Value.Amount.BigInt())))

	stateDB.AddLog(&ethtypes.Log{
		Address:     p.Address(),
		Topics:      topics,
		Data:        b.Bytes(),
		BlockNumber: uint64(ctx.BlockHeight()), //nolint:gosec // G115 // won't exceed uint64
	})

	return nil
}

// EmitWrappedEditValidatorEvent creates a new edit validator event emitted on a EditValidator transaction.
func (p Precompile) EmitWrappedEditValidatorEvent(ctx sdk.Context, stateDB vm.StateDB, msg *epochingtypes.MsgWrappedEditValidator, validatorAddr common.Address, epochBoundary uint64) error {
	// Prepare the event topics
	event := p.Events[EventTypeWrappedEditValidator]

	topics, err := p.createEditValidatorTxTopics(2, event, validatorAddr)
	if err != nil {
		return err
	}

	commissionRate := big.NewInt(DoNotModifyCommissionRate)
	if msg.Msg.CommissionRate != nil {
		commissionRate = msg.Msg.CommissionRate.BigInt()
	}

	minSelfDelegation := big.NewInt(DoNotModifyMinSelfDelegation)
	if msg.Msg.MinSelfDelegation != nil {
		minSelfDelegation = msg.Msg.MinSelfDelegation.BigInt()
	}

	// Prepare the event data
	var b bytes.Buffer
	b.Write(cmn.PackNum(reflect.ValueOf(commissionRate)))
	b.Write(cmn.PackNum(reflect.ValueOf(minSelfDelegation)))
	b.Write(cmn.PackNum(reflect.ValueOf(epochBoundary)))

	stateDB.AddLog(&ethtypes.Log{
		Address:     p.Address(),
		Topics:      topics,
		Data:        b.Bytes(),
		BlockNumber: uint64(ctx.BlockHeight()), //nolint:gosec // G115 // won't exceed uint64
	})

	return nil
}

// EmitWrappedDelegateEvent creates a new delegate event emitted on a Delegate transaction.
func (p Precompile) EmitWrappedDelegateEvent(ctx sdk.Context, stateDB vm.StateDB, msg *epochingtypes.MsgWrappedDelegate, delegatorAddr common.Address, epochBoundary uint64) error {
	valAddr, err := sdk.ValAddressFromBech32(msg.Msg.ValidatorAddress)
	if err != nil {
		return err
	}

	// TODO: verify obtaining newShares does make sense for WrappedDelegate method
	//// Get the validator to estimate the new shares delegated
	//// NOTE: At this point the validator has already been checked, so no need to check again
	// validator, _ := p.stakingKeeper.GetValidator(ctx, valAddr)
	//
	//// Get only the new shares based on the delegation amount
	// newShares, err := validator.SharesFromTokens(msg.Msg.Amount.Amount)
	// if err != nil {
	//	return err
	// }

	// Prepare the event topics
	event := p.Events[EventTypeWrappedDelegate]
	topics, err := p.createStakingTxTopics(3, event, delegatorAddr, common.BytesToAddress(valAddr.Bytes()))
	if err != nil {
		return err
	}

	// Prepare the event data
	var b bytes.Buffer
	b.Write(cmn.PackNum(reflect.ValueOf(msg.Msg.Amount.Amount.BigInt())))
	// b.Write(cmn.PackNum(reflect.ValueOf(newShares.BigInt())))
	b.Write(cmn.PackNum(reflect.ValueOf(epochBoundary)))

	stateDB.AddLog(&ethtypes.Log{
		Address:     p.Address(),
		Topics:      topics,
		Data:        b.Bytes(),
		BlockNumber: uint64(ctx.BlockHeight()), //nolint:gosec // G115 // won't exceed uint64
	})

	return nil
}

// EmitWrappedUnbondEvent creates a new unbond event emitted on an Undelegate transaction.
func (p Precompile) EmitWrappedUnbondEvent(ctx sdk.Context, stateDB vm.StateDB, msg *epochingtypes.MsgWrappedUndelegate, delegatorAddr common.Address, epochBoundary uint64) error {
	valAddr, err := sdk.ValAddressFromBech32(msg.Msg.ValidatorAddress)
	if err != nil {
		return err
	}

	// Prepare the event topics
	event := p.Events[EventTypeWrappedUnbond]
	topics, err := p.createStakingTxTopics(3, event, delegatorAddr, common.BytesToAddress(valAddr.Bytes()))
	if err != nil {
		return err
	}

	// Prepare the event data
	var b bytes.Buffer
	b.Write(cmn.PackNum(reflect.ValueOf(msg.Msg.Amount.Amount.BigInt())))
	b.Write(cmn.PackNum(reflect.ValueOf(epochBoundary)))

	stateDB.AddLog(&ethtypes.Log{
		Address:     p.Address(),
		Topics:      topics,
		Data:        b.Bytes(),
		BlockNumber: uint64(ctx.BlockHeight()), //nolint:gosec // G115 // won't exceed uint64
	})

	return nil
}

// EmitWrappedRedelegateEvent creates a new redelegate event emitted on a Redelegate transaction.
func (p Precompile) EmitWrappedRedelegateEvent(ctx sdk.Context, stateDB vm.StateDB, msg *epochingtypes.MsgWrappedBeginRedelegate, delegatorAddr common.Address, epochBoundary uint64) error {
	valSrcAddr, err := sdk.ValAddressFromBech32(msg.Msg.ValidatorSrcAddress)
	if err != nil {
		return err
	}

	valDstAddr, err := sdk.ValAddressFromBech32(msg.Msg.ValidatorDstAddress)
	if err != nil {
		return err
	}

	// Prepare the event topics
	event := p.Events[EventTypeWrappedRedelegate]
	topics, err := p.createStakingTxTopics(4, event, delegatorAddr, common.BytesToAddress(valSrcAddr.Bytes()))
	if err != nil {
		return err
	}

	topics[3], err = cmn.MakeTopic(common.BytesToAddress(valDstAddr.Bytes()))
	if err != nil {
		return err
	}

	// Prepare the event data
	var b bytes.Buffer
	b.Write(cmn.PackNum(reflect.ValueOf(msg.Msg.Amount.Amount.BigInt())))
	b.Write(cmn.PackNum(reflect.ValueOf(epochBoundary)))

	stateDB.AddLog(&ethtypes.Log{
		Address:     p.Address(),
		Topics:      topics,
		Data:        b.Bytes(),
		BlockNumber: uint64(ctx.BlockHeight()), //nolint:gosec // G115 // won't exceed uint64
	})

	return nil
}

// EmitWrappedCancelUnbondingDelegationEvent creates a new cancel unbonding delegation event emitted on a CancelUnbondingDelegation transaction.
func (p Precompile) EmitWrappedCancelUnbondingDelegationEvent(ctx sdk.Context, stateDB vm.StateDB, msg *epochingtypes.MsgWrappedCancelUnbondingDelegation, delegatorAddr common.Address, epochBoundary uint64) error {
	valAddr, err := sdk.ValAddressFromBech32(msg.Msg.ValidatorAddress)
	if err != nil {
		return err
	}

	// Prepare the event topics
	event := p.Events[EventTypeWrappedCancelUnbondingDelegation]
	topics, err := p.createStakingTxTopics(3, event, delegatorAddr, common.BytesToAddress(valAddr.Bytes()))
	if err != nil {
		return err
	}

	// Prepare the event data
	var b bytes.Buffer
	b.Write(cmn.PackNum(reflect.ValueOf(msg.Msg.Amount.Amount.BigInt())))
	b.Write(cmn.PackNum(reflect.ValueOf(big.NewInt(msg.Msg.CreationHeight))))
	b.Write(cmn.PackNum(reflect.ValueOf(epochBoundary)))

	stateDB.AddLog(&ethtypes.Log{
		Address:     p.Address(),
		Topics:      topics,
		Data:        b.Bytes(),
		BlockNumber: uint64(ctx.BlockHeight()), //nolint:gosec // G115 // won't exceed uint64
	})

	return nil
}

// createStakingTxTopics creates the topics for staking transactions Delegate, Undelegate, Redelegate and CancelUnbondingDelegation.
func (p Precompile) createStakingTxTopics(topicsLen uint64, event abi.Event, delegatorAddr common.Address, validatorAddr common.Address) ([]common.Hash, error) {
	topics := make([]common.Hash, topicsLen)
	// NOTE: If your solidity event contains indexed event types, then they become a topic rather than part of the data property of the log.
	// In solidity you may only have up to 4 topics but only 3 indexed event types. The first topic is always the signature of the event.

	// The first topic is always the signature of the event.
	topics[0] = event.ID

	var err error
	topics[1], err = cmn.MakeTopic(delegatorAddr)
	if err != nil {
		return nil, err
	}

	topics[2], err = cmn.MakeTopic(validatorAddr)
	if err != nil {
		return nil, err
	}

	return topics, nil
}

// createEditValidatorTxTopics creates the topics for staking transactions CreateValidator and EditValidator.
func (p Precompile) createEditValidatorTxTopics(topicsLen uint64, event abi.Event, validatorAddr common.Address) ([]common.Hash, error) {
	topics := make([]common.Hash, topicsLen)
	// NOTE: If your solidity event contains indexed event types, then they become a topic rather than part of the data property of the log.
	// In solidity you may only have up to 4 topics but only 3 indexed event types. The first topic is always the signature of the event.

	// The first topic is always the signature of the event.
	topics[0] = event.ID

	var err error
	topics[1], err = cmn.MakeTopic(validatorAddr)
	if err != nil {
		return nil, err
	}

	return topics, nil
}
