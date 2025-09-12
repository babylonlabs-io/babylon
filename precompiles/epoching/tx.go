package epoching

import (
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/core/vm"

	cmn "github.com/cosmos/evm/precompiles/common"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	// epoching precompile transactions
	WrappedCreateValidatorMethod           = "wrappedCreateValidator"
	WrappedEditValidatorMethod             = "wrappedEditValidator"
	WrappedDelegateMethod                  = "wrappedDelegate"
	WrappedUndelegateMethod                = "wrappedUndelegate"
	WrappedRedelegateMethod                = "wrappedRedelegate"
	WrappedCancelUnbondingDelegationMethod = "wrappedCancelUnbondingDelegation"

	// bech32 versions of epoching precompile transactions
	WrappedDelegateBech32Method                  = "wrappedDelegateBech32"
	WrappedUndelegateBech32Method                = "wrappedUndelegateBech32"
	WrappedRedelegateBech32Method                = "wrappedRedelegateBech32"
	WrappedCancelUnbondingDelegationBech32Method = "wrappedCancelUnbondingDelegationBech32"
)

// WrappedCreateValidator performs create validator
func (p Precompile) WrappedCreateValidator(
	ctx sdk.Context,
	contract *vm.Contract,
	stateDB vm.StateDB,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	bondDenom, err := p.stakingKeeper.BondDenom(ctx)
	if err != nil {
		return nil, err
	}
	msg, validatorHexAddr, err := NewMsgWrappedCreateValidator(args, bondDenom, p.addrCdc, p.valCdc)
	if err != nil {
		return nil, err
	}

	p.Logger(ctx).Debug(
		"tx called",
		"method", method.Name,
		"commission", msg.MsgCreateValidator.Commission.String(),
		"min_self_delegation", msg.MsgCreateValidator.MinSelfDelegation.String(),
		"validator_address", validatorHexAddr.String(),
		"pubkey", msg.MsgCreateValidator.Pubkey.String(),
		"value", msg.MsgCreateValidator.Value.Amount.String(),
	)

	msgSender := contract.Caller()
	// we won't allow calls from smart contracts
	if hasCode := stateDB.GetCode(msgSender) != nil; hasCode {
		return nil, errors.New(ErrCannotCallFromContract)
	}
	if msgSender != validatorHexAddr {
		return nil, fmt.Errorf(cmn.ErrRequesterIsNotMsgSender, msgSender.String(), validatorHexAddr.String())
	}

	if _, err = p.checkpointingMsgServer.WrappedCreateValidator(ctx, msg); err != nil {
		return nil, err
	}

	// Emit the event for the create validator transaction
	if err = p.EmitWrappedCreateValidatorEvent(ctx, stateDB, msg, validatorHexAddr); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}

func (p Precompile) WrappedEditValidator(
	ctx sdk.Context,
	contract *vm.Contract,
	stateDB vm.StateDB,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	msg, validatorHexAddr, err := NewMsgWrappedEditValidator(args, p.valCdc)
	if err != nil {
		return nil, err
	}

	p.Logger(ctx).Debug(
		"tx called",
		"method", method.Name,
		"validator_address", msg.Msg.ValidatorAddress,
		"commission_rate", msg.Msg.CommissionRate,
		"min_self_delegation", msg.Msg.MinSelfDelegation,
	)

	msgSender := contract.Caller()
	// we won't allow calls from smart contracts
	if hasCode := stateDB.GetCode(msgSender) != nil; hasCode {
		return nil, errors.New(ErrCannotCallFromContract)
	}
	if msgSender != validatorHexAddr {
		return nil, fmt.Errorf(cmn.ErrRequesterIsNotMsgSender, msgSender.String(), validatorHexAddr.String())
	}

	if _, err = p.epochingMsgServer.WrappedEditValidator(ctx, msg); err != nil {
		return nil, err
	}

	epochBoundary := p.epochingKeeper.GetEpoch(ctx).GetLastBlockHeight()

	// Emit the event for the edit validator transaction
	if err = p.EmitWrappedEditValidatorEvent(ctx, stateDB, msg, validatorHexAddr, epochBoundary); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}

func (p Precompile) WrappedDelegateBech32(
	ctx sdk.Context,
	contract *vm.Contract,
	stateDB vm.StateDB,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	bondDenom, err := p.stakingKeeper.BondDenom(ctx)
	if err != nil {
		return nil, err
	}
	msg, delegatorHexAddr, err := NewMsgWrappedDelegateBech32(args, bondDenom, p.addrCdc)
	if err != nil {
		return nil, err
	}

	p.Logger(ctx).Debug(
		"tx called",
		"method", method.Name,
		"args", fmt.Sprintf(
			"{ delegator_address: %s, validator_address: %s, amount: %s }",
			delegatorHexAddr,
			msg.Msg.ValidatorAddress,
			msg.Msg.Amount.Amount,
		),
	)

	msgSender := contract.Caller()
	if msgSender != delegatorHexAddr {
		return nil, fmt.Errorf(cmn.ErrRequesterIsNotMsgSender, msgSender.String(), delegatorHexAddr.String())
	}

	if _, err = p.epochingMsgServer.WrappedDelegate(ctx, msg); err != nil {
		return nil, err
	}

	epochBoundary := p.epochingKeeper.GetEpoch(ctx).GetLastBlockHeight()

	// Emit the event for the delegate transaction
	if err = p.EmitWrappedDelegateEvent(ctx, stateDB, msg, delegatorHexAddr, epochBoundary); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}

func (p Precompile) WrappedDelegate(
	ctx sdk.Context,
	contract *vm.Contract,
	stateDB vm.StateDB,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	bondDenom, err := p.stakingKeeper.BondDenom(ctx)
	if err != nil {
		return nil, err
	}
	msg, delegatorHexAddr, err := NewMsgWrappedDelegate(args, bondDenom, p.addrCdc, p.valCdc)
	if err != nil {
		return nil, err
	}

	p.Logger(ctx).Debug(
		"tx called",
		"method", method.Name,
		"args", fmt.Sprintf(
			"{ delegator_address: %s, validator_address: %s, amount: %s }",
			delegatorHexAddr,
			msg.Msg.ValidatorAddress,
			msg.Msg.Amount.Amount,
		),
	)

	msgSender := contract.Caller()
	if msgSender != delegatorHexAddr {
		return nil, fmt.Errorf(cmn.ErrRequesterIsNotMsgSender, msgSender.String(), delegatorHexAddr.String())
	}

	if _, err = p.epochingMsgServer.WrappedDelegate(ctx, msg); err != nil {
		return nil, err
	}

	epochBoundary := p.epochingKeeper.GetEpoch(ctx).GetLastBlockHeight()

	// Emit the event for the delegate transaction
	if err = p.EmitWrappedDelegateEvent(ctx, stateDB, msg, delegatorHexAddr, epochBoundary); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}

func (p Precompile) WrappedUndelegateBech32(
	ctx sdk.Context,
	contract *vm.Contract,
	stateDB vm.StateDB,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	bondDenom, err := p.stakingKeeper.BondDenom(ctx)
	if err != nil {
		return nil, err
	}
	msg, delegatorHexAddr, err := NewMsgWrappedUndelegateBech32(args, bondDenom, p.addrCdc)
	if err != nil {
		return nil, err
	}

	p.Logger(ctx).Debug(
		"tx called",
		"method", method.Name,
		"args", fmt.Sprintf(
			"{ delegator_address: %s, validator_address: %s, amount: %s }",
			delegatorHexAddr,
			msg.Msg.ValidatorAddress,
			msg.Msg.Amount.Amount,
		),
	)

	msgSender := contract.Caller()
	if msgSender != delegatorHexAddr {
		return nil, fmt.Errorf(cmn.ErrRequesterIsNotMsgSender, msgSender.String(), delegatorHexAddr.String())
	}

	if _, err = p.epochingMsgServer.WrappedUndelegate(ctx, msg); err != nil {
		return nil, err
	}

	epochBoundary := p.epochingKeeper.GetEpoch(ctx).GetLastBlockHeight()

	// Emit the event for the delegate transaction
	if err = p.EmitWrappedUnbondEvent(ctx, stateDB, msg, delegatorHexAddr, epochBoundary); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}

func (p Precompile) WrappedUndelegate(
	ctx sdk.Context,
	contract *vm.Contract,
	stateDB vm.StateDB,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	bondDenom, err := p.stakingKeeper.BondDenom(ctx)
	if err != nil {
		return nil, err
	}
	msg, delegatorHexAddr, err := NewMsgWrappedUndelegate(args, bondDenom, p.addrCdc, p.valCdc)
	if err != nil {
		return nil, err
	}

	p.Logger(ctx).Debug(
		"tx called",
		"method", method.Name,
		"args", fmt.Sprintf(
			"{ delegator_address: %s, validator_address: %s, amount: %s }",
			delegatorHexAddr,
			msg.Msg.ValidatorAddress,
			msg.Msg.Amount.Amount,
		),
	)

	msgSender := contract.Caller()
	if msgSender != delegatorHexAddr {
		return nil, fmt.Errorf(cmn.ErrRequesterIsNotMsgSender, msgSender.String(), delegatorHexAddr.String())
	}

	if _, err = p.epochingMsgServer.WrappedUndelegate(ctx, msg); err != nil {
		return nil, err
	}

	epochBoundary := p.epochingKeeper.GetEpoch(ctx).GetLastBlockHeight()

	// Emit the event for the delegate transaction
	if err = p.EmitWrappedUnbondEvent(ctx, stateDB, msg, delegatorHexAddr, epochBoundary); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}

func (p Precompile) WrappedRedelegateBech32(
	ctx sdk.Context,
	contract *vm.Contract,
	stateDB vm.StateDB,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	bondDenom, err := p.stakingKeeper.BondDenom(ctx)
	if err != nil {
		return nil, err
	}
	msg, delegatorHexAddr, err := NewMsgWrappedRedelegateBech32(args, bondDenom, p.addrCdc)
	if err != nil {
		return nil, err
	}

	p.Logger(ctx).Debug(
		"tx called",
		"method", method.Name,
		"args", fmt.Sprintf(
			"{ delegator_address: %s, validator_src_address: %s, validator_dst_address: %s, amount: %s }",
			delegatorHexAddr,
			msg.Msg.ValidatorSrcAddress,
			msg.Msg.ValidatorDstAddress,
			msg.Msg.Amount.Amount,
		),
	)

	msgSender := contract.Caller()
	if msgSender != delegatorHexAddr {
		return nil, fmt.Errorf(cmn.ErrRequesterIsNotMsgSender, msgSender.String(), delegatorHexAddr.String())
	}

	if _, err = p.epochingMsgServer.WrappedBeginRedelegate(ctx, msg); err != nil {
		return nil, err
	}

	epochBoundary := p.epochingKeeper.GetEpoch(ctx).GetLastBlockHeight()

	// Emit the event for the delegate transaction
	if err = p.EmitWrappedRedelegateEvent(ctx, stateDB, msg, delegatorHexAddr, epochBoundary); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}

func (p Precompile) WrappedRedelegate(
	ctx sdk.Context,
	contract *vm.Contract,
	stateDB vm.StateDB,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	bondDenom, err := p.stakingKeeper.BondDenom(ctx)
	if err != nil {
		return nil, err
	}
	msg, delegatorHexAddr, err := NewMsgWrappedRedelegate(args, bondDenom, p.addrCdc, p.valCdc)
	if err != nil {
		return nil, err
	}

	p.Logger(ctx).Debug(
		"tx called",
		"method", method.Name,
		"args", fmt.Sprintf(
			"{ delegator_address: %s, validator_src_address: %s, validator_dst_address: %s, amount: %s }",
			delegatorHexAddr,
			msg.Msg.ValidatorSrcAddress,
			msg.Msg.ValidatorDstAddress,
			msg.Msg.Amount.Amount,
		),
	)

	msgSender := contract.Caller()
	if msgSender != delegatorHexAddr {
		return nil, fmt.Errorf(cmn.ErrRequesterIsNotMsgSender, msgSender.String(), delegatorHexAddr.String())
	}

	if _, err = p.epochingMsgServer.WrappedBeginRedelegate(ctx, msg); err != nil {
		return nil, err
	}

	epochBoundary := p.epochingKeeper.GetEpoch(ctx).GetLastBlockHeight()

	// Emit the event for the delegate transaction
	if err = p.EmitWrappedRedelegateEvent(ctx, stateDB, msg, delegatorHexAddr, epochBoundary); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}

func (p Precompile) WrappedCancelUnbondingDelegationBech32(
	ctx sdk.Context,
	contract *vm.Contract,
	stateDB vm.StateDB,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	bondDenom, err := p.stakingKeeper.BondDenom(ctx)
	if err != nil {
		return nil, err
	}
	msg, delegatorHexAddr, err := NewMsgWrappedCancelUnbondingDelegationBech32(args, bondDenom, p.addrCdc)
	if err != nil {
		return nil, err
	}

	p.Logger(ctx).Debug(
		"tx called",
		"method", method.Name,
		"args", fmt.Sprintf(
			"{ delegator_address: %s, validator_address: %s, amount: %s, creation_height: %d }",
			delegatorHexAddr,
			msg.Msg.ValidatorAddress,
			msg.Msg.Amount.Amount,
			msg.Msg.CreationHeight,
		),
	)

	msgSender := contract.Caller()
	if msgSender != delegatorHexAddr {
		return nil, fmt.Errorf(cmn.ErrRequesterIsNotMsgSender, msgSender.String(), delegatorHexAddr.String())
	}

	if _, err = p.epochingMsgServer.WrappedCancelUnbondingDelegation(ctx, msg); err != nil {
		return nil, err
	}

	epochBoundary := p.epochingKeeper.GetEpoch(ctx).GetLastBlockHeight()

	// Emit the event for the delegate transaction
	if err = p.EmitWrappedCancelUnbondingDelegationEvent(ctx, stateDB, msg, delegatorHexAddr, epochBoundary); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}

func (p Precompile) WrappedCancelUnbondingDelegation(
	ctx sdk.Context,
	contract *vm.Contract,
	stateDB vm.StateDB,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	bondDenom, err := p.stakingKeeper.BondDenom(ctx)
	if err != nil {
		return nil, err
	}
	msg, delegatorHexAddr, err := NewMsgWrappedCancelUnbondingDelegation(args, bondDenom, p.addrCdc, p.valCdc)
	if err != nil {
		return nil, err
	}

	p.Logger(ctx).Debug(
		"tx called",
		"method", method.Name,
		"args", fmt.Sprintf(
			"{ delegator_address: %s, validator_address: %s, amount: %s, creation_height: %d }",
			delegatorHexAddr,
			msg.Msg.ValidatorAddress,
			msg.Msg.Amount.Amount,
			msg.Msg.CreationHeight,
		),
	)

	msgSender := contract.Caller()
	if msgSender != delegatorHexAddr {
		return nil, fmt.Errorf(cmn.ErrRequesterIsNotMsgSender, msgSender.String(), delegatorHexAddr.String())
	}

	if _, err = p.epochingMsgServer.WrappedCancelUnbondingDelegation(ctx, msg); err != nil {
		return nil, err
	}

	epochBoundary := p.epochingKeeper.GetEpoch(ctx).GetLastBlockHeight()

	// Emit the event for the delegate transaction
	if err = p.EmitWrappedCancelUnbondingDelegationEvent(ctx, stateDB, msg, delegatorHexAddr, epochBoundary); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}
