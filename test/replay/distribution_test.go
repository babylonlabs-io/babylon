package replay

import (
	"math/rand"
	"testing"
	"time"

	sdkmath "cosmossdk.io/math"
	minttypes "github.com/babylonlabs-io/babylon/v2/x/mint/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/distribution/types"
	"github.com/stretchr/testify/require"
)

const MaxSupply = "115792089237316195423570985008687907853269984665640564039457584007913129639935"

func TestCumulativeRewardsOverflow(t *testing.T) {
	t.Parallel()
	r := rand.New(rand.NewSource(time.Now().Unix()))
	d := NewBabylonAppDriverTmpDir(r, t)
	d.GenerateNewBlockAssertExecutionSuccess()

	// mint the token and transfer to another acc
	rDenom := "ibc/65D0BEC6DAD96C7F5043D1E54E54B6BB5D5B3AEC3FF6CEBB75B9E059F3580EA3" // any IBC denom

	// check that only has one validator
	vals, err := d.App.StakingKeeper.GetValidators(d.Ctx(), 3)
	require.NoError(t, err)
	require.Len(t, vals, 1)
	val := vals[0]
	valAddr, err := sdk.ValAddressFromBech32(val.OperatorAddress)
	require.NoError(t, err)

	valAccAddr := sdk.AccAddress(valAddr)

	maxSupplyInt, ok := sdkmath.NewIntFromString(MaxSupply)
	require.True(t, ok)

	coinMaxSupply := sdk.NewCoin(rDenom, maxSupplyInt)
	maxDumbCoin := sdk.NewCoins(coinMaxSupply)

	// mints the max supply of the dumb token
	err = d.App.MintKeeper.MintCoins(d.Ctx(), maxDumbCoin)
	require.NoError(t, err)

	err = d.App.BankKeeper.SendCoinsFromModuleToAccount(d.Ctx(), minttypes.ModuleName, valAccAddr, maxDumbCoin)
	require.NoError(t, err)

	d.GenerateNewBlockAssertExecutionSuccess()

	msgSvrDstr, msgSvrEpoch := d.MsgSrvrDstr(), d.MsgSrvrEpoching()
	require.NotNil(t, msgSvrEpoch)

	// checks the delegation exists
	del, err := d.App.StakingKeeper.Delegation(d.Ctx(), valAccAddr, valAddr)
	require.NoError(t, err)
	require.NotNil(t, del)

	// distribute the funds to the reward pool
	_, err = msgSvrDstr.DepositValidatorRewardsPool(d.Ctx(), types.NewMsgDepositValidatorRewardsPool(valAccAddr.String(), valAddr.String(), maxDumbCoin))
	require.NoError(t, err)
	for i := 0; i < 20; i++ {
		d.GenerateNewBlockAssertExecutionSuccess()
	}

	// withdraw commission and rewards
	_, err = msgSvrDstr.WithdrawValidatorCommission(d.Ctx(), types.NewMsgWithdrawValidatorCommission(valAddr.String()))
	require.NoError(t, err)
	_, err = msgSvrDstr.WithdrawDelegatorReward(d.Ctx(), types.NewMsgWithdrawDelegatorReward(valAccAddr.String(), valAddr.String()))
	require.NoError(t, err)
	for i := 0; i < 20; i++ {
		d.GenerateNewBlockAssertExecutionSuccess()
	}

	// deposits again
	rewardsAmountInt, ok := sdkmath.NewIntFromString("5963292596005040462609982330006597585015759079040119860848489729157008230953")
	require.True(t, ok)
	dumbCoinToDeposit := sdk.NewCoin(rDenom, rewardsAmountInt)

	_, err = msgSvrDstr.DepositValidatorRewardsPool(d.Ctx(), types.NewMsgDepositValidatorRewardsPool(valAccAddr.String(), valAddr.String(), sdk.NewCoins(dumbCoinToDeposit)))
	require.NoError(t, err)
	d.GenerateNewBlockAssertExecutionSuccess()
	for i := 0; i < 20; i++ {
		d.GenerateNewBlockAssertExecutionSuccess()
	}

	// simulates the slash
	slashFractionDoubleSign, err := d.App.SlashingKeeper.SlashFractionDoubleSign(d.Ctx())
	require.NoError(t, err)

	valI, err := d.App.StakingKeeper.Validator(d.Ctx(), valAddr)
	require.NoError(t, err)
	err = d.App.SlashingKeeper.Slash(d.Ctx(), sdk.ConsAddress(valAddr), slashFractionDoubleSign, valI.GetConsensusPower(d.App.StakingKeeper.PowerReduction(d.Ctx())), d.Ctx().BlockHeader().Height+1)
	require.NoError(t, err)

	for i := 0; i < 20; i++ {
		d.GenerateNewBlockAssertExecutionSuccess()
	}
}
