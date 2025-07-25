package chain

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"testing"

	appparams "github.com/babylonlabs-io/babylon/v3/app/params"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/stretchr/testify/require"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"

	sdkmath "cosmossdk.io/math"
	cometbytes "github.com/cometbft/cometbft/libs/bytes"
	cmtcrypto "github.com/cometbft/cometbft/proto/tendermint/crypto"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/babylonlabs-io/babylon/v3/app/signingcontext"
	"github.com/babylonlabs-io/babylon/v3/crypto/eots"
	asig "github.com/babylonlabs-io/babylon/v3/crypto/schnorr-adaptor-signature"
	"github.com/babylonlabs-io/babylon/v3/test/e2e/containers"
	"github.com/babylonlabs-io/babylon/v3/test/e2e/initialization"
	"github.com/babylonlabs-io/babylon/v3/testutil/datagen"
	bbn "github.com/babylonlabs-io/babylon/v3/types"
	"github.com/babylonlabs-io/babylon/v3/x/btcstaking/types"
	bstypes "github.com/babylonlabs-io/babylon/v3/x/btcstaking/types"
)

func (n *NodeConfig) CreateFinalityProvider(walletAddrOrName string, btcPK *bbn.BIP340PubKey, pop *bstypes.ProofOfPossessionBTC, moniker, identity, website, securityContract, details string, commission *sdkmath.LegacyDec, commissionMaxRate, commissionMaxRateChange sdkmath.LegacyDec) {
	n.CreateConsumerFinalityProvider(walletAddrOrName, "", btcPK, pop, moniker, identity, website, securityContract, details, commission, commissionMaxRate, commissionMaxRateChange)
}

func (n *NodeConfig) CreateBTCDelegation(
	btcPK *bbn.BIP340PubKey,
	pop *bstypes.ProofOfPossessionBTC,
	stakingTx []byte,
	inclusionProof *bstypes.InclusionProof,
	fpPKs []bbn.BIP340PubKey,
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

	btcPKHex := btcPK.MarshalHex()

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

	fpPKHexList := make([]string, len(fpPKs))
	for i, fpPK := range fpPKs {
		fpPKHexList[i] = fpPK.MarshalHex()
	}
	fpPKHexes := strings.Join(fpPKHexList, ",")

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
		btcPKHex, popHex, stakingTxHex, inclusionProofHex, fpPKHexes, stakingTimeString, stakingValueString, slashingTxHex, delegatorSigHex, unbondingTxHex, unbondingSlashingTxHex, unbondingTimeStr, unbondingValueStr, delUnbondingSlashingSigHex,
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

	cmd = append(cmd, fmt.Sprintf("--chain-id=%s", n.chainId), "-b=sync", "--yes")
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

	cmd = append(cmd, fmt.Sprintf("--chain-id=%s", n.chainId), "-b=sync", "--yes")
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
	cmd = append(cmd, "--gas=3000000")
	cmd = append(cmd, "--gas-adjustment=1.2")

	outBuf, _, err := n.containerManager.ExecTxCmd(n.t, n.chainId, n.Name, cmd)
	require.NoError(n.t, err)
	n.LogActionF("successfully added covenant signatures")

	return GetTxHashFromOutput(outBuf.String())
}

func (n *NodeConfig) AddBsnRewards(
	fromWalletName, bsnId string,
	rewards sdk.Coins,
	fpRatios []bstypes.FpRatio,
) (outBuf, errBuf bytes.Buffer, err error) {
	n.LogActionF("adding BSN rewards %s to %s from nodeName: %s", rewards.String(), bsnId, n.Name)

	cmd := []string{"babylond", "tx", "btcstaking", "add-bsn-rewards", bsnId, rewards.String()}

	var fpRatioStrList []string
	for _, fp := range fpRatios {
		fpPkHex := fp.BtcPk.MarshalHex()
		fpRatioStrList = append(fpRatioStrList, fmt.Sprintf("%s:%s", fpPkHex, fp.Ratio.String()))
	}
	fpRatiosStr := strings.Join(fpRatioStrList, ",")
	cmd = append(cmd, fpRatiosStr)

	cmd = append(cmd, fmt.Sprintf("--from=%s", fromWalletName))
	cmd = append(cmd, "--gas=3000000")
	cmd = append(cmd, "--gas-adjustment=2")

	outBuf, errBuf, err = n.containerManager.ExecTxCmdWithSuccessString(n.t, n.chainId, n.Name, cmd, "")
	require.NoError(n.t, err)
	n.LogActionF("successfully added BSN rewards")
	return outBuf, errBuf, err
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
	return n.BTCStakingUnbondSlashInfoMultipleFPs(
		r,
		t,
		btcNet,
		params,
		[]*bstypes.FinalityProvider{fp},
		btcStakerSK,
		stakingTimeBlocks,
		stakingSatAmt,
	)
}

// BTCStakingUnbondSlashInfoMultipleFPs generate BTC information to create a BTC
// delegation over multiple finality providers
func (n *NodeConfig) BTCStakingUnbondSlashInfoMultipleFPs(
	r *rand.Rand,
	t testing.TB,
	btcNet *chaincfg.Params,
	params *bstypes.Params,
	fps []*bstypes.FinalityProvider,
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

	// Convert finality provider BIP340 PKs to PublicKeys
	fpPKs := make([]*btcec.PublicKey, len(fps))
	for i, fp := range fps {
		fpPKs[i] = fp.BtcPk.MustToBTCPK()
	}

	testStakingInfo = datagen.GenBTCStakingSlashingInfo(
		r,
		t,
		btcNet,
		btcStakerSK,
		fpPKs,
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
		fpPKs,
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
	return n.CreateBTCDelegationMultipleFPsAndCheck(
		r,
		t,
		btcNet,
		walletNameSender,
		[]*bstypes.FinalityProvider{fp},
		btcStakerSK,
		delAddr,
		stakingTimeBlocks,
		stakingSatAmt,
	)
}

func (n *NodeConfig) CreateBTCDelegationMultipleFPsAndCheck(
	r *rand.Rand,
	t testing.TB,
	btcNet *chaincfg.Params,
	walletNameSender string,
	fps []*bstypes.FinalityProvider,
	btcStakerSK *btcec.PrivateKey,
	delAddr string,
	stakingTimeBlocks uint16,
	stakingSatAmt int64,
) (testStakingInfo *datagen.TestStakingSlashingInfo) {
	// BTC staking params, BTC delegation key pairs and PoP
	params := n.QueryBTCStakingParams()

	// NOTE: we use the node's address for the BTC delegation
	del1Addr := sdk.MustAccAddressFromBech32(delAddr)

	stakerPopContext := signingcontext.StakerPopContextV0(n.chainId, appparams.AccBTCStaking.String())

	popDel1, err := datagen.NewPoPBTC(stakerPopContext, del1Addr, btcStakerSK)
	require.NoError(t, err)

	testStakingInfo, stakingTx, inclusionProof, testUnbondingInfo, delegatorSig := n.BTCStakingUnbondSlashInfoMultipleFPs(r, t, btcNet, params, fps, btcStakerSK, stakingTimeBlocks, stakingSatAmt)

	delUnbondingSlashingSig, err := testUnbondingInfo.GenDelSlashingTxSig(btcStakerSK)
	require.NoError(t, err)

	// Convert finality provider BTC PKs to BIP340 PKs
	fpPKs := make([]bbn.BIP340PubKey, len(fps))
	for i, fp := range fps {
		fpPKs[i] = *fp.BtcPk
	}

	// submit the message for creating BTC delegation
	n.CreateBTCDelegation(
		bbn.NewBIP340PubKeyFromBTCPK(btcStakerSK.PubKey()),
		popDel1,
		stakingTx,
		inclusionProof,
		fpPKs,
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

	// wait for a block so that above txs take effect
	n.WaitForNextBlock()

	// check if the address matches
	btcDelegationResp := n.QueryBtcDelegation(testStakingInfo.StakingTx.TxHash().String())
	require.NotNil(t, btcDelegationResp)
	require.Equal(t, btcDelegationResp.BtcDelegation.StakerAddr, delAddr)
	require.Equal(t, btcStakerSK.PubKey().SerializeCompressed()[1:], btcDelegationResp.BtcDelegation.BtcPk.MustToBTCPK().SerializeCompressed()[1:])

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
) (blockVotedAppHash cometbytes.HexBytes) {
	blockToVote, err := n.QueryBlock(int64(blockHeight))
	require.NoError(n.t, err)
	appHash := blockToVote.AppHash

	fpFinVoteContext := signingcontext.FpFinVoteContextV0(n.chainId, appparams.AccFinality.String())

	msgToSign := []byte(fpFinVoteContext)
	msgToSign = append(msgToSign, sdk.Uint64ToBigEndian(blockHeight)...)
	msgToSign = append(msgToSign, appHash...)

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
	fundingTx := datagen.GenFundingTx(
		t,
		r,
		btcNet,
		&wire.OutPoint{Index: 0},
		stakingValue,
		prevDel.MustGetStakingTx().TxOut[prevDel.StakingOutputIdx],
	)
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
	stakerPopContext := signingcontext.StakerPopContextV0(n.chainId, appparams.AccBTCStaking.String())
	pop, err := datagen.NewPoPBTC(stakerPopContext, stakerAddr, delSK)
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
