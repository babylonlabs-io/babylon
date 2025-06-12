package types

import (
	"fmt"
	"sort"

	"github.com/babylonlabs-io/babylon/v3/types"
)

// DefaultGenesis returns the default Capability genesis state
func DefaultGenesis() *GenesisState {
	return &GenesisState{
		Params: DefaultParams(),
	}
}

// NewGenesis creates a new GenesisState instance
func NewGenesis(params Params, epochs []*Epoch, queues []*EpochQueue, valSet, slashedValSet []*EpochValidatorSet, valsLC []*ValidatorLifecycle, delsLC []*DelegationLifecycle) *GenesisState {
	return &GenesisState{
		Params:               params,
		Epochs:               epochs,
		Queues:               queues,
		ValidatorSets:        valSet,
		SlashedValidatorSets: slashedValSet,
		ValidatorsLifecycle:  valsLC,
		DelegationsLifecycle: delsLC,
	}
}

// Validate performs basic genesis state validation returning an error upon any
// failure.
func (gs GenesisState) Validate() error {
	if err := validateEpochs(gs.Epochs); err != nil {
		return err
	}

	if err := types.ValidateEntries(gs.Queues, func(eq *EpochQueue) uint64 { return eq.EpochNumber }); err != nil {
		return err
	}

	if err := types.ValidateEntries(gs.ValidatorSets, func(ev *EpochValidatorSet) uint64 { return ev.EpochNumber }); err != nil {
		return err
	}

	if err := types.ValidateEntries(gs.SlashedValidatorSets, func(v *EpochValidatorSet) uint64 { return v.EpochNumber }); err != nil {
		return err
	}

	if err := types.ValidateEntries(gs.ValidatorsLifecycle, func(v *ValidatorLifecycle) string { return v.ValAddr }); err != nil {
		return err
	}

	if err := types.ValidateEntries(gs.DelegationsLifecycle, func(v *DelegationLifecycle) string { return v.DelAddr }); err != nil {
		return err
	}

	return gs.Params.Validate()
}

func (eq EpochQueue) Validate() error {
	if len(eq.Msgs) == 0 {
		return fmt.Errorf("empty Msgs in epoch queue. EpochNum: %d", eq.EpochNumber)
	}

	for _, e := range eq.Msgs {
		if e.Msg == nil {
			return fmt.Errorf("null Msg in epoch queue. EpochNum: %d", eq.EpochNumber)
		}
	}
	return nil
}

func (ev EpochValidatorSet) Validate() error {
	if len(ev.Validators) == 0 {
		return fmt.Errorf("empty validators. Epoch %d", ev.EpochNumber)
	}
	return types.ValidateEntries(ev.Validators, func(v *Validator) string { return string(v.Addr) })
}

func validateEpochs(epochs []*Epoch) error {
	if len(epochs) == 0 {
		return nil
	}

	epochNumberSet := make(map[uint64]struct{})
	firstBlockHeightSet := make(map[uint64]struct{})
	sealerBlockHashSet := make(map[string]struct{})

	// Collect all epoch numbers for sequential validation
	epochNumbers := make([]uint64, 0, len(epochs))

	for _, e := range epochs {
		if _, exists := epochNumberSet[e.EpochNumber]; exists {
			return fmt.Errorf("duplicate EpochNumber: %d", e.EpochNumber)
		}
		epochNumberSet[e.EpochNumber] = struct{}{}
		epochNumbers = append(epochNumbers, e.EpochNumber)

		if _, exists := firstBlockHeightSet[e.FirstBlockHeight]; exists {
			return fmt.Errorf("duplicate FirstBlockHeight: %d", e.FirstBlockHeight)
		}
		firstBlockHeightSet[e.FirstBlockHeight] = struct{}{}

		if len(e.SealerBlockHash) > 0 {
			hashKey := string(e.SealerBlockHash)
			if _, exists := sealerBlockHashSet[hashKey]; exists {
				return fmt.Errorf("duplicate SealerBlockHash: %x", e.SealerBlockHash)
			}
			sealerBlockHashSet[hashKey] = struct{}{}
		}

		if err := e.ValidateBasic(); err != nil {
			return err
		}
	}

	if err := ValidateSequentialEpochs(epochNumbers); err != nil {
		return err
	}

	return nil
}

// ValidateSequentialEpochs ensures epoch numbers form a consecutive sequence
func ValidateSequentialEpochs(epochNumbers []uint64) error {
	if len(epochNumbers) <= 1 {
		return nil
	}

	sort.Slice(epochNumbers, func(i, j int) bool {
		return epochNumbers[i] < epochNumbers[j]
	})

	for i := 1; i < len(epochNumbers); i++ {
		if epochNumbers[i] != epochNumbers[i-1]+1 {
			return fmt.Errorf("epoch numbers are not consecutive: found gap between %d and %d",
				epochNumbers[i-1], epochNumbers[i])
		}
	}

	return nil
}

// Helper function to sort slices to get a deterministic
// result on the tests
func SortData(gs *GenesisState) {
	sort.Slice(gs.Epochs, func(i, j int) bool {
		return gs.Epochs[i].EpochNumber < gs.Epochs[j].EpochNumber
	})
	sort.Slice(gs.Queues, func(i, j int) bool {
		return gs.Queues[i].EpochNumber < gs.Queues[j].EpochNumber
	})
	sort.Slice(gs.ValidatorSets, func(i, j int) bool {
		return gs.ValidatorSets[i].EpochNumber < gs.ValidatorSets[j].EpochNumber
	})
	sort.Slice(gs.SlashedValidatorSets, func(i, j int) bool {
		return gs.SlashedValidatorSets[i].EpochNumber < gs.SlashedValidatorSets[j].EpochNumber
	})
	sort.Slice(gs.ValidatorsLifecycle, func(i, j int) bool {
		return gs.ValidatorsLifecycle[i].ValAddr < gs.ValidatorsLifecycle[j].ValAddr
	})
	sort.Slice(gs.DelegationsLifecycle, func(i, j int) bool {
		return gs.DelegationsLifecycle[i].DelAddr < gs.DelegationsLifecycle[j].DelAddr
	})
}
