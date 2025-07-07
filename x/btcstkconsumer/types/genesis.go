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
	consumersMap, err := getConsumersMap(gs.Consumers)
	if err != nil {
		return err
	}

	for _, fp := range gs.FinalityProviders {
		if err := fp.ValidateBasic(); err != nil {
			return err
		}
		// validate that FP's bsnId is registered
		if _, exists := consumersMap[fp.BsnId]; !exists {
			return fmt.Errorf("finality provider consumer is not registered. Consumer id : %s, BTC pk: %s", fp.BsnId, fp.BtcPk.MarshalHex())
		}
	}

	return gs.Params.Validate()
}

// getConsumersMap validates the consumers
// and returns a map with the consumer ids
func getConsumersMap(consumers []*ConsumerRegister) (map[string]bool, error) {
	consumersMap := make(map[string]bool)
	for _, c := range consumers {
		if _, exists := consumersMap[c.ConsumerId]; exists {
			return consumersMap, fmt.Errorf("duplicate consumer id: %s", c.ConsumerId)
		}
		consumersMap[c.ConsumerId] = true

		if err := c.Validate(); err != nil {
			return consumersMap, err
		}
	}
	return consumersMap, nil
}

// Helper function to sort slices to get a deterministic
// result on the tests
func SortData(gs *GenesisState) {
	sort.Slice(gs.Consumers, func(i, j int) bool {
		return gs.Consumers[i].ConsumerId < gs.Consumers[j].ConsumerId
	})

	sort.Slice(gs.FinalityProviders, func(i, j int) bool {
		return gs.FinalityProviders[i].Addr < gs.FinalityProviders[j].Addr
	})
}
