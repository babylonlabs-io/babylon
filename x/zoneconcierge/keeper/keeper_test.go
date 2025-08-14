package keeper_test

import (
	"context"
	"math/rand"

	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	zckeeper "github.com/babylonlabs-io/babylon/v4/x/zoneconcierge/keeper"
	ibctmtypes "github.com/cosmos/ibc-go/v10/modules/light-clients/07-tendermint"
)

// SimulateNewHeaders generates a non-zero number of canonical headers
func SimulateNewHeaders(ctx context.Context, r *rand.Rand, k *zckeeper.Keeper, consumerID string, startHeight uint64, numHeaders uint64) []*ibctmtypes.Header {
	headers := []*ibctmtypes.Header{}
	// invoke the hook a number of times to simulate a number of blocks
	for i := uint64(0); i < numHeaders; i++ {
		header := datagen.GenRandomIBCTMHeader(r, startHeight+i)
		k.HandleHeaderWithValidCommit(ctx, datagen.GenRandomByteArray(r, 32), datagen.NewZCHeaderInfo(header, consumerID), false)
		headers = append(headers, header)
	}
	return headers
}
