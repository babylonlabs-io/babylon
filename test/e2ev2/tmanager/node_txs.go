package tmanager

import (
	"time"

	"cosmossdk.io/math"
	appparams "github.com/babylonlabs-io/babylon/v4/app/params"
	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	tkeeper "github.com/babylonlabs-io/babylon/v4/testutil/keeper"
	bbn "github.com/babylonlabs-io/babylon/v4/types"
	epochingtypes "github.com/babylonlabs-io/babylon/v4/x/epoching/types"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/wire"
	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	stktypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	transfertypes "github.com/cosmos/ibc-go/v10/modules/apps/transfer/types"
	clienttypes "github.com/cosmos/ibc-go/v10/modules/core/02-client/types"
	tokenfactorytypes "github.com/strangelove-ventures/tokenfactory/x/tokenfactory/types"
	"github.com/stretchr/testify/require"

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
func (n *Node) SendCoins(receiverAddrStr string, coins sdk.Coins) {
	recipientAddr, err := sdk.AccAddressFromBech32(receiverAddrStr)
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

func (n *Node) WrappedDelegate(walletName string, valAddr sdk.ValAddress, amt math.Int) {
	wallet := n.Wallet(walletName)
	require.NotNil(n.T(), wallet, "Wallet %s not found", walletName)

	amtDelegate := sdk.NewCoin(appparams.DefaultBondDenom, amt)
	msg := epochingtypes.NewMsgWrappedDelegate(stktypes.NewMsgDelegate(wallet.Addr(), valAddr.String(), amtDelegate))

	_, tx := wallet.SubmitMsgs(msg)
	require.NotNil(n.T(), tx, "Delegate tx should not be nil")
	n.T().Logf("delegation created %+v", msg)
}

func (n *Node) WrappedUndelegate(walletName string, valAddr sdk.ValAddress, amt math.Int) {
	wallet := n.Wallet(walletName)
	require.NotNil(n.T(), wallet, "Wallet %s not found", walletName)

	amtDelegate := sdk.NewCoin(appparams.DefaultBondDenom, amt)
	msg := epochingtypes.NewMsgWrappedUndelegate(stktypes.NewMsgUndelegate(wallet.Addr(), valAddr.String(), amtDelegate))

	_, tx := wallet.SubmitMsgs(msg)
	require.NotNil(n.T(), tx, "Undelegate tx should not be nil")
	n.T().Logf("undelegation created %+v", msg)
}

func (n *Node) WrappedCreateValidator(walletName string, addr sdk.AccAddress) {
	wallet := n.Wallet(walletName)
	require.NotNil(n.T(), wallet, "Wallet %s not found", walletName)

	stkParams := n.QueryStakingParams()

	wcvMsg, err := datagen.BuildMsgWrappedCreateValidator(addr)
	require.NoError(n.T(), err)

	wcvMsg.MsgCreateValidator.Commission = stktypes.NewCommissionRates(
		stkParams.MinCommissionRate,
		stkParams.MinCommissionRate.Add(math.LegacyNewDecWithPrec(1, 2)),
		math.LegacyNewDecWithPrec(1, 3),
	)

	_, tx := wallet.SubmitMsgs(wcvMsg)
	require.NotNil(n.T(), tx, "Wrapped create validator msg should not be nil")
	n.T().Logf("new validator created %+v", wcvMsg)
}

// BuildSingleSigDelegationMsg constructs a original single-sig BTC delegation message
func (n *Node) BuildSingleSigDelegationMsg(
	wallet *WalletSender,
	stakerSK *btcec.PrivateKey,
	fpPK *btcec.PublicKey,
	stakingValue int64,
	stakingTime uint16,
) (*bstypes.MsgCreateBTCDelegation, *datagen.TestStakingSlashingInfo) {
	params := n.QueryBtcStakingParams()
	net := &chaincfg.SimNetParams

	covPKs, err := bbn.NewBTCPKsFromBIP340PKs(params.CovenantPks)
	require.NoError(n.T(), err)

	// generate staking + slashing info
	stakingInfo := datagen.GenBTCStakingSlashingInfo(
		n.Tm.R, n.T(), net,
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
		n.Tm.R, n.T(), net,
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
	require.NoError(n.T(), err)

	delegatorSig, err := stakingInfo.SlashingTx.Sign(
		stakingInfo.StakingTx, 0,
		slashingSpendInfo.GetPkScriptPath(),
		stakerSK,
	)
	require.NoError(n.T(), err)

	// sign unbonding slashing tx
	delUnbondingSig, err := unbondingInfo.GenDelSlashingTxSig(stakerSK)
	require.NoError(n.T(), err)

	// generate PoP
	pop, err := datagen.NewPoPBTC(wallet.Address, stakerSK)
	require.NoError(n.T(), err)

	// serialize transactions
	serializedStakingTx, err := bbn.SerializeBTCTx(stakingInfo.StakingTx)
	require.NoError(n.T(), err)
	serializedUnbondingTx, err := bbn.SerializeBTCTx(unbondingInfo.UnbondingTx)
	require.NoError(n.T(), err)

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
	}, stakingInfo
}

func (n *Node) CreateBtcDelegation(wallet *WalletSender, fpPK *btcec.PublicKey) {
	wallet.VerifySentTx = true

	// single-sig delegation from n to fp
	stakerSK, _, err := datagen.GenRandomBTCKeyPair(n.Tm.R)
	require.NoError(n.T(), err)

	msg, stakingInfoBuilt := n.BuildSingleSigDelegationMsg(
		wallet,
		stakerSK,
		fpPK,
		int64(2*10e8),
		1000,
	)

	n.CreateBTCDelegation(wallet.KeyName, msg)
	n.WaitForNextBlock()

	pendingDelResp := n.QueryBTCDelegation(stakingInfoBuilt.StakingTx.TxHash().String())
	require.NotNil(n.T(), pendingDelResp)
	require.Equal(n.T(), "PENDING", pendingDelResp.StatusDesc)

	/*
		generate and insert new covenant signatures, in order to verify the BTC delegation
	*/
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
	require.NoError(n.T(), err)

	// cov Schnorr sigs on unbonding signature
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

	/*
		generate and add inclusion proof, in order to activate the BTC delegation
	*/
	// wait for btc delegation is k-deep
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

	// add btc inclusion proof
	n.SubmitRefundableTxWithAssertion(func() {
		n.AddBTCDelegationInclusionProof(wallet.KeyName, stakingTxHash, inclusionProof)
	}, true, wallet.KeyName)

	activeBtcDelResp := n.QueryBTCDelegation(stakingTxHash)
	require.Equal(n.T(), "ACTIVE", activeBtcDelResp.StatusDesc)
}
