package testutil

import (
	"math/rand"
	"testing"

	"cosmossdk.io/log"
	"cosmossdk.io/store"
	storemetrics "cosmossdk.io/store/metrics"
	"github.com/btcsuite/btcd/btcec/v2"
	dbm "github.com/cosmos/cosmos-db"
	sdk "github.com/cosmos/cosmos-sdk/types"
	bankk "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/golang/mock/gomock"

	btcstkhelper "github.com/babylonlabs-io/babylon/v2/testutil/btcstaking-helper"
	testutil "github.com/babylonlabs-io/babylon/v2/testutil/btcstaking-helper"
	"github.com/babylonlabs-io/babylon/v2/testutil/datagen"
	keepertest "github.com/babylonlabs-io/babylon/v2/testutil/keeper"
	btclctypes "github.com/babylonlabs-io/babylon/v2/x/btclightclient/types"
	"github.com/babylonlabs-io/babylon/v2/x/btcstaking/types"
	bstypes "github.com/babylonlabs-io/babylon/v2/x/btcstaking/types"
	ftypes "github.com/babylonlabs-io/babylon/v2/x/finality/types"
	"github.com/babylonlabs-io/babylon/v2/x/incentive/keeper"
)

type IncentiveHelper struct {
	*btcstkhelper.Helper
	BankKeeper       bankk.Keeper
	IncentivesKeeper *keeper.Keeper
}

func NewIncentiveHelper(
	t testing.TB,
	btclcKeeper *bstypes.MockBTCLightClientKeeper,
	btccKForBtcStaking *bstypes.MockBtcCheckpointKeeper,
	btccKForFinality *ftypes.MockCheckpointingKeeper,
) *IncentiveHelper {
	db := dbm.NewMemDB()
	stateStore := store.NewCommitMultiStore(db, log.NewTestLogger(t), storemetrics.NewNoOpMetrics())

	accK := keepertest.AccountKeeper(t, db, stateStore)
	bankK := keepertest.BankKeeper(t, db, stateStore, accK)

	ictvK, _ := keepertest.IncentiveKeeperWithStore(t, db, stateStore, nil, bankK, accK, nil)
	btcstkH := btcstkhelper.NewHelperWithStoreAndIncentive(t, db, stateStore, btclcKeeper, btccKForBtcStaking, btccKForFinality, ictvK)

	return &IncentiveHelper{
		Helper:           btcstkH,
		BankKeeper:       bankK,
		IncentivesKeeper: ictvK,
	}
}

func (h *IncentiveHelper) SetFinalityActivationHeight(newActivationHeight uint64) {
	finalityParams := h.FinalityKeeper.GetParams(h.Ctx)
	finalityParams.FinalityActivationHeight = newActivationHeight
	err := h.FinalityKeeper.SetParams(h.Ctx, finalityParams)
	h.NoError(err)
}

func (h *IncentiveHelper) CreateBtcDelegation(
	r *rand.Rand,
	fpPK *secp256k1.PublicKey,
	stakingValue int64,
	stakingTime uint16,
	btcLightClientTipHeight uint32,
) (
	delSK *btcec.PrivateKey, stakingTxHash string, msgCreateBTCDel *bstypes.MsgCreateBTCDelegation, actualDel *bstypes.BTCDelegation, unbondingInfo *btcstkhelper.UnbondingTxInfo,
) {
	delSK, _, err := datagen.GenRandomBTCKeyPair(r)
	h.NoError(err)

	stakingTxHash, msgCreateBTCDel, _, _, _, unbondingInfo, err = h.CreateDelegationWithBtcBlockHeight(
		r,
		delSK,
		fpPK,
		stakingValue,
		stakingTime,
		0,
		0,
		false,
		false,
		10,
		btcLightClientTipHeight,
	)
	h.NoError(err)

	// ensure consistency between the msg and the BTC delegation in DB
	actualDel, err = h.BTCStakingKeeper.GetBTCDelegation(h.Ctx, stakingTxHash)
	h.NoError(err)

	h.Equal(msgCreateBTCDel.StakerAddr, actualDel.StakerAddr)
	h.Equal(msgCreateBTCDel.Pop, actualDel.Pop)
	h.Equal(msgCreateBTCDel.StakingTx, actualDel.StakingTx)
	h.Equal(msgCreateBTCDel.SlashingTx, actualDel.SlashingTx)

	return delSK, stakingTxHash, msgCreateBTCDel, actualDel, unbondingInfo
}

func (h *IncentiveHelper) CreateActiveBtcDelegation(
	r *rand.Rand,
	covenantSKs []*secp256k1.PrivateKey,
	fpPK *secp256k1.PublicKey,
	stakingValue int64,
	stakingTime uint16,
	btcLightClientTipHeight uint32,
) (
	delSK *btcec.PrivateKey, stakingTxHash string, actualDel *bstypes.BTCDelegation, unbondingInfo *btcstkhelper.UnbondingTxInfo,
) {
	delSK, stakingTxHash, msgCreateBTCDel, actualDel, unbondingInfo := h.CreateBtcDelegation(r, fpPK, stakingValue, stakingTime, btcLightClientTipHeight)

	bsParams := h.BTCStakingKeeper.GetParams(h.Ctx)
	h.BTCLightClientKeeper.EXPECT().GetTipInfo(gomock.Any()).Return(&btclctypes.BTCHeaderInfo{Height: btcLightClientTipHeight}).MaxTimes(len(bsParams.CovenantPks) * 2)

	h.GenerateAndSendCovenantSignatures(r, covenantSKs, msgCreateBTCDel, actualDel)
	h.EqualBtcDelegationStatus(stakingTxHash, btcLightClientTipHeight, bstypes.BTCDelegationStatus_ACTIVE)
	return delSK, stakingTxHash, actualDel, unbondingInfo
}

func (h *IncentiveHelper) EqualBtcDelRwdTrackerActiveSat(fp, del sdk.AccAddress, expectedSatAmount uint64) {
	btcDel, err := h.IncentivesKeeper.GetBTCDelegationRewardsTracker(h.Ctx, fp, del)
	h.NoError(err)
	h.Equal(btcDel.TotalActiveSat.Uint64(), expectedSatAmount)
}

func (h *IncentiveHelper) BtcUndelegate(
	stakingTxHash string,
	del *bstypes.BTCDelegation,
	unbondingInfo *testutil.UnbondingTxInfo,
	unbondingTx []byte,
	btcLightClientTipHeight uint32,
) {
	msgUndelegate := &bstypes.MsgBTCUndelegate{
		Signer:                        datagen.GenRandomAccount().Address,
		StakingTxHash:                 stakingTxHash,
		StakeSpendingTx:               unbondingTx,
		StakeSpendingTxInclusionProof: unbondingInfo.UnbondingTxInclusionProof,
		FundingTransactions:           [][]byte{del.StakingTx},
	}

	// early unbond
	_, err := h.MsgServer.BTCUndelegate(h.Ctx, msgUndelegate)
	h.NoError(err)

	h.EqualBtcDelegationStatus(stakingTxHash, btcLightClientTipHeight, bstypes.BTCDelegationStatus_UNBONDED)
}

func (h *IncentiveHelper) GenerateAndSendCovenantSignatures(
	r *rand.Rand,
	covenantSKs []*btcec.PrivateKey,
	msgCreateBTCDel *types.MsgCreateBTCDelegation,
	del *types.BTCDelegation,
) {
	covMsgs := h.GenerateCovenantSignaturesMessages(r, covenantSKs, msgCreateBTCDel, del)
	for _, msg := range covMsgs {
		_, err := h.MsgServer.AddCovenantSigs(h.Ctx, msg)
		h.NoError(err)
	}
}

func (h *IncentiveHelper) EqualBtcDelegationStatus(
	stakingTxHashStr string,
	tipHeight uint32,
	expectedStatus bstypes.BTCDelegationStatus,
) {
	actualDel, err := h.BTCStakingKeeper.GetBTCDelegation(h.Ctx, stakingTxHashStr)
	h.NoError(err)

	covenantQuorum := h.BTCStakingKeeper.GetParams(h.Ctx).CovenantQuorum

	status := actualDel.GetStatus(tipHeight, covenantQuorum)
	h.Equal(expectedStatus, status)
}

func (h *IncentiveHelper) CtxAddBlkHeight(blocksToAdd int64) {
	headerInfo := h.Ctx.HeaderInfo()
	headerInfo.Height += blocksToAdd
	h.Ctx = h.Ctx.WithHeaderInfo(headerInfo)
}

func (h *IncentiveHelper) FpAddPubRand(
	r *rand.Rand,
	sk *btcec.PrivateKey,
	startHeight uint64,
) *datagen.RandListInfo {
	numPubRand := uint64(200)
	randListInfo, msgCommitPubRandList, err := datagen.GenRandomMsgCommitPubRandList(
		r,
		sk,
		h.FpRandCommitContext(),
		startHeight,
		numPubRand,
	)
	h.NoError(err)

	_, err = h.FMsgServer.CommitPubRandList(h.Ctx, msgCommitPubRandList)
	h.NoError(err)
	return randListInfo
}
