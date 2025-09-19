package types

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	tmjson "github.com/cometbft/cometbft/libs/json"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/babylonlabs-io/babylon/v4/crypto/bls12381"
	"github.com/babylonlabs-io/babylon/v4/types"
)

// DefaultGenesis returns the default Capability genesis state
func DefaultGenesis() *GenesisState {
	return &GenesisState{}
}

// Validate performs basic genesis state validation returning an error upon any
// failure.
func (gs GenesisState) Validate() error {
	if err := types.ValidateEntries(gs.GenesisKeys, func(gk *GenesisKey) string { return gk.ValidatorAddress }); err != nil {
		return err
	}

	if err := types.ValidateEntries(gs.ValidatorSets, func(e *ValidatorSetEntry) uint64 { return e.EpochNumber }); err != nil {
		return err
	}

	return types.ValidateEntries(
		gs.Checkpoints,
		func(chkpt *RawCheckpointWithMeta) uint64 {
			if chkpt.Ckpt == nil {
				return 0
			}
			return chkpt.Ckpt.EpochNum
		},
	)
}

func NewGenesisKey(delAddr sdk.ValAddress, blsPubKey *bls12381.PublicKey, pop *ProofOfPossession, pubkey cryptotypes.PubKey) (*GenesisKey, error) {
	if !pop.IsValid(*blsPubKey, pubkey) {
		return nil, ErrInvalidPoP
	}
	gk := &GenesisKey{
		ValidatorAddress: delAddr.String(),
		BlsKey: &BlsKey{
			Pubkey: blsPubKey,
			Pop:    pop,
		},
		ValPubkey: pubkey.(*ed25519.PubKey),
	}

	return gk, nil
}

func LoadGenesisKeyFromFile(filePath string) (*GenesisKey, error) {
	genBlsJSONBytes, err := os.ReadFile(filepath.Clean(filePath))
	if err != nil {
		return nil, err
	}
	genBls := new(GenesisKey)
	err = tmjson.Unmarshal(genBlsJSONBytes, genBls)
	if err != nil {
		return nil, err
	}
	err = genBls.Validate()
	if err != nil {
		return nil, err
	}
	return genBls, nil
}

func (gk *GenesisKey) Validate() error {
	if !gk.BlsKey.Pop.IsValid(*gk.BlsKey.Pubkey, gk.ValPubkey) {
		return ErrInvalidPoP
	}
	return nil
}

// GetGenesisStateFromAppState returns x/Checkpointing GenesisState given raw application
// genesis state.
func GetGenesisStateFromAppState(cdc codec.Codec, appState map[string]json.RawMessage) GenesisState {
	var genesisState GenesisState

	if appState[ModuleName] != nil {
		cdc.MustUnmarshalJSON(appState[ModuleName], &genesisState)
	}

	return genesisState
}

func GenTxMessageValidatorWrappedCreateValidator(msgs []sdk.Msg) error {
	if len(msgs) != 1 {
		return fmt.Errorf("unexpected number of GenTx messages; got: %d, expected: 1", len(msgs))
	}

	msg, ok := msgs[0].(*MsgWrappedCreateValidator)
	if !ok {
		return fmt.Errorf("unexpected GenTx message type; expected: MsgWrappedCreateValidator, got: %T", msgs[0])
	}

	if err := msg.ValidateBasic(); err != nil {
		return fmt.Errorf("invalid GenTx '%s': %w", msgs[0], err)
	}

	return nil
}

func (gk *ValidatorSetEntry) Validate() error {
	return gk.ValidatorSet.Validate()
}

// Helper function to sort slices to get a deterministic
// result on the tests
func SortData(gs *GenesisState) {
	sort.Slice(gs.GenesisKeys, func(i, j int) bool {
		return gs.GenesisKeys[i].ValidatorAddress < gs.GenesisKeys[j].ValidatorAddress
	})

	sort.Slice(gs.ValidatorSets, func(i, j int) bool {
		return gs.ValidatorSets[i].EpochNumber < gs.ValidatorSets[j].EpochNumber
	})

	sort.Slice(gs.Checkpoints, func(i, j int) bool {
		if gs.Checkpoints[i].Ckpt != nil && gs.Checkpoints[j].Ckpt != nil {
			return gs.Checkpoints[i].Ckpt.EpochNum < gs.Checkpoints[j].Ckpt.EpochNum
		}
		return gs.Checkpoints[i].PowerSum < gs.Checkpoints[j].PowerSum
	})
}
