package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"cosmossdk.io/core/address"
	errorsmod "cosmossdk.io/errors"
	sdkmath "cosmossdk.io/math"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/tx"
	"github.com/cosmos/cosmos-sdk/codec"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	cosmoscli "github.com/cosmos/cosmos-sdk/x/staking/client/cli"
	staketypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	flag "github.com/spf13/pflag"

	appsigner "github.com/babylonlabs-io/babylon/v3/app/signer"
	"github.com/babylonlabs-io/babylon/v3/crypto/bls12381"
	"github.com/babylonlabs-io/babylon/v3/x/checkpointing/types"
)

// validator struct to define the fields of the validator
type validator struct {
	Amount            sdk.Coin
	PubKey            cryptotypes.PubKey
	Moniker           string
	Identity          string
	Website           string
	Security          string
	Details           string
	CommissionRates   staketypes.CommissionRates
	MinSelfDelegation sdkmath.Int
}

// copied from https://github.com/cosmos/cosmos-sdk/blob/v0.50.1/x/staking/client/cli/utils.go#L20
func parseAndValidateValidatorJSON(cdc codec.Codec, path string) (validator, error) {
	type internalVal struct {
		Amount              string          `json:"amount"`
		PubKey              json.RawMessage `json:"pubkey"`
		Moniker             string          `json:"moniker"`
		Identity            string          `json:"identity,omitempty"`
		Website             string          `json:"website,omitempty"`
		Security            string          `json:"security,omitempty"`
		Details             string          `json:"details,omitempty"`
		CommissionRate      string          `json:"commission-rate"`
		CommissionMaxRate   string          `json:"commission-max-rate"`
		CommissionMaxChange string          `json:"commission-max-change-rate"`
		MinSelfDelegation   string          `json:"min-self-delegation"`
	}

	contents, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return validator{}, err
	}

	var v internalVal
	err = json.Unmarshal(contents, &v)
	if err != nil {
		return validator{}, err
	}

	if v.Amount == "" {
		return validator{}, fmt.Errorf("must specify amount of coins to bond")
	}
	amount, err := sdk.ParseCoinNormalized(v.Amount)
	if err != nil {
		return validator{}, err
	}

	if v.PubKey == nil {
		return validator{}, fmt.Errorf("must specify the JSON encoded pubkey")
	}
	var pk cryptotypes.PubKey
	if err := cdc.UnmarshalInterfaceJSON(v.PubKey, &pk); err != nil {
		return validator{}, err
	}

	if v.Moniker == "" {
		return validator{}, fmt.Errorf("must specify the moniker name")
	}

	commissionRates, err := buildCommissionRates(v.CommissionRate, v.CommissionMaxRate, v.CommissionMaxChange)
	if err != nil {
		return validator{}, err
	}

	if v.MinSelfDelegation == "" {
		return validator{}, fmt.Errorf("must specify minimum self delegation")
	}
	minSelfDelegation, ok := sdkmath.NewIntFromString(v.MinSelfDelegation)
	if !ok {
		return validator{}, errorsmod.Wrap(sdkerrors.ErrInvalidRequest, "minimum self delegation must be a positive integer")
	}

	return validator{
		Amount:            amount,
		PubKey:            pk,
		Moniker:           v.Moniker,
		Identity:          v.Identity,
		Website:           v.Website,
		Security:          v.Security,
		Details:           v.Details,
		CommissionRates:   commissionRates,
		MinSelfDelegation: minSelfDelegation,
	}, nil
}

// copied from https://github.com/cosmos/cosmos-sdk/blob/v0.50.1/x/staking/client/cli/tx.go#L382
func newBuildCreateValidatorMsg(clientCtx client.Context, txf tx.Factory, fs *flag.FlagSet, val validator, valAc address.Codec) (tx.Factory, *staketypes.MsgCreateValidator, error) {
	valAddr := clientCtx.GetFromAddress()

	description := staketypes.NewDescription(
		val.Moniker,
		val.Identity,
		val.Website,
		val.Security,
		val.Details,
	)

	valStr, err := valAc.BytesToString(valAddr)
	if err != nil {
		return txf, nil, err
	}
	msg, err := staketypes.NewMsgCreateValidator(
		valStr, val.PubKey, val.Amount, description, val.CommissionRates, val.MinSelfDelegation,
	)
	if err != nil {
		return txf, nil, err
	}
	if err := msg.Validate(valAc); err != nil {
		return txf, nil, err
	}

	genOnly, _ := fs.GetBool(flags.FlagGenerateOnly)
	if genOnly {
		ip, _ := fs.GetString(cosmoscli.FlagIP)
		p2pPort, _ := fs.GetUint(cosmoscli.FlagP2PPort)
		nodeID, _ := fs.GetString(cosmoscli.FlagNodeID)

		if nodeID != "" && ip != "" && p2pPort > 0 {
			txf = txf.WithMemo(fmt.Sprintf("%s@%s:%d", nodeID, ip, p2pPort))
		}
	}

	return txf, msg, nil
}

// buildWrappedCreateValidatorMsg builds a MsgWrappedCreateValidator that wraps MsgCreateValidator with BLS key
func buildWrappedCreateValidatorMsg(clientCtx client.Context, txf tx.Factory, fs *flag.FlagSet, val validator, valAc address.Codec) (tx.Factory, *types.MsgWrappedCreateValidator, error) {
	txf, msg, err := newBuildCreateValidatorMsg(clientCtx, txf, fs, val, valAc)
	if err != nil {
		return txf, nil, fmt.Errorf("failed to build create validator message: %w", err)
	}

	var blsPK *bls12381.PublicKey
	var pop *types.ProofOfPossession

	blsPopFilePath, _ := fs.GetString(FlagBlsPopFilePath)
	if blsPopFilePath != "" {
		// if blsPopFilePath is not empty, load bls key from the provided path
		blsPop, err := appsigner.LoadBlsPop(blsPopFilePath)
		if err != nil {
			return txf, nil, fmt.Errorf("failed to load bls pop from provided path: %w", err)
		}
		blsPK = &blsPop.BlsPubkey
		pop = blsPop.Pop
	} else {
		// if blsPopFilePath is empty, load bls key from the node directory
		// both priv_validator_key.json and bls_key.json are required
		// to be present in the node directory
		home, _ := fs.GetString(flags.FlagHome)
		valKey, err := getValKeyFromFile(home)
		if err != nil {
			return txf, nil, fmt.Errorf("failed to load bls key from node directory: %w", err)
		}
		blsPK = &valKey.BlsPubkey
		pop = valKey.PoP
	}

	wrappedMsg, err := types.NewMsgWrappedCreateValidator(msg, blsPK, pop)
	if err != nil {
		return txf, nil, fmt.Errorf("failed to create wrapped create validator message: %w", err)
	}
	if err := wrappedMsg.ValidateBasic(); err != nil {
		return txf, nil, fmt.Errorf("failed to validate wrapped create validator message: %w", err)
	}

	return txf, wrappedMsg, nil
}

// Copied from https://github.com/cosmos/cosmos-sdk/blob/v0.50.1/x/staking/client/cli/utils.go#L104
func buildCommissionRates(rateStr, maxRateStr, maxChangeRateStr string) (commission staketypes.CommissionRates, err error) {
	if rateStr == "" || maxRateStr == "" || maxChangeRateStr == "" {
		return commission, errors.New("must specify all validator commission parameters")
	}

	rate, err := sdkmath.LegacyNewDecFromStr(rateStr)
	if err != nil {
		return commission, err
	}

	maxRate, err := sdkmath.LegacyNewDecFromStr(maxRateStr)
	if err != nil {
		return commission, err
	}

	maxChangeRate, err := sdkmath.LegacyNewDecFromStr(maxChangeRateStr)
	if err != nil {
		return commission, err
	}

	commission = staketypes.NewCommissionRates(rate, maxRate, maxChangeRate)

	return commission, nil
}

// getValKeyFromFile loads the validator key from the node directory
// Both FilePV from priv_validator_key.json and Bls should be present in the node directory
// befor function is called.
func getValKeyFromFile(homeDir string) (*appsigner.ValidatorKeys, error) {
	ck, err := appsigner.LoadConsensusKey(homeDir)
	if err != nil {
		return nil, fmt.Errorf("failed to load consensus key: %w", err)
	}

	return appsigner.NewValidatorKeys(ck.Comet.PrivKey, ck.Bls.PrivKey)
}
