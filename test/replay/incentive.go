package replay

import (
	ictvkeeper "github.com/babylonlabs-io/babylon/v4/x/incentive/keeper"
	ictvtypes "github.com/babylonlabs-io/babylon/v4/x/incentive/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/test-go/testify/require"
)

func (d *BabylonAppDriver) GenerateBlocksUntilLastProcessedBtcStkEventsHeightIs(untilBlock uint64) {
	ictvK := d.App.IncentiveKeeper

	lastProcessedBtcStkEvtsHeight, err := ictvK.GetRewardTrackerEventLastProcessedHeight(d.Ctx())
	require.NoError(d.t, err)

	for lastProcessedBtcStkEvtsHeight < untilBlock {
		d.GenerateNewBlockAssertExecutionSuccess()
		lastProcessedBtcStkEvtsHeight, err = ictvK.GetRewardTrackerEventLastProcessedHeight(d.Ctx())
		require.NoError(d.t, err)
	}
}

func (d *BabylonAppDriver) MsgServerIncentive() ictvtypes.MsgServer {
	return ictvkeeper.NewMsgServerImpl(d.App.IncentiveKeeper)
}

func (s *StandardScenario) WithdrawBtcDelRewards() {
	for _, staker := range s.stakers {
		addr := staker.Address()

		msg := &ictvtypes.MsgWithdrawReward{
			Type:    ictvtypes.BTC_STAKER.String(),
			Address: addr.String(),
		}

		DefaultSendTxWithMessagesSuccess(
			staker.t,
			staker.app,
			staker.SenderInfo,
			msg,
		)
		staker.IncSeq()
	}
}

func (s *StandardScenario) IctvWithdrawBtcStakerRewardsByAddr() map[string]sdk.Coin {
	d := s.driver
	balancesByAddrBeforeWithdrawReward := d.BankBalanceBond(s.StakersAddr()...)

	s.WithdrawBtcDelRewards()
	d.GenerateNewBlockAssertExecutionSuccess()

	balancesByAddrAfterWithdrawReward := d.BankBalanceBond(s.StakersAddr()...)

	rewards := make(map[string]sdk.Coin, len(balancesByAddrAfterWithdrawReward))
	for addr, bAfter := range balancesByAddrAfterWithdrawReward {
		bBefore := balancesByAddrBeforeWithdrawReward[addr]
		amtBeforeMinusTxFee := bBefore.Sub(defaultFeeCoin)

		rewards[addr] = bAfter.Sub(amtBeforeMinusTxFee)
	}

	return rewards
}
