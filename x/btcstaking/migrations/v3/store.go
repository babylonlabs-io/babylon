package v3

import (
	"context"

	storetypes "cosmossdk.io/store/types"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// MigrateStore performs in-place store migrations.
// Migration removes deprecated AllowedStakingTxHashesKeySet and AllowedMultiStakingTxHashesKeySet data.
func MigrateStore(
	ctx sdk.Context,
	s storetypes.KVStore,
	c codec.BinaryCodec,
	removeAllowListsRecords func(ctx context.Context) error,
) error {
	// Clear all entries from both KeySets (nil ranger = clear all)
	if err := removeAllowListsRecords(ctx); err != nil {
		return err
	}

	return nil
}
