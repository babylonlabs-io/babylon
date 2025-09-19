package keeper

import (
	"context"
	"errors"
	"fmt"

	corestoretypes "cosmossdk.io/core/store"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"

	txformat "github.com/babylonlabs-io/babylon/v4/btctxformatter"

	"cosmossdk.io/log"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"

	cmtprotocrypto "github.com/cometbft/cometbft/proto/tendermint/crypto"

	"github.com/babylonlabs-io/babylon/v4/crypto/bls12381"
	"github.com/babylonlabs-io/babylon/v4/x/checkpointing/types"
	epochingtypes "github.com/babylonlabs-io/babylon/v4/x/epoching/types"
)

type (
	Keeper struct {
		cdc            codec.BinaryCodec
		storeService   corestoretypes.KVStoreService
		blsSigner      types.BlsSigner
		epochingKeeper types.EpochingKeeper
		hooks          types.CheckpointingHooks
	}
)

func NewKeeper(
	cdc codec.BinaryCodec,
	storeService corestoretypes.KVStoreService,
	signer types.BlsSigner,
	ek types.EpochingKeeper,
) Keeper {
	return Keeper{
		cdc:            cdc,
		storeService:   storeService,
		blsSigner:      signer,
		epochingKeeper: ek,
		hooks:          nil,
	}
}

func (k Keeper) Logger(ctx sdk.Context) log.Logger {
	return ctx.Logger().With("module", fmt.Sprintf("x/%s", types.ModuleName))
}

// SetHooks sets the validator hooks
func (k *Keeper) SetHooks(sh types.CheckpointingHooks) *Keeper {
	if k.hooks != nil {
		panic("cannot set validator hooks twice")
	}

	k.hooks = sh

	return k
}

func (k Keeper) SealCheckpoint(ctx context.Context, ckptWithMeta *types.RawCheckpointWithMeta) error {
	if ckptWithMeta.Status != types.Sealed {
		return fmt.Errorf("the checkpoint is not Sealed")
	}

	sdkCtx := sdk.UnwrapSDKContext(ctx)

	// if reaching this line, it means ckptWithMeta is updated,
	// and we need to write the updated ckptWithMeta back to KVStore
	if err := k.AddRawCheckpoint(ctx, ckptWithMeta); err != nil {
		return err
	}

	// record state update of Sealed
	ckptWithMeta.RecordStateUpdate(ctx, types.Sealed)
	// emit event
	if err := sdkCtx.EventManager().EmitTypedEvent(
		&types.EventCheckpointSealed{Checkpoint: ckptWithMeta},
	); err != nil {
		panic(fmt.Errorf("failed to emit checkpoint sealed event for epoch %v", ckptWithMeta.Ckpt.EpochNum))
	}
	// invoke hook
	if err := k.AfterRawCheckpointSealed(ctx, ckptWithMeta.Ckpt.EpochNum); err != nil {
		k.Logger(sdkCtx).Error("failed to trigger checkpoint sealed hook for epoch %v: %v", ckptWithMeta.Ckpt.EpochNum, err)
	}

	// log in console
	k.Logger(sdkCtx).Info(fmt.Sprintf("Checkpointing: checkpoint for epoch %v is Sealed", ckptWithMeta.Ckpt.EpochNum))

	return nil
}

func (k Keeper) VerifyBLSSig(ctx context.Context, sig *types.BlsSig) error {
	// get signer's address
	signerAddr, err := sdk.ValAddressFromBech32(sig.SignerAddress)
	if err != nil {
		return err
	}

	signerBlsKey, err := k.GetBlsPubKey(ctx, signerAddr)
	if err != nil {
		return err
	}

	// verify BLS sig
	signBytes := types.GetSignBytes(sig.GetEpochNum(), *sig.BlockHash)
	ok, err := bls12381.Verify(*sig.BlsSig, signerBlsKey, signBytes)
	if err != nil {
		return err
	}
	if !ok {
		return types.ErrInvalidBlsSignature
	}

	return nil
}

func (k Keeper) GetVotingPowerByAddress(ctx context.Context, epochNum uint64, valAddr sdk.ValAddress) (int64, error) {
	vals := k.GetValidatorSet(ctx, epochNum)

	v, _, err := vals.FindValidatorWithIndex(valAddr)
	if err != nil {
		return 0, err
	}

	return v.Power, nil
}

func (k Keeper) GetRawCheckpoint(ctx context.Context, epochNum uint64) (*types.RawCheckpointWithMeta, error) {
	return k.CheckpointsState(ctx).GetRawCkptWithMeta(epochNum)
}

func (k Keeper) GetStatus(ctx context.Context, epochNum uint64) (types.CheckpointStatus, error) {
	ckptWithMeta, err := k.GetRawCheckpoint(ctx, epochNum)
	if err != nil {
		return -1, err
	}
	return ckptWithMeta.Status, nil
}

// AddRawCheckpoint adds a raw checkpoint into the storage
func (k Keeper) AddRawCheckpoint(ctx context.Context, ckptWithMeta *types.RawCheckpointWithMeta) error {
	return k.CheckpointsState(ctx).CreateRawCkptWithMeta(ckptWithMeta)
}

func (k Keeper) BuildRawCheckpoint(ctx context.Context, epochNum uint64, blockHash types.BlockHash) (*types.RawCheckpointWithMeta, error) {
	ckptWithMeta := types.NewCheckpointWithMeta(types.NewCheckpoint(epochNum, blockHash), types.Accumulating)
	ckptWithMeta.RecordStateUpdate(ctx, types.Accumulating) // record the state update of Accumulating
	err := k.AddRawCheckpoint(ctx, ckptWithMeta)
	if err != nil {
		return nil, err
	}
	k.Logger(sdk.UnwrapSDKContext(ctx)).Info(fmt.Sprintf("Checkpointing: a new raw checkpoint is built for epoch %v", epochNum))

	return ckptWithMeta, nil
}

func (k Keeper) VerifyRawCheckpoint(ctx context.Context, ckpt *types.RawCheckpoint) error {
	// check whether sufficient voting power is accumulated
	// and verify if the multi signature is valid
	totalPower := k.GetTotalVotingPower(ctx, ckpt.EpochNum)
	signerSet, err := k.GetValidatorSet(ctx, ckpt.EpochNum).FindSubset(ckpt.Bitmap)
	if err != nil {
		return fmt.Errorf("failed to get the signer set via bitmap of epoch %d: %w", ckpt.EpochNum, err)
	}
	var sum int64
	signersPubKeys := make([]bls12381.PublicKey, len(signerSet))
	for i, v := range signerSet {
		signersPubKeys[i], err = k.GetBlsPubKey(ctx, v.Addr)
		if err != nil {
			return err
		}
		sum += v.Power
	}
	if sum*3 <= totalPower*2 {
		return types.ErrInvalidRawCheckpoint.Wrap("insufficient voting power")
	}
	msgBytes := types.GetSignBytes(ckpt.GetEpochNum(), *ckpt.BlockHash)
	ok, err := bls12381.VerifyMultiSig(*ckpt.BlsMultiSig, signersPubKeys, msgBytes)
	if err != nil {
		return err
	}
	if !ok {
		return types.ErrInvalidRawCheckpoint.Wrap("invalid BLS multi-sig")
	}

	return nil
}

// VerifyCheckpoint verifies checkpoint from BTC. It verifies
// the raw checkpoint and decides whether it is an invalid checkpoint or a
// conflicting checkpoint. A conflicting checkpoint indicates the existence
// of a fork
func (k Keeper) VerifyCheckpoint(ctx context.Context, checkpoint txformat.RawBtcCheckpoint) error {
	_, err := k.verifyCkptBytes(ctx, &checkpoint)
	if err != nil {
		if errors.Is(err, types.ErrConflictingCheckpoint) {
			// Set the conflicting checkpoint flag that will halt
			// the chain based on module's EndBlock logic
			k.SetConflictingCheckpointReceived(ctx, true)
		}
		return err
	}
	return nil
}

// verifyCkptBytes verifies checkpoint from BTC. A checkpoint is valid if
// it equals to the existing raw checkpoint. Otherwise, it further verifies
// the raw checkpoint and decides whether it is an invalid checkpoint or a
// conflicting checkpoint. A conflicting checkpoint indicates the existence
// of a fork
func (k Keeper) verifyCkptBytes(ctx context.Context, rawCheckpoint *txformat.RawBtcCheckpoint) (*types.RawCheckpointWithMeta, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	ckpt, err := types.FromBTCCkptToRawCkpt(rawCheckpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to decode raw checkpoint from BTC raw checkpoint: %w", err)
	}
	// sanity check
	err = ckpt.ValidateBasic()
	if err != nil {
		return nil, err
	}
	ckptWithMeta, err := k.GetRawCheckpoint(ctx, ckpt.EpochNum)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch the raw checkpoint at epoch %d from database: %w", ckpt.EpochNum, err)
	}

	// can skip the checks if it is identical with the local checkpoint that is not accumulating
	if ckptWithMeta.Ckpt.Equal(ckpt) && ckptWithMeta.Status != types.Accumulating {
		// record verified checkpoint
		err = k.AfterRawCheckpointBlsSigVerified(ctx, ckpt)
		if err != nil {
			return nil, fmt.Errorf("failed to record verified checkpoint of epoch %d for monitoring: %w", ckpt.EpochNum, err)
		}
		return ckptWithMeta, nil
	}

	// verify raw checkpoint
	if err := k.VerifyRawCheckpoint(ctx, ckpt); err != nil {
		return nil, err
	}

	// record verified checkpoint
	err = k.AfterRawCheckpointBlsSigVerified(ctx, ckpt)
	if err != nil {
		return nil, fmt.Errorf("failed to record verified checkpoint of epoch %d for monitoring: %w", ckpt.EpochNum, err)
	}

	// now the checkpoint's multi-sig is valid, if the BlockHash is the
	// same with that of the local checkpoint, it means it is valid except that
	// it is signed by a different signer set
	if ckptWithMeta.Ckpt.BlockHash.Equal(*ckpt.BlockHash) {
		return ckptWithMeta, nil
	}

	// multi-sig is valid but the quorum is on a different branch, meaning conflicting is observed
	k.Logger(sdkCtx).Error(types.ErrConflictingCheckpoint.Wrapf("epoch %v", ckpt.EpochNum).Error())
	// report conflicting checkpoint event
	err = sdkCtx.EventManager().EmitTypedEvent(
		&types.EventConflictingCheckpoint{
			ConflictingCheckpoint: ckpt,
			LocalCheckpoint:       ckptWithMeta,
		},
	)
	if err != nil {
		panic(err)
	}

	return nil, types.ErrConflictingCheckpoint
}

func (k *Keeper) SetEpochingKeeper(ek types.EpochingKeeper) {
	k.epochingKeeper = ek
}

func (k Keeper) validateCheckpointStatus(
	ckptWithMeta *types.RawCheckpointWithMeta,
	allowedStatuses []types.CheckpointStatus,
) error {
	for _, status := range allowedStatuses {
		if ckptWithMeta.Status == status {
			return nil
		}
	}

	var statusStrings []string
	for _, status := range allowedStatuses {
		statusStrings = append(statusStrings, status.String())
	}
	return types.ErrInvalidCkptStatus.Wrapf("the status of the checkpoint should be one of %v", statusStrings)
}

// SetCheckpointSubmitted sets the status of a checkpoint to SUBMITTED,
// and records the associated state update in lifecycle
func (k Keeper) SetCheckpointSubmitted(ctx context.Context, epoch uint64) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	ckpt, err := k.setCheckpointStatus(ctx, epoch, []types.CheckpointStatus{types.Sealed}, types.Submitted)
	if err != nil {
		k.Logger(sdkCtx).Error("failed to set checkpoint status to SUBMITTED for epoch %v: %v", epoch, err)
		return
	}
	err = sdkCtx.EventManager().EmitTypedEvent(
		&types.EventCheckpointSubmitted{Checkpoint: ckpt},
	)
	if err != nil {
		k.Logger(sdkCtx).Error("failed to emit checkpoint submitted event for epoch %v", ckpt.Ckpt.EpochNum)
	}
}

// SetCheckpointConfirmed sets the status of a checkpoint to CONFIRMED,
// and records the associated state update in lifecycle
func (k Keeper) SetCheckpointConfirmed(ctx context.Context, epoch uint64) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	ckpt, err := k.setCheckpointStatus(ctx, epoch, []types.CheckpointStatus{types.Submitted}, types.Confirmed)
	if err != nil {
		k.Logger(sdkCtx).Error("failed to set checkpoint status to CONFIRMED for epoch %v: %v", epoch, err)
		return
	}
	err = sdkCtx.EventManager().EmitTypedEvent(
		&types.EventCheckpointConfirmed{Checkpoint: ckpt},
	)
	if err != nil {
		k.Logger(sdkCtx).Error("failed to emit checkpoint confirmed event for epoch %v: %v", ckpt.Ckpt.EpochNum, err)
	}
	// invoke hook
	if err := k.AfterRawCheckpointConfirmed(ctx, epoch); err != nil {
		k.Logger(sdkCtx).Error("failed to trigger checkpoint confirmed hook for epoch %v: %v", ckpt.Ckpt.EpochNum, err)
	}
}

// SetCheckpointFinalized sets the status of a checkpoint to FINALIZED,
// and records the associated state update in lifecycle
func (k Keeper) SetCheckpointFinalized(ctx context.Context, epoch uint64) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	// set the checkpoint's status to be finalised
	ckpt, err := k.setCheckpointStatus(ctx, epoch, []types.CheckpointStatus{types.Confirmed}, types.Finalized)
	if err != nil {
		k.Logger(sdkCtx).Error("failed to set checkpoint status to FINALIZED for epoch %v: %v", epoch, err)
		return
	}
	// remember the last finalised epoch
	k.SetLastFinalizedEpoch(ctx, epoch)
	// emit event
	err = sdkCtx.EventManager().EmitTypedEvent(
		&types.EventCheckpointFinalized{Checkpoint: ckpt},
	)
	if err != nil {
		k.Logger(sdkCtx).Error("failed to emit checkpoint finalized event for epoch %v: %v", ckpt.Ckpt.EpochNum, err)
	}
	// invoke hook, which is currently subscribed by ZoneConcierge
	if err := k.AfterRawCheckpointFinalized(ctx, epoch); err != nil {
		k.Logger(sdkCtx).Error("failed to trigger checkpoint finalized hook for epoch %v: %v", ckpt.Ckpt.EpochNum, err)
	}
}

// SetCheckpointForgotten rolls back the status of a checkpoint to Sealed,
// and records the associated state update in lifecycle
func (k Keeper) SetCheckpointForgotten(ctx context.Context, epoch uint64) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	// In case of big reorg, the checkpoint might be forgotten even if it was already
	// confirmed
	ckpt, err := k.setCheckpointStatus(ctx, epoch, []types.CheckpointStatus{types.Submitted, types.Confirmed}, types.Sealed)
	if err != nil {
		k.Logger(sdkCtx).Error("failed to set checkpoint status to SEALED for epoch %v: %v", epoch, err)
		return
	}
	err = sdkCtx.EventManager().EmitTypedEvent(
		&types.EventCheckpointForgotten{Checkpoint: ckpt},
	)
	if err != nil {
		k.Logger(sdkCtx).Error("failed to emit checkpoint forgotten event for epoch %v", ckpt.Ckpt.EpochNum)
	}

	// invoke hook
	if err := k.AfterRawCheckpointForgotten(ctx, ckpt.Ckpt); err != nil {
		k.Logger(sdkCtx).Error("failed to trigger checkpoint forgotten hook for epoch %v: %v", ckpt.Ckpt.EpochNum, err)
	}
}

// setCheckpointStatus sets a ckptWithMeta to the given state,
// and records the state update in its lifecycle
func (k Keeper) setCheckpointStatus(
	ctx context.Context,
	epoch uint64,
	expectedStatus []types.CheckpointStatus,
	to types.CheckpointStatus,
) (*types.RawCheckpointWithMeta, error) {
	ckptWithMeta, err := k.GetRawCheckpoint(ctx, epoch)
	if err != nil {
		return nil, err
	}

	if err := k.validateCheckpointStatus(ckptWithMeta, expectedStatus); err != nil {
		return nil, err
	}

	from := ckptWithMeta.Status
	ckptWithMeta.Status = to                    // set status
	ckptWithMeta.RecordStateUpdate(ctx, to)     // record state update to the lifecycle
	err = k.UpdateCheckpoint(ctx, ckptWithMeta) // write back to KVStore
	if err != nil {
		panic("failed to update checkpoint status")
	}
	statusChangeMsg := fmt.Sprintf("Checkpointing: checkpoint status for epoch %v successfully changed from %v to %v", epoch, from.String(), to.String())
	k.Logger(sdk.UnwrapSDKContext(ctx)).Info(statusChangeMsg)
	return ckptWithMeta, nil
}

func (k Keeper) UpdateCheckpoint(ctx context.Context, ckptWithMeta *types.RawCheckpointWithMeta) error {
	return k.CheckpointsState(ctx).UpdateCheckpoint(ckptWithMeta)
}

func (k Keeper) CreateRegistration(ctx context.Context, blsPubKey bls12381.PublicKey, valAddr sdk.ValAddress) error {
	return k.RegistrationState(ctx).CreateRegistration(blsPubKey, valAddr)
}

// GetBLSPubKeySet returns the set of BLS public keys in the same order of the validator set for a given epoch
func (k Keeper) GetBLSPubKeySet(ctx context.Context, epochNumber uint64) ([]*types.ValidatorWithBlsKey, error) {
	valset := k.GetValidatorSet(ctx, epochNumber)
	valWithblsKeys := make([]*types.ValidatorWithBlsKey, len(valset))
	for i, val := range valset {
		pubkey, err := k.GetBlsPubKey(ctx, val.Addr)
		if err != nil {
			return nil, err
		}
		valWithblsKeys[i] = &types.ValidatorWithBlsKey{
			ValidatorAddress: val.GetValAddressStr(),
			BlsPubKey:        pubkey,
			VotingPower:      uint64(val.Power),
		}
	}

	return valWithblsKeys, nil
}

// GetBlsPubKey returns the BLS public key of the validator
func (k Keeper) GetBlsPubKey(ctx context.Context, address sdk.ValAddress) (bls12381.PublicKey, error) {
	return k.RegistrationState(ctx).GetBlsPubKey(address)
}

// GetValAddr returns the validator address of the BLS public key
func (k Keeper) GetValAddr(ctx context.Context, key bls12381.PublicKey) (sdk.ValAddress, error) {
	return k.RegistrationState(ctx).GetValAddr(key)
}

// GetEpoch returns the current epoch
func (k Keeper) GetEpoch(ctx context.Context) *epochingtypes.Epoch {
	return k.epochingKeeper.GetEpoch(ctx)
}

// GetValidatorSet returns the validator set for a given epoch
func (k Keeper) GetValidatorSet(ctx context.Context, epochNumber uint64) epochingtypes.ValidatorSet {
	return k.epochingKeeper.GetValidatorSet(ctx, epochNumber)
}

// GetTotalVotingPower returns the total voting power for a given epoch
func (k Keeper) GetTotalVotingPower(ctx context.Context, epochNumber uint64) int64 {
	return k.epochingKeeper.GetTotalVotingPower(ctx, epochNumber)
}

// GetPubKeyByConsAddr returns the public key of a validator by consensus address
func (k Keeper) GetPubKeyByConsAddr(ctx context.Context, consAddr sdk.ConsAddress) (cmtprotocrypto.PublicKey, error) {
	return k.epochingKeeper.GetPubKeyByConsAddr(ctx, consAddr)
}

// GetValidatorByConsAddr returns the validator by consensus address
func (k Keeper) GetValidator(ctx context.Context, addr sdk.ValAddress) (stakingtypes.Validator, error) {
	return k.epochingKeeper.GetValidator(ctx, addr)
}

// GetLastFinalizedEpoch gets the last finalised epoch
func (k Keeper) GetLastFinalizedEpoch(ctx context.Context) uint64 {
	store := k.storeService.OpenKVStore(ctx)
	epochNumberBytes, err := store.Get(types.LastFinalizedEpochKey)
	if err != nil {
		// we have set epoch 0 to be finalised at genesis so this can
		// only be a programming error
		panic(err)
	}
	return sdk.BigEndianToUint64(epochNumberBytes)
}

// GetEpochByHeight returns the epoch number for a given height
func (k Keeper) GetEpochByHeight(ctx context.Context, height uint64) uint64 {
	return k.epochingKeeper.GetEpochNumByHeight(ctx, height)
}

// SetLastFinalizedEpoch sets the last finalised epoch
func (k Keeper) SetLastFinalizedEpoch(ctx context.Context, epochNumber uint64) {
	store := k.storeService.OpenKVStore(ctx)
	epochNumberBytes := sdk.Uint64ToBigEndian(epochNumber)
	if err := store.Set(types.LastFinalizedEpochKey, epochNumberBytes); err != nil {
		panic(err)
	}
}

// GetConflictingCheckpointReceived gets the stored ConflictingCheckpointReceived value
func (k Keeper) GetConflictingCheckpointReceived(ctx context.Context) bool {
	store := k.storeService.OpenKVStore(ctx)
	value, err := store.Get(types.ConflictingCheckpointReceivedKey)
	if err != nil {
		panic(err)
	}
	// If the key is not set, assume false (default behavior)
	if len(value) == 0 {
		return false
	}

	// Convert the first byte to a boolean
	return value[0] == 1
}

// SetConflictingCheckpointReceived sets the ConflictingCheckpointReceived flag value
func (k Keeper) SetConflictingCheckpointReceived(ctx context.Context, value bool) {
	store := k.storeService.OpenKVStore(ctx)
	var byteValue []byte
	switch value {
	case true:
		byteValue = []byte{1}
	default:
		byteValue = []byte{0}
	}
	if err := store.Set(types.ConflictingCheckpointReceivedKey, byteValue); err != nil {
		panic(err)
	}
}
