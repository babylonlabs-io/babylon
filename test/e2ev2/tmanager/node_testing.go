package tmanager

import (
	sdkmath "cosmossdk.io/math"
	costktypes "github.com/babylonlabs-io/babylon/v4/x/costaking/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
)

func (n *Node) CheckCostaking(addr sdk.AccAddress, expActiveSats, expActiveBaby, expTotalScore sdkmath.Int) *costktypes.QueryCostakerRewardsTrackerResponse {
	costk := n.QueryCostkRwdTrckCli(addr)
	require.Equal(n.T(), expActiveSats.String(), costk.ActiveSatoshis.String(), "addr %s should have active sats: %s, but has %s", addr.String(), expActiveSats.String(), costk.ActiveSatoshis.String())
	require.Equal(n.T(), expActiveBaby.String(), costk.ActiveBaby.String(), "addr %s should have active baby: %s, but has %s", addr.String(), expActiveBaby.String(), costk.ActiveBaby.String())
	require.Equal(n.T(), expTotalScore.String(), costk.TotalScore.String(), "addr %s should have total score: %s, but has %s", addr.String(), expTotalScore.String(), costk.TotalScore.String())
	return costk
}
