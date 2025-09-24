package chain

import (
	"encoding/hex"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"testing"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/stretchr/testify/require"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"

	sdkmath "cosmossdk.io/math"
	"github.com/cometbft/cometbft/libs/bytes"
	cmtcrypto "github.com/cometbft/cometbft/proto/tendermint/crypto"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/babylonlabs-io/babylon/v4/crypto/eots"
	asig "github.com/babylonlabs-io/babylon/v4/crypto/schnorr-adaptor-signature"
	"github.com/babylonlabs-io/babylon/v4/test/e2e/containers"
	"github.com/babylonlabs-io/babylon/v4/test/e2e/initialization"
	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	bbn "github.com/babylonlabs-io/babylon/v4/types"
	"github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
	bstypes "github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
)

func (n *NodeConfig) CreateFinalityProvider(walletAddrOrName string, btcPK *bbn.BIP340PubKey, pop *bstypes.ProofOfPossessionBTC, moniker, identity, website, securityContract, details string, commission *sdkmath.LegacyDec, commissionMaxRate, commissionMaxRateChange sdkmath.LegacyDec) {
	n.LogActionF("creating finality provider")

	// get BTC PK hex
	btcPKHex := btcPK.MarshalHex()
	// get pop hex
	popHex, err := pop.ToHexStr()
	require.NoError(n.t, err)

	cmd := []string{
		"babylond", "tx", "btcstaking", "create-finality-provider", btcPKHex, popHex,
		fmt.Sprintf("--from=%s", walletAddrOrName), "--moniker", moniker, "--identity", identity, "--website", website,
		"--security-contact", securityContract, "--details", details, "--commission-rate", commission.String(),
		"--commission-max-rate", commissionMaxRate.String(), "--commission-max-change-rate", commissionMaxRateChange.String(),
	}
	_, _, err = n.containerManager.ExecTxCmd(n.t, n.chainId, n.Name, cmd)
	require.NoError(n.t, err)
	n.LogActionF("successfully created finality provider")
}

func (n *NodeConfig) CreateBTCDelegation(
	btcPk *bbn.BIP340PubKey,
	pop *bstypes.ProofOfPossessionBTC,
	stakingTx []byte,
	inclusionProof *bstypes.InclusionProof,
	fpPK *bbn.BIP340PubKey,
	stakingTimeBlocks uint16,
	stakingValue btcutil.Amount,
	slashingTx *bstypes.BTCSlashingTx,
	delegatorSig *bbn.BIP340Signature,
	unbondingTx *wire.MsgTx,
	unbondingSlashingTx *bstypes.BTCSlashingTx,
	unbondingTime uint16,
	unbondingValue btcutil.Amount,
	delUnbondingSlashingSig *bbn.BIP340Signature,
	fromWalletName string,
	generateOnly bool,
	overallFlags ...string,
) (outStr string) {
	n.LogActionF("creating BTC delegation")

	btcPkHex := btcPk.MarshalHex()

	// get pop hex
	popHex, err := pop.ToHexStr()
	require.NoError(n.t, err)

	// get staking tx info hex
	stakingTxHex := hex.EncodeToString(stakingTx)

	// get inclusion proof hex
	var inclusionProofHex string

	if inclusionProof != nil {
		inclusionProofHex, err = inclusionProof.MarshalHex()
		require.NoError(n.t, err)
	}

	fpPKHex := fpPK.MarshalHex()

	stakingTimeString := sdkmath.NewUint(uint64(stakingTimeBlocks)).String()
	stakingValueString := sdkmath.NewInt(int64(stakingValue)).String()

	// get slashing tx hex
	slashingTxHex := slashingTx.ToHexStr()
	// get delegator sig hex
	delegatorSigHex := delegatorSig.ToHexStr()

	// on-demand unbonding related
	unbondingTxBytes, err := bbn.SerializeBTCTx(unbondingTx)
	require.NoError(n.t, err)
	unbondingTxHex := hex.EncodeToString(unbondingTxBytes)
	unbondingSlashingTxHex := unbondingSlashingTx.ToHexStr()
	unbondingTimeStr := sdkmath.NewUint(uint64(unbondingTime)).String()
	unbondingValueStr := sdkmath.NewInt(int64(unbondingValue)).String()
	delUnbondingSlashingSigHex := delUnbondingSlashingSig.ToHexStr()

	cmd := []string{
		"babylond", "tx", "btcstaking", "create-btc-delegation",
		btcPkHex, popHex, stakingTxHex, inclusionProofHex, fpPKHex, stakingTimeString, stakingValueString, slashingTxHex, delegatorSigHex, unbondingTxHex, unbondingSlashingTxHex, unbondingTimeStr, unbondingValueStr, delUnbondingSlashingSigHex,
		fmt.Sprintf("--from=%s", fromWalletName), containers.FlagHome, flagKeyringTest,
		n.FlagChainID(), "--log_format=json",
	}

	// gas price
	cmd = append(cmd, "--gas-prices=0.1ubbn")

	if generateOnly {
		cmd = append(cmd, "--generate-only")
	} else {
		// broadcast stuff
		cmd = append(cmd, "-b=sync", "--yes")
	}

	cmd = append(cmd, fmt.Sprintf("--chain-id=%s", n.chainId))
	outBuff, _, err := n.containerManager.ExecCmd(n.t, n.Name, append(cmd, overallFlags...), "")
	require.NoError(n.t, err)
	n.LogActionF("successfully created BTC delegation")
	return outBuff.String()
}

func (n *NodeConfig) CreateBTCStakeExpansionDelegation(
	msg *bstypes.MsgBtcStakeExpand,
	fromWalletName string,
	generateOnly bool,
	overallFlags ...string,
) (outStr string) {
	n.LogActionF("creating BTC delegation")

	btcPKHex := msg.BtcPk.MarshalHex()

	// get pop hex
	popHex, err := msg.Pop.ToHexStr()
	require.NoError(n.t, err)

	// get staking tx info hex
	stakingTxHex := hex.EncodeToString(msg.StakingTx)

	fpPKHexList := make([]string, len(msg.FpBtcPkList))
	for i, fpPK := range msg.FpBtcPkList {
		fpPKHexList[i] = fpPK.MarshalHex()
	}
	fpPKHexes := strings.Join(fpPKHexList, ",")

	stakingTimeString := sdkmath.NewUint(uint64(msg.StakingTime)).String()
	stakingValueString := sdkmath.NewInt(msg.StakingValue).String()

	// get slashing tx hex
	slashingTxHex := msg.SlashingTx.ToHexStr()
	// get delegator sig hex
	delegatorSigHex := msg.DelegatorSlashingSig.ToHexStr()

	// on-demand unbonding related
	unbondingTxHex := hex.EncodeToString(msg.UnbondingTx)
	unbondingSlashingTxHex := msg.UnbondingSlashingTx.ToHexStr()
	unbondingTimeStr := sdkmath.NewUint(uint64(msg.UnbondingTime)).String()
	unbondingValueStr := sdkmath.NewInt(msg.UnbondingValue).String()
	delUnbondingSlashingSigHex := msg.DelegatorUnbondingSlashingSig.ToHexStr()

	fundingTxHex := hex.EncodeToString(msg.FundingTx)

	var inclusionProofHex string
	cmd := []string{
		"babylond", "tx", "btcstaking", "btc-stake-expand",
		btcPKHex, popHex, stakingTxHex, inclusionProofHex, fpPKHexes, stakingTimeString, stakingValueString, slashingTxHex, delegatorSigHex, unbondingTxHex, unbondingSlashingTxHex, unbondingTimeStr, unbondingValueStr, delUnbondingSlashingSigHex,
		msg.PreviousStakingTxHash, fundingTxHex,
		fmt.Sprintf("--from=%s", fromWalletName), containers.FlagHome, flagKeyringTest,
		n.FlagChainID(), "--log_format=json",
	}

	// gas price
	cmd = append(cmd, "--gas-prices=0.1ubbn")

	if generateOnly {
		cmd = append(cmd, "--generate-only")
	} else {
		// broadcast stuff
		cmd = append(cmd, "-b=sync", "--yes")
	}

	cmd = append(cmd, fmt.Sprintf("--chain-id=%s", n.chainId))
	outBuff, _, err := n.containerManager.ExecCmd(n.t, n.Name, append(cmd, overallFlags...), "")

	require.NoError(n.t, err)
	n.LogActionF("successfully created BTC stake expansion delegation")
	return outBuff.String()
}

func (n *NodeConfig) AddCovenantSigsFromVal(covPK *bbn.BIP340PubKey, stakingTxHash string, slashingSigs [][]byte, unbondingSig *bbn.BIP340Signature, unbondingSlashingSigs [][]byte) {
	n.AddCovenantSigs("val", covPK, stakingTxHash, slashingSigs, unbondingSig, unbondingSlashingSigs, nil)
}

func (n *NodeConfig) AddCovenantSigsFromValForStakeExp(covPK *bbn.BIP340PubKey, stakingTxHash string, slashingSigs [][]byte, unbondingSig *bbn.BIP340Signature, unbondingSlashingSigs [][]byte, stkExpSig *bbn.BIP340Signature) string {
	return n.AddCovenantSigs("val", covPK, stakingTxHash, slashingSigs, unbondingSig, unbondingSlashingSigs, stkExpSig)
}

func (n *NodeConfig) AddCovenantSigs(
	fromWalletName string,
	covPK *bbn.BIP340PubKey,
	stakingTxHash string,
	slashingSigs [][]byte,
	unbondingSig *bbn.BIP340Signature,
	unbondingSlashingSigs [][]byte,
	stakeExpTxSig *bbn.BIP340Signature,
) string {
	n.LogActionF("adding covenant signature from nodeName: %s", n.Name)

	covPKHex := covPK.MarshalHex()

	cmd := []string{"babylond", "tx", "btcstaking", "add-covenant-sigs", covPKHex, stakingTxHash}

	// slashing signatures
	var slashingSigStrList []string
	for _, sig := range slashingSigs {
		slashingSigStrList = append(slashingSigStrList, hex.EncodeToString(sig))
	}
	slashingSigStr := strings.Join(slashingSigStrList, ",")
	cmd = append(cmd, slashingSigStr)

	// on-demand unbonding stuff
	cmd = append(cmd, unbondingSig.ToHexStr())
	var unbondingSlashingSigStrList []string
	for _, sig := range unbondingSlashingSigs {
		unbondingSlashingSigStrList = append(unbondingSlashingSigStrList, hex.EncodeToString(sig))
	}
	unbondingSlashingSigStr := strings.Join(unbondingSlashingSigStrList, ",")
	cmd = append(cmd, unbondingSlashingSigStr)

	if stakeExpTxSig != nil {
		cmd = append(cmd, stakeExpTxSig.ToHexStr())
	}

	// used key
	cmd = append(cmd, fmt.Sprintf("--from=%s", fromWalletName))
	// gas
	cmd = append(cmd, "--gas-adjustment=2")

	outBuf, _, err := n.containerManager.ExecTxCmd(n.t, n.chainId, n.Name, cmd)
	require.NoError(n.t, err)
	n.LogActionF("successfully added covenant signatures")

	return GetTxHashFromOutput(outBuf.String())
}

func (n *NodeConfig) CommitPubRandList(fpBTCPK *bbn.BIP340PubKey, startHeight uint64, numPubrand uint64, commitment []byte, sig *bbn.BIP340Signature) {
	n.LogActionF("committing public randomness list")

	cmd := []string{"babylond", "tx", "finality", "commit-pubrand-list"}

	// add finality provider BTC PK to cmd
	fpBTCPKHex := fpBTCPK.MarshalHex()
	cmd = append(cmd, fpBTCPKHex)

	// add start height to cmd
	startHeightStr := strconv.FormatUint(startHeight, 10)
	cmd = append(cmd, startHeightStr)

	// add num_pub_rand to cmd
	numPubRandStr := strconv.FormatUint(numPubrand, 10)
	cmd = append(cmd, numPubRandStr)

	// add commitment to cmd
	commitmentHex := hex.EncodeToString(commitment)
	cmd = append(cmd, commitmentHex)

	// add sig to cmd
	sigHex := sig.ToHexStr()
	cmd = append(cmd, sigHex)

	// specify used key
	cmd = append(cmd, "--from=val")

	// gas
	cmd = append(cmd, "--gas=500000")

	_, _, err := n.containerManager.ExecTxCmd(n.t, n.chainId, n.Name, cmd)
	require.NoError(n.t, err)
	n.LogActionF("successfully committed public randomness list")
}

func (n *NodeConfig) AddFinalitySig(
	fpBTCPK *bbn.BIP340PubKey,
	blockHeight uint64,
	pubRand *bbn.SchnorrPubRand,
	proof cmtcrypto.Proof,
	appHash []byte,
	finalitySig *bbn.SchnorrEOTSSig,
	overallFlags ...string,
) {
	n.LogActionF("add finality signature")

	fpBTCPKHex := fpBTCPK.MarshalHex()
	blockHeightStr := strconv.FormatUint(blockHeight, 10)
	pubRandHex := pubRand.MarshalHex()
	proofBytes, err := proof.Marshal()
	require.NoError(n.t, err)
	proofHex := hex.EncodeToString(proofBytes)
	appHashHex := hex.EncodeToString(appHash)
	finalitySigHex := finalitySig.ToHexStr()

	cmd := []string{"babylond", "tx", "finality", "add-finality-sig", fpBTCPKHex, blockHeightStr, pubRandHex, proofHex, appHashHex, finalitySigHex, "--gas=500000"}
	additionalArgs := []string{fmt.Sprintf("--chain-id=%s", n.chainId), "--gas-prices=0.1ubbn", "-b=sync", "--yes", "--keyring-backend=test", "--log_format=json", "--home=/home/babylon/babylondata"}
	cmd = append(cmd, additionalArgs...)

	outBuff, _, err := n.containerManager.ExecCmd(n.t, n.Name, append(cmd, overallFlags...), "code: 0")
	require.NoError(n.t, err)
	n.LogActionF("successfully added finality signature: %s", outBuff.String())
}

func (n *NodeConfig) AddFinalitySigFromVal(
	fpBTCPK *bbn.BIP340PubKey,
	blockHeight uint64,
	pubRand *bbn.SchnorrPubRand,
	proof cmtcrypto.Proof,
	appHash []byte,
	finalitySig *bbn.SchnorrEOTSSig,
	overallFlags ...string,
) {
	n.AddFinalitySig(fpBTCPK, blockHeight, pubRand, proof, appHash, finalitySig, append(overallFlags, "--from=val")...)
}

func (n *NodeConfig) AddCovenantUnbondingSigs(
	covPK *bbn.BIP340PubKey,
	stakingTxHash string,
	unbondingTxSig *bbn.BIP340Signature,
	slashUnbondingTxSigs []*asig.AdaptorSignature) {
	n.LogActionF("adding finality provider signature")

	covPKHex := covPK.MarshalHex()
	unbondingTxSigHex := unbondingTxSig.ToHexStr()

	cmd := []string{"babylond", "tx", "btcstaking", "add-covenant-unbonding-sigs", covPKHex, stakingTxHash, unbondingTxSigHex}
	for _, sig := range slashUnbondingTxSigs {
		cmd = append(cmd, sig.MarshalHex())
	}
	cmd = append(cmd, "--from=val")
	_, _, err := n.containerManager.ExecTxCmd(n.t, n.chainId, n.Name, cmd)
	require.NoError(n.t, err)
	n.LogActionF("successfully added covenant unbonding sigs")
}

func (n *NodeConfig) BTCUndelegate(
	stakingTxHash *chainhash.Hash,
	spendStakeTx *wire.MsgTx,
	spendStakeTxInclusionProof *bstypes.InclusionProof,
	fundingTxs []*wire.MsgTx,
) string {
	n.LogActionF("undelegate by using signature on unbonding tx from delegator")

	spendStakeTxBytes, err := bbn.SerializeBTCTx(spendStakeTx)
	require.NoError(n.t, err)
	spendStakeTxHex := hex.EncodeToString(spendStakeTxBytes)
	inclusionProofHex, err := spendStakeTxInclusionProof.MarshalHex()
	require.NoError(n.t, err)

	fundingTxsHex := make([]string, len(fundingTxs))
	for i, tx := range fundingTxs {
		fundingTxBytes, err := bbn.SerializeBTCTx(tx)
		require.NoError(n.t, err)
		fundingTxsHex[i] = hex.EncodeToString(fundingTxBytes)
	}
	fundingTxsHexStr := strings.Join(fundingTxsHex, ",")

	cmd := []string{"babylond", "tx", "btcstaking", "btc-undelegate", stakingTxHash.String(), spendStakeTxHex, inclusionProofHex, fundingTxsHexStr, "--from=val", "--gas=500000"}

	outBuf, _, err := n.containerManager.ExecTxCmd(n.t, n.chainId, n.Name, cmd)
	require.NoError(n.t, err)
	n.LogActionF("successfully added signature on unbonding tx from delegator")
	return GetTxHashFromOutput(outBuf.String())
}

func (n *NodeConfig) AddBTCDelegationInclusionProof(
	stakingTxHash *chainhash.Hash,
	inclusionProof *bstypes.InclusionProof) {
	n.LogActionF("activate delegation by adding inclusion proof")
	inclusionProofHex, err := inclusionProof.MarshalHex()
	require.NoError(n.t, err)

	cmd := []string{"babylond", "tx", "btcstaking", "add-btc-inclusion-proof", stakingTxHash.String(), inclusionProofHex, "--from=val"}
	_, _, err = n.containerManager.ExecTxCmd(n.t, n.chainId, n.Name, cmd)
	require.NoError(n.t, err)
	n.LogActionF("successfully added inclusion proof")
}

// BTCStakingUnbondSlashInfo generate BTC information to create BTC delegation.
func (n *NodeConfig) BTCStakingUnbondSlashInfo(
	r *rand.Rand,
	t testing.TB,
	btcNet *chaincfg.Params,
	params *bstypes.Params,
	fp *bstypes.FinalityProvider,
	btcStakerSK *btcec.PrivateKey,
	stakingTimeBlocks uint16,
	stakingSatAmt int64,
) (
	testStakingInfo *datagen.TestStakingSlashingInfo,
	stakingTx []byte,
	txInclusionProof *bstypes.InclusionProof,
	testUnbondingInfo *datagen.TestUnbondingSlashingInfo,
	delegatorSig *bbn.BIP340Signature,
) {
	covenantBTCPKs := CovenantBTCPKs(params)
	// required unbonding time
	unbondingTime := params.UnbondingTimeBlocks

	testStakingInfo = datagen.GenBTCStakingSlashingInfo(
		r,
		t,
		btcNet,
		btcStakerSK,
		[]*btcec.PublicKey{fp.BtcPk.MustToBTCPK()},
		covenantBTCPKs,
		params.CovenantQuorum,
		stakingTimeBlocks,
		stakingSatAmt,
		params.SlashingPkScript,
		params.SlashingRate,
		uint16(unbondingTime),
	)

	// submit staking tx to Bitcoin and get inclusion proof
	currentBtcTipResp, err := n.QueryTip()
	require.NoError(t, err)
	currentBtcTip, err := ParseBTCHeaderInfoResponseToInfo(currentBtcTipResp)
	require.NoError(t, err)

	stakingMsgTx := testStakingInfo.StakingTx

	blockWithStakingTx := datagen.CreateBlockWithTransaction(r, currentBtcTip.Header.ToBlockHeader(), stakingMsgTx)
	n.InsertHeader(&blockWithStakingTx.HeaderBytes)
	// make block k-deep
	for i := 0; i < initialization.BabylonBtcConfirmationPeriod; i++ {
		n.InsertNewEmptyBtcHeader(r)
	}
	inclusionProof := bstypes.NewInclusionProofFromSpvProof(blockWithStakingTx.SpvProof)

	// generate BTC undelegation stuff
	stkTxHash := testStakingInfo.StakingTx.TxHash()
	unbondingValue := stakingSatAmt - datagen.UnbondingTxFee
	testUnbondingInfo = datagen.GenBTCUnbondingSlashingInfo(
		r,
		t,
		btcNet,
		btcStakerSK,
		[]*btcec.PublicKey{fp.BtcPk.MustToBTCPK()},
		covenantBTCPKs,
		params.CovenantQuorum,
		wire.NewOutPoint(&stkTxHash, datagen.StakingOutIdx),
		stakingTimeBlocks,
		unbondingValue,
		params.SlashingPkScript,
		params.SlashingRate,
		uint16(unbondingTime),
	)

	stakingSlashingPathInfo, err := testStakingInfo.StakingInfo.SlashingPathSpendInfo()
	require.NoError(t, err)

	delegatorSig, err = testStakingInfo.SlashingTx.Sign(
		stakingMsgTx,
		datagen.StakingOutIdx,
		stakingSlashingPathInfo.GetPkScriptPath(),
		btcStakerSK,
	)
	require.NoError(t, err)

	return testStakingInfo, blockWithStakingTx.SpvProof.BtcTransaction, inclusionProof, testUnbondingInfo, delegatorSig
}

func (n *NodeConfig) CreateBTCDelegationAndCheck(
	r *rand.Rand,
	t testing.TB,
	btcNet *chaincfg.Params,
	walletNameSender string,
	fp *bstypes.FinalityProvider,
	btcStakerSK *btcec.PrivateKey,
	delAddr string,
	stakingTimeBlocks uint16,
	stakingSatAmt int64,
) (testStakingInfo *datagen.TestStakingSlashingInfo) {
	testStakingInfo = n.CreateBTCDel(r, t, btcNet, walletNameSender, fp, btcStakerSK, delAddr, stakingTimeBlocks, stakingSatAmt)

	// wait for a block so that above txs take effect
	n.WaitForNextBlock()

	// check if the address matches
	btcDelegationResp := n.QueryBtcDelegation(testStakingInfo.StakingTx.TxHash().String())
	require.NotNil(t, btcDelegationResp)
	require.Equal(t, btcDelegationResp.BtcDelegation.StakerAddr, delAddr)
	require.Equal(t, btcStakerSK.PubKey().SerializeCompressed()[1:], btcDelegationResp.BtcDelegation.BtcPk.MustToBTCPK().SerializeCompressed()[1:])

	return testStakingInfo
}

func (n *NodeConfig) CreateBTCDel(
	r *rand.Rand,
	t testing.TB,
	btcNet *chaincfg.Params,
	walletNameSender string,
	fp *bstypes.FinalityProvider,
	btcStakerSK *btcec.PrivateKey,
	delAddr string,
	stakingTimeBlocks uint16,
	stakingSatAmt int64,
) (testStakingInfo *datagen.TestStakingSlashingInfo) {
	// BTC staking params, BTC delegation key pairs and PoP
	params := n.QueryBTCStakingParams()

	// NOTE: we use the node's address for the BTC delegation
	del1Addr := sdk.MustAccAddressFromBech32(delAddr)
	popDel1, err := datagen.NewPoPBTC(del1Addr, btcStakerSK)
	require.NoError(t, err)

	testStakingInfo, stakingTx, inclusionProof, testUnbondingInfo, delegatorSig := n.BTCStakingUnbondSlashInfo(r, t, btcNet, params, fp, btcStakerSK, stakingTimeBlocks, stakingSatAmt)

	delUnbondingSlashingSig, err := testUnbondingInfo.GenDelSlashingTxSig(btcStakerSK)
	require.NoError(t, err)

	// submit the message for creating BTC delegation
	n.CreateBTCDelegation(
		bbn.NewBIP340PubKeyFromBTCPK(btcStakerSK.PubKey()),
		popDel1,
		stakingTx,
		inclusionProof,
		fp.BtcPk,
		stakingTimeBlocks,
		btcutil.Amount(stakingSatAmt),
		testStakingInfo.SlashingTx,
		delegatorSig,
		testUnbondingInfo.UnbondingTx,
		testUnbondingInfo.SlashingTx,
		uint16(params.UnbondingTimeBlocks),
		btcutil.Amount(testUnbondingInfo.UnbondingInfo.UnbondingOutput.Value),
		delUnbondingSlashingSig,
		walletNameSender,
		false,
	)

	return testStakingInfo
}

func (n *NodeConfig) CreateBTCStakeExpDelegationMultipleFPsAndCheck(
	r *rand.Rand,
	t *testing.T,
	btcNet *chaincfg.Params,
	walletNameSender string,
	fps []*bstypes.FinalityProvider,
	btcStakerSK *btcec.PrivateKey,
	delAddr string,
	stakingTimeBlocks uint16,
	stakingSatAmt int64,
	prevDel *bstypes.BTCDelegation,
) (*datagen.TestStakingSlashingInfo, *wire.MsgTx) {
	// Convert finality provider BTC PKs to BIP340 PKs
	fpPKs := make([]*bbn.BIP340PubKey, len(fps))
	for i, fp := range fps {
		fpPKs[i] = fp.BtcPk
	}

	msg, testStakingInfo := n.createBtcStakeExpandMessage(
		r,
		t,
		btcNet,
		btcStakerSK,
		fps,
		stakingSatAmt,
		stakingTimeBlocks,
		prevDel,
	)

	// submit the message for creating BTC delegation
	n.CreateBTCStakeExpansionDelegation(msg, walletNameSender, false)

	// wait for a block so that above txs take effect
	n.WaitForNextBlock()

	// check if the address matches
	btcDelegationResp := n.QueryBtcDelegation(testStakingInfo.StakingTx.TxHash().String())
	require.NotNil(t, btcDelegationResp)
	require.Equal(t, btcDelegationResp.BtcDelegation.StakerAddr, delAddr)
	require.Equal(t, btcStakerSK.PubKey().SerializeCompressed()[1:], btcDelegationResp.BtcDelegation.BtcPk.MustToBTCPK().SerializeCompressed()[1:])

	fundingTx, err := bbn.NewBTCTxFromBytes(msg.FundingTx)
	require.NoError(t, err)
	return testStakingInfo, fundingTx
}

func (n *NodeConfig) AddFinalitySignatureToBlock(
	fpBTCSK *secp256k1.PrivateKey,
	fpBTCPK *bbn.BIP340PubKey,
	blockHeight uint64,
	privateRand *secp256k1.ModNScalar,
	pubRand *bbn.SchnorrPubRand,
	proof cmtcrypto.Proof,
	overallFlags ...string,
) (blockVotedAppHash bytes.HexBytes) {
	blockToVote, err := n.QueryBlock(int64(blockHeight))
	require.NoError(n.t, err)
	appHash := blockToVote.AppHash

	msgToSign := append(sdk.Uint64ToBigEndian(blockHeight), appHash...)
	// generate EOTS signature
	fp1Sig, err := eots.Sign(fpBTCSK, privateRand, msgToSign)
	require.NoError(n.t, err)

	finalitySig := bbn.NewSchnorrEOTSSigFromModNScalar(fp1Sig)

	// submit finality signature
	n.AddFinalitySig(
		fpBTCPK,
		blockHeight,
		pubRand,
		proof,
		appHash,
		finalitySig,
		overallFlags...,
	)
	return appHash
}

// CovenantBTCPKs returns the covenantBTCPks as slice from parameters
func CovenantBTCPKs(params *bstypes.Params) []*btcec.PublicKey {
	// get covenant BTC PKs
	covenantBTCPKs := make([]*btcec.PublicKey, len(params.CovenantPks))
	for i, covenantPK := range params.CovenantPks {
		covenantBTCPKs[i] = covenantPK.MustToBTCPK()
	}
	return covenantBTCPKs
}

func (n *NodeConfig) createBtcStakeExpandMessage(
	r *rand.Rand,
	t *testing.T,
	btcNet *chaincfg.Params,
	delSK *btcec.PrivateKey,
	fps []*bstypes.FinalityProvider,
	stakingValue int64,
	stakingTime uint16,
	prevDel *bstypes.BTCDelegation,
) (*bstypes.MsgBtcStakeExpand, *datagen.TestStakingSlashingInfo) {
	// BTC staking params, BTC delegation key pairs and PoP
	params := n.QueryBTCStakingParams()

	// get fpPKs in BIP340PubKey and BIP340 formats
	var fpBtcPkList []bbn.BIP340PubKey
	for _, fp := range fps {
		fpBtcPkList = append(fpBtcPkList, *fp.BtcPk)
	}
	fpPKs, err := bbn.NewBTCPKsFromBIP340PKs(fpBtcPkList)
	require.NoError(t, err)

	// Convert covenant keys
	var covenantPks []*btcec.PublicKey
	for _, pk := range params.CovenantPks {
		covenantPks = append(covenantPks, pk.MustToBTCPK())
	}

	// Create funding transaction
	fundingTx := datagen.GenRandomTxWithOutputValue(r, 10000000)

	// Convert previousStakingTxHash to OutPoint
	prevDelTxHash := prevDel.MustGetStakingTxHash()
	prevStakingOutPoint := wire.NewOutPoint(&prevDelTxHash, datagen.StakingOutIdx)

	// Convert fundingTxHash to OutPoint
	fundingTxHash := fundingTx.TxHash()
	fundingOutPoint := wire.NewOutPoint(&fundingTxHash, 0)
	outPoints := []*wire.OutPoint{prevStakingOutPoint, fundingOutPoint}

	// Generate staking slashing info using multiple inputs
	stakingSlashingInfo := datagen.GenBTCStakingSlashingInfoWithInputs(
		r,
		t,
		btcNet,
		outPoints,
		delSK,
		fpPKs,
		covenantPks,
		params.CovenantQuorum,
		stakingTime,
		stakingValue,
		params.SlashingPkScript,
		params.SlashingRate,
		uint16(params.UnbondingTimeBlocks),
	)

	slashingPathSpendInfo, err := stakingSlashingInfo.StakingInfo.SlashingPathSpendInfo()
	require.NoError(t, err)

	// Sign the slashing tx with delegator key
	delegatorSig, err := stakingSlashingInfo.SlashingTx.Sign(
		stakingSlashingInfo.StakingTx,
		datagen.StakingOutIdx,
		slashingPathSpendInfo.GetPkScriptPath(),
		delSK,
	)
	require.NoError(t, err)

	// Serialize the staking tx bytes
	serializedStakingTx, err := bbn.SerializeBTCTx(stakingSlashingInfo.StakingTx)
	require.NoError(t, err)

	stkTxHash := stakingSlashingInfo.StakingTx.TxHash()
	unbondingValue := uint64(stakingValue) - uint64(params.UnbondingFeeSat)

	// Generate unbonding slashing info
	unbondingSlashingInfo := datagen.GenBTCUnbondingSlashingInfo(
		r,
		t,
		btcNet,
		delSK,
		fpPKs,
		covenantPks,
		params.CovenantQuorum,
		wire.NewOutPoint(&stkTxHash, datagen.StakingOutIdx),
		uint16(params.UnbondingTimeBlocks),
		int64(unbondingValue),
		params.SlashingPkScript,
		params.SlashingRate,
		uint16(params.UnbondingTimeBlocks),
	)

	unbondingTxBytes, err := bbn.SerializeBTCTx(unbondingSlashingInfo.UnbondingTx)
	require.NoError(t, err)

	delSlashingTxSig, err := unbondingSlashingInfo.GenDelSlashingTxSig(delSK)
	require.NoError(t, err)

	// Create proof of possession
	stakerAddr := sdk.MustAccAddressFromBech32(prevDel.StakerAddr)
	pop, err := datagen.NewPoPBTC(stakerAddr, delSK)
	require.NoError(t, err)

	fundingTxBz, err := bbn.SerializeBTCTx(fundingTx)
	require.NoError(t, err)

	return &types.MsgBtcStakeExpand{
		StakerAddr:                    prevDel.StakerAddr,
		Pop:                           pop,
		BtcPk:                         bbn.NewBIP340PubKeyFromBTCPK(delSK.PubKey()),
		FpBtcPkList:                   fpBtcPkList,
		StakingTime:                   uint32(stakingTime),
		StakingValue:                  stakingValue,
		StakingTx:                     serializedStakingTx,
		SlashingTx:                    stakingSlashingInfo.SlashingTx,
		DelegatorSlashingSig:          delegatorSig,
		UnbondingValue:                int64(unbondingValue),
		UnbondingTime:                 params.UnbondingTimeBlocks,
		UnbondingTx:                   unbondingTxBytes,
		UnbondingSlashingTx:           unbondingSlashingInfo.SlashingTx,
		DelegatorUnbondingSlashingSig: delSlashingTxSig,
		PreviousStakingTxHash:         prevDelTxHash.String(),
		FundingTx:                     fundingTxBz,
	}, stakingSlashingInfo
}

func (n *NodeConfig) SendCovenantSigs(
	r *rand.Rand,
	t testing.TB,
	btcNet *chaincfg.Params,
	covenantSKs []*btcec.PrivateKey,
	covWallets []string,
	pendingDel *bstypes.BTCDelegation,
) []string {
	require.Len(t, pendingDel.CovenantSigs, 0)

	params := n.QueryBTCStakingParams()
	slashingTx := pendingDel.SlashingTx
	stakingTx := pendingDel.StakingTx

	stakingMsgTx, err := bbn.NewBTCTxFromBytes(stakingTx)
	require.NoError(t, err)
	stakingTxHash := stakingMsgTx.TxHash().String()

	fpBTCPKs, err := bbn.NewBTCPKsFromBIP340PKs(pendingDel.FpBtcPkList)
	require.NoError(t, err)

	stakingInfo, err := pendingDel.GetStakingInfo(params, btcNet)
	require.NoError(t, err)

	stakingSlashingPathInfo, err := stakingInfo.SlashingPathSpendInfo()
	require.NoError(t, err)

	/*
		generate and insert new covenant signature, in order to activate the BTC delegation
	*/
	// covenant signatures on slashing tx
	covenantSlashingSigs, err := datagen.GenCovenantAdaptorSigs(
		covenantSKs,
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
		covenantSKs,
		stakingMsgTx,
		pendingDel.StakingOutputIdx,
		unbondingPathInfo.GetPkScriptPath(),
		unbondingTx,
	)
	require.NoError(t, err)

	unbondingInfo, err := pendingDel.GetUnbondingInfo(params, btcNet)
	require.NoError(t, err)
	unbondingSlashingPathInfo, err := unbondingInfo.SlashingPathSpendInfo()
	require.NoError(t, err)
	covenantUnbondingSlashingSigs, err := datagen.GenCovenantAdaptorSigs(
		covenantSKs,
		fpBTCPKs,
		unbondingTx,
		unbondingSlashingPathInfo.GetPkScriptPath(),
		pendingDel.BtcUndelegation.SlashingTx,
	)
	require.NoError(t, err)

	covStkExpSigs := []*bbn.BIP340Signature{}
	if pendingDel.IsStakeExpansion() {
		prevDelTxHash, err := chainhash.NewHash(pendingDel.StkExp.PreviousStakingTxHash)
		require.NoError(t, err)
		prevDelRes := n.QueryBtcDelegation(prevDelTxHash.String())
		require.NotNil(t, prevDelRes)
		prevDel := prevDelRes.BtcDelegation
		require.NotNil(t, prevDel)
		prevParams := n.QueryBTCStakingParamsByVersion(prevDel.ParamsVersion)
		pDel, err := ParseRespBTCDelToBTCDel(prevDel)
		require.NoError(t, err)
		prevDelStakingInfo, err := pDel.GetStakingInfo(prevParams, btcNet)
		require.NoError(t, err)
		covStkExpSigs, err = datagen.GenCovenantStakeExpSig(covenantSKs, pendingDel, prevDelStakingInfo)
		require.NoError(t, err)
	}

	txHashes := make([]string, params.CovenantQuorum)
	for i := 0; i < int(params.CovenantQuorum); i++ {
		// add covenant sigs
		var stkExpSig *bbn.BIP340Signature
		if pendingDel.IsStakeExpansion() {
			stkExpSig = covStkExpSigs[i]
		}
		// add covenant sigs
		txHashes[i] = n.AddCovenantSigs(
			covWallets[i],
			covenantSlashingSigs[i].CovPk,
			stakingTxHash,
			covenantSlashingSigs[i].AdaptorSigs,
			bbn.NewBIP340SignatureFromBTCSig(covUnbondingSigs[i]),
			covenantUnbondingSlashingSigs[i].AdaptorSigs,
			stkExpSig,
		)
		n.WaitForNextBlock()
	}
	return txHashes
}

func (n *NodeConfig) SendCovenantSigsAsValAndCheck(
	r *rand.Rand,
	t testing.TB,
	btcNet *chaincfg.Params,
	covenantSKs []*btcec.PrivateKey,
	pendingDel *bstypes.BTCDelegation,
) {
	/*
		generate and insert new covenant signature, in order to activate the BTC delegation
	*/
	wallets := make([]string, len(covenantSKs))
	for i := range covenantSKs {
		wallets[i] = "val"
	}
	txHashes := n.SendCovenantSigs(
		r, t,
		btcNet,
		covenantSKs,
		wallets,
		pendingDel,
	)

	// wait for a block so that above txs take effect
	n.WaitForNextBlocks(2)
	for _, txHash := range txHashes {
		res, _ := n.QueryTx(txHash)
		require.Equal(t, res.Code, uint32(0), res.RawLog)
	}
}

func (n *NodeConfig) CreateBTCDelegationWithExpansionAndCheck(
	r *rand.Rand,
	t *testing.T,
	btcNet *chaincfg.Params,
	walletNameSender string,
	fps []*bstypes.FinalityProvider,
	btcStakerSK *btcec.PrivateKey,
	delAddr string,
	stakingTimeBlocks uint16,
	stakingSatAmt int64,
	covenantSKs []*btcec.PrivateKey,
	covenantQuorum uint32,
) (*datagen.TestStakingSlashingInfo, *datagen.TestStakingSlashingInfo, *wire.MsgTx) {
	// Step 1: we create a BTC delegation
	// NOTE: we use the node's address for the BTC delegation
	prevDelStakingInfo := n.CreateBTCDelegationAndCheck(
		r,
		t,
		btcNet,
		n.WalletName,
		fps[0],
		btcStakerSK,
		delAddr,
		stakingTimeBlocks,
		stakingSatAmt,
	)

	pendingDelSet := n.QueryFinalityProviderDelegations(fps[0].BtcPk.MarshalHex())
	require.Len(t, pendingDelSet, 1)
	pendingDels := pendingDelSet[0]
	require.Len(t, pendingDels.Dels, 1)
	require.Equal(t, btcStakerSK.PubKey().SerializeCompressed()[1:], pendingDels.Dels[0].BtcPk.MustToBTCPK().SerializeCompressed()[1:])
	require.Len(t, pendingDels.Dels[0].CovenantSigs, 0)

	// check delegation
	delegation := n.QueryBtcDelegation(prevDelStakingInfo.StakingTx.TxHash().String())
	require.NotNil(t, delegation)
	require.Equal(t, delegation.BtcDelegation.StakerAddr, n.PublicAddress)

	// Step 2: submit covenant signature to activate the BTC delegation
	originalDel, err := ParseRespBTCDelToBTCDel(pendingDels.Dels[0])
	require.NoError(t, err)
	n.SendCovenantSigsAsValAndCheck(r, t, btcNet, covenantSKs, originalDel)

	// ensure the BTC delegation has covenant sigs now
	activeDelsSet := n.QueryFinalityProviderDelegations(fps[0].BtcPk.MarshalHex())
	require.Len(t, activeDelsSet, 1)

	activeDels, err := ParseRespsBTCDelToBTCDel(activeDelsSet[0])
	require.NoError(t, err)
	require.NotNil(t, activeDels)
	require.Len(t, activeDels.Dels, 1)

	activeDel := activeDels.Dels[0]
	require.True(t, activeDel.HasCovenantQuorums(covenantQuorum, 0))

	// Step 3: create a BTC expansion delegation
	stkExpDelStakingSlashingInfo, fundingTx := n.CreateBTCStakeExpDelegationMultipleFPsAndCheck(
		r,
		t,
		btcNet,
		n.WalletName,
		fps,
		btcStakerSK,
		n.PublicAddress,
		stakingTimeBlocks,
		stakingSatAmt,
		activeDel,
	)

	// check stake expansion delegation is pending
	stkExpTxHash := stkExpDelStakingSlashingInfo.StakingTx.TxHash()
	stkExpDelegation := n.QueryBtcDelegation(stkExpTxHash.String())
	require.NotNil(t, stkExpDelegation)
	require.Equal(t, stkExpDelegation.BtcDelegation.StakerAddr, n.PublicAddress)
	require.NotNil(t, stkExpDelegation.BtcDelegation.StkExp)
	require.Equal(t, stkExpDelegation.BtcDelegation.StatusDesc, bstypes.BTCDelegationStatus_PENDING.String())

	return stkExpDelStakingSlashingInfo, prevDelStakingInfo, fundingTx
}
