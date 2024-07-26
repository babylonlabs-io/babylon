package keeper

import (
	"context"
	"github.com/babylonlabs-io/babylon/x/checkpointing/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// SetGenBlsKeys registers BLS keys with each validator at genesis
func (k Keeper) SetGenBlsKeys(ctx context.Context, genKeys []*types.GenesisKey) {
	for _, key := range genKeys {
		addr, err := sdk.ValAddressFromBech32(key.ValidatorAddress)
		if err != nil {
			panic(err)
		}
		exists := k.RegistrationState(ctx).Exists(addr)
		if exists {
			panic("a validator's BLS key has already been registered")
		}
		ok := key.BlsKey.Pop.IsValid(*key.BlsKey.Pubkey, key.ValPubkey)
		if !ok {
			panic("Proof-of-Possession is not valid")
		}
		err = k.RegistrationState(ctx).CreateRegistration(*key.BlsKey.Pubkey, addr)
		if err != nil {
			panic("failed to register a BLS key")
		}
	}
}
