package app

import erc20types "github.com/cosmos/evm/x/erc20/types"

// WTokenContractMainnet is the WrappedToken contract address for mainnet
const WTokenContractMainnet = "0xD4949664cD82660AaE99bEdc034a0deA8A0bd517"

// TokenPairs creates a slice of token pairs, that contains a pair for the native denom of the example chain
// implementation.
var TokenPairs = []erc20types.TokenPair{
	{
		Erc20Address:  WTokenContractMainnet,
		Denom:         BaseCosmosDenom,
		Enabled:       true,
		ContractOwner: erc20types.OWNER_MODULE,
	},
}
