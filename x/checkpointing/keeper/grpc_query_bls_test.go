package keeper_test

import (
	"math/rand"
	"testing"

	"cosmossdk.io/math"

	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/types/query"
	"github.com/stretchr/testify/require"

	"github.com/babylonlabs-io/babylon/app"
	"github.com/babylonlabs-io/babylon/testutil/datagen"
	testhelper "github.com/babylonlabs-io/babylon/testutil/helper"
	checkpointingkeeper "github.com/babylonlabs-io/babylon/x/checkpointing/keeper"
	"github.com/babylonlabs-io/babylon/x/checkpointing/types"
)

// FuzzQueryBLSKeySet does the following checks
// 1. check the query when there's only a genesis validator
// 2. check the query when there are n+1 validators without pagination
// 3. check the query when there are n+1 validators with pagination
func FuzzQueryBLSKeySet(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))
		// a genesis validator is generated for setup
		helper := testhelper.NewHelper(t)
		ctx := helper.Ctx
		ek := helper.App.EpochingKeeper
		ck := helper.App.CheckpointingKeeper
		queryHelper := baseapp.NewQueryServerTestHelper(helper.Ctx, helper.App.InterfaceRegistry())
		types.RegisterQueryServer(queryHelper, ck)
		queryClient := types.NewQueryClient(queryHelper)
		msgServer := checkpointingkeeper.NewMsgServerImpl(ck)
		genesisVal := ek.GetValidatorSet(helper.Ctx, 0)[0]
		genesisBLSPubkey, err := ck.GetBlsPubKey(helper.Ctx, genesisVal.Addr)
		require.NoError(t, err)

		epoch := ek.GetEpoch(ctx)
		require.Equal(t, uint64(1), epoch.EpochNumber)

		// 1. query public key list when there's only a genesis validator
		queryRequest := &types.QueryBlsPublicKeyListRequest{
			EpochNum: 1,
		}
		res, err := queryClient.BlsPublicKeyList(ctx, queryRequest)
		require.NoError(t, err)
		require.Len(t, res.ValidatorWithBlsKeys, 1)
		require.Equal(t, res.ValidatorWithBlsKeys[0].BlsPubKey, genesisBLSPubkey.Bytes())
		require.Equal(t, res.ValidatorWithBlsKeys[0].VotingPower, uint64(1000))
		require.Equal(t, res.ValidatorWithBlsKeys[0].ValidatorAddress, genesisVal.GetValAddressStr())

		// add n new validators via MsgWrappedCreateValidator
		n := r.Intn(3) + 1
		addrs, err := app.AddTestAddrs(helper.App, helper.Ctx, n, math.NewInt(100000000))
		require.NoError(t, err)

		wcvMsgs := make([]*types.MsgWrappedCreateValidator, n)
		for i := 0; i < n; i++ {
			msg, err := buildMsgWrappedCreateValidator(addrs[i])
			require.NoError(t, err)
			wcvMsgs[i] = msg
			_, err = msgServer.WrappedCreateValidator(ctx, msg)
			require.NoError(t, err)
		}

		// go to block 11, and thus entering epoch 2
		for i := uint64(0); i < ek.GetParams(ctx).EpochInterval; i++ {
			ctx, err = helper.ApplyEmptyBlockWithVoteExtension(r)
			require.NoError(t, err)
		}
		epoch = ek.GetEpoch(ctx)
		require.Equal(t, uint64(2), epoch.EpochNumber)

		// 2. query BLS public keys when there are n+1 validators without pagination
		req := types.QueryBlsPublicKeyListRequest{
			EpochNum: 2,
		}
		resp, err := queryClient.BlsPublicKeyList(ctx, &req)
		require.NoError(t, err)
		require.Len(t, resp.ValidatorWithBlsKeys, n+1)
		expectedValSet := ek.GetValidatorSet(ctx, 2)
		require.Len(t, expectedValSet, n+1)
		for i, expectedVal := range expectedValSet {
			require.Equal(t, uint64(expectedVal.Power), resp.ValidatorWithBlsKeys[i].VotingPower)
			require.Equal(t, expectedVal.GetValAddressStr(), resp.ValidatorWithBlsKeys[i].ValidatorAddress)
		}

		// 3.1 query BLS public keys when there are n+1 validators with limit pagination
		req = types.QueryBlsPublicKeyListRequest{
			EpochNum: 2,
			Pagination: &query.PageRequest{
				Limit: 1,
			},
		}
		resp, err = queryClient.BlsPublicKeyList(ctx, &req)
		require.NoError(t, err)
		require.Len(t, resp.ValidatorWithBlsKeys, 1)

		// 3.2 query BLS public keys when there are n+1 validators with offset pagination
		req = types.QueryBlsPublicKeyListRequest{
			EpochNum: 2,
			Pagination: &query.PageRequest{
				Offset: 1,
			},
		}
		resp, err = queryClient.BlsPublicKeyList(ctx, &req)
		require.NoError(t, err)
		require.Len(t, resp.ValidatorWithBlsKeys, n)
	})
}
