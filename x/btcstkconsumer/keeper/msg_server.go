package keeper

import (
	"context"
	"fmt"

	errorsmod "cosmossdk.io/errors"
	"github.com/babylonlabs-io/babylon/v4/x/btcstkconsumer/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
)

type msgServer struct {
	Keeper
}

// NewMsgServerImpl returns an implementation of the MsgServer interface
// for the provided Keeper.
func NewMsgServerImpl(keeper Keeper) types.MsgServer {
	return &msgServer{Keeper: keeper}
}

var _ types.MsgServer = msgServer{}

// RegisterConsumer registers a consumer
func (ms msgServer) RegisterConsumer(goCtx context.Context, req *types.MsgRegisterConsumer) (*types.MsgRegisterConsumerResponse, error) {
	// if the permissioned integration is enabled and the signer is not the authority
	// then this is not an authorised registration request, reject
	if ms.GetParams(goCtx).PermissionedIntegration && ms.authority != req.Signer {
		return nil, errorsmod.Wrapf(govtypes.ErrInvalidSigner, "invalid authority; expected %s, got %s", ms.authority, req.Signer)
	}

	if err := req.ValidateBasic(); err != nil {
		return nil, err
	}

	var consumerType types.ConsumerType
	if len(req.RollupFinalityContractAddress) > 0 {
		// this is a rollup consumer
		consumerType = types.ConsumerType_ROLLUP
		// ensure the rollup finality contract exists
		contractAddr, err := sdk.AccAddressFromBech32(req.RollupFinalityContractAddress)
		if err != nil {
			return nil, types.ErrInvalidRollupConsumerRequest.Wrapf("%s", err.Error())
		}
		contractInfo := ms.wasmKeeper.GetContractInfo(goCtx, contractAddr)
		if contractInfo == nil {
			return nil, types.ErrInvalidRollupConsumerRequest.Wrapf("rollup finality contract does not exist")
		}

		// all good, register this rollup consumer
		consumerRegister := types.NewRollupConsumerRegister(
			req.ConsumerId,
			req.ConsumerName,
			req.ConsumerDescription,
			req.RollupFinalityContractAddress,
			req.MaxMultiStakedFps,
		)
		if err := ms.Keeper.RegisterConsumer(goCtx, consumerRegister); err != nil {
			return nil, err
		}
	} else {
		// this is a Cosmos consumer
		consumerType = types.ConsumerType_COSMOS
		// ensure the IBC light client exists
		sdkCtx := sdk.UnwrapSDKContext(goCtx)
		_, exist := ms.clientKeeper.GetClientState(sdkCtx, req.ConsumerId)
		if !exist {
			return nil, types.ErrInvalidCosmosConsumerRequest.Wrapf("IBC light client does not exist")
		}

		// all good, register this Cosmos consumer
		consumerRegister := types.NewCosmosConsumerRegister(
			req.ConsumerId,
			req.ConsumerName,
			req.ConsumerDescription,
			req.MaxMultiStakedFps,
		)
		if err := ms.Keeper.RegisterConsumer(goCtx, consumerRegister); err != nil {
			return nil, err
		}
	}

	ctx := sdk.UnwrapSDKContext(goCtx)
	if err := ctx.EventManager().EmitTypedEvent(
		types.NewConsumerRegisteredEvent(
			req.ConsumerId,
			req.ConsumerName,
			req.ConsumerDescription,
			consumerType,
			req.RollupFinalityContractAddress,
			req.MaxMultiStakedFps)); err != nil {
		panic(fmt.Errorf("failed to emit NewConsumerRegisteredEvent event: %w", err))
	}

	return &types.MsgRegisterConsumerResponse{}, nil
}
