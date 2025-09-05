package epoching

import (
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/core/vm"

	cmn "github.com/cosmos/evm/precompiles/common"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	// staking precompile queries
	DelegationMethod          = "delegation"
	UnbondingDelegationMethod = "unbondingDelegation"
	ValidatorMethod           = "validator"
	ValidatorsMethod          = "validators"
	RedelegationMethod        = "redelegation"
	RedelegationsMethod       = "redelegations"

	// bech32 versions of staking queries
	ValidatorBech32Method     = "validatorBech32"
	ValidatorsBech32Method    = "validatorsBech32"
	RedelegationBech32Method  = "redelegationBech32"
	RedelegationsBech32Method = "redelegationsBech32"

	// epoching precompile queries
	EpochInfoMethod           = "epochInfo"
	CurrentEpochMethod        = "currentEpoch"
	EpochMsgsMethod           = "epochMsgs"
	LatestEpochMsgsMethod     = "latestEpochMsgs"
	ValidatorLifecycleMethod  = "validatorLifecycle"
	DelegationLifecycleMethod = "delegationLifecycle"
	EpochValSetMethod         = "epochValSet"

	// bech32 versions of epoching queries
	DelegationLifecycleBech32Method = "delegationLifecycleBech32"
)

// DelegationBech32 returns the delegation that a delegator has with a specific validator.
func (p Precompile) DelegationBech32(
	ctx sdk.Context,
	_ *vm.Contract,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	req, err := NewDelegationBech32Request(args, p.addrCdc)
	if err != nil {
		return nil, err
	}

	res, err := p.stakingQuerier.Delegation(ctx, req)
	if err != nil {
		// If there is no delegation found, return the response with zero values.
		if strings.Contains(err.Error(), fmt.Sprintf(ErrNoDelegationFound, req.DelegatorAddr, req.ValidatorAddr)) {
			bondDenom, err := p.stakingKeeper.BondDenom(ctx)
			if err != nil {
				return nil, err
			}
			return method.Outputs.Pack(big.NewInt(0), cmn.Coin{Denom: bondDenom, Amount: big.NewInt(0)})
		}

		return nil, err
	}

	out := new(DelegationOutput).FromResponse(res)

	return out.Pack(method.Outputs)
}

// Delegation returns the delegation that a delegator has with a specific validator.
func (p Precompile) Delegation(
	ctx sdk.Context,
	_ *vm.Contract,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	req, err := NewDelegationRequest(args, p.addrCdc)
	if err != nil {
		return nil, err
	}

	res, err := p.stakingQuerier.Delegation(ctx, req)
	if err != nil {
		// If there is no delegation found, return the response with zero values.
		if strings.Contains(err.Error(), fmt.Sprintf(ErrNoDelegationFound, req.DelegatorAddr, req.ValidatorAddr)) {
			bondDenom, err := p.stakingKeeper.BondDenom(ctx)
			if err != nil {
				return nil, err
			}
			return method.Outputs.Pack(big.NewInt(0), cmn.Coin{Denom: bondDenom, Amount: big.NewInt(0)})
		}

		return nil, err
	}

	out := new(DelegationOutput).FromResponse(res)

	return out.Pack(method.Outputs)
}

// UnbondingDelegationBech32 returns the delegation currently being unbonded for a delegator from
// a specific validator.
func (p Precompile) UnbondingDelegationBech32(
	ctx sdk.Context,
	_ *vm.Contract,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	req, err := NewUnbondingDelegationBech32Request(args, p.addrCdc)
	if err != nil {
		return nil, err
	}

	res, err := p.stakingQuerier.UnbondingDelegation(ctx, req)
	if err != nil {
		// return empty unbonding delegation output if the unbonding delegation is not found
		expError := fmt.Sprintf("unbonding delegation with delegator %s not found for validator %s", req.DelegatorAddr, req.ValidatorAddr)
		if strings.Contains(err.Error(), expError) {
			return method.Outputs.Pack(UnbondingDelegationResponse{})
		}
		return nil, err
	}

	out := new(UnbondingDelegationOutputBech32).FromResponse(res)

	return method.Outputs.Pack(out.UnbondingDelegation)
}

// UnbondingDelegation returns the delegation currently being unbonded for a delegator from
// a specific validator.
func (p Precompile) UnbondingDelegation(
	ctx sdk.Context,
	_ *vm.Contract,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	req, err := NewUnbondingDelegationRequest(args, p.addrCdc, p.valCodec)
	if err != nil {
		return nil, err
	}

	res, err := p.stakingQuerier.UnbondingDelegation(ctx, req)
	if err != nil {
		// return empty unbonding delegation output if the unbonding delegation is not found
		expError := fmt.Sprintf("unbonding delegation with delegator %s not found for validator %s", req.DelegatorAddr, req.ValidatorAddr)
		if strings.Contains(err.Error(), expError) {
			return method.Outputs.Pack(UnbondingDelegationResponse{})
		}
		return nil, err
	}

	out := new(UnbondingDelegationOutput).FromResponse(res, p.addrCdc, p.valCodec)

	return method.Outputs.Pack(out.UnbondingDelegation)
}

// ValidatorBech32 returns the validator information for a given validator address.
func (p Precompile) ValidatorBech32(
	ctx sdk.Context,
	method *abi.Method,
	_ *vm.Contract,
	args []interface{},
) ([]byte, error) {
	req, err := NewValidatorRequest(args)
	if err != nil {
		return nil, err
	}

	res, err := p.stakingQuerier.Validator(ctx, req)
	if err != nil {
		// return empty validator info if the validator is not found
		expError := fmt.Sprintf("validator %s not found", req.ValidatorAddr)
		if strings.Contains(err.Error(), expError) {
			return method.Outputs.Pack(DefaultValidatorOutputBech32().Validator)
		}
		return nil, err
	}

	out := new(ValidatorOutputBech32).FromResponse(res)

	return method.Outputs.Pack(out.Validator)
}

// Validator returns the validator information for a given validator address.
func (p Precompile) Validator(
	ctx sdk.Context,
	method *abi.Method,
	_ *vm.Contract,
	args []interface{},
) ([]byte, error) {
	req, err := NewValidatorRequest(args)
	if err != nil {
		return nil, err
	}

	res, err := p.stakingQuerier.Validator(ctx, req)
	if err != nil {
		// return empty validator info if the validator is not found
		expError := fmt.Sprintf("validator %s not found", req.ValidatorAddr)
		if strings.Contains(err.Error(), expError) {
			return method.Outputs.Pack(DefaultValidatorOutput().Validator)
		}
		return nil, err
	}

	out := new(ValidatorOutput).FromResponse(res)

	return method.Outputs.Pack(out.Validator)
}

// ValidatorsBech32 returns the validators information with a provided status & pagination (optional).
func (p Precompile) ValidatorsBech32(
	ctx sdk.Context,
	method *abi.Method,
	_ *vm.Contract,
	args []interface{},
) ([]byte, error) {
	req, err := NewValidatorsRequest(method, args)
	if err != nil {
		return nil, err
	}

	res, err := p.stakingQuerier.Validators(ctx, req)
	if err != nil {
		return nil, err
	}

	out := new(ValidatorsOutputBech32).FromResponse(res)

	return out.Pack(method.Outputs)
}

// Validators returns the validators information with a provided status & pagination (optional).
func (p Precompile) Validators(
	ctx sdk.Context,
	method *abi.Method,
	_ *vm.Contract,
	args []interface{},
) ([]byte, error) {
	req, err := NewValidatorsRequest(method, args)
	if err != nil {
		return nil, err
	}

	res, err := p.stakingQuerier.Validators(ctx, req)
	if err != nil {
		return nil, err
	}

	out := new(ValidatorsOutput).FromResponse(res)

	return out.Pack(method.Outputs)
}

// RedelegationBech32 returns the redelegation between two validators for a delegator.
func (p Precompile) RedelegationBech32(
	ctx sdk.Context,
	method *abi.Method,
	_ *vm.Contract,
	args []interface{},
) ([]byte, error) {
	req, err := NewRedelegationBech32Request(args)
	if err != nil {
		return nil, err
	}

	res, _ := p.stakingKeeper.GetRedelegation(ctx, req.DelegatorAddress, req.ValidatorSrcAddress, req.ValidatorDstAddress)

	out := new(RedelegationOutputBech32).FromResponse(res)

	return method.Outputs.Pack(out.Redelegation)
}

// Redelegation returns the redelegation between two validators for a delegator.
func (p Precompile) Redelegation(
	ctx sdk.Context,
	method *abi.Method,
	_ *vm.Contract,
	args []interface{},
) ([]byte, error) {
	req, err := NewRedelegationRequest(args)
	if err != nil {
		return nil, err
	}

	res, _ := p.stakingKeeper.GetRedelegation(ctx, req.DelegatorAddress, req.ValidatorSrcAddress, req.ValidatorDstAddress)

	out := new(RedelegationOutput).FromResponse(res)

	return method.Outputs.Pack(out.Redelegation)
}

// RedelegationsBech32 returns the redelegations according to
// the specified criteria (delegator address and/or validator source address
// and/or validator destination address or all existing redelegations) with pagination.
// Pagination is only supported for querying redelegations from a source validator or to query all redelegations.
func (p Precompile) RedelegationsBech32(
	ctx sdk.Context,
	method *abi.Method,
	_ *vm.Contract,
	args []interface{},
) ([]byte, error) {
	req, err := NewRedelegationsBech32Request(method, args, p.addrCdc)
	if err != nil {
		return nil, err
	}

	res, err := p.stakingQuerier.Redelegations(ctx, req)
	if err != nil {
		return nil, err
	}

	out := new(RedelegationsOutputBech32).FromResponse(res)

	return out.Pack(method.Outputs)
}

// Redelegations returns the redelegations according to
// the specified criteria (delegator address and/or validator source address
// and/or validator destination address or all existing redelegations) with pagination.
// Pagination is only supported for querying redelegations from a source validator or to query all redelegations.
func (p Precompile) Redelegations(
	ctx sdk.Context,
	method *abi.Method,
	_ *vm.Contract,
	args []interface{},
) ([]byte, error) {
	req, err := NewRedelegationsRequest(method, args, p.addrCdc)
	if err != nil {
		return nil, err
	}

	res, err := p.stakingQuerier.Redelegations(ctx, req)
	if err != nil {
		return nil, err
	}

	out := new(RedelegationsOutput).FromResponse(res)

	return out.Pack(method.Outputs)
}

func (p Precompile) EpochInfo(
	ctx sdk.Context,
	_ *vm.Contract,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	req, err := NewEpochInfoRequest(args)
	if err != nil {
		return nil, err
	}

	res, err := p.epochingQuerier.EpochInfo(ctx, req)
	if err != nil {
		return nil, err
	}

	out := new(EpochInfoOutput).FromResponse(res)

	return method.Outputs.Pack(out.Epoch)
}

func (p Precompile) CurrentEpoch(
	ctx sdk.Context,
	_ *vm.Contract,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	req, err := NewCurrentEpochRequest(args)
	if err != nil {
		return nil, err
	}

	res, err := p.epochingQuerier.CurrentEpoch(ctx, req)
	if err != nil {
		return nil, err
	}

	out := new(CurrentEpochOutput).FromResponse(res)

	return method.Outputs.Pack(out.Response)
}

func (p Precompile) EpochMsgs(
	ctx sdk.Context,
	_ *vm.Contract,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	req, err := NewEpochMsgsRequest(method, args)
	if err != nil {
		return nil, err
	}

	res, err := p.epochingQuerier.EpochMsgs(ctx, req)
	if err != nil {
		return nil, err
	}

	out := new(EpochMsgsOutput).FromResponse(res)

	return out.Pack(method.Outputs)
}

func (p Precompile) LatestEpochMsgs(
	ctx sdk.Context,
	_ *vm.Contract,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	req, err := NewLatestEpochMsgsRequest(method, args)
	if err != nil {
		return nil, err
	}

	res, err := p.epochingQuerier.LatestEpochMsgs(ctx, req)
	if err != nil {
		return nil, err
	}

	out := new(LatestEpochMsgsOutput).FromResponse(res)

	return out.Pack(method.Outputs)
}

func (p Precompile) ValidatorLifecycle(
	ctx sdk.Context,
	_ *vm.Contract,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	req, err := NewValidatorLifecycleRequest(args, p.valCodec)
	if err != nil {
		return nil, err
	}

	res, err := p.epochingQuerier.ValidatorLifecycle(ctx, req)
	if err != nil {
		return nil, err
	}

	out := new(ValidatorLifecycleOutput).FromResponse(res)

	return method.Outputs.Pack(out.ValidatorLife)
}

func (p Precompile) DelegationLifecycleBech32(
	ctx sdk.Context,
	_ *vm.Contract,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	req, err := NewDelegationLifecycleRequest(args, p.addrCdc)
	if err != nil {
		return nil, err
	}

	res, err := p.epochingQuerier.DelegationLifecycle(ctx, req)
	if err != nil {
		return nil, err
	}

	out := new(DelegationLifecycleOutputBech32).FromResponse(res)

	return method.Outputs.Pack(out.DelegationLifecycle)
}

func (p Precompile) DelegationLifecycle(
	ctx sdk.Context,
	_ *vm.Contract,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	req, err := NewDelegationLifecycleRequest(args, p.addrCdc)
	if err != nil {
		return nil, err
	}

	res, err := p.epochingQuerier.DelegationLifecycle(ctx, req)
	if err != nil {
		return nil, err
	}

	out := new(DelegationLifecycleOutput).FromResponse(res)

	return method.Outputs.Pack(out.DelegationLifecycle)
}

func (p Precompile) EpochValSet(
	ctx sdk.Context,
	_ *vm.Contract,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	req, err := NewEpochValSetRequest(method, args)
	if err != nil {
		return nil, err
	}

	res, err := p.epochingQuerier.EpochValSet(ctx, req)
	if err != nil {
		return nil, err
	}

	out := new(EpochValSetOutput).FromResponse(res)

	return out.Pack(method.Outputs)
}
