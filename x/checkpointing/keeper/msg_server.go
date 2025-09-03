package keeper

import (
	"context"

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
	if err := m.k.epochingKeeper.CheckMsgCreateValidator(ctx, msg.MsgCreateValidator); err != nil {
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

	if ctx.HeaderInfo().Height == 0 {
		// no need to put in a queue if it is a genesis transactions
		err = m.k.epochingKeeper.StkMsgCreateValidator(ctx, msg.MsgCreateValidator)
		if err != nil {
			return nil, err
		}
		return &types.MsgWrappedCreateValidatorResponse{}, nil
	}

	// enqueue the msg into the epoching module
	queueMsg := epochingtypes.QueuedMessage{
		Msg: &epochingtypes.QueuedMessage_MsgCreateValidator{MsgCreateValidator: msg.MsgCreateValidator},
	}

	err = m.k.epochingKeeper.LockFunds(ctx, &queueMsg)
	if err != nil {
		return nil, err
	}

	m.k.epochingKeeper.EnqueueMsg(ctx, queueMsg)
	ctx.GasMeter().ConsumeGas(m.k.epochingKeeper.GetParams(ctx).EnqueueGasFees.CreateValidator, "epoching cancel unbonding delegation enqueue fee")

	return &types.MsgWrappedCreateValidatorResponse{}, nil
}
