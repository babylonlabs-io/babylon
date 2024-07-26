package keeper_test

import (
	"math/rand"
	"testing"

	"cosmossdk.io/math"
	"github.com/cometbft/cometbft/crypto/ed25519"
	"github.com/cosmos/cosmos-sdk/crypto/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/stretchr/testify/require"

	"github.com/babylonlabs-io/babylon/app"
	appparams "github.com/babylonlabs-io/babylon/app/params"
	"github.com/babylonlabs-io/babylon/crypto/bls12381"
	"github.com/babylonlabs-io/babylon/privval"
	"github.com/babylonlabs-io/babylon/testutil/datagen"
	testhelper "github.com/babylonlabs-io/babylon/testutil/helper"
	checkpointingkeeper "github.com/babylonlabs-io/babylon/x/checkpointing/keeper"
	"github.com/babylonlabs-io/babylon/x/checkpointing/types"
	epochingtypes "github.com/babylonlabs-io/babylon/x/epoching/types"
)

// FuzzWrappedCreateValidator_InsufficientTokens tests adding new validators with zero voting power
// It ensures that validators with zero voting power (i.e., with tokens fewer than sdk.DefaultPowerReduction)
// are unbonded, thus are not included in the validator set
func FuzzWrappedCreateValidator_InsufficientTokens(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 4)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		// a genesis validator is generate for setup
		helper := testhelper.NewHelper(t)
		ctx := helper.Ctx
		ek := helper.App.EpochingKeeper
		ck := helper.App.CheckpointingKeeper
		msgServer := checkpointingkeeper.NewMsgServerImpl(ck)

		// epoch 1 right now
		epoch := ek.GetEpoch(ctx)
		require.Equal(t, uint64(1), epoch.EpochNumber)

		n := r.Intn(3) + 1
		addrs, err := app.AddTestAddrs(helper.App, helper.Ctx, n, math.NewInt(100000000))
		require.NoError(t, err)

		// add n new validators with zero voting power via MsgWrappedCreateValidator
		wcvMsgs := make([]*types.MsgWrappedCreateValidator, n)
		for i := 0; i < n; i++ {
			msg, err := buildMsgWrappedCreateValidatorWithAmount(addrs[i], sdk.DefaultPowerReduction.SubRaw(1))
			require.NoError(t, err)
			wcvMsgs[i] = msg
			_, err = msgServer.WrappedCreateValidator(ctx, msg)
			require.NoError(t, err)
			blsPK, err := ck.GetBlsPubKey(ctx, sdk.ValAddress(addrs[i]))
			require.NoError(t, err)
			require.True(t, msg.Key.Pubkey.Equal(blsPK))
		}
		require.Len(t, ek.GetCurrentEpochMsgs(ctx), n)

		// go to block 11, and thus entering epoch 2
		for i := uint64(0); i < ek.GetParams(ctx).EpochInterval; i++ {
			ctx, err = helper.ApplyEmptyBlockWithVoteExtension(r)
			require.NoError(t, err)
		}
		epoch = ek.GetEpoch(ctx)
		require.Equal(t, uint64(2), epoch.EpochNumber)
		// ensure epoch 2 has initialised an empty msg queue
		require.Empty(t, ek.GetCurrentEpochMsgs(ctx))

		// ensure the length of current validator set equals to 1
		// since one genesis validator was added when setup
		// the rest n validators have zero voting power and thus are ruled out
		valSet = ck.GetValidatorSet(ctx, 2)
		require.Equal(t, 1, len(valSet))

		// ensure all validators (not just validators in the val set) have correct bond status
		// - the 1st validator is bonded
		// - all the rest are unbonded since they have zero voting power
		iterator, err := helper.App.StakingKeeper.ValidatorsPowerStoreIterator(ctx)
		require.NoError(t, err)
		defer iterator.Close()
		count := 0
		for ; iterator.Valid(); iterator.Next() {
			valAddr := sdk.ValAddress(iterator.Value())
			val, err := helper.App.StakingKeeper.GetValidator(ctx, valAddr)
			require.NoError(t, err)
			count++
			if count == 1 {
				require.Equal(t, stakingtypes.Bonded, val.Status)
			} else {
				require.Equal(t, stakingtypes.Unbonded, val.Status)
			}
		}
		require.Equal(t, len(wcvMsgs)+1, count)
	})
}

// FuzzWrappedCreateValidator_InsufficientBalance tests adding a new validator
// but the delegator has insufficient balance to perform delegating
func FuzzWrappedCreateValidator_InsufficientBalance(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 4)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		// a genesis validator is generate for setup
		helper := testhelper.NewHelper(t)
		ctx := helper.Ctx
		ek := helper.App.EpochingKeeper
		ck := helper.App.CheckpointingKeeper
		msgServer := checkpointingkeeper.NewMsgServerImpl(ck)

		// epoch 1 right now
		epoch := ek.GetEpoch(ctx)
		require.Equal(t, uint64(1), epoch.EpochNumber)

		n := r.Intn(3) + 1
		balance := r.Int63n(100)
		addrs, err := app.AddTestAddrs(helper.App, helper.Ctx, n, math.NewInt(balance))
		require.NoError(t, err)

		// add n new validators with value more than the delegator balance via MsgWrappedCreateValidator
		wcvMsgs := make([]*types.MsgWrappedCreateValidator, n)
		for i := 0; i < n; i++ {
			// make sure the value is more than the balance
			value := math.NewInt(balance).Add(math.NewInt(r.Int63n(100)))
			msg, err := buildMsgWrappedCreateValidatorWithAmount(addrs[i], value)
			require.NoError(t, err)
			wcvMsgs[i] = msg
			_, err = msgServer.WrappedCreateValidator(ctx, msg)
			require.ErrorIs(t, err, epochingtypes.ErrInsufficientBalance)
		}
	})
}

// FuzzWrappedCreateValidator tests adding new validators via
// MsgWrappedCreateValidator, which first registers BLS pubkey
// and then unwrapped into MsgCreateValidator and enqueued into
// the epoching module, and delivered to the staking module
// at epoch ends for execution
func FuzzWrappedCreateValidator(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 4)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		// a genesis validator is generate for setup
		helper := testhelper.NewHelper(t)
		ctx := helper.Ctx
		ek := helper.App.EpochingKeeper
		ck := helper.App.CheckpointingKeeper
		msgServer := checkpointingkeeper.NewMsgServerImpl(ck)

		// epoch 1 right now
		epoch := ek.GetEpoch(ctx)
		require.Equal(t, uint64(1), epoch.EpochNumber)

		// add n new validators via MsgWrappedCreateValidator
		n := r.Intn(3)
		addrs, err := app.AddTestAddrs(helper.App, helper.Ctx, n, math.NewInt(100000000))
		require.NoError(t, err)

		wcvMsgs := make([]*types.MsgWrappedCreateValidator, n)
		for i := 0; i < n; i++ {
			msg, err := buildMsgWrappedCreateValidator(addrs[i])
			require.NoError(t, err)
			wcvMsgs[i] = msg
			_, err = msgServer.WrappedCreateValidator(ctx, msg)
			require.NoError(t, err)
			blsPK, err := ck.GetBlsPubKey(ctx, sdk.ValAddress(addrs[i]))
			require.NoError(t, err)
			require.True(t, msg.Key.Pubkey.Equal(blsPK))
		}
		require.Len(t, ek.GetCurrentEpochMsgs(ctx), n)

		// go to block 11, and thus entering epoch 2
		for i := uint64(0); i < ek.GetParams(ctx).EpochInterval; i++ {
			ctx, err = helper.ApplyEmptyBlockWithVoteExtension(r)
			require.NoError(t, err)
		}
		epoch = ek.GetEpoch(ctx)
		require.Equal(t, uint64(2), epoch.EpochNumber)
		// ensure epoch 2 has initialised an empty msg queue
		require.Empty(t, ek.GetCurrentEpochMsgs(ctx))

		// check whether the length of current validator set equals to 1 + n
		// since one genesis validator was added when setup
		valSet = ck.GetValidatorSet(ctx, 2)
		require.Equal(t, len(wcvMsgs)+1, len(valSet))
		for _, msg := range wcvMsgs {
			found := false
			for _, val := range valSet {
				if msg.MsgCreateValidator.ValidatorAddress == val.GetValAddressStr() {
					found = true
				}
			}
			require.True(t, found)
		}
	})
}

func buildMsgWrappedCreateValidator(addr sdk.AccAddress) (*types.MsgWrappedCreateValidator, error) {
	bondTokens := sdk.TokensFromConsensusPower(10, sdk.DefaultPowerReduction)
	return buildMsgWrappedCreateValidatorWithAmount(addr, bondTokens)
}

func buildMsgWrappedCreateValidatorWithAmount(addr sdk.AccAddress, bondTokens math.Int) (*types.MsgWrappedCreateValidator, error) {
	cmtValPrivkey := ed25519.GenPrivKey()
	bondCoin := sdk.NewCoin(appparams.DefaultBondDenom, bondTokens)
	description := stakingtypes.NewDescription("foo_moniker", "", "", "", "")
	commission := stakingtypes.NewCommissionRates(math.LegacyZeroDec(), math.LegacyZeroDec(), math.LegacyZeroDec())

	pk, err := codec.FromCmtPubKeyInterface(cmtValPrivkey.PubKey())
	if err != nil {
		return nil, err
	}

	createValidatorMsg, err := stakingtypes.NewMsgCreateValidator(
		sdk.ValAddress(addr).String(), pk, bondCoin, description, commission, math.OneInt(),
	)
	if err != nil {
		return nil, err
	}
	blsPrivKey := bls12381.GenPrivKey()
	pop, err := privval.BuildPoP(cmtValPrivkey, blsPrivKey)
	if err != nil {
		return nil, err
	}
	blsPubKey := blsPrivKey.PubKey()

	return types.NewMsgWrappedCreateValidator(createValidatorMsg, &blsPubKey, pop)
}
