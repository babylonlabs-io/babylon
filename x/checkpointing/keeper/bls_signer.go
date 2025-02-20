package keeper

import (
	"context"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/babylonlabs-io/babylon/crypto/bls12381"
	"github.com/babylonlabs-io/babylon/x/checkpointing/types"
)

// SignBLS signs a BLS signature over the given information
func (k Keeper) SignBLS(epochNum uint64, blockHash types.BlockHash) (bls12381.Signature, error) {
	// get BLS signature by signing
	signBytes := types.GetSignBytes(epochNum, blockHash)
	return k.blsSigner.SignMsgWithBls(signBytes)
}

// GetValidatorAddress returns the validator address of the signer
func (k Keeper) GetValidatorAddress(ctx context.Context) (sdk.ValAddress, error) {
	blsPubKey, err := k.blsSigner.BlsPubKey()
	if err != nil {
		return nil, fmt.Errorf("failed to get BLS public key: %w", err)
	}
	return k.GetValAddr(ctx, blsPubKey)
}
