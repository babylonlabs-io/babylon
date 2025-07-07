package types

import (
	"fmt"
	"sort"
)

// DefaultGenesis returns the default genesis state
func DefaultGenesis() *GenesisState {
	return &GenesisState{
		Params: DefaultParams(),
	}
}

// Validate performs basic genesis state validation returning an error upon any
// failure.
func (gs GenesisState) Validate() error {
	err := validateConsumers(gs.Consumers)
	if err != nil {
		return err
	}
	return gs.Params.Validate()
}

// validateConsumers validates the consumers
func validateConsumers(consumers []*ConsumerRegister) error {
	consumersMap := make(map[string]bool)
	for _, c := range consumers {
		if _, exists := consumersMap[c.ConsumerId]; exists {
			return fmt.Errorf("duplicate consumer id: %s", c.ConsumerId)
		}
		consumersMap[c.ConsumerId] = true

		if err := c.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// Helper function to sort slices to get a deterministic
// result on the tests
func SortData(gs *GenesisState) {
	sort.Slice(gs.Consumers, func(i, j int) bool {
		return gs.Consumers[i].ConsumerId < gs.Consumers[j].ConsumerId
	})
}
