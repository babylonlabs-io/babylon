package types

// DONTCOVER

import (
	sdkerrors "cosmossdk.io/errors"
)

// x/btcstkconsumer module sentinel errors
var (
	ErrInvalidSigner                = sdkerrors.Register(ModuleName, 1100, "expected gov account as only signer for proposal message")
	ErrConsumerNotRegistered        = sdkerrors.Register(ModuleName, 1101, "consumer not registered")
	ErrInvalidConsumerRegister      = sdkerrors.Register(ModuleName, 1102, "invalid consumer register")
	ErrConsumerAlreadyRegistered    = sdkerrors.Register(ModuleName, 1103, "consumer already registered")
	ErrInvalidConsumerIDs           = sdkerrors.Register(ModuleName, 1104, "consumer ids contain duplicates or empty strings")
	ErrInvalidCosmosConsumerRequest = sdkerrors.Register(ModuleName, 1105, "invalid registration request of Cosmos consumer")
	ErrInvalidETHL2ConsumerRequest  = sdkerrors.Register(ModuleName, 1106, "invalid registration request of ETH L2 consumer")
)
