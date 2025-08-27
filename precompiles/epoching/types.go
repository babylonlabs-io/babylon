package epoching

import (
	"bytes"
	"cosmossdk.io/math"
	"encoding/base64"
	"errors"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"

	cmn "github.com/cosmos/evm/precompiles/common"

	"cosmossdk.io/core/address"

	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"

	"github.com/babylonlabs-io/babylon/v4/crypto/bls12381"
	checkpointingtypes "github.com/babylonlabs-io/babylon/v4/x/checkpointing/types"
	epochingtypes "github.com/babylonlabs-io/babylon/v4/x/epoching/types"
)

const (
	// DoNotModifyCommissionRate constant used in flags to indicate that commission rate field should not be updated
	DoNotModifyCommissionRate = -1
	// DoNotModifyMinSelfDelegation constant used in flags to indicate that min self delegation field should not be updated
	DoNotModifyMinSelfDelegation = -1
)

const (
	EpochingPrecompileAddress = "0x0000000000000000000000000000000000001000"
)

const (
	ErrInvalidBlsKey = "invalid bls key %v"
)

// EventCreateValidator defines the event data for the staking CreateValidator transaction.
type EventCreateValidator struct {
	ValidatorAddress common.Address
	Value            *big.Int
}

// EventEditValidator defines the event data for the staking EditValidator transaction.
type EventEditValidator struct {
	ValidatorAddress  common.Address
	CommissionRate    *big.Int
	MinSelfDelegation *big.Int
}

// EventDelegate defines the event data for the staking Delegate transaction.
type EventDelegate struct {
	DelegatorAddress common.Address
	ValidatorAddress common.Address
	Amount           *big.Int
	NewShares        *big.Int
}

// EventUnbond defines the event data for the staking Undelegate transaction.
type EventUnbond struct {
	DelegatorAddress common.Address
	ValidatorAddress common.Address
	Amount           *big.Int
	CompletionTime   *big.Int
}

// EventRedelegate defines the event data for the staking Redelegate transaction.
type EventRedelegate struct {
	DelegatorAddress    common.Address
	ValidatorSrcAddress common.Address
	ValidatorDstAddress common.Address
	Amount              *big.Int
	CompletionTime      *big.Int
}

// EventCancelUnbonding defines the event data for the staking CancelUnbond transaction.
type EventCancelUnbonding struct {
	DelegatorAddress common.Address
	ValidatorAddress common.Address
	Amount           *big.Int
	CreationHeight   *big.Int
}

type BlsKey = struct {
	Pubkey     bls12381.PublicKey "json:\"pubKey\""
	Ed25519Sig []byte             "json:\"ed25519Sig\""
	BlsSig     bls12381.Signature "json:\"blsSig\""
}

// Description use golang type alias defines a validator description.
type Description = struct {
	Moniker         string "json:\"moniker\""
	Identity        string "json:\"identity\""
	Website         string "json:\"website\""
	SecurityContact string "json:\"securityContact\""
	Details         string "json:\"details\""
}

// Commission use golang type alias defines a validator commission.
// since solidity does not support decimals, after passing in the big int, convert the big int into a decimal with a precision of 18
type Commission = struct {
	Rate          *big.Int "json:\"rate\""
	MaxRate       *big.Int "json:\"maxRate\""
	MaxChangeRate *big.Int "json:\"maxChangeRate\""
}

// NewMsgWrappedCreateValidator creates a new MsgWrappedCreateValidator instance and does sanity checks
// on the given arguments before populating the message.
func NewMsgWrappedCreateValidator(args []interface{}, denom string, addrCdc address.Codec) (*checkpointingtypes.MsgWrappedCreateValidator, common.Address, error) {
	if len(args) != 7 {
		return nil, common.Address{}, fmt.Errorf(cmn.ErrInvalidNumberOfArgs, 7, len(args))
	}

	blsKey, ok := args[0].(*BlsKey)
	if !ok {
		return nil, common.Address{}, fmt.Errorf(ErrInvalidBlsKey, args[0])
	}

	description, ok := args[1].(Description)
	if !ok {
		return nil, common.Address{}, fmt.Errorf(cmn.ErrInvalidDescription, args[1])
	}

	commission, ok := args[2].(Commission)
	if !ok {
		return nil, common.Address{}, fmt.Errorf(cmn.ErrInvalidCommission, args[2])
	}

	minSelfDelegation, ok := args[3].(*big.Int)
	if !ok {
		return nil, common.Address{}, fmt.Errorf(cmn.ErrInvalidAmount, args[3])
	}

	validatorAddress, ok := args[4].(common.Address)
	if !ok || validatorAddress == (common.Address{}) {
		return nil, common.Address{}, fmt.Errorf(cmn.ErrInvalidValidator, args[4])
	}

	// use cli `evmd comet show-validator` get pubkey
	pubkeyBase64Str, ok := args[5].(string)
	if !ok {
		return nil, common.Address{}, fmt.Errorf(cmn.ErrInvalidType, "pubkey", "string", args[5])
	}
	pubkeyBytes, err := base64.StdEncoding.DecodeString(pubkeyBase64Str)
	if err != nil {
		return nil, common.Address{}, err
	}

	// more details see https://github.com/cosmos/cosmos-sdk/pull/18506
	if len(pubkeyBytes) != ed25519.PubKeySize {
		return nil, common.Address{}, fmt.Errorf("consensus pubkey len is invalid, got: %d, expected: %d", len(pubkeyBytes), ed25519.PubKeySize)
	}

	var ed25519pk cryptotypes.PubKey = &ed25519.PubKey{Key: pubkeyBytes}
	pubkey, err := codectypes.NewAnyWithValue(ed25519pk)
	if err != nil {
		return nil, common.Address{}, err
	}

	value, ok := args[6].(*big.Int)
	if !ok {
		return nil, common.Address{}, fmt.Errorf(cmn.ErrInvalidAmount, args[6])
	}

	delegatorAddr, err := addrCdc.BytesToString(validatorAddress.Bytes())
	if err != nil {
		return nil, common.Address{}, fmt.Errorf("failed to decode delegator address: %w", err)
	}
	msg := &checkpointingtypes.MsgWrappedCreateValidator{
		Key: &checkpointingtypes.BlsKey{
			Pubkey: &blsKey.Pubkey,
			Pop: &checkpointingtypes.ProofOfPossession{
				Ed25519Sig: blsKey.Ed25519Sig,
				BlsSig:     &blsKey.BlsSig,
			},
		},
		MsgCreateValidator: &stakingtypes.MsgCreateValidator{
			Description: stakingtypes.Description{
				Moniker:         description.Moniker,
				Identity:        description.Identity,
				Website:         description.Website,
				SecurityContact: description.SecurityContact,
				Details:         description.Details,
			},
			Commission: stakingtypes.CommissionRates{
				Rate:          math.LegacyNewDecFromBigIntWithPrec(commission.Rate, math.LegacyPrecision),
				MaxRate:       math.LegacyNewDecFromBigIntWithPrec(commission.MaxRate, math.LegacyPrecision),
				MaxChangeRate: math.LegacyNewDecFromBigIntWithPrec(commission.MaxChangeRate, math.LegacyPrecision),
			},
			MinSelfDelegation: math.NewIntFromBigInt(minSelfDelegation),
			DelegatorAddress:  delegatorAddr,
			ValidatorAddress:  sdk.ValAddress(validatorAddress.Bytes()).String(),
			Pubkey:            pubkey,
			Value:             sdk.Coin{Denom: denom, Amount: math.NewIntFromBigInt(value)},
		},
	}

	return msg, validatorAddress, nil
}

// NewMsgWrappedEditValidator creates a new MsgEditValidator instance and does sanity checks
// on the given arguments before populating the message.
func NewMsgWrappedEditValidator(args []interface{}) (*epochingtypes.MsgWrappedEditValidator, common.Address, error) {
	if len(args) != 4 {
		return nil, common.Address{}, fmt.Errorf(cmn.ErrInvalidNumberOfArgs, 4, len(args))
	}

	description, ok := args[0].(Description)
	if !ok {
		return nil, common.Address{}, fmt.Errorf(cmn.ErrInvalidDescription, args[0])
	}

	validatorHexAddr, ok := args[1].(common.Address)
	if !ok || validatorHexAddr == (common.Address{}) {
		return nil, common.Address{}, fmt.Errorf(cmn.ErrInvalidValidator, args[1])
	}

	commissionRateBigInt, ok := args[2].(*big.Int)
	if !ok {
		return nil, common.Address{}, fmt.Errorf(cmn.ErrInvalidType, "commissionRate", &big.Int{}, args[2])
	}

	// The default value of a variable declared using a pointer is nil, indicating that the user does not want to modify its value.
	// If the value passed in by the user is not DoNotModifyCommissionRate, which is -1, it means that the user wants to modify its value.
	var commissionRate *math.LegacyDec
	if commissionRateBigInt.Cmp(big.NewInt(DoNotModifyCommissionRate)) != 0 {
		cr := math.LegacyNewDecFromBigIntWithPrec(commissionRateBigInt, math.LegacyPrecision)
		commissionRate = &cr
	}

	minSelfDelegationBigInt, ok := args[3].(*big.Int)
	if !ok {
		return nil, common.Address{}, fmt.Errorf(cmn.ErrInvalidType, "minSelfDelegation", &big.Int{}, args[3])
	}

	var minSelfDelegation *math.Int
	if minSelfDelegationBigInt.Cmp(big.NewInt(DoNotModifyMinSelfDelegation)) != 0 {
		msd := math.NewIntFromBigInt(minSelfDelegationBigInt)
		minSelfDelegation = &msd
	}

	msg := &epochingtypes.MsgWrappedEditValidator{
		Msg: &stakingtypes.MsgEditValidator{
			Description: stakingtypes.Description{
				Moniker:         description.Moniker,
				Identity:        description.Identity,
				Website:         description.Website,
				SecurityContact: description.SecurityContact,
				Details:         description.Details,
			},
			ValidatorAddress:  sdk.ValAddress(validatorHexAddr.Bytes()).String(),
			CommissionRate:    commissionRate,
			MinSelfDelegation: minSelfDelegation,
		},
	}

	return msg, validatorHexAddr, nil
}

// NewMsgWrappedDelegateBech32 creates a new MsgDelegate instance and does sanity checks
// on the given arguments before populating the message.
func NewMsgWrappedDelegateBech32(args []interface{}, denom string, addrCdc address.Codec) (*epochingtypes.MsgWrappedDelegate, common.Address, error) {
	delegatorAddr, validatorAddress, amount, err := checkDelegationUndelegationArgsBech32(args)
	if err != nil {
		return nil, common.Address{}, err
	}

	delegatorAddrStr, err := addrCdc.BytesToString(delegatorAddr.Bytes())
	if err != nil {
		return nil, common.Address{}, fmt.Errorf("failed to decode delegator address: %w", err)
	}
	msg := &epochingtypes.MsgWrappedDelegate{
		Msg: &stakingtypes.MsgDelegate{
			DelegatorAddress: delegatorAddrStr,
			ValidatorAddress: validatorAddress,
			Amount: sdk.Coin{
				Denom:  denom,
				Amount: math.NewIntFromBigInt(amount),
			},
		},
	}

	return msg, delegatorAddr, nil
}

// NewMsgWrappedDelegate creates a new MsgDelegate instance and does sanity checks
// on the given arguments before populating the message.
func NewMsgWrappedDelegate(args []interface{}, denom string, addrCdc address.Codec) (*epochingtypes.MsgWrappedDelegate, common.Address, error) {
	delegatorAddr, validatorAddr, amount, err := checkDelegationUndelegationArgs(args)
	if err != nil {
		return nil, common.Address{}, err
	}

	delegatorAddrStr, err := addrCdc.BytesToString(delegatorAddr.Bytes())
	validatorAddrStr := sdk.ValAddress(validatorAddr.Bytes()).String()
	if err != nil {
		return nil, common.Address{}, fmt.Errorf("failed to decode delegator address: %w", err)
	}
	msg := &epochingtypes.MsgWrappedDelegate{
		Msg: &stakingtypes.MsgDelegate{
			DelegatorAddress: delegatorAddrStr,
			ValidatorAddress: validatorAddrStr,
			Amount: sdk.Coin{
				Denom:  denom,
				Amount: math.NewIntFromBigInt(amount),
			},
		},
	}

	return msg, delegatorAddr, nil
}

// NewMsgWrappedUndelegateBech32 creates a new MsgUndelegate instance and does sanity checks
// on the given arguments before populating the message.
func NewMsgWrappedUndelegateBech32(args []interface{}, denom string, addrCdc address.Codec) (*epochingtypes.MsgWrappedUndelegate, common.Address, error) {
	delegatorAddr, validatorAddress, amount, err := checkDelegationUndelegationArgsBech32(args)
	if err != nil {
		return nil, common.Address{}, err
	}

	delegatorAddrStr, err := addrCdc.BytesToString(delegatorAddr.Bytes())
	if err != nil {
		return nil, common.Address{}, fmt.Errorf("failed to decode delegator address: %w", err)
	}
	msg := &epochingtypes.MsgWrappedUndelegate{
		Msg: &stakingtypes.MsgUndelegate{
			DelegatorAddress: delegatorAddrStr,
			ValidatorAddress: validatorAddress,
			Amount: sdk.Coin{
				Denom:  denom,
				Amount: math.NewIntFromBigInt(amount),
			},
		},
	}

	return msg, delegatorAddr, nil
}

// NewMsgWrappedUndelegate creates a new MsgUndelegate instance and does sanity checks
// on the given arguments before populating the message.
func NewMsgWrappedUndelegate(args []interface{}, denom string, addrCdc address.Codec) (*epochingtypes.MsgWrappedUndelegate, common.Address, error) {
	delegatorAddr, validatorAddr, amount, err := checkDelegationUndelegationArgs(args)
	if err != nil {
		return nil, common.Address{}, err
	}

	delegatorAddrStr, err := addrCdc.BytesToString(delegatorAddr.Bytes())
	validatorAddrStr := sdk.ValAddress(validatorAddr.Bytes()).String()
	if err != nil {
		return nil, common.Address{}, fmt.Errorf("failed to decode delegator address: %w", err)
	}
	msg := &epochingtypes.MsgWrappedUndelegate{
		Msg: &stakingtypes.MsgUndelegate{
			DelegatorAddress: delegatorAddrStr,
			ValidatorAddress: validatorAddrStr,
			Amount: sdk.Coin{
				Denom:  denom,
				Amount: math.NewIntFromBigInt(amount),
			},
		},
	}

	return msg, delegatorAddr, nil
}

// NewMsgWrappedRedelegateBech32 creates a new MsgRedelegate instance and does sanity checks
// on the given arguments before populating the message.
func NewMsgWrappedRedelegateBech32(args []interface{}, denom string, addrCdc address.Codec) (*epochingtypes.MsgWrappedBeginRedelegate, common.Address, error) {
	if len(args) != 4 {
		return nil, common.Address{}, fmt.Errorf(cmn.ErrInvalidNumberOfArgs, 4, len(args))
	}

	delegatorAddr, ok := args[0].(common.Address)
	if !ok || delegatorAddr == (common.Address{}) {
		return nil, common.Address{}, fmt.Errorf(cmn.ErrInvalidDelegator, args[0])
	}

	validatorSrcAddress, ok := args[1].(string)
	if !ok {
		return nil, common.Address{}, fmt.Errorf(cmn.ErrInvalidType, "validatorSrcAddress", "string", args[1])
	}

	validatorDstAddress, ok := args[2].(string)
	if !ok {
		return nil, common.Address{}, fmt.Errorf(cmn.ErrInvalidType, "validatorDstAddress", "string", args[2])
	}

	amount, ok := args[3].(*big.Int)
	if !ok {
		return nil, common.Address{}, fmt.Errorf(cmn.ErrInvalidAmount, args[3])
	}

	delegatorAddrStr, err := addrCdc.BytesToString(delegatorAddr.Bytes())
	if err != nil {
		return nil, common.Address{}, fmt.Errorf("failed to decode delegator address: %w", err)
	}
	msg := &epochingtypes.MsgWrappedBeginRedelegate{
		Msg: &stakingtypes.MsgBeginRedelegate{
			DelegatorAddress:    delegatorAddrStr,
			ValidatorSrcAddress: validatorSrcAddress,
			ValidatorDstAddress: validatorDstAddress,
			Amount: sdk.Coin{
				Denom:  denom,
				Amount: math.NewIntFromBigInt(amount),
			},
		},
	}

	return msg, delegatorAddr, nil
}

// NewMsgWrappedRedelegate creates a new MsgRedelegate instance and does sanity checks
// on the given arguments before populating the message.
func NewMsgWrappedRedelegate(args []interface{}, denom string, addrCdc address.Codec) (*epochingtypes.MsgWrappedBeginRedelegate, common.Address, error) {
	if len(args) != 4 {
		return nil, common.Address{}, fmt.Errorf(cmn.ErrInvalidNumberOfArgs, 4, len(args))
	}

	delegatorAddr, ok := args[0].(common.Address)
	if !ok || delegatorAddr == (common.Address{}) {
		return nil, common.Address{}, fmt.Errorf(cmn.ErrInvalidDelegator, args[0])
	}

	validatorSrcAddr, ok := args[1].(common.Address)
	if !ok {
		return nil, common.Address{}, fmt.Errorf(cmn.ErrInvalidType, "validatorSrcAddress", "common.Address", args[1])
	}

	validatorDstAddr, ok := args[2].(common.Address)
	if !ok {
		return nil, common.Address{}, fmt.Errorf(cmn.ErrInvalidType, "validatorDstAddress", "common.Address", args[2])
	}

	amount, ok := args[3].(*big.Int)
	if !ok {
		return nil, common.Address{}, fmt.Errorf(cmn.ErrInvalidAmount, args[3])
	}

	delegatorAddrStr, err := addrCdc.BytesToString(delegatorAddr.Bytes())
	validatorSrcAddrStr := sdk.ValAddress(validatorSrcAddr.Bytes()).String()
	validatorDstAddrStr := sdk.ValAddress(validatorDstAddr.Bytes()).String()
	if err != nil {
		return nil, common.Address{}, fmt.Errorf("failed to decode delegator address: %w", err)
	}
	msg := &epochingtypes.MsgWrappedBeginRedelegate{
		Msg: &stakingtypes.MsgBeginRedelegate{
			DelegatorAddress:    delegatorAddrStr,
			ValidatorSrcAddress: validatorSrcAddrStr,
			ValidatorDstAddress: validatorDstAddrStr,
			Amount: sdk.Coin{
				Denom:  denom,
				Amount: math.NewIntFromBigInt(amount),
			},
		},
	}

	return msg, delegatorAddr, nil
}

// NewMsgWrappedCancelUnbondingDelegationBech32 creates a new MsgCancelUnbondingDelegation instance and does sanity checks
// on the given arguments before populating the message.
func NewMsgWrappedCancelUnbondingDelegationBech32(args []interface{}, denom string, addrCdc address.Codec) (*epochingtypes.MsgWrappedCancelUnbondingDelegation, common.Address, error) {
	if len(args) != 4 {
		return nil, common.Address{}, fmt.Errorf(cmn.ErrInvalidNumberOfArgs, 4, len(args))
	}

	delegatorAddr, ok := args[0].(common.Address)
	if !ok || delegatorAddr == (common.Address{}) {
		return nil, common.Address{}, fmt.Errorf(cmn.ErrInvalidDelegator, args[0])
	}

	validatorAddress, ok := args[1].(string)
	if !ok {
		return nil, common.Address{}, fmt.Errorf(cmn.ErrInvalidType, "validatorAddress", "string", args[1])
	}

	amount, ok := args[2].(*big.Int)
	if !ok {
		return nil, common.Address{}, fmt.Errorf(cmn.ErrInvalidAmount, args[2])
	}

	creationHeight, ok := args[3].(*big.Int)
	if !ok {
		return nil, common.Address{}, fmt.Errorf("invalid creation height")
	}

	delegatorAddrStr, err := addrCdc.BytesToString(delegatorAddr.Bytes())
	if err != nil {
		return nil, common.Address{}, fmt.Errorf("failed to decode delegator address: %w", err)
	}
	msg := &epochingtypes.MsgWrappedCancelUnbondingDelegation{
		Msg: &stakingtypes.MsgCancelUnbondingDelegation{
			DelegatorAddress: delegatorAddrStr,
			ValidatorAddress: validatorAddress,
			Amount: sdk.Coin{
				Denom:  denom,
				Amount: math.NewIntFromBigInt(amount),
			},
			CreationHeight: creationHeight.Int64(),
		},
	}

	return msg, delegatorAddr, nil
}

// NewMsgWrappedCancelUnbondingDelegation creates a new MsgCancelUnbondingDelegation instance and does sanity checks
// on the given arguments before populating the message.
func NewMsgWrappedCancelUnbondingDelegation(args []interface{}, denom string, addrCdc address.Codec) (*epochingtypes.MsgWrappedCancelUnbondingDelegation, common.Address, error) {
	if len(args) != 4 {
		return nil, common.Address{}, fmt.Errorf(cmn.ErrInvalidNumberOfArgs, 4, len(args))
	}

	delegatorAddr, ok := args[0].(common.Address)
	if !ok || delegatorAddr == (common.Address{}) {
		return nil, common.Address{}, fmt.Errorf(cmn.ErrInvalidDelegator, args[0])
	}

	validatorAddr, ok := args[1].(common.Address)
	if !ok {
		return nil, common.Address{}, fmt.Errorf(cmn.ErrInvalidType, "validatorAddr", "common.Address", args[1])
	}

	amount, ok := args[2].(*big.Int)
	if !ok {
		return nil, common.Address{}, fmt.Errorf(cmn.ErrInvalidAmount, args[2])
	}

	creationHeight, ok := args[3].(*big.Int)
	if !ok {
		return nil, common.Address{}, fmt.Errorf("invalid creation height")
	}

	delegatorAddrStr, err := addrCdc.BytesToString(delegatorAddr.Bytes())
	validatorAddrStr := sdk.ValAddress(validatorAddr.Bytes()).String()
	if err != nil {
		return nil, common.Address{}, fmt.Errorf("failed to decode delegator address: %w", err)
	}
	msg := &epochingtypes.MsgWrappedCancelUnbondingDelegation{
		Msg: &stakingtypes.MsgCancelUnbondingDelegation{
			DelegatorAddress: delegatorAddrStr,
			ValidatorAddress: validatorAddrStr,
			Amount: sdk.Coin{
				Denom:  denom,
				Amount: math.NewIntFromBigInt(amount),
			},
			CreationHeight: creationHeight.Int64(),
		},
	}

	return msg, delegatorAddr, nil
}

// NewDelegationBech32Request creates a new QueryDelegationRequest instance and does sanity checks
// on the given arguments before populating the request.
// NOTE: bring this from cosmos EVM v0.4.1
func NewDelegationBech32Request(args []interface{}, addrCdc address.Codec) (*stakingtypes.QueryDelegationRequest, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf(cmn.ErrInvalidNumberOfArgs, 2, len(args))
	}

	delegatorAddr, ok := args[0].(common.Address)
	if !ok || delegatorAddr == (common.Address{}) {
		return nil, fmt.Errorf(cmn.ErrInvalidDelegator, args[0])
	}

	validatorAddress, ok := args[1].(string)
	if !ok {
		return nil, fmt.Errorf(cmn.ErrInvalidType, "validatorAddress", "string", args[1])
	}

	delegatorAddrStr, err := addrCdc.BytesToString(delegatorAddr.Bytes())
	if err != nil {
		return nil, fmt.Errorf("failed to decode delegator address: %w", err)
	}
	return &stakingtypes.QueryDelegationRequest{
		DelegatorAddr: delegatorAddrStr,
		ValidatorAddr: validatorAddress,
	}, nil
}

// NewDelegationRequest creates a new QueryDelegationRequest instance and does sanity checks
// on the given arguments before populating the request.
// NOTE: bring this from cosmos EVM v0.4.1
func NewDelegationRequest(args []interface{}, addrCdc address.Codec) (*stakingtypes.QueryDelegationRequest, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf(cmn.ErrInvalidNumberOfArgs, 2, len(args))
	}

	delegatorAddr, ok := args[0].(common.Address)
	if !ok || delegatorAddr == (common.Address{}) {
		return nil, fmt.Errorf(cmn.ErrInvalidDelegator, args[0])
	}

	validatorAddr, ok := args[1].(common.Address)
	if !ok || validatorAddr == (common.Address{}) {
		return nil, fmt.Errorf(cmn.ErrInvalidType, "validatorAddr", "common.Address", args[1])
	}

	delegatorAddrStr, err := addrCdc.BytesToString(delegatorAddr.Bytes())
	if err != nil {
		return nil, fmt.Errorf("failed to decode delegator address: %w", err)
	}
	validatorAddrStr := sdk.ValAddress(validatorAddr.Bytes()).String()
	return &stakingtypes.QueryDelegationRequest{
		DelegatorAddr: delegatorAddrStr,
		ValidatorAddr: validatorAddrStr,
	}, nil
}

// NewValidatorRequest create a new QueryValidatorRequest instance and does sanity checks
// on the given arguments before populating the request.
func NewValidatorRequest(args []interface{}) (*stakingtypes.QueryValidatorRequest, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf(cmn.ErrInvalidNumberOfArgs, 1, len(args))
	}

	validatorHexAddr, ok := args[0].(common.Address)
	if !ok || validatorHexAddr == (common.Address{}) {
		return nil, fmt.Errorf(cmn.ErrInvalidValidator, args[0])
	}

	validatorAddress := sdk.ValAddress(validatorHexAddr.Bytes()).String()

	return &stakingtypes.QueryValidatorRequest{ValidatorAddr: validatorAddress}, nil
}

// NewValidatorsRequest create a new QueryValidatorsRequest instance and does sanity checks
// on the given arguments before populating the request.
func NewValidatorsRequest(method *abi.Method, args []interface{}) (*stakingtypes.QueryValidatorsRequest, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf(cmn.ErrInvalidNumberOfArgs, 2, len(args))
	}

	var input ValidatorsInput
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, fmt.Errorf("error while unpacking args to ValidatorsInput struct: %s", err)
	}

	if bytes.Equal(input.PageRequest.Key, []byte{0}) {
		input.PageRequest.Key = nil
	}

	return &stakingtypes.QueryValidatorsRequest{
		Status:     input.Status,
		Pagination: &input.PageRequest,
	}, nil
}

// NewRedelegationRequest create a new QueryRedelegationRequest instance and does sanity checks
// on the given arguments before populating the request.
func NewRedelegationRequest(args []interface{}) (*RedelegationRequest, error) {
	if len(args) != 3 {
		return nil, fmt.Errorf(cmn.ErrInvalidNumberOfArgs, 3, len(args))
	}

	delegatorAddr, ok := args[0].(common.Address)
	if !ok || delegatorAddr == (common.Address{}) {
		return nil, fmt.Errorf(cmn.ErrInvalidDelegator, args[0])
	}

	validatorSrcAddress, ok := args[1].(string)
	if !ok {
		return nil, fmt.Errorf(cmn.ErrInvalidType, "validatorSrcAddress", "string", args[1])
	}

	validatorSrcAddr, err := sdk.ValAddressFromBech32(validatorSrcAddress)
	if err != nil {
		return nil, err
	}

	validatorDstAddress, ok := args[2].(string)
	if !ok {
		return nil, fmt.Errorf(cmn.ErrInvalidType, "validatorDstAddress", "string", args[2])
	}

	validatorDstAddr, err := sdk.ValAddressFromBech32(validatorDstAddress)
	if err != nil {
		return nil, err
	}

	return &RedelegationRequest{
		DelegatorAddress:    delegatorAddr.Bytes(), // bech32 formatted
		ValidatorSrcAddress: validatorSrcAddr,
		ValidatorDstAddress: validatorDstAddr,
	}, nil
}

// NewRedelegationsRequest create a new QueryRedelegationsRequest instance and does sanity checks
// on the given arguments before populating the request.
func NewRedelegationsRequest(method *abi.Method, args []interface{}, addrCdc address.Codec) (*stakingtypes.QueryRedelegationsRequest, error) {
	if len(args) != 4 {
		return nil, fmt.Errorf(cmn.ErrInvalidNumberOfArgs, 4, len(args))
	}

	// delAddr, srcValAddr & dstValAddr
	// can be empty strings. The query will return the
	// corresponding redelegations according to the addresses specified
	// however, cannot pass all as empty strings, need to provide at least
	// the delegator address or the source validator address
	var input RedelegationsInput
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, fmt.Errorf("error while unpacking args to RedelegationsInput struct: %s", err)
	}

	var (
		// delegatorAddr is the string representation of the delegator address
		delegatorAddr = ""
		// emptyAddr is an empty address
		emptyAddr = common.Address{}.Hex()
	)
	if input.DelegatorAddress.Hex() != emptyAddr {
		var err error
		delegatorAddr, err = addrCdc.BytesToString(input.DelegatorAddress.Bytes())
		if err != nil {
			return nil, fmt.Errorf("failed to decode delegator address: %w", err)
		}
	}

	if delegatorAddr == "" && input.SrcValidatorAddress == "" && input.DstValidatorAddress == "" ||
		delegatorAddr == "" && input.SrcValidatorAddress == "" && input.DstValidatorAddress != "" {
		return nil, errors.New("invalid query. Need to specify at least a source validator address or delegator address")
	}

	return &stakingtypes.QueryRedelegationsRequest{
		DelegatorAddr:    delegatorAddr, // bech32 formatted
		SrcValidatorAddr: input.SrcValidatorAddress,
		DstValidatorAddr: input.DstValidatorAddress,
		Pagination:       &input.PageRequest,
	}, nil
}

// NewEpochInfoRequest create a new QueryEpochInfoRequest instance and does sanity checks
// on the given arguments before populating the request.
func NewEpochInfoRequest(args []interface{}) (*epochingtypes.QueryEpochInfoRequest, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf(cmn.ErrInvalidNumberOfArgs, 1, len(args))
	}

	epochNum, ok := args[0].(uint64)
	if !ok {
		return nil, fmt.Errorf(cmn.ErrInvalidType, "epochNum", "uint64", args[0])
	}

	return &epochingtypes.QueryEpochInfoRequest{
		EpochNum: epochNum,
	}, nil
}

// NewCurrentEpochRequest create a new QueryCurrentEpochRequest instance and does sanity checks
// on the given arguments before populating the request.
func NewCurrentEpochRequest(args []interface{}) (*epochingtypes.QueryCurrentEpochRequest, error) {
	if len(args) != 0 {
		return nil, fmt.Errorf(cmn.ErrInvalidNumberOfArgs, 0, len(args))
	}

	return &epochingtypes.QueryCurrentEpochRequest{}, nil
}

// NewEpochMsgsRequest create a new QueryEpochMsgsRequest instance and does sanity checks
// on the given arguments before populating the request.
func NewEpochMsgsRequest(method *abi.Method, args []interface{}) (*epochingtypes.QueryEpochMsgsRequest, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf(cmn.ErrInvalidNumberOfArgs, 2, len(args))
	}

	var input EpochMsgsInput
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, fmt.Errorf("error while unpacking args to EpochMsgsInput struct: %s", err)
	}

	if bytes.Equal(input.PageRequest.Key, []byte{0}) {
		input.PageRequest.Key = nil
	}

	return &epochingtypes.QueryEpochMsgsRequest{
		EpochNum:   input.EpochNumber,
		Pagination: &input.PageRequest,
	}, nil
}

// NewLatestEpochMsgsRequest create a new QueryLatestEpochMsgsRequest instance and does sanity checks
// on the given arguments before populating the request.
func NewLatestEpochMsgsRequest(method *abi.Method, args []interface{}) (*epochingtypes.QueryLatestEpochMsgsRequest, error) {
	if len(args) != 3 {
		return nil, fmt.Errorf(cmn.ErrInvalidNumberOfArgs, 3, len(args))
	}

	var input LatestEpochMsgsInput
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, fmt.Errorf("error while unpacking args to LatestEpochMsgsInput struct: %s", err)
	}

	if bytes.Equal(input.PageRequest.Key, []byte{0}) {
		input.PageRequest.Key = nil
	}

	return &epochingtypes.QueryLatestEpochMsgsRequest{
		EndEpoch:   input.EndEpoch,
		EpochCount: input.EpochCount,
		Pagination: &input.PageRequest,
	}, nil
}

// NewValidatorLifecycleRequest create a new QueryValidatorLifecycleRequest instance and does sanity checks
// on the given arguments before populating the request.
func NewValidatorLifecycleRequest(args []interface{}, valCodec address.Codec) (*epochingtypes.QueryValidatorLifecycleRequest, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf(cmn.ErrInvalidNumberOfArgs, 1, len(args))
	}

	validatorAddr, ok := args[0].(common.Address)
	if !ok || validatorAddr == (common.Address{}) {
		return nil, fmt.Errorf(cmn.ErrInvalidValidator, args[0])
	}

	validatorAddrStr, err := valCodec.BytesToString(validatorAddr.Bytes())
	if err != nil {
		return nil, fmt.Errorf("failed to decode validator address: %w", err)
	}

	return &epochingtypes.QueryValidatorLifecycleRequest{
		ValAddr: validatorAddrStr,
	}, nil
}

// NewDelegationLifecycleRequest create a new QueryDelegationLifecycleRequest instance and does sanity checks
// on the given arguments before populating the request.
func NewDelegationLifecycleRequest(args []interface{}, addrCdc address.Codec) (*epochingtypes.QueryDelegationLifecycleRequest, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf(cmn.ErrInvalidNumberOfArgs, 1, len(args))
	}

	delegatorAddr, ok := args[0].(common.Address)
	if !ok || delegatorAddr == (common.Address{}) {
		return nil, fmt.Errorf(cmn.ErrInvalidDelegator, args[0])
	}

	delegatorAddrStr, err := addrCdc.BytesToString(delegatorAddr.Bytes())
	if err != nil {
		return nil, fmt.Errorf("failed to decode delegator address: %w", err)
	}

	return &epochingtypes.QueryDelegationLifecycleRequest{
		DelAddr: delegatorAddrStr,
	}, nil
}

// NewEpochValSetRequest create a new QueryEpochValSetRequest instance and does sanity checks
// on the given arguments before populating the request.
func NewEpochValSetRequest(method *abi.Method, args []interface{}) (*epochingtypes.QueryEpochValSetRequest, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf(cmn.ErrInvalidNumberOfArgs, 2, len(args))
	}

	var input EpochValSetInput
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, fmt.Errorf("error while unpacking args to EpochValSetInput struct: %s", err)
	}

	if bytes.Equal(input.PageRequest.Key, []byte{0}) {
		input.PageRequest.Key = nil
	}

	return &epochingtypes.QueryEpochValSetRequest{
		EpochNum:   input.EpochNumber,
		Pagination: &input.PageRequest,
	}, nil
}

type EpochResponse struct {
	EpochNumber          uint64 `abi:"epochNumber"`
	CurrentEpochInterval uint64 `abi:"currentEpochInterval"`
	FirstBlockHeight     uint64 `abi:"firstBlockHeight"`
	LastBlockTime        int64  `abi:"lastBlockTime"`
	SealerAppHashHex     string `abi:"sealerAppHashHex"`
	SealerBlockHash      string `abi:"sealerBlockHash"`
}
type EpochInfoOutput struct {
	Epoch EpochResponse
}

func (eo *EpochInfoOutput) FromResponse(res *epochingtypes.QueryEpochInfoResponse) *EpochInfoOutput {
	eo.Epoch.EpochNumber = res.Epoch.EpochNumber
	eo.Epoch.CurrentEpochInterval = res.Epoch.CurrentEpochInterval
	eo.Epoch.FirstBlockHeight = res.Epoch.FirstBlockHeight
	if res.Epoch.LastBlockTime != nil {
		eo.Epoch.LastBlockTime = res.Epoch.LastBlockTime.UTC().Unix()
	} else {
		eo.Epoch.LastBlockTime = -1
	}
	eo.Epoch.SealerAppHashHex = res.Epoch.SealerAppHashHex
	eo.Epoch.SealerBlockHash = res.Epoch.SealerBlockHash
	return eo
}

type CurrentEpochResponse struct {
	CurrentEpoch  uint64 `abi:"currentEpoch"`
	EpochBoundary uint64 `abi:"epochBoundary"`
}

type CurrentEpochOutput struct {
	Response CurrentEpochResponse
}

func (co *CurrentEpochOutput) FromResponse(res *epochingtypes.QueryCurrentEpochResponse) *CurrentEpochOutput {
	co.Response.CurrentEpoch = res.CurrentEpoch
	co.Response.EpochBoundary = res.EpochBoundary
	return co
}

type EpochMsgsInput struct {
	EpochNumber uint64
	PageRequest query.PageRequest
}

type QueuedMessageResponse struct {
	TxId        string `abi:"txId"`
	MsgId       string `abi:"msgId"`
	BlockHeight uint64 `abi:"blockHeight"`
	BlockTime   int64  `abi:"blockTime"`
	Msg         string `abi:"msg"`
	MsgType     string `abi:"msgType"`
}

type EpochMsgsOutput struct {
	QueuedMsgs   []QueuedMessageResponse
	PageResponse query.PageResponse
}

func (eo *EpochMsgsOutput) FromResponse(res *epochingtypes.QueryEpochMsgsResponse) *EpochMsgsOutput {
	eo.QueuedMsgs = make([]QueuedMessageResponse, len(res.Msgs))
	for i, msg := range res.Msgs {
		eo.QueuedMsgs[i] = QueuedMessageResponse{
			TxId:        msg.TxId,
			MsgId:       msg.MsgId,
			BlockHeight: msg.BlockHeight,
			BlockTime:   msg.BlockTime.UTC().Unix(),
			Msg:         msg.Msg,
			MsgType:     msg.MsgType,
		}
	}

	if res.Pagination != nil {
		eo.PageResponse.Total = res.Pagination.Total
		eo.PageResponse.NextKey = res.Pagination.NextKey
	}

	return eo
}

func (eo *EpochMsgsOutput) Pack(args abi.Arguments) ([]byte, error) {
	return args.Pack(eo.QueuedMsgs, eo.PageResponse)
}

type LatestEpochMsgsInput struct {
	EndEpoch    uint64
	EpochCount  uint64
	PageRequest query.PageRequest
}

type QueuedMessageList struct {
	EpochNumber uint64 `abi:"epochNumber"`
	Msgs        []QueuedMessageResponse
}

type LatestEpochMsgsOutput struct {
	LatestEpochMsgs []QueuedMessageList
	PageResponse    query.PageResponse
}

func (leo *LatestEpochMsgsOutput) FromResponse(res *epochingtypes.QueryLatestEpochMsgsResponse) *LatestEpochMsgsOutput {
	leo.LatestEpochMsgs = make([]QueuedMessageList, len(res.LatestEpochMsgs))
	for i, epochMsgList := range res.LatestEpochMsgs {
		msgs := make([]QueuedMessageResponse, len(epochMsgList.Msgs))
		for j, msg := range epochMsgList.Msgs {
			msgs[j] = QueuedMessageResponse{
				TxId:        msg.TxId,
				MsgId:       msg.MsgId,
				BlockHeight: msg.BlockHeight,
				BlockTime:   msg.BlockTime.UTC().Unix(),
				Msg:         msg.Msg,
				MsgType:     msg.MsgType,
			}
		}
		leo.LatestEpochMsgs[i] = QueuedMessageList{
			EpochNumber: epochMsgList.EpochNumber,
			Msgs:        msgs,
		}
	}

	if res.Pagination != nil {
		leo.PageResponse.Total = res.Pagination.Total
		leo.PageResponse.NextKey = res.Pagination.NextKey
	}

	return leo
}

func (leo *LatestEpochMsgsOutput) Pack(args abi.Arguments) ([]byte, error) {
	return args.Pack(leo.LatestEpochMsgs, leo.PageResponse)
}

type ValidatorUpdateResponse struct {
	StateDesc   string `abi:"stateDesc"`
	BlockHeight uint64 `abi:"blockHeight"`
	BlockTime   int64  `abi:"blockTime"`
}

type ValidatorLifecycleOutput struct {
	ValidatorLife []ValidatorUpdateResponse
}

func (vlo *ValidatorLifecycleOutput) FromResponse(res *epochingtypes.QueryValidatorLifecycleResponse) *ValidatorLifecycleOutput {
	vlo.ValidatorLife = make([]ValidatorUpdateResponse, len(res.ValLife))
	for i, valUpdate := range res.ValLife {
		vlo.ValidatorLife[i] = ValidatorUpdateResponse{
			StateDesc:   valUpdate.StateDesc,
			BlockHeight: valUpdate.BlockHeight,
			BlockTime:   valUpdate.BlockTime.UTC().Unix(),
		}
	}

	return vlo
}

type DelegationStateUpdateBech32 struct {
	State       uint8    `abi:"state"`
	ValAddr     string   `abi:"valAddr"`
	Amount      cmn.Coin `abi:"amount"`
	BlockHeight uint64   `abi:"blockHeight"`
	BlockTime   int64    `abi:"blockTime"`
}

type DelegationStateUpdate struct {
	State       uint8          `abi:"state"`
	ValAddr     common.Address `abi:"valAddr"`
	Amount      cmn.Coin       `abi:"amount"`
	BlockHeight uint64         `abi:"blockHeight"`
	BlockTime   int64          `abi:"blockTime"`
}

type DelegationLifecycleStructBech32 struct {
	DelAddr string `abi:"delAddr"`
	DelLife []DelegationStateUpdateBech32
}

type DelegationLifecycleStruct struct {
	DelAddr common.Address `abi:"delAddr"`
	DelLife []DelegationStateUpdate
}

type DelegationLifecycleOutputBech32 struct {
	DelegationLifecycle DelegationLifecycleStructBech32
}

type DelegationLifecycleOutput struct {
	DelegationLifecycle DelegationLifecycleStruct
}

func (dlo *DelegationLifecycleOutputBech32) FromResponse(res *epochingtypes.QueryDelegationLifecycleResponse) *DelegationLifecycleOutputBech32 {
	// return nil if there are no DelLife found in the records
	if (res.DelLife == nil) || (res.DelLife.DelLife == nil) {
		dlo.DelegationLifecycle = DelegationLifecycleStructBech32{
			DelAddr: "",
			DelLife: []DelegationStateUpdateBech32{},
		}
		return dlo
	}

	delLife := make([]DelegationStateUpdateBech32, len(res.DelLife.DelLife))
	for i, delUpdate := range res.DelLife.DelLife {
		delLife[i] = DelegationStateUpdateBech32{
			State:       uint8(delUpdate.State), // Convert BondState enum to uint8
			ValAddr:     delUpdate.ValAddr,
			Amount:      cmn.Coin{Denom: delUpdate.Amount.Denom, Amount: delUpdate.Amount.Amount.BigInt()},
			BlockHeight: delUpdate.BlockHeight,
			BlockTime:   delUpdate.BlockTime.UTC().Unix(),
		}
	}

	dlo.DelegationLifecycle = DelegationLifecycleStructBech32{
		DelAddr: res.DelLife.DelAddr,
		DelLife: delLife,
	}

	return dlo
}

func (dlo *DelegationLifecycleOutput) FromResponse(res *epochingtypes.QueryDelegationLifecycleResponse) *DelegationLifecycleOutput {
	// return nil if there are no DelLife found in the records
	if (res.DelLife == nil) || (res.DelLife.DelLife == nil) {
		dlo.DelegationLifecycle = DelegationLifecycleStruct{
			DelAddr: common.Address{},
			DelLife: []DelegationStateUpdate{},
		}
		return dlo
	}

	delLife := make([]DelegationStateUpdate, len(res.DelLife.DelLife))
	for i, delUpdate := range res.DelLife.DelLife {
		// convert bech32 validator address to hex
		valAddr, err := sdk.ValAddressFromBech32(delUpdate.ValAddr)
		var valAddrHex common.Address
		if err == nil {
			valAddrHex = common.BytesToAddress(valAddr.Bytes())
		}

		delLife[i] = DelegationStateUpdate{
			State:       uint8(delUpdate.State), // Convert BondState enum to uint8
			ValAddr:     valAddrHex,
			Amount:      cmn.Coin{Denom: delUpdate.Amount.Denom, Amount: delUpdate.Amount.Amount.BigInt()},
			BlockHeight: delUpdate.BlockHeight,
			BlockTime:   delUpdate.BlockTime.UTC().Unix(),
		}
	}

	// convert bech32 delegator address to hex
	delegatorAddr, err := sdk.AccAddressFromBech32(res.DelLife.DelAddr)
	var delegatorAddrHex common.Address
	if err == nil {
		delegatorAddrHex = common.BytesToAddress(delegatorAddr.Bytes())
	}

	dlo.DelegationLifecycle = DelegationLifecycleStruct{
		DelAddr: delegatorAddrHex,
		DelLife: delLife,
	}

	return dlo
}

type EpochValSetInput struct {
	EpochNumber uint64
	PageRequest query.PageRequest
}

type SimpleValidator struct {
	Addr  common.Address `abi:"addr"`
	Power int64          `abi:"power"`
}

type EpochValSetOutput struct {
	Validators       []SimpleValidator
	TotalVotingPower int64 `abi:"totalVotingPower"`
	PageResponse     query.PageResponse
}

func (evso *EpochValSetOutput) FromResponse(res *epochingtypes.QueryEpochValSetResponse) *EpochValSetOutput {
	evso.Validators = make([]SimpleValidator, len(res.Validators))
	for i, validator := range res.Validators {
		evso.Validators[i] = SimpleValidator{
			Addr:  common.BytesToAddress(validator.Addr),
			Power: validator.Power,
		}
	}

	evso.TotalVotingPower = res.TotalVotingPower

	if res.Pagination != nil {
		evso.PageResponse.Total = res.Pagination.Total
		evso.PageResponse.NextKey = res.Pagination.NextKey
	}

	return evso
}

// Pack packs a given slice of abi arguments into a byte array.
func (evso *EpochValSetOutput) Pack(args abi.Arguments) ([]byte, error) {
	return args.Pack(evso.Validators, evso.TotalVotingPower, evso.PageResponse)
}

// RedelegationRequest is a struct that contains the information to pass into a redelegation query.
type RedelegationRequest struct {
	DelegatorAddress    sdk.AccAddress
	ValidatorSrcAddress sdk.ValAddress
	ValidatorDstAddress sdk.ValAddress
}

// UnbondingDelegationEntry is a struct that contains the information about an unbonding delegation entry.
type UnbondingDelegationEntry struct {
	CreationHeight          int64
	CompletionTime          int64
	InitialBalance          *big.Int
	Balance                 *big.Int
	UnbondingId             uint64 //nolint
	UnbondingOnHoldRefCount int64
}

// UnbondingDelegationResponseBech32 is a struct that contains the information about an unbonding delegation.
type UnbondingDelegationResponseBech32 struct {
	DelegatorAddress string
	ValidatorAddress string
	Entries          []UnbondingDelegationEntry
}

// UnbondingDelegationResponse is a struct that contains the information about an unbonding delegation.
type UnbondingDelegationResponse struct {
	DelegatorAddress common.Address
	ValidatorAddress common.Address
	Entries          []UnbondingDelegationEntry
}

// UnbondingDelegationOutputBech32 is the output response returned by the query method.
type UnbondingDelegationOutputBech32 struct {
	UnbondingDelegation UnbondingDelegationResponseBech32
}

// UnbondingDelegationOutput is the output response returned by the query method.
type UnbondingDelegationOutput struct {
	UnbondingDelegation UnbondingDelegationResponse
}

// FromResponse populates the UnbondingDelegationOutputBech32 from a QueryUnbondingDelegationResponse.
func (do *UnbondingDelegationOutputBech32) FromResponse(res *stakingtypes.QueryUnbondingDelegationResponse) *UnbondingDelegationOutputBech32 {
	do.UnbondingDelegation.Entries = make([]UnbondingDelegationEntry, len(res.Unbond.Entries))
	do.UnbondingDelegation.ValidatorAddress = res.Unbond.ValidatorAddress
	do.UnbondingDelegation.DelegatorAddress = res.Unbond.DelegatorAddress
	for i, entry := range res.Unbond.Entries {
		do.UnbondingDelegation.Entries[i] = UnbondingDelegationEntry{
			UnbondingId:             entry.UnbondingId,
			UnbondingOnHoldRefCount: entry.UnbondingOnHoldRefCount,
			CreationHeight:          entry.CreationHeight,
			CompletionTime:          entry.CompletionTime.UTC().Unix(),
			InitialBalance:          entry.InitialBalance.BigInt(),
			Balance:                 entry.Balance.BigInt(),
		}
	}
	return do
}

// FromResponse populates the UnbondingDelegationOutput from a QueryUnbondingDelegationResponse.
func (do *UnbondingDelegationOutput) FromResponse(res *stakingtypes.QueryUnbondingDelegationResponse, addrCdc address.Codec) *UnbondingDelegationOutput {
	do.UnbondingDelegation.Entries = make([]UnbondingDelegationEntry, len(res.Unbond.Entries))
	valAddrBytes, err := addrCdc.StringToBytes(res.Unbond.ValidatorAddress)
	if err != nil {
		return nil
	}
	delAddrBytes, err := addrCdc.StringToBytes(res.Unbond.DelegatorAddress)
	if err != nil {
		return nil
	}
	do.UnbondingDelegation.ValidatorAddress = common.BytesToAddress(valAddrBytes)
	do.UnbondingDelegation.DelegatorAddress = common.BytesToAddress(delAddrBytes)
	for i, entry := range res.Unbond.Entries {
		do.UnbondingDelegation.Entries[i] = UnbondingDelegationEntry{
			UnbondingId:             entry.UnbondingId,
			UnbondingOnHoldRefCount: entry.UnbondingOnHoldRefCount,
			CreationHeight:          entry.CreationHeight,
			CompletionTime:          entry.CompletionTime.UTC().Unix(),
			InitialBalance:          entry.InitialBalance.BigInt(),
			Balance:                 entry.Balance.BigInt(),
		}
	}
	return do
}

// DelegationOutput is a struct to represent the key information from
// a delegation response.
type DelegationOutput struct {
	Shares  *big.Int
	Balance cmn.Coin
}

// FromResponse populates the DelegationOutput from a QueryDelegationResponse.
func (do *DelegationOutput) FromResponse(res *stakingtypes.QueryDelegationResponse) *DelegationOutput {
	do.Shares = res.DelegationResponse.Delegation.Shares.BigInt()
	do.Balance = cmn.Coin{
		Denom:  res.DelegationResponse.Balance.Denom,
		Amount: res.DelegationResponse.Balance.Amount.BigInt(),
	}
	return do
}

// Pack packs a given slice of abi arguments into a byte array.
func (do *DelegationOutput) Pack(args abi.Arguments) ([]byte, error) {
	return args.Pack(do.Shares, do.Balance)
}

// ValidatorInfoBech32 is a struct to represent the key information from
// a validator response with bech32 addresses.
type ValidatorInfoBech32 struct {
	OperatorAddress   string   `abi:"operatorAddress"`
	ConsensusPubkey   string   `abi:"consensusPubkey"`
	Jailed            bool     `abi:"jailed"`
	Status            uint8    `abi:"status"`
	Tokens            *big.Int `abi:"tokens"`
	DelegatorShares   *big.Int `abi:"delegatorShares"` // TODO: Decimal
	Description       string   `abi:"description"`
	UnbondingHeight   int64    `abi:"unbondingHeight"`
	UnbondingTime     int64    `abi:"unbondingTime"`
	Commission        *big.Int `abi:"commission"`
	MinSelfDelegation *big.Int `abi:"minSelfDelegation"`
}

// ValidatorInfo is a struct to represent the key information from
// a validator response with hex addresses.
type ValidatorInfo struct {
	OperatorAddress   common.Address `abi:"operatorAddress"`
	ConsensusPubkey   string         `abi:"consensusPubkey"`
	Jailed            bool           `abi:"jailed"`
	Status            uint8          `abi:"status"`
	Tokens            *big.Int       `abi:"tokens"`
	DelegatorShares   *big.Int       `abi:"delegatorShares"` // TODO: Decimal
	Description       string         `abi:"description"`
	UnbondingHeight   int64          `abi:"unbondingHeight"`
	UnbondingTime     int64          `abi:"unbondingTime"`
	Commission        *big.Int       `abi:"commission"`
	MinSelfDelegation *big.Int       `abi:"minSelfDelegation"`
}

type ValidatorOutputBech32 struct {
	Validator ValidatorInfoBech32
}

type ValidatorOutput struct {
	Validator ValidatorInfo
}

// DefaultValidatorOutputBech32 returns a ValidatorOutputBech32 with default values.
func DefaultValidatorOutputBech32() ValidatorOutputBech32 {
	return ValidatorOutputBech32{
		ValidatorInfoBech32{
			OperatorAddress:   "",
			ConsensusPubkey:   "",
			Jailed:            false,
			Status:            uint8(0),
			Tokens:            big.NewInt(0),
			DelegatorShares:   big.NewInt(0),
			Description:       "",
			UnbondingHeight:   int64(0),
			UnbondingTime:     int64(0),
			Commission:        big.NewInt(0),
			MinSelfDelegation: big.NewInt(0),
		},
	}
}

// DefaultValidatorOutput returns a ValidatorOutput with default values.
func DefaultValidatorOutput() ValidatorOutput {
	return ValidatorOutput{
		ValidatorInfo{
			OperatorAddress:   common.Address{},
			ConsensusPubkey:   "",
			Jailed:            false,
			Status:            uint8(0),
			Tokens:            big.NewInt(0),
			DelegatorShares:   big.NewInt(0),
			Description:       "",
			UnbondingHeight:   int64(0),
			UnbondingTime:     int64(0),
			Commission:        big.NewInt(0),
			MinSelfDelegation: big.NewInt(0),
		},
	}
}

// FromResponse populates the ValidatorOutputBech32 from a QueryValidatorResponse.
func (vo *ValidatorOutputBech32) FromResponse(res *stakingtypes.QueryValidatorResponse) ValidatorOutputBech32 {
	return ValidatorOutputBech32{
		Validator: ValidatorInfoBech32{
			OperatorAddress: res.Validator.OperatorAddress,
			ConsensusPubkey: FormatConsensusPubkey(res.Validator.ConsensusPubkey),
			Jailed:          res.Validator.Jailed,
			Status:          uint8(stakingtypes.BondStatus_value[res.Validator.Status.String()]), //#nosec G115 // enum will always be convertible to uint8
			Tokens:          res.Validator.Tokens.BigInt(),
			DelegatorShares: res.Validator.DelegatorShares.BigInt(), // TODO: Decimal
			// TODO: create description type,
			Description:       res.Validator.Description.Details,
			UnbondingHeight:   res.Validator.UnbondingHeight,
			UnbondingTime:     res.Validator.UnbondingTime.UTC().Unix(),
			Commission:        res.Validator.Commission.Rate.BigInt(),
			MinSelfDelegation: res.Validator.MinSelfDelegation.BigInt(),
		},
	}
}

// FromResponse populates the ValidatorOutput from a QueryValidatorResponse.
func (vo *ValidatorOutput) FromResponse(res *stakingtypes.QueryValidatorResponse) ValidatorOutput {
	operatorAddress, err := sdk.ValAddressFromBech32(res.Validator.OperatorAddress)
	if err != nil {
		return DefaultValidatorOutput()
	}

	return ValidatorOutput{
		Validator: ValidatorInfo{
			OperatorAddress: common.BytesToAddress(operatorAddress.Bytes()),
			ConsensusPubkey: FormatConsensusPubkey(res.Validator.ConsensusPubkey),
			Jailed:          res.Validator.Jailed,
			Status:          uint8(stakingtypes.BondStatus_value[res.Validator.Status.String()]), //#nosec G115 // enum will always be convertible to uint8
			Tokens:          res.Validator.Tokens.BigInt(),
			DelegatorShares: res.Validator.DelegatorShares.BigInt(), // TODO: Decimal
			// TODO: create description type,
			Description:       res.Validator.Description.Details,
			UnbondingHeight:   res.Validator.UnbondingHeight,
			UnbondingTime:     res.Validator.UnbondingTime.UTC().Unix(),
			Commission:        res.Validator.Commission.Rate.BigInt(),
			MinSelfDelegation: res.Validator.MinSelfDelegation.BigInt(),
		},
	}
}

// ValidatorsInput is a struct to represent the input information for
// the validators query. Needed to unpack arguments into the PageRequest struct.
type ValidatorsInput struct {
	Status      string
	PageRequest query.PageRequest
}

// ValidatorsOutputBech32 is a struct to represent the key information from
// a validators response with bech32 addresses.
type ValidatorsOutputBech32 struct {
	Validators   []ValidatorInfoBech32
	PageResponse query.PageResponse
}

// ValidatorsOutput is a struct to represent the key information from
// a validators response with hex addresses.
type ValidatorsOutput struct {
	Validators   []ValidatorInfo
	PageResponse query.PageResponse
}

// FromResponse populates the ValidatorsOutputBech32 from a QueryValidatorsResponse.
func (vo *ValidatorsOutputBech32) FromResponse(res *stakingtypes.QueryValidatorsResponse) *ValidatorsOutputBech32 {
	vo.Validators = make([]ValidatorInfoBech32, len(res.Validators))
	for i, v := range res.Validators {
		vo.Validators[i] = ValidatorInfoBech32{
			OperatorAddress:   v.OperatorAddress,
			ConsensusPubkey:   FormatConsensusPubkey(v.ConsensusPubkey),
			Jailed:            v.Jailed,
			Status:            uint8(stakingtypes.BondStatus_value[v.Status.String()]), //#nosec G115 // enum will always be convertible to uint8
			Tokens:            v.Tokens.BigInt(),
			DelegatorShares:   v.DelegatorShares.BigInt(),
			Description:       v.Description.Details,
			UnbondingHeight:   v.UnbondingHeight,
			UnbondingTime:     v.UnbondingTime.UTC().Unix(),
			Commission:        v.Commission.Rate.BigInt(),
			MinSelfDelegation: v.MinSelfDelegation.BigInt(),
		}
	}

	if res.Pagination != nil {
		vo.PageResponse.Total = res.Pagination.Total
		vo.PageResponse.NextKey = res.Pagination.NextKey
	}

	return vo
}

// Pack packs a given slice of abi arguments into a byte array.
func (vo *ValidatorsOutputBech32) Pack(args abi.Arguments) ([]byte, error) {
	return args.Pack(vo.Validators, vo.PageResponse)
}

// FromResponse populates the ValidatorsOutput from a QueryValidatorsResponse.
func (vo *ValidatorsOutput) FromResponse(res *stakingtypes.QueryValidatorsResponse) *ValidatorsOutput {
	vo.Validators = make([]ValidatorInfo, len(res.Validators))
	for i, v := range res.Validators {
		operatorAddress, err := sdk.ValAddressFromBech32(v.OperatorAddress)
		if err != nil {
			vo.Validators[i] = DefaultValidatorOutput().Validator
		} else {
			vo.Validators[i] = ValidatorInfo{
				OperatorAddress:   common.BytesToAddress(operatorAddress.Bytes()),
				ConsensusPubkey:   FormatConsensusPubkey(v.ConsensusPubkey),
				Jailed:            v.Jailed,
				Status:            uint8(stakingtypes.BondStatus_value[v.Status.String()]), //#nosec G115 // enum will always be convertible to uint8
				Tokens:            v.Tokens.BigInt(),
				DelegatorShares:   v.DelegatorShares.BigInt(),
				Description:       v.Description.Details,
				UnbondingHeight:   v.UnbondingHeight,
				UnbondingTime:     v.UnbondingTime.UTC().Unix(),
				Commission:        v.Commission.Rate.BigInt(),
				MinSelfDelegation: v.MinSelfDelegation.BigInt(),
			}
		}
	}

	if res.Pagination != nil {
		vo.PageResponse.Total = res.Pagination.Total
		vo.PageResponse.NextKey = res.Pagination.NextKey
	}

	return vo
}

// Pack packs a given slice of abi arguments into a byte array.
func (vo *ValidatorsOutput) Pack(args abi.Arguments) ([]byte, error) {
	return args.Pack(vo.Validators, vo.PageResponse)
}

// RedelegationEntry is a struct to represent the key information from
// a redelegation entry response.
type RedelegationEntry struct {
	CreationHeight int64
	CompletionTime int64
	InitialBalance *big.Int
	SharesDst      *big.Int
}

// RedelegationValuesBech32 is a struct to represent the key information from
// a redelegation response with bech32 addresses.
type RedelegationValuesBech32 struct {
	DelegatorAddress    string
	ValidatorSrcAddress string
	ValidatorDstAddress string
	Entries             []RedelegationEntry
}

// RedelegationValues is a struct to represent the key information from
// a redelegation response with hex addresses.
type RedelegationValues struct {
	DelegatorAddress    common.Address
	ValidatorSrcAddress common.Address
	ValidatorDstAddress common.Address
	Entries             []RedelegationEntry
}

// RedelegationOutputBech32 returns the output for a redelegation query with bech32 addresses.
type RedelegationOutputBech32 struct {
	Redelegation RedelegationValuesBech32
}

// RedelegationOutput returns the output for a redelegation query with hex addresses.
type RedelegationOutput struct {
	Redelegation RedelegationValues
}

// FromResponse populates the RedelegationOutputBech32 from a QueryRedelegationsResponse.
func (ro *RedelegationOutputBech32) FromResponse(res stakingtypes.Redelegation) *RedelegationOutputBech32 {
	ro.Redelegation.Entries = make([]RedelegationEntry, len(res.Entries))
	ro.Redelegation.DelegatorAddress = res.DelegatorAddress
	ro.Redelegation.ValidatorSrcAddress = res.ValidatorSrcAddress
	ro.Redelegation.ValidatorDstAddress = res.ValidatorDstAddress
	for i, entry := range res.Entries {
		ro.Redelegation.Entries[i] = RedelegationEntry{
			CreationHeight: entry.CreationHeight,
			CompletionTime: entry.CompletionTime.UTC().Unix(),
			InitialBalance: entry.InitialBalance.BigInt(),
			SharesDst:      entry.SharesDst.BigInt(),
		}
	}
	return ro
}

// FromResponse populates the RedelegationOutput from a QueryRedelegationsResponse.
func (ro *RedelegationOutput) FromResponse(res stakingtypes.Redelegation) *RedelegationOutput {
	// convert bech32 addresses to hex addresses
	delegatorAddr, err := sdk.AccAddressFromBech32(res.DelegatorAddress)
	if err != nil {
		ro.Redelegation.DelegatorAddress = common.Address{}
	} else {
		ro.Redelegation.DelegatorAddress = common.BytesToAddress(delegatorAddr.Bytes())
	}

	validatorSrcAddr, err := sdk.ValAddressFromBech32(res.ValidatorSrcAddress)
	if err != nil {
		ro.Redelegation.ValidatorSrcAddress = common.Address{}
	} else {
		ro.Redelegation.ValidatorSrcAddress = common.BytesToAddress(validatorSrcAddr.Bytes())
	}

	validatorDstAddr, err := sdk.ValAddressFromBech32(res.ValidatorDstAddress)
	if err != nil {
		ro.Redelegation.ValidatorDstAddress = common.Address{}
	} else {
		ro.Redelegation.ValidatorDstAddress = common.BytesToAddress(validatorDstAddr.Bytes())
	}

	ro.Redelegation.Entries = make([]RedelegationEntry, len(res.Entries))
	for i, entry := range res.Entries {
		ro.Redelegation.Entries[i] = RedelegationEntry{
			CreationHeight: entry.CreationHeight,
			CompletionTime: entry.CompletionTime.UTC().Unix(),
			InitialBalance: entry.InitialBalance.BigInt(),
			SharesDst:      entry.SharesDst.BigInt(),
		}
	}
	return ro
}

// RedelegationEntryResponse is equivalent to a RedelegationEntry except that it
// contains a balance in addition to shares which is more suitable for client
// responses.
type RedelegationEntryResponse struct {
	RedelegationEntry RedelegationEntry
	Balance           *big.Int
}

// RedelegationBech32 contains the list of a particular delegator's redelegating bonds
// from a particular source validator to a particular destination validator with bech32 addresses.
type RedelegationBech32 struct {
	DelegatorAddress    string
	ValidatorSrcAddress string
	ValidatorDstAddress string
	Entries             []RedelegationEntry
}

// Redelegation contains the list of a particular delegator's redelegating bonds
// from a particular source validator to a particular destination validator with hex addresses.
type Redelegation struct {
	DelegatorAddress    common.Address
	ValidatorSrcAddress common.Address
	ValidatorDstAddress common.Address
	Entries             []RedelegationEntry
}

// RedelegationResponseBech32 is equivalent to a RedelegationBech32 except that its entries
// contain a balance in addition to shares which is more suitable for client
// responses with bech32 addresses.
type RedelegationResponseBech32 struct {
	Redelegation RedelegationBech32
	Entries      []RedelegationEntryResponse
}

// RedelegationResponse is equivalent to a Redelegation except that its entries
// contain a balance in addition to shares which is more suitable for client
// responses with hex addresses.
type RedelegationResponse struct {
	Redelegation Redelegation
	Entries      []RedelegationEntryResponse
}

// RedelegationsInput is a struct to represent the input information for
// the redelegations query. Needed to unpack arguments into the PageRequest struct.
type RedelegationsInput struct {
	DelegatorAddress    common.Address
	SrcValidatorAddress string
	DstValidatorAddress string
	PageRequest         query.PageRequest
}

// RedelegationsOutputBech32 is a struct to represent the key information from
// a redelegations response with bech32 addresses.
type RedelegationsOutputBech32 struct {
	Response     []RedelegationResponseBech32
	PageResponse query.PageResponse
}

// RedelegationsOutput is a struct to represent the key information from
// a redelegations response with hex addresses.
type RedelegationsOutput struct {
	Response     []RedelegationResponse
	PageResponse query.PageResponse
}

// FromResponse populates the RedelegationsOutputBech32 from a QueryRedelegationsResponse.
func (ro *RedelegationsOutputBech32) FromResponse(res *stakingtypes.QueryRedelegationsResponse) *RedelegationsOutputBech32 {
	ro.Response = make([]RedelegationResponseBech32, len(res.RedelegationResponses))
	for i, resp := range res.RedelegationResponses {
		// for each RedelegationResponse
		// there's a RedelegationEntryResponse array ('Entries' field)
		entries := make([]RedelegationEntryResponse, len(resp.Entries))
		for j, e := range resp.Entries {
			entries[j] = RedelegationEntryResponse{
				RedelegationEntry: RedelegationEntry{
					CreationHeight: e.RedelegationEntry.CreationHeight,
					CompletionTime: e.RedelegationEntry.CompletionTime.Unix(),
					InitialBalance: e.RedelegationEntry.InitialBalance.BigInt(),
					SharesDst:      e.RedelegationEntry.SharesDst.BigInt(),
				},
				Balance: e.Balance.BigInt(),
			}
		}

		// the Redelegation field has also an 'Entries' field of type RedelegationEntry
		redelEntries := make([]RedelegationEntry, len(resp.Redelegation.Entries))
		for j, e := range resp.Redelegation.Entries {
			redelEntries[j] = RedelegationEntry{
				CreationHeight: e.CreationHeight,
				CompletionTime: e.CompletionTime.Unix(),
				InitialBalance: e.InitialBalance.BigInt(),
				SharesDst:      e.SharesDst.BigInt(),
			}
		}

		ro.Response[i] = RedelegationResponseBech32{
			Entries: entries,
			Redelegation: RedelegationBech32{
				DelegatorAddress:    resp.Redelegation.DelegatorAddress,
				ValidatorSrcAddress: resp.Redelegation.ValidatorSrcAddress,
				ValidatorDstAddress: resp.Redelegation.ValidatorDstAddress,
				Entries:             redelEntries,
			},
		}
	}

	if res.Pagination != nil {
		ro.PageResponse.Total = res.Pagination.Total
		ro.PageResponse.NextKey = res.Pagination.NextKey
	}

	return ro
}

// Pack packs a given slice of abi arguments into a byte array.
func (ro *RedelegationsOutputBech32) Pack(args abi.Arguments) ([]byte, error) {
	return args.Pack(ro.Response, ro.PageResponse)
}

// FromResponse populates the RedelgationsOutput from a QueryRedelegationsResponse.
func (ro *RedelegationsOutput) FromResponse(res *stakingtypes.QueryRedelegationsResponse) *RedelegationsOutput {
	ro.Response = make([]RedelegationResponse, len(res.RedelegationResponses))
	for i, resp := range res.RedelegationResponses {
		// for each RedelegationResponse
		// there's a RedelegationEntryResponse array ('Entries' field)
		entries := make([]RedelegationEntryResponse, len(resp.Entries))
		for j, e := range resp.Entries {
			entries[j] = RedelegationEntryResponse{
				RedelegationEntry: RedelegationEntry{
					CreationHeight: e.RedelegationEntry.CreationHeight,
					CompletionTime: e.RedelegationEntry.CompletionTime.Unix(),
					InitialBalance: e.RedelegationEntry.InitialBalance.BigInt(),
					SharesDst:      e.RedelegationEntry.SharesDst.BigInt(),
				},
				Balance: e.Balance.BigInt(),
			}
		}

		// the Redelegation field has also an 'Entries' field of type RedelegationEntry
		redelEntries := make([]RedelegationEntry, len(resp.Redelegation.Entries))
		for j, e := range resp.Redelegation.Entries {
			redelEntries[j] = RedelegationEntry{
				CreationHeight: e.CreationHeight,
				CompletionTime: e.CompletionTime.Unix(),
				InitialBalance: e.InitialBalance.BigInt(),
				SharesDst:      e.SharesDst.BigInt(),
			}
		}

		// Convert bech32 addresses to hex addresses for hex version
		delegatorAddr, err := sdk.AccAddressFromBech32(resp.Redelegation.DelegatorAddress)
		var delegatorAddrHex common.Address
		if err == nil {
			delegatorAddrHex = common.BytesToAddress(delegatorAddr.Bytes())
		}

		validatorSrcAddr, err := sdk.ValAddressFromBech32(resp.Redelegation.ValidatorSrcAddress)
		var validatorSrcAddrHex common.Address
		if err == nil {
			validatorSrcAddrHex = common.BytesToAddress(validatorSrcAddr.Bytes())
		}

		validatorDstAddr, err := sdk.ValAddressFromBech32(resp.Redelegation.ValidatorDstAddress)
		var validatorDstAddrHex common.Address
		if err == nil {
			validatorDstAddrHex = common.BytesToAddress(validatorDstAddr.Bytes())
		}

		ro.Response[i] = RedelegationResponse{
			Entries: entries,
			Redelegation: Redelegation{
				DelegatorAddress:    delegatorAddrHex,
				ValidatorSrcAddress: validatorSrcAddrHex,
				ValidatorDstAddress: validatorDstAddrHex,
				Entries:             redelEntries,
			},
		}
	}

	if res.Pagination != nil {
		ro.PageResponse.Total = res.Pagination.Total
		ro.PageResponse.NextKey = res.Pagination.NextKey
	}

	return ro
}

// Pack packs a given slice of abi arguments into a byte array.
func (ro *RedelegationsOutput) Pack(args abi.Arguments) ([]byte, error) {
	return args.Pack(ro.Response, ro.PageResponse)
}

// NewUnbondingDelegationBech32Request creates a new QueryUnbondingDelegationRequest instance and does sanity checks
// on the given arguments before populating the request.
func NewUnbondingDelegationBech32Request(args []interface{}, addrCdc address.Codec) (*stakingtypes.QueryUnbondingDelegationRequest, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf(cmn.ErrInvalidNumberOfArgs, 2, len(args))
	}

	delegatorAddr, ok := args[0].(common.Address)
	if !ok || delegatorAddr == (common.Address{}) {
		return nil, fmt.Errorf(cmn.ErrInvalidDelegator, args[0])
	}

	validatorAddress, ok := args[1].(string)
	if !ok {
		return nil, fmt.Errorf(cmn.ErrInvalidType, "validatorAddress", "string", args[1])
	}

	delegatorAddrStr, err := addrCdc.BytesToString(delegatorAddr.Bytes())
	if err != nil {
		return nil, fmt.Errorf("failed to decode delegator address: %w", err)
	}
	return &stakingtypes.QueryUnbondingDelegationRequest{
		DelegatorAddr: delegatorAddrStr,
		ValidatorAddr: validatorAddress,
	}, nil
}

// NewUnbondingDelegationRequest creates a new QueryUnbondingDelegationRequest instance and does sanity checks
// on the given arguments before populating the request.
func NewUnbondingDelegationRequest(args []interface{}, addrCdc address.Codec, valCodec address.Codec) (*stakingtypes.QueryUnbondingDelegationRequest, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf(cmn.ErrInvalidNumberOfArgs, 2, len(args))
	}

	delegatorAddr, ok := args[0].(common.Address)
	if !ok || delegatorAddr == (common.Address{}) {
		return nil, fmt.Errorf(cmn.ErrInvalidDelegator, args[0])
	}

	validatorAddr, ok := args[1].(common.Address)
	if !ok || validatorAddr == (common.Address{}) {
		return nil, fmt.Errorf(cmn.ErrInvalidType, "validatorAddr", "common.Address", args[1])
	}

	delegatorAddrStr, err := addrCdc.BytesToString(delegatorAddr.Bytes())
	if err != nil {
		return nil, fmt.Errorf("failed to decode delegator address: %w", err)
	}
	validatorAddrStr, err := valCodec.BytesToString(validatorAddr.Bytes())
	if err != nil {
		return nil, fmt.Errorf("failed to decode validator address: %w", err)
	}
	return &stakingtypes.QueryUnbondingDelegationRequest{
		DelegatorAddr: delegatorAddrStr,
		ValidatorAddr: validatorAddrStr,
	}, nil
}

// checkDelegationUndelegationArgs checks the arguments for the delegation and undelegation functions.
func checkDelegationUndelegationArgs(args []interface{}) (common.Address, common.Address, *big.Int, error) {
	if len(args) != 3 {
		return common.Address{}, common.Address{}, nil, fmt.Errorf(cmn.ErrInvalidNumberOfArgs, 3, len(args))
	}

	delegatorAddr, ok := args[0].(common.Address)
	if !ok || delegatorAddr == (common.Address{}) {
		return common.Address{}, common.Address{}, nil, fmt.Errorf(cmn.ErrInvalidDelegator, args[0])
	}

	validatorAddr, ok := args[1].(common.Address)
	if !ok {
		return common.Address{}, common.Address{}, nil, fmt.Errorf(cmn.ErrInvalidType, "validatorAddr", "common.Address", args[1])
	}

	amount, ok := args[2].(*big.Int)
	if !ok {
		return common.Address{}, common.Address{}, nil, fmt.Errorf(cmn.ErrInvalidAmount, args[2])
	}

	return delegatorAddr, validatorAddr, amount, nil
}

// checkDelegationUndelegationArgsBech32 checks the arguments for the delegation and undelegation functions.
func checkDelegationUndelegationArgsBech32(args []interface{}) (common.Address, string, *big.Int, error) {
	if len(args) != 3 {
		return common.Address{}, "", nil, fmt.Errorf(cmn.ErrInvalidNumberOfArgs, 3, len(args))
	}

	delegatorAddr, ok := args[0].(common.Address)
	if !ok || delegatorAddr == (common.Address{}) {
		return common.Address{}, "", nil, fmt.Errorf(cmn.ErrInvalidDelegator, args[0])
	}

	validatorAddress, ok := args[1].(string)
	if !ok {
		return common.Address{}, "", nil, fmt.Errorf(cmn.ErrInvalidType, "validatorAddress", "string", args[1])
	}

	amount, ok := args[2].(*big.Int)
	if !ok {
		return common.Address{}, "", nil, fmt.Errorf(cmn.ErrInvalidAmount, args[2])
	}

	return delegatorAddr, validatorAddress, amount, nil
}

// NewRedelegationBech32Request create a new RedelegationRequest instance for Bech32 addresses and does sanity checks
// on the given arguments before populating the request.
func NewRedelegationBech32Request(args []interface{}) (*RedelegationRequest, error) {
	if len(args) != 3 {
		return nil, fmt.Errorf(cmn.ErrInvalidNumberOfArgs, 3, len(args))
	}

	delegatorAddr, ok := args[0].(common.Address)
	if !ok || delegatorAddr == (common.Address{}) {
		return nil, fmt.Errorf(cmn.ErrInvalidDelegator, args[0])
	}

	validatorSrcAddress, ok := args[1].(string)
	if !ok {
		return nil, fmt.Errorf(cmn.ErrInvalidType, "validatorSrcAddress", "string", args[1])
	}

	validatorSrcAddr, err := sdk.ValAddressFromBech32(validatorSrcAddress)
	if err != nil {
		return nil, err
	}

	validatorDstAddress, ok := args[2].(string)
	if !ok {
		return nil, fmt.Errorf(cmn.ErrInvalidType, "validatorDstAddress", "string", args[2])
	}

	validatorDstAddr, err := sdk.ValAddressFromBech32(validatorDstAddress)
	if err != nil {
		return nil, err
	}

	return &RedelegationRequest{
		DelegatorAddress:    delegatorAddr.Bytes(),
		ValidatorSrcAddress: validatorSrcAddr,
		ValidatorDstAddress: validatorDstAddr,
	}, nil
}

// NewRedelegationsBech32Request create a new QueryRedelegationsRequest instance for Bech32 addresses and does sanity checks
// on the given arguments before populating the request.
func NewRedelegationsBech32Request(method *abi.Method, args []interface{}, addrCdc address.Codec) (*stakingtypes.QueryRedelegationsRequest, error) {
	if len(args) != 4 {
		return nil, fmt.Errorf(cmn.ErrInvalidNumberOfArgs, 4, len(args))
	}

	var input RedelegationsInput
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, fmt.Errorf("error while unpacking args to RedelegationsInput struct: %s", err)
	}

	var (
		delegatorAddr = ""
		emptyAddr     = common.Address{}.Hex()
	)
	if input.DelegatorAddress.Hex() != emptyAddr {
		var err error
		delegatorAddr, err = addrCdc.BytesToString(input.DelegatorAddress.Bytes())
		if err != nil {
			return nil, fmt.Errorf("failed to decode delegator address: %w", err)
		}
	}

	if delegatorAddr == "" && input.SrcValidatorAddress == "" && input.DstValidatorAddress == "" ||
		delegatorAddr == "" && input.SrcValidatorAddress == "" && input.DstValidatorAddress != "" {
		return nil, errors.New("invalid query. Need to specify at least a source validator address or delegator address")
	}

	return &stakingtypes.QueryRedelegationsRequest{
		DelegatorAddr:    delegatorAddr,
		SrcValidatorAddr: input.SrcValidatorAddress,
		DstValidatorAddr: input.DstValidatorAddress,
		Pagination:       &input.PageRequest,
	}, nil
}

// FormatConsensusPubkey format ConsensusPubkey into a base64 string
func FormatConsensusPubkey(consensusPubkey *codectypes.Any) string {
	ed25519pk, ok := consensusPubkey.GetCachedValue().(cryptotypes.PubKey)
	if ok {
		return base64.StdEncoding.EncodeToString(ed25519pk.Bytes())
	}
	return consensusPubkey.String()
}
