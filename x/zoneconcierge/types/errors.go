package types

import (
	errorsmod "cosmossdk.io/errors"
)

// x/zoneconcierge module sentinel errors
var (
	ErrInvalidVersion          = errorsmod.Register(ModuleName, 1101, "invalid version")
	ErrConsumerInfoNotFound    = errorsmod.Register(ModuleName, 1102, "no consumer info exists at this epoch")
	ErrEpochHeadersNotFound    = errorsmod.Register(ModuleName, 1103, "no timestamped header exists at this epoch")
	ErrInvalidProofEpochSealed = errorsmod.Register(ModuleName, 1104, "invalid ProofEpochSealed")
	ErrInvalidMerkleProof      = errorsmod.Register(ModuleName, 1105, "invalid Merkle inclusion proof")
	ErrInvalidConsumerIDs      = errorsmod.Register(ModuleName, 1106, "chain ids contain duplicates or empty strings")
)
