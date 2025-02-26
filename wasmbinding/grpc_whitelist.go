package wasmbinding

import (
	wasmkeeper "github.com/CosmWasm/wasmd/x/wasm/keeper"
	bsctypes "github.com/babylonlabs-io/babylon/x/btcstkconsumer/types"
	epochtypes "github.com/babylonlabs-io/babylon/x/epoching/types"
	ftypes "github.com/babylonlabs-io/babylon/x/finality/types"
	"github.com/cosmos/gogoproto/proto"
)

// WhitelistedGrpcQuery returns the whitelisted Grpc queries
func WhitelistedGrpcQuery() wasmkeeper.AcceptedQueries {
	return wasmkeeper.AcceptedQueries{
		// btcstkconsumer
		"/babylon.btcstkconsumer.v1.Query/FinalityProvider": func() proto.Message {
			return &bsctypes.QueryFinalityProviderResponse{}
		},
		// btcstaking
		"/babylon.btcstaking.v1.Query/FinalityProviderCurrentPower": func() proto.Message {
			return &ftypes.QueryFinalityProviderCurrentPowerResponse{}
		},
		// for testing
		"/babylon.epoching.v1.Query/CurrentEpoch": func() proto.Message {
			return &epochtypes.QueryCurrentEpochResponse{}
		},
	}
}
