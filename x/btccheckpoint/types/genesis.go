package types

import (
	fmt "fmt"
	"sort"

	"github.com/babylonlabs-io/babylon/types"
)

// DefaultGenesis returns the default Capability genesis state
func DefaultGenesis() *GenesisState {
	return &GenesisState{
		Params:                   DefaultParams(),
		LastFinalizedEpochNumber: 0,
		Epochs:                   []EpochEntry{},
		Submissions:              []SubmissionEntry{},
	}
}

// Validate performs basic genesis state validation returning an error upon any
// failure.
func (gs GenesisState) Validate() error {
	if err := validateEpochs(gs.Epochs); err != nil {
		return err
	}
	if err := validateSubmissions(gs.Submissions); err != nil {
		return err
	}
	// Ensure LastFinalizedEpochNumber is <= max epoch number
	var maxEpochNumber uint64
	for _, epoch := range gs.Epochs {
		if epoch.EpochNumber > maxEpochNumber {
			maxEpochNumber = epoch.EpochNumber
		}
	}
	if gs.LastFinalizedEpochNumber > maxEpochNumber {
		return fmt.Errorf("last finalized epoch number (%d) cannot be greater than the highest epoch number (%d)",
			gs.LastFinalizedEpochNumber, maxEpochNumber)
	}
	return gs.Params.Validate()
}

func validateEpochs(epochs []EpochEntry) error {
	return types.ValidateEntries(epochs, func(e EpochEntry) uint64 {
		return e.EpochNumber
	})
}

func validateSubmissions(submissions []SubmissionEntry) error {
	return types.ValidateEntries(submissions, func(s SubmissionEntry) *SubmissionKey {
		return s.SubmissionKey
	})
}

func (e EpochEntry) Validate() error {
	return e.Data.Validate()
}

func (s SubmissionEntry) Validate() error {
	if err := s.SubmissionKey.Validate(); err != nil {
		return err
	}
	return s.Data.Validate()
}

// Helper function to sort slices to get a deterministic
// result on the tests
func SortData(gs *GenesisState) {
	sort.Slice(gs.Epochs, func(i, j int) bool {
		return gs.Epochs[i].EpochNumber < gs.Epochs[j].EpochNumber
	})
	sort.Slice(gs.Submissions, func(i, j int) bool {
		return gs.Submissions[i].SubmissionKey.Key[0].Hash.String() < gs.Submissions[j].SubmissionKey.Key[0].Hash.String()
	})
}
