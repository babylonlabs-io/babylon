package tmanager

import (
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/wire"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	transfertypes "github.com/cosmos/ibc-go/v10/modules/apps/transfer/types"
	clienttypes "github.com/cosmos/ibc-go/v10/modules/core/02-client/types"
	tokenfactorytypes "github.com/strangelove-ventures/tokenfactory/x/tokenfactory/types"

	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	bbn "github.com/babylonlabs-io/babylon/v4/types"
	bstypes "github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
)

// SendIBCTransfer creates and submits an IBC transfer transaction
func (n *Node) SendIBCTransfer(wallet *WalletSender, recipient string, token sdk.Coin, channelID string, memo string) string {
	n.T().Logf("Sending %s from %s (BSN) to %s (BBN) via channel %s", token.String(), wallet.Address.String(), recipient, channelID)
	timeoutHeight := clienttypes.NewHeight(0, 1000)
	timeoutTimestamp := uint64(time.Now().Add(time.Hour).UnixNano())

	// Create IBC transfer message
	msg := transfertypes.NewMsgTransfer(
		"transfer",              // source port
		channelID,               // source channel
		token,                   // token to transfer
		wallet.Address.String(), // sender
		recipient,               // receiver
		timeoutHeight,           // timeout height
		timeoutTimestamp,        // timeout timestamp
		memo,                    // memo
	)

	txHash, _ := wallet.SubmitMsgs(msg)
	return txHash
}

// SendCoins sends coins to a recipient address using the node's default wallet
func (n *Node) SendCoins(recipient string, coins sdk.Coins) {
	recipientAddr, err := sdk.AccAddressFromBech32(recipient)
	require.NoError(n.T(), err)

	msg := banktypes.NewMsgSend(n.DefaultWallet().Address, recipientAddr, coins)
	n.DefaultWallet().SubmitMsgs(msg)
}

// CreateDenom creates a new token factory denomination using the specified wallet
func (n *Node) CreateDenom(walletName, denomName string) {
	wallet := n.Wallet(walletName)
	require.NotNil(n.T(), wallet, "Wallet %s not found", walletName)

	msg := tokenfactorytypes.NewMsgCreateDenom(wallet.Address.String(), denomName)
	_, tx := wallet.SubmitMsgs(msg)
	require.NotNil(n.T(), tx, "CreateDenom transaction should not be nil")
	n.T().Logf("Created denomination: factory/%s/%s", wallet.Address.String(), denomName)
}

// MintDenom mints tokens of a custom denomination using the specified wallet
func (n *Node) MintDenom(walletName, amount, denom string) {
	wallet := n.Wallet(walletName)
	require.NotNil(n.T(), wallet, "Wallet %s not found", walletName)

	amountInt, ok := math.NewIntFromString(amount)
	require.True(n.T(), ok, "Invalid amount: %s", amount)

	coin := sdk.NewCoin(denom, amountInt)
	msg := tokenfactorytypes.NewMsgMint(wallet.Address.String(), coin)
	_, tx := wallet.SubmitMsgs(msg)
	require.NotNil(n.T(), tx, "MintDenom transaction should not be nil")
	n.T().Logf("Minted %s %s to %s", amount, denom, wallet.Address.String())
}

/*
	x/btcstaking txs
*/
// CreateFinalityProvider creates a finality provider on the given chain/consumer using the specified wallet
func (n *Node) CreateFinalityProvider(walletName string, fp *bstypes.FinalityProvider) {
	wallet := n.Wallet(walletName)
	require.NotNil(n.T(), wallet, "Wallet %s not found", walletName)

	// Create commission rates
	commission := bstypes.NewCommissionRates(
		*fp.Commission,
		fp.CommissionInfo.MaxRate,
		fp.CommissionInfo.MaxChangeRate,
	)

	msg := &bstypes.MsgCreateFinalityProvider{
		Addr:        wallet.Address.String(),
		BtcPk:       fp.BtcPk,
		Pop:         fp.Pop,
		Commission:  commission,
		Description: fp.Description,
	}

	_, tx := wallet.SubmitMsgs(msg)
	require.NotNil(n.T(), tx, "CreateFinalityProvider transaction should not be nil")
	n.T().Logf("Created finality provider: %s", fp.BtcPk.MarshalHex())
}

// CreateBTCDelegation submits a BTC delegation transaction with a specified wallet
func (n *Node) CreateBTCDelegation(walletName string, msg *bstypes.MsgCreateBTCDelegation) string {
	wallet := n.Wallet(walletName)
	require.NotNil(n.T(), wallet, "Wallet %s not found", walletName)

	txHash, tx := wallet.SubmitMsgs(msg)
	require.NotNil(n.T(), tx, "CreateBTCDelegation transaction should not be nil")
	n.T().Logf("BTC delegation created, tx hash: %s", txHash)
	return txHash
}

// AddCovenantSigs submits covenant signatures of the covenant committee with a specified wallet
func (n *Node) AddCovenantSigs(
	walletName string,
	covPK *bbn.BIP340PubKey,
	stakingTxHash string,
	slashingSigs [][]byte,
	unbondingSig *bbn.BIP340Signature,
	unbondingSlashingSigs [][]byte,
	stakeExpTxSig *bbn.BIP340Signature,
) {
	wallet := n.Wallet(walletName)
	require.NotNil(n.T(), wallet, "Wallet %s not found", walletName)

	msg := &bstypes.MsgAddCovenantSigs{
		Signer:                  wallet.Address.String(),
		Pk:                      covPK,
		StakingTxHash:           stakingTxHash,
		SlashingTxSigs:          slashingSigs,
		UnbondingTxSig:          unbondingSig,
		SlashingUnbondingTxSigs: unbondingSlashingSigs,
		StakeExpansionTxSig:     stakeExpTxSig,
	}
	_, tx := wallet.SubmitMsgs(msg)
	require.NotNil(n.T(), tx, "AddCovenantSigs transaction should not be nil")
	n.T().Logf("Covenant signatures added")
}

// AddBTCDelegationInclusionProof adds btc delegation inclusion proof with a specified wallet
func (n *Node) AddBTCDelegationInclusionProof(
	walletName string,
	stakingTxHash string,
	inclusionProof *bstypes.InclusionProof,
) {
	wallet := n.Wallet(walletName)
	require.NotNil(n.T(), wallet, "Wallet %s not found", walletName)

	msg := &bstypes.MsgAddBTCDelegationInclusionProof{
		Signer:                  wallet.Address.String(),
		StakingTxHash:           stakingTxHash,
		StakingTxInclusionProof: inclusionProof,
	}
	_, tx := wallet.SubmitMsgs(msg)
	require.NotNil(n.T(), tx, "AddBTCDelegationInclusionProof transaction should not be nil")
	n.T().Logf("BTC delegation inclusion proof added")
}

func (n *Node) BtcStakeExpand(
	walletName string,
	prevDel *bstypes.BTCDelegation,
	r *rand.Rand,
	stakerSKs []*btcec.PrivateKey,
	stakerQuorum uint32,
	fpPK *btcec.PublicKey,
	expErr error,
) (*datagen.TestStakingSlashingInfo, *wire.MsgTx) {
	wallet := n.Wallet(walletName)
	require.NotNil(n.T(), wallet, "Wallet %s not found", walletName)

	var (
		msg             *bstypes.MsgBtcStakeExpand
		testStakingInfo *datagen.TestStakingSlashingInfo
		fundingTx       *wire.MsgTx
	)

	if len(stakerSKs) == 1 {
		msg, testStakingInfo, fundingTx = n.createBtcStakeExpandMessage(
			n.T(), r,
			wallet,
			stakerSKs[0],
			fpPK,
			int64(2*10e8),
			1000,
			prevDel,
		)
	} else {
		msg, testStakingInfo, fundingTx = n.createMultisigBtcStakeExpandMessage(
			n.T(), r,
			wallet,
			stakerSKs,
			stakerQuorum,
			fpPK,
			int64(2*10e8),
			1000,
			prevDel,
		)
	}

	_, tx := wallet.SubmitMsgsWithErrContain(expErr, msg)
	require.NotNil(n.T(), tx, "BtcStakeExpand transaction should not be nil")
	n.T().Logf("BtcStakeExpand transaction submitted")

	return testStakingInfo, fundingTx
}

func (n *Node) BTCUndelegate(
	walletName string,
	stakingTxHash string,
	spendStakeTx *wire.MsgTx,
	spendStakeInclusionProof *bstypes.InclusionProof,
	fundingTxs []*wire.MsgTx,
) string {
	wallet := n.Wallet(walletName)
	require.NotNil(n.T(), wallet, "Wallet %s not found", walletName)

	spendStakeTxBz, err := bbn.SerializeBTCTx(spendStakeTx)
	require.NoError(n.T(), err)
	fundingTxsBz := make([][]byte, 0, len(fundingTxs))
	for _, tx := range fundingTxs {
		fundingTxBz, err := bbn.SerializeBTCTx(tx)
		require.NoError(n.T(), err)
		fundingTxsBz = append(fundingTxsBz, fundingTxBz)
	}

	txHash, tx := wallet.SubmitMsgs(&bstypes.MsgBTCUndelegate{
		Signer:                        wallet.Address.String(),
		StakingTxHash:                 stakingTxHash,
		StakeSpendingTx:               spendStakeTxBz,
		StakeSpendingTxInclusionProof: spendStakeInclusionProof,
		FundingTransactions:           fundingTxsBz,
	})
	require.NotNil(n.T(), tx, "BTCUndelegate transaction should not be nil")

	return txHash
}

/*
	x/gov txs
*/
// SubmitProposal submits a governance proposal with a specified wallet
func (n *Node) SubmitProposal(walletName string, govMsg *govtypes.MsgSubmitProposal) {
	wallet := n.Wallet(walletName)
	require.NotNil(n.T(), wallet, "Wallet %s not found", walletName)

	_, tx := wallet.SubmitMsgs(govMsg)
	require.NotNil(n.T(), tx, "SubmitProposal transaction should not be nil")
	n.T().Logf("Governance proposal submitted")
}

func (n *Node) Vote(walletName string, proposalID uint64, voteOption govtypes.VoteOption) {
	wallet := n.Wallet(walletName)
	require.NotNil(n.T(), wallet, "Wallet %s not found", walletName)

	govMsg := &govtypes.MsgVote{
		ProposalId: proposalID,
		Voter:      wallet.Address.String(),
		Option:     voteOption,
		Metadata:   "",
	}
	_, tx := wallet.SubmitMsgs(govMsg)
	require.NotNil(n.T(), tx, "Vote transaction should not be nil")
	n.T().Logf("Governance vote submitted")
}

/*
	helper functions
*/

// createBtcStakeExpandMessage create a btc stake expansion message and return
// MsgBtcStakeExpand, staking info, and funding tx
func (n *Node) createBtcStakeExpandMessage(
	t *testing.T,
	r *rand.Rand,
	wallet *WalletSender,
	stakerSK *btcec.PrivateKey,
	fpPK *btcec.PublicKey,
	stakingValue int64,
	stakingTime uint16,
	prevDel *bstypes.BTCDelegation,
) (*bstypes.MsgBtcStakeExpand, *datagen.TestStakingSlashingInfo, *wire.MsgTx) {
	params := n.QueryBtcStakingParams()
	net := &chaincfg.SimNetParams

	covPKs, err := bbn.NewBTCPKsFromBIP340PKs(params.CovenantPks)
	require.NoError(t, err)

	// create funding transaction
	fundingTx := datagen.GenRandomTxWithOutputValue(r, 10000000)

	// convert previousStakingTxHash to OutPoint
	prevDelTxHash := prevDel.MustGetStakingTxHash()
	prevStakingOutPoint := wire.NewOutPoint(&prevDelTxHash, datagen.StakingOutIdx)

	// convert fundingTxHash to OutPoint
	fundingTxHash := fundingTx.TxHash()
	fundingOutPoint := wire.NewOutPoint(&fundingTxHash, 0)
	outPoints := []*wire.OutPoint{prevStakingOutPoint, fundingOutPoint}

	// Generate staking slashing info using multiple inputs
	stakingSlashingInfo := datagen.GenBTCStakingSlashingInfoWithInputs(
		r,
		t,
		net,
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

	slashingPathSpendInfo, err := stakingSlashingInfo.StakingInfo.SlashingPathSpendInfo()
	require.NoError(t, err)

	// sign the slashing tx with the staker
	delegatorSig, err := stakingSlashingInfo.SlashingTx.Sign(
		stakingSlashingInfo.StakingTx, 0,
		slashingPathSpendInfo.GetPkScriptPath(),
		stakerSK,
	)
	require.NoError(t, err)

	stkTxHash := stakingSlashingInfo.StakingTx.TxHash()
	unbondingValue := uint64(stakingValue) - uint64(params.UnbondingFeeSat)

	// Generate unbonding slashing info
	unbondingSlashingInfo := datagen.GenBTCUnbondingSlashingInfo(
		r,
		t,
		net,
		stakerSK,
		[]*btcec.PublicKey{fpPK},
		covPKs,
		params.CovenantQuorum,
		wire.NewOutPoint(&stkTxHash, datagen.StakingOutIdx),
		uint16(params.UnbondingTimeBlocks),
		int64(unbondingValue),
		params.SlashingPkScript,
		params.SlashingRate,
		uint16(params.UnbondingTimeBlocks),
	)

	// sign unbonding slashing tx with staker
	delUnbondingSig, err := unbondingSlashingInfo.GenDelSlashingTxSig(stakerSK)
	require.NoError(t, err)

	// generate PoP for primary staker
	pop, err := datagen.NewPoPBTC(wallet.Address, stakerSK)
	require.NoError(t, err)

	// serialize transactions
	serializedStakingTx, err := bbn.SerializeBTCTx(stakingSlashingInfo.StakingTx)
	require.NoError(t, err)
	serializedUnbondingTx, err := bbn.SerializeBTCTx(unbondingSlashingInfo.UnbondingTx)
	require.NoError(t, err)
	fundingTxBz, err := bbn.SerializeBTCTx(fundingTx)
	require.NoError(t, err)

	return &bstypes.MsgBtcStakeExpand{
		StakerAddr:                    prevDel.StakerAddr,
		Pop:                           pop,
		BtcPk:                         bbn.NewBIP340PubKeyFromBTCPK(stakerSK.PubKey()),
		FpBtcPkList:                   []bbn.BIP340PubKey{*bbn.NewBIP340PubKeyFromBTCPK(fpPK)},
		StakingTime:                   uint32(stakingTime),
		StakingValue:                  stakingValue,
		StakingTx:                     serializedStakingTx,
		SlashingTx:                    stakingSlashingInfo.SlashingTx,
		DelegatorSlashingSig:          delegatorSig,
		UnbondingValue:                int64(unbondingValue),
		UnbondingTime:                 params.UnbondingTimeBlocks,
		UnbondingTx:                   serializedUnbondingTx,
		UnbondingSlashingTx:           unbondingSlashingInfo.SlashingTx,
		DelegatorUnbondingSlashingSig: delUnbondingSig,
		PreviousStakingTxHash:         prevDelTxHash.String(),
		FundingTx:                     fundingTxBz,
	}, stakingSlashingInfo, fundingTx
}

// createMultisigBtcStakeExpandMessage create a multisig btc stake expansion message and return
// MsgBtcStakeExpand, staking info, and funding tx
func (n *Node) createMultisigBtcStakeExpandMessage(
	t *testing.T,
	r *rand.Rand,
	wallet *WalletSender,
	stakerSKs []*btcec.PrivateKey,
	stakerQuorum uint32,
	fpPK *btcec.PublicKey,
	stakingValue int64,
	stakingTime uint16,
	prevDel *bstypes.BTCDelegation,
) (*bstypes.MsgBtcStakeExpand, *datagen.TestStakingSlashingInfo, *wire.MsgTx) {
	params := n.QueryBtcStakingParams()
	net := &chaincfg.SimNetParams

	covPKs, err := bbn.NewBTCPKsFromBIP340PKs(params.CovenantPks)
	require.NoError(t, err)

	// create funding transaction
	fundingTx := datagen.GenRandomTxWithOutputValue(r, 10000000)

	// convert previousStakingTxHash to OutPoint
	prevDelTxHash := prevDel.MustGetStakingTxHash()
	prevStakingOutPoint := wire.NewOutPoint(&prevDelTxHash, datagen.StakingOutIdx)

	// convert fundingTxHash to OutPoint
	fundingTxHash := fundingTx.TxHash()
	fundingOutPoint := wire.NewOutPoint(&fundingTxHash, 0)
	outPoints := []*wire.OutPoint{prevStakingOutPoint, fundingOutPoint}

	// Generate staking slashing info using multiple inputs
	stakingSlashingInfo := datagen.GenMultisigBTCStakingSlashingInfoWithInputs(
		r,
		t,
		net,
		outPoints,
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
		10000,
	)

	slashingPathSpendInfo, err := stakingSlashingInfo.StakingInfo.SlashingPathSpendInfo()
	require.NoError(t, err)

	// sign the slashing tx with the primary staker
	delegatorSig, err := stakingSlashingInfo.SlashingTx.Sign(
		stakingSlashingInfo.StakingTx, 0,
		slashingPathSpendInfo.GetPkScriptPath(),
		stakerSKs[0],
	)
	require.NoError(t, err)

	// generate extra staker signatures (for remaining stakers)
	var extraSlashingSigs []*bstypes.SignatureInfo
	stakerSKList := stakerSKs[1:stakerQuorum]
	for _, sk := range stakerSKList {
		sig, err := stakingSlashingInfo.SlashingTx.Sign(
			stakingSlashingInfo.StakingTx, 0,
			slashingPathSpendInfo.GetPkScriptPath(),
			sk,
		)
		require.NoError(t, err)

		extraSlashingSigs = append(extraSlashingSigs, &bstypes.SignatureInfo{
			Pk:  bbn.NewBIP340PubKeyFromBTCPK(sk.PubKey()),
			Sig: sig,
		})
	}

	stkTxHash := stakingSlashingInfo.StakingTx.TxHash()
	unbondingValue := uint64(stakingValue) - uint64(params.UnbondingFeeSat)

	// Generate unbonding slashing info
	unbondingSlashingInfo := datagen.GenMultisigBTCUnbondingSlashingInfo(
		r,
		t,
		net,
		stakerSKs,
		stakerQuorum,
		[]*btcec.PublicKey{fpPK},
		covPKs,
		params.CovenantQuorum,
		wire.NewOutPoint(&stkTxHash, datagen.StakingOutIdx),
		uint16(params.UnbondingTimeBlocks),
		int64(unbondingValue),
		params.SlashingPkScript,
		params.SlashingRate,
		uint16(params.UnbondingTimeBlocks),
	)

	// sign unbonding slashing tx with primary staker
	delUnbondingSig, err := unbondingSlashingInfo.GenDelSlashingTxSig(stakerSKs[0])
	require.NoError(t, err)

	// generate extra unbonding signatures
	var extraUnbondingSigs []*bstypes.SignatureInfo
	for _, sk := range stakerSKList {
		sig, err := unbondingSlashingInfo.GenDelSlashingTxSig(sk)
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
	serializedStakingTx, err := bbn.SerializeBTCTx(stakingSlashingInfo.StakingTx)
	require.NoError(t, err)
	serializedUnbondingTx, err := bbn.SerializeBTCTx(unbondingSlashingInfo.UnbondingTx)
	require.NoError(t, err)
	fundingTxBz, err := bbn.SerializeBTCTx(fundingTx)
	require.NoError(t, err)

	// build extra staker PK list (all stakers except the first one)
	extraStakerPKs := make([]bbn.BIP340PubKey, len(stakerSKs)-1)
	for i, sk := range stakerSKs[1:] {
		extraStakerPKs[i] = *bbn.NewBIP340PubKeyFromBTCPK(sk.PubKey())
	}

	return &bstypes.MsgBtcStakeExpand{
		StakerAddr:                    prevDel.StakerAddr,
		Pop:                           pop,
		BtcPk:                         bbn.NewBIP340PubKeyFromBTCPK(stakerSKs[0].PubKey()),
		FpBtcPkList:                   []bbn.BIP340PubKey{*bbn.NewBIP340PubKeyFromBTCPK(fpPK)},
		StakingTime:                   uint32(stakingTime),
		StakingValue:                  stakingValue,
		StakingTx:                     serializedStakingTx,
		SlashingTx:                    stakingSlashingInfo.SlashingTx,
		DelegatorSlashingSig:          delegatorSig,
		UnbondingValue:                int64(unbondingValue),
		UnbondingTime:                 params.UnbondingTimeBlocks,
		UnbondingTx:                   serializedUnbondingTx,
		UnbondingSlashingTx:           unbondingSlashingInfo.SlashingTx,
		DelegatorUnbondingSlashingSig: delUnbondingSig,
		PreviousStakingTxHash:         prevDelTxHash.String(),
		FundingTx:                     fundingTxBz,
		MultisigInfo: &bstypes.AdditionalStakerInfo{
			StakerBtcPkList:                extraStakerPKs,
			StakerQuorum:                   stakerQuorum,
			DelegatorSlashingSigs:          extraSlashingSigs,
			DelegatorUnbondingSlashingSigs: extraUnbondingSigs,
		},
	}, stakingSlashingInfo, fundingTx
}
