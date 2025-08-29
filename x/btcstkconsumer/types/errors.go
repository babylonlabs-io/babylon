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
	ErrInvalidRollupConsumerRequest = sdkerrors.Register(ModuleName, 1105, "invalid registration request of rollup consumer")
	ErrFinalityContractAlreadyRegistered = sdkerrors.Register(ModuleName, 1106, "finality contract already registered with another consumer")
)
