package replay

import (
	"math/rand"
	"testing"
	"time"

	"cosmossdk.io/math"
	"github.com/babylonlabs-io/babylon/v3/app/params"
	"github.com/babylonlabs-io/babylon/v3/testutil/datagen"
	bbn "github.com/babylonlabs-io/babylon/v3/types"
	"github.com/babylonlabs-io/babylon/v3/x/btcstaking/types"
	ictvtypes "github.com/babylonlabs-io/babylon/v3/x/incentive/types"
	minttypes "github.com/babylonlabs-io/babylon/v3/x/mint/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	ibctmtypes "github.com/cosmos/ibc-go/v10/modules/light-clients/07-tendermint"
	"github.com/stretchr/testify/require"
)

func TestConsumerBsnRewardDistribution(t *testing.T) {
	t.Parallel()
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	d := NewBabylonAppDriverTmpDir(r, t)

	d.GenerateNewBlock()

	covSender := d.CreateCovenantSender()
	require.NotNil(t, covSender)

	consumerID := "bsn-consumer-0"
	d.App.IBCKeeper.ClientKeeper.SetClientState(d.Ctx(), consumerID, &ibctmtypes.ClientState{})
	d.GenerateNewBlock()

	consumer0 := d.RegisterConsumer(r, consumerID)
	d.GenerateNewBlockAssertExecutionSuccess()

	babylonFp := d.CreateNFinalityProviderAccounts(1)[0]
	babylonFp.RegisterFinalityProvider("")

	consumerFp := []*FinalityProvider{
		d.CreateFinalityProviderForConsumer(consumer0),
		d.CreateFinalityProviderForConsumer(consumer0),
		d.CreateFinalityProviderForConsumer(consumer0),
	}

	staker := d.CreateNStakerAccounts(1)[0]
	amtSatFp0 := int64(100000000)
	staker.CreatePreApprovalDelegation(
		[]*bbn.BIP340PubKey{consumerFp[0].BTCPublicKey(), babylonFp.BTCPublicKey()},
		1000,
		amtSatFp0,
	)
	staker.CreatePreApprovalDelegation(
		[]*bbn.BIP340PubKey{consumerFp[1].BTCPublicKey(), babylonFp.BTCPublicKey()},
		1000,
		200000000,
	)

	d.GenerateNewBlockAssertExecutionSuccess()

	covSender.SendCovenantSignatures()
	d.GenerateNewBlockAssertExecutionSuccess()

	d.ActivateVerifiedDelegations(2)
	activeDelegations := d.GetActiveBTCDelegations(t)
	require.Len(t, activeDelegations, 2)

	fpRatios := []types.FpRatio{
		{BtcPk: consumerFp[0].BTCPublicKey(), Ratio: math.LegacyMustNewDecFromStr("0.7")},
		{BtcPk: consumerFp[1].BTCPublicKey(), Ratio: math.LegacyMustNewDecFromStr("0.3")},
	}

	mintCoins := datagen.GenRandomCoins(r)
	err := d.App.MintKeeper.MintCoins(d.Ctx(), mintCoins)
	require.NoError(t, err)

	recipient := d.GetDriverAccountAddress()
	err = d.App.BankKeeper.SendCoinsFromModuleToAccount(d.Ctx(), minttypes.ModuleName, recipient, mintCoins)

	require.NoError(t, err)
	bbnRwd := sdk.NewCoin("ubbn", math.NewInt(1_000000))
	totalRewards := mintCoins.Add(bbnRwd)

	conFpAddr0, conFpAddr1 := consumerFp[0].Address(), consumerFp[1].Address()
	conFpAddrStr0, conFpAddrStr1 := conFpAddr0.String(), conFpAddr1.String()
	stkAddrStr, bbnCommAddrStr := staker.AddressString(), params.AccBbnComissionCollectorBsn.String()

	// verify the balances change
	addrs := []sdk.AccAddress{staker.Address(), conFpAddr0, conFpAddr1, params.AccBbnComissionCollectorBsn}
	beforeWithdrawBalances := d.BankBalances(addrs...)

	// send the BSN rewards
	d.AddBsnRewardsFromDriver(consumer0.ID, totalRewards, fpRatios)
	d.GenerateNewBlockAssertExecutionSuccess()

	consumerFp[0].WithdrawBtcStakingRewards()
	beforeWithdrawBalances[conFpAddrStr0] = beforeWithdrawBalances[conFpAddrStr0].Sub(defaultFeeCoin)

	consumerFp[1].WithdrawBtcStakingRewards()
	beforeWithdrawBalances[conFpAddrStr1] = beforeWithdrawBalances[conFpAddrStr1].Sub(defaultFeeCoin)

	staker.WithdrawBtcStakingRewards()
	beforeWithdrawBalances[stkAddrStr] = beforeWithdrawBalances[stkAddrStr].Sub(defaultFeeCoin)

	d.GenerateNewBlockAssertExecutionSuccess()

	afterWithdrawBalances := d.BankBalances(addrs...)

	balancesDiff := BankBalancesDiff(afterWithdrawBalances, beforeWithdrawBalances, addrs...)

	// check babylon commission
	expBbnCommission := ictvtypes.GetCoinsPortion(totalRewards, consumer0.BabylonCommission)
	require.Equal(t, expBbnCommission.String(), balancesDiff[bbnCommAddrStr].String())

	remaining := totalRewards.Sub(expBbnCommission...)
	// check commission fp consumer 0
	fpInfo0, err := d.App.BTCStakingKeeper.GetFinalityProvider(d.Ctx(), *consumerFp[0].BTCPublicKey())
	require.NoError(t, err)

	entitledToFp0 := ictvtypes.GetCoinsPortion(remaining, fpRatios[0].Ratio)
	commissionFp0 := ictvtypes.GetCoinsPortion(entitledToFp0, *fpInfo0.Commission)
	require.Equal(t, commissionFp0.String(), balancesDiff[conFpAddrStr0].String())

	// check commission fp consumer 1
	fpInfo1, err := d.App.BTCStakingKeeper.GetFinalityProvider(d.Ctx(), *consumerFp[1].BTCPublicKey())
	require.NoError(t, err)

	entitledToFp1 := ictvtypes.GetCoinsPortion(remaining, fpRatios[1].Ratio)
	commissionFp1 := ictvtypes.GetCoinsPortion(entitledToFp1, *fpInfo1.Commission)
	require.Equal(t, commissionFp1.String(), balancesDiff[conFpAddrStr1].String())

	// since there is only one btc staker, all the rest of the entitled goes to him
	expectedToEarnFromFp0 := entitledToFp0.Sub(commissionFp0...)
	expectedToEarnFromFp1 := entitledToFp1.Sub(commissionFp1...)
	expectedRewardsBtcStk := expectedToEarnFromFp0.Add(expectedToEarnFromFp1...)
	require.Equal(t, expectedRewardsBtcStk.String(), balancesDiff[stkAddrStr].String())

	// Spend funds from the babylon commission module account
	rReceiver := datagen.GenRandomAddress()
	err = d.App.BankKeeper.SendCoinsFromModuleToAccount(d.Ctx(), ictvtypes.ModAccCommissionCollectorBSN, rReceiver, expBbnCommission)
	require.NoError(t, err)

	receivedCoins := d.BankBalances(rReceiver)[rReceiver.String()]
	require.Equal(t, expBbnCommission.String(), receivedCoins.String())

	// send rewads to one finality provider that never received delegations
	basicRewards := sdk.NewCoins(bbnRwd)
	_, err = SendTxWithMessages(
		d.t,
		d.App,
		d.SenderInfo,
		&types.MsgAddBsnRewards{
			Sender:        d.SenderInfo.AddressString(),
			BsnConsumerId: consumer0.ID,
			TotalRewards:  basicRewards,
			FpRatios:      []types.FpRatio{types.FpRatio{BtcPk: consumerFp[2].BTCPublicKey(), Ratio: math.LegacyOneDec()}},
		},
	)
	d.SenderInfo.IncSeq()
	require.NoError(t, err)

	txResults := d.GenerateNewBlockAssertExecutionFailure()
	require.Len(t, txResults, 1)
	require.Contains(t, txResults[0].Log, "finality provider current rewards not found")

	// unbond one btc delegation until fp has zero vp and send bsn rewards to it.
	err = d.App.IncentiveKeeper.BtcDelegationUnbonded(d.Ctx(), conFpAddr0, staker.Address(), math.NewInt(amtSatFp0))
	require.NoError(t, err)

	d.GenerateNewBlockAssertExecutionSuccess()

	resp, err := SendTxWithMessages(
		d.t,
		d.App,
		d.SenderInfo,
		&types.MsgAddBsnRewards{
			Sender:        d.GetDriverAccountAddress().String(),
			BsnConsumerId: consumer0.ID,
			TotalRewards:  basicRewards,
			FpRatios:      fpRatios,
		},
	)
	require.NoError(t, err)
	require.NotNil(t, resp)

	txResults = d.GenerateNewBlockAssertExecutionFailure()
	require.Len(t, txResults, 1)
	require.Contains(t, txResults[0].Log, ictvtypes.ErrFPCurrentRewardsWithoutVotingPower.Error())
}

// AddBsnRewardsFromDriver sends BSN rewards using MsgAddBsnRewards
func (d *BabylonAppDriver) AddBsnRewardsFromDriver(consumerID string, totalRewards sdk.Coins, fpRatios []types.FpRatio) {
	d.AddBsnRewards(d.SenderInfo, consumerID, totalRewards, fpRatios)
}

// AddBsnRewards sends BSN rewards using MsgAddBsnRewards
func (d *BabylonAppDriver) AddBsnRewards(sender *SenderInfo, consumerID string, totalRewards sdk.Coins, fpRatios []types.FpRatio) {
	msg := &types.MsgAddBsnRewards{
		Sender:        sender.AddressString(),
		BsnConsumerId: consumerID,
		TotalRewards:  totalRewards,
		FpRatios:      fpRatios,
	}

	d.SendTxWithMessagesSuccess(d.t, sender, defaultGasLimit, defaultFeeCoin, msg)
	sender.IncSeq()
}
