package types

import (
	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	tokenfactorytypes "github.com/strangelove-ventures/tokenfactory/x/tokenfactory/types"
	"github.com/stretchr/testify/require"

	bstypes "github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
	bsctypes "github.com/babylonlabs-io/babylon/v4/x/btcstkconsumer/types"
)

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

// RegisterConsumerChain registers a new consumer chain using the specified wallet
func (n *Node) RegisterConsumerChain(walletName, consumerID, consumerName, consumerDescription, commission string) {
	wallet := n.Wallet(walletName)
	require.NotNil(n.T(), wallet, "Wallet %s not found", walletName)

	commissionDec, err := math.LegacyNewDecFromStr(commission)
	require.NoError(n.T(), err, "Invalid commission: %s", commission)

	msg := &bsctypes.MsgRegisterConsumer{
		Signer:                   wallet.Address.String(),
		ConsumerId:               consumerID,
		ConsumerName:             consumerName,
		ConsumerDescription:      consumerDescription,
		BabylonRewardsCommission: commissionDec,
	}

	_, tx := wallet.SubmitMsgs(msg)
	require.NotNil(n.T(), tx, "RegisterConsumerChain transaction should not be nil")
	n.T().Logf("Registered consumer chain: %s (%s)", consumerName, consumerID)
}

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
		BsnId:       fp.BsnId,
	}

	_, tx := wallet.SubmitMsgs(msg)
	require.NotNil(n.T(), tx, "CreateFinalityProvider transaction should not be nil")
	n.T().Logf("Created finality provider for %s: %s", fp.BsnId, fp.BtcPk.MarshalHex())
}
