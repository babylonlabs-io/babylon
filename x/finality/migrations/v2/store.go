package v2

import (
	"context"
	"fmt"

	"cosmossdk.io/store/prefix"
	storetypes "cosmossdk.io/store/types"
	bbn "github.com/babylonlabs-io/babylon/v4/types"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/babylonlabs-io/babylon/v4/x/finality/types"
)

// MigrateStore performs in-place store migrations. The
// migration includes adding the PubRandCommit index to achieve
// performance improvements on PubRandCommit lookup
func MigrateStore(ctx sdk.Context, s storetypes.KVStore, upsertPubRandCommitIdx func(ctx context.Context, fpBtcPK *bbn.BIP340PubKey, startHeight uint64) error) error {
	store := prefix.NewStore(s, types.PubRandCommitKey)
	iter := store.Iterator(nil, nil)
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		// key is <fpBtcPK><startHeight>
		keyBz := iter.Key()
		if len(keyBz) <= bbn.BIP340PubKeyLen {
			return fmt.Errorf("store key with smaller length (%d) than expected (>%d)", len(keyBz), bbn.BIP340PubKeyLen)
		}
		fpBtcPK := bbn.BIP340PubKey(keyBz[:bbn.BIP340PubKeyLen])
		startHeight := sdk.BigEndianToUint64(keyBz[bbn.BIP340PubKeyLen:])
		if err := upsertPubRandCommitIdx(ctx, &fpBtcPK, startHeight); err != nil {
			return err
		}
	}

	return nil
}
