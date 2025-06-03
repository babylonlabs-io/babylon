package types

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"sort"

	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/cosmos/cosmos-sdk/codec"
)

// DefaultGenesis returns the default genesis state
func DefaultGenesis() *GenesisState {
	p := DefaultParams()
	return &GenesisState{
		Params: []*Params{&p},
	}
}

// Validate performs basic genesis state validation returning an error upon any
// failure.
func (gs GenesisState) Validate() error {
	if len(gs.Params) == 0 {
		return fmt.Errorf("params cannot be empty")
	}

	heightToVersionMap := NewHeightToVersionMap()
	for i, params := range gs.Params {
		if err := params.Validate(); err != nil {
			return err
		}

		err := heightToVersionMap.AddNewPair(
			uint64(params.BtcActivationHeight),
			uint32(i),
		)

		if err != nil {
			return err
		}
	}

<<<<<<< HEAD
=======
	if err := types.ValidateEntries(gs.FinalityProviders, func(f *FinalityProvider) string {
		return f.BtcPk.MarshalHex()
	}); err != nil {
		return err
	}

	for _, d := range gs.BtcDelegations {
		if err := d.ValidateBasic(); err != nil {
			return err
		}
	}

	for _, d := range gs.BtcDelegators {
		if err := d.Validate(); err != nil {
			return err
		}
	}

	if err := types.ValidateEntries(gs.BlockHeightChains, func(bh *BlockHeightBbnToBtc) uint64 {
		return bh.BlockHeightBbn
	}); err != nil {
		return err
	}

	if err := types.ValidateEntries(gs.Events, func(e *EventIndex) string {
		return fmt.Sprintf("%d-%d", e.BlockHeightBtc, e.Idx)
	}); err != nil {
		return err
	}

>>>>>>> c4ac498 (chore(btcstaking): update genesis & validations (#1046))
	if gs.LargestBtcReorg != nil {
		if err := gs.LargestBtcReorg.Validate(); err != nil {
			return err
		}
	}

	return gs.validateAllowedStakingTxHashes()
}

// GenesisStateFromAppState returns x/btcstaking GenesisState given raw application
// genesis state.
func GenesisStateFromAppState(cdc codec.Codec, appState map[string]json.RawMessage) GenesisState {
	var genesisState GenesisState

	if appState[ModuleName] != nil {
		cdc.MustUnmarshalJSON(appState[ModuleName], &genesisState)
	}

	return genesisState
}

type AllowedStakingTxHashStr string

func (h AllowedStakingTxHashStr) Validate() error {
	// hashes are hex encoded for better readability
	bz, err := hex.DecodeString(string(h))
	if err != nil {
		return fmt.Errorf("error decoding tx hash: %w", err)
	}
	// NewHash validates hash size
	if _, err := chainhash.NewHash(bz); err != nil {
		return err
	}
	return nil
}

// validateAllowedStakingTxHashes validates there're no duplicate entries
// and the hash has the corresponding size
func (gs GenesisState) validateAllowedStakingTxHashes() error {
	hashes := make(map[string]bool)
	for _, hStr := range gs.AllowedStakingTxHashes {
		if _, exists := hashes[hStr]; exists {
			return fmt.Errorf("duplicate staking tx hash: %s", hStr)
		}
		hashes[hStr] = true
		h := AllowedStakingTxHashStr(hStr)
		if err := h.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// Helper function to sort slices to get a deterministic
// result on the tests
func SortData(gs *GenesisState) {
	sort.Slice(gs.FinalityProviders, func(i, j int) bool {
		return gs.FinalityProviders[i].Addr < gs.FinalityProviders[j].Addr
	})

	sort.Slice(gs.BtcDelegations, func(i, j int) bool {
		return gs.BtcDelegations[i].StakerAddr < gs.BtcDelegations[j].StakerAddr
	})

	sort.Slice(gs.BlockHeightChains, func(i, j int) bool {
		return gs.BlockHeightChains[i].BlockHeightBbn < gs.BlockHeightChains[j].BlockHeightBbn
	})

	sort.Slice(gs.BtcDelegators, func(i, j int) bool {
		return gs.BtcDelegators[i].String() < gs.BtcDelegators[j].String()
	})

	sort.Slice(gs.Events, func(i, j int) bool {
		return gs.Events[i].Idx < gs.Events[j].Idx
	})

	slices.Sort(gs.AllowedStakingTxHashes)
}
