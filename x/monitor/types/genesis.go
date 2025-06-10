package types

import (
	"errors"
	fmt "fmt"
	"sort"

	"github.com/babylonlabs-io/babylon/v3/types"
	chkpttypes "github.com/babylonlabs-io/babylon/v3/x/checkpointing/types"
)

// DefaultGenesis returns the default Capability genesis state
func DefaultGenesis() *GenesisState {
	return &GenesisState{
		EpochEndRecords:     []*EpochEndLightClient{},
		CheckpointsReported: []*CheckpointReportedLightClient{},
	}
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

func (c CheckpointReportedLightClient) Validate() error {
	if len(c.CkptHash) == 0 {
		return errors.New("checkpoint hash cannot be empty")
	}
	if _, err := chkpttypes.FromStringToCkptHash(c.CkptHash); err != nil {
		return fmt.Errorf("invalid hash string %s: %w", c.CkptHash, err)
	}
	return nil
}

// Helper function to sort slices to get a deterministic
// result on the tests
func SortData(gs *GenesisState) {
	sort.Slice(gs.EpochEndRecords, func(i, j int) bool {
		return gs.EpochEndRecords[i].Epoch < gs.EpochEndRecords[j].Epoch
	})
	sort.Slice(gs.CheckpointsReported, func(i, j int) bool {
		return gs.CheckpointsReported[i].CkptHash < gs.CheckpointsReported[j].CkptHash
	})
}
