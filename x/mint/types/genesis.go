package types

// NewGenesisState creates a new GenesisState object
func NewGenesisState(minter Minter, genTime GenesisTime) *GenesisState {
	return &GenesisState{
		Minter:      &minter,
		GenesisTime: &genTime,
	}
}

// DefaultGenesisState creates a default GenesisState object.
// By leaving GenesisTime as nil, on InitGenesis will be populated with the ctx.BlockTime()
func DefaultGenesisState() *GenesisState {
	dm := DefaultMinter()
	return &GenesisState{
		Minter: &dm,
	}
}

// ValidateGenesis validates the provided genesis state to ensure the
// expected invariants holds.
func (gs GenesisState) Validate() error {
	if gs.Minter != nil {
		if err := gs.Minter.Validate(); err != nil {
			return err
		}
	}
	return nil
}
