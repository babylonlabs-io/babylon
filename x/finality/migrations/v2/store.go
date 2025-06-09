package v2

import (
	"cosmossdk.io/collections"
	"cosmossdk.io/store/prefix"
	storetypes "cosmossdk.io/store/types"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"

	bbntypes "github.com/babylonlabs-io/babylon/v4/types"
	"github.com/babylonlabs-io/babylon/v4/x/finality/types"
)

// MigrateStore performs in-place store migrations. The
// migration includes adding the PubRandCommit index to achieve
// performance improvements on PubRandCommit lookup
func MigrateStore(ctx sdk.Context, s storetypes.KVStore, cdc codec.BinaryCodec) error {
	store := prefix.NewStore(s, types.PubRandCommitKey)
	iter := store.Iterator(nil, nil)
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		// key is <fpBtcPK><startHeight>
		keyBz := iter.Key()
		fpBtcPK := bbntypes.BIP340PubKey(keyBz[:bbntypes.BIP340PubKeyLen])
		startHeight := sdk.BigEndianToUint64(keyBz[bbntypes.BIP340PubKeyLen:])
		if err := upsertPubRandCommitIdx(s, cdc, fpBtcPK, startHeight); err != nil {
			return err
		}
	}

	return nil
}

func upsertPubRandCommitIdx(s storetypes.KVStore, cdc codec.BinaryCodec, fpBtcPK bbntypes.BIP340PubKey, startHeight uint64) error {
	bytesKey, err := collections.EncodeKeyWithPrefix(types.PubRandCommitIndexKeyPrefix.Bytes(), collections.BytesKey, fpBtcPK.MustMarshal())
	if err != nil {
		return err
	}

	var (
		index    types.PubRandCommitIndexValue
		valueCdc = codec.CollValue[types.PubRandCommitIndexValue](cdc)
		valueBz  = s.Get(bytesKey)
	)
	if valueBz == nil {
		// non-existent, create the index
		index = types.PubRandCommitIndexValue{}
	} else {
		// index exists
		index, err = valueCdc.Decode(valueBz)
		if err != nil {
			return err
		}
	}
	// Append the new height to the index
	index.Heights = append(index.Heights, startHeight)

	updatedValueBz, err := valueCdc.Encode(index)
	if err != nil {
		return err
	}
	s.Set(bytesKey, updatedValueBz)

	return nil
}
