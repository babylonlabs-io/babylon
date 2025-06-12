package keeper

import (
	"context"
	"fmt"

	corestoretypes "cosmossdk.io/core/store"
	"cosmossdk.io/log"

	bbn "github.com/babylonlabs-io/babylon/v3/types"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/wire"

	"github.com/babylonlabs-io/babylon/v3/x/btclightclient/types"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	proto "github.com/cosmos/gogoproto/proto"
)

type (
	Keeper struct {
		cdc          codec.BinaryCodec
		storeService corestoretypes.KVStoreService
		hooks        types.BTCLightClientHooks
		iKeeper      types.IncentiveKeeper
		btcConfig    bbn.BtcConfig
		bl           *types.BtcLightClient
		authority    string
	}
)

var _ types.BtcChainReadStore = (*headersState)(nil)

func NewKeeper(
	cdc codec.BinaryCodec,
	storeService corestoretypes.KVStoreService,
	btcConfig bbn.BtcConfig,
	iKeeper types.IncentiveKeeper,
	authority string,
) Keeper {
	bl := types.NewBtcLightClientFromParams(btcConfig.NetParams())

	return Keeper{
		cdc:          cdc,
		storeService: storeService,
		hooks:        nil,
		iKeeper:      iKeeper,
		btcConfig:    btcConfig,
		bl:           bl,
		authority:    authority,
	}
}

// Logger returns the logger with the key value of the current module.
func (Keeper) Logger(ctx sdk.Context) log.Logger {
	return ctx.Logger().With("module", fmt.Sprintf("x/%s", types.ModuleName))
}

// emitTypedEventWithLog emits an event and logs if it errors.
func (k Keeper) emitTypedEventWithLog(ctx context.Context, evt proto.Message) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	if err := sdkCtx.EventManager().EmitTypedEvent(evt); err != nil {
		k.Logger(sdkCtx).Error(
			"failed to emit event",
			"type", evt.String(),
			"reason", err.Error(),
		)
	}
}

// SetHooks sets the btclightclient hooks
func (k *Keeper) SetHooks(bh types.BTCLightClientHooks) *Keeper {
	if k.hooks != nil {
		panic("cannot set btclightclient hooks twice")
	}
	k.hooks = bh

	return k
}

func (k Keeper) insertHandler() func(ctx context.Context, s headersState, result *types.InsertResult) error {
	return func(ctx context.Context, s headersState, result *types.InsertResult) error {
		// if we receive rollback, should return error
		if result.RollbackInfo != nil {
			return fmt.Errorf("rollback should not happen %+v", result.RollbackInfo)
		}

		for _, header := range result.HeadersToInsert {
			h := header
			s.insertHeader(h)
		}
		return nil
	}
}

func (k Keeper) triggerEventAndHandleHooksHandler() func(ctx context.Context, s headersState, result *types.InsertResult) error {
	return func(ctx context.Context, s headersState, result *types.InsertResult) error {
		// if we have rollback, first delete all headers up to the rollback point
		if result.RollbackInfo != nil {
			// gets the tip prior to rollback and delete
			lastTip := s.GetTip()
			// roll back to the height
			s.rollBackHeadersUpTo(result.RollbackInfo.HeaderToRollbackTo.Height)
			// trigger rollback event
			k.triggerRollBack(ctx, lastTip, result.RollbackInfo.HeaderToRollbackTo)
		}

		for _, header := range result.HeadersToInsert {
			h := header
			s.insertHeader(h)
			k.triggerHeaderInserted(ctx, h)
			k.triggerRollForward(ctx, h)
		}
		return nil
	}
}

func (k Keeper) insertHeadersWithHookAndEvents(
	ctx context.Context,
	headers []*wire.BlockHeader) error {
	return k.insertHeadersInternal(
		ctx,
		headers,
		k.triggerEventAndHandleHooksHandler(),
	)
}

func (k Keeper) insertHeaders(
	ctx context.Context,
	headers []*wire.BlockHeader) error {
	return k.insertHeadersInternal(
		ctx,
		headers,
		k.insertHandler(),
	)
}

func (k Keeper) insertHeadersInternal(
	ctx context.Context,
	headers []*wire.BlockHeader,
	handleInsertResult func(ctx context.Context, s headersState, result *types.InsertResult) error,
) error {
	headerState := k.headersState(ctx)

	result, err := k.bl.InsertHeaders(
		headerState,
		headers,
	)

	if err != nil {
		return err
	}

	return handleInsertResult(ctx, headerState, result)
}

// InsertHeaderInfos inserts multiple headers info at the store.
func (k Keeper) InsertHeaderInfos(ctx context.Context, infos []*types.BTCHeaderInfo) {
	hs := k.headersState(ctx)
	for _, inf := range infos {
		hs.insertHeader(inf)
	}
}

func (k Keeper) InsertHeadersWithHookAndEvents(ctx context.Context, headers []bbn.BTCHeaderBytes) error {
	if len(headers) == 0 {
		return types.ErrEmptyMessage
	}

	blockHeaders := BtcHeadersBytesToBlockHeader(headers)
	return k.insertHeadersWithHookAndEvents(ctx, blockHeaders)
}

func (k Keeper) InsertHeaders(ctx context.Context, headers []bbn.BTCHeaderBytes) error {
	if len(headers) == 0 {
		return types.ErrEmptyMessage
	}

	blockHeaders := BtcHeadersBytesToBlockHeader(headers)
	return k.insertHeaders(ctx, blockHeaders)
}

func BtcHeadersBytesToBlockHeader(headers []bbn.BTCHeaderBytes) []*wire.BlockHeader {
	blockHeaders := make([]*wire.BlockHeader, len(headers))
	for i, header := range headers {
		blockHeaders[i] = header.ToBlockHeader()
	}

	return blockHeaders
}

// BlockHeight returns the height of the provided header
func (k Keeper) BlockHeight(ctx context.Context, headerHash *bbn.BTCHeaderHashBytes) (uint32, error) {
	if headerHash == nil {
		return 0, types.ErrEmptyMessage
	}

	headerInfo, err := k.headersState(ctx).GetHeaderByHash(headerHash)

	if err != nil {
		return 0, err
	}

	return headerInfo.Height, nil
}

// MainChainDepth returns the depth of the header in the main chain, or error if it does not exist
func (k Keeper) MainChainDepth(ctx context.Context, headerHashBytes *bbn.BTCHeaderHashBytes) (uint32, error) {
	if headerHashBytes == nil {
		return 0, types.ErrEmptyMessage
	}
	// Retrieve the header. If it does not exist, return an error
	headerInfo, err := k.headersState(ctx).GetHeaderByHash(headerHashBytes)
	if err != nil {
		return 0, err
	}
	// Retrieve the tip
	tipInfo := k.headersState(ctx).GetTip()

	// sanity check, to avoid silent error if something is wrong.
	if tipInfo.Height < headerInfo.Height {
		// panic, as tip should always be higher than the header than every header
		panic("tip height is less than header height")
	}

	headerDepth := tipInfo.Height - headerInfo.Height
	return headerDepth, nil
}

func (k Keeper) GetTipInfo(ctx context.Context) *types.BTCHeaderInfo {
	return k.headersState(ctx).GetTip()
}

// GetHeaderByHash returns header with given hash, if it does not exists returns nil
func (k Keeper) GetHeaderByHash(ctx context.Context, hash *bbn.BTCHeaderHashBytes) (*types.BTCHeaderInfo, error) {
	return k.headersState(ctx).GetHeaderByHash(hash)
}

// GetHeaderByHeight returns header with given height from main chain, returns nil if such header is not found
func (k Keeper) GetHeaderByHeight(ctx context.Context, height uint32) *types.BTCHeaderInfo {
	header, err := k.headersState(ctx).GetHeaderByHeight(height)

	if err != nil {
		return nil
	}

	return header
}

// GetMainChainFrom returns the current canonical chain from the given height up to the tip
// If the height is higher than the tip, it returns an empty slice
// If startHeight is 0, it returns the entire main chain
func (k Keeper) GetMainChainFrom(ctx context.Context, startHeight uint32) []*types.BTCHeaderInfo {
	headers := make([]*types.BTCHeaderInfo, 0)
	accHeaderFn := func(header *types.BTCHeaderInfo) bool {
		headers = append(headers, header)
		return false
	}
	k.headersState(ctx).IterateForwardHeaders(startHeight, accHeaderFn)
	return headers
}

// GetMainChainFromWithLimit returns the current canonical chain from the given height up to the tip
// If the height is higher than the tip, it returns an empty slice
// If startHeight is 0, it returns the entire main chain
func (k Keeper) GetMainChainFromWithLimit(ctx context.Context, startHeight uint32, limit uint32) []*types.BTCHeaderInfo {
	headers := make([]*types.BTCHeaderInfo, 0, limit)
	fn := func(header *types.BTCHeaderInfo) bool {
		if len(headers) >= int(limit) {
			return true
		}
		headers = append(headers, header)
		return false
	}
	k.headersState(ctx).IterateForwardHeaders(startHeight, fn)
	return headers
}

// GetMainChainUpTo returns the current canonical chain as a collection of block headers
// starting from the tip and ending on the header that has `depth` distance from it.
func (k Keeper) GetMainChainUpTo(ctx context.Context, depth uint32) []*types.BTCHeaderInfo {
	headers := make([]*types.BTCHeaderInfo, 0)

	var currentDepth = uint32(0)
	accHeaderFn := func(header *types.BTCHeaderInfo) bool {
		// header header is at depth 0.
		if currentDepth > depth {
			return true
		}

		headers = append(headers, header)
		currentDepth++
		return false
	}

	k.headersState(ctx).IterateReverseHeaders(accHeaderFn)

	return headers
}

// GetMainChainReverse Retrieves whole header chain in reverse order
func (k Keeper) GetMainChainReverse(ctx context.Context) []*types.BTCHeaderInfo {
	headers := make([]*types.BTCHeaderInfo, 0)
	accHeaderFn := func(header *types.BTCHeaderInfo) bool {
		headers = append(headers, header)
		return false
	}
	k.headersState(ctx).IterateReverseHeaders(accHeaderFn)
	return headers
}

func (k Keeper) GetBTCNet() *chaincfg.Params {
	return k.btcConfig.NetParams()
}
