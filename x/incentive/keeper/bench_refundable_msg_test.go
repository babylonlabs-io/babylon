package keeper_test

import (
	"context"
	"fmt"
	"testing"

	protov2 "google.golang.org/protobuf/proto"

	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"

	dbm "github.com/cosmos/cosmos-db"

	"cosmossdk.io/collections"
	"cosmossdk.io/log"
	"cosmossdk.io/store"
	storemetrics "cosmossdk.io/store/metrics"
	storetypes "cosmossdk.io/store/types"

	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"

	keepertest "github.com/babylonlabs-io/babylon/v4/testutil/keeper"
	ftypes "github.com/babylonlabs-io/babylon/v4/x/finality/types"
	"github.com/babylonlabs-io/babylon/v4/x/incentive/keeper"
	"github.com/babylonlabs-io/babylon/v4/x/incentive/types"
)

// optimized approach of refundable msgs
func BenchmarkRefundTxDecorator_InMemoryCounter(b *testing.B) {
	iKeeper, ctx := keepertest.IncentiveKeeper(b, nil, nil, nil, nil)
	tKey := storetypes.NewTransientStoreKey("test_transient")
	rtd := keeper.NewRefundTxDecorator(iKeeper, tKey)
	txs := generateDummyRefundableTxs(100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, tx := range txs {
			for range tx.GetMsgs() {
				iKeeper.IncRefundableMsgCount()
			}
			rtd.CheckTxAndClearIndex(ctx, tx)
		}
	}
}

// original approach of refundable msgs
func BenchmarkRefundTxDecorator_KeySetWithKVStore(b *testing.B) {
	ctx, kvKey := setupSdkContext()
	approach := NewCollectionsKVStoreApproach(kvKey)
	txs := generateDummyRefundableTxs(100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, tx := range txs {
			for _, msg := range tx.GetMsgs() {
				msgHash := types.HashMsg(msg)
				approach.IndexRefundableMsg(ctx, msgHash)
			}

			for _, msg := range tx.GetMsgs() {
				msgHash := types.HashMsg(msg)
				if approach.HasRefundableMsg(ctx, msgHash) {
					approach.RemoveRefundableMsg(ctx, msgHash)
				}
			}
		}
	}
}

var RefundableMsgKeySetPrefix = collections.NewPrefix(5) // key prefix for refundable msg key set

type CollectionsKVStoreApproach struct {
	refundableMsgKeySet collections.KeySet[[]byte]
}

func NewCollectionsKVStoreApproach(kvKey *storetypes.KVStoreKey) *CollectionsKVStoreApproach {
	kvStoreService := runtime.NewKVStoreService(kvKey)
	sb := collections.NewSchemaBuilderFromAccessor(kvStoreService.OpenKVStore)

	refundableMsgKeySet := collections.NewKeySet(
		sb,
		RefundableMsgKeySetPrefix,
		"refundable_msg_keyset_kv",
		collections.BytesKey,
	)

	_, err := sb.Build()
	if err != nil {
		panic(err) // Should not happen in benchmark
	}

	return &CollectionsKVStoreApproach{
		refundableMsgKeySet: refundableMsgKeySet,
	}
}

func (a *CollectionsKVStoreApproach) IndexRefundableMsg(ctx context.Context, msgHash []byte) {
	err := a.refundableMsgKeySet.Set(ctx, msgHash)
	if err != nil {
		panic(err) // Should not happen in benchmark
	}
}

func (a *CollectionsKVStoreApproach) HasRefundableMsg(ctx context.Context, msgHash []byte) bool {
	has, err := a.refundableMsgKeySet.Has(ctx, msgHash)
	if err != nil {
		panic(err) // Should not happen in benchmark
	}
	return has
}

func (a *CollectionsKVStoreApproach) RemoveRefundableMsg(ctx context.Context, msgHash []byte) {
	err := a.refundableMsgKeySet.Remove(ctx, msgHash)
	if err != nil {
		panic(err) // Should not happen in benchmark
	}
}

func setupSdkContext() (sdk.Context, *storetypes.KVStoreKey) {
	db := dbm.NewMemDB()
	cms := store.NewCommitMultiStore(db, log.NewNopLogger(), storemetrics.NewNoOpMetrics())
	kvKey := storetypes.NewKVStoreKey("test_kv")

	cms.MountStoreWithDB(kvKey, storetypes.StoreTypeIAVL, db)
	err := cms.LoadLatestVersion()
	if err != nil {
		panic(err)
	}

	cmtHeader := cmtproto.Header{}
	ctx := sdk.NewContext(cms, cmtHeader, false, log.NewNopLogger())
	return ctx, kvKey
}

// mockTx implements sdk.Tx interface for benchmarking
type mockTx struct {
	msgs []sdk.Msg
}

func (tx mockTx) GetMsgs() []sdk.Msg {
	return tx.msgs
}

func (tx mockTx) GetMsgsV2() ([]protov2.Message, error) {
	// For benchmarking, we can return empty slice
	// as this method is not used in our benchmark logic
	return nil, nil
}

func (tx mockTx) ValidateBasic() error {
	return nil
}

func generateDummyRefundableTxs(count int) []sdk.Tx {
	var txs []sdk.Tx
	for i := 0; i < count; i++ {
		msgs := []sdk.Msg{
			&ftypes.MsgAddFinalitySig{
				Signer: fmt.Sprintf("signer_%d", i),
			},
		}

		tx := mockTx{msgs: msgs}
		txs = append(txs, tx)
	}
	return txs
}
