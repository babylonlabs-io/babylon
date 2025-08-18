package chain

import (
	"math/rand"
	"strings"
	"testing"
	"time"

	sdkmath "cosmossdk.io/math"
	appparams "github.com/babylonlabs-io/babylon/v4/app/params"
	"github.com/babylonlabs-io/babylon/v4/app/signingcontext"
	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	bstypes "github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/stretchr/testify/require"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// CreateFpFromNodeAddr creates a random Babylon finality provider.
func CreateFpFromNodeAddr(
	t *testing.T,
	r *rand.Rand,
	fpSk *btcec.PrivateKey,
	node *NodeConfig,
) (newFP *bstypes.FinalityProvider) {
	// the node is the new FP
	nodeAddr, err := sdk.AccAddressFromBech32(node.PublicAddress)
	require.NoError(t, err)

	fpPopContext := signingcontext.FpPopContextV0(node.chainId, appparams.AccBTCStaking.String())

	newFP, err = datagen.GenCustomFinalityProvider(r, fpSk, fpPopContext, nodeAddr, "")
	require.NoError(t, err)

	// empty defaults to Babylon Genesis
	previousFps := node.QueryFinalityProviders(node.chainId)

	// use a higher commission to ensure the reward is more than tx fee of a finality sig
	commission := sdkmath.LegacyNewDecWithPrec(20, 2)
	newFP.Commission = &commission
	node.CreateConsumerFinalityProvider(newFP.Addr, node.chainId, newFP.BtcPk, newFP.Pop,
		newFP.Description.Moniker, newFP.Description.Identity,
		newFP.Description.Website, newFP.Description.SecurityContact,
		newFP.Description.Details, newFP.Commission, newFP.CommissionInfo.MaxRate,
		newFP.CommissionInfo.MaxChangeRate)

	// query the existence of finality provider and assert equivalence
	actualFps := node.QueryFinalityProviders(node.chainId)
	require.Len(t, actualFps, len(previousFps)+1)

	// get chain ID to assert equality with the BsnId field
	status, err := node.Status()
	require.NoError(t, err)
	newFP.BsnId = status.NodeInfo.Network

	for _, fpResp := range actualFps {
		if !strings.EqualFold(fpResp.Addr, newFP.Addr) {
			continue
		}
		EqualFinalityProviderResp(t, newFP, fpResp)
		return newFP
	}

	return nil
}

// CreateConsumerFpFromNodeAddr creates a random Consumer finality provider.
func CreateConsumerFpFromNodeAddr(
	t *testing.T,
	r *rand.Rand,
	consumerId string,
	fpSk *btcec.PrivateKey,
	node *NodeConfig,
) (newFP *bstypes.FinalityProvider) {
	// the node is the new FP
	nodeAddr, err := sdk.AccAddressFromBech32(node.PublicAddress)
	require.NoError(t, err)

	fpPopContext := signingcontext.FpPopContextV0(node.chainId, appparams.AccBTCStaking.String())

	newFP, err = datagen.GenCustomFinalityProvider(r, fpSk, fpPopContext, nodeAddr, consumerId)
	require.NoError(t, err)

	previousFps := node.QueryFinalityProviders(consumerId)

	// use a higher commission to ensure the reward is more than tx fee of a
	// finality sig
	commission := sdkmath.LegacyNewDecWithPrec(20, 2)
	newFP.Commission = &commission
	node.CreateConsumerFinalityProvider(newFP.Addr, consumerId, newFP.BtcPk, newFP.Pop, newFP.Description.Moniker, newFP.Description.Identity, newFP.Description.Website, newFP.Description.SecurityContact, newFP.Description.Details, newFP.Commission, newFP.CommissionInfo.MaxRate, newFP.CommissionInfo.MaxChangeRate)

	// get chain ID to assert equality with the ConsumerId field
	if consumerId == "" {
		status, err := node.Status()
		require.NoError(t, err)
		newFP.BsnId = status.NodeInfo.Network
	}

	// query the existence of finality provider and assert equivalence
	actualFps := node.QueryFinalityProviders(consumerId)
	require.Len(t, actualFps, len(previousFps)+1)

	for _, fpResp := range actualFps {
		if !strings.EqualFold(fpResp.Addr, newFP.Addr) {
			continue
		}
		EqualConsumerFinalityProviderResp(t, newFP, fpResp)
		return newFP
	}

	return nil
}

func EqualFinalityProviderResp(t *testing.T, fp *bstypes.FinalityProvider, fpResp *bstypes.FinalityProviderResponse) {
	require.Equal(t, fp.Description, fpResp.Description)
	require.Equal(t, fp.Commission, fpResp.Commission)
	require.Equal(t, fp.Addr, fpResp.Addr)
	require.Equal(t, fp.BtcPk, fpResp.BtcPk)
	require.Equal(t, fp.Pop, fpResp.Pop)
	require.Equal(t, fp.SlashedBabylonHeight, fpResp.SlashedBabylonHeight)
	require.Equal(t, fp.SlashedBtcHeight, fpResp.SlashedBtcHeight)
	require.Equal(t, fp.BsnId, fpResp.BsnId)
	require.Equal(t, fp.CommissionInfo.MaxRate, fpResp.CommissionInfo.MaxRate)
	require.Equal(t, fp.CommissionInfo.MaxChangeRate, fpResp.CommissionInfo.MaxChangeRate)
	// UpdateTime field is set to the
	// current block time on creation, so we can check in the response
	// if the UpdateTime is within the last 15 secs
	require.GreaterOrEqual(t, fpResp.CommissionInfo.UpdateTime, time.Now().UTC().Add(-10*time.Minute))
}

func EqualConsumerFinalityProviderResp(t *testing.T, fp *bstypes.FinalityProvider, fpResp *bstypes.FinalityProviderResponse) {
	require.Equal(t, fp.Description, fpResp.Description)
	require.Equal(t, fp.Commission, fpResp.Commission)
	require.Equal(t, fp.Addr, fpResp.Addr)
	require.Equal(t, fp.BtcPk, fpResp.BtcPk)
	require.Equal(t, fp.Pop, fpResp.Pop)
	require.Equal(t, fp.SlashedBabylonHeight, fpResp.SlashedBabylonHeight)
	require.Equal(t, fp.SlashedBtcHeight, fpResp.SlashedBtcHeight)
	require.Equal(t, fp.BsnId, fpResp.BsnId)
}
