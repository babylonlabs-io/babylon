package types

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sort"

	types "github.com/babylonlabs-io/babylon/v3/types"
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

	expDelegatorIdx, err := validateBTCDelegations(gs.BtcDelegations)
	if err != nil {
		return err
	}

	if err := validateDelegatorIdx(gs.BtcDelegators, expDelegatorIdx); err != nil {
		return err
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

	if err := types.ValidateEntries(gs.ConsumerEvents, func(e *ConsumerEvent) string {
		return e.ConsumerId
	}); err != nil {
		return err
	}

	if err := gs.validateAllowedMultiStakingTxHashes(); err != nil {
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
	return validateTxHashes(gs.AllowedStakingTxHashes)
}

// validateAllowedMultiStakingTxHashes validates there're no duplicate entries
// and the hash has the corresponding size
func (gs GenesisState) validateAllowedMultiStakingTxHashes() error {
	return validateTxHashes(gs.AllowedMultiStakingTxHashes)
}

func validateTxHashes(hashes []string) error {
	seen := make(map[string]bool)
	for _, hStr := range hashes {
		if _, exists := seen[hStr]; exists {
			return fmt.Errorf("duplicate staking tx hash: %s", hStr)
		}
		seen[hStr] = true
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
		len(e.Events.UnbondedDel) == 0 {
		return errors.New("empty Events")
	}

	return nil
}

// validateBTCDelegations validates the BTC delegation and returns the
// expected delegation index to compare with the provided on genesis state
func validateBTCDelegations(delegations []*BTCDelegation) (map[string]*BTCDelegatorDelegationIndex, error) {
	// prevent duplicate staking tx hashes
	keyMap := make(map[string]struct{})

	// map from (fpBtcPk || delBtcPk) -> delegation index
	indexMap := make(map[string]*BTCDelegatorDelegationIndex)

	for _, d := range delegations {
		stakingTxHash, err := d.GetStakingTxHash()
		if err != nil {
			return nil, err
		}

		key := stakingTxHash.String()

		if _, exists := keyMap[key]; exists {
			return nil, fmt.Errorf("duplicate entry for key: %v", key)
		}
		keyMap[key] = struct{}{}

		if err := d.ValidateBasic(); err != nil {
			return nil, err
		}

		// create or update index for each (fpBtcPk, delBtcPk) pair
		for _, fpBTCPK := range d.FpBtcPkList {
			mapKey := buildDelegationIndexKey(&fpBTCPK, d.BtcPk)

			idx, ok := indexMap[mapKey]
			if !ok {
				idx = NewBTCDelegatorDelegationIndex()
				indexMap[mapKey] = idx
			}

			if err := idx.Add(stakingTxHash); err != nil {
				return nil, fmt.Errorf("error adding staking tx hash to index: %w", err)
			}
		}
	}

	return indexMap, nil
}

// validateDelegatorIdx validates the provided genesis state delegator index
// with the expected index got from the provided BTC delegations in the genesis state
func validateDelegatorIdx(gsEntries []*BTCDelegator, expIdx map[string]*BTCDelegatorDelegationIndex) error {
	for _, del := range gsEntries {
		if del.FpBtcPk == nil || del.DelBtcPk == nil {
			return fmt.Errorf("missing FpBtcPk or DelBtcPk in BTCDelegator")
		}

		mapKey := buildDelegationIndexKey(del.FpBtcPk, del.DelBtcPk)

		expectedIdx, exists := expIdx[mapKey]
		if !exists {
			return fmt.Errorf("expected index for key (fpBtcPk::delBtcPk) %s not found", mapKey)
		}

		if expectedIdx.String() != del.Idx.String() {
			return fmt.Errorf("mismatched index for key %s: expected %s, got  %s", mapKey, expectedIdx, del.Idx)
		}
		if err := del.Validate(); err != nil {
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

	sort.Slice(gs.ConsumerEvents, func(i, j int) bool {
		return gs.ConsumerEvents[i].ConsumerId < gs.ConsumerEvents[j].ConsumerId
	})

	slices.Sort(gs.AllowedStakingTxHashes)
	slices.Sort(gs.AllowedMultiStakingTxHashes)
}

func buildDelegationIndexKey(fp, del *types.BIP340PubKey) string {
	return fp.MarshalHex() + "::" + del.MarshalHex()
}
