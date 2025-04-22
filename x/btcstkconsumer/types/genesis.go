package types

import "fmt"

// DefaultGenesis returns the default genesis state
func DefaultGenesis() *GenesisState {
	return &GenesisState{
		Params: DefaultParams(),
	}
}

// Validate performs basic genesis state validation returning an error upon any
// failure.
func (gs GenesisState) Validate() error {
	consumersMap, err := validateConsumers(gs.Consumers)
	if err != nil {
		return err
	}

	for _, fp := range gs.FinalityProviders {
		if err := fp.ValidateBasic(); err != nil {
			return err
		}
		// validate that FP's consumerId is registered
		if _, exists := consumersMap[fp.ConsumerId]; !exists {
			return fmt.Errorf("finality provider consumer is not registered. Consumer id : %s, BTC pk: %s", fp.ConsumerId, fp.BtcPk.MarshalHex())
		}
	}

	return gs.Params.Validate()
}

// validateConsumers validates the consumers
// and returns a map with the consumer ids
func validateConsumers(consumers []*ConsumerRegister) (map[string]bool, error) {
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
