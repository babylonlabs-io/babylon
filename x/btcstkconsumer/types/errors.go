package types

// DONTCOVER

import (
	sdkerrors "cosmossdk.io/errors"
)

// x/btcstkconsumer module sentinel errors
var (
	ErrInvalidSigner                = sdkerrors.Register(ModuleName, 1100, "expected gov account as only signer for proposal message")
	ErrConsumerNotRegistered        = sdkerrors.Register(ModuleName, 1101, "consumer not registered")
	ErrConsumerAlreadyRegistered    = sdkerrors.Register(ModuleName, 1102, "consumer already registered")
	ErrInvalidConsumerIDs           = sdkerrors.Register(ModuleName, 1103, "consumer ids contain duplicates or empty strings")
	ErrInvalidCosmosConsumerRequest = sdkerrors.Register(ModuleName, 1104, "invalid registration request of Cosmos consumer")
	ErrInvalidETHL2ConsumerRequest  = sdkerrors.Register(ModuleName, 1105, "invalid registration request of ETH L2 consumer")
	ErrInvalidMaxMultiStakedFps     = sdkerrors.Register(ModuleName, 1106, "max multi staked fps must be at least 2 to allow for at least one Babylon FP and one consumer FP")
	ErrEmptyConsumerId              = sdkerrors.Register(ModuleName, 1107, "consumer id must be non-empty")
	ErrEmptyConsumerName            = sdkerrors.Register(ModuleName, 1108, "consumer name must be non-empty")
	ErrEmptyConsumerDescription     = sdkerrors.Register(ModuleName, 1109, "consumer description must be non-empty")
)
