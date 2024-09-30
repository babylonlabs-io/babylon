package v1_test

import (
	"testing"

	v1 "github.com/babylonlabs-io/babylon/app/upgrades/v1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
)

func TestCheckTokensDistributionFromData(t *testing.T) {
	d, err := v1.LoadTokenDistributionFromData()
	require.NoError(t, err)
	require.Greater(t, len(d.TokenDistribution), 1)

	for _, td := range d.TokenDistribution {
		sender, err := sdk.AccAddressFromBech32(td.AddressSender)
		require.NoError(t, err)
		require.Equal(t, sender.String(), td.AddressSender)

		receiver, err := sdk.AccAddressFromBech32(td.AddressReceiver)
		require.NoError(t, err)
		require.Equal(t, receiver.String(), td.AddressReceiver)

		require.True(t, td.Amount > 0)
	}
}
