package v2

import (
	"context"

	"cosmossdk.io/store/prefix"
	storetypes "cosmossdk.io/store/types"
	"github.com/babylonlabs-io/babylon/v3/x/btcstaking/types"
	"github.com/babylonlabs-io/babylon/v3/x/btcstaking/types/allowlist"
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
) error {
	if err := migrateParams(ctx, s, c, modifyParams); err != nil {
		return err
	}

	if err := indexAllowedMultiStakingTxs(ctx, indexTxHash); err != nil {
		return err
	}

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
	txMap := make(map[string]struct{})
	var txCount int
	for _, txHash := range txHashes {
		// Check for duplicates in the allow list
		// If a duplicate is found, log an error and skip indexing
		txHashStr := txHash.String()
		if _, exists := txMap[txHashStr]; exists {
			logger.Error("Duplicate transaction hash found in multi-staking allow list", "tx_hash",
				txHashStr)
			continue
		}
		txMap[txHashStr] = struct{}{}
		indexTxHash(ctx, txHash)
		txCount++
	}
	logger.Info("Indexed multi-staking allow list transactions", "count", txCount)
	return nil
}
