package types

import (
	errorsmod "cosmossdk.io/errors"
)

// x/finality module sentinel errors
var (
	ErrBlockNotFound                  = errorsmod.Register(ModuleName, 1100, "Block is not found")
	ErrVoteNotFound                   = errorsmod.Register(ModuleName, 1101, "vote is not found")
	ErrHeightTooHigh                  = errorsmod.Register(ModuleName, 1102, "the chain has not reached the given height yet")
	ErrPubRandNotFound                = errorsmod.Register(ModuleName, 1103, "public randomness is not found")
	ErrNoPubRandYet                   = errorsmod.Register(ModuleName, 1104, "the finality provider has not committed any public randomness yet")
	ErrTooFewPubRand                  = errorsmod.Register(ModuleName, 1105, "the request contains too few public randomness")
	ErrInvalidPubRand                 = errorsmod.Register(ModuleName, 1106, "the public randomness list is invalid")
	ErrEvidenceNotFound               = errorsmod.Register(ModuleName, 1107, "evidence is not found")
	ErrInvalidFinalitySig             = errorsmod.Register(ModuleName, 1108, "finality signature is not valid")
	ErrDuplicatedFinalitySig          = errorsmod.Register(ModuleName, 1109, "duplicated finality vote")
	ErrNoSlashableEvidence            = errorsmod.Register(ModuleName, 1110, "there is no slashable evidence")
	ErrPubRandCommitNotBTCTimestamped = errorsmod.Register(ModuleName, 1111, "the public randomness commit is not BTC timestamped yet")
	ErrJailingPeriodNotPassed         = errorsmod.Register(ModuleName, 1112, "the jailing period is not passed")
	ErrVotingPowerTableNotUpdated     = errorsmod.Register(ModuleName, 1113, "voting power table has not been updated")
	ErrBTCStakingNotActivated         = errorsmod.Register(ModuleName, 1114, "the BTC staking protocol is not activated yet")
	ErrFinalityNotActivated           = errorsmod.Register(ModuleName, 1115, "finality is not active yet")
	ErrSigHeightOutdated              = errorsmod.Register(ModuleName, 1116, "the voting block is already finalized and timestamped")
)
