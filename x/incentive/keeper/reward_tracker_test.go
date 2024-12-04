package keeper_test

import (
	"testing"

	"cosmossdk.io/math"
	"github.com/babylonlabs-io/babylon/testutil/datagen"
	keepertest "github.com/babylonlabs-io/babylon/testutil/keeper"
	"github.com/golang/mock/gomock"
)

func TestXxx(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	fpAddr, delAddr := datagen.GenRandomAddress(), datagen.GenRandomAddress()

	k, ctx := keepertest.IncentiveKeeper(t, nil, nil, nil)
	k.AddDelegationSat(ctx, fpAddr, delAddr, math.NewInt(2000))
}
