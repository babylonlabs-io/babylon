package types

import (
	"github.com/btcsuite/btcd/btcec/v2"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

type ChainKeyInfo struct {
	Name       string
	Mnemonic   string
	AccAddress sdk.AccAddress
	PublicKey  *btcec.PublicKey
	PrivateKey *btcec.PrivateKey
}
