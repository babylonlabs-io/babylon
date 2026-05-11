package tmanager

import (
	"github.com/babylonlabs-io/babylon/v4/btcstaking"
	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	tkeeper "github.com/babylonlabs-io/babylon/v4/testutil/keeper"
	bbn "github.com/babylonlabs-io/babylon/v4/types"
	bstypes "github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
	"github.com/stretchr/testify/require"
)

// CreateBtcDelegationWithSK is like CreateBtcDelegation but takes a caller-supplied
// staker BTC private key, so the test can later reuse it to build a stake-expansion
// child whose taproot keypath spends from the same parent staking output.
func (n *Node) CreateBtcDelegationWithSK(
	wallet *WalletSender,
	stakerSK *btcec.PrivateKey,
	fpPK *btcec.PublicKey,
	stakingValue int64,
	stakingTime uint16,
) (*bstypes.BTCDelegationResponse, *wire.MsgTx) {
	wallet.VerifySentTx = true

	msg, stakingInfoBuilt := n.BuildSingleSigDelegationMsg(
		wallet,
		stakerSK,
		fpPK,
		stakingValue,
		stakingTime,
	)

	n.CreateBTCDelegation(wallet.KeyName, msg)
	n.WaitForNextBlock()

	stakingMsgTxHash := stakingInfoBuilt.StakingTx.TxHash().String()
	pendingDelResp := n.QueryBTCDelegation(stakingMsgTxHash)
	require.NotNil(n.T(), pendingDelResp)
	require.Equal(n.T(), "PENDING", pendingDelResp.StatusDesc)

	pendingDel, err := tkeeper.ParseRespBTCDelToBTCDel(pendingDelResp)
	require.NoError(n.T(), err)
	require.Len(n.T(), pendingDel.CovenantSigs, 0)
	stakingMsgTx, err := bbn.NewBTCTxFromBytes(pendingDel.StakingTx)
	require.NoError(n.T(), err)

	slashingTx := pendingDel.SlashingTx
	stakingTxHash := stakingMsgTx.TxHash().String()
	bsParams := n.QueryBtcStakingParams()

	fpBTCPKs, err := bbn.NewBTCPKsFromBIP340PKs(pendingDel.FpBtcPkList)
	require.NoError(n.T(), err)

	btcCfg := &chaincfg.SimNetParams
	stakingInfo, err := pendingDel.GetStakingInfo(bsParams, btcCfg)
	require.NoError(n.T(), err)

	stakingSlashingPathInfo, err := stakingInfo.SlashingPathSpendInfo()
	require.NoError(n.T(), err)

	covSKs, _, _ := bstypes.DefaultCovenantCommittee()

	covenantSlashingSigs, err := datagen.GenCovenantAdaptorSigs(
		covSKs,
		fpBTCPKs,
		stakingMsgTx,
		stakingSlashingPathInfo.GetPkScriptPath(),
		slashingTx,
	)
	require.NoError(n.T(), err)

	unbondingPathInfo, err := stakingInfo.UnbondingPathSpendInfo()
	require.NoError(n.T(), err)
	unbondingTx, err := bbn.NewBTCTxFromBytes(pendingDel.BtcUndelegation.UnbondingTx)
	require.NoError(n.T(), err)

	covUnbondingSigs, err := datagen.GenCovenantUnbondingSigs(
		covSKs,
		stakingMsgTx,
		pendingDel.StakingOutputIdx,
		unbondingPathInfo.GetPkScriptPath(),
		unbondingTx,
	)
	require.NoError(n.T(), err)

	unbondingInfo, err := pendingDel.GetUnbondingInfo(bsParams, btcCfg)
	require.NoError(n.T(), err)
	unbondingSlashingPathInfo, err := unbondingInfo.SlashingPathSpendInfo()
	require.NoError(n.T(), err)
	covenantUnbondingSlashingSigs, err := datagen.GenCovenantAdaptorSigs(
		covSKs,
		fpBTCPKs,
		unbondingTx,
		unbondingSlashingPathInfo.GetPkScriptPath(),
		pendingDel.BtcUndelegation.SlashingTx,
	)
	require.NoError(n.T(), err)

	for i := 0; i < int(bsParams.CovenantQuorum); i++ {
		n.SubmitRefundableTxWithAssertion(func() {
			n.AddCovenantSigs(
				wallet.KeyName,
				covenantSlashingSigs[i].CovPk,
				stakingTxHash,
				covenantSlashingSigs[i].AdaptorSigs,
				bbn.NewBIP340SignatureFromBTCSig(covUnbondingSigs[i]),
				covenantUnbondingSlashingSigs[i].AdaptorSigs,
				nil,
			)
		}, true, wallet.KeyName)
	}

	verifiedDelResp := n.QueryBTCDelegation(stakingTxHash)
	require.Equal(n.T(), "VERIFIED", verifiedDelResp.StatusDesc)
	verifiedDel, err := tkeeper.ParseRespBTCDelToBTCDel(verifiedDelResp)
	require.NoError(n.T(), err)
	require.Len(n.T(), verifiedDel.CovenantSigs, int(bsParams.CovenantQuorum))
	require.True(n.T(), verifiedDel.HasCovenantQuorums(bsParams.CovenantQuorum, 0))

	currentBtcTipResp, err := n.QueryTip()
	require.NoError(n.T(), err)
	currentBtcTip, err := tkeeper.ParseBTCHeaderInfoResponseToInfo(currentBtcTipResp)
	require.NoError(n.T(), err)
	blockWithStakingTx := datagen.CreateBlockWithTransaction(n.Tm.R, currentBtcTip.Header.ToBlockHeader(), stakingMsgTx)
	n.InsertHeader(&blockWithStakingTx.HeaderBytes)

	inclusionProof := bstypes.NewInclusionProofFromSpvProof(blockWithStakingTx.SpvProof)
	for i := 0; i < BabylonBtcConfirmationPeriod; i++ {
		n.InsertNewEmptyBtcHeader(n.Tm.R)
	}

	n.SubmitRefundableTxWithAssertion(func() {
		n.AddBTCDelegationInclusionProof(wallet.KeyName, stakingTxHash, inclusionProof)
	}, true, wallet.KeyName)

	activeBtcDelResp := n.QueryBTCDelegation(stakingTxHash)
	require.Equal(n.T(), "ACTIVE", activeBtcDelResp.StatusDesc)
	return activeBtcDelResp, stakingMsgTx
}

// BuildSingleSigStakeExpansionMsg builds a MsgBtcStakeExpand that spends
// (parentStkOutput, fundingTxOutput) into a fresh staking output, with all
// slashing/unbonding artifacts derived for a single-sig staker.
func (n *Node) BuildSingleSigStakeExpansionMsg(
	wallet *WalletSender,
	stakerSK *btcec.PrivateKey,
	fpPK *btcec.PublicKey,
	parentStkTx *wire.MsgTx,
	stakingValue int64,
	stakingTime uint16,
	fundingValue int64,
) (*bstypes.MsgBtcStakeExpand, *datagen.TestStakingSlashingInfo, *wire.MsgTx) {
	params := n.QueryBtcStakingParams()
	net := &chaincfg.SimNetParams

	covPKs, err := bbn.NewBTCPKsFromBIP340PKs(params.CovenantPks)
	require.NoError(n.T(), err)

	parentStkOut := parentStkTx.TxOut[datagen.StakingOutIdx]
	dummyHash := chainhash.Hash{}
	for i := range dummyHash {
		dummyHash[i] = byte(i + 1)
	}
	dummyOutPoint := &wire.OutPoint{Hash: dummyHash, Index: 0}
	fundingTx := datagen.GenFundingTx(n.T(), n.Tm.R, net, dummyOutPoint, fundingValue, parentStkOut)
	fundingTxHash := fundingTx.TxHash()

	parentStkTxHash := parentStkTx.TxHash()
	outPoints := []*wire.OutPoint{
		wire.NewOutPoint(&parentStkTxHash, datagen.StakingOutIdx),
		wire.NewOutPoint(&fundingTxHash, 0),
	}

	stakingInfo := datagen.GenBTCStakingSlashingInfoWithInputs(
		n.Tm.R, n.T(), net,
		outPoints,
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

	slashingPathSpendInfo, err := stakingInfo.StakingInfo.SlashingPathSpendInfo()
	require.NoError(n.T(), err)
	delegatorSig, err := stakingInfo.SlashingTx.Sign(
		stakingInfo.StakingTx,
		datagen.StakingOutIdx,
		slashingPathSpendInfo.GetPkScriptPath(),
		stakerSK,
	)
	require.NoError(n.T(), err)

	serializedStakingTx, err := bbn.SerializeBTCTx(stakingInfo.StakingTx)
	require.NoError(n.T(), err)

	stkTxHash := stakingInfo.StakingTx.TxHash()
	unbondingValue := stakingValue - params.UnbondingFeeSat
	unbondingInfo := datagen.GenBTCUnbondingSlashingInfo(
		n.Tm.R, n.T(), net,
		stakerSK,
		[]*btcec.PublicKey{fpPK},
		covPKs,
		params.CovenantQuorum,
		wire.NewOutPoint(&stkTxHash, datagen.StakingOutIdx),
		uint16(params.UnbondingTimeBlocks),
		unbondingValue,
		params.SlashingPkScript,
		params.SlashingRate,
		uint16(params.UnbondingTimeBlocks),
	)

	unbondingTxBytes, err := bbn.SerializeBTCTx(unbondingInfo.UnbondingTx)
	require.NoError(n.T(), err)
	delSlashingTxSig, err := unbondingInfo.GenDelSlashingTxSig(stakerSK)
	require.NoError(n.T(), err)

	pop, err := datagen.NewPoPBTC(wallet.Address, stakerSK)
	require.NoError(n.T(), err)

	fundingTxBz, err := bbn.SerializeBTCTx(fundingTx)
	require.NoError(n.T(), err)

	msg := &bstypes.MsgBtcStakeExpand{
		StakerAddr:                    wallet.Address.String(),
		Pop:                           pop,
		BtcPk:                         bbn.NewBIP340PubKeyFromBTCPK(stakerSK.PubKey()),
		FpBtcPkList:                   []bbn.BIP340PubKey{*bbn.NewBIP340PubKeyFromBTCPK(fpPK)},
		StakingTime:                   uint32(stakingTime),
		StakingValue:                  stakingValue,
		StakingTx:                     serializedStakingTx,
		SlashingTx:                    stakingInfo.SlashingTx,
		DelegatorSlashingSig:          delegatorSig,
		UnbondingValue:                unbondingValue,
		UnbondingTime:                 params.UnbondingTimeBlocks,
		UnbondingTx:                   unbondingTxBytes,
		UnbondingSlashingTx:           unbondingInfo.SlashingTx,
		DelegatorUnbondingSlashingSig: delSlashingTxSig,
		PreviousStakingTxHash:         parentStkTxHash.String(),
		FundingTx:                     fundingTxBz,
	}
	return msg, stakingInfo, fundingTx
}

// CreateBtcStakeExpansionVerified registers a stake-expansion delegation,
// drives it through covenant quorum (including the stake-expansion-specific
// signature on the parent's unbonding path), and asserts the child reaches
// VERIFIED. The child remains without inclusion proof so the test can
// exercise the unbond-before-inclusion-proof scenario.
func (n *Node) CreateBtcStakeExpansionVerified(
	wallet *WalletSender,
	stakerSK *btcec.PrivateKey,
	fpPK *btcec.PublicKey,
	parentDel *bstypes.BTCDelegationResponse,
	parentStkTx *wire.MsgTx,
	stakingValue int64,
	stakingTime uint16,
	fundingValue int64,
) (childResp *bstypes.BTCDelegationResponse, expansionMsg *bstypes.MsgBtcStakeExpand, fundingTx *wire.MsgTx) {
	wallet.VerifySentTx = true

	expansionMsg, _, fundingTx = n.BuildSingleSigStakeExpansionMsg(
		wallet, stakerSK, fpPK, parentStkTx, stakingValue, stakingTime, fundingValue,
	)

	_, tx := wallet.SubmitMsgs(expansionMsg)
	require.NotNil(n.T(), tx, "MsgBtcStakeExpand should not be nil")

	expansionStakingTx, err := bbn.NewBTCTxFromBytes(expansionMsg.StakingTx)
	require.NoError(n.T(), err)
	expansionStakingTxHash := expansionStakingTx.TxHash().String()

	pendingResp := n.QueryBTCDelegation(expansionStakingTxHash)
	require.NotNil(n.T(), pendingResp)
	require.Equal(n.T(), "PENDING", pendingResp.StatusDesc, "child must be PENDING after MsgBtcStakeExpand")
	require.NotNil(n.T(), pendingResp.StkExp, "child must carry stake-expansion metadata")

	pendingDel, err := tkeeper.ParseRespBTCDelToBTCDel(pendingResp)
	require.NoError(n.T(), err)
	bsParams := n.QueryBtcStakingParams()
	fpBTCPKs, err := bbn.NewBTCPKsFromBIP340PKs(pendingDel.FpBtcPkList)
	require.NoError(n.T(), err)
	btcCfg := &chaincfg.SimNetParams

	stakingInfo, err := pendingDel.GetStakingInfo(bsParams, btcCfg)
	require.NoError(n.T(), err)
	stakingSlashingPathInfo, err := stakingInfo.SlashingPathSpendInfo()
	require.NoError(n.T(), err)

	covSKs, _, _ := bstypes.DefaultCovenantCommittee()

	covenantSlashingSigs, err := datagen.GenCovenantAdaptorSigs(
		covSKs, fpBTCPKs, expansionStakingTx,
		stakingSlashingPathInfo.GetPkScriptPath(),
		pendingDel.SlashingTx,
	)
	require.NoError(n.T(), err)

	unbondingPathInfo, err := stakingInfo.UnbondingPathSpendInfo()
	require.NoError(n.T(), err)
	unbondingTx, err := bbn.NewBTCTxFromBytes(pendingDel.BtcUndelegation.UnbondingTx)
	require.NoError(n.T(), err)
	covUnbondingSigs, err := datagen.GenCovenantUnbondingSigs(
		covSKs, expansionStakingTx, pendingDel.StakingOutputIdx,
		unbondingPathInfo.GetPkScriptPath(),
		unbondingTx,
	)
	require.NoError(n.T(), err)

	unbondingInfo, err := pendingDel.GetUnbondingInfo(bsParams, btcCfg)
	require.NoError(n.T(), err)
	unbondingSlashingPathInfo, err := unbondingInfo.SlashingPathSpendInfo()
	require.NoError(n.T(), err)
	covenantUnbondingSlashingSigs, err := datagen.GenCovenantAdaptorSigs(
		covSKs, fpBTCPKs, unbondingTx,
		unbondingSlashingPathInfo.GetPkScriptPath(),
		pendingDel.BtcUndelegation.SlashingTx,
	)
	require.NoError(n.T(), err)

	prevStkHash, err := chainhash.NewHash(pendingDel.StkExp.PreviousStakingTxHash)
	require.NoError(n.T(), err)
	parentStkInfoResp := n.QueryBTCDelegation(prevStkHash.String())
	parentStkDel, err := tkeeper.ParseRespBTCDelToBTCDel(parentStkInfoResp)
	require.NoError(n.T(), err)
	parentStkInfo, err := parentStkDel.GetStakingInfo(bsParams, btcCfg)
	require.NoError(n.T(), err)
	parentUnbondingPathInfo, err := parentStkInfo.UnbondingPathSpendInfo()
	require.NoError(n.T(), err)
	parentStkRawTx, err := bbn.NewBTCTxFromBytes(parentStkDel.StakingTx)
	require.NoError(n.T(), err)

	for i := 0; i < int(bsParams.CovenantQuorum); i++ {
		stkExpSig, err := btcstaking.SignTxForFirstScriptSpendWithTwoInputsFromScript(
			expansionStakingTx,
			parentStkRawTx.TxOut[datagen.StakingOutIdx],
			fundingTx.TxOut[0],
			covSKs[i],
			parentUnbondingPathInfo.GetPkScriptPath(),
		)
		require.NoError(n.T(), err)
		stkExpSigBIP := bbn.NewBIP340SignatureFromBTCSig(stkExpSig)

		n.SubmitRefundableTxWithAssertion(func() {
			n.AddCovenantSigs(
				wallet.KeyName,
				covenantSlashingSigs[i].CovPk,
				expansionStakingTxHash,
				covenantSlashingSigs[i].AdaptorSigs,
				bbn.NewBIP340SignatureFromBTCSig(covUnbondingSigs[i]),
				covenantUnbondingSlashingSigs[i].AdaptorSigs,
				stkExpSigBIP,
			)
		}, true, wallet.KeyName)
	}

	verifiedResp := n.QueryBTCDelegation(expansionStakingTxHash)
	require.Equal(n.T(), "VERIFIED", verifiedResp.StatusDesc,
		"child must be VERIFIED after covenant quorum (no inclusion proof yet)")
	return verifiedResp, expansionMsg, fundingTx
}

// SubmitBTCUndelegate submits a MsgBTCUndelegate and asserts it is accepted at
// DeliverTx; the wallet is configured to verify the next block.
func (n *Node) SubmitBTCUndelegate(wallet *WalletSender, msg *bstypes.MsgBTCUndelegate) string {
	wallet.VerifySentTx = true
	txHash, tx := wallet.SubmitMsgs(msg)
	require.NotNil(n.T(), tx, "MsgBTCUndelegate should not be nil")
	return txHash
}

// SubmitBTCUndelegateExpectFail signs and broadcasts a MsgBTCUndelegate that is
// expected to pass CheckTx but fail at DeliverTx. Returns the raw log so the
// caller can assert on the rejection reason.
func (n *Node) SubmitBTCUndelegateExpectFail(wallet *WalletSender, msg *bstypes.MsgBTCUndelegate) string {
	wallet.VerifySentTx = false
	signedTx := wallet.SignMsg(msg)
	txHash, err := n.SubmitTx(signedTx)
	require.NoError(n.T(), err, "broadcast must succeed; failure is expected at DeliverTx")

	n.WaitForNextBlock()
	txResp := n.QueryTxByHash(txHash)
	require.NotZero(n.T(), txResp.TxResponse.Code,
		"MsgBTCUndelegate must fail at DeliverTx — RawLog=%q", txResp.TxResponse.RawLog)
	return txResp.TxResponse.RawLog
}
