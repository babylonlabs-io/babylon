package keeper_test

import (
	"context"
	"math/rand"
	"testing"

	"github.com/babylonlabs-io/babylon/testutil/datagen"
	testhelper "github.com/babylonlabs-io/babylon/testutil/helper"
	zckeeper "github.com/babylonlabs-io/babylon/x/zoneconcierge/keeper"
	"github.com/babylonlabs-io/babylon/x/zoneconcierge/types"
	zctypes "github.com/babylonlabs-io/babylon/x/zoneconcierge/types"
	"github.com/cosmos/cosmos-sdk/baseapp"
	ibctmtypes "github.com/cosmos/ibc-go/v8/modules/light-clients/07-tendermint"
	"github.com/stretchr/testify/require"
)

func init() {
	zctypes.EnableIntegration = true
}

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

// SimulateNewHeadersAndForks generates a random non-zero number of canonical headers and fork headers
func SimulateNewHeadersAndForks(ctx context.Context, r *rand.Rand, k *zckeeper.Keeper, consumerID string, startHeight uint64, numHeaders uint64, numForkHeaders uint64) ([]*ibctmtypes.Header, []*ibctmtypes.Header) {
	headers := []*ibctmtypes.Header{}
	// invoke the hook a number of times to simulate a number of blocks
	for i := uint64(0); i < numHeaders; i++ {
		header := datagen.GenRandomIBCTMHeader(r, startHeight+i)
		k.HandleHeaderWithValidCommit(ctx, datagen.GenRandomByteArray(r, 32), datagen.NewZCHeaderInfo(header, consumerID), false)
		headers = append(headers, header)
	}

	// generate a number of fork headers
	forkHeaders := []*ibctmtypes.Header{}
	for i := uint64(0); i < numForkHeaders; i++ {
		header := datagen.GenRandomIBCTMHeader(r, startHeight+numHeaders-1)
		k.HandleHeaderWithValidCommit(ctx, datagen.GenRandomByteArray(r, 32), datagen.NewZCHeaderInfo(header, consumerID), true)
		forkHeaders = append(forkHeaders, header)
	}
	return headers, forkHeaders
}

func FuzzFeatureGate(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		// Save the original value of EnableIntegration
		originalEnableIntegration := zctypes.EnableIntegration
		// Restore the original value after the test
		defer func() {
			zctypes.EnableIntegration = originalEnableIntegration
		}()
		// Set EnableIntegration to a random value
		currentEnableIntegration := datagen.OneInN(r, 2)
		zctypes.EnableIntegration = currentEnableIntegration

		helper := testhelper.NewHelper(t)
		zcKeeper := helper.App.ZoneConciergeKeeper
		ctx := helper.Ctx

		// Create a random header and consumer ID
		header := datagen.GenRandomIBCTMHeader(r, 1)
		consumerID := datagen.GenRandomHexStr(r, 30)
		headerInfo := datagen.NewZCHeaderInfo(header, consumerID)

		/*
			Ensure PostHandler is feature gated
		*/
		// Call HandleHeaderWithValidCommit
		zcKeeper.HandleHeaderWithValidCommit(ctx, datagen.GenRandomByteArray(r, 32), headerInfo, false)
		// Check that the header was not stored
		_, err := zcKeeper.GetHeader(ctx, consumerID, 1)
		if currentEnableIntegration {
			require.NoError(t, err, "Header should be stored when EnableIntegration is true")
		} else {
			require.Error(t, err, "Header should not be stored when EnableIntegration is false")
		}

		/*
			Ensure GRPC query server is feature gated
		*/
		// Get zone concierge query client
		zcQueryHelper := baseapp.NewQueryServerTestHelper(ctx, helper.App.InterfaceRegistry())
		querier := zckeeper.Querier{Keeper: zcKeeper}
		types.RegisterQueryServer(zcQueryHelper, querier)

		queryClient := zctypes.NewQueryClient(zcQueryHelper)

		// Test GetParams query
		paramsReq := &zctypes.QueryParamsRequest{}
		_, err = queryClient.Params(ctx, paramsReq)
		if currentEnableIntegration {
			require.NoError(t, err, "Params query should work when EnableIntegration is true")
		} else {
			require.Error(t, err, "Params query should be blocked when EnableIntegration is false")
			require.ErrorIs(t, err, types.ErrIntegrationDisabled)
		}

		/*
			Ensure msg server is feature gated
		*/
		msgSrvr := zckeeper.NewMsgServerImpl(zcKeeper)
		msgReq := &zctypes.MsgUpdateParams{
			Authority: helper.App.GovKeeper.GetGovernanceAccount(ctx).GetAddress().String(),
			Params:    zctypes.DefaultParams(),
		}
		_, err = msgSrvr.UpdateParams(ctx, msgReq)
		if currentEnableIntegration {
			require.NoError(t, err, "MsgUpdateParams should work when EnableIntegration is true")
		} else {
			require.Error(t, err, "MsgUpdateParams should be blocked when EnableIntegration is false")
			require.ErrorIs(t, err, types.ErrIntegrationDisabled)
		}

		/*
			Ensure IBC route does not contain zone concierge
		*/
		// Get the IBC keeper
		ibcKeeper := helper.App.IBCKeeper
		// Get the IBC router
		router := ibcKeeper.Router
		// Ensure the zone concierge module is not in the router
		_, found := router.GetRoute(zctypes.ModuleName)
		if currentEnableIntegration {
			require.True(t, found, "Zone concierge module should be in the IBC router when EnableIntegration is true")
		} else {
			require.False(t, found, "Zone concierge module should not be in the IBC router when EnableIntegration is false")
		}
	})
}
