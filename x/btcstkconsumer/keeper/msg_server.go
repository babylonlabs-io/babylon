package keeper

import (
	"context"

	errorsmod "cosmossdk.io/errors"
	"github.com/babylonlabs-io/babylon/x/btcstkconsumer/types"
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
func (ms msgServer) RegisterConsumer(ctx context.Context, req *types.MsgRegisterConsumer) (*types.MsgRegisterConsumerResponse, error) {
	// if the permissioned integration is enabled and the signer is not the authority
	// then this is not an authorised registration request, reject
	if ms.GetParams(ctx).PermissionedIntegration && ms.authority != req.Signer {
		return nil, errorsmod.Wrapf(govtypes.ErrInvalidSigner, "invalid authority; expected %s, got %s", ms.authority, req.Signer)
	}

	if err := req.ValidateBasic(); err != nil {
		return nil, err
	}

	if len(req.EthL2FinalityContractAddress) > 0 {
		// this is a ETH L2 consumer

		// ensure the ETH L2 finality contract exists
		contractAddr, err := sdk.AccAddressFromBech32(req.EthL2FinalityContractAddress)
		if err != nil {
			return nil, types.ErrInvalidETHL2ConsumerRequest.Wrapf("%s", err.Error())
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
		if err := ms.Keeper.RegisterConsumer(ctx, consumerRegister); err != nil {
			return nil, err
		}
	} else {
		// this is a Cosmos consumer

		// ensure the IBC light client exists
		sdkCtx := sdk.UnwrapSDKContext(ctx)
		_, exist := ms.clientKeeper.GetClientState(sdkCtx, req.ConsumerId)
		if !exist {
			return nil, types.ErrInvalidCosmosConsumerRequest.Wrapf("IBC light client does not exist")
		}

		// all good, register this Cosmos consumer
		consumerRegister := types.NewCosmosConsumerRegister(
			req.ConsumerId,
			req.ConsumerName,
			req.ConsumerDescription,
		)
		if err := ms.Keeper.RegisterConsumer(ctx, consumerRegister); err != nil {
			return nil, err
		}
	}

	return &types.MsgRegisterConsumerResponse{}, nil
}
