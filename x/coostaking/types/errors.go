package types

import (
	errorsmod "cosmossdk.io/errors"
)

// x/coostaking module sentinel errors
var (
	ErrInvalidCoostakingPortion   = errorsmod.Register(ModuleName, 1100, "CoostakingPortion should not be nil")
	ErrCoostakingPortionTooHigh   = errorsmod.Register(ModuleName, 1101, "coostaking portion should be less or equal 1")
	ErrInvalidScoreRatioBtcByBaby = errorsmod.Register(ModuleName, 1102, "ScoreRatioBtcByBaby should not be nil")
	ErrScoreRatioTooLow           = errorsmod.Register(ModuleName, 1103, "score ratio of btc to baby should be higher or equal 1")
)