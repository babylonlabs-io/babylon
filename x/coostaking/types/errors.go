package types

import (
	errorsmod "cosmossdk.io/errors"
)

// x/coostaking module sentinel errors
var (
	ErrInvalidScoreRatioBtcByBaby = errorsmod.Register(ModuleName, 1102, "score ratio of btc to baby is invalid")
	ErrScoreRatioTooLow           = errorsmod.Register(ModuleName, 1103, "score ratio of btc to baby should be higher or equal 1")
	ErrInvalidPercentage          = errorsmod.Register(ModuleName, 1104, "percentage is invalid")
	ErrPercentageTooHigh          = errorsmod.Register(ModuleName, 1105, "percentage should be less or equal 1")
)
