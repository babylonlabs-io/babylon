package replay

import (
	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
)

func (d *BabylonAppDriver) CheckCostakerRewards(
	addr sdk.AccAddress,
	expActiveBaby, expActiveSats, expTotalScore sdkmath.Int,
	expStartPeriod uint64,
) {
	costkK := d.App.CostakingKeeper

	del, err := costkK.GetCostakerRewards(d.Ctx(), addr)
	require.NoError(d.t, err)
	require.Equal(d.t, del.ActiveBaby.String(), expActiveBaby.String())
	require.Equal(d.t, del.ActiveSatoshis.String(), expActiveSats.String())
	require.Equal(d.t, del.StartPeriodCumulativeReward, expStartPeriod)
	require.Equal(d.t, del.TotalScore.String(), expTotalScore.String())
}
