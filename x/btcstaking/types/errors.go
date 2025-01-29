package types

import (
	errorsmod "cosmossdk.io/errors"
)

// x/btcstaking module sentinel errors
var (
	ErrFpNotFound                = errorsmod.Register(ModuleName, 1100, "the finality provider is not found")
	ErrBTCDelegatorNotFound      = errorsmod.Register(ModuleName, 1101, "the BTC delegator is not found")
	ErrBTCDelegationNotFound     = errorsmod.Register(ModuleName, 1102, "the BTC delegation is not found")
	ErrFpRegistered              = errorsmod.Register(ModuleName, 1103, "the finality provider has already been registered")
	ErrFpAlreadySlashed          = errorsmod.Register(ModuleName, 1104, "the finality provider has already been slashed")
	ErrBTCHeightNotFound         = errorsmod.Register(ModuleName, 1105, "the BTC height is not found")
	ErrReusedStakingTx           = errorsmod.Register(ModuleName, 1106, "the BTC staking tx is already used")
	ErrInvalidCovenantPK         = errorsmod.Register(ModuleName, 1107, "the BTC staking tx specifies a wrong covenant PK")
	ErrInvalidStakingTx          = errorsmod.Register(ModuleName, 1108, "the BTC staking tx is not valid")
	ErrInvalidSlashingTx         = errorsmod.Register(ModuleName, 1109, "the BTC slashing tx is not valid")
	ErrInvalidCovenantSig        = errorsmod.Register(ModuleName, 1110, "the covenant signature is not valid")
	ErrCommissionLTMinRate       = errorsmod.Register(ModuleName, 1111, "commission cannot be less than min rate")
	ErrCommissionGTMaxRate       = errorsmod.Register(ModuleName, 1112, "commission cannot be more than one")
	ErrInvalidDelegationState    = errorsmod.Register(ModuleName, 1113, "Unexpected delegation state")
	ErrInvalidUnbondingTx        = errorsmod.Register(ModuleName, 1114, "the BTC unbonding tx is not valid")
	ErrEmptyFpList               = errorsmod.Register(ModuleName, 1115, "the finality provider list is empty")
	ErrTooManyFpKeys             = errorsmod.Register(ModuleName, 1116, "the finality provider list contains too many public keys, it must contain exactly one")
	ErrInvalidProofOfPossession  = errorsmod.Register(ModuleName, 1117, "the proof of possession is not valid")
	ErrDuplicatedFp              = errorsmod.Register(ModuleName, 1118, "the staking request contains duplicated finality provider public key")
	ErrInvalidBTCUndelegateReq   = errorsmod.Register(ModuleName, 1119, "invalid undelegation request")
	ErrParamsNotFound            = errorsmod.Register(ModuleName, 1120, "the parameters are not found")
	ErrFpAlreadyJailed           = errorsmod.Register(ModuleName, 1121, "the finality provider has already been jailed")
	ErrFpNotJailed               = errorsmod.Register(ModuleName, 1122, "the finality provider is not jailed")
	ErrDuplicatedCovenantSig     = errorsmod.Register(ModuleName, 1123, "the covenant signature is already submitted")
	ErrStakingTxIncludedTooEarly = errorsmod.Register(ModuleName, 1124, "the staking transaction is included too early in BTC chain")
	ErrConsumerIDNotRegistered   = errorsmod.Register(ModuleName, 1125, "Consumer is not registered")
	ErrNoBabylonFPRestaked       = errorsmod.Register(ModuleName, 1126, "the BTC delegation request does not restake to any Babylon finality provider")
)
