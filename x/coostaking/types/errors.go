package types

import (
	errorsmod "cosmossdk.io/errors"
)

// x/coostaking module sentinel errors
var (
	ErrInvalidCoostakingPortion   = errorsmod.Register(ModuleName, 1100, "coostaking portion is invalid")
	ErrCoostakingPortionTooHigh   = errorsmod.Register(ModuleName, 1101, "coostaking portion should be less or equal 1")
	ErrInvalidScoreRatioBtcByBaby = errorsmod.Register(ModuleName, 1102, "score ratio of btc to baby is invalid")
	ErrScoreRatioTooLow           = errorsmod.Register(ModuleName, 1103, "score ratio of btc to baby should be higher or equal 1")
	ErrInvalidCurrentRewards      = errorsmod.Register(ModuleName, 1104, "current rewards is invalid")
)
