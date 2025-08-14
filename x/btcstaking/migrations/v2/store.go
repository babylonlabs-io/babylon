package v2

import (
	"context"

	"cosmossdk.io/store/prefix"
	storetypes "cosmossdk.io/store/types"
	"github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
	"github.com/babylonlabs-io/babylon/v4/x/btcstaking/types/allowlist"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// MigrateStore performs in-place store migrations.
// Migration sets every parameter's max finality providers to 1.
func MigrateStore(
	ctx sdk.Context,
	s storetypes.KVStore,
	c codec.BinaryCodec,
	modifyParams func(ctx context.Context, p *types.Params) error,
	indexTxHash func(ctx context.Context, txHash *chainhash.Hash),
	migrateBabylonFinalityProviders func(ctx sdk.Context),
) error {
	if err := migrateParams(ctx, s, c, modifyParams); err != nil {
		return err
	}

	if err := indexAllowedMultiStakingTxs(ctx, indexTxHash); err != nil {
		return err
	}

	migrateBabylonFinalityProviders(ctx)

	return nil
}

func migrateParams(ctx sdk.Context, s storetypes.KVStore, c codec.BinaryCodec, modifyParams func(ctx context.Context, p *types.Params) error) error {
	paramsStore := prefix.NewStore(s, types.ParamsKey)

	iter := paramsStore.Iterator(nil, nil)
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		var sp types.StoredParams
		c.MustUnmarshal(iter.Value(), &sp)
		if err := modifyParams(ctx, &sp.Params); err != nil {
			return err
		}
		paramsStore.Set(iter.Key(), c.MustMarshal(&sp))
	}
	return nil
}

func indexAllowedMultiStakingTxs(
	ctx sdk.Context,
	indexTxHash func(ctx context.Context, txHash *chainhash.Hash),
) error {
	txHashes, err := allowlist.LoadMultiStakingAllowList()
	if err != nil {
		return err
	}
	logger := ctx.Logger().With("module", "x/btcstaking/migrations/v2")
	var txCount int
	for _, txHash := range txHashes {
		indexTxHash(ctx, txHash)
		txCount++
	}
	logger.Info("Indexed multi-staking allow list transactions", "count", txCount)
	return nil
}
