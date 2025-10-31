package e2e2

import (
	"testing"

	"github.com/stretchr/testify/require"

	"cosmossdk.io/math"
	upgradetypes "cosmossdk.io/x/upgrade/types"
	"github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types/v1"

	v5 "github.com/babylonlabs-io/babylon/v4/app/upgrades/v5"
	"github.com/babylonlabs-io/babylon/v4/test/e2ev2/tmanager"
)

func TestUpgradeV5(t *testing.T) {
	t.Parallel()
	tm := tmanager.NewTmWithUpgrade(t, 0, "")
	validator := tm.ChainValidator()
	govMsg, preUpgradeFunc, err := createGovPropAndPreUpgradeFunc(validator.Wallet.WalletSender)
	require.NoError(t, err)

	tm.Start(govMsg, preUpgradeFunc)

	bsParams := validator.QueryBtcStakingParams()
	require.Equal(t, uint32(1), bsParams.MaxStakerQuorum)
	require.Equal(t, uint32(1), bsParams.MaxStakerNum)
}

func createGovPropAndPreUpgradeFunc(valWallet *tmanager.WalletSender) (*govtypes.MsgSubmitProposal, tmanager.PreUpgradeFunc, error) {
	// create the upgrade message
	upgradeMsg := &upgradetypes.MsgSoftwareUpgrade{
		Authority: "bbn10d07y265gmmuvt4z0w9aw880jnsr700jduz5f2",
		Plan: upgradetypes.Plan{
			Name:   v5.UpgradeName,
			Height: int64(20),
			Info:   "Upgrade to v5",
		},
	}

	anyMsg, err := types.NewAnyWithValue(upgradeMsg)
	if err != nil {
		return nil, nil, err
	}

	govMsg := &govtypes.MsgSubmitProposal{
		Messages:       []*types.Any{anyMsg},
		InitialDeposit: []sdk.Coin{sdk.NewCoin("ubbn", math.NewInt(1000000))},
		Proposer:       valWallet.Address.String(),
		Metadata:       "",
		Title:          "v5",
		Summary:        "v5 upgrade",
		Expedited:      false,
	}

	// create PreUpgradeFunc for a v5 upgrade scenario. this function will be executed before upgrade.
	preUpgradeFunc := func(nodes []*tmanager.Node) {
		// TODO: setup fp, create single-sig delegation, failed multisig delegation, query btcstaking params

	}

	// return the path that will be accessible in Docker containers
	return govMsg, preUpgradeFunc, nil
}
