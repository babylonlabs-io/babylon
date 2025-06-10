package types

import (
	"errors"
	"sort"
	"strconv"

	"github.com/babylonlabs-io/babylon/v3/types"
	host "github.com/cosmos/ibc-go/v8/modules/core/24-host"
)

// DefaultGenesis returns the default genesis state
func DefaultGenesis() *GenesisState {
	return &GenesisState{
		PortId: PortID,
		Params: DefaultParams(),
	}
}

// NewGenesis creates a new GenesisState instance
func NewGenesis(
	params Params,
	chainsInfo []*ChainInfo,
	indexedHeaders []*IndexedHeader,
	forks []*Forks,
	epochsInfo []*EpochChainInfoEntry,
	lastSentSegment *BTCChainSegment,
	sealedEpochs []*SealedEpochProofEntry,
) *GenesisState {
	return &GenesisState{
		PortId:               PortID,
		Params:               params,
		ChainsInfo:           chainsInfo,
		ChainsIndexedHeaders: indexedHeaders,
		ChainsForks:          forks,
		ChainsEpochsInfo:     epochsInfo,
		LastSentSegment:      lastSentSegment,
		SealedEpochsProofs:   sealedEpochs,
	}
}

// Validate performs basic genesis state validation returning an error upon any
// failure.
func (gs GenesisState) Validate() error {
	if err := host.PortIdentifierValidator(gs.PortId); err != nil {
		return err
	}

	if err := types.ValidateEntries(gs.ChainsInfo, func(ci *ChainInfo) string { return ci.ConsumerId }); err != nil {
		return err
	}

	if err := types.ValidateEntries(
		gs.ChainsIndexedHeaders,
		func(ih *IndexedHeader) string {
			// unique key is consumer id + epoch number
			return ih.ConsumerId + strconv.FormatUint(ih.BabylonEpoch, 10)
		}); err != nil {
		return err
	}

	for _, f := range gs.ChainsForks {
		if err := f.Validate(); err != nil {
			return err
		}
	}

	if err := types.ValidateEntries(
		gs.ChainsEpochsInfo,
		func(eci *EpochChainInfoEntry) string {
			var consumerId string
			// if this is nil, the corresponding error is returned later
			if eci.ChainInfo != nil && eci.ChainInfo.ChainInfo != nil {
				consumerId = eci.ChainInfo.ChainInfo.ConsumerId
			}
			// unique key is consumer id + epoch number
			return consumerId + strconv.FormatUint(eci.EpochNumber, 10)
		}); err != nil {
		return err
	}

	if gs.LastSentSegment != nil {
		if err := gs.LastSentSegment.Validate(); err != nil {
			return err
		}
	}

	if err := types.ValidateEntries(gs.SealedEpochsProofs, func(se *SealedEpochProofEntry) uint64 { return se.EpochNumber }); err != nil {
		return err
	}

	return gs.Params.Validate()
}

func (eci EpochChainInfoEntry) Validate() error {
	if eci.ChainInfo == nil {
		return errors.New("invalid epoch chain info entry. empty chain info")
	}
	return eci.ChainInfo.Validate()
}

func (sep SealedEpochProofEntry) Validate() error {
	if sep.Proof == nil {
		return errors.New("invalid sealed epoch entry. empty proof")
	}
	return sep.Proof.ValidateBasic()
}

// Helper function to sort slices to get a deterministic
// result on the tests
func SortData(gs *GenesisState) {
	sort.Slice(gs.ChainsInfo, func(i, j int) bool {
		return gs.ChainsInfo[i].ConsumerId < gs.ChainsInfo[j].ConsumerId
	})
	sort.Slice(gs.ChainsIndexedHeaders, func(i, j int) bool {
		return gs.ChainsIndexedHeaders[i].ConsumerId < gs.ChainsIndexedHeaders[j].ConsumerId
	})
	sort.Slice(gs.ChainsForks, func(i, j int) bool {
		return gs.ChainsForks[i].Headers[0].ConsumerId < gs.ChainsForks[j].Headers[0].ConsumerId
	})
	sort.Slice(gs.ChainsEpochsInfo, func(i, j int) bool {
		if gs.ChainsEpochsInfo[i].EpochNumber != gs.ChainsEpochsInfo[j].EpochNumber {
			return gs.ChainsEpochsInfo[i].EpochNumber < gs.ChainsEpochsInfo[j].EpochNumber
		}
		return gs.ChainsEpochsInfo[i].ChainInfo.ChainInfo.ConsumerId < gs.ChainsEpochsInfo[j].ChainInfo.ChainInfo.ConsumerId
	})

	sort.Slice(gs.SealedEpochsProofs, func(i, j int) bool {
		if gs.SealedEpochsProofs[i].EpochNumber != gs.SealedEpochsProofs[j].EpochNumber {
			return gs.SealedEpochsProofs[i].EpochNumber < gs.SealedEpochsProofs[j].EpochNumber
		}
		return gs.SealedEpochsProofs[i].Proof.String() < gs.SealedEpochsProofs[j].Proof.String()
	})
}
