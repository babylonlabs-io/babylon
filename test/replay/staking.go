package replay

import (
	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	appparams "github.com/babylonlabs-io/babylon/v4/app/params"
	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
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

func (d *BabylonAppDriver) TxCreateValidator(operator *SenderInfo, valAddr string, amount sdkmath.Int) {
	msgCreateValidator, err := datagen.BuildMsgWrappedCreateValidatorWithAmount(operator.Address(), amount)
	if err != nil {
		d.t.Fatal(err)
	}
	msgCreateValidator.MsgCreateValidator.Commission = stktypes.NewCommissionRates(
		sdkmath.LegacyMustNewDecFromStr("0.1"),
		sdkmath.LegacyMustNewDecFromStr("0.9"),
		sdkmath.LegacyMustNewDecFromStr("0.05"),
	)

	d.SendTxWithMessagesSuccess(d.t, operator, DefaultGasLimit, defaultFeeCoin, msgCreateValidator)
	operator.IncSeq()
}

func (d *BabylonAppDriver) TxWrappedUndelegate(delegator *SenderInfo, valAddr string, amount sdkmath.Int) {
	msgUndelegate := stktypes.NewMsgUndelegate(
		delegator.AddressString(), valAddr, sdk.NewCoin(appparams.DefaultBondDenom, amount),
	)

	msg := epochingtypes.NewMsgWrappedUndelegate(msgUndelegate)
	d.SendTxWithMessagesSuccess(d.t, delegator, DefaultGasLimit, defaultFeeCoin, msg)
	delegator.IncSeq()
}
