package v1_test

import (
	"testing"

	v1 "github.com/babylonlabs-io/babylon/v4/app/upgrades/v1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
)

func TestCheckTokensDistributionFromData(t *testing.T) {
	for _, upgradeData := range UpgradeV1Data {
		d, err := v1.LoadTokenDistributionFromData(upgradeData.TokensDistributionStr)
		require.NoError(t, err)

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
}
