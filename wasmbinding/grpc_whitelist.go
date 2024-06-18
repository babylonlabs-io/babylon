package wasmbinding

import (
	wasmkeeper "github.com/CosmWasm/wasmd/x/wasm/keeper"
	bsttypes "github.com/babylonchain/babylon/x/btcstaking/types"
	bsctypes "github.com/babylonchain/babylon/x/btcstkconsumer/types"
	epochtypes "github.com/babylonchain/babylon/x/epoching/types"
)

// WhitelistedGrpcQuery returns the whitelisted Grpc queries
func WhitelistedGrpcQuery() wasmkeeper.AcceptedQueries {
	return wasmkeeper.AcceptedQueries{
		// btcstkconsumer
		"/babylon.btcstkconsumer.v1.Query/FinalityProvider": &bsctypes.QueryFinalityProviderResponse{},
		// btcstaking
		"/babylon.btcstaking.v1.Query/FinalityProviderCurrentPower": &bsttypes.QueryFinalityProviderCurrentPowerResponse{},
		// for testing
		"/babylon.epoching.v1.Query/CurrentEpoch": &epochtypes.QueryCurrentEpochResponse{},
	}
}
