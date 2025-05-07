package types

import (
	"errors"

	"github.com/babylonlabs-io/babylon/v2/types"
)

// DefaultGenesis returns the default Capability genesis state
func DefaultGenesis() *GenesisState {
	return &GenesisState{}
}

// Validate performs basic genesis state validation returning an error upon any
// failure.
func (gs GenesisState) Validate() error {
	if len(gs.EpochEndRecords) > 0 {
		if err := types.ValidateEntries(gs.EpochEndRecords, func(e *EpochEndLightClient) uint64 { return e.Epoch }); err != nil {
			return err
		}
	}
	if len(gs.CheckpointsReported) > 0 {
		if err := types.ValidateEntries(gs.CheckpointsReported, func(c *CheckpointReportedLightClient) string { return c.CkptHash }); err != nil {
			return err
		}
	}
	return nil
}

func (e EpochEndLightClient) Validate() error {
	if e.BtcLightClientHeight == 0 {
		return errors.New("BTC light client height cannot be 0")
	}
	return nil
}

func (c CheckpointReportedLightClient) Validate() error {
	if c.BtcLightClientHeight == 0 {
		return errors.New("BTC light client height cannot be 0")
	}
	if len(c.CkptHash) == 0 {
		return errors.New("checkpoint hash cannot be empty")
	}
	return nil
}
