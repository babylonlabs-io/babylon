package keeper

import (
	"github.com/cometbft/cometbft/crypto/tmhash"
	sdk "github.com/cosmos/cosmos-sdk/types"
	clienttypes "github.com/cosmos/ibc-go/v10/modules/core/02-client/types" //nolint:staticcheck
	ibctmtypes "github.com/cosmos/ibc-go/v10/modules/light-clients/07-tendermint"

	"github.com/babylonlabs-io/babylon/v4/x/zoneconcierge/types"
)

var _ sdk.PostDecorator = &IBCHeaderDecorator{}

type IBCHeaderDecorator struct {
	k *Keeper
}

// NewIBCHeaderDecorator creates a new IBCHeaderDecorator
func NewIBCHeaderDecorator(k *Keeper) *IBCHeaderDecorator {
	return &IBCHeaderDecorator{
		k: k,
	}
}

// getHeaderAndClientState extracts the header info and client state from an IBC update client message
func (d *IBCHeaderDecorator) getHeaderAndClientState(ctx sdk.Context, m sdk.Msg) (*types.HeaderInfo, *ibctmtypes.ClientState) {
	// ensure the message is MsgUpdateClient
	msgUpdateClient, ok := m.(*clienttypes.MsgUpdateClient)
	if !ok {
		return nil, nil
	}
	// unpack ClientMsg inside MsgUpdateClient
	clientMsg, err := clienttypes.UnpackClientMessage(msgUpdateClient.ClientMessage)
	if err != nil {
		return nil, nil
	}
	// ensure the ClientMsg is a Comet header
	ibctmHeader, ok := clientMsg.(*ibctmtypes.Header)
	if !ok {
		return nil, nil
	}

	// all good, we get the headerInfo
	headerInfo := &types.HeaderInfo{
		ClientId: msgUpdateClient.ClientId,
		ChainId:  ibctmHeader.Header.ChainID,
		AppHash:  ibctmHeader.Header.AppHash,
		Height:   uint64(ibctmHeader.Header.Height),
		Time:     ibctmHeader.Header.Time,
	}

	// ensure the corresponding clientState exists
	clientState, exist := d.k.clientKeeper.GetClientState(ctx, msgUpdateClient.ClientId)
	if !exist {
		return nil, nil
	}
	// ensure the clientState is a Comet clientState
	cmtClientState, ok := clientState.(*ibctmtypes.ClientState)
	if !ok {
		return nil, nil
	}

	return headerInfo, cmtClientState
}

// PostHandle processes IBC client update messages after they are executed. For each message:
// - Extracts header info and client state if it's a valid IBC client update message
// - Determines if the header is a fork header by checking if client is frozen
// - Handles the header appropriately via HandleHeaderWithValidCommit
// - Unfreezes client if it was frozen due to a fork header
// Only runs during block finalization or tx simulation, and only for successful txs.
func (d *IBCHeaderDecorator) PostHandle(ctx sdk.Context, tx sdk.Tx, simulate, success bool, next sdk.PostHandler) (sdk.Context, error) {
	// only do this when finalizing a block or simulating the current tx
	if ctx.ExecMode() != sdk.ExecModeFinalize && !simulate {
		return next(ctx, tx, simulate, success)
	}
	// ignore unsuccessful tx
	// NOTE: tx with a misbehaving header will still succeed, but will make the client to be frozen
	if !success {
		return next(ctx, tx, simulate, success)
	}

	// calculate tx hash
	txHash := tmhash.Sum(ctx.TxBytes())

	for _, msg := range tx.GetMsgs() {
		// try to extract the headerInfo and the client's status
		headerInfo, clientState := d.getHeaderAndClientState(ctx, msg)
		if headerInfo == nil {
			continue
		}

		isOnFork := !clientState.FrozenHeight.IsZero()
		d.k.HandleHeaderWithValidCommit(ctx, txHash, headerInfo, isOnFork)
	}

	return next(ctx, tx, simulate, success)
}
