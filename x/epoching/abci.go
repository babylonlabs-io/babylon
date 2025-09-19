package epoching

import (
	"context"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/babylonlabs-io/babylon/v4/x/epoching/keeper"
	"github.com/babylonlabs-io/babylon/v4/x/epoching/types"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/cosmos/cosmos-sdk/telemetry"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// BeginBlocker is called at the beginning of every block.
// Upon each BeginBlock,
// - record the current BlockHash
// - if reaching the epoch beginning, then
//   - increment epoch number
//   - trigger AfterEpochBegins hook
//   - emit BeginEpoch event
//
// - if reaching the sealer header, i.e., the 2nd header of a non-zero epoch, then
//   - record the sealer header for the previous epoch
//
// NOTE: we follow Cosmos SDK's slashing/evidence modules for MVP. No need to modify them at the moment.
func BeginBlocker(ctx context.Context, k keeper.Keeper) error {
	defer telemetry.ModuleMeasureSince(types.ModuleName, time.Now(), telemetry.MetricKeyBeginBlocker)

	sdkCtx := sdk.UnwrapSDKContext(ctx)

	// if this block is the first block of the next epoch
	// note that we haven't incremented the epoch number yet
	epoch := k.GetEpoch(ctx)
	if epoch.IsFirstBlockOfNextEpoch(ctx) {
		// increase epoch number
		incEpoch := k.IncEpoch(ctx)
		// record the AppHash referencing
		// the last block of the previous epoch
		k.RecordSealerAppHashForPrevEpoch(ctx)

		// Clean up message queue from the previous epoch to prevent unbounded growth
		// Only epochs after the first epoch need cleanup as epoch 1 has no previous epoch
		// This ensures that processed messages don't accumulate over time and consume storage
		if incEpoch.EpochNumber > 1 {
			prevEpochNumber := incEpoch.EpochNumber - 1
			k.ClearEpochMsgs(ctx, prevEpochNumber)
		}
		// init the msg queue of this new epoch
		k.InitMsgQueue(ctx)
		// init the slashed voting power of this new epoch
		k.InitSlashedVotingPower(ctx)
		// store the current validator set
		k.InitValidatorSet(ctx)
		// trigger AfterEpochBegins hook
		k.AfterEpochBegins(ctx, incEpoch.EpochNumber)
		// emit BeginEpoch event
		err := sdkCtx.EventManager().EmitTypedEvent(
			&types.EventBeginEpoch{
				EpochNumber: incEpoch.EpochNumber,
			},
		)
		if err != nil {
			return err
		}
	}

	if epoch.IsLastBlock(ctx) {
		// record the block hash of the last block
		// of the epoch to be sealed
		k.RecordSealerBlockHashForEpoch(ctx)
	}
	return nil
}

// EndBlocker is called at the end of every block.
// If reaching an epoch boundary, then
// - forward validator-related msgs (bonded -> unbonding) to the staking module
// - trigger AfterEpochEnds hook
// - emit EndEpoch event
// NOTE: The epoching module is not responsible for checkpoint-assisted unbonding (unbonding -> unbonded). Instead, it wraps the staking module and exposes interfaces to the checkpointing module. The checkpointing module will do the actual checkpoint-assisted unbonding upon each EndBlock.
func EndBlocker(ctx context.Context, k keeper.Keeper) ([]abci.ValidatorUpdate, error) {
	defer telemetry.ModuleMeasureSince(types.ModuleName, time.Now(), telemetry.MetricKeyEndBlocker)

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	validatorSetUpdate := []abci.ValidatorUpdate{}

	// if reaching an epoch boundary, then
	epoch := k.GetEpoch(ctx)
	if epoch.IsLastBlock(ctx) {
		// finalise this epoch, i.e., record the current header and the Merkle root of all AppHashs in this epoch
		if err := k.RecordLastHeaderTime(ctx); err != nil {
			return nil, err
		}
		// get all msgs in the msg queue
		queuedMsgs := k.GetCurrentEpochMsgs(ctx)
		// forward each msg in the msg queue to the right keeper
		for _, msg := range queuedMsgs {
			msgId := hex.EncodeToString(msg.MsgId)

			// Unlock funds first
			if err := k.UnlockFundsForDelegateMsgs(sdkCtx, msg); err != nil {
				k.Logger(sdkCtx).Error("failed to unlock funds for message",
					"msgId", msgId,
					"error", err)

				// Determine message type for context
				msgType := "unknown"
				switch msg.Msg.(type) {
				case *types.QueuedMessage_MsgDelegate:
					msgType = "MsgDelegate"
				case *types.QueuedMessage_MsgCreateValidator:
					msgType = "MsgCreateValidator"
				}

				// Emit typed event for fund unlock failure
				if eventErr := sdkCtx.EventManager().EmitTypedEvent(
					&types.EventUnlockFundsFailed{
						EpochNumber: epoch.EpochNumber,
						Height:      msg.BlockHeight,
						TxId:        msg.TxId,
						MsgId:       msg.MsgId,
						Error:       err.Error(),
						MsgType:     msgType,
					},
				); eventErr != nil {
					k.Logger(sdkCtx).Error("failed to emit EventUnlockFundsFailed", "error", eventErr)
				}
				continue
			} else {
				k.Logger(sdkCtx).Info("successfully unlocked funds for message", "msgId", msgId)
			}

			_, errQueuedMsg := k.HandleQueuedMsg(ctx, msg)
			// skip this failed msg and emit and event signalling it
			// we do not panic here as some users may wrap an invalid message
			// (e.g., self-delegate coins more than its balance, wrong coding of addresses, ...)
			// honest validators will have consistent execution results on the queued messages
			if errQueuedMsg != nil {
				// emit an event signalling the failed execution
				err := sdkCtx.EventManager().EmitTypedEvent(
					&types.EventHandleQueuedMsg{
						EpochNumber: epoch.EpochNumber,
						Height:      msg.BlockHeight,
						TxId:        msg.TxId,
						MsgId:       msg.MsgId,
						Error:       errQueuedMsg.Error(),
					},
				)
				if err != nil {
					return nil, err
				}
			}
		}

		// update validator set
		validatorSetUpdate = k.ApplyAndReturnValidatorSetUpdates(ctx)
		sdkCtx.Logger().Info(fmt.Sprintf("Epoching: validator set update of epoch %d: %v", epoch.EpochNumber, validatorSetUpdate))

		// trigger AfterEpochEnds hook
		k.AfterEpochEnds(ctx, epoch.EpochNumber)
		// emit EndEpoch event
		err := sdkCtx.EventManager().EmitTypedEvent(
			&types.EventEndEpoch{
				EpochNumber: epoch.EpochNumber,
			},
		)
		if err != nil {
			return nil, err
		}
	}

	return validatorSetUpdate, nil
}
