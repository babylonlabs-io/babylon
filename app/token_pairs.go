package app

import (
	appparams "github.com/babylonlabs-io/babylon/v3/app/params"
	erc20types "github.com/cosmos/evm/x/erc20/types"
)

// WTokenContractMainnet is the WrappedToken contract address for mainnet
const WTokenContractMainnet = "0xD4949664cD82660AaE99bEdc034a0deA8A0bd517"

// DefaultTokenPairs creates a slice of token pairs, that contains a pair for the native denom of the Babylon
// chain and it's corresponding ERC20 address.
// DefaultTokenPairs represents all of the Coins that have a corresponding ERC20 precompile.
var DefaultTokenPairs = []erc20types.TokenPair{
	{
		Erc20Address:  WTokenContractMainnet,
		Denom:         appparams.BaseCosmosDenom,
		Enabled:       true,
		ContractOwner: erc20types.OWNER_MODULE,
	},
}
