package keeper_test

import (
	"context"
	"math/rand"
	"testing"
	"time"

	"github.com/babylonlabs-io/babylon/testutil/datagen"
	testhelper "github.com/babylonlabs-io/babylon/testutil/helper"
	zckeeper "github.com/babylonlabs-io/babylon/x/zoneconcierge/keeper"
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

func TestFeatureGate(t *testing.T) {
	// Save the original value of EnableIntegration
	originalEnableIntegration := zctypes.EnableIntegration
	// Restore the original value after the test
	defer func() {
		zctypes.EnableIntegration = originalEnableIntegration
	}()
	// Set EnableIntegration to false
	zctypes.EnableIntegration = false

	helper := testhelper.NewHelper(t)
	zcKeeper := helper.App.ZoneConciergeKeeper
	ctx := helper.Ctx

	// Create a random header and consumer ID
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
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
	require.Error(t, err, "Header should not be stored when EnableIntegration is false")

	/*
		Ensure GRPC query server is feature gated
	*/
	// Get zone concierge query client
	zcQueryHelper := baseapp.NewQueryServerTestHelper(ctx, helper.App.InterfaceRegistry())
	queryClient := zctypes.NewQueryClient(zcQueryHelper)
	// Test GetParams query
	paramsReq := &zctypes.QueryParamsRequest{}
	_, err = queryClient.Params(ctx, paramsReq)
	require.Error(t, err, "Params query should be blocked when EnableIntegration is false")
	require.Contains(t, err.Error(), "handler not found for /babylon.zoneconcierge.v1.Query/Params")

	/*
		Ensure msg server is feature gated
	*/
	msgClient := zctypes.NewMsgClient(zcQueryHelper)
	msgReq := &zctypes.MsgUpdateParams{
		Authority: helper.App.GovKeeper.GetGovernanceAccount(ctx).GetAddress().String(),
		Params:    zctypes.DefaultParams(),
	}
	_, err = msgClient.UpdateParams(ctx, msgReq)
	require.Error(t, err, "MsgUpdateParams should be blocked when EnableIntegration is false")
	require.Contains(t, err.Error(), "handler not found for /babylon.zoneconcierge.v1.Msg/UpdateParams")
}
