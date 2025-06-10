package keeper_test

import (
	"encoding/hex"
	"math/rand"
	"testing"

	"cosmossdk.io/math"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/types/query"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/babylonlabs-io/babylon/v4/app"
	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	testhelper "github.com/babylonlabs-io/babylon/v4/testutil/helper"
	testkeeper "github.com/babylonlabs-io/babylon/v4/testutil/keeper"
	"github.com/babylonlabs-io/babylon/v4/testutil/mocks"
	checkpointingkeeper "github.com/babylonlabs-io/babylon/v4/x/checkpointing/keeper"
	"github.com/babylonlabs-io/babylon/v4/x/checkpointing/types"
	epochingtypes "github.com/babylonlabs-io/babylon/v4/x/epoching/types"
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
		genesisVal := ek.GetValidatorSet(helper.Ctx, 1)[0]
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
		require.Equal(t, res.ValidatorWithBlsKeys[0].BlsPubKeyHex, hex.EncodeToString(genesisBLSPubkey.Bytes()))
		require.Equal(t, res.ValidatorWithBlsKeys[0].VotingPower, uint64(1000))
		require.Equal(t, res.ValidatorWithBlsKeys[0].ValidatorAddress, genesisVal.GetValAddressStr())

		// add n new validators via MsgWrappedCreateValidator
		n := r.Intn(3) + 1
		addrs, err := app.AddTestAddrs(helper.App, helper.Ctx, n, math.NewInt(100000000))
		require.NoError(t, err)

		wcvMsgs := make([]*types.MsgWrappedCreateValidator, n)
		for i := 0; i < n; i++ {
			msg, err := datagen.BuildMsgWrappedCreateValidator(addrs[i])
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

func TestBlsPublicKeyList(t *testing.T) {
	epochWithVals := uint64(1)
	epochWithoutVals := epochWithVals + 10
	expTotalVals := uint64(4)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	vals, _ := datagen.GenerateValidatorSetWithBLSPrivKeys(int(expTotalVals))
	expReturn := make([]epochingtypes.Validator, len(vals.ValSet))
	for i, v := range vals.ValSet {
		valAddr, err := v.Addr()
		require.NoError(t, err)

		expReturn[i] = epochingtypes.Validator{
			Addr:  valAddr,
			Power: int64(v.VotingPower),
		}
	}

	ek := mocks.NewMockEpochingKeeper(ctrl)
	ek.EXPECT().GetValidatorSet(gomock.Any(), epochWithVals).Return(expReturn).AnyTimes()
	ek.EXPECT().GetValidatorSet(gomock.Any(), epochWithoutVals).Return(epochingtypes.ValidatorSet{}).AnyTimes()

	k, ctx, _ := testkeeper.CheckpointingKeeper(t, ek, nil)

	for _, v := range vals.ValSet {
		valAddr, err := v.Addr()
		require.NoError(t, err)

		k.RegistrationState(ctx).CreateRegistration(v.BlsPubKey, valAddr)
	}

	tcs := []struct {
		name        string
		req         *types.QueryBlsPublicKeyListRequest
		expectErr   error
		expectCount uint64
		offset      uint64
		total       uint64
	}{
		{
			name: "no pagination",
			req: &types.QueryBlsPublicKeyListRequest{
				EpochNum:   1,
				Pagination: nil,
			},
			expectErr:   nil,
			expectCount: expTotalVals,
			total:       expTotalVals,
		},
		{
			name: "limit 2 offset 0",
			req: &types.QueryBlsPublicKeyListRequest{
				EpochNum: epochWithVals,
				Pagination: &query.PageRequest{
					Limit:  2,
					Offset: 0,
				},
			},
			expectErr:   nil,
			expectCount: 2,
			offset:      0,
			total:       expTotalVals,
		},
		{
			name: "limit 2 offset 2",
			req: &types.QueryBlsPublicKeyListRequest{
				EpochNum: epochWithVals,
				Pagination: &query.PageRequest{
					Limit:  2,
					Offset: 2,
				},
			},
			expectErr:   nil,
			expectCount: 2,
			offset:      2,
			total:       expTotalVals,
		},
		{
			name: "offset beyond total",
			req: &types.QueryBlsPublicKeyListRequest{
				EpochNum: epochWithVals,
				Pagination: &query.PageRequest{
					Limit:  2,
					Offset: 10,
				},
			},
			expectErr: status.Errorf(codes.InvalidArgument, "pagination offset out of range: offset %d higher than total %d", 10, expTotalVals),
		},
		{
			name: "limit zero returns all from offset",
			req: &types.QueryBlsPublicKeyListRequest{
				EpochNum: epochWithVals,
				Pagination: &query.PageRequest{
					Limit:  0,
					Offset: 1,
				},
			},
			expectErr:   nil,
			expectCount: 3,
			offset:      1,
			total:       expTotalVals,
		},
		{
			name: "offset equal than total",
			req: &types.QueryBlsPublicKeyListRequest{
				EpochNum: epochWithVals,
				Pagination: &query.PageRequest{
					Offset: expTotalVals,
				},
			},
			expectErr:   nil,
			expectCount: 0, // set the offset to the limit
			offset:      expTotalVals,
			total:       expTotalVals,
		},
		{
			name: "epoch without vals",
			req: &types.QueryBlsPublicKeyListRequest{
				EpochNum: epochWithoutVals,
			},
			expectErr:   nil,
			expectCount: 0,
			offset:      0,
			total:       expTotalVals,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := k.BlsPublicKeyList(ctx, tc.req)
			if tc.expectErr != nil {
				require.EqualError(t, err, tc.expectErr.Error())
				return
			}

			require.NoError(t, err)
			require.Len(t, resp.ValidatorWithBlsKeys, int(tc.expectCount))

			if tc.req.Pagination != nil {
				require.NotNil(t, resp.Pagination)
				require.Equal(t, tc.total, resp.Pagination.Total)
			}
		})
	}
}
