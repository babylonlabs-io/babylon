package keeper_test

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"testing"
	"time"

	sdkmath "cosmossdk.io/math"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	sdk "github.com/cosmos/cosmos-sdk/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	appparams "github.com/babylonlabs-io/babylon/v4/app/params"
	testutil "github.com/babylonlabs-io/babylon/v4/testutil/btcstaking-helper"
	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	testutilevents "github.com/babylonlabs-io/babylon/v4/testutil/events"
	testhelper "github.com/babylonlabs-io/babylon/v4/testutil/helper"
	bbn "github.com/babylonlabs-io/babylon/v4/types"
	btcctypes "github.com/babylonlabs-io/babylon/v4/x/btccheckpoint/types"
	btclctypes "github.com/babylonlabs-io/babylon/v4/x/btclightclient/types"
	"github.com/babylonlabs-io/babylon/v4/x/btcstaking"
	"github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
	ftypes "github.com/babylonlabs-io/babylon/v4/x/finality/types"
)

func FuzzMsgServer_UpdateParams(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 500)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// mock BTC light client and BTC checkpoint modules
		btclcKeeper := types.NewMockBTCLightClientKeeper(ctrl)
		btccKeeper := types.NewMockBtcCheckpointKeeper(ctrl)
		h := testutil.NewHelper(t, btclcKeeper, btccKeeper, nil)

		// set all parameters
		h.GenAndApplyParams(r)

		params := h.BTCStakingKeeper.GetParams(h.Ctx)
		ckptFinalizationTimeout := btccKeeper.GetParams(h.Ctx).CheckpointFinalizationTimeout
		params.UnbondingTimeBlocks = uint32(r.Intn(int(ckptFinalizationTimeout))) + 1
		params.BtcActivationHeight++

		// Try to update params with minUnbondingTime less than or equal to checkpointFinalizationTimeout
		msg := &types.MsgUpdateParams{
			Authority: appparams.AccGov.String(),
			Params:    params,
		}

		_, err := h.MsgServer.UpdateParams(h.Ctx, msg)
		require.ErrorIs(t, err, govtypes.ErrInvalidProposalMsg,
			"should not set minUnbondingTime to be less than checkpointFinalizationTimeout")

		// Try to update params with minUnbondingTime larger than checkpointFinalizationTimeout
		msg.Params.UnbondingTimeBlocks = uint32(r.Intn(1000)) + ckptFinalizationTimeout + 1
		_, err = h.MsgServer.UpdateParams(h.Ctx, msg)
		require.NoError(t, err)
	})
}

func FuzzMsgCreateFinalityProvider(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// mock BTC light client and BTC checkpoint modules
		btclcKeeper := types.NewMockBTCLightClientKeeper(ctrl)
		btccKeeper := types.NewMockBtcCheckpointKeeper(ctrl)
		h := testutil.NewHelper(t, btclcKeeper, btccKeeper, nil)

		// set all parameters
		h.GenAndApplyParams(r)

		// generate new finality providers
		fps := []*types.FinalityProvider{}
		for i := 0; i < int(datagen.RandomInt(r, 20)+2); i++ {
			fp, err := datagen.GenRandomFinalityProvider(r)
			require.NoError(t, err)
			msg := &types.MsgCreateFinalityProvider{
				Addr:        fp.Addr,
				Description: fp.Description,
				Commission: types.NewCommissionRates(
					*fp.Commission,
					fp.CommissionInfo.MaxRate,
					fp.CommissionInfo.MaxChangeRate,
				),
				BtcPk: fp.BtcPk,
				Pop:   fp.Pop,
			}
			_, err = h.MsgServer.CreateFinalityProvider(h.Ctx, msg)
			require.NoError(t, err)

			fps = append(fps, fp)
		}
		// assert these finality providers exist in KVStore
		for _, fp := range fps {
			btcPK := *fp.BtcPk
			require.True(t, h.BTCStakingKeeper.HasFinalityProvider(h.Ctx, btcPK))
		}

		// duplicated finality providers should not pass
		for _, fp2 := range fps {
			msg := &types.MsgCreateFinalityProvider{
				Addr:        fp2.Addr,
				Description: fp2.Description,
				Commission: types.NewCommissionRates(
					*fp2.Commission,
					fp2.CommissionInfo.MaxRate,
					fp2.CommissionInfo.MaxChangeRate,
				),
				BtcPk: fp2.BtcPk,
				Pop:   fp2.Pop,
			}
			_, err := h.MsgServer.CreateFinalityProvider(h.Ctx, msg)
			require.Error(t, err)
		}

		// tries to create another fp with same bbn address as an registered one
		fp, err := datagen.GenRandomFinalityProvider(r)
		require.NoError(t, err)
		dupFpAddr := fps[0].Addr

		btcSK, _, err := datagen.GenRandomBTCKeyPair(r)
		require.NoError(t, err)
		pop, err := datagen.NewPoPBTC(sdk.MustAccAddressFromBech32(dupFpAddr), btcSK)
		require.NoError(t, err)
		btcPK := btcSK.PubKey()
		bip340PK := bbn.NewBIP340PubKeyFromBTCPK(btcPK)

		msg := &types.MsgCreateFinalityProvider{
			Addr:        dupFpAddr,
			Description: fp.Description,
			Commission: types.NewCommissionRates(
				*fp.Commission,
				fp.CommissionInfo.MaxRate,
				fp.CommissionInfo.MaxChangeRate,
			),
			BtcPk: bip340PK,
			Pop:   pop,
		}
		_, dupFpBbnAddrErr := h.MsgServer.CreateFinalityProvider(h.Ctx, msg)
		require.EqualError(t, dupFpBbnAddrErr, types.ErrFpRegistered.Wrapf("there is already an finality provider registered with the same babylon address: %s", dupFpAddr).Error())
	})
}

func FuzzMsgEditFinalityProvider(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// mock BTC light client and BTC checkpoint modules
		btclcKeeper := types.NewMockBTCLightClientKeeper(ctrl)
		btccKeeper := types.NewMockBtcCheckpointKeeper(ctrl)
		h := testutil.NewHelper(t, btclcKeeper, btccKeeper, nil)

		h.GenAndApplyParams(r)

		// insert the finality provider
		_, _, fp := h.CreateFinalityProvider(r)
		// assert the finality providers exist in KVStore
		require.True(t, h.BTCStakingKeeper.HasFinalityProvider(h.Ctx, *fp.BtcPk))

		// updated commission and description
		newCommission := datagen.GenRandomCommission(r)
		newDescription := datagen.GenRandomDescription(r)

		// scenario 1: editing finality provider should succeed
		// Note that, on finality provider creation, the commission update time is set to the current block time.
		// So we need to update block time to be after 24hs to edit the commission
		h.Ctx = h.Ctx.WithBlockTime(h.Ctx.BlockTime().Add(25 * time.Hour))
		msg := &types.MsgEditFinalityProvider{
			Addr:        fp.Addr,
			BtcPk:       *fp.BtcPk,
			Description: newDescription,
			Commission:  &newCommission,
		}
		_, err := h.MsgServer.EditFinalityProvider(h.Ctx, msg)
		h.NoError(err)
		editedFp, err := h.BTCStakingKeeper.GetFinalityProvider(h.Ctx, *fp.BtcPk)
		h.NoError(err)
		require.Equal(t, newCommission, *editedFp.Commission)
		require.Equal(t, newDescription, editedFp.Description)

		// scenario 2: message from an unauthorised signer should fail
		newCommission = datagen.GenRandomCommission(r)
		newDescription = datagen.GenRandomDescription(r)
		invalidAddr := datagen.GenRandomAccount().Address
		msg = &types.MsgEditFinalityProvider{
			Addr:        invalidAddr,
			BtcPk:       *fp.BtcPk,
			Description: newDescription,
			Commission:  &newCommission,
		}
		_, err = h.MsgServer.EditFinalityProvider(h.Ctx, msg)
		require.Equal(h.T(), err, status.Errorf(codes.PermissionDenied, "the signer does not correspond to the finality provider's Babylon address"))
		errStatus := status.Convert(err)
		require.Equal(h.T(), codes.PermissionDenied, errStatus.Code())
	})
}

func FuzzCreateBTCDelegation(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// mock BTC light client and BTC checkpoint modules
		btclcKeeper := types.NewMockBTCLightClientKeeper(ctrl)
		btccKeeper := types.NewMockBtcCheckpointKeeper(ctrl)
		h := testutil.NewHelper(t, btclcKeeper, btccKeeper, nil)

		// set all parameters
		h.GenAndApplyParams(r)

		// generate and insert new finality provider
		_, fpPK, _ := h.CreateFinalityProvider(r)

		usePreApproval := datagen.OneInN(r, 2)

		// generate and insert new BTC delegation
		stakingValue := int64(2 * 10e8)
		delSK, _, err := datagen.GenRandomBTCKeyPair(r)
		h.NoError(err)

		var stakingTxHash string
		var msgCreateBTCDel *types.MsgCreateBTCDelegation
		if usePreApproval {
			stakingTxHash, msgCreateBTCDel, _, _, _, _, err = h.CreateDelegationWithBtcBlockHeight(
				r,
				delSK,
				fpPK,
				stakingValue,
				1000,
				0,
				0,
				usePreApproval,
				false,
				10,
				10,
			)
			h.NoError(err)
		} else {
			stakingTxHash, msgCreateBTCDel, _, _, _, _, err = h.CreateDelegationWithBtcBlockHeight(
				r,
				delSK,
				fpPK,
				stakingValue,
				1000,
				0,
				0,
				usePreApproval,
				false,
				10,
				30,
			)
			h.NoError(err)
		}

		// ensure consistency between the msg and the BTC delegation in DB
		actualDel, err := h.BTCStakingKeeper.GetBTCDelegation(h.Ctx, stakingTxHash)
		h.NoError(err)
		require.Equal(h.T(), msgCreateBTCDel.StakerAddr, actualDel.StakerAddr)
		require.Equal(h.T(), msgCreateBTCDel.Pop, actualDel.Pop)
		require.Equal(h.T(), msgCreateBTCDel.StakingTx, actualDel.StakingTx)
		require.Equal(h.T(), msgCreateBTCDel.SlashingTx, actualDel.SlashingTx)
		// actual btc delegation has a field `MultisigInfo`
		require.Nil(h.T(), actualDel.MultisigInfo)
		// ensure the BTC delegation in DB is correctly formatted
		err = actualDel.ValidateBasic()
		h.NoError(err)
		// delegation is not activated by covenant yet
		hasQuorum, err := h.BTCStakingKeeper.BtcDelHasCovenantQuorums(h.Ctx, actualDel, h.BTCStakingKeeper.GetParams(h.Ctx).CovenantQuorum)
		h.NoError(err)
		require.False(h.T(), hasQuorum)

		if usePreApproval {
			require.Zero(h.T(), actualDel.StartHeight)
			require.Zero(h.T(), actualDel.EndHeight)
		} else {
			require.Positive(h.T(), actualDel.StartHeight)
			require.Positive(h.T(), actualDel.EndHeight)
		}
	})
}

func FuzzCreateBTCDelegationWithParamsFromBtcHeight(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(time.Now().Unix()))
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		btcTipHeight := uint32(30)
		// mock BTC light client and BTC checkpoint modules
		btclcKeeper := types.NewMockBTCLightClientKeeper(ctrl)
		btccKeeper := types.NewMockBtcCheckpointKeeper(ctrl)
		h := testutil.NewHelper(t, btclcKeeper, btccKeeper, nil)

		// set all parameters
		h.GenAndApplyParams(r)
		ctx, k := h.Ctx, h.BTCStakingKeeper

		versionedParams := k.GetParamsWithVersion(ctx)
		currentParams := versionedParams.Params

		maxGapBlocksBetweenParams := datagen.RandomUInt32(r, 100) + 100
		// we are adding btcTipHeight so that delegation will always be included
		// after the initial tip height
		expectedParamsBlockHeight := btcTipHeight + datagen.RandomUInt32(r, maxGapBlocksBetweenParams) + currentParams.BtcActivationHeight + 1
		expectedParamsVersion := versionedParams.Version + 1

		currentParams.BtcActivationHeight = expectedParamsBlockHeight
		err := k.SetParams(ctx, currentParams)
		require.NoError(t, err)

		nextBtcActivationHeight := btcTipHeight + datagen.RandomUInt32(r, maxGapBlocksBetweenParams) + currentParams.BtcActivationHeight + 1
		currentParams.BtcActivationHeight = nextBtcActivationHeight
		err = k.SetParams(ctx, currentParams)
		require.NoError(t, err)

		// makes sure that at the BTC block height 300 will use the expected param
		p, version, err := k.GetParamsForBtcHeight(ctx, uint64(nextBtcActivationHeight-1))
		h.NoError(err)
		require.Equal(t, p.BtcActivationHeight, expectedParamsBlockHeight)
		require.Equal(t, version, expectedParamsVersion)

		// creates one BTC delegation with BTC block height between expectedParamsBlockHeight and 500
		// generate and insert new finality provider
		_, fpPK, _ := h.CreateFinalityProvider(r)

		btcBlockHeight := datagen.RandomUInt32(r, nextBtcActivationHeight-expectedParamsBlockHeight) + expectedParamsBlockHeight
		// generate and insert new BTC delegation
		stakingValue := int64(2 * 10e8)
		delSK, _, err := datagen.GenRandomBTCKeyPair(r)
		h.NoError(err)
		_, _, btcDel, _, _, _, err := h.CreateDelegationWithBtcBlockHeight(
			r,
			delSK,
			fpPK,
			stakingValue,
			1000,
			0,
			0,
			false,
			false,
			btcBlockHeight,
			btcTipHeight,
		)
		h.NoError(err)
		require.NotNil(t, btcDel.ParamsVersion, expectedParamsVersion)
	})
}

func TestProperVersionInDelegation(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// mock BTC light client and BTC checkpoint modules
	btclcKeeper := types.NewMockBTCLightClientKeeper(ctrl)
	btccKeeper := types.NewMockBtcCheckpointKeeper(ctrl)
	h := testutil.NewHelper(t, btclcKeeper, btccKeeper, nil)

	// set all parameters
	h.GenAndApplyParams(r)

	// generate and insert new finality provider
	_, fpPK, _ := h.CreateFinalityProvider(r)

	// generate and insert new BTC delegation
	stakingValue := int64(2 * 10e8)
	delSK, _, err := datagen.GenRandomBTCKeyPair(r)
	h.NoError(err)
	stakingTxHash, _, _, _, _, _, err := h.CreateDelegationWithBtcBlockHeight(
		r,
		delSK,
		fpPK,
		stakingValue,
		1000,
		0,
		0,
		false,
		false,
		10,
		30,
	)
	h.NoError(err)

	// ensure consistency between the msg and the BTC delegation in DB
	actualDel, err := h.BTCStakingKeeper.GetBTCDelegation(h.Ctx, stakingTxHash)
	h.NoError(err)
	err = actualDel.ValidateBasic()
	h.NoError(err)
	// Current version will be `1` as:
	// - version `0` is initialized by `NewHelper`
	// - version `1` is set by `GenAndApplyParams`
	require.Equal(t, uint32(1), actualDel.ParamsVersion)

	customMinUnbondingTime := uint32(2000)
	currentParams := h.BTCStakingKeeper.GetParams(h.Ctx)
	currentParams.UnbondingTimeBlocks = customMinUnbondingTime
	currentParams.BtcActivationHeight++
	// Update new params
	err = h.BTCStakingKeeper.SetParams(h.Ctx, currentParams)
	require.NoError(t, err)
	// create new delegation
	stakingTxHash1, _, _, _, _, _, err := h.CreateDelegationWithBtcBlockHeight(
		r,
		delSK,
		fpPK,
		stakingValue,
		10000,
		stakingValue-1000,
		uint16(customMinUnbondingTime),
		false,
		false,
		10,
		30,
	)
	h.NoError(err)
	actualDel1, err := h.BTCStakingKeeper.GetBTCDelegation(h.Ctx, stakingTxHash1)
	h.NoError(err)
	err = actualDel1.ValidateBasic()
	h.NoError(err)
	// Assert that the new delegation has the updated params version
	require.Equal(t, uint32(2), actualDel1.ParamsVersion)
}

// TestBtcStakingWithBtcReOrg creates an BTC staking delegation
// with enough covenant signatures submitted to be considered ACTIVE.
func TestBtcStakingWithBtcReOrg(t *testing.T) {
	btcLightclientTipHeight := uint32(30)
	// btc staking tx will be included at btcLightclientTipHeight - BTC confirmation depth
	h, r, btcctParams, stakingTxHash := createActiveBtcDel(t, btcLightclientTipHeight)

	// verifies the largest reorg without anything set
	_, err := h.BTCStakingKeeper.LargestBtcReOrg(h.Ctx, &types.QueryLargestBtcReOrgRequest{})
	require.EqualError(t, err, types.ErrLargestBtcReorgNotFound.Error())

	// should not panic in end blocker since there is no reorg
	require.NotPanics(t, func() {
		_, err = btcstaking.EndBlocker(h.Ctx, *h.BTCStakingKeeper)
		h.NoError(err)
	})

	// -------------- simulates a reorg of current tip - (BTC depth - 1) --------
	// It should not panic in x/btcstaking end blocker as the reorg is at the limit allowed
	// It should consider the BTC staking as PENDING, since the block depth was revoked
	rBlockFrom, rBlockTo := datagen.GenRandomBTCHeaderInfo(r), datagen.GenRandomBTCHeaderInfo(r)
	rBlockFrom.Height = btcLightclientTipHeight
	rBlockTo.Height = btcLightclientTipHeight - (btcctParams.BtcConfirmationDepth - 1)
	currLargestReorg := types.NewLargestBtcReOrg(rBlockFrom, rBlockTo)

	err = h.BTCStakingKeeper.SetLargestBtcReorg(h.Ctx, currLargestReorg)
	h.NoError(err)

	// should not panic in end blocker since the reorg is less than the allowed
	require.NotPanics(t, func() {
		_, err = btcstaking.EndBlocker(h.Ctx, *h.BTCStakingKeeper)
		h.NoError(err)
	})

	// checks the query with a reorg set
	respLargestReOrg, err := h.BTCStakingKeeper.LargestBtcReOrg(h.Ctx, &types.QueryLargestBtcReOrgRequest{})
	h.NoError(err)
	require.Equal(t, respLargestReOrg.BlockDiff, currLargestReorg.BlockDiff)
	require.Equal(t, respLargestReOrg.RollbackFrom.HashHex, rBlockFrom.ToResponse().HashHex)
	require.Equal(t, respLargestReOrg.RollbackTo.HashHex, rBlockTo.ToResponse().HashHex)

	// BTC staking tx is still seen as active rolling back to a block where the confirmation depth is less than btcctParams.BtcConfirmationDepth
	h.BTCLightClientKeeper.EXPECT().GetTipInfo(gomock.Eq(h.Ctx)).Return(&btclctypes.BTCHeaderInfo{Height: rBlockTo.Height})
	delResp, err := h.BTCStakingKeeper.BTCDelegation(h.Ctx, &types.QueryBTCDelegationRequest{
		StakingTxHashHex: stakingTxHash,
	})
	h.NoError(err)
	require.Equal(t, types.BTCDelegationStatus_ACTIVE.String(), delResp.BtcDelegation.StatusDesc)

	// -------------- simulates a reorg of current tip - (BTC depth) --------
	// Should panic in x/btcstaking end blocker as the reorg is the size of k'
	// If a big reorg happened each btc staking transaction included in this last reorg blocks
	// will need to be analyzed if they are included in the new reorganization of blocks
	// and a emergency upgrade will be needed to revoke this values stored in voting power and rewards
	rBlockFrom.Height = btcLightclientTipHeight
	rBlockTo.Height = btcLightclientTipHeight - (btcctParams.BtcConfirmationDepth)
	currLargestReorg = types.NewLargestBtcReOrg(rBlockFrom, rBlockTo)

	err = h.BTCStakingKeeper.SetLargestBtcReorg(h.Ctx, currLargestReorg)
	h.NoError(err)

	// should panic in end blocker since the reorg is the size of BTC Confirmation Depth
	require.Panics(t, func() {
		_, err = btcstaking.EndBlocker(h.Ctx, *h.BTCStakingKeeper)
		h.NoError(err)
	})

	// verifies the query of the largest reorg again
	respLargestReOrg, err = h.BTCStakingKeeper.LargestBtcReOrg(h.Ctx, &types.QueryLargestBtcReOrgRequest{})
	h.NoError(err)
	require.Equal(t, respLargestReOrg.BlockDiff, currLargestReorg.BlockDiff)
	require.Equal(t, respLargestReOrg.RollbackFrom.HashHex, rBlockFrom.ToResponse().HashHex)
	require.Equal(t, respLargestReOrg.RollbackTo.HashHex, rBlockTo.ToResponse().HashHex)
}

func createActiveBtcDel(t *testing.T, btcLightclientTipHeight uint32) (*testutil.Helper, *rand.Rand, btcctypes.Params, string) {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// mock BTC light client and BTC checkpoint modules
	btclcKeeper := types.NewMockBTCLightClientKeeper(ctrl)
	btccKeeper := types.NewMockBtcCheckpointKeeper(ctrl)

	h := testutil.NewHelper(t, btclcKeeper, btccKeeper, nil)

	// set all parameters
	covenantSKs, _ := h.GenAndApplyParams(r)

	// makes sure of the BTC depth
	btcctParams := btcctypes.DefaultParams()
	btccKeeper.EXPECT().GetParams(gomock.Any()).Return(btcctParams).AnyTimes()

	// generate and insert new finality provider
	_, fpPK, _ := h.CreateFinalityProvider(r)

	// generate and insert new BTC delegation
	stakingValue := int64(2 * 10e8)
	delSK, _, err := datagen.GenRandomBTCKeyPair(r)
	h.NoError(err)

	btcBlockHeightTxInserted := btcLightclientTipHeight - btcctParams.BtcConfirmationDepth
	stakingTxHash, _, _, _, _, _, err := h.CreateDelegationWithBtcBlockHeight(
		r,
		delSK,
		fpPK,
		stakingValue,
		1000,
		0,
		0,
		false,
		false,
		btcBlockHeightTxInserted,
		btcLightclientTipHeight,
	)
	h.NoError(err)

	actualDel, err := h.BTCStakingKeeper.GetBTCDelegation(h.Ctx, stakingTxHash)
	h.NoError(err)
	require.NotNil(t, actualDel)

	msgs := h.GenerateCovenantSignaturesMessages(r, covenantSKs, actualDel)
	for _, msg := range msgs {
		h.BTCLightClientKeeper.EXPECT().GetTipInfo(gomock.Any()).Return(&btclctypes.BTCHeaderInfo{Height: btcBlockHeightTxInserted})
		_, err = h.MsgServer.AddCovenantSigs(h.Ctx, msg)
		h.NoError(err)
	}

	// ensure consistency between the msg and the BTC delegation in DB
	h.BTCLightClientKeeper.EXPECT().GetTipInfo(gomock.Eq(h.Ctx)).Return(&btclctypes.BTCHeaderInfo{Height: btcBlockHeightTxInserted})
	delResp, err := h.BTCStakingKeeper.BTCDelegation(h.Ctx, &types.QueryBTCDelegationRequest{
		StakingTxHashHex: stakingTxHash,
	})
	h.NoError(err)
	require.Equal(t, types.BTCDelegationStatus_ACTIVE.String(), delResp.BtcDelegation.StatusDesc)

	decodeStakingTxHashBz, err := hex.DecodeString(delResp.BtcDelegation.StakingTxHex)
	h.NoError(err)

	decodeStakingTxHash, err := bbn.NewBTCTxFromBytes(decodeStakingTxHashBz)
	h.NoError(err)

	require.Equal(t, decodeStakingTxHash.TxHash().String(), stakingTxHash)
	require.Equal(t, uint32(1), delResp.BtcDelegation.ParamsVersion)

	return h, r, btcctParams, stakingTxHash
}

func TestRejectActivationThatShouldNotUsePreApprovalFlow(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// mock BTC light client and BTC checkpoint modules
	btclcKeeper := types.NewMockBTCLightClientKeeper(ctrl)
	btccKeeper := types.NewMockBtcCheckpointKeeper(ctrl)
	h := testutil.NewHelper(t, btclcKeeper, btccKeeper, nil)

	// set all parameters
	covenantSKs, _ := h.GenAndApplyParams(r)

	// generate and insert new finality provider
	_, fpPK, _ := h.CreateFinalityProvider(r)

	// create fresh version of params
	currentParams := h.BTCStakingKeeper.GetParams(h.Ctx)
	// params will be activate at block height 2
	currentParams.BtcActivationHeight++
	// Update new params
	err := h.BTCStakingKeeper.SetParams(h.Ctx, currentParams)
	require.NoError(t, err)

	// generate and insert new BTC delegation
	stakingValue := int64(2 * 10e8)
	delSK, _, err := datagen.GenRandomBTCKeyPair(r)
	h.NoError(err)
	stakingTxHash, _, _, headerInfo, inclusionProof, _, err := h.CreateDelegationWithBtcBlockHeight(
		r,
		delSK,
		fpPK,
		stakingValue,
		1000,
		0,
		0,
		// use the pre-approval flow
		true,
		false,
		// staking tx will be included in BTC block height 1, which is before the activation of the new params
		1,
		// current tip is 10
		10,
	)
	h.NoError(err)
	require.NotNil(t, headerInfo)
	require.NotNil(t, inclusionProof)

	// ensure consistency between the msg and the BTC delegation in DB
	actualDel, err := h.BTCStakingKeeper.GetBTCDelegation(h.Ctx, stakingTxHash)
	h.NoError(err)
	require.NotNil(t, actualDel)

	msgs := h.GenerateCovenantSignaturesMessages(r, covenantSKs, actualDel)
	for _, msg := range msgs {
		h.BTCLightClientKeeper.EXPECT().GetTipInfo(gomock.Any()).Return(&btclctypes.BTCHeaderInfo{Height: 10})
		_, err = h.MsgServer.AddCovenantSigs(h.Ctx, msg)
		h.NoError(err)
	}

	// get updated delegation
	actualDel, err = h.BTCStakingKeeper.GetBTCDelegation(h.Ctx, stakingTxHash)
	h.NoError(err)
	require.NotNil(t, actualDel)

	tipHeight := uint32(1)
	covenantQuorum := h.BTCStakingKeeper.GetParams(h.Ctx).CovenantQuorum
	status, err := h.BTCStakingKeeper.BtcDelStatus(h.Ctx, actualDel, covenantQuorum, tipHeight)
	h.NoError(err)
	require.Equal(t, types.BTCDelegationStatus_VERIFIED, status)

	msg := &types.MsgAddBTCDelegationInclusionProof{
		StakingTxHash:           stakingTxHash,
		StakingTxInclusionProof: inclusionProof,
	}

	// mock BTC header that includes the staking tx
	h.BTCLightClientKeeper.EXPECT().GetHeaderByHash(gomock.Eq(h.Ctx), gomock.Eq(headerInfo.Header.Hash())).Return(headerInfo, nil).AnyTimes()
	h.BTCLightClientKeeper.EXPECT().GetTipInfo(gomock.Any()).Return(&btclctypes.BTCHeaderInfo{Height: 30})
	// Call the AddBTCDelegationInclusionProof handler
	_, err = h.MsgServer.AddBTCDelegationInclusionProof(h.Ctx, msg)
	h.Error(err)
	require.ErrorAs(t, err, &types.ErrStakingTxIncludedTooEarly)
}

func FuzzAddCovenantSigs(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// mock BTC light client and BTC checkpoint modules
		btclcKeeper := types.NewMockBTCLightClientKeeper(ctrl)
		btccKeeper := types.NewMockBtcCheckpointKeeper(ctrl)
		h := testutil.NewHelper(t, btclcKeeper, btccKeeper, nil)

		// set all parameters
		covenantSKs, _ := h.GenAndApplyParams(r)

		// generate and insert new finality provider
		_, fpPK, _ := h.CreateFinalityProvider(r)

		usePreApproval := datagen.OneInN(r, 2)

		// generate and insert new BTC delegation
		stakingValue := int64(2 * 10e8)
		delSK, _, err := datagen.GenRandomBTCKeyPair(r)
		h.NoError(err)

		var stakingTxHash string

		stakingTxHash, _, _, _, _, _, err = h.CreateDelegationWithBtcBlockHeight(
			r,
			delSK,
			fpPK,
			stakingValue,
			1000,
			0,
			0,
			usePreApproval,
			false,
			10,
			30,
		)
		h.NoError(err)

		// ensure consistency between the msg and the BTC delegation in DB
		actualDel, err := h.BTCStakingKeeper.GetBTCDelegation(h.Ctx, stakingTxHash)
		h.NoError(err)
		// delegation is not activated by covenant yet
		hasQuorum, err := h.BTCStakingKeeper.BtcDelHasCovenantQuorums(h.Ctx, actualDel, h.BTCStakingKeeper.GetParams(h.Ctx).CovenantQuorum)
		h.NoError(err)
		require.False(h.T(), hasQuorum)

		msgs := h.GenerateCovenantSignaturesMessages(r, covenantSKs, actualDel)
		h.BTCLightClientKeeper.EXPECT().GetTipInfo(gomock.Eq(h.Ctx)).Return(&btclctypes.BTCHeaderInfo{Height: 30}).AnyTimes()
		// ensure the system does not panick due to a bogus covenant sig request
		bogusMsg := *msgs[0]
		bogusMsg.StakingTxHash = datagen.GenRandomBtcdHash(r).String()
		_, err = h.MsgServer.AddCovenantSigs(h.Ctx, &bogusMsg)
		h.Error(err)

		for i, msg := range msgs {
			_, err = h.MsgServer.AddCovenantSigs(h.Ctx, msg)
			h.NoError(err)
			// check that submitting the same covenant signature returns error
			_, err = h.MsgServer.AddCovenantSigs(h.Ctx, msg)
			h.Error(err, "i: %d", i)
		}

		// ensure the BTC delegation now has voting power
		actualDel, err = h.BTCStakingKeeper.GetBTCDelegation(h.Ctx, stakingTxHash)
		h.NoError(err)
		hasQuorum, err = h.BTCStakingKeeper.BtcDelHasCovenantQuorums(h.Ctx, actualDel, h.BTCStakingKeeper.GetParams(h.Ctx).CovenantQuorum)
		h.NoError(err)
		require.True(h.T(), hasQuorum)
		require.True(h.T(), actualDel.BtcUndelegation.HasCovenantQuorums(h.BTCStakingKeeper.GetParams(h.Ctx).CovenantQuorum))

		tipHeight := uint32(30)
		covenantQuorum := h.BTCStakingKeeper.GetParams(h.Ctx).CovenantQuorum
		status, err := h.BTCStakingKeeper.BtcDelStatus(h.Ctx, actualDel, covenantQuorum, tipHeight)
		h.NoError(err)
		votingPower := actualDel.VotingPower(tipHeight, covenantQuorum, 0)

		if usePreApproval {
			require.Equal(t, status, types.BTCDelegationStatus_VERIFIED)
			require.Zero(t, votingPower)
		} else {
			require.Equal(t, status, types.BTCDelegationStatus_ACTIVE)
			require.Equal(t, uint64(stakingValue), votingPower)
		}
	})
}

func FuzzAddBTCDelegationInclusionProof(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// mock BTC light client and BTC checkpoint modules
		btclcKeeper := types.NewMockBTCLightClientKeeper(ctrl)
		btccKeeper := types.NewMockBtcCheckpointKeeper(ctrl)
		h := testutil.NewHelper(t, btclcKeeper, btccKeeper, nil)

		// set all parameters
		covenantSKs, _ := h.GenAndApplyParams(r)

		// generate and insert new finality provider
		_, fpPK, _ := h.CreateFinalityProvider(r)

		// generate and insert new BTC delegation
		stakingValue := int64(2 * 10e8)
		delSK, _, err := datagen.GenRandomBTCKeyPair(r)
		h.NoError(err)
		stakingTxHash, msgCreateBTCDel, actualDel, btcHeaderInfo, inclusionProof, _, err := h.CreateDelegationWithBtcBlockHeight(
			r,
			delSK,
			fpPK,
			stakingValue,
			1000,
			0,
			0,
			true,
			false,
			10,
			10,
		)
		h.NoError(err)

		// add covenant signatures to this BTC delegation
		h.CreateCovenantSigs(r, covenantSKs, msgCreateBTCDel, actualDel, 10)

		actualDel, err = h.BTCStakingKeeper.GetBTCDelegation(h.Ctx, stakingTxHash)
		h.NoError(err)

		// ensure the BTC delegation is now verified and does not have voting power
		tipHeight := uint32(10)

		covenantQuorum := h.BTCStakingKeeper.GetParams(h.Ctx).CovenantQuorum
		status, err := h.BTCStakingKeeper.BtcDelStatus(h.Ctx, actualDel, covenantQuorum, tipHeight)
		h.NoError(err)
		votingPower := actualDel.VotingPower(tipHeight, covenantQuorum, 0)

		require.Equal(t, status, types.BTCDelegationStatus_VERIFIED)
		require.Zero(t, votingPower)

		// activate the BTC delegation, such that the BTC delegation becomes active
		// and has voting power
		newTipHeight := uint32(30)
		h.AddInclusionProof(stakingTxHash, btcHeaderInfo, inclusionProof, newTipHeight)

		actualDel, err = h.BTCStakingKeeper.GetBTCDelegation(h.Ctx, stakingTxHash)
		h.NoError(err)
		status, err = h.BTCStakingKeeper.BtcDelStatus(h.Ctx, actualDel, covenantQuorum, tipHeight)
		h.NoError(err)
		votingPower = actualDel.VotingPower(tipHeight, covenantQuorum, 0)

		require.Equal(t, status, types.BTCDelegationStatus_ACTIVE)
		require.Equal(t, uint64(stakingValue), votingPower)
	})
}

func FuzzBTCUndelegate(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// mock BTC light client and BTC checkpoint modules
		btclcKeeper := types.NewMockBTCLightClientKeeper(ctrl)
		btccKeeper := types.NewMockBtcCheckpointKeeper(ctrl)
		h := testutil.NewHelper(t, btclcKeeper, btccKeeper, nil)

		// set all parameters
		covenantSKs, _ := h.GenAndApplyParams(r)

		bsParams := h.BTCStakingKeeper.GetParams(h.Ctx)

		// generate and insert new finality provider
		_, fpPK, _ := h.CreateFinalityProvider(r)

		// generate and insert new BTC delegation
		stakingValue := int64(2 * 10e8)
		delSK, _, err := datagen.GenRandomBTCKeyPair(r)
		h.NoError(err)
		stakingTime := uint16(1000)
		stakingTxHash, msgCreateBTCDel, actualDel, btcHeaderInfo, inclusionProof, unbondingInfo, err := h.CreateDelegationWithBtcBlockHeight(
			r,
			delSK,
			fpPK,
			stakingValue,
			stakingTime,
			0,
			0,
			true,
			false,
			10,
			10,
		)
		h.NoError(err)

		// add covenant signatures to this BTC delegation
		h.CreateCovenantSigs(r, covenantSKs, msgCreateBTCDel, actualDel, 10)
		// activate the BTC delegation
		btcTip := uint32(30)
		h.AddInclusionProof(stakingTxHash, btcHeaderInfo, inclusionProof, btcTip)

		// ensure the BTC delegation is bonded right now
		actualDel, err = h.BTCStakingKeeper.GetBTCDelegation(h.Ctx, stakingTxHash)
		h.NoError(err)

		status, err := h.BTCStakingKeeper.BtcDelStatus(h.Ctx, actualDel, bsParams.CovenantQuorum, btcTip)
		h.NoError(err)
		require.Equal(t, types.BTCDelegationStatus_ACTIVE, status)

		unbondingTx := actualDel.MustGetUnbondingTx()
		stakingTx := actualDel.MustGetStakingTx()

		serializedUnbondingTxWithWitness, _ := datagen.AddWitnessToUnbondingTx(
			t,
			stakingTx.TxOut[0],
			delSK,
			covenantSKs,
			bsParams.CovenantQuorum,
			[]*btcec.PublicKey{fpPK},
			stakingTime,
			stakingValue,
			unbondingTx,
			h.Net,
		)

		msg := &types.MsgBTCUndelegate{
			Signer:                        datagen.GenRandomAccount().Address,
			StakingTxHash:                 stakingTxHash,
			StakeSpendingTx:               serializedUnbondingTxWithWitness,
			StakeSpendingTxInclusionProof: unbondingInfo.UnbondingTxInclusionProof,
			FundingTransactions: [][]byte{
				actualDel.StakingTx,
			},
		}

		// ensure the system does not panick due to a bogus unbonding msg
		bogusMsg := *msg
		bogusMsg.StakingTxHash = datagen.GenRandomBtcdHash(r).String()
		h.BTCLightClientKeeper.EXPECT().GetTipInfo(gomock.Eq(h.Ctx)).Return(&btclctypes.BTCHeaderInfo{Height: 30}).AnyTimes()
		_, err = h.MsgServer.BTCUndelegate(h.Ctx, &bogusMsg)
		h.Error(err)

		// unbond
		_, err = h.MsgServer.BTCUndelegate(h.Ctx, msg)
		h.NoError(err)

		// ensure the BTC delegation is unbonded
		actualDel, err = h.BTCStakingKeeper.GetBTCDelegation(h.Ctx, stakingTxHash)
		h.NoError(err)
		status, err = h.BTCStakingKeeper.BtcDelStatus(h.Ctx, actualDel, bsParams.CovenantQuorum, btcTip)
		h.NoError(err)
		require.Equal(t, types.BTCDelegationStatus_UNBONDED, status)
	})
}

func FuzzBTCUndelegateExpired(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// mock BTC light client and BTC checkpoint modules
		btclcKeeper := types.NewMockBTCLightClientKeeper(ctrl)
		btccKeeper := types.NewMockBtcCheckpointKeeper(ctrl)
		h := testutil.NewHelper(t, btclcKeeper, btccKeeper, nil)

		// set all parameters
		covenantSKs, _ := h.GenAndApplyParams(r)

		bsParams := h.BTCStakingKeeper.GetParams(h.Ctx)

		// generate and insert new finality provider
		_, fpPK, _ := h.CreateFinalityProvider(r)

		// generate and insert new BTC delegation
		stakingValue := int64(2 * 10e8)
		delSK, _, err := datagen.GenRandomBTCKeyPair(r)
		h.NoError(err)
		stakingTxHash, msgCreateBTCDel, actualDel, btcHeaderInfo, inclusionProof, unbondingInfo, err := h.CreateDelegationWithBtcBlockHeight(
			r,
			delSK,
			fpPK,
			stakingValue,
			1000,
			0,
			0,
			true,
			false,
			10,
			10,
		)
		h.NoError(err)

		// add covenant signatures to this BTC delegation
		h.CreateCovenantSigs(r, covenantSKs, msgCreateBTCDel, actualDel, 10)
		// activate the BTC delegation
		btcTip := uint32(30)
		h.AddInclusionProof(stakingTxHash, btcHeaderInfo, inclusionProof, btcTip)

		// ensure the BTC delegation is bonded right now
		actualDel, err = h.BTCStakingKeeper.GetBTCDelegation(h.Ctx, stakingTxHash)
		h.NoError(err)
		status, err := h.BTCStakingKeeper.BtcDelStatus(h.Ctx, actualDel, bsParams.CovenantQuorum, btcTip)
		h.NoError(err)
		require.Equal(t, types.BTCDelegationStatus_ACTIVE, status)

		msg := &types.MsgBTCUndelegate{
			Signer:                        datagen.GenRandomAccount().Address,
			StakingTxHash:                 stakingTxHash,
			StakeSpendingTx:               actualDel.BtcUndelegation.UnbondingTx,
			StakeSpendingTxInclusionProof: unbondingInfo.UnbondingTxInclusionProof,
		}

		// expires the delegation
		h.BTCLightClientKeeper.EXPECT().GetTipInfo(gomock.Eq(h.Ctx)).Return(&btclctypes.BTCHeaderInfo{Height: 2000}).AnyTimes()
		_, err = h.MsgServer.BTCUndelegate(h.Ctx, msg)
		require.EqualError(t, err, types.ErrInvalidBTCUndelegateReq.Wrap("cannot unbond an unbonded BTC delegation").Error())
	})
}

func FuzzSelectiveSlashing(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// mock BTC light client and BTC checkpoint modules
		btclcKeeper := types.NewMockBTCLightClientKeeper(ctrl)
		btccKeeper := types.NewMockBtcCheckpointKeeper(ctrl)
		h := testutil.NewHelper(t, btclcKeeper, btccKeeper, nil)

		// set all parameters
		h.GenAndApplyParams(r)

		tipHeight := 150
		mockTip := &btclctypes.BTCHeaderInfo{Height: uint32(tipHeight)}
		btclcKeeper.EXPECT().GetTipInfo(gomock.Any()).Return(mockTip).AnyTimes()

		// generate and insert new finality provider
		fpSK, fpPK, _ := h.CreateFinalityProvider(r)
		fpBtcPk := bbn.NewBIP340PubKeyFromBTCPK(fpPK)

		// construct message for the evidence of selective slashing
		msg := &types.MsgSelectiveSlashingEvidence{
			Signer:           datagen.GenRandomAccount().Address,
			RecoveredFpBtcSk: fpSK.Serialize(),
		}

		// ensure the system does not panic due to a bogus unbonding msg
		// In the new logic, a "bogus" message is one with an unregistered SK.
		bogusSK, _, err := datagen.GenRandomBTCKeyPair(r)
		require.NoError(t, err)
		bogusMsg := &types.MsgSelectiveSlashingEvidence{
			Signer:           datagen.GenRandomAccount().Address,
			RecoveredFpBtcSk: bogusSK.Serialize(),
		}

		_, err = h.MsgServer.SelectiveSlashingEvidence(h.Ctx, bogusMsg)
		h.Error(err)
		require.ErrorIs(t, err, types.ErrFpNotFound)

		// submit evidence of selective slashing
		_, err = h.MsgServer.SelectiveSlashingEvidence(h.Ctx, msg)
		h.NoError(err)

		// ensure the finality provider is slashed
		slashedFp, err := h.BTCStakingKeeper.GetFinalityProvider(h.Ctx, fpBtcPk.MustMarshal())
		h.NoError(err)
		require.True(t, slashedFp.IsSlashed())

		// ensure a second attempt to slash fails
		_, err = h.MsgServer.SelectiveSlashingEvidence(h.Ctx, msg)
		h.Error(err)
		require.ErrorIs(t, err, types.ErrFpAlreadySlashed)
	})
}

func TestDoNotAllowDelegationWithoutFinalityProvider(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// mock BTC light client and BTC checkpoint modules
	btclcKeeper := types.NewMockBTCLightClientKeeper(ctrl)
	btccKeeper := types.NewMockBtcCheckpointKeeper(ctrl)
	btccKeeper.EXPECT().GetParams(gomock.Any()).Return(btcctypes.DefaultParams()).AnyTimes()
	h := testutil.NewHelper(t, btclcKeeper, btccKeeper, nil)

	// set covenant PK to params
	_, covenantPKs := h.GenAndApplyParams(r)
	bsParams := h.BTCStakingKeeper.GetParams(h.Ctx)

	unbondingTime := bsParams.UnbondingTimeBlocks

	slashingChangeLockTime := uint16(unbondingTime)

	// We only generate a finality provider, but not insert it into KVStore. So later
	// insertion of delegation should fail.

	fp, err := datagen.GenRandomFinalityProvider(r)
	require.NoError(t, err)

	/*
		generate and insert valid new BTC delegation
	*/
	delSK, _, err := datagen.GenRandomBTCKeyPair(r)
	require.NoError(t, err)
	stakingTimeBlocks := bsParams.MinStakingTimeBlocks
	stakingValue := int64(2 * 10e8)
	testStakingInfo := datagen.GenBTCStakingSlashingInfo(
		r,
		t,
		h.Net,
		delSK,
		[]*btcec.PublicKey{fp.BtcPk.MustToBTCPK()},
		covenantPKs,
		bsParams.CovenantQuorum,
		uint16(stakingTimeBlocks),
		stakingValue,
		bsParams.SlashingPkScript,
		bsParams.SlashingRate,
		slashingChangeLockTime,
	)
	// get msgTx
	stakingMsgTx := testStakingInfo.StakingTx
	serializedStakingTx, err := bbn.SerializeBTCTx(stakingMsgTx)
	require.NoError(t, err)
	// random Babylon SK
	acc := datagen.GenRandomAccount()
	stakerAddr := sdk.MustAccAddressFromBech32(acc.Address)

	// PoP
	pop, err := datagen.NewPoPBTC(stakerAddr, delSK)
	require.NoError(t, err)
	// generate staking tx info
	prevBlock, _ := datagen.GenRandomBtcdBlock(r, 0, nil)
	btcHeaderWithProof := datagen.CreateBlockWithTransaction(r, &prevBlock.Header, stakingMsgTx)
	btcHeader := btcHeaderWithProof.HeaderBytes
	txInclusionProof := types.NewInclusionProof(
		&btcctypes.TransactionKey{Index: 1, Hash: btcHeader.Hash()},
		btcHeaderWithProof.SpvProof.MerkleNodes,
	)

	slashingPathInfo, err := testStakingInfo.StakingInfo.SlashingPathSpendInfo()
	require.NoError(t, err)

	// generate proper delegator sig
	delegatorSig, err := testStakingInfo.SlashingTx.Sign(
		stakingMsgTx,
		0,
		slashingPathInfo.GetPkScriptPath(),
		delSK,
	)
	require.NoError(t, err)

	stkTxHash := testStakingInfo.StakingTx.TxHash()
	unbondingValue := stakingValue - datagen.UnbondingTxFee // TODO: parameterise fee
	testUnbondingInfo := datagen.GenBTCUnbondingSlashingInfo(
		r,
		t,
		h.Net,
		delSK,
		[]*btcec.PublicKey{fp.BtcPk.MustToBTCPK()},
		covenantPKs,
		bsParams.CovenantQuorum,
		wire.NewOutPoint(&stkTxHash, datagen.StakingOutIdx),
		uint16(unbondingTime),
		unbondingValue,
		bsParams.SlashingPkScript,
		bsParams.SlashingRate,
		slashingChangeLockTime,
	)
	unbondingTx, err := bbn.SerializeBTCTx(testUnbondingInfo.UnbondingTx)
	h.NoError(err)
	delUnbondingSlashingSig, err := testUnbondingInfo.GenDelSlashingTxSig(delSK)
	h.NoError(err)

	// all good, construct and send MsgCreateBTCDelegation message
	msgCreateBTCDel := &types.MsgCreateBTCDelegation{
		StakerAddr:                    stakerAddr.String(),
		FpBtcPkList:                   []bbn.BIP340PubKey{*fp.BtcPk},
		BtcPk:                         bbn.NewBIP340PubKeyFromBTCPK(delSK.PubKey()),
		Pop:                           pop,
		StakingTime:                   stakingTimeBlocks,
		StakingValue:                  stakingValue,
		StakingTx:                     serializedStakingTx,
		StakingTxInclusionProof:       txInclusionProof,
		SlashingTx:                    testStakingInfo.SlashingTx,
		DelegatorSlashingSig:          delegatorSig,
		UnbondingTx:                   unbondingTx,
		UnbondingTime:                 unbondingTime,
		UnbondingValue:                unbondingValue,
		UnbondingSlashingTx:           testUnbondingInfo.SlashingTx,
		DelegatorUnbondingSlashingSig: delUnbondingSlashingSig,
	}

	_, err = h.MsgServer.CreateBTCDelegation(h.Ctx, msgCreateBTCDel)
	require.Error(t, err)
	require.True(t, errors.Is(err, types.ErrFpNotFound))

	AddFinalityProvider(t, h.Ctx, *h.BTCStakingKeeper, fp)
	inclusionHeight := uint32(100)
	inclusionHeader := &btclctypes.BTCHeaderInfo{
		Header: &btcHeader,
		Height: inclusionHeight,
	}
	tipHeight := 150
	mockTipHeaderInfo := &btclctypes.BTCHeaderInfo{Height: uint32(tipHeight)}
	btclcKeeper.EXPECT().GetHeaderByHash(gomock.Any(), btcHeader.Hash()).Return(inclusionHeader, nil).Times(1)
	btclcKeeper.EXPECT().GetTipInfo(gomock.Any()).Return(mockTipHeaderInfo).Times(1)
	_, err = h.MsgServer.CreateBTCDelegation(h.Ctx, msgCreateBTCDel)
	require.NoError(t, err)
}

func TestCorrectUnbondingTimeInDelegation(t *testing.T) {
	tests := []struct {
		name                      string
		finalizationTimeout       uint32
		unbondingTimeInParams     uint32
		unbondingTimeInDelegation uint16
		err                       error
	}{
		{
			name:                      "successful delegation if unbonding time in delegation is equal to unbonding time in params",
			unbondingTimeInDelegation: 101,
			unbondingTimeInParams:     101,
			finalizationTimeout:       100,
			err:                       nil,
		},
		{
			name:                      "invalid delegation if unbonding time is different from unbonding time in params",
			unbondingTimeInDelegation: 102,
			unbondingTimeInParams:     101,
			finalizationTimeout:       100,
			err:                       types.ErrInvalidUnbondingTx,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := rand.New(rand.NewSource(time.Now().Unix()))
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			// mock BTC light client and BTC checkpoint modules
			btclcKeeper := types.NewMockBTCLightClientKeeper(ctrl)
			btccKeeper := types.NewMockBtcCheckpointKeeper(ctrl)
			h := testutil.NewHelper(t, btclcKeeper, btccKeeper, nil)

			// set all parameters
			_, _ = h.GenAndApplyCustomParams(r, tt.finalizationTimeout, tt.unbondingTimeInParams, 0)

			// generate and insert new finality provider
			_, fpPK, _ := h.CreateFinalityProvider(r)

			// generate and insert new BTC delegation
			stakingValue := int64(2 * 10e8)
			delSK, _, err := datagen.GenRandomBTCKeyPair(r)
			h.NoError(err)
			stakingTxHash, _, _, _, _, _, err := h.CreateDelegation(
				r,
				delSK,
				fpPK,
				stakingValue,
				1000,
				stakingValue-1000,
				tt.unbondingTimeInDelegation,
				true,
				false,
			)
			if tt.err != nil {
				require.Error(t, err)
				require.True(t, errors.Is(err, tt.err))
			} else {
				require.NoError(t, err)
				// Retrieve delegation from keeper
				delegation, err := h.BTCStakingKeeper.GetBTCDelegation(h.Ctx, stakingTxHash)
				require.NoError(t, err)
				require.Equal(t, tt.unbondingTimeInDelegation, uint16(delegation.UnbondingTime))
			}
		})
	}
}

func TestAllowList(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// mock BTC light client and BTC checkpoint modules
	btclcKeeper := types.NewMockBTCLightClientKeeper(ctrl)
	btccKeeper := types.NewMockBtcCheckpointKeeper(ctrl)
	h := testutil.NewHelper(t, btclcKeeper, btccKeeper, nil)

	allowListExpirationHeight := uint64(10)
	// set all parameters, use the allow list
	h.GenAndApplyCustomParams(r, 100, 0, allowListExpirationHeight)

	// generate and insert new finality provider
	_, fpPK, _ := h.CreateFinalityProvider(r)

	usePreApproval := datagen.OneInN(r, 2)

	// generate and insert new BTC delegation
	stakingValue := int64(2 * 10e8)
	delSK, _, err := datagen.GenRandomBTCKeyPair(r)
	h.NoError(err)
	_, msgCreateBTCDel, _, _, _, _, err := h.CreateDelegationWithBtcBlockHeight(
		r,
		delSK,
		fpPK,
		stakingValue,
		1000,
		0,
		0,
		usePreApproval,
		// add delegation to the allow list, it should succeed
		true,
		10,
		30,
	)
	h.NoError(err)
	require.NotNil(t, msgCreateBTCDel)

	delSK1, _, err := datagen.GenRandomBTCKeyPair(r)
	h.NoError(err)
	_, msgCreateBTCDel1, _, _, _, _, err := h.CreateDelegationWithBtcBlockHeight(
		r,
		delSK1,
		fpPK,
		stakingValue,
		1000,
		0,
		0,
		usePreApproval,
		// do not add delegation to the allow list, it should fail
		false,
		10,
		30,
	)
	require.Error(t, err)
	require.ErrorIs(t, err, types.ErrInvalidStakingTx)
	require.Nil(t, msgCreateBTCDel1)

	// move forward in the block height, allow list should be expired
	h.Ctx = h.Ctx.WithBlockHeight(int64(allowListExpirationHeight))
	delSK2, _, err := datagen.GenRandomBTCKeyPair(r)
	h.NoError(err)
	_, msgCreateBTCDel2, _, _, _, _, err := h.CreateDelegationWithBtcBlockHeight(
		r,
		delSK2,
		fpPK,
		stakingValue,
		1000,
		0,
		0,
		usePreApproval,
		// do not add delegation to the allow list, it should succeed as allow list is expired
		false,
		10,
		30,
	)
	h.NoError(err)
	require.NotNil(t, msgCreateBTCDel2)
}

func createNDelegationsForFinalityProvider(
	r *rand.Rand,
	t *testing.T,
	fpPK *btcec.PublicKey,
	stakingValue int64,
	numDelegations int,
	quorum uint32,
) []*types.BTCDelegation {
	var delegations []*types.BTCDelegation
	for i := 0; i < numDelegations; i++ {
		covenatnSks, covenantPks, err := datagen.GenRandomBTCKeyPairs(r, int(quorum))
		require.NoError(t, err)

		delSK, _, err := datagen.GenRandomBTCKeyPair(r)
		require.NoError(t, err)

		slashingAddress, err := datagen.GenRandomBTCAddress(r, net)
		require.NoError(t, err)
		slashingPkScript, err := txscript.PayToAddrScript(slashingAddress)
		require.NoError(t, err)

		slashingRate := sdkmath.LegacyNewDecWithPrec(int64(datagen.RandomInt(r, 41)+10), 2)

		startHeight, endHeight := 1, math.MaxUint16
		stakingTime := uint32(endHeight) - uint32(startHeight)
		del, err := datagen.GenRandomBTCDelegation(
			r,
			t,
			net,
			[]bbn.BIP340PubKey{*bbn.NewBIP340PubKeyFromBTCPK(fpPK)},
			delSK,
			covenatnSks,
			covenantPks,
			quorum,
			slashingPkScript,
			stakingTime,
			1,
			1+(math.MaxUint16-1),
			uint64(stakingValue),
			slashingRate,
			math.MaxUint16,
		)
		require.NoError(t, err)

		delegations = append(delegations, del)
	}
	return delegations
}

type ExpectedProviderData struct {
	numDelegations int32
	stakingValue   int32
}

func FuzzDeterminismBtcstakingBeginBlocker(f *testing.F) {
	// less seeds than usual as this is pretty long running test
	datagen.AddRandomSeedsToFuzzer(f, 5)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))
		valSet, privSigner, err := datagen.GenesisValidatorSetWithPrivSigner(2)
		require.NoError(t, err)

		var expectedProviderData = make(map[string]*ExpectedProviderData)

		// Create two test apps from the same set of validators
		h := testhelper.NewHelperWithValSet(t, valSet, privSigner)
		h1 := testhelper.NewHelperWithValSet(t, valSet, privSigner)
		// app hash should be same at the beginning
		appHash1 := hex.EncodeToString(h.Ctx.BlockHeader().AppHash)
		appHash2 := hex.EncodeToString(h1.Ctx.BlockHeader().AppHash)
		require.Equal(t, appHash1, appHash2)

		// Execute block for both apps
		h.Ctx, err = h.ApplyEmptyBlockWithVoteExtension(r)
		require.NoError(t, err)
		h1.Ctx, err = h1.ApplyEmptyBlockWithVoteExtension(r)
		require.NoError(t, err)
		// Given that there is no transactions and the data in db is the same
		// app hash produced by both apps should be the same
		appHash1 = hex.EncodeToString(h.Ctx.BlockHeader().AppHash)
		appHash2 = hex.EncodeToString(h1.Ctx.BlockHeader().AppHash)
		require.Equal(t, appHash1, appHash2)

		// Default params are the same in both apps
		stakingParams := h.App.BTCStakingKeeper.GetParams(h.Ctx)
		covQuorum := stakingParams.CovenantQuorum
		maxFinalityProviders := int32(h.App.FinalityKeeper.GetParams(h.Ctx).MaxActiveFinalityProviders)

		// Number of finality providers from 10 to maxFinalityProviders + 10
		numFinalityProviders := int(r.Int31n(maxFinalityProviders) + 10)

		fps := datagen.CreateNFinalityProviders(r, t, numFinalityProviders)

		// Fill the database of both apps with the same finality providers and delegations
		for _, fp := range fps {
			h.AddFinalityProvider(fp)
			h1.AddFinalityProvider(fp)
		}

		for _, fp := range fps {
			// each finality provider has different amount of delegations with different amount
			stakingValue := r.Int31n(200000) + 10000
			numDelegations := r.Int31n(10)

			if numDelegations > 0 {
				expectedProviderData[fp.BtcPk.MarshalHex()] = &ExpectedProviderData{
					numDelegations: numDelegations,
					stakingValue:   stakingValue,
				}
			}

			delegations := createNDelegationsForFinalityProvider(
				r,
				t,
				fp.BtcPk.MustToBTCPK(),
				int64(stakingValue),
				int(numDelegations),
				covQuorum,
			)

			for _, del := range delegations {
				h.AddDelegation(del)
				h1.AddDelegation(del)
			}
		}

		// Execute block for both apps
		h.Ctx, err = h.ApplyEmptyBlockWithVoteExtension(r)
		require.NoError(t, err)
		h1.Ctx, err = h1.ApplyEmptyBlockWithVoteExtension(r)
		require.NoError(t, err)
		// Given that there is no transactions and the data in db is the same
		// app hash produced by both apps should be the same
		appHash1 = hex.EncodeToString(h.Ctx.BlockHeader().AppHash)
		appHash2 = hex.EncodeToString(h1.Ctx.BlockHeader().AppHash)
		require.Equal(t, appHash1, appHash2)
	})
}

func TestActiveAndExpiredEventsSameBlock(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// mock BTC light client and BTC checkpoint modules
	btclcKeeper := types.NewMockBTCLightClientKeeper(ctrl)
	btccKeeper := types.NewMockBtcCheckpointKeeper(ctrl)
	heightAfterMultiStakingAllowListExpiration := int64(10)

	h := testutil.NewHelperWithIncentiveKeeper(t, btclcKeeper, btccKeeper).WithBlockHeight(heightAfterMultiStakingAllowListExpiration)

	// TODO: add expected values
	fHooks := h.FinalityHooks.(*ftypes.MockFinalityHooks)
	fHooks.EXPECT().AfterBtcDelegationActivated(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	fHooks.EXPECT().AfterBtcDelegationUnbonded(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	fHooks.EXPECT().AfterBbnFpEntersActiveSet(gomock.Any(), gomock.Any()).AnyTimes()
	fHooks.EXPECT().AfterBbnFpRemovedFromActiveSet(gomock.Any(), gomock.Any()).AnyTimes()

	// set all parameters
	covenantSKs, _ := h.GenAndApplyCustomParams(r, 100, 200, 0)

	// Get BTC confirmation depth
	btccParams := btcctypes.DefaultParams()
	btccKeeper.EXPECT().GetParams(gomock.Any()).Return(btccParams).AnyTimes()
	confirmationDepth := btccParams.BtcConfirmationDepth

	// generate and insert new finality provider
	_, fpPK, _ := h.CreateFinalityProvider(r)

	// Critical setup to trigger the bug:
	unbondingTime := uint16(200)
	stakingTime := uint16(500)
	txInclusionHeight := uint32(10)
	btcTipAtCreation := txInclusionHeight + confirmationDepth // 20

	// Generate staking transaction
	stakingValue := int64(2 * 10e8)
	delSK, _, err := datagen.GenRandomBTCKeyPair(r)
	h.NoError(err)

	// Create delegation with pre-computed parameters
	stakingTxHash, _, _, _, _, _, err := h.CreateDelegationWithBtcBlockHeight(
		r,
		delSK,
		fpPK,
		stakingValue,
		stakingTime,
		0,
		unbondingTime,
		false, // not using pre-approval
		false,
		txInclusionHeight,
		btcTipAtCreation,
	)
	h.NoError(err)

	// Verify delegation has inclusion proof
	actualDel, err := h.BTCStakingKeeper.GetBTCDelegation(h.Ctx, stakingTxHash)
	h.NoError(err)
	require.True(t, actualDel.HasInclusionProof())
	require.Equal(t, txInclusionHeight, actualDel.StartHeight)

	// Calculate where EXPIRED event is scheduled
	expectedEndHeight := actualDel.EndHeight
	expiredEventHeight := expectedEndHeight - uint32(unbondingTime)

	// Check events at the expired event height BEFORE adding covenant signatures
	eventsBeforeSigs := h.BTCStakingKeeper.GetAllPowerDistUpdateEvents(h.Ctx, expiredEventHeight, expiredEventHeight)
	expiredEventCount := 0
	for _, event := range eventsBeforeSigs {
		if delEvent, ok := event.Ev.(*types.EventPowerDistUpdate_BtcDelStateUpdate); ok {
			if delEvent.BtcDelStateUpdate.StakingTxHash == stakingTxHash &&
				delEvent.BtcDelStateUpdate.NewState == types.BTCDelegationStatus_EXPIRED {
				expiredEventCount++
			}
		}
	}
	require.Equal(t, 1, expiredEventCount, "Should have exactly one EXPIRED event before adding covenant sigs")

	// Now add covenant signatures at the height where EXPIRED event is scheduled
	btclcKeeper.EXPECT().GetTipInfo(gomock.Eq(h.Ctx)).Return(&btclctypes.BTCHeaderInfo{Height: expiredEventHeight}).AnyTimes()

	// Add covenant signatures to reach  quorum -1
	msgs := h.GenerateCovenantSignaturesMessages(r, covenantSKs, actualDel)
	for i := 0; i < len(msgs)-3; i++ {
		_, err = h.MsgServer.AddCovenantSigs(h.Ctx, msgs[i])
		h.NoError(err)
	}

	// Verify delegation is still PENDING without quorum
	actualDel, err = h.BTCStakingKeeper.GetBTCDelegation(h.Ctx, stakingTxHash)
	h.NoError(err)
	status := actualDel.GetStatus(expiredEventHeight, h.BTCStakingKeeper.GetParams(h.Ctx).CovenantQuorum, 0)
	require.Equal(t, types.BTCDelegationStatus_PENDING, status, "Should be PENDING without quorum")

	// Add the final covenant signature to reach quorum
	_, err = h.MsgServer.AddCovenantSigs(h.Ctx, msgs[len(msgs)-2])
	h.NoError(err)

	// Now check events at the same height AFTER adding all covenant signatures
	eventsAfterSigs := h.BTCStakingKeeper.GetAllPowerDistUpdateEvents(h.Ctx, expiredEventHeight, expiredEventHeight)
	activeEventCount := 0
	expiredEventCount = 0

	for _, event := range eventsAfterSigs {
		if delEvent, ok := event.Ev.(*types.EventPowerDistUpdate_BtcDelStateUpdate); ok {
			if delEvent.BtcDelStateUpdate.StakingTxHash == stakingTxHash {
				if delEvent.BtcDelStateUpdate.NewState == types.BTCDelegationStatus_ACTIVE {
					activeEventCount++
				} else if delEvent.BtcDelStateUpdate.NewState == types.BTCDelegationStatus_EXPIRED {
					expiredEventCount++
				}
			}
		}
	}

	// This is the bug: both events exist at the same height
	require.Equal(t, 1, activeEventCount, "Should have exactly one ACTIVE event")
	require.Equal(t, 1, expiredEventCount, "Should have exactly one EXPIRED event")

	dc := ftypes.NewVotingPowerDistCache()
	var newDc *ftypes.VotingPowerDistCache
	require.NotPanics(t, func() {
		// Process the events after adding covenant signatures
		newDc, _ = h.FinalityKeeper.ProcessAllPowerDistUpdateEvents(h.Ctx, dc, expiredEventHeight, expiredEventHeight)
	}, "Processing events should not panic")

	require.Equal(t, dc, newDc)
}

func TestBtcStakeExpansion(t *testing.T) {
	testCases := []struct {
		name                    string
		setupOriginalDelegation func(t *testing.T, h *testutil.Helper, r *rand.Rand, covenantSKs []*btcec.PrivateKey, babylonFPPK *btcec.PublicKey, delSK *btcec.PrivateKey, stakingValue int64) (*types.BTCDelegation, string, uint32)
	}{
		{
			name: "expand regular delegation",
			setupOriginalDelegation: func(t *testing.T, h *testutil.Helper, r *rand.Rand, covenantSKs []*btcec.PrivateKey, babylonFPPK *btcec.PublicKey, delSK *btcec.PrivateKey, stakingValue int64) (*types.BTCDelegation, string, uint32) {
				lcTip := uint32(30)
				prevDelStakingTxHash, prevMsgCreateBTCDel, prevDel, _, _, _, err := h.CreateDelegationWithBtcBlockHeight(
					r,
					delSK,
					babylonFPPK,
					stakingValue,
					1000,
					0,
					0,
					false,
					true,
					10,
					lcTip,
				)
				h.NoError(err)
				require.NotNil(t, prevMsgCreateBTCDel)

				h.CreateCovenantSigs(r, covenantSKs, prevMsgCreateBTCDel, prevDel, 10)

				bsParams := h.BTCStakingKeeper.GetParams(h.Ctx)
				prevDel, err = h.BTCStakingKeeper.GetBTCDelegation(h.Ctx, prevDelStakingTxHash)
				require.NoError(t, err)
				status, err := h.BTCStakingKeeper.BtcDelStatus(h.Ctx, prevDel, bsParams.CovenantQuorum, lcTip)
				h.NoError(err)
				require.Equal(t, types.BTCDelegationStatus_ACTIVE, status)

				return prevDel, prevDelStakingTxHash, lcTip
			},
		},
		{
			name: "expand expanded delegation",
			setupOriginalDelegation: func(t *testing.T, h *testutil.Helper, r *rand.Rand, covenantSKs []*btcec.PrivateKey, babylonFPPK *btcec.PublicKey, delSK *btcec.PrivateKey, stakingValue int64) (*types.BTCDelegation, string, uint32) {
				lcTip := uint32(30)
				originalDelStakingTxHash, originalMsgCreateBTCDel, originalDel, _, _, _, err := h.CreateDelegationWithBtcBlockHeight(
					r,
					delSK,
					babylonFPPK,
					stakingValue,
					1000,
					0,
					0,
					false,
					true,
					10,
					lcTip,
				)
				h.NoError(err)
				require.NotNil(t, originalMsgCreateBTCDel)

				h.CreateCovenantSigs(r, covenantSKs, originalMsgCreateBTCDel, originalDel, 10)

				bsParams := h.BTCStakingKeeper.GetParams(h.Ctx)
				originalDel, err = h.BTCStakingKeeper.GetBTCDelegation(h.Ctx, originalDelStakingTxHash)
				require.NoError(t, err)
				status, err := h.BTCStakingKeeper.BtcDelStatus(h.Ctx, originalDel, bsParams.CovenantQuorum, lcTip)
				h.NoError(err)
				require.Equal(t, types.BTCDelegationStatus_ACTIVE, status)

				firstExpansionSpendingTx, firstExpansionFundingTx, err := h.CreateBtcStakeExpansionWithBtcTipHeight(
					r,
					delSK,
					babylonFPPK,
					stakingValue,
					1000,
					originalDel,
					lcTip,
				)
				require.NoError(t, err)

				firstExpandedDel, err := h.BTCStakingKeeper.GetBTCDelegation(h.Ctx, firstExpansionSpendingTx.TxHash().String())
				require.NoError(t, err)
				require.True(t, firstExpandedDel.IsStakeExpansion())

				h.CreateCovenantSigs(r, covenantSKs, nil, firstExpandedDel, 10)

				originalStkTx, err := bbn.NewBTCTxFromBytes(originalDel.GetStakingTx())
				require.NoError(t, err)

				firstExpansionSpendingTxWithWitnessBz, _ := datagen.AddWitnessToStakeExpTx(
					t,
					originalStkTx.TxOut[0],
					firstExpansionFundingTx.TxOut[0],
					delSK,
					covenantSKs,
					bsParams.CovenantQuorum,
					[]*btcec.PublicKey{babylonFPPK},
					uint16(1000),
					stakingValue,
					firstExpansionSpendingTx,
					h.Net,
				)

				firstExpansionTxInclusionProof := h.BuildBTCInclusionProofForSpendingTx(r, firstExpansionSpendingTx, lcTip)

				fundingTxBz, err := bbn.SerializeBTCTx(firstExpansionFundingTx)
				h.NoError(err)
				msg := &types.MsgBTCUndelegate{
					Signer:                        originalDel.StakerAddr,
					StakingTxHash:                 originalStkTx.TxHash().String(),
					StakeSpendingTx:               firstExpansionSpendingTxWithWitnessBz,
					StakeSpendingTxInclusionProof: firstExpansionTxInclusionProof,
					FundingTransactions:           [][]byte{originalDel.GetStakingTx(), fundingTxBz},
				}

				lcTip += 11
				h.BTCLightClientKeeper.EXPECT().GetTipInfo(gomock.Eq(h.Ctx)).Return(&btclctypes.BTCHeaderInfo{Height: lcTip}).Times(3)
				_, err = h.MsgServer.BTCUndelegate(h.Ctx, msg)
				h.NoError(err)

				firstExpandedDel, err = h.BTCStakingKeeper.GetBTCDelegation(h.Ctx, firstExpansionSpendingTx.TxHash().String())
				require.NoError(t, err)
				status, err = h.BTCStakingKeeper.BtcDelStatus(h.Ctx, firstExpandedDel, bsParams.CovenantQuorum, lcTip)
				h.NoError(err)
				require.Equal(t, types.BTCDelegationStatus_ACTIVE, status)

				return firstExpandedDel, firstExpansionSpendingTx.TxHash().String(), lcTip
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			r := rand.New(rand.NewSource(time.Now().UnixNano()))
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			btclcKeeper := types.NewMockBTCLightClientKeeper(ctrl)
			btccKeeper := types.NewMockBtcCheckpointKeeper(ctrl)
			h := testutil.NewHelper(t, btclcKeeper, btccKeeper, nil)

			covenantSKs, _ := h.GenAndApplyParams(r)

			_, babylonFPPK, _ := h.CreateFinalityProvider(r)

			stakingValue := int64(2 * 10e8)
			delSK, _, err := datagen.GenRandomBTCKeyPair(r)
			h.NoError(err)

			prevDel, prevDelStakingTxHash, lcTip := tc.setupOriginalDelegation(t, h, r, covenantSKs, babylonFPPK, delSK, stakingValue)

			spendingTx, fundingTx, err := h.CreateBtcStakeExpansionWithBtcTipHeight(
				r,
				delSK,
				babylonFPPK,
				stakingValue,
				1000,
				prevDel,
				lcTip,
			)
			require.NoError(t, err)

			expandedDel, err := h.BTCStakingKeeper.GetBTCDelegation(h.Ctx, spendingTx.TxHash().String())
			require.NoError(t, err)
			require.True(t, expandedDel.IsStakeExpansion())
			bsParams := h.BTCStakingKeeper.GetParams(h.Ctx)
			status, err := h.BTCStakingKeeper.BtcDelStatus(h.Ctx, expandedDel, bsParams.CovenantQuorum, lcTip)
			h.NoError(err)
			require.Equal(t, types.BTCDelegationStatus_PENDING, status)

			h.CreateCovenantSigs(r, covenantSKs, nil, expandedDel, 10)
			expandedDel, err = h.BTCStakingKeeper.GetBTCDelegation(h.Ctx, spendingTx.TxHash().String())
			require.NoError(t, err)
			status, err = h.BTCStakingKeeper.BtcDelStatus(h.Ctx, expandedDel, bsParams.CovenantQuorum, lcTip)
			h.NoError(err)
			require.Equal(t, types.BTCDelegationStatus_VERIFIED, status)

			prevStkTx, err := bbn.NewBTCTxFromBytes(prevDel.GetStakingTx())
			require.NoError(t, err)

			spendingTxWithWitnessBz, _ := datagen.AddWitnessToStakeExpTx(
				t,
				prevStkTx.TxOut[0],
				fundingTx.TxOut[0],
				delSK,
				covenantSKs,
				bsParams.CovenantQuorum,
				[]*btcec.PublicKey{babylonFPPK},
				uint16(1000),
				stakingValue,
				spendingTx,
				h.Net,
			)

			expansionTxInclusionProof := h.BuildBTCInclusionProofForSpendingTx(r, spendingTx, lcTip)

			fundingTxBz, err := bbn.SerializeBTCTx(fundingTx)
			h.NoError(err)
			msg := &types.MsgBTCUndelegate{
				Signer:                        prevDel.StakerAddr,
				StakingTxHash:                 prevStkTx.TxHash().String(),
				StakeSpendingTx:               spendingTxWithWitnessBz,
				StakeSpendingTxInclusionProof: expansionTxInclusionProof,
				FundingTransactions:           [][]byte{prevDel.GetStakingTx(), fundingTxBz},
			}

			lcTip += 11
			h.BTCLightClientKeeper.EXPECT().GetTipInfo(gomock.Eq(h.Ctx)).Return(&btclctypes.BTCHeaderInfo{Height: lcTip}).Times(3)
			_, err = h.MsgServer.BTCUndelegate(h.Ctx, msg)
			h.NoError(err)

			events := h.Ctx.EventManager().Events()
			evtCount := len(events)
			require.GreaterOrEqual(t, evtCount, 2)
			var foundInclusionProofEvent, foundEarlyUnbondedEvent bool
			for _, event := range events[evtCount-2:] {
				switch fmt.Sprintf("/%s", event.Type) {
				case sdk.MsgTypeURL(&types.EventBTCDelegationInclusionProofReceived{}):
					foundInclusionProofEvent = true
					testutilevents.RequireEventAttribute(t, event, "staking_tx_hash", fmt.Sprintf("\"%s\"", spendingTx.TxHash().String()), "Inclusion proof event should match the stake expansion delegation tx hash")
				case sdk.MsgTypeURL(&types.EventBTCDelgationUnbondedEarly{}):
					foundEarlyUnbondedEvent = true
					testutilevents.RequireEventAttribute(t, event, "staking_tx_hash", fmt.Sprintf("\"%s\"", prevDelStakingTxHash), "Early unbonded event should match the original delegation tx hash")
					testutilevents.RequireEventAttribute(t, event, "stake_expansion_tx_hash", fmt.Sprintf("\"%s\"", spendingTx.TxHash().String()), "Early unbonded event should match the stake expansion tx hash")
				}
			}
			require.True(t, foundInclusionProofEvent, "EventBTCDelegationInclusionProofReceived should be emitted")
			require.True(t, foundEarlyUnbondedEvent, "EventBTCDelgationUnbondedEarly should be emitted")

			expandedDel, err = h.BTCStakingKeeper.GetBTCDelegation(h.Ctx, spendingTx.TxHash().String())
			require.NoError(t, err)
			status, err = h.BTCStakingKeeper.BtcDelStatus(h.Ctx, expandedDel, bsParams.CovenantQuorum, lcTip)
			h.NoError(err)
			require.Equal(t, types.BTCDelegationStatus_ACTIVE, status)

			prevDel, err = h.BTCStakingKeeper.GetBTCDelegation(h.Ctx, prevDelStakingTxHash)
			require.NoError(t, err)
			status, err = h.BTCStakingKeeper.BtcDelStatus(h.Ctx, prevDel, bsParams.CovenantQuorum, lcTip)
			h.NoError(err)
			require.Equal(t, types.BTCDelegationStatus_UNBONDED, status)
		})
	}
}

func TestMultisigCreateBTCDelegationWithMaxStakerParams(t *testing.T) {
	// 1. create btc delegation with 2-of-3 multisig -> success
	// 2. create btc delegation with 3-of-5 multisig -> fail since current test helper set 2-of-3 as a max
	testCases := []struct {
		name         string
		stakerQuorum uint32
		stakerNum    uint32
		expErr       error
	}{
		{
			name:         "create btc delegation with 2-of-3 multisig - default max params: 2-of-3",
			stakerQuorum: 2,
			stakerNum:    3,
			expErr:       nil,
		},
		{
			name:         "create btc delegation with 3-of-5 multisig - default max params: 2-of-3",
			stakerQuorum: 3,
			stakerNum:    5,
			expErr:       types.ErrInvalidMultisigInfo,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			r := rand.New(rand.NewSource(time.Now().UnixNano()))
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			// mock BTC light client and BTC checkpoint modules
			btclcKeeper := types.NewMockBTCLightClientKeeper(ctrl)
			btccKeeper := types.NewMockBtcCheckpointKeeper(ctrl)
			h := testutil.NewHelper(t, btclcKeeper, btccKeeper, nil)

			// set all parameters, default max 2-of-3 multisig
			h.GenAndApplyParams(r)

			// generate and insert new finality provider
			_, fpPK, _ := h.CreateFinalityProvider(r)

			usePreApproval := datagen.OneInN(r, 2)

			// generate and insert new BTC delegation
			stakingValue := int64(2 * 10e8)
			delSKs, _, err := datagen.GenRandomBTCKeyPairs(r, int(tc.stakerNum))
			h.NoError(err)

			var stakingTxHash string
			var msgCreateBTCDel *types.MsgCreateBTCDelegation
			if usePreApproval {
				stakingTxHash, msgCreateBTCDel, _, _, _, _, err = h.CreateMultisigDelegationWithBtcBlockHeight(
					r,
					delSKs,
					tc.stakerQuorum,
					fpPK,
					stakingValue,
					1000,
					0,
					0,
					usePreApproval,
					false,
					10,
					10,
				)
			} else {
				stakingTxHash, msgCreateBTCDel, _, _, _, _, err = h.CreateMultisigDelegationWithBtcBlockHeight(
					r,
					delSKs,
					tc.stakerQuorum,
					fpPK,
					stakingValue,
					1000,
					0,
					0,
					usePreApproval,
					false,
					10,
					30,
				)
			}

			// check error based on expectation
			if tc.expErr != nil {
				// expect error
				require.Error(t, err)
				require.ErrorIs(t, err, tc.expErr)
				return // stop here for error cases
			}

			// no error expected - continue with validation
			h.NoError(err)

			// ensure consistency between the msg and the BTC delegation in DB
			actualDel, err := h.BTCStakingKeeper.GetBTCDelegation(h.Ctx, stakingTxHash)
			h.NoError(err)
			require.Equal(h.T(), msgCreateBTCDel.StakerAddr, actualDel.StakerAddr)
			require.Equal(h.T(), msgCreateBTCDel.Pop, actualDel.Pop)
			require.Equal(h.T(), msgCreateBTCDel.StakingTx, actualDel.StakingTx)
			require.Equal(h.T(), msgCreateBTCDel.SlashingTx, actualDel.SlashingTx)
			require.Equal(h.T(), msgCreateBTCDel.MultisigInfo, actualDel.MultisigInfo)

			// MultisigInfo contains staker info except the one representative staker info,
			// that is, for M-of-N multisig, length of StakerBtcPkList of MultisigInfo is N-1
			require.Equal(h.T(), int(tc.stakerNum), len(actualDel.MultisigInfo.StakerBtcPkList)+1)
			require.Equal(h.T(), tc.stakerQuorum, actualDel.MultisigInfo.StakerQuorum)

			// ensure the BTC delegation in DB is correctly formatted
			err = actualDel.ValidateBasic()
			h.NoError(err)
			// delegation is not activated by covenant yet
			hasQuorum, err := h.BTCStakingKeeper.BtcDelHasCovenantQuorums(h.Ctx, actualDel, h.BTCStakingKeeper.GetParams(h.Ctx).CovenantQuorum)
			h.NoError(err)
			require.False(h.T(), hasQuorum)

			if usePreApproval {
				require.Zero(h.T(), actualDel.StartHeight)
				require.Zero(h.T(), actualDel.EndHeight)
			} else {
				require.Positive(h.T(), actualDel.StartHeight)
				require.Positive(h.T(), actualDel.EndHeight)
			}

			// check events emitted
			events := h.Ctx.EventManager().Events()
			var foundBtcDelCreatedEvent bool

			// build expected multisig staker pk hexs from delSKs (skip first key since it's the main staker key)
			var expectedMultisigStakerPkHexs string
			if tc.stakerNum > 1 {
				multisigPkHexs := make([]string, 0, tc.stakerNum-1)
				for i := 1; i < int(tc.stakerNum); i++ {
					multisigPkHexs = append(multisigPkHexs, hex.EncodeToString(delSKs[i].PubKey().SerializeCompressed()[1:]))
				}

				jsonBytes, err := json.Marshal(multisigPkHexs)
				require.NoError(t, err)
				expectedMultisigStakerPkHexs = string(jsonBytes)
			}

			for _, event := range events {
				if fmt.Sprintf("/%s", event.Type) == sdk.MsgTypeURL(&types.EventBTCDelegationCreated{}) {
					foundBtcDelCreatedEvent = true
					testutilevents.RequireEventAttribute(t, event, "staking_tx_hex", fmt.Sprintf("\"%s\"", hex.EncodeToString(actualDel.StakingTx)), "BTC Delegation created event should match the staking tx hash")

					if tc.stakerNum > 1 {
						// for multisig, multisig_staker_btc_pk_hexs should have N-1 keys
						testutilevents.RequireEventAttribute(t, event, "multisig_staker_btc_pk_hexs", expectedMultisigStakerPkHexs, "BTC Delegation Created event should have extra staker info field with correct keys")
					}
				}
			}
			require.True(t, foundBtcDelCreatedEvent, "EventBTCDelegationCreated should be emitted")
		})
	}
}
