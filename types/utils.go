package types

import (
	"fmt"
	"reflect"

	"github.com/btcsuite/btcd/chaincfg"
)

func Reverse(s interface{}) {
	n := reflect.ValueOf(s).Len()
	swap := reflect.Swapper(s)
	for i, j := 0, n-1; i < j; i, j = i+1, j-1 {
		swap(i, j)
	}
}

func CheckForDuplicatesAndEmptyStrings(input []string) error {
	encountered := map[string]bool{}
	for _, str := range input {
		if len(str) == 0 {
			return fmt.Errorf("empty string is not allowed")
		}

		if encountered[str] {
			return fmt.Errorf("duplicate entry found: %s", str)
		}

		encountered[str] = true
	}

	return nil
}

// GetActivationHeight returns the minimum block height from which the BTC
// staking protocol starts updating the voting power table.
func GetActivationHeight(btcNetwork string) uint64 {
	switch btcNetwork {
	// The activation height might differ accordingly
	// with the test that will be done and the block time
	// that the validator sets in config.
	case chaincfg.MainNetParams.Name:
		// For mainnet considering we want 48 hours
		// at a block time of 10s that would be 17280 blocks
		// considering the upgrade for Phase-2 will happen at block
		// 220, the mainnet activation height for btcstaking should
		// be 17280 + 220.
		return 17500
	case chaincfg.SigNetParams.Name:
		// Signet is used by the devnet testing and 50 blocks
		// is enough to execute proper checks and verify the voting
		// power table is not activated before this.
		return 50
	case chaincfg.RegressionNetParams.Name:
		// regtest is only used for internal deployment testing
		// and 40 blocks is enough to do the proper checks before
		// the height is reached.
		return 40
	default:
		// Overall btc network configs do not need to wait for activation
		// as unit tests should not be affected by this.
		return 0
	}
}
