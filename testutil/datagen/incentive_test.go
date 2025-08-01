package datagen_test

import (
	"math/rand"
	"testing"
	"time"

	sdkmath "cosmossdk.io/math"

	"github.com/babylonlabs-io/babylon/v3/testutil/datagen"
	"github.com/stretchr/testify/require"
)

func TestNewIntMaxSupply(t *testing.T) {
	maxSupply := datagen.NewIntMaxSupply()

	require.PanicsWithError(t, sdkmath.ErrIntOverflow.Error(), func() {
		maxSupply.AddRaw(1)
	})
}

func TestGenRandomCoinMaxSupply(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	c := datagen.GenRandomCoinMaxSupply(r)
	require.PanicsWithError(t, sdkmath.ErrIntOverflow.Error(), func() {
		c.AddAmount(sdkmath.NewInt(1))
	})
}
