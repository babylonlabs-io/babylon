package btcstaking_test

import (
	"math/rand"
	"testing"
	"time"

	"github.com/babylonlabs-io/babylon/v4/btcstaking"
	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	bbn "github.com/babylonlabs-io/babylon/v4/types"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/stretchr/testify/require"
)

func TestSortKeys(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().Unix()))

	_, pks, err := datagen.GenRandomBTCKeyPairs(r, 10)
	require.NoError(t, err)

	sortedPKs := btcstaking.SortKeys(pks)

	btcPKs := bbn.NewBIP340PKsFromBTCPKs(pks)
	sortedBTCPKs := bbn.SortBIP340PKs(btcPKs)

	// ensure sorted PKs and sorted BIP340 PKs are in reverse order
	for i := range sortedPKs {
		pkBytes := schnorr.SerializePubKey(sortedPKs[i])

		btcPK := sortedBTCPKs[len(sortedBTCPKs)-1-i]
		btcPKBytes := btcPK.MustMarshal()

		require.Equal(t, pkBytes, btcPKBytes, "comparing %d-th key", i)
	}
}
