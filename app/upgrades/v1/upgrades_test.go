package v1_test

import (
	"fmt"
	"testing"
	"time"

	"cosmossdk.io/core/appmodule"
	"cosmossdk.io/core/header"
	sdkmath "cosmossdk.io/math"
	"cosmossdk.io/x/upgrade"
	upgradetypes "cosmossdk.io/x/upgrade/types"
	appparams "github.com/babylonlabs-io/babylon/app/params"
	"github.com/babylonlabs-io/babylon/app/upgrades"
	"github.com/babylonlabs-io/babylon/test/e2e/util"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	"github.com/stretchr/testify/suite"

	"github.com/babylonlabs-io/babylon/app"
	v1 "github.com/babylonlabs-io/babylon/app/upgrades/v1"
	mainnetdata "github.com/babylonlabs-io/babylon/app/upgrades/v1/mainnet"
	testnetdata "github.com/babylonlabs-io/babylon/app/upgrades/v1/testnet"
	"github.com/babylonlabs-io/babylon/x/btclightclient"
	btclighttypes "github.com/babylonlabs-io/babylon/x/btclightclient/types"
	"github.com/babylonlabs-io/babylon/x/btcstaking/types"
)

const (
	DummyUpgradeHeight = 5
)

var (
	UpgradeV1DataTestnet = v1.UpgradeDataString{
		BtcStakingParamStr:    testnetdata.BtcStakingParamStr,
		FinalityParamStr:      testnetdata.FinalityParamStr,
		CosmWasmParamStr:      testnetdata.CosmWasmParamStr,
		NewBtcHeadersStr:      testnetdata.NewBtcHeadersStr,
		SignedFPsStr:          testnetdata.SignedFPsStr,
		TokensDistributionStr: testnetdata.TokensDistributionStr,
	}
	UpgradeV1DataMainnet = v1.UpgradeDataString{
		BtcStakingParamStr:    mainnetdata.BtcStakingParamStr,
		FinalityParamStr:      mainnetdata.FinalityParamStr,
		CosmWasmParamStr:      mainnetdata.CosmWasmParamStr,
		NewBtcHeadersStr:      mainnetdata.NewBtcHeadersStr,
		SignedFPsStr:          mainnetdata.SignedFPsStr,
		TokensDistributionStr: mainnetdata.TokensDistributionStr,
	}
	UpgradeV1Data = []v1.UpgradeDataString{UpgradeV1DataTestnet, UpgradeV1DataMainnet}
)

type UpgradeTestSuite struct {
	suite.Suite

	ctx       sdk.Context
	app       *app.BabylonApp
	preModule appmodule.HasPreBlocker

	upgradeDataStr v1.UpgradeDataString
	// BTC Header checker
	btcHeadersLenPreUpgrade int
	// FPs checker
	finalityProvidersLenPreUpgrade int
	// TokenDistribution checker
	balanceDiffByAddr     map[string]int64
	balancesBeforeUpgrade map[string]sdk.Coin
}

func TestKeeperTestSuite(t *testing.T) {
	suite.Run(t, new(UpgradeTestSuite))
}

func (s *UpgradeTestSuite) TestUpgrade() {
	testCases := []struct {
		msg            string
		upgradeDataStr v1.UpgradeDataString
		preUpgrade     func()
		upgrade        func()
		postUpgrade    func()
	}{
		{
			"Test launch software upgrade v1 mainnet",
			UpgradeV1DataMainnet,
			s.PreUpgrade,
			s.Upgrade,
			s.PostUpgrade,
		},
		{
			"Test launch software upgrade v1 testnet",
			UpgradeV1DataTestnet,
			s.PreUpgrade,
			s.Upgrade,
			s.PostUpgrade,
		},
	}

	for _, tc := range testCases {
		s.Run(fmt.Sprintf("Case %s", tc.msg), func() {
			s.SetupTest(tc.upgradeDataStr) // reset

			tc.preUpgrade()
			tc.upgrade()
			tc.postUpgrade()
		})
	}
}

func (s *UpgradeTestSuite) SetupTest(upgradeDataStr v1.UpgradeDataString) {
	s.upgradeDataStr = upgradeDataStr

	// add the upgrade plan
	app.Upgrades = []upgrades.Upgrade{v1.CreateUpgrade(upgradeDataStr)}

	// set up app
	s.app = app.Setup(s.T(), false)
	s.ctx = s.app.BaseApp.NewContextLegacy(false, tmproto.Header{Height: 1, ChainID: "babylon-1", Time: time.Now().UTC()})
	s.preModule = upgrade.NewAppModule(s.app.UpgradeKeeper, s.app.AccountKeeper.AddressCodec())

	btcHeaderGenesis, err := app.SignetBtcHeaderGenesis(s.app.EncodingConfig().Codec)
	s.NoError(err)

	k := s.app.BTCLightClientKeeper
	btclightclient.InitGenesis(s.ctx, s.app.BTCLightClientKeeper, btclighttypes.GenesisState{
		Params:     k.GetParams(s.ctx),
		BtcHeaders: []*btclighttypes.BTCHeaderInfo{btcHeaderGenesis},
	})

	tokenDistData, err := v1.LoadTokenDistributionFromData(upgradeDataStr.TokensDistributionStr)
	s.NoError(err)

	s.balancesBeforeUpgrade = make(map[string]sdk.Coin)
	s.balanceDiffByAddr = make(map[string]int64)
	for _, td := range tokenDistData.TokenDistribution {
		s.balanceDiffByAddr[td.AddressSender] -= td.Amount
		s.balanceDiffByAddr[td.AddressReceiver] += td.Amount
	}
}

func (s *UpgradeTestSuite) PreUpgrade() {
	allBtcHeaders := s.app.BTCLightClientKeeper.GetMainChainFrom(s.ctx, 0)
	s.btcHeadersLenPreUpgrade = len(allBtcHeaders)

	resp, err := s.app.BTCStakingKeeper.FinalityProviders(s.ctx, &types.QueryFinalityProvidersRequest{})
	s.NoError(err)
	s.finalityProvidersLenPreUpgrade = len(resp.FinalityProviders)

	// Before upgrade, the params should be different
	bsParamsFromUpgrade, err := v1.LoadBtcStakingParamsFromData(s.app.AppCodec(), s.upgradeDataStr.BtcStakingParamStr)
	s.NoError(err)
	bsModuleParams := s.app.BTCStakingKeeper.GetParams(s.ctx)
	s.NotEqualValues(bsModuleParams, bsParamsFromUpgrade)
	fParamsFromUpgrade, err := v1.LoadFinalityParamsFromData(s.app.AppCodec(), s.upgradeDataStr.FinalityParamStr)
	s.NoError(err)
	fModuleParams := s.app.FinalityKeeper.GetParams(s.ctx)
	s.NotEqualValues(fModuleParams, fParamsFromUpgrade)

	for addr, amountDiff := range s.balanceDiffByAddr {
		sdkAddr := sdk.MustAccAddressFromBech32(addr)

		if amountDiff < 0 {
			// if the amount is lower than zero, it means the addr is going to spend tokens and
			// could be that the addr does not have enough funds.
			// For test completeness, mint the coins that the acc is going to spend.
			coinsToMint := sdk.NewCoins(sdk.NewCoin(appparams.DefaultBondDenom, sdkmath.NewInt(util.Abs(amountDiff))))
			err = s.app.BankKeeper.MintCoins(s.ctx, minttypes.ModuleName, coinsToMint)
			s.NoError(err)

			err = s.app.BankKeeper.SendCoinsFromModuleToAccount(s.ctx, minttypes.ModuleName, sdkAddr, coinsToMint)
			s.NoError(err)
		}

		// update the balances before upgrade only after mint check is done
		s.balancesBeforeUpgrade[addr] = s.app.BankKeeper.GetBalance(s.ctx, sdkAddr, appparams.DefaultBondDenom)
	}
}

func (s *UpgradeTestSuite) Upgrade() {
	// inject upgrade plan
	s.ctx = s.ctx.WithBlockHeight(DummyUpgradeHeight - 1)
	plan := upgradetypes.Plan{Name: v1.UpgradeName, Height: DummyUpgradeHeight}
	err := s.app.UpgradeKeeper.ScheduleUpgrade(s.ctx, plan)
	s.NoError(err)

	// ensure upgrade plan exists
	actualPlan, err := s.app.UpgradeKeeper.GetUpgradePlan(s.ctx)
	s.NoError(err)
	s.Equal(plan, actualPlan)

	// execute upgrade
	s.ctx = s.ctx.WithHeaderInfo(header.Info{Height: DummyUpgradeHeight, Time: s.ctx.BlockTime().Add(time.Second)}).WithBlockHeight(DummyUpgradeHeight)
	s.NotPanics(func() {
		_, err := s.preModule.PreBlock(s.ctx)
		s.NoError(err)
	})
}

func (s *UpgradeTestSuite) PostUpgrade() {
	// ensure the btc headers were added
	allBtcHeaders := s.app.BTCLightClientKeeper.GetMainChainFrom(s.ctx, 0)

	btcHeadersInserted, err := v1.LoadBTCHeadersFromData(s.app.AppCodec(), s.upgradeDataStr.NewBtcHeadersStr)
	s.NoError(err)
	lenHeadersInserted := len(btcHeadersInserted)

	newHeadersLen := len(allBtcHeaders)
	s.Equal(newHeadersLen, s.btcHeadersLenPreUpgrade+lenHeadersInserted)

	// ensure the headers were inserted as expected
	for i, btcHeaderInserted := range btcHeadersInserted {
		btcHeaderInState := allBtcHeaders[s.btcHeadersLenPreUpgrade+i]

		s.EqualValues(btcHeaderInserted.Header.MarshalHex(), btcHeaderInState.Header.MarshalHex())
	}

	resp, err := s.app.BTCStakingKeeper.FinalityProviders(s.ctx, &types.QueryFinalityProvidersRequest{})
	s.NoError(err)
	newFPsLen := len(resp.FinalityProviders)

	fpsInserted, err := v1.LoadSignedFPsFromData(s.app.AppCodec(), s.app.TxConfig().TxJSONDecoder(), s.upgradeDataStr.SignedFPsStr)
	s.NoError(err)

	s.Equal(newFPsLen, s.finalityProvidersLenPreUpgrade+len(fpsInserted))
	for _, fpInserted := range fpsInserted {
		fpFromKeeper, err := s.app.BTCStakingKeeper.GetFinalityProvider(s.ctx, *fpInserted.BtcPk)
		s.NoError(err)

		s.EqualValues(fpFromKeeper.Addr, fpInserted.Addr)
		s.EqualValues(fpFromKeeper.Description, fpInserted.Description)
		s.EqualValues(fpFromKeeper.Commission.String(), fpInserted.Commission.String())
		s.EqualValues(fpFromKeeper.Pop.String(), fpInserted.Pop.String())
	}

	// After upgrade, the params should be the same
	bsParamsFromUpgrade, err := v1.LoadBtcStakingParamsFromData(s.app.AppCodec(), s.upgradeDataStr.BtcStakingParamStr)
	s.NoError(err)
	bsModuleParams := s.app.BTCStakingKeeper.GetParams(s.ctx)
	s.EqualValues(bsModuleParams, bsParamsFromUpgrade)
	fParamsFromUpgrade, err := v1.LoadFinalityParamsFromData(s.app.AppCodec(), s.upgradeDataStr.FinalityParamStr)
	s.NoError(err)
	fModuleParams := s.app.FinalityKeeper.GetParams(s.ctx)
	s.EqualValues(fModuleParams, fParamsFromUpgrade)

	// verifies that all the modified balances match as expected after the upgrade
	for addr, diff := range s.balanceDiffByAddr {
		coinDiff := sdk.NewCoin(appparams.DefaultBondDenom, sdkmath.NewInt(util.Abs(diff)))
		expectedBalance := s.balancesBeforeUpgrade[addr].Add(coinDiff)
		if diff < 0 {
			expectedBalance = s.balancesBeforeUpgrade[addr].Sub(coinDiff)
		}

		sdkAddr := sdk.MustAccAddressFromBech32(addr)
		balanceAfterUpgrade := s.app.BankKeeper.GetBalance(s.ctx, sdkAddr, appparams.DefaultBondDenom)
		s.Equal(expectedBalance.String(), balanceAfterUpgrade.String())
	}
}
