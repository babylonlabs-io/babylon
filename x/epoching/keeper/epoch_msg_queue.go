package keeper

import (
	"context"
	"fmt"

	errorsmod "cosmossdk.io/errors"
	"cosmossdk.io/store/prefix"
	storetypes "cosmossdk.io/store/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/gogoproto/proto"

	"github.com/babylonlabs-io/babylon/x/epoching/types"
)

// InitMsgQueue initialises the msg queue length of the current epoch to 0
func (k Keeper) InitMsgQueue(ctx context.Context) {
	store := k.msgQueueLengthStore(ctx)

	epochNumber := k.GetEpoch(ctx).EpochNumber
	epochNumberBytes := sdk.Uint64ToBigEndian(epochNumber)
	queueLenBytes := sdk.Uint64ToBigEndian(0)
	store.Set(epochNumberBytes, queueLenBytes)
}

// GetQueueLength fetches the number of queued messages of a given epoch
func (k Keeper) GetQueueLength(ctx context.Context, epochNumber uint64) uint64 {
	store := k.msgQueueLengthStore(ctx)
	epochNumberBytes := sdk.Uint64ToBigEndian(epochNumber)

	// get queue len in bytes from DB
	bz := store.Get(epochNumberBytes)
	if bz == nil {
		return 0 // BBN has not reached this epoch yet
	}
	// unmarshal
	return sdk.BigEndianToUint64(bz)
}

// GetCurrentQueueLength fetches the number of queued messages of the current epoch
func (k Keeper) GetCurrentQueueLength(ctx context.Context) uint64 {
	epochNumber := k.GetEpoch(ctx).EpochNumber
	return k.GetQueueLength(ctx, epochNumber)
}

// incCurrentQueueLength adds the queue length of the current epoch by 1
func (k Keeper) incCurrentQueueLength(ctx context.Context) {
	store := k.msgQueueLengthStore(ctx)

	epochNumber := k.GetEpoch(ctx).EpochNumber
	epochNumberBytes := sdk.Uint64ToBigEndian(epochNumber)

	queueLen := k.GetQueueLength(ctx, epochNumber)
	incrementedQueueLen := queueLen + 1
	incrementedQueueLenBytes := sdk.Uint64ToBigEndian(incrementedQueueLen)

	store.Set(epochNumberBytes, incrementedQueueLenBytes)
}

// EnqueueMsg enqueues a message to the queue of the current epoch
func (k Keeper) EnqueueMsg(ctx context.Context, msg types.QueuedMessage) {
	epochNumber := k.GetEpoch(ctx).EpochNumber
	store := k.msgQueueStore(ctx, epochNumber)

	// key: index, in this case = queueLenBytes
	queueLen := k.GetCurrentQueueLength(ctx)
	queueLenBytes := sdk.Uint64ToBigEndian(queueLen)
	// value: msgBytes
	msgBytes, err := k.cdc.MarshalInterface(&msg)
	if err != nil {
		panic(errorsmod.Wrap(types.ErrMarshal, err.Error()))
	}
	store.Set(queueLenBytes, msgBytes)

	// increment queue length
	k.incCurrentQueueLength(ctx)
}

// GetEpochMsgs returns the set of messages queued in a given epoch
func (k Keeper) GetEpochMsgs(ctx context.Context, epochNumber uint64) []*types.QueuedMessage {
	queuedMsgs := []*types.QueuedMessage{}
	store := k.msgQueueStore(ctx, epochNumber)

	// add each queued msg to queuedMsgs
	iterator := storetypes.KVStorePrefixIterator(store, nil)
	defer iterator.Close()
	for ; iterator.Valid(); iterator.Next() {
		queuedMsgBytes := iterator.Value()
		var sdkMsg sdk.Msg
		if err := k.cdc.UnmarshalInterface(queuedMsgBytes, &sdkMsg); err != nil {
			panic(errorsmod.Wrap(types.ErrUnmarshal, err.Error()))
		}
		queuedMsg, ok := sdkMsg.(*types.QueuedMessage)
		if !ok {
			panic("invalid queued message")
		}
		queuedMsgs = append(queuedMsgs, queuedMsg)
	}

	return queuedMsgs
}

// GetCurrentEpochMsgs returns the set of messages queued in the current epoch
func (k Keeper) GetCurrentEpochMsgs(ctx context.Context) []*types.QueuedMessage {
	epochNumber := k.GetEpoch(ctx).EpochNumber
	return k.GetEpochMsgs(ctx, epochNumber)
}

// HandleQueuedMsg unwraps a QueuedMessage and forwards it to the staking module
func (k Keeper) HandleQueuedMsg(goCtx context.Context, qMsg *types.QueuedMessage) (*sdk.Result, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	res, err := k.runUnwrappedMsg(ctx, qMsg)

	return sdk.WrapServiceResult(ctx, res, err)
}

func (k Keeper) runUnwrappedMsg(ctx sdk.Context, qMsg *types.QueuedMessage) (proto.Message, error) {
	msgSvrCtx, msCache := cacheTxContext(ctx, qMsg.TxId, qMsg.MsgId, qMsg.BlockHeight)

	// record lifecycle for delegation
	switch msg := qMsg.Msg.(type) {
	case *types.QueuedMessage_MsgCreateValidator:
		res, err := k.stkMsgServer.CreateValidator(msgSvrCtx, msg.MsgCreateValidator)
		if err != nil {
			return res, err
		}
		msCache.Write()

		// handle self-delegation
		// Delegator and Validator address are the same
		delAddr, valAddr, err := parseDelValAddr(msg.MsgCreateValidator.ValidatorAddress, msg.MsgCreateValidator.ValidatorAddress)
		if err != nil {
			return nil, err
		}
		amount := &msg.MsgCreateValidator.Value
		// self-bonded to the created validator
		if err := k.RecordNewDelegationState(ctx, delAddr, valAddr, amount, types.BondState_CREATED); err != nil {
			return nil, err
		}
		if err := k.RecordNewDelegationState(ctx, delAddr, valAddr, amount, types.BondState_BONDED); err != nil {
			return nil, err
		}
		return res, nil

	case *types.QueuedMessage_MsgDelegate:
		res, err := k.stkMsgServer.Delegate(msgSvrCtx, msg.MsgDelegate)
		if err != nil {
			return nil, err
		}
		msCache.Write()

		del, val, err := parseDelValAddr(msg.MsgDelegate.DelegatorAddress, msg.MsgDelegate.ValidatorAddress)
		if err != nil {
			return nil, err
		}
		amount := &msg.MsgDelegate.Amount
		// created and bonded to the validator
		if err := k.RecordNewDelegationState(ctx, del, val, amount, types.BondState_CREATED); err != nil {
			return nil, err
		}
		if err := k.RecordNewDelegationState(ctx, del, val, amount, types.BondState_BONDED); err != nil {
			return nil, err
		}
		return res, nil

	case *types.QueuedMessage_MsgUndelegate:
		res, err := k.stkMsgServer.Undelegate(msgSvrCtx, msg.MsgUndelegate)
		if err != nil {
			return nil, err
		}
		msCache.Write()

		del, val, err := parseDelValAddr(msg.MsgUndelegate.DelegatorAddress, msg.MsgUndelegate.ValidatorAddress)
		if err != nil {
			return nil, err
		}
		amount := &msg.MsgUndelegate.Amount
		// unbonding from the validator
		// (in `ApplyMatureUnbonding`) AFTER mature, unbonded from the validator
		if err := k.RecordNewDelegationState(ctx, del, val, amount, types.BondState_UNBONDING); err != nil {
			return nil, err
		}
		return res, nil

	case *types.QueuedMessage_MsgBeginRedelegate:
		res, err := k.stkMsgServer.BeginRedelegate(msgSvrCtx, msg.MsgBeginRedelegate)
		if err != nil {
			return nil, err
		}
		msCache.Write()

		del, srcVal, err := parseDelValAddr(msg.MsgBeginRedelegate.DelegatorAddress, msg.MsgBeginRedelegate.ValidatorSrcAddress)
		if err != nil {
			return nil, err
		}
		amount := &msg.MsgBeginRedelegate.Amount
		// unbonding from the source validator
		// (in `ApplyMatureUnbonding`) AFTER mature, unbonded from the source validator, created/bonded to the destination validator
		if err := k.RecordNewDelegationState(ctx, del, srcVal, amount, types.BondState_UNBONDING); err != nil {
			return nil, err
		}
		return res, nil

	case *types.QueuedMessage_MsgCancelUnbondingDelegation:
		res, err := k.stkMsgServer.CancelUnbondingDelegation(msgSvrCtx, msg.MsgCancelUnbondingDelegation)
		if err != nil {
			return nil, err
		}
		msCache.Write()

		del, val, err := parseDelValAddr(msg.MsgCancelUnbondingDelegation.DelegatorAddress, msg.MsgCancelUnbondingDelegation.ValidatorAddress)
		if err != nil {
			return nil, err
		}
		amount := &msg.MsgCancelUnbondingDelegation.Amount
		// this delegation is now bonded again
		if err := k.RecordNewDelegationState(ctx, del, val, amount, types.BondState_BONDED); err != nil {
			return nil, err
		}
		return res, nil

	case *types.QueuedMessage_MsgEditValidator:
		res, err := k.stkMsgServer.EditValidator(msgSvrCtx, msg.MsgEditValidator)
		if err != nil {
			return res, err
		}
		msCache.Write()
		return res, nil

	case *types.QueuedMessage_MsgUpdateParams:
		res, err := k.stkMsgServer.UpdateParams(msgSvrCtx, msg.MsgUpdateParams)
		if err != nil {
			return res, err
		}
		msCache.Write()
		return res, nil

	default:
		panic(errorsmod.Wrap(types.ErrInvalidQueuedMessageType, qMsg.String()))
	}
}

func parseDelValAddr(delAddr, valAddr string) (sdk.AccAddress, sdk.ValAddress, error) {
	del, err := sdk.AccAddressFromBech32(delAddr)
	if err != nil {
		return nil, nil, err
	}
	val, err := sdk.ValAddressFromBech32(valAddr)
	if err != nil {
		return nil, nil, err
	}
	return del, val, nil
}

// msgQueueStore returns the queue of msgs of a given epoch
// prefix: MsgQueueKey || epochNumber
// key: index
// value: msg
func (k Keeper) msgQueueStore(ctx context.Context, epochNumber uint64) prefix.Store {
	storeAdapter := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	msgQueueStore := prefix.NewStore(storeAdapter, types.MsgQueueKey)
	epochNumberBytes := sdk.Uint64ToBigEndian(epochNumber)
	return prefix.NewStore(msgQueueStore, epochNumberBytes)
}

// msgQueueLengthStore returns the length of the msg queue of a given epoch
// prefix: QueueLengthKey
// key: epochNumber
// value: queue length
func (k Keeper) msgQueueLengthStore(ctx context.Context) prefix.Store {
	storeAdapter := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	return prefix.NewStore(storeAdapter, types.QueueLengthKey)
}

// based on a function with the same name in `baseapp.go`
func cacheTxContext(ctx sdk.Context, txid []byte, msgid []byte, height uint64) (sdk.Context, storetypes.CacheMultiStore) {
	ms := ctx.MultiStore()
	// TODO: https://github.com/cosmos/cosmos-sdk/issues/2824
	msCache := ms.CacheMultiStore()
	if msCache.TracingEnabled() {
		msCache = msCache.SetTracingContext(
			map[string]interface{}{
				"txHash":  fmt.Sprintf("%X", txid),
				"msgHash": fmt.Sprintf("%X", msgid),
				"height":  fmt.Sprintf("%d", height),
			},
		).(storetypes.CacheMultiStore)
	}

	return ctx.WithMultiStore(msCache), msCache
}
