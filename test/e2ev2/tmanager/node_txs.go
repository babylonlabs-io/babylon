package tmanager

import (
	"io"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"cosmossdk.io/math"
	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	transfertypes "github.com/cosmos/ibc-go/v10/modules/apps/transfer/types"
	clienttypes "github.com/cosmos/ibc-go/v10/modules/core/02-client/types"
	tokenfactorytypes "github.com/strangelove-ventures/tokenfactory/x/tokenfactory/types"
	"github.com/stretchr/testify/require"

	bstypes "github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
	bsctypes "github.com/babylonlabs-io/babylon/v4/x/btcstkconsumer/types"
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

// RegisterRollupConsumer registers a new rollup consumer with contract address
func (n *Node) RegisterRollupConsumer(walletName, consumerID, consumerName, consumerDescription, commission, contractAddress string) {
	wallet := n.Wallet(walletName)
	require.NotNil(n.T(), wallet, "Wallet %s not found", walletName)

	commissionDec, err := math.LegacyNewDecFromStr(commission)
	require.NoError(n.T(), err, "Invalid commission: %s", commission)

	msg := &bsctypes.MsgRegisterConsumer{
		Signer:                        wallet.Address.String(),
		ConsumerId:                    consumerID,
		ConsumerName:                  consumerName,
		ConsumerDescription:           consumerDescription,
		BabylonRewardsCommission:      commissionDec,
		RollupFinalityContractAddress: contractAddress,
	}

	_, tx := wallet.SubmitMsgs(msg)
	require.NotNil(n.T(), tx, "RegisterRollupConsumer transaction should not be nil")
}

// StoreWasmCode stores WASM bytecode on the blockchain using messages
func (n *Node) StoreWasmCode(wasmFile, walletName string) {
	wallet := n.Wallet(walletName)
	require.NotNil(n.T(), wallet, "Wallet %s not found", walletName)

	wasmCode, err := os.Open(wasmFile)
	require.NoError(n.T(), err)
	defer wasmCode.Close()

	code, err := io.ReadAll(wasmCode)
	require.NoError(n.T(), err)

	msg := &wasmtypes.MsgStoreCode{
		Sender:       wallet.Address.String(),
		WASMByteCode: code,
	}

	wallet.VerifySentTx = true
	gasLimit := uint64(6000000) // for large contracts
	txHash, tx := wallet.SubmitMsgsWithGas(gasLimit, msg)
	wallet.VerifySentTx = false

	require.NotNil(n.T(), tx, "StoreWasmCode transaction should not be nil")
}

// InstantiateWasmContract instantiates a stored WASM contract using messages
func (n *Node) InstantiateWasmContract(codeId, initMsg, walletName string) {
	wallet := n.Wallet(walletName)
	require.NotNil(n.T(), wallet, "Wallet %s not found", walletName)

	codeIDUint, err := strconv.ParseUint(codeId, 10, 64)
	require.NoError(n.T(), err, "Invalid code ID: %s", codeId)

	msg := &wasmtypes.MsgInstantiateContract{
		Sender: wallet.Address.String(),
		CodeID: codeIDUint,
		Label:  "contract",
		Msg:    []byte(initMsg),
		Funds:  sdk.NewCoins(),
	}

	wallet.VerifySentTx = true
	txHash, tx := wallet.SubmitMsgs(msg)
	wallet.VerifySentTx = false

	require.NotNil(n.T(), tx, "InstantiateWasmContract transaction should not be nil")
}

// CreateFinalityContract creates a finality contract for the given BSN ID
func (n *Node) CreateFinalityContract(bsnId string) string {
	pwd, err := os.Getwd()
	require.NoError(n.T(), err)
	finalityContractPath := filepath.Join(pwd, "bytecode", "finality.wasm")

	wasmContractId := int(n.QueryLatestWasmCodeID())

	n.StoreWasmCode(finalityContractPath, n.DefaultWallet().KeyName)

	n.WaitForNextBlock()

	require.Eventually(n.T(), func() bool {
		newLatestWasmId := int(n.QueryLatestWasmCodeID())
		if newLatestWasmId >= wasmContractId+1 {
			wasmContractId = newLatestWasmId
			return true
		}
		return false
	}, time.Second*15, time.Second*1)

	n.InstantiateWasmContract(
		strconv.Itoa(wasmContractId),
		`{
			"admin": "`+n.DefaultWallet().Address.String()+`",
			"bsn_id": "`+bsnId+`"
		}`,
		n.DefaultWallet().KeyName,
	)

	var (
		contracts []string
	)
	require.Eventually(n.T(), func() bool {
		contracts, err = n.QueryContractsFromId(wasmContractId)
		return err == nil && len(contracts) == 1
	}, time.Second*10, time.Millisecond*100)

	return contracts[0]
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
