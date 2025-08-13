package types

import (
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"cosmossdk.io/math"
)

// WalletSender manages transaction sending for an account
type WalletSender struct {
	Address sdk.AccAddress
	PrivKey cryptotypes.PrivKey

	SequenceNumber uint64
	AccountNumber  uint64

	ChainConfig *ChainConfig
	// KeyName used in keys add
	KeyName string
}

type ValidatorWallet struct {
	*WalletSender
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
func NewWalletSender(keyName string, chainConfig *ChainConfig) *WalletSender {
	privKey := secp256k1.GenPrivKey()

	return &WalletSender{
		PrivKey:        privKey,
		SequenceNumber: 0,
		AccountNumber:  0,
		Address:        sdk.AccAddress(privKey.PubKey().Address()),
		ChainConfig:    chainConfig,
		KeyName:        keyName,
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

// NewFinalityProvider creates a new finality provider
func NewFinalityProvider(keyName string, chainConfig *ChainConfig) *FinalityProvider {
	walletSender := NewWalletSender(keyName, chainConfig)
	btcPrivKey, _ := btcec.NewPrivateKey()

	return &FinalityProvider{
		WalletSender: walletSender,
		BtcPrivKey:   btcPrivKey,
		Description:  "Test Finality Provider",
		Commission:   math.LegacyNewDecWithPrec(5, 2), // 5%
	}
}

// NewBtcStaker creates a new Bitcoin staker
func NewBtcStaker(keyName string, chainConfig *ChainConfig) *BtcStaker {
	walletSender := NewWalletSender(keyName, chainConfig)
	btcPrivKey, _ := btcec.NewPrivateKey()

	return &BtcStaker{
		WalletSender:  walletSender,
		BtcPrivKey:    btcPrivKey,
		StakingAmount: 1000000, // 1 BBN
	}
}

// NewCovenantSender creates a new covenant sender
func NewCovenantSender(keyName string, chainConfig *ChainConfig) *CovenantSender {
	walletSender := NewWalletSender(keyName, chainConfig)

	// Generate some covenant keys
	covenantKeys := make([]*btcec.PrivateKey, 3)
	for i := range covenantKeys {
		covenantKeys[i], _ = btcec.NewPrivateKey()
	}

	return &CovenantSender{
		WalletSender: walletSender,
		CovenantKeys: covenantKeys,
	}
}
