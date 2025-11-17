package tmanager

import (
	"time"

	"github.com/stretchr/testify/require"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	transfertypes "github.com/cosmos/ibc-go/v10/modules/apps/transfer/types"
	clienttypes "github.com/cosmos/ibc-go/v10/modules/core/02-client/types"
	tokenfactorytypes "github.com/strangelove-ventures/tokenfactory/x/tokenfactory/types"

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
