package types

import (
	errorsmod "cosmossdk.io/errors"
)

// x/incentive module sentinel errors
var (
	ErrBTCStakingGaugeNotFound                   = errorsmod.Register(ModuleName, 1100, "BTC staking gauge not found")
	ErrRewardGaugeNotFound                       = errorsmod.Register(ModuleName, 1101, "reward gauge not found")
	ErrNoWithdrawableCoins                       = errorsmod.Register(ModuleName, 1102, "no coin is withdrawable")
	ErrFPCurrentRewardsNotFound                  = errorsmod.Register(ModuleName, 1103, "finality provider current rewards not found")
	ErrFPHistoricalRewardsNotFound               = errorsmod.Register(ModuleName, 1104, "finality provider historical rewards not found")
	ErrBTCDelegationRewardsTrackerNotFound       = errorsmod.Register(ModuleName, 1105, "BTC delegation rewards tracker not found")
	ErrBTCDelegationRewardsTrackerNegativeAmount = errorsmod.Register(ModuleName, 1106, "BTC delegation rewards tracker has a negative amount of TotalActiveSat")
)
