package types

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sort"

	types "github.com/babylonlabs-io/babylon/v4/types"
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

	if gs.LargestBtcReorg != nil {
		if err := gs.LargestBtcReorg.Validate(); err != nil {
			return err
		}
	}

	for _, d := range gs.BtcConsumerDelegators {
		if err := d.Validate(); err != nil {
			return err
		}
	}

	if err := types.ValidateEntries(gs.ConsumerEvents, func(e *ConsumerEvent) string {
		return e.ConsumerId
	}); err != nil {
		return err
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

func (d BTCDelegator) Validate() error {
	if d.FpBtcPk == nil {
		return errors.New("null FP BTC PubKey")
	}

	if d.DelBtcPk == nil {
		return errors.New("null Delegator BTC PubKey")
	}

	if d.Idx == nil {
		return errors.New("null Index")
	}

	// validate BIP340PubKey length
	if d.FpBtcPk.Size() != types.BIP340PubKeyLen {
		return fmt.Errorf("invalid FP BTC PubKey. Expected length %d, got %d", types.BIP340PubKeyLen, d.FpBtcPk.Size())
	}

	if d.DelBtcPk.Size() != types.BIP340PubKeyLen {
		return fmt.Errorf("invalid Delegator BTC PubKey. Expected length %d, got %d", types.BIP340PubKeyLen, d.DelBtcPk.Size())
	}

	return d.Idx.Validate()
}

func (e ConsumerEvent) Validate() error {
	if e.ConsumerId == "" {
		return errors.New("empty Consumer ID")
	}

	if e.Events == nil {
		return errors.New("null Events")
	}

	if len(e.Events.NewFp) == 0 &&
		len(e.Events.ActiveDel) == 0 &&
		len(e.Events.SlashedDel) == 0 &&
		len(e.Events.UnbondedDel) == 0 {
		return errors.New("empty Events")
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

	sort.Slice(gs.BtcConsumerDelegators, func(i, j int) bool {
		return gs.BtcConsumerDelegators[i].String() < gs.BtcConsumerDelegators[j].String()
	})

	sort.Slice(gs.ConsumerEvents, func(i, j int) bool {
		return gs.ConsumerEvents[i].ConsumerId < gs.ConsumerEvents[j].ConsumerId
	})

	slices.Sort(gs.AllowedStakingTxHashes)
}
