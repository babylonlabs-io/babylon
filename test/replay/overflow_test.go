package replay

import (
	"math/rand"
	"testing"
	"time"

	sdkmath "cosmossdk.io/math"

	sec256k1 "github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stkkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	stktypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/stretchr/testify/require"
)

func TestOverflowIntSlash(t *testing.T) {
	t.Parallel()
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	d := NewBabylonAppDriverTmpDir(r, t)
	d.GenerateNewBlock()

	vals, err := d.App.StakingKeeper.GetValidators(d.Ctx(), 10)
	require.NoError(t, err)
	require.Len(t, vals, 1)

	stkMsgSvr := stkkeeper.NewMsgServerImpl(d.App.StakingKeeper)
	newValSk := sec256k1.GenPrivKey()
	valAccAddr := newValSk.PubKey().Address().Bytes()
	valAddr := sdk.ValAddress(valAccAddr)

	// d.App.BankKeeper.SendCoins(d.Ctx())

	// datagen.GenRandomMsgCreateFinalityProvider()
	_, err = stkMsgSvr.CreateValidator(d.Ctx(), &stktypes.MsgCreateValidator{
		Description:       stktypes.NewDescription("", "", "", "", ""),
		Commission:        stktypes.NewCommissionRates(sdkmath.LegacyZeroDec(), sdkmath.LegacyZeroDec(), sdkmath.LegacyZeroDec()),
		MinSelfDelegation: sdkmath.NewInt(1),
		ValidatorAddress:  valAddr.String(),
	})
	require.NoError(t, err)
}
