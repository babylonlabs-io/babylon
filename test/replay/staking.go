package replay

import (
	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	appparams "github.com/babylonlabs-io/babylon/v4/app/params"
	epochingtypes "github.com/babylonlabs-io/babylon/v4/x/epoching/types"
	stktypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

func (d *BabylonAppDriver) TxWrappedDelegate(delegator *SenderInfo, valAddr string, amount sdkmath.Int) {
	msgDelegate := stktypes.NewMsgDelegate(
		delegator.AddressString(), valAddr, sdk.NewCoin(appparams.DefaultBondDenom, amount),
	)

	msg := epochingtypes.NewMsgWrappedDelegate(msgDelegate)
	d.SendTxWithMessagesSuccess(d.t, delegator, DefaultGasLimit, defaultFeeCoin, msg)
	delegator.IncSeq()
}
