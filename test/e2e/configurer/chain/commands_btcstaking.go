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

	"github.com/babylonlabs-io/babylon/crypto/eots"
	asig "github.com/babylonlabs-io/babylon/crypto/schnorr-adaptor-signature"
	"github.com/babylonlabs-io/babylon/test/e2e/containers"
	"github.com/babylonlabs-io/babylon/test/e2e/initialization"
	"github.com/babylonlabs-io/babylon/testutil/datagen"
	bbn "github.com/babylonlabs-io/babylon/types"
	bstypes "github.com/babylonlabs-io/babylon/x/btcstaking/types"
)

func (n *NodeConfig) CreateFinalityProvider(walletAddrOrName string, btcPK *bbn.BIP340PubKey, pop *bstypes.ProofOfPossessionBTC, moniker, identity, website, securityContract, details string, commission *sdkmath.LegacyDec) {
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
	}
	_, _, err = n.containerManager.ExecTxCmd(n.t, n.chainId, n.Name, cmd)
	require.NoError(n.t, err)
	n.LogActionF("successfully created finality provider")
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
	cmd = append(cmd, "--gas-prices=0.002ubbn")

	if generateOnly {
		cmd = append(cmd, "--generate-only")
	} else {
		// gas
		cmd = append(cmd, "--gas=auto", "--gas-adjustment=1.3")
		// broadcast stuff
		cmd = append(cmd, "-b=sync", "--yes")
	}

	cmd = append(cmd, fmt.Sprintf("--chain-id=%s", n.chainId), "-b=sync", "--yes")
	outBuff, _, err := n.containerManager.ExecCmd(n.t, n.Name, append(cmd, overallFlags...), "")

	require.NoError(n.t, err)
	n.LogActionF("successfully created BTC delegation")
	return outBuff.String()
}

func (n *NodeConfig) AddCovenantSigsFromVal(covPK *bbn.BIP340PubKey, stakingTxHash string, slashingSigs [][]byte, unbondingSig *bbn.BIP340Signature, unbondingSlashingSigs [][]byte) {
	n.AddCovenantSigs("val", covPK, stakingTxHash, slashingSigs, unbondingSig, unbondingSlashingSigs)
}

func (n *NodeConfig) AddCovenantSigs(
	fromWalletName string,
	covPK *bbn.BIP340PubKey,
	stakingTxHash string,
	slashingSigs [][]byte,
	unbondingSig *bbn.BIP340Signature,
	unbondingSlashingSigs [][]byte,
) {
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

	// used key
	cmd = append(cmd, fmt.Sprintf("--from=%s", fromWalletName))
	// gas
	cmd = append(cmd, "--gas=auto", "--gas-adjustment=2")

	_, _, err := n.containerManager.ExecTxCmd(n.t, n.chainId, n.Name, cmd)
	require.NoError(n.t, err)
	n.LogActionF("successfully added covenant signatures")
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
	additionalArgs := []string{fmt.Sprintf("--chain-id=%s", n.chainId), "--gas-prices=0.002ubbn", "-b=sync", "--yes", "--keyring-backend=test", "--log_format=json", "--home=/home/babylon/babylondata"}
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
) {
	n.LogActionF("undelegate by using signature on unbonding tx from delegator")

	spendStakeTxBytes, err := bbn.SerializeBTCTx(spendStakeTx)
	require.NoError(n.t, err)
	spendStakeTxHex := hex.EncodeToString(spendStakeTxBytes)
	inclusionProofHex, err := spendStakeTxInclusionProof.MarshalHex()
	require.NoError(n.t, err)

	cmd := []string{"babylond", "tx", "btcstaking", "btc-undelegate", stakingTxHash.String(), spendStakeTxHex, inclusionProofHex, "--from=val"}

	_, _, err = n.containerManager.ExecTxCmd(n.t, n.chainId, n.Name, cmd)
	require.NoError(n.t, err)
	n.LogActionF("successfully added signature on unbonding tx from delegator")
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
	// BTC staking params, BTC delegation key pairs and PoP
	params := n.QueryBTCStakingParams()

	// NOTE: we use the node's address for the BTC delegation
	del1Addr := sdk.MustAccAddressFromBech32(delAddr)
	popDel1, err := bstypes.NewPoPBTC(del1Addr, btcStakerSK)
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
		bbn.NewBIP340PKsFromBTCPKs([]*btcec.PublicKey{fp.BtcPk.MustToBTCPK()}),
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
