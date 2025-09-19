package keeper

import (
	"context"
	"fmt"

	"cosmossdk.io/store/prefix"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/babylonlabs-io/babylon/v4/crypto/bls12381"
	"github.com/babylonlabs-io/babylon/v4/x/checkpointing/types"
)

// SetGenBlsKeys registers BLS keys with each validator at genesis
func (k Keeper) SetGenBlsKeys(ctx context.Context, genKeys []*types.GenesisKey) error {
	for _, key := range genKeys {
		addr, err := sdk.ValAddressFromBech32(key.ValidatorAddress)
		if err != nil {
			return err
		}
		exists := k.RegistrationState(ctx).Exists(addr)
		if exists {
			return fmt.Errorf("a validator's BLS key has already been registered. Duplicate address: %s", key.ValidatorAddress)
		}
		ok := key.BlsKey.Pop.IsValid(*key.BlsKey.Pubkey, key.ValPubkey)
		if !ok {
			return fmt.Errorf("Proof-of-Possession is not valid. Pop: %s", key.BlsKey.Pop.String())
		}
		err = k.RegistrationState(ctx).CreateRegistration(*key.BlsKey.Pubkey, addr)
		if err != nil {
			return fmt.Errorf("failed to register a BLS key. Val Addr: %s", key.ValidatorAddress)
		}
	}
	return nil
}

// GetBlsKeys gets all the BLS keys stored.
// This function is called in ExportGenesis
// NOTE: validator ed25519 pub key and PoP are not stored in the module
// but used on InitGenesis for validation. Make sure to populate these fields
// before using the exported data as input in the InitGenesis logic.
func (k Keeper) GetBlsKeys(ctx context.Context) ([]*types.GenesisKey, error) {
	genKeys := make([]*types.GenesisKey, 0)
	storeAdapter := runtime.KVStoreAdapter(k.storeService.OpenKVStore(ctx))
	store := prefix.NewStore(storeAdapter, types.BlsKeyToAddrPrefix)

	iter := store.Iterator(nil, nil)
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		blsKey := new(bls12381.PublicKey)
		if err := blsKey.Unmarshal(iter.Key()); err != nil {
			return nil, err
		}
		valAddr := sdk.ValAddress(iter.Value())

		genKeys = append(genKeys, &types.GenesisKey{
			ValidatorAddress: valAddr.String(),
			BlsKey: &types.BlsKey{
				Pubkey: blsKey,
			},
		})
	}

	return genKeys, nil
}
