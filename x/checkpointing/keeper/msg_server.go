package keeper

import (
	"context"

	"github.com/cometbft/cometbft/crypto/tmhash"
	sdk "github.com/cosmos/cosmos-sdk/types"

	epochingtypes "github.com/babylonlabs-io/babylon/v4/x/epoching/types"

	"github.com/babylonlabs-io/babylon/v4/x/checkpointing/types"
)

type msgServer struct {
	k Keeper
}

// NewMsgServerImpl returns an implementation of the MsgServer interface
// for the provided Keeper.
func NewMsgServerImpl(keeper Keeper) types.MsgServer {
	return &msgServer{keeper}
}

var _ types.MsgServer = msgServer{}

// WrappedCreateValidator registers validator's BLS public key
// and forwards corresponding MsgCreateValidator message to
// the epoching module
func (m msgServer) WrappedCreateValidator(goCtx context.Context, msg *types.MsgWrappedCreateValidator) (*types.MsgWrappedCreateValidatorResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	// stateless checks on the inside `MsgCreateValidator` msg
	executeGas, err := m.k.epochingKeeper.CheckMsgCreateValidator(ctx, msg.MsgCreateValidator)
	if err != nil {
		return nil, err
	}

	valAddr, err := sdk.ValAddressFromBech32(msg.MsgCreateValidator.ValidatorAddress)
	if err != nil {
		return nil, err
	}

	// store BLS public key
	err = m.k.CreateRegistration(ctx, *msg.Key.Pubkey, valAddr)
	if err != nil {
		return nil, err
	}

	blockHeight := uint64(ctx.HeaderInfo().Height)
	if blockHeight == 0 {
		// no need to put in a queue if it is a genesis transactions
		err = m.k.epochingKeeper.StkMsgCreateValidator(ctx, msg.MsgCreateValidator)
		if err != nil {
			return nil, err
		}
		return &types.MsgWrappedCreateValidatorResponse{}, nil
	}

	blockTime := ctx.HeaderInfo().Time
	txid := tmhash.Sum(ctx.TxBytes())
	queueMsg, err := epochingtypes.NewQueuedMessage(blockHeight, blockTime, txid, msg.MsgCreateValidator)
	if err != nil {
		return nil, err
	}

	// lock the delegation amount to ensure funds are available when the queued message executes
	// this prevents spam attacks by requiring actual fund ownership and guarantees successful execution
	err = m.k.epochingKeeper.LockFundsForDelegateMsgs(ctx, &queueMsg)
	if err != nil {
		return nil, err
	}

	m.k.epochingKeeper.EnqueueMsg(ctx, queueMsg)

	// charge gas upfront for executing the message later at epoch end
	ctx.GasMeter().ConsumeGas(executeGas, "epoching create validator execution fee")

	return &types.MsgWrappedCreateValidatorResponse{}, nil
}
