package types

import (
	"errors"
	"fmt"
	"sort"
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
	epochsMap, err := validateEpochs(gs.Epochs)
	if err != nil {
		return err
	}
	if err := validateSubmissions(gs.Submissions, epochsMap); err != nil {
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

func validateEpochs(epochs []EpochEntry) (map[uint64]struct{}, error) {
	epochsMap := make(map[uint64]struct{})
	for _, e := range epochs {
		key := e.EpochNumber
		if _, exists := epochsMap[key]; exists {
			return nil, fmt.Errorf("duplicate entry for key: %v", key)
		}
		epochsMap[key] = struct{}{}

		if err := e.Validate(); err != nil {
			return nil, err
		}
	}
	return epochsMap, nil
}

func validateSubmissions(submissions []SubmissionEntry, epochsMap map[uint64]struct{}) error {
	keyMap := make(map[*SubmissionKey]struct{})
	for _, s := range submissions {
		if err := s.Validate(); err != nil {
			return err
		}

		key := s.SubmissionKey
		if _, exists := keyMap[key]; exists {
			return fmt.Errorf("duplicate entry for key: %v", key)
		}
		keyMap[key] = struct{}{}

		// check epoch exists
		if _, epochExists := epochsMap[s.Data.Epoch]; !epochExists {
			return fmt.Errorf("epoch with number %d not found in genesis", s.Data.Epoch)
		}
	}
	return nil
}

func (e EpochEntry) Validate() error {
	return e.Data.Validate()
}

func (s SubmissionEntry) Validate() error {
	if s.SubmissionKey == nil {
		return errors.New("invalid SubmissionEntry. SubmissionKey is nil")
	}
	if err := s.SubmissionKey.Validate(); err != nil {
		return err
	}
	if s.Data == nil {
		return errors.New("invalid SubmissionEntry. Data is nil")
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
