package tmanager

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/btcsuite/btcd/btcec/v2"

	"cosmossdk.io/math"
	cmtcfg "github.com/cometbft/cometbft/config"
	"github.com/cometbft/cometbft/crypto/ed25519"
	"github.com/cometbft/cometbft/privval"
	sdkcrypto "github.com/cosmos/cosmos-sdk/crypto"
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/cosmos/cosmos-sdk/server"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdktx "github.com/cosmos/cosmos-sdk/types/tx"
	sdksigning "github.com/cosmos/cosmos-sdk/types/tx/signing"
	authsigning "github.com/cosmos/cosmos-sdk/x/auth/signing"
	"github.com/cosmos/go-bip39"

	appparams "github.com/babylonlabs-io/babylon/v4/app/params"
	appsigner "github.com/babylonlabs-io/babylon/v4/app/signer"
	"github.com/babylonlabs-io/babylon/v4/test/e2e/util"
)

const (
	KeyringPassphrase = "testpassphrase"
	KeyringAppName    = "testnet"
	BlsPwd            = "password"
)

// WalletSender manages transaction sending for an account
type WalletSender struct {
	Node *Node

	Address sdk.AccAddress
	PrivKey cryptotypes.PrivKey

	// KeyName used in babylond keys add <KeyName>
	KeyName  string
	Mnemonic string
	// Home is the path for the home folder created locally, not inside the container
	Home string
	Info *keyring.Record

	// transaction control properties are only set after the chain is running
	SequenceNumber uint64
	AccountNumber  uint64

	// Txs Latest TxHash Sent
	Txs []string
	// VerifySentTx defines whether it should or not wait for the next chain block to verify the result of the
	// transaction
	VerifySentTx bool
}

type ValidatorWallet struct {
	*WalletSender
	ConsKey          *appsigner.ConsensusKey
	ConsensusAddress sdk.ConsAddress
	ValidatorAddress sdk.ValAddress
}

// FinalityProvider represents a finality provider actor
type FinalityProvider struct {
	*WalletSender
	BtcPrivKey  *btcec.PrivateKey
	Description string
	Commission  math.LegacyDec
}

// BtcStaker represents a Bitcoin staker actor
type BtcStaker struct {
	*WalletSender
	BtcPrivKey    *btcec.PrivateKey
	StakingAmount int64
}

// CovenantSender represents a covenant committee member
type CovenantSender struct {
	*WalletSender
	CovenantKeys []*btcec.PrivateKey
}

// NewWalletSender creates a new wallet sender with generated keys
func NewWalletSender(keyName string, n *Node) *WalletSender {
	mnemonic, err := CreateMnemonic()
	require.NoError(n.T(), err)

	info, privKey, err := CreateKeyFromMnemonic(keyName, mnemonic, n.Home)
	require.NoError(n.T(), err)

	return &WalletSender{
		Node:    n,
		Address: sdk.AccAddress(privKey.PubKey().Address()),
		PrivKey: privKey,

		KeyName:  keyName,
		Mnemonic: mnemonic,
		Home:     n.Home,
		Info:     info,

		SequenceNumber: 0,
		AccountNumber:  0,

		VerifySentTx: false,
	}
}

func (ws *WalletSender) T() *testing.T {
	return ws.Node.T()
}

// IncSeq increments the sequence number
func (ws *WalletSender) IncSeq() {
	ws.SequenceNumber++
}

// DecSeq decrements the sequence number
func (ws *WalletSender) DecSeq() {
	if ws.SequenceNumber == 0 {
		ws.T().Fatalf("sequence number is 0")
	}
	ws.SequenceNumber--
}

// Addr returns the account address
func (ws *WalletSender) Addr() string {
	return ws.Address.String()
}

// ChainID returns the chain ID from the chain config
func (ws *WalletSender) ChainID() string {
	return ws.Node.ChainConfig.ChainID
}

// SignMsg creates and signs a transaction with the provided messages
func (ws *WalletSender) SignMsg(msgs ...sdk.Msg) *sdktx.Tx {
	txBuilder := util.EncodingConfig.TxConfig.NewTxBuilder()
	err := txBuilder.SetMsgs(msgs...)
	require.NoError(ws.T(), err, "failed to set messages")

	// Set fee and gas
	txBuilder.SetFeeAmount(sdk.NewCoins(sdk.NewCoin(appparams.DefaultBondDenom, math.NewInt(20000))))
	txBuilder.SetGasLimit(500000)

	pubKey := ws.PrivKey.PubKey()
	signerData := authsigning.SignerData{
		ChainID:       ws.ChainID(),
		AccountNumber: ws.AccountNumber,
		Sequence:      ws.SequenceNumber,
		Address:       ws.Address.String(),
		PubKey:        pubKey,
	}

	// For SIGN_MODE_DIRECT, calling SetSignatures calls setSignerInfos on
	// TxBuilder under the hood, and SignerInfos is needed to generate the sign
	// bytes. This is the reason for setting SetSignatures here, with a nil
	// signature.
	sig := sdksigning.SignatureV2{
		PubKey: pubKey,
		Data: &sdksigning.SingleSignatureData{
			SignMode:  sdksigning.SignMode_SIGN_MODE_DIRECT,
			Signature: nil,
		},
		Sequence: ws.SequenceNumber,
	}

	err = txBuilder.SetSignatures(sig)
	require.NoError(ws.T(), err, "failed to set signatures")

	bytesToSign, err := authsigning.GetSignBytesAdapter(
		sdk.Context{}, // Empty context for now
		util.EncodingConfig.TxConfig.SignModeHandler(),
		sdksigning.SignMode_SIGN_MODE_DIRECT,
		signerData,
		txBuilder.GetTx(),
	)
	require.NoError(ws.T(), err, "failed to get sign bytes")

	sigBytes, err := ws.PrivKey.Sign(bytesToSign)
	require.NoError(ws.T(), err, "failed to sign bytes")

	sig = sdksigning.SignatureV2{
		PubKey: pubKey,
		Data: &sdksigning.SingleSignatureData{
			SignMode:  sdksigning.SignMode_SIGN_MODE_DIRECT,
			Signature: sigBytes,
		},
		Sequence: ws.SequenceNumber,
	}

	err = txBuilder.SetSignatures(sig)
	require.NoError(ws.T(), err, "failed to set final signatures")

	// Increment sequence number for next transaction
	ws.IncSeq()

	signedTx := txBuilder.GetTx()

	// Convert to *sdktx.Tx format
	bz, err := util.EncodingConfig.TxConfig.TxEncoder()(signedTx)
	require.NoError(ws.T(), err, "failed to encode tx")

	txDecoded, err := DecodeTx(bz)
	require.NoError(ws.T(), err, "failed to decode tx")

	return txDecoded
}

// SubmitMsgs builds the tx with the messages and sign it.
// If the wallet is tagged to wait to verify the transaction it waits for one block
// and checks if the transaction execution was success (code == 0).
func (ws *WalletSender) SubmitMsgs(msgs ...sdk.Msg) (txHash string, tx *sdktx.Tx) {
	// Sign and submit the transaction
	signedTx := ws.SignMsg(msgs...)

	txHash, err := ws.Node.SubmitTx(signedTx)
	require.NoError(ws.T(), err, "Failed to submit transaction")

	ws.AddTxSent(txHash)
	if ws.VerifySentTx {
		ws.Node.WaitForNextBlock()
		ws.T().Logf("Wallet %s is set to verify tx: %s", ws.KeyName, txHash)
		ws.Node.RequireTxSuccess(txHash)
	}

	return txHash, signedTx
}

// SubmitMsgsWithErrContain builds the tx with the messages and sign it.
// If the wallet is tagged to wait to verify the transaction it waits for one block
// and checks if the transaction execution was success or contain expected error.
func (ws *WalletSender) SubmitMsgsWithErrContain(expErr error, msgs ...sdk.Msg) (txHash string, tx *sdktx.Tx) {
	// Sign and submit the transaction
	signedTx := ws.SignMsg(msgs...)

	txHash, err := ws.Node.SubmitTx(signedTx)
	if expErr != nil && err != nil {
		require.Error(ws.T(), err, "Expected error not found")
		require.Contains(ws.T(), err.Error(), expErr.Error(), "Expected error not found")
		// revert sequence increment since it fails to submit tx, transaction rejected before block inclusion
		ws.DecSeq()
		return txHash, signedTx
	}
	require.NoError(ws.T(), err, "Failed to submit transaction")

	ws.AddTxSent(txHash)
	if ws.VerifySentTx {
		ws.Node.WaitForNextBlock()
		ws.T().Logf("Wallet %s is set to verify tx: %s", ws.KeyName, txHash)
		if expErr != nil {
			ws.Node.RequireTxErrorContain(txHash, expErr.Error())
			return txHash, signedTx
		}
		ws.Node.RequireTxSuccess(txHash)
	}

	return txHash, signedTx
}

func (ws *WalletSender) AddTxSent(txHash string) {
	ws.Txs = append(ws.Txs, txHash)
}

// UpdateAccNumberAndSeq updates the wallet's sequence and account numbers by querying the chain
func (ws *WalletSender) UpdateAccNumberAndSeq(accNum, seqNum uint64) {
	ws.AccountNumber = accNum
	ws.SequenceNumber = seqNum
}

func CreateKeyFromMnemonic(
	name, mnemonic, directoryPath string,
) (info *keyring.Record, privKey cryptotypes.PrivKey, err error) {
	kb, err := keyring.New(KeyringAppName, keyring.BackendTest, directoryPath, nil, util.Cdc)
	if err != nil {
		return nil, nil, err
	}

	keyringAlgos, _ := kb.SupportedAlgorithms()
	algo, err := keyring.NewSigningAlgoFromString(string(hd.Secp256k1Type), keyringAlgos)
	if err != nil {
		return nil, nil, err
	}

	info, err = kb.NewAccount(name, mnemonic, "", sdk.FullFundraiserPath, algo)
	if err != nil {
		return nil, nil, err
	}

	privKeyArmor, err := kb.ExportPrivKeyArmor(name, KeyringPassphrase)
	if err != nil {
		return nil, nil, err
	}

	privKey, _, err = sdkcrypto.UnarmorDecryptPrivKey(privKeyArmor, KeyringPassphrase)
	if err != nil {
		return nil, nil, err
	}

	return info, privKey, nil
}

func CreateMnemonic() (string, error) {
	entropySeed, err := bip39.NewEntropy(256)
	if err != nil {
		return "", err
	}

	mnemonic, err := bip39.NewMnemonic(entropySeed)
	if err != nil {
		return "", err
	}

	return mnemonic, nil
}

func CreateConsensusBlsKey(mnemonic, rootDir string) (*appsigner.ConsensusKey, error) {
	bls, err := GenBlsKey(rootDir)
	if err != nil {
		return nil, err
	}

	filePV := GenCometKey(mnemonic, rootDir)
	return &appsigner.ConsensusKey{
		Comet: &filePV.Key,
		Bls:   &bls.Key,
	}, nil
}

func GenCometKey(mnemonic, rootDir string) *privval.FilePV {
	config := NodeConfig(rootDir)
	pvKeyFile := config.PrivValidatorKeyFile()
	pvStateFile := config.PrivValidatorStateFile()

	var privKey ed25519.PrivKey
	if mnemonic == "" {
		privKey = ed25519.GenPrivKey()
	} else {
		privKey = ed25519.GenPrivKeyFromSecret([]byte(mnemonic))
	}

	filePV := privval.NewFilePV(privKey, pvKeyFile, pvStateFile)
	filePV.Key.Save()
	filePV.LastSignState.Save()
	return filePV
}

func GenBlsKey(rootDir string) (*appsigner.Bls, error) {
	config := NodeConfig(rootDir)
	pvKeyFile := config.PrivValidatorKeyFile()
	pvStateFile := config.PrivValidatorStateFile()

	blsKeyFile := appsigner.DefaultBlsKeyFile(rootDir)
	blsPasswordFile := appsigner.DefaultBlsPasswordFile(rootDir)

	if err := appsigner.EnsureDirs(pvKeyFile, pvStateFile, blsKeyFile, blsPasswordFile); err != nil {
		return nil, fmt.Errorf("failed to ensure dirs: %w", err)
	}

	return appsigner.GenBls(blsKeyFile, blsPasswordFile, BlsPwd), nil
}

func NodeConfig(rootDir string) *cmtcfg.Config {
	serverCtx := server.NewDefaultContext()
	config := serverCtx.Config
	config.SetRoot(rootDir)
	return config
}
