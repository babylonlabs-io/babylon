package keeper_test

import (
	"bytes"
	"fmt"
	"math/rand"
	"testing"

	sdkmath "cosmossdk.io/math"
	babylonApp "github.com/babylonlabs-io/babylon/app"
	bbn "github.com/babylonlabs-io/babylon/types"
	btcctypes "github.com/babylonlabs-io/babylon/x/btccheckpoint/types"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/babylonlabs-io/babylon/testutil/datagen"
	btclckeeper "github.com/babylonlabs-io/babylon/x/btclightclient/keeper"
	btclctypes "github.com/babylonlabs-io/babylon/x/btclightclient/types"
	btcstakingkeeper "github.com/babylonlabs-io/babylon/x/btcstaking/keeper"
	"github.com/babylonlabs-io/babylon/x/btcstaking/types"
	btcstakingtype "github.com/babylonlabs-io/babylon/x/btcstaking/types"
	epochingtypes "github.com/babylonlabs-io/babylon/x/epoching/types"
	finalitykeeper "github.com/babylonlabs-io/babylon/x/finality/keeper"
	ftypes "github.com/babylonlabs-io/babylon/x/finality/types"
)

func TestDcNewDcPOC1(t *testing.T) {
	// 초기 설정
	r := rand.New(rand.NewSource(12312312312))
	app := babylonApp.Setup(t, false)
	ctx := app.BaseApp.NewContext(false)

	initHeader := ctx.HeaderInfo()
	initHeader.Height = int64(1)
	ctx = ctx.WithHeaderInfo(initHeader)

	defaultStakingKeeper := app.StakingKeeper
	btcStakingKeeper := app.BTCStakingKeeper
	btcStakingMsgServer := btcstakingkeeper.NewMsgServerImpl(btcStakingKeeper)
	btcLcKeeper := app.BTCLightClientKeeper
	btcLcMsgServer := btclckeeper.NewMsgServerImpl(btcLcKeeper)

	finalityKeeper := app.FinalityKeeper
	finalityMsgServer := finalitykeeper.NewMsgServerImpl(finalityKeeper)
	finalityParams := ftypes.DefaultParams()
	finalityParams.MaxActiveFinalityProviders = 5
	_ = finalityKeeper.SetParams(ctx, finalityParams)

	// Covenant 관련 키 생성
	covenantSKs, covenantPKs, err := datagen.GenRandomBTCKeyPairs(r, 1)
	require.NoError(t, err)
	slashingAddress, err := datagen.GenRandomBTCAddress(r, &chaincfg.SimNetParams)
	require.NoError(t, err)
	slashingPkScript, err := txscript.PayToAddrScript(slashingAddress)
	require.NoError(t, err)

	btcCcKeeper := app.BtcCheckpointKeeper
	epochingKeeper := app.EpochingKeeper
	checkpointingKeeper := app.CheckpointingKeeper
	CcParams := btcCcKeeper.GetParams(ctx)
	CcParams.BtcConfirmationDepth = 1 // for simulation
	err = btcCcKeeper.SetParams(ctx, CcParams)
	require.NoError(t, err)

	// 0. BTCStakingKeeper 파라미터 설정
	err = btcStakingKeeper.SetParams(ctx, types.Params{
		CovenantPks:               bbn.NewBIP340PKsFromBTCPKs(covenantPKs),
		CovenantQuorum:            1,
		MinStakingValueSat:        10000,
		MaxStakingValueSat:        int64(4000 * 10e8),
		MinStakingTimeBlocks:      400,
		MaxStakingTimeBlocks:      10000,
		SlashingPkScript:          slashingPkScript,
		MinSlashingTxFeeSat:       10,
		MinCommissionRate:         sdkmath.LegacyMustNewDecFromStr("0.01"),
		SlashingRate:              sdkmath.LegacyNewDecWithPrec(int64(datagen.RandomInt(r, 41)+10), 2),
		UnbondingTimeBlocks:       100,
		UnbondingFeeSat:           1000,
		AllowListExpirationHeight: 0,
		BtcActivationHeight:       1,
	})
	require.NoError(t, err)

	valset, err := defaultStakingKeeper.GetAllValidators(ctx)
	require.NoError(t, err)
	fmt.Printf("[+] initial validator set length : %d\n", len(valset))

	header := ctx.HeaderInfo()
	maximumSimulateBlocks := 30

	// Epoch 및 checkpoint 설정
	newEpoch := epochingtypes.NewEpoch(1, 10, uint64(ctx.HeaderInfo().Height), nil)
	epochingKeeper.SetEpochInfo(ctx, 1, &newEpoch)
	fmt.Printf("Current Epoch Number : %d\n", epochingKeeper.GetEpoch(ctx).GetEpochNumber())
	checkpointingKeeper.SetLastFinalizedEpoch(ctx, 1)

	// 외부에서 생성한 FP들 중, i==5인 FP를 저장해둔다.
	var targetFp *types.FinalityProvider
	var targetFpSK *btcec.PrivateKey
	var stakingTxHash chainhash.Hash
	var slashedSigner string
	_ = stakingTxHash
	_ = slashedSigner

	fpNum := 6
	for i := 0; i < fpNum; i++ {
		// 호출 시 FP를 외부에서 생성하여 전달
		fpSK, _, err := datagen.GenRandomBTCKeyPair(r)
		require.NoError(t, err)
		fp, err := datagen.GenRandomFinalityProviderWithBTCSK(r, fpSK, "")
		require.NoError(t, err)
		// i가 5일 때 저장
		if i == 1 {
			targetFp = fp
			targetFpSK = fpSK
			stakingTxHash, slashedSigner = createDelegationWithFinalityProvider(
				t, ctx, r, i,
				fp, fpSK, // 이미 생성된 FP 정보 전달
				btcStakingMsgServer, btcLcMsgServer, finalityMsgServer,
				btcStakingKeeper, btcLcKeeper,
				covenantSKs, covenantPKs, false,
			)

		} else {
			createDelegationWithFinalityProvider(
				t, ctx, r, i,
				fp, fpSK, // 이미 생성된 FP 정보 전달
				btcStakingMsgServer, btcLcMsgServer, finalityMsgServer,
				btcStakingKeeper, btcLcKeeper,
				covenantSKs, covenantPKs, false,
			)
		}
	}

	// 블록 시뮬레이션
	var pendingStakingTxHash chainhash.Hash
	_ = pendingStakingTxHash
	var pendingStakingSigner string
	_ = pendingStakingSigner

	for i := 0; i < maximumSimulateBlocks; i++ {
		ctx = ctx.WithHeaderInfo(header)
		ctx = ctx.WithBlockHeight(header.Height)

		fmt.Printf("-------- BeginBlock : %d, btc block height : %d ---------\n", header.Height, btcLcKeeper.GetTipInfo(ctx).Height)
		_, err := app.BeginBlocker(ctx)
		require.NoError(t, err)

		prevBlockHeader := btcLcKeeper.GetTipInfo(ctx).Header.ToBlockHeader()
		dummyGeneralTx := datagen.CreateDummyTx()
		dummyGeneralHeaderWithProof := datagen.CreateBlockWithTransaction(r, prevBlockHeader, dummyGeneralTx)
		dummyGeneralHeader := dummyGeneralHeaderWithProof.HeaderBytes
		generalHeaders := []bbn.BTCHeaderBytes{dummyGeneralHeader}
		insertHeaderMsg := &btclctypes.MsgInsertHeaders{
			Signer:  datagen.GenRandomAddress().String(),
			Headers: generalHeaders,
		}
		_, err = btcLcMsgServer.InsertHeaders(ctx, insertHeaderMsg)
		require.NoError(t, err)

		dc := finalityKeeper.GetVotingPowerDistCache(ctx, uint64(header.Height))
		activeFps := dc.GetActiveFinalityProviderSet()
		var fpsList []*ftypes.FinalityProviderDistInfo
		for _, v := range activeFps {
			fpsList = append(fpsList, v)
		}
		ftypes.SortFinalityProvidersWithZeroedVotingPower(fpsList)

		fmt.Printf("block height : %d, activeFps length : %d\n", ctx.HeaderInfo().Height, len(fpsList))
		for fpIndex, fp := range fpsList {
			fmt.Printf("fpIndex : %d, active fp address : %v, voting power : %d\n",
				fpIndex, fp.BtcPk.MarshalHex(), fp.TotalBondedSat)
		}

		// Target FP에게 Proof없이 Create Delegation.
		if i == 2 {
			// createDelegationWithFinalityProviderWithoutProof
			pendingStakingTxHash, pendingStakingSigner = createDelegationWithFinalityProviderWithoutProof(
				t, ctx, r, 5,
				targetFp, targetFpSK, // i==5 FP 정보 사용
				btcStakingMsgServer, btcLcMsgServer, finalityMsgServer,
				btcStakingKeeper, btcLcKeeper,
				covenantSKs, covenantPKs, true,
			)
		}

		if i == 3 {
			// targetFp와 targetFpSK는 반드시 non-nil이어야 함
			require.NotNil(t, targetFp)
			require.NotNil(t, targetFpSK)
			// Slashing
			fmt.Printf("Slashing target Fp : %v\n", targetFp.BtcPk.MarshalHex())
			slashMsg := &btcstakingtype.MsgSelectiveSlashingEvidence{
				Signer:           slashedSigner,
				StakingTxHash:    stakingTxHash.String(),
				RecoveredFpBtcSk: targetFpSK.Serialize(),
			}
			_, err = btcStakingMsgServer.SelectiveSlashingEvidence(ctx, slashMsg)
			require.NoError(t, err)
		}

		if i == 7 {
			// Target FP에게 Add Proof
			fmt.Printf("Add Proof~~~~\n")
			prevBlockHeader = btcLcKeeper.GetTipInfo(ctx).Header.ToBlockHeader()
			del, err := btcStakingKeeper.GetBTCDelegation(ctx, pendingStakingTxHash.String())
			require.NoError(t, err)
			var pendingStakingMsgTx wire.MsgTx
			err = pendingStakingMsgTx.Deserialize(bytes.NewReader(del.StakingTx))
			require.NoError(t, err)
			btcHeaderWithProof := datagen.CreateBlockWithTransaction(r, prevBlockHeader, &pendingStakingMsgTx)
			btcHeader := btcHeaderWithProof.HeaderBytes

			dummy1Tx := datagen.CreateDummyTx()
			dummy1HeaderWithProof := datagen.CreateBlockWithTransaction(r, btcHeader.ToBlockHeader(), dummy1Tx)
			dummy1Header := dummy1HeaderWithProof.HeaderBytes

			txInclusionProof := types.NewInclusionProof(
				&btcctypes.TransactionKey{Index: 1, Hash: btcHeader.Hash()},
				btcHeaderWithProof.SpvProof.MerkleNodes,
			)
			headers := []bbn.BTCHeaderBytes{btcHeader, dummy1Header}
			insertHeaderMsg := &btclctypes.MsgInsertHeaders{
				Signer:  datagen.GenRandomAddress().String(),
				Headers: headers,
			}
			_, err = btcLcMsgServer.InsertHeaders(ctx, insertHeaderMsg)
			require.NoError(t, err)

			proofMsg := &btcstakingtype.MsgAddBTCDelegationInclusionProof{
				Signer:                  pendingStakingSigner,
				StakingTxHash:           pendingStakingTxHash.String(),
				StakingTxInclusionProof: txInclusionProof,
			}
			_, err = btcStakingMsgServer.AddBTCDelegationInclusionProof(ctx, proofMsg)
			require.NoError(t, err)
		}

		_, err = app.EndBlocker(ctx)
		fmt.Printf("-------- EndBlock height : %d---------\n", header.Height)
		require.NoError(t, err)
		header.Height++
	}

	dc := finalityKeeper.GetVotingPowerDistCache(ctx, uint64(maximumSimulateBlocks))
	actives := dc.GetActiveFinalityProviderSet()
	_, fpSlashedActive := actives[targetFp.BtcPk.MarshalHex()]
	require.False(t, fpSlashedActive, "the slashed FP %s, should not be active", targetFp.BtcPk.MarshalHex())
}

func createDelegationWithFinalityProviderWithoutProof(
	t *testing.T,
	ctx sdk.Context,
	r *rand.Rand,
	fpIndex int,
	fpInfo *types.FinalityProvider, // 반드시 non-nil
	fpSK *btcec.PrivateKey, // 반드시 non-nil
	btcStakingMsgServer types.MsgServer,
	btcLcMsgServer btclctypes.MsgServer,
	finalityMsgServer ftypes.MsgServer, // finality 관련 MsgServer 타입 사용
	btcStakingKeeper btcstakingkeeper.Keeper, // keeper (값으로 전달)
	btcLcKeeper btclckeeper.Keeper,
	covenantSKs []*btcec.PrivateKey,
	covenantPKs []*btcec.PublicKey,
	createFinalityProviderSkip bool,
) (chainhash.Hash, string) {
	require.NotNil(t, fpInfo, "fpInfo must be provided")
	require.NotNil(t, fpSK, "fpSK must be provided")
	finalityFP := fpInfo
	finalityPriv := fpSK

	fmt.Printf("createDelegationWithFinalityProviderWithoutProof - finalityFP BTC Pubkey : %v\n", finalityFP.BtcPk.MarshalHex())

	// 1. FinalityProvider 생성 및 Commit (별도 함수 호출)
	if createFinalityProviderSkip == false {
		createAndCommitFinalityProvider(t, ctx, r, finalityFP, finalityPriv, btcStakingMsgServer, finalityMsgServer)
	}

	// 2. 위임 생성 준비
	bsParams := btcStakingKeeper.GetParams(ctx)
	covPKs, err := bbn.NewBTCPKsFromBIP340PKs(bsParams.CovenantPks)
	require.NoError(t, err)
	stakingValue := int64((fpIndex + 1) * 10e8)
	unbondingTime := bsParams.UnbondingTimeBlocks

	// 위임자 키 생성 및 Staking 정보 생성
	delSK, _, err := datagen.GenRandomBTCKeyPair(r)
	require.NoError(t, err)
	stakingTime := 1000

	testStakingInfo := datagen.GenBTCStakingSlashingInfo(
		r, t, &chaincfg.SimNetParams,
		delSK, []*btcec.PublicKey{finalityFP.BtcPk.MustToBTCPK()},
		covPKs, bsParams.CovenantQuorum,
		uint16(stakingTime), stakingValue,
		bsParams.SlashingPkScript, bsParams.SlashingRate,
		uint16(unbondingTime),
	)

	stakingMsgTx := testStakingInfo.StakingTx
	serializedStakingTx, err := bbn.SerializeBTCTx(stakingMsgTx)
	require.NoError(t, err)

	// 위임자 계정 및 PoP 생성
	acc := datagen.GenRandomAccount()
	stakerAddr := sdk.MustAccAddressFromBech32(acc.Address)
	pop, err := datagen.NewPoPBTC(stakerAddr, delSK)
	require.NoError(t, err)

	// Delegator signature 생성
	slashingPathInfo, err := testStakingInfo.StakingInfo.SlashingPathSpendInfo()
	require.NoError(t, err)
	delegatorSig, err := testStakingInfo.SlashingTx.Sign(
		stakingMsgTx, 0, slashingPathInfo.GetPkScriptPath(), delSK,
	)
	require.NoError(t, err)

	// 3. Unbonding 관련 정보 생성
	stkTxHash := testStakingInfo.StakingTx.TxHash()
	unbondingValue := stakingValue - datagen.UnbondingTxFee
	testUnbondingInfo := datagen.GenBTCUnbondingSlashingInfo(
		r, t, &chaincfg.SimNetParams,
		delSK, []*btcec.PublicKey{finalityFP.BtcPk.MustToBTCPK()},
		covenantPKs, bsParams.CovenantQuorum,
		wire.NewOutPoint(&stkTxHash, datagen.StakingOutIdx),
		uint16(unbondingTime), unbondingValue,
		bsParams.SlashingPkScript, bsParams.SlashingRate,
		uint16(unbondingTime),
	)
	unbondingTx, err := bbn.SerializeBTCTx(testUnbondingInfo.UnbondingTx)
	require.NoError(t, err)
	delUnbondingSlashingSig, err := testUnbondingInfo.GenDelSlashingTxSig(delSK)
	require.NoError(t, err)

	// 4. 위임 생성 메시지 전송
	msgCreateBTCDel := &types.MsgCreateBTCDelegation{
		StakerAddr:                    stakerAddr.String(),
		FpBtcPkList:                   []bbn.BIP340PubKey{*finalityFP.BtcPk},
		BtcPk:                         bbn.NewBIP340PubKeyFromBTCPK(delSK.PubKey()),
		Pop:                           pop,
		StakingTime:                   uint32(stakingTime),
		StakingValue:                  stakingValue,
		StakingTx:                     serializedStakingTx,
		StakingTxInclusionProof:       nil,
		SlashingTx:                    testStakingInfo.SlashingTx,
		DelegatorSlashingSig:          delegatorSig,
		UnbondingTx:                   unbondingTx,
		UnbondingTime:                 unbondingTime,
		UnbondingValue:                unbondingValue,
		UnbondingSlashingTx:           testUnbondingInfo.SlashingTx,
		DelegatorUnbondingSlashingSig: delUnbondingSlashingSig,
	}
	_, err = btcStakingMsgServer.CreateBTCDelegation(ctx, msgCreateBTCDel)
	require.NoError(t, err)

	// 5. Covenant Signature 추가
	stakingTxHash := testStakingInfo.StakingTx.TxHash()
	vPKs, err := bbn.NewBTCPKsFromBIP340PKs(msgCreateBTCDel.FpBtcPkList)
	require.NoError(t, err)

	covenantSlashingTxSigs, err := datagen.GenCovenantAdaptorSigs(
		covenantSKs, vPKs,
		testStakingInfo.StakingTx, slashingPathInfo.GetPkScriptPath(),
		msgCreateBTCDel.SlashingTx,
	)
	require.NoError(t, err)

	unbondingSlashingPathInfo, err := testUnbondingInfo.UnbondingInfo.SlashingPathSpendInfo()
	require.NoError(t, err)
	covenantUnbondingSlashingTxSigs, err := datagen.GenCovenantAdaptorSigs(
		covenantSKs, vPKs,
		testUnbondingInfo.UnbondingTx, unbondingSlashingPathInfo.GetPkScriptPath(),
		testUnbondingInfo.SlashingTx,
	)
	require.NoError(t, err)

	unbondingPathInfo, err := testStakingInfo.StakingInfo.UnbondingPathSpendInfo()
	require.NoError(t, err)
	covUnbondingSigs, err := datagen.GenCovenantUnbondingSigs(
		covenantSKs, testStakingInfo.StakingTx,
		0, unbondingPathInfo.GetPkScriptPath(),
		testUnbondingInfo.UnbondingTx,
	)
	require.NoError(t, err)

	msgAddCovenantSig := &types.MsgAddCovenantSigs{
		Signer:                  msgCreateBTCDel.StakerAddr,
		Pk:                      covenantSlashingTxSigs[0].CovPk,
		StakingTxHash:           stakingTxHash.String(),
		SlashingTxSigs:          covenantSlashingTxSigs[0].AdaptorSigs,
		UnbondingTxSig:          bbn.NewBIP340SignatureFromBTCSig(covUnbondingSigs[0]),
		SlashingUnbondingTxSigs: covenantUnbondingSlashingTxSigs[0].AdaptorSigs,
	}
	_, err = btcStakingMsgServer.AddCovenantSigs(ctx, msgAddCovenantSig)
	require.NoError(t, err)
	return stakingTxHash, msgCreateBTCDel.StakerAddr
}

// createDelegationWithFinalityProvider는
// (1) 전달받은 FinalityProvider의 생성 및 Commit (createAndCommitFinalityProvider 호출),
// (2) 위임 생성, 그리고
// (3) Covenant Signature 추가의 과정을 수행합니다.
// fpInfo와 fpSK는 반드시 nil이 아니어야 합니다.
func createDelegationWithFinalityProvider(
	t *testing.T,
	ctx sdk.Context,
	r *rand.Rand,
	fpIndex int,
	fpInfo *types.FinalityProvider, // 반드시 non-nil
	fpSK *btcec.PrivateKey, // 반드시 non-nil
	btcStakingMsgServer types.MsgServer,
	btcLcMsgServer btclctypes.MsgServer,
	finalityMsgServer ftypes.MsgServer, // finality 관련 MsgServer 타입 사용
	btcStakingKeeper btcstakingkeeper.Keeper, // keeper (값으로 전달)
	btcLcKeeper btclckeeper.Keeper,
	covenantSKs []*btcec.PrivateKey,
	covenantPKs []*btcec.PublicKey,
	createFinalityProviderSkip bool,
) (chainhash.Hash, string) {
	require.NotNil(t, fpInfo, "fpInfo must be provided")
	require.NotNil(t, fpSK, "fpSK must be provided")
	finalityFP := fpInfo
	finalityPriv := fpSK

	// 1. FinalityProvider 생성 및 Commit (별도 함수 호출)
	if createFinalityProviderSkip == false {
		createAndCommitFinalityProvider(t, ctx, r, finalityFP, finalityPriv, btcStakingMsgServer, finalityMsgServer)
	}

	// 2. 위임 생성 준비
	bsParams := btcStakingKeeper.GetParams(ctx)
	covPKs, err := bbn.NewBTCPKsFromBIP340PKs(bsParams.CovenantPks)
	require.NoError(t, err)
	stakingValue := int64((fpIndex + 1) * 10e8)
	unbondingTime := bsParams.UnbondingTimeBlocks

	// 위임자 키 생성 및 Staking 정보 생성
	delSK, _, err := datagen.GenRandomBTCKeyPair(r)
	require.NoError(t, err)
	stakingTime := 1000

	testStakingInfo := datagen.GenBTCStakingSlashingInfo(
		r, t, &chaincfg.SimNetParams,
		delSK, []*btcec.PublicKey{finalityFP.BtcPk.MustToBTCPK()},
		covPKs, bsParams.CovenantQuorum,
		uint16(stakingTime), stakingValue,
		bsParams.SlashingPkScript, bsParams.SlashingRate,
		uint16(unbondingTime),
	)

	stakingMsgTx := testStakingInfo.StakingTx
	serializedStakingTx, err := bbn.SerializeBTCTx(stakingMsgTx)
	require.NoError(t, err)

	// 위임자 계정 및 PoP 생성
	acc := datagen.GenRandomAccount()
	stakerAddr := sdk.MustAccAddressFromBech32(acc.Address)
	pop, err := datagen.NewPoPBTC(stakerAddr, delSK)
	require.NoError(t, err)

	// Tx 포함증명을 위한 헤더 삽입
	prevBlockHeader := btcLcKeeper.GetTipInfo(ctx).Header.ToBlockHeader()
	btcHeaderWithProof := datagen.CreateBlockWithTransaction(r, prevBlockHeader, stakingMsgTx)
	btcHeader := btcHeaderWithProof.HeaderBytes

	dummy1Tx := datagen.CreateDummyTx()
	dummy1HeaderWithProof := datagen.CreateBlockWithTransaction(r, btcHeader.ToBlockHeader(), dummy1Tx)
	dummy1Header := dummy1HeaderWithProof.HeaderBytes

	txInclusionProof := types.NewInclusionProof(
		&btcctypes.TransactionKey{Index: 1, Hash: btcHeader.Hash()},
		btcHeaderWithProof.SpvProof.MerkleNodes,
	)
	headers := []bbn.BTCHeaderBytes{btcHeader, dummy1Header}
	insertHeaderMsg := &btclctypes.MsgInsertHeaders{
		Signer:  stakerAddr.String(),
		Headers: headers,
	}
	_, err = btcLcMsgServer.InsertHeaders(ctx, insertHeaderMsg)
	require.NoError(t, err)

	// Delegator signature 생성
	slashingPathInfo, err := testStakingInfo.StakingInfo.SlashingPathSpendInfo()
	require.NoError(t, err)
	delegatorSig, err := testStakingInfo.SlashingTx.Sign(
		stakingMsgTx, 0, slashingPathInfo.GetPkScriptPath(), delSK,
	)
	require.NoError(t, err)

	// 3. Unbonding 관련 정보 생성
	stkTxHash := testStakingInfo.StakingTx.TxHash()
	unbondingValue := stakingValue - datagen.UnbondingTxFee
	testUnbondingInfo := datagen.GenBTCUnbondingSlashingInfo(
		r, t, &chaincfg.SimNetParams,
		delSK, []*btcec.PublicKey{finalityFP.BtcPk.MustToBTCPK()},
		covenantPKs, bsParams.CovenantQuorum,
		wire.NewOutPoint(&stkTxHash, datagen.StakingOutIdx),
		uint16(unbondingTime), unbondingValue,
		bsParams.SlashingPkScript, bsParams.SlashingRate,
		uint16(unbondingTime),
	)
	unbondingTx, err := bbn.SerializeBTCTx(testUnbondingInfo.UnbondingTx)
	require.NoError(t, err)
	delUnbondingSlashingSig, err := testUnbondingInfo.GenDelSlashingTxSig(delSK)
	require.NoError(t, err)

	// 4. 위임 생성 메시지 전송
	msgCreateBTCDel := &types.MsgCreateBTCDelegation{
		StakerAddr:                    stakerAddr.String(),
		FpBtcPkList:                   []bbn.BIP340PubKey{*finalityFP.BtcPk},
		BtcPk:                         bbn.NewBIP340PubKeyFromBTCPK(delSK.PubKey()),
		Pop:                           pop,
		StakingTime:                   uint32(stakingTime),
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
	_, err = btcStakingMsgServer.CreateBTCDelegation(ctx, msgCreateBTCDel)
	require.NoError(t, err)

	// 5. Covenant Signature 추가
	stakingTxHash := testStakingInfo.StakingTx.TxHash()
	vPKs, err := bbn.NewBTCPKsFromBIP340PKs(msgCreateBTCDel.FpBtcPkList)
	require.NoError(t, err)

	covenantSlashingTxSigs, err := datagen.GenCovenantAdaptorSigs(
		covenantSKs, vPKs,
		testStakingInfo.StakingTx, slashingPathInfo.GetPkScriptPath(),
		msgCreateBTCDel.SlashingTx,
	)
	require.NoError(t, err)

	unbondingSlashingPathInfo, err := testUnbondingInfo.UnbondingInfo.SlashingPathSpendInfo()
	require.NoError(t, err)
	covenantUnbondingSlashingTxSigs, err := datagen.GenCovenantAdaptorSigs(
		covenantSKs, vPKs,
		testUnbondingInfo.UnbondingTx, unbondingSlashingPathInfo.GetPkScriptPath(),
		testUnbondingInfo.SlashingTx,
	)
	require.NoError(t, err)

	unbondingPathInfo, err := testStakingInfo.StakingInfo.UnbondingPathSpendInfo()
	require.NoError(t, err)
	covUnbondingSigs, err := datagen.GenCovenantUnbondingSigs(
		covenantSKs, testStakingInfo.StakingTx,
		0, unbondingPathInfo.GetPkScriptPath(),
		testUnbondingInfo.UnbondingTx,
	)
	require.NoError(t, err)

	msgAddCovenantSig := &types.MsgAddCovenantSigs{
		Signer:                  msgCreateBTCDel.StakerAddr,
		Pk:                      covenantSlashingTxSigs[0].CovPk,
		StakingTxHash:           stakingTxHash.String(),
		SlashingTxSigs:          covenantSlashingTxSigs[0].AdaptorSigs,
		UnbondingTxSig:          bbn.NewBIP340SignatureFromBTCSig(covUnbondingSigs[0]),
		SlashingUnbondingTxSigs: covenantUnbondingSlashingTxSigs[0].AdaptorSigs,
	}
	_, err = btcStakingMsgServer.AddCovenantSigs(ctx, msgAddCovenantSig)
	require.NoError(t, err)
	return stakingTxHash, msgCreateBTCDel.StakerAddr
}

// createAndCommitFinalityProvider는 지정된 FinalityProvider 정보를 바탕으로
// CreateFinalityProvider와 CommitPubRandList 과정을 수행합니다.
func createAndCommitFinalityProvider(
	t *testing.T,
	ctx sdk.Context,
	r *rand.Rand,
	fp *types.FinalityProvider,
	fpSK *btcec.PrivateKey,
	btcStakingMsgServer types.MsgServer,
	finalityMsgServer ftypes.MsgServer,
) {
	fpMsg := &types.MsgCreateFinalityProvider{
		Addr:        fp.Addr,
		Description: fp.Description,
		Commission:  btcstakingtype.NewCommissionRates(*fp.Commission, fp.CommissionInfo.MaxRate, fp.CommissionInfo.MaxChangeRate),
		BtcPk:       fp.BtcPk,
		Pop:         fp.Pop,
	}
	_, err := btcStakingMsgServer.CreateFinalityProvider(ctx, fpMsg)
	require.NoError(t, err)

	_, msgCommitPubRandList, err := datagen.GenRandomMsgCommitPubRandList(r, fpSK, 1, 300)
	require.NoError(t, err)
	_, err = finalityMsgServer.CommitPubRandList(ctx, msgCommitPubRandList)
	require.NoError(t, err)
}
