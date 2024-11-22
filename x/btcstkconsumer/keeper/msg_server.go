package keeper

import (
	"context"

	"github.com/babylonlabs-io/babylon/x/btcstkconsumer/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
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
func (ms msgServer) RegisterConsumer(ctx context.Context, req *types.MsgRegisterConsumer) (*types.MsgRegisterConsumerResponse, error) {
	if err := req.ValidateBasic(); err != nil {
		return nil, err
	}

	if len(req.CosmosIbcClientId) > 0 {
		// this is a Cosmos consumer

		// ensure the IBC light client exists
		sdkCtx := sdk.UnwrapSDKContext(ctx)
		_, exist := ms.clientKeeper.GetClientState(sdkCtx, req.CosmosIbcClientId)
		if !exist {
			return nil, types.ErrInvalidCosmosConsumerRequest.Wrapf("IBC light client does not exist")
		}

		// all good, register this Cosmos consumer
		consumerRegister := types.NewCosmosConsumerRegister(
			req.ConsumerId,
			req.ConsumerName,
			req.ConsumerDescription,
			req.CosmosIbcClientId,
		)
		ms.Keeper.RegisterConsumer(ctx, consumerRegister)
	}

	if len(req.EthL2FinalityContractAddress) > 0 {
		// this is a ETH L2 consumer

		// ensure the ETH L2 finality contract exists
		contractAddr, err := sdk.AccAddressFromBech32(req.EthL2FinalityContractAddress)
		if err != nil {
			return nil, types.ErrInvalidETHL2ConsumerRequest.Wrapf(err.Error())
		}
		contractInfo := ms.wasmKeeper.GetContractInfo(ctx, contractAddr)
		if contractInfo == nil {
			return nil, types.ErrInvalidETHL2ConsumerRequest.Wrapf("ETH L2 finality contract does not exist")
		}

		// all good, register this ETH L2 consumer
		consumerRegister := types.NewETHL2ConsumerRegister(
			req.ConsumerId,
			req.ConsumerName,
			req.ConsumerDescription,
			req.EthL2FinalityContractAddress,
		)
		ms.Keeper.RegisterConsumer(ctx, consumerRegister)
	}

	return &types.MsgRegisterConsumerResponse{}, nil
}
