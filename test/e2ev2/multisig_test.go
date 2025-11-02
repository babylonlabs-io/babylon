package e2e2

import (
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/wire"

	"github.com/babylonlabs-io/babylon/v4/test/e2e/configurer/chain"
	"github.com/babylonlabs-io/babylon/v4/test/e2ev2/tmanager"
	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	bbn "github.com/babylonlabs-io/babylon/v4/types"
	bstypes "github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
)

func TestMultisigBtcDel(t *testing.T) {
	bbn2, fpSK, r := startChainAndCreateFp(t)
	bbn2.DefaultWallet().VerifySentTx = true

	testCases := []struct {
		title        string
		stakerQuorum uint32
		stakerCount  uint32
		sigsCount    uint32
		expErr       string
	}{
		{
			title:        "2-of-3 multisig delegation, 2 valid signatures",
			stakerQuorum: 2,
			stakerCount:  3,
			sigsCount:    2,
			expErr:       "",
		},
		{
			title:        "2-of-3 multisig delegation, 3 valid signatures",
			stakerQuorum: 2,
			stakerCount:  3,
			sigsCount:    3,
			expErr:       "",
		},
		{
			title:        "3-of-5 multisig delegation, 3 valid signatures - max 2-of-3 multisig params",
			stakerQuorum: 3,
			stakerCount:  5,
			sigsCount:    3,
			expErr:       "invalid multisig info",
		},
		{
			title:        "2-of-3 multisig delegation, 1 valid signatures",
			stakerQuorum: 2,
			stakerCount:  3,
			sigsCount:    1,
			expErr:       "invalid multisig info",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.title, func(t *testing.T) {
			// override VerifySentTx if it expects error
			if tc.expErr != "" {
				bbn2.DefaultWallet().VerifySentTx = false
			}

			// multisig delegation from bbn2 to fp (bbn1)
			stkSKs, _, err := datagen.GenRandomBTCKeyPairs(r, int(tc.stakerCount))
			require.NoError(t, err)

			msg, stakingInfo := buildMultisigDelegationMsgWithSigCount(
				t, r, bbn2,
				bbn2.DefaultWallet(),
				stkSKs,
				tc.stakerQuorum,
				fpSK.PubKey(),
				int64(2*10e8),
				1000,
				tc.sigsCount,
			)

			txHash := bbn2.CreateBTCDelegation(bbn2.DefaultWallet().KeyName, msg)
			bbn2.WaitForNextBlock()

			// if it expects error, don't query btc delegation and stop here
			if tc.expErr != "" {
				bbn2.RequireTxErrorContain(txHash, tc.expErr)
				return
			}

			// query and verify delegation
			del := bbn2.QueryBTCDelegation(stakingInfo.StakingTx.TxHash().String())
			require.NotNil(t, del)
			require.Equal(t, "PENDING", del.StatusDesc)
			require.NotNil(t, del.MultisigInfo)
			require.Equal(t, tc.stakerQuorum, del.MultisigInfo.StakerQuorum)
			require.Len(t, del.MultisigInfo.StakerBtcPkList, int(tc.stakerCount-1))
			require.Len(t, del.MultisigInfo.DelegatorSlashingSigs, int(tc.sigsCount-1))
		})
	}
}

// TestSingleSigBtcDel tests original single-signature BTC delegation (no multisig info)
// this is a regression test to ensure multisig changes don't break single-sig functionality
func TestSingleSigBtcDel(t *testing.T) {
	bbn2, fpSK, r := startChainAndCreateFp(t)
	bbn2.DefaultWallet().VerifySentTx = true

	// single-sig delegation from bbn2 to fp (bbn1)
	stakerSK, _, err := datagen.GenRandomBTCKeyPair(r)
	require.NoError(t, err)

	msg, stakingInfoBuilt := BuildSingleSigDelegationMsg(
		t, r, bbn2,
		bbn2.DefaultWallet(),
		stakerSK,
		fpSK.PubKey(),
		int64(2*10e8),
		1000,
	)

	bbn2.CreateBTCDelegation(bbn2.DefaultWallet().KeyName, msg)
	bbn2.WaitForNextBlock()

	pendingDelResp := bbn2.QueryBTCDelegation(stakingInfoBuilt.StakingTx.TxHash().String())
	require.NotNil(t, pendingDelResp)
	require.Equal(t, "PENDING", pendingDelResp.StatusDesc)

	/*
		generate and insert new covenant signatures, in order to verify the BTC delegation
	*/
	pendingDel, err := chain.ParseRespBTCDelToBTCDel(pendingDelResp)
	require.NoError(t, err)
	require.Len(t, pendingDel.CovenantSigs, 0)
	stakingMsgTx, err := bbn.NewBTCTxFromBytes(pendingDel.StakingTx)
	require.NoError(t, err)

	slashingTx := pendingDel.SlashingTx
	stakingTxHash := stakingMsgTx.TxHash().String()
	bsParams := bbn2.QueryBtcStakingParams()

	fpBTCPKs, err := bbn.NewBTCPKsFromBIP340PKs(pendingDel.FpBtcPkList)
	require.NoError(t, err)

	btcCfg := &chaincfg.SimNetParams
	stakingInfo, err := pendingDel.GetStakingInfo(bsParams, btcCfg)
	require.NoError(t, err)

	stakingSlashingPathInfo, err := stakingInfo.SlashingPathSpendInfo()
	require.NoError(t, err)

	// it should be changed when modifying covenant pk on chain start
	covSKs, _, _ := bstypes.DefaultCovenantCommittee()

	// covenant signatures on slashing tx
	covenantSlashingSigs, err := datagen.GenCovenantAdaptorSigs(
		covSKs,
		fpBTCPKs,
		stakingMsgTx,
		stakingSlashingPathInfo.GetPkScriptPath(),
		slashingTx,
	)
	require.NoError(t, err)

	// cov Schnorr sigs on unbonding signature
	unbondingPathInfo, err := stakingInfo.UnbondingPathSpendInfo()
	require.NoError(t, err)
	unbondingTx, err := bbn.NewBTCTxFromBytes(pendingDel.BtcUndelegation.UnbondingTx)
	require.NoError(t, err)

	covUnbondingSigs, err := datagen.GenCovenantUnbondingSigs(
		covSKs,
		stakingMsgTx,
		pendingDel.StakingOutputIdx,
		unbondingPathInfo.GetPkScriptPath(),
		unbondingTx,
	)
	require.NoError(t, err)

	unbondingInfo, err := pendingDel.GetUnbondingInfo(bsParams, btcCfg)
	require.NoError(t, err)
	unbondingSlashingPathInfo, err := unbondingInfo.SlashingPathSpendInfo()
	require.NoError(t, err)
	covenantUnbondingSlashingSigs, err := datagen.GenCovenantAdaptorSigs(
		covSKs,
		fpBTCPKs,
		unbondingTx,
		unbondingSlashingPathInfo.GetPkScriptPath(),
		pendingDel.BtcUndelegation.SlashingTx,
	)
	require.NoError(t, err)

	for i := 0; i < int(bsParams.CovenantQuorum); i++ {
		bbn2.SubmitRefundableTxWithAssertion(func() {
			bbn2.AddCovenantSigs(
				bbn2.DefaultWallet().KeyName,
				covenantSlashingSigs[i].CovPk,
				stakingTxHash,
				covenantSlashingSigs[i].AdaptorSigs,
				bbn.NewBIP340SignatureFromBTCSig(covUnbondingSigs[i]),
				covenantUnbondingSlashingSigs[i].AdaptorSigs,
				nil,
			)
		}, true, bbn2.DefaultWallet().KeyName)
	}

	verifiedDelResp := bbn2.QueryBTCDelegation(stakingTxHash)
	require.Equal(t, "VERIFIED", verifiedDelResp.StatusDesc)
	verifiedDel, err := chain.ParseRespBTCDelToBTCDel(verifiedDelResp)
	require.NoError(t, err)
	require.Len(t, verifiedDel.CovenantSigs, int(bsParams.CovenantQuorum))
	require.True(t, verifiedDel.HasCovenantQuorums(bsParams.CovenantQuorum, 0))

	/*
		generate and add inclusion proof, in order to activate the BTC delegation
	*/
	// wait for btc delegation is k-deep
	currentBtcTipResp, err := bbn2.QueryTip()
	require.NoError(t, err)
	currentBtcTip, err := chain.ParseBTCHeaderInfoResponseToInfo(currentBtcTipResp)
	blockWithStakingTx := datagen.CreateBlockWithTransaction(r, currentBtcTip.Header.ToBlockHeader(), stakingMsgTx)
	bbn2.InsertHeader(&blockWithStakingTx.HeaderBytes)

	inclusionProof := bstypes.NewInclusionProofFromSpvProof(blockWithStakingTx.SpvProof)
	for i := 0; i < tmanager.BabylonBtcConfirmationPeriod; i++ {
		bbn2.InsertNewEmptyBtcHeader(r)
	}

	// add btc inclusion proof
	bbn2.SubmitRefundableTxWithAssertion(func() {
		bbn2.AddBTCDelegationInclusionProof(bbn2.DefaultWallet().KeyName, stakingTxHash, inclusionProof)
	}, true, bbn2.DefaultWallet().KeyName)

	activeBtcDelResp := bbn2.QueryBTCDelegation(stakingTxHash)
	require.Equal(t, "ACTIVE", activeBtcDelResp.StatusDesc)
	activeBtcDel, err := chain.ParseRespBTCDelToBTCDel(activeBtcDelResp)
	require.NoError(t, err)
	require.Len(t, activeBtcDel.CovenantSigs, int(bsParams.CovenantQuorum))
	require.True(t, activeBtcDel.HasCovenantQuorums(bsParams.CovenantQuorum, 0))
	require.Nil(t, activeBtcDel.MultisigInfo, "Single-sig delegation should not have MultisigInfo")
	require.NotNil(t, activeBtcDel.DelegatorSig, "Single-sig delegation should have delegator signature")
}

// TestMultisigBtcDelWithDuplicates tests that duplicate staker keys and signatures
func TestMultisigBtcDelWithDuplicates(t *testing.T) {
	bbn2, fpSK, r := startChainAndCreateFp(t)

	testCases := []struct {
		title    string
		dupSetup func(*bstypes.MsgCreateBTCDelegation) *bstypes.MsgCreateBTCDelegation
		expErr   string
	}{
		{
			title: "duplicated staker pk in StakerBtcPkList",
			dupSetup: func(msg *bstypes.MsgCreateBTCDelegation) *bstypes.MsgCreateBTCDelegation {
				msg.MultisigInfo.StakerBtcPkList[0] = msg.MultisigInfo.StakerBtcPkList[1]
				return msg
			},
			expErr: "multisig staker key is duplicated",
		},
		{
			title: "duplicated between BtcPk and StakerBtcPkList",
			dupSetup: func(msg *bstypes.MsgCreateBTCDelegation) *bstypes.MsgCreateBTCDelegation {
				msg.MultisigInfo.StakerBtcPkList[0] = *msg.BtcPk
				return msg
			},
			expErr: "staker pk list contains the main staker pk",
		},
		{
			title: "duplicated slashing sigs in the multisig info",
			dupSetup: func(msg *bstypes.MsgCreateBTCDelegation) *bstypes.MsgCreateBTCDelegation {
				msg.MultisigInfo.DelegatorSlashingSigs[0].Sig = msg.DelegatorSlashingSig
				return msg
			},
			expErr: "invalid delegator signature",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.title, func(t *testing.T) {
			// multisig delegation from bbn2 to fp (bbn1)
			stkSKs, _, err := datagen.GenRandomBTCKeyPairs(r, 3)
			require.NoError(t, err)

			msg, _ := buildMultisigDelegationMsgWithSigCount(
				t, r, bbn2,
				bbn2.DefaultWallet(),
				stkSKs,
				2,
				fpSK.PubKey(),
				int64(2*10e8),
				1000,
				2,
			)

			// setup duplicated staker keys
			msg = tc.dupSetup(msg)

			// for duplicate checks, the error might happen before block inclusion (during ValidateBasic)
			wallet2 := bbn2.DefaultWallet()
			signedTx := wallet2.SignMsg(msg)
			txHash, err := bbn2.SubmitTx(signedTx)
			if err != nil {
				// transaction rejected before block inclusion
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.expErr)
				// reset sequence since it fails to submit tx
				bbn2.DefaultWallet().DecSeq()
			} else {
				// transaction included in block but failed during execution
				bbn2.WaitForNextBlock()
				bbn2.RequireTxErrorContain(txHash, tc.expErr)
			}
		})
	}
}

func TestMultisigBtcDelWithZeroQuorum(t *testing.T) {
	bbn2, fpSK, r := startChainAndCreateFp(t)

	// multisig delegation from bbn2 to fp (bbn1)
	stkSKs, _, err := datagen.GenRandomBTCKeyPairs(r, 3)
	require.NoError(t, err)

	msg, _ := buildMultisigDelegationMsgWithSigCount(
		t, r, bbn2,
		bbn2.DefaultWallet(),
		stkSKs,
		2,
		fpSK.PubKey(),
		int64(2*10e8),
		1000,
		2,
	)
	msg.MultisigInfo.StakerQuorum = 0

	txHash := bbn2.CreateBTCDelegation(bbn2.DefaultWallet().KeyName, msg)
	bbn2.WaitForNextBlock()

	bbn2.RequireTxErrorContain(txHash, "number of staker btc pk list and staker quorum must be greater than 0")
}

func startChainAndCreateFp(t *testing.T) (bbn2 *tmanager.Node, fpSK *btcec.PrivateKey, r *rand.Rand) {
	t.Parallel()
	tm := tmanager.NewTestManager(t)
	cfg := tmanager.NewChainConfig(tm.TempDir, tmanager.CHAIN_ID_BABYLON)
	cfg.NodeCount = 2
	cfg.StartingBtcStakingParams = &tmanager.StartingBtcStakingParams{
		MaxStakerNum:    3,
		MaxStakerQuorum: 2,
	}
	tm.Chains[tmanager.CHAIN_ID_BABYLON] = tmanager.NewChain(tm, cfg)
	tm.Start()

	tm.ChainsWaitUntilHeight(3)

	bbns := tm.ChainNodes()
	bbn1 := bbns[0]
	bbn2 = bbns[1]
	bbn1.DefaultWallet().VerifySentTx = true
	bbn2.DefaultWallet().VerifySentTx = false

	// create bbn1 as a fp
	r = rand.New(rand.NewSource(time.Now().Unix()))
	fpSK, _, err := datagen.GenRandomBTCKeyPair(r)
	require.NoError(t, err)
	fp, err := datagen.GenCustomFinalityProvider(r, fpSK, bbn1.DefaultWallet().Address)
	require.NoError(t, err)
	bbn1.CreateFinalityProvider(bbn1.DefaultWallet().KeyName, fp)
	bbn1.WaitForNextBlock()

	fpResp := bbn1.QueryFinalityProvider(fp.BtcPk.MarshalHex())
	require.NotNil(t, fpResp)

	return
}

// buildMultisigDelegationWithSigCount construct multisig btc delegation msg
// - sigsCount is the number of signatures
func buildMultisigDelegationMsgWithSigCount(
	t *testing.T,
	r *rand.Rand,
	node *tmanager.Node,
	wallet *tmanager.WalletSender,
	stakerSKs []*btcec.PrivateKey,
	stakerQuorum uint32,
	fpPK *btcec.PublicKey,
	stakingValue int64,
	stakingTime uint16,
	sigsCount uint32,
) (*bstypes.MsgCreateBTCDelegation, *datagen.TestStakingSlashingInfo) {
	params := node.QueryBtcStakingParams()
	net := &chaincfg.SimNetParams

	covPKs, err := bbn.NewBTCPKsFromBIP340PKs(params.CovenantPks)
	require.NoError(t, err)

	// generate staking + slashing info
	stakingInfo := datagen.GenMultisigBTCStakingSlashingInfo(
		r, t, net,
		stakerSKs,
		stakerQuorum,
		[]*btcec.PublicKey{fpPK},
		covPKs,
		params.CovenantQuorum,
		stakingTime,
		stakingValue,
		params.SlashingPkScript,
		params.SlashingRate,
		uint16(params.UnbondingTimeBlocks),
	)

	// generate unbonding info
	unbondingValue := stakingValue - params.UnbondingFeeSat
	stkTxHash := stakingInfo.StakingTx.TxHash()

	unbondingInfo := datagen.GenMultisigBTCUnbondingSlashingInfo(
		r, t, net,
		stakerSKs,
		stakerQuorum,
		[]*btcec.PublicKey{fpPK},
		covPKs,
		params.CovenantQuorum,
		&wire.OutPoint{Hash: stkTxHash, Index: 0},
		uint16(params.UnbondingTimeBlocks),
		unbondingValue,
		params.SlashingPkScript,
		params.SlashingRate,
		uint16(params.UnbondingTimeBlocks),
	)

	// sign slashing tx with primary staker (first one)
	slashingSpendInfo, err := stakingInfo.StakingInfo.SlashingPathSpendInfo()
	require.NoError(t, err)

	delegatorSig, err := stakingInfo.SlashingTx.Sign(
		stakingInfo.StakingTx, 0,
		slashingSpendInfo.GetPkScriptPath(),
		stakerSKs[0],
	)
	require.NoError(t, err)

	// generate extra staker signatures (for remaining stakers)
	var extraSlashingSigs []*bstypes.SignatureInfo
	stakerSKList := stakerSKs[1:sigsCount]
	for _, sk := range stakerSKList {
		sig, err := stakingInfo.SlashingTx.Sign(
			stakingInfo.StakingTx, 0,
			slashingSpendInfo.GetPkScriptPath(),
			sk,
		)
		require.NoError(t, err)

		extraSlashingSigs = append(extraSlashingSigs, &bstypes.SignatureInfo{
			Pk:  bbn.NewBIP340PubKeyFromBTCPK(sk.PubKey()),
			Sig: sig,
		})
	}

	// sign unbonding slashing tx with primary staker
	delUnbondingSig, err := unbondingInfo.GenDelSlashingTxSig(stakerSKs[0])
	require.NoError(t, err)

	// generate extra unbonding signatures
	var extraUnbondingSigs []*bstypes.SignatureInfo
	for _, sk := range stakerSKList {
		sig, err := unbondingInfo.GenDelSlashingTxSig(sk)
		require.NoError(t, err)

		extraUnbondingSigs = append(extraUnbondingSigs, &bstypes.SignatureInfo{
			Pk:  bbn.NewBIP340PubKeyFromBTCPK(sk.PubKey()),
			Sig: sig,
		})
	}

	// generate PoP for primary staker
	pop, err := datagen.NewPoPBTC(wallet.Address, stakerSKs[0])
	require.NoError(t, err)

	// serialize transactions
	serializedStakingTx, err := bbn.SerializeBTCTx(stakingInfo.StakingTx)
	require.NoError(t, err)
	serializedUnbondingTx, err := bbn.SerializeBTCTx(unbondingInfo.UnbondingTx)
	require.NoError(t, err)

	// build extra staker PK list (all stakers except the first one)
	extraStakerPKs := make([]bbn.BIP340PubKey, len(stakerSKs)-1)
	for i, sk := range stakerSKs[1:] {
		extraStakerPKs[i] = *bbn.NewBIP340PubKeyFromBTCPK(sk.PubKey())
	}

	return &bstypes.MsgCreateBTCDelegation{
		StakerAddr:                    wallet.Address.String(),
		BtcPk:                         bbn.NewBIP340PubKeyFromBTCPK(stakerSKs[0].PubKey()),
		FpBtcPkList:                   []bbn.BIP340PubKey{*bbn.NewBIP340PubKeyFromBTCPK(fpPK)},
		Pop:                           pop,
		StakingTime:                   uint32(stakingTime),
		StakingValue:                  stakingValue,
		StakingTx:                     serializedStakingTx,
		SlashingTx:                    stakingInfo.SlashingTx,
		DelegatorSlashingSig:          delegatorSig,
		UnbondingTx:                   serializedUnbondingTx,
		UnbondingTime:                 params.UnbondingTimeBlocks,
		UnbondingValue:                unbondingValue,
		UnbondingSlashingTx:           unbondingInfo.SlashingTx,
		DelegatorUnbondingSlashingSig: delUnbondingSig,
		MultisigInfo: &bstypes.AdditionalStakerInfo{
			StakerBtcPkList:                extraStakerPKs,
			StakerQuorum:                   stakerQuorum,
			DelegatorSlashingSigs:          extraSlashingSigs,
			DelegatorUnbondingSlashingSigs: extraUnbondingSigs,
		},
	}, stakingInfo
}

// BuildSingleSigDelegationMsg constructs a original single-sig BTC delegation message
func BuildSingleSigDelegationMsg(
	t *testing.T,
	r *rand.Rand,
	node *tmanager.Node,
	wallet *tmanager.WalletSender,
	stakerSK *btcec.PrivateKey,
	fpPK *btcec.PublicKey,
	stakingValue int64,
	stakingTime uint16,
) (*bstypes.MsgCreateBTCDelegation, *datagen.TestStakingSlashingInfo) {
	params := node.QueryBtcStakingParams()
	net := &chaincfg.SimNetParams

	covPKs, err := bbn.NewBTCPKsFromBIP340PKs(params.CovenantPks)
	require.NoError(t, err)

	// generate staking + slashing info
	stakingInfo := datagen.GenBTCStakingSlashingInfo(
		r, t, net,
		stakerSK,
		[]*btcec.PublicKey{fpPK},
		covPKs,
		params.CovenantQuorum,
		stakingTime,
		stakingValue,
		params.SlashingPkScript,
		params.SlashingRate,
		uint16(params.UnbondingTimeBlocks),
	)

	// generate unbonding info
	unbondingValue := stakingValue - params.UnbondingFeeSat
	stkTxHash := stakingInfo.StakingTx.TxHash()

	unbondingInfo := datagen.GenBTCUnbondingSlashingInfo(
		r, t, net,
		stakerSK,
		[]*btcec.PublicKey{fpPK},
		covPKs,
		params.CovenantQuorum,
		&wire.OutPoint{Hash: stkTxHash, Index: 0},
		uint16(params.UnbondingTimeBlocks),
		unbondingValue,
		params.SlashingPkScript,
		params.SlashingRate,
		uint16(params.UnbondingTimeBlocks),
	)

	// sign slashing tx
	slashingSpendInfo, err := stakingInfo.StakingInfo.SlashingPathSpendInfo()
	require.NoError(t, err)

	delegatorSig, err := stakingInfo.SlashingTx.Sign(
		stakingInfo.StakingTx, 0,
		slashingSpendInfo.GetPkScriptPath(),
		stakerSK,
	)
	require.NoError(t, err)

	// sign unbonding slashing tx
	delUnbondingSig, err := unbondingInfo.GenDelSlashingTxSig(stakerSK)
	require.NoError(t, err)

	// generate PoP
	pop, err := datagen.NewPoPBTC(wallet.Address, stakerSK)
	require.NoError(t, err)

	// serialize transactions
	serializedStakingTx, err := bbn.SerializeBTCTx(stakingInfo.StakingTx)
	require.NoError(t, err)
	serializedUnbondingTx, err := bbn.SerializeBTCTx(unbondingInfo.UnbondingTx)
	require.NoError(t, err)

	return &bstypes.MsgCreateBTCDelegation{
		StakerAddr:                    wallet.Address.String(),
		BtcPk:                         bbn.NewBIP340PubKeyFromBTCPK(stakerSK.PubKey()),
		FpBtcPkList:                   []bbn.BIP340PubKey{*bbn.NewBIP340PubKeyFromBTCPK(fpPK)},
		Pop:                           pop,
		StakingTime:                   uint32(stakingTime),
		StakingValue:                  stakingValue,
		StakingTx:                     serializedStakingTx,
		SlashingTx:                    stakingInfo.SlashingTx,
		DelegatorSlashingSig:          delegatorSig,
		UnbondingTx:                   serializedUnbondingTx,
		UnbondingTime:                 params.UnbondingTimeBlocks,
		UnbondingValue:                unbondingValue,
		UnbondingSlashingTx:           unbondingInfo.SlashingTx,
		DelegatorUnbondingSlashingSig: delUnbondingSig,
		MultisigInfo:                  nil, // no multisig info for single-sig delegation
	}, stakingInfo
}
