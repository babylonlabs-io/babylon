package cli

import (
	"fmt"
	"math"
	"os"

	"github.com/spf13/pflag"

	sdkmath "cosmossdk.io/math"
	"github.com/btcsuite/btcd/btcutil"

	"github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
)

func parseLockTime(str string) (uint16, error) {
	num, ok := sdkmath.NewIntFromString(str)

	if !ok {
		return 0, fmt.Errorf("invalid staking time: %s", str)
	}

	if !num.IsUint64() {
		return 0, fmt.Errorf("staking time is not valid uint")
	}

	asUint64 := num.Uint64()

	if asUint64 > math.MaxUint16 {
		return 0, fmt.Errorf("staking time is too large. Max is %d", math.MaxUint16)
	}

	return uint16(asUint64), nil
}

func parseBtcAmount(str string) (btcutil.Amount, error) {
	num, ok := sdkmath.NewIntFromString(str)

	if !ok {
		return 0, fmt.Errorf("invalid staking value: %s", str)
	}

	if num.IsNegative() {
		return 0, fmt.Errorf("staking value is negative")
	}

	if !num.IsInt64() {
		return 0, fmt.Errorf("staking value is not valid uint")
	}

	asInt64 := num.Int64()

	return btcutil.Amount(asInt64), nil
}

// parseMultisigInfoJSON parse json multisig into AdditionalStakerInfo
// Note: it returns (nil, nil), if path/to/multisig.json is not provided (or empty string)
func parseMultisigInfoJSON(fs *pflag.FlagSet) (*types.AdditionalStakerInfo, error) {
	var multisigInfo types.AdditionalStakerInfo

	path, err := fs.GetString(FlagMultisigInfoJSON)
	if err != nil {
		return nil, err
	}

	// multisig info is not provided
	if path == "" {
		return nil, nil
	}

	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	err = types.ModuleCdc.UnmarshalJSON(contents, &multisigInfo)
	if err != nil {
		return nil, err
	}

	return &multisigInfo, nil
}
