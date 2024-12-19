package keeper_test

import (
	"math/rand"
	"testing"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/babylonlabs-io/babylon/testutil/datagen"
	keepertest "github.com/babylonlabs-io/babylon/testutil/keeper"
	bbntypes "github.com/babylonlabs-io/babylon/types"
	"github.com/babylonlabs-io/babylon/x/finality/keeper"
	"github.com/babylonlabs-io/babylon/x/finality/types"
)

func TestHandleResumeFinalityProposal(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	bsKeeper := types.NewMockBTCStakingKeeper(ctrl)
	iKeeper := types.NewMockIncentiveKeeper(ctrl)
	cKeeper := types.NewMockCheckpointingKeeper(ctrl)
	fKeeper, ctx := keepertest.FinalityKeeper(t, bsKeeper, iKeeper, cKeeper)

	haltingHeight := uint64(100)
	currentHeight := uint64(110)

	activeFpNum := 3
	activeFpPks := generateNFpPks(t, r, activeFpNum)
	setupActiveFps(t, activeFpPks, haltingHeight, fKeeper, ctx)
	// set voting power table for each height, only the first fp votes
	votedFpPk := activeFpPks[0]
	for h := haltingHeight; h <= currentHeight; h++ {
		fKeeper.SetBlock(ctx, &types.IndexedBlock{
			Height:    h,
			AppHash:   datagen.GenRandomByteArray(r, 32),
			Finalized: false,
		})
		dc := types.NewVotingPowerDistCache()
		for i := 0; i < activeFpNum; i++ {
			fKeeper.SetVotingPower(ctx, activeFpPks[i].MustMarshal(), h, 1)
			dc.AddFinalityProviderDistInfo(&types.FinalityProviderDistInfo{
				BtcPk:          &activeFpPks[i],
				TotalBondedSat: 1,
				IsTimestamped:  true,
			})
		}
		dc.ApplyActiveFinalityProviders(uint32(activeFpNum))
		votedSig, err := bbntypes.NewSchnorrEOTSSig(datagen.GenRandomByteArray(r, 32))
		require.NoError(t, err)
		fKeeper.SetSig(ctx, h, &votedFpPk, votedSig)
		fKeeper.SetVotingPowerDistCache(ctx, h, dc)
	}

	// tally blocks and none of them should be finalised
	iKeeper.EXPECT().RewardBTCStaking(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return().AnyTimes()
	ctx = datagen.WithCtxHeight(ctx, currentHeight)
	fKeeper.TallyBlocks(ctx)
	for i := haltingHeight; i < currentHeight; i++ {
		ib, err := fKeeper.GetBlock(ctx, i)
		require.NoError(t, err)
		require.False(t, ib.Finalized)
	}

	// create a resume finality proposal to jail the last fp
	bsKeeper.EXPECT().JailFinalityProvider(ctx, gomock.Any()).Return(nil).AnyTimes()
	err := fKeeper.HandleResumeFinalityProposal(ctx, publicKeysToHex(activeFpPks[1:]), uint32(haltingHeight))
	require.NoError(t, err)

	for i := haltingHeight; i < currentHeight; i++ {
		ib, err := fKeeper.GetBlock(ctx, i)
		require.NoError(t, err)
		require.True(t, ib.Finalized)
	}
}

func generateNFpPks(t *testing.T, r *rand.Rand, n int) []bbntypes.BIP340PubKey {
	fpPks := make([]bbntypes.BIP340PubKey, 0, n)
	for i := 0; i < n; i++ {
		fpPk, err := datagen.GenRandomBIP340PubKey(r)
		require.NoError(t, err)
		fpPks = append(fpPks, *fpPk)
	}

	return fpPks
}

func publicKeysToHex(pks []bbntypes.BIP340PubKey) []string {
	hexPks := make([]string, len(pks))
	for i, pk := range pks {
		hexPks[i] = pk.MarshalHex()
	}
	return hexPks
}

func setupActiveFps(t *testing.T, fpPks []bbntypes.BIP340PubKey, height uint64, fKeeper *keeper.Keeper, ctx sdk.Context) {
	for _, fpPk := range fpPks {
		signingInfo := types.NewFinalityProviderSigningInfo(
			&fpPk,
			int64(height),
			0,
		)
		err := fKeeper.FinalityProviderSigningTracker.Set(ctx, fpPk, signingInfo)
		require.NoError(t, err)
	}
}
