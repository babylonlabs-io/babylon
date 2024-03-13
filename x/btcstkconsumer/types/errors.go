package types

// DONTCOVER

import (
	sdkerrors "cosmossdk.io/errors"
)

// x/btcstkconsumer module sentinel errors
var (
	ErrInvalidSigner          = sdkerrors.Register(ModuleName, 1100, "expected gov account as only signer for proposal message")
	ErrChainNotRegistered     = sdkerrors.Register(ModuleName, 1101, "chain not registered")
	ErrInvalidChainRegister   = sdkerrors.Register(ModuleName, 1102, "invalid chain register")
	ErrChainAlreadyRegistered = sdkerrors.Register(ModuleName, 1103, "chain already registered")
	ErrInvalidChainIDs        = sdkerrors.Register(ModuleName, 1104, "chain ids contain duplicates or empty strings")
)
