package prepare

import (
	"context"

	cmtprotocrypto "github.com/cometbft/cometbft/proto/tendermint/crypto"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/babylonlabs-io/babylon/v3/crypto/bls12381"
	"github.com/babylonlabs-io/babylon/v3/x/checkpointing/types"
	epochingtypes "github.com/babylonlabs-io/babylon/v3/x/epoching/types"
)

type CheckpointingKeeper interface {
	GetPubKeyByConsAddr(context.Context, sdk.ConsAddress) (cmtprotocrypto.PublicKey, error)
	GetEpoch(ctx context.Context) *epochingtypes.Epoch
	GetValidatorSet(ctx context.Context, epochNumber uint64) epochingtypes.ValidatorSet
	GetTotalVotingPower(ctx context.Context, epochNumber uint64) int64
	GetBlsPubKey(ctx context.Context, address sdk.ValAddress) (bls12381.PublicKey, error)
	VerifyBLSSig(ctx context.Context, sig *types.BlsSig) error
	SealCheckpoint(ctx context.Context, ckptWithMeta *types.RawCheckpointWithMeta) error
}
