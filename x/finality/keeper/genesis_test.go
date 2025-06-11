package keeper_test

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"

	"github.com/babylonlabs-io/babylon/v3/testutil/datagen"
	keepertest "github.com/babylonlabs-io/babylon/v3/testutil/keeper"
	bbn "github.com/babylonlabs-io/babylon/v3/types"
	"github.com/babylonlabs-io/babylon/v3/x/finality/types"
)

func FuzzTestExportGenesis(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))
		k, ctx := keepertest.FinalityKeeper(t, nil, nil, nil)

		btcSK, btcPK, err := datagen.GenRandomBTCKeyPair(r)
		require.NoError(t, err)

		fpBTCPK := bbn.NewBIP340PubKeyFromBTCPK(btcPK)
		blkHeight, startHeight, numPubRand := uint64(1), uint64(0), uint64(5)

		randListInfo, _, err := datagen.GenRandomMsgCommitPubRandList(r, btcSK, startHeight, numPubRand)
		require.NoError(t, err)

		blockHash := datagen.GenRandomByteArray(r, 32)
		signer := datagen.GenRandomAccount().Address
		msgAddFinalitySig, err := datagen.NewMsgAddFinalitySig(signer, btcSK, startHeight, blkHeight, randListInfo, blockHash)
		require.NoError(t, err)

		allVotes := make([]*types.VoteSig, numPubRand)
		allBlocks := make([]*types.IndexedBlock, numPubRand)
		allEvidences := make([]*types.Evidence, numPubRand)
		allPublicRandomness := make([]*types.PublicRandomness, numPubRand)

		for i := 0; i < int(numPubRand); i++ {
			// Votes
			vt := &types.VoteSig{
				FpBtcPk:     fpBTCPK,
				BlockHeight: blkHeight,
				FinalitySig: msgAddFinalitySig.FinalitySig,
			}
			k.SetSig(ctx, vt.BlockHeight, vt.FpBtcPk, vt.FinalitySig)
			allVotes[i] = vt

			// Blocks
			blk := &types.IndexedBlock{
				Height:    blkHeight,
				AppHash:   blockHash,
				Finalized: i%2 == 0,
			}
			k.SetBlock(ctx, blk)
			allBlocks[i] = blk

			// Evidences
			evidence := &types.Evidence{
				FpBtcPk:              fpBTCPK,
				BlockHeight:          blkHeight,
				PubRand:              &randListInfo.PRList[i],
				ForkAppHash:          msgAddFinalitySig.BlockAppHash,
				ForkFinalitySig:      msgAddFinalitySig.FinalitySig,
				CanonicalAppHash:     blockHash,
				CanonicalFinalitySig: msgAddFinalitySig.FinalitySig,
			}
			k.SetEvidence(ctx, evidence)
			allEvidences[i] = evidence

			// public randomness
			pubRand := randListInfo.PRList[i]
			k.SetPubRand(ctx, fpBTCPK, blkHeight, pubRand)
			randomness := &types.PublicRandomness{
				BlockHeight: blkHeight,
				FpBtcPk:     fpBTCPK,
				PubRand:     &pubRand,
			}
			allPublicRandomness[i] = randomness

			// updates the block every time to make sure something is different.
			blkHeight++
		}

		prc := &types.PubRandCommit{
			StartHeight: startHeight,
			NumPubRand:  numPubRand,
			Commitment:  randListInfo.Commitment,
		}
		require.NoError(t, k.SetPubRandCommit(ctx, fpBTCPK, prc))
		pubRandCommitIdx := &types.PubRandCommitIdx{
			FpBtcPk: fpBTCPK,
			Index: &types.PubRandCommitIndexValue{
				Heights: []uint64{startHeight},
			},
		}

		numSigningInfo := datagen.RandomInt(r, 100) + 10
		fpSigningInfos := map[string]*types.FinalityProviderSigningInfo{}
		fpPks := make([]string, 0)
		for i := uint64(0); i < numSigningInfo; i++ {
			// random key pair
			fpPk, err := datagen.GenRandomBIP340PubKey(r)
			require.NoError(t, err)
			fpPks = append(fpPks, fpPk.MarshalHex())

			// random height and missed block counter
			height := int64(datagen.RandomInt(r, 100) + 1)
			missedBlockCounter := int64(datagen.RandomInt(r, 100) + 1)

			// create signing info and add it to map and finality keeper
			signingInfo := types.NewFinalityProviderSigningInfo(fpPk, height, missedBlockCounter)
			err = k.FinalityProviderSigningTracker.Set(ctx, fpPk.MustMarshal(), signingInfo)
			require.NoError(t, err)
			fpSigningInfos[fpPk.MarshalHex()] = &signingInfo
		}

		numFps := datagen.RandomInt(r, 10) + 1
		fps := datagen.CreateNFinalityProviders(r, t, int(numFps))
		vpFps := make(map[string]*types.VotingPowerFP, 0)
		for _, fp := range fps {
			vp := datagen.RandomInt(r, 1000000)
			// sets voting power
			k.SetVotingPower(ctx, *fp.BtcPk, blkHeight, vp)
			vpFps[fp.BtcPk.MarshalHex()] = &types.VotingPowerFP{
				BlockHeight: blkHeight,
				FpBtcPk:     fp.BtcPk,
				VotingPower: vp,
			}
		}

		require.Equal(t, len(allVotes), int(numPubRand))
		require.Equal(t, len(allBlocks), int(numPubRand))
		require.Equal(t, len(allEvidences), int(numPubRand))
		require.Equal(t, len(allPublicRandomness), int(numPubRand))

		gs, err := k.ExportGenesis(ctx)
		require.NoError(t, err)
		require.Equal(t, k.GetParams(ctx), gs.Params)

		require.Equal(t, allVotes, gs.VoteSigs)
		require.Equal(t, allBlocks, gs.IndexedBlocks)
		require.Equal(t, allEvidences, gs.Evidences)
		require.Equal(t, allPublicRandomness, gs.PublicRandomness)
		require.Equal(t, prc, gs.PubRandCommit[0].PubRandCommit)
		require.Equal(t, pubRandCommitIdx, gs.PubRandCommitIndexes[0])
		require.Equal(t, len(fpPks), len(gs.SigningInfos))
		for _, info := range gs.SigningInfos {
			require.Equal(t, fpSigningInfos[info.FpBtcPk.MarshalHex()].MissedBlocksCounter, info.FpSigningInfo.MissedBlocksCounter)
			require.Equal(t, fpSigningInfos[info.FpBtcPk.MarshalHex()].StartHeight, info.FpSigningInfo.StartHeight)
		}

		require.Equal(t, len(vpFps), len(gs.VotingPowers))
		for _, fpVp := range gs.VotingPowers {
			require.Equal(t, vpFps[fpVp.FpBtcPk.MarshalHex()], fpVp)
		}
	})
}

func FuzzTestInitGenesis(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))
		k, ctx := keepertest.FinalityKeeper(t, nil, nil, nil)
		gs, err := datagen.GenRandomFinalityGenesisState(r)
		require.NoError(t, err)

		// Run the InitGenesis
		err = k.InitGenesis(ctx, *gs)
		require.NoError(t, err)

		// get the current state
		exported, err := k.ExportGenesis(ctx)
		require.NoError(t, err)

		types.SortData(gs)
		types.SortData(exported)

		require.Equal(t, gs, exported, fmt.Sprintf("Found diff: %s | seed %d", cmp.Diff(gs, exported), seed))
	})
}
