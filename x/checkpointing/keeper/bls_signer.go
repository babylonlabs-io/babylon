package keeper

import (
	"github.com/cometbft/cometbft/crypto"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/babylonlabs-io/babylon/crypto/bls12381"
	"github.com/babylonlabs-io/babylon/x/checkpointing/types"
)

type BlsSigner interface {
	// GetAddress() sdk.ValAddress
	SignMsgWithBls(msg []byte) (bls12381.Signature, error)
	GetBlsPubkey() (bls12381.PublicKey, error)
	GetValidatorPubkey() crypto.PubKey
}

// SignBLS signs a BLS signature over the given information
func (k Keeper) SignBLS(epochNum uint64, blockHash types.BlockHash) (bls12381.Signature, error) {
	// get BLS signature by signing
	signBytes := types.GetSignBytes(epochNum, blockHash)
	return k.blsSigner.SignMsgWithBls(signBytes)
}

// GetConAddressFromPubkey returns the consensus address
func (k Keeper) GetConAddressFromPubkey() sdk.ConsAddress {
	pk := k.blsSigner.GetValidatorPubkey()
	return sdk.ConsAddress(pk.Address())
}

// GetValAddressFromPubkey returns the validator address
func (k Keeper) GetValAddressFromPubkey() sdk.ValAddress {
	pk := k.blsSigner.GetValidatorPubkey()
	return sdk.ValAddress(pk.Address())
}
