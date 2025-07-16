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

	consumerID := "bsn-consumer"
	d.App.IBCKeeper.ClientKeeper.SetClientState(d.Ctx(), consumerID, &ibctmtypes.ClientState{})
	d.GenerateNewBlock()

	consumer := d.RegisterConsumer(r, consumerID)
	d.GenerateNewBlockAssertExecutionSuccess()

	babylonFp := d.CreateNFinalityProviderAccounts(1)[0]
	babylonFp.RegisterFinalityProvider("")

	consumerFp := []*FinalityProvider{
		d.CreateFinalityProviderForConsumer(consumer),
		d.CreateFinalityProviderForConsumer(consumer),
	}

	staker := d.CreateNStakerAccounts(1)[0]
	staker.CreatePreApprovalDelegation(
		[]*bbn.BIP340PubKey{consumerFp[0].BTCPublicKey(), consumerFp[1].BTCPublicKey(), babylonFp.BTCPublicKey()},
		1000,
		100000000,
	)

	d.GenerateNewBlockAssertExecutionSuccess()

	covSender.SendCovenantSignatures()
	d.GenerateNewBlockAssertExecutionSuccess()

	d.ActivateVerifiedDelegations(1)
	activeDelegations := d.GetActiveBTCDelegations(t)
	require.Len(t, activeDelegations, 1)

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
	totalRewards := mintCoins.Add(sdk.NewCoin("ubbn", math.NewInt(1_000000)))

	conFpAddr0, conFpAddr1 := consumerFp[0].Address(), consumerFp[1].Address()
	conFpAddrStr0, conFpAddrStr1 := conFpAddr0.String(), conFpAddr1.String()
	stkAddrStr, bbnCommAddrStr := staker.AddressString(), params.AccBbnComissionCollectorBsn.String()

	// verify the balances change
	addrs := []sdk.AccAddress{staker.Address(), conFpAddr0, conFpAddr1, params.AccBbnComissionCollectorBsn}
	beforeWithdrawBalances := d.BankBalances(addrs...)

	d.SendBsnRewards(consumer.ID, totalRewards, fpRatios)
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
	expBbnCommission := ictvtypes.GetCoinsPortion(totalRewards, consumer.BabylonCommission)
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
}

// SendBsnRewards sends BSN rewards using MsgAddBsnRewards
func (d *BabylonAppDriver) SendBsnRewards(consumerID string, totalRewards sdk.Coins, fpRatios []types.FpRatio) {
	msg := &types.MsgAddBsnRewards{
		Sender:        d.GetDriverAccountAddress().String(),
		BsnConsumerId: consumerID,
		TotalRewards:  totalRewards,
		FpRatios:      fpRatios,
	}

	d.SendTxWithMsgsFromDriverAccount(d.t, msg)
}
