package v1_test

import (
	"testing"

	v1 "github.com/babylonlabs-io/babylon/v4/app/upgrades/v1"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/stretchr/testify/require"
)

func TestLoadAllowedStakingTxHashesFromData(t *testing.T) {
	for _, upgradeData := range UpgradeV1Data {
		d, err := v1.LoadAllowedStakingTransactionHashesFromData(upgradeData.AllowedStakingTxHashesStr)
		require.NoError(t, err)
		require.NotNil(t, d)
		require.Greater(t, len(d.TxHashes), 0)

		for _, txHash := range d.TxHashes {
			_, err := chainhash.NewHashFromStr(txHash)
			require.NoError(t, err)
		}
	}
}
