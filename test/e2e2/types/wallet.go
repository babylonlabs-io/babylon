package types

import (
	"fmt"

	"cosmossdk.io/math"
	"github.com/babylonlabs-io/babylon/v4/app/signer"
	appsigner "github.com/babylonlabs-io/babylon/v4/app/signer"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/cometbft/cometbft/crypto/ed25519"
	"github.com/cometbft/cometbft/privval"
	sdkcrypto "github.com/cosmos/cosmos-sdk/crypto"
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/cosmos/cosmos-sdk/server"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/go-bip39"
	"github.com/stretchr/testify/require"

	"github.com/babylonlabs-io/babylon/v4/test/e2e/util"
)

const (
	KeyringPassphrase = "testpassphrase"
	KeyringAppName    = "testnet"
)

// WalletSender manages transaction sending for an account
type WalletSender struct {
	Address sdk.AccAddress
	PrivKey cryptotypes.PrivKey

	ChainConfig *ChainConfig

	// KeyName used in babylond keys add <KeyName>
	KeyName  string
	Mnemonic string
	// Home is the path for the home folder created locally, not inside the container
	Home string
	Info *keyring.Record

	// transaction control properties are only set after the chain is running
	SequenceNumber uint64
	AccountNumber  uint64

	// Latest Txs IDs Sent
	Txs []string
}

type ValidatorWallet struct {
	*WalletSender
	ConsKey          *signer.ConsensusKey
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
		Address: sdk.AccAddress(privKey.PubKey().Address()),
		PrivKey: privKey,

		KeyName:  keyName,
		Mnemonic: mnemonic,
		Home:     n.Home,
		Info:     info,

		SequenceNumber: 0,
		AccountNumber:  0,
	}
}

// IncSeq increments the sequence number
func (ws *WalletSender) IncSeq() {
	ws.SequenceNumber++
}

// Addr returns the account address
func (ws *WalletSender) Addr() string {
	return ws.Address.String()
}

// ChainID returns the chain ID from the chain config
func (ws *WalletSender) ChainID() string {
	return ws.ChainConfig.ChainID
}

// // NewFinalityProvider creates a new finality provider
// func NewFinalityProvider(keyName string, chainConfig *ChainConfig) *FinalityProvider {
// 	walletSender := NewWalletSender(keyName, chainConfig)
// 	btcPrivKey, _ := btcec.NewPrivateKey()

// 	return &FinalityProvider{
// 		WalletSender: walletSender,
// 		BtcPrivKey:   btcPrivKey,
// 		Description:  "Test Finality Provider",
// 		Commission:   math.LegacyNewDecWithPrec(5, 2), // 5%
// 	}
// }

// // NewBtcStaker creates a new Bitcoin staker
// func NewBtcStaker(keyName string, chainConfig *ChainConfig) *BtcStaker {
// 	walletSender := NewWalletSender(keyName, chainConfig)
// 	btcPrivKey, _ := btcec.NewPrivateKey()

// 	return &BtcStaker{
// 		WalletSender:  walletSender,
// 		BtcPrivKey:    btcPrivKey,
// 		StakingAmount: 1000000, // 1 BBN
// 	}
// }

// // NewCovenantSender creates a new covenant sender
// func NewCovenantSender(keyName string, chainConfig *ChainConfig) *CovenantSender {
// 	walletSender := NewWalletSender(keyName, chainConfig)

// 	// Generate some covenant keys
// 	covenantKeys := make([]*btcec.PrivateKey, 3)
// 	for i := range covenantKeys {
// 		covenantKeys[i], _ = btcec.NewPrivateKey()
// 	}

// 	return &CovenantSender{
// 		WalletSender: walletSender,
// 		CovenantKeys: covenantKeys,
// 	}
// }

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

func CreateConsensusKey(moniker, mnemonic, rootDir string) (*appsigner.ConsensusKey, error) {
	serverCtx := server.NewDefaultContext()
	config := serverCtx.Config
	config.SetRoot(rootDir)
	config.Moniker = moniker

	pvKeyFile := config.PrivValidatorKeyFile()
	pvStateFile := config.PrivValidatorStateFile()
	blsKeyFile := appsigner.DefaultBlsKeyFile(rootDir)
	blsPasswordFile := appsigner.DefaultBlsPasswordFile(rootDir)

	if err := appsigner.EnsureDirs(pvKeyFile, pvStateFile, blsKeyFile, blsPasswordFile); err != nil {
		return nil, fmt.Errorf("failed to ensure dirs: %w", err)
	}

	// create file pv
	var privKey ed25519.PrivKey
	if mnemonic == "" {
		privKey = ed25519.GenPrivKey()
	} else {
		privKey = ed25519.GenPrivKeyFromSecret([]byte(mnemonic))
	}
	filePV := privval.NewFilePV(privKey, pvKeyFile, pvStateFile)
	filePV.Key.Save()
	filePV.LastSignState.Save()

	// create bls pv
	bls := appsigner.GenBls(blsKeyFile, blsPasswordFile, "password")

	return &appsigner.ConsensusKey{
		Comet: &filePV.Key,
		Bls:   &bls.Key,
	}, nil
}
