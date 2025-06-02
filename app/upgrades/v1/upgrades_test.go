package v1_test

import (
	_ "embed"
	"fmt"
	"slices"
	"testing"
	"time"

	"cosmossdk.io/core/appmodule"
	"cosmossdk.io/core/header"
	errorsmod "cosmossdk.io/errors"
	sdkmath "cosmossdk.io/math"
	"cosmossdk.io/x/upgrade"
	upgradetypes "cosmossdk.io/x/upgrade/types"
	wasmkeeper "github.com/CosmWasm/wasmd/x/wasm/keeper"
	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
	wasmvm "github.com/CosmWasm/wasmvm/v2"
	wasmvmtypes "github.com/CosmWasm/wasmvm/v2/types"
	appparams "github.com/babylonlabs-io/babylon/v4/app/params"
	"github.com/babylonlabs-io/babylon/v4/app/upgrades"
	"github.com/babylonlabs-io/babylon/v4/test/e2e/util"
	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	"github.com/babylonlabs-io/babylon/v4/testutil/sample"
	bbn "github.com/babylonlabs-io/babylon/v4/types"
	minttypes "github.com/babylonlabs-io/babylon/v4/x/mint/types"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/stretchr/testify/suite"

	"github.com/babylonlabs-io/babylon/v4/app"
	v1 "github.com/babylonlabs-io/babylon/v4/app/upgrades/v1"
	mainnetdata "github.com/babylonlabs-io/babylon/v4/app/upgrades/v1/mainnet"
	testnetdata "github.com/babylonlabs-io/babylon/v4/app/upgrades/v1/testnet"
	"github.com/babylonlabs-io/babylon/v4/x/btclightclient"
	btclighttypes "github.com/babylonlabs-io/babylon/v4/x/btclightclient/types"
)

const (
	DummyUpgradeHeight = 5
)

var (
	//go:embed testdata/reflect_1_5.wasm
	wasmContract []byte

	UpgradeV1DataTestnet = v1.UpgradeDataString{
		BtcStakingParamsStr:       testnetdata.BtcStakingParamsStr,
		FinalityParamStr:          testnetdata.FinalityParamStr,
		IncentiveParamStr:         testnetdata.IncentiveParamStr,
		CosmWasmParamStr:          testnetdata.CosmWasmParamStr,
		NewBtcHeadersStr:          testnetdata.NewBtcHeadersStr,
		TokensDistributionStr:     testnetdata.TokensDistributionStr,
		AllowedStakingTxHashesStr: testnetdata.AllowedStakingTxHashesStr,
	}
	UpgradeV1DataMainnet = v1.UpgradeDataString{
		BtcStakingParamsStr:       mainnetdata.BtcStakingParamsStr,
		FinalityParamStr:          mainnetdata.FinalityParamStr,
		IncentiveParamStr:         mainnetdata.IncentiveParamStr,
		CosmWasmParamStr:          mainnetdata.CosmWasmParamStr,
		NewBtcHeadersStr:          mainnetdata.NewBtcHeadersStr,
		TokensDistributionStr:     mainnetdata.TokensDistributionStr,
		AllowedStakingTxHashesStr: mainnetdata.AllowedStakingTxHashesStr,
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
	// TokenDistribution checker
	balanceDiffByAddr     map[string]int64
	balancesBeforeUpgrade map[string]sdk.Coin
}

func TestKeeperTestSuite(t *testing.T) {
	suite.Run(t, new(UpgradeTestSuite))
}

func (s *UpgradeTestSuite) TestUpgrade() {
	stakingWasmChecksum, err := wasmvm.CreateChecksum(wasmContract)
	s.NoError(err)

	testCases := []struct {
		msg            string
		baseBtcHeader  *btclighttypes.BTCHeaderInfo
		upgradeDataStr v1.UpgradeDataString
		upgradeParams  v1.ParamUpgradeFn
		preUpgrade     func()
		upgrade        func()
		postUpgrade    func()
	}{
		{
			"Test launch software upgrade v1 mainnet",
			sample.MainnetBtcHeader854784(s.T()),
			UpgradeV1DataMainnet,
			mainnetdata.ParamUpgrade,
			s.PreUpgrade,
			s.Upgrade,
			func() {
				s.PostUpgrade()

				randAddr := datagen.GenRandomAddress().String()
				// checks that not everybody can instantiate a contract
				wasmMsgServer := wasmkeeper.NewMsgServerImpl(&s.app.WasmKeeper)
				resp, err := wasmMsgServer.StoreCode(s.ctx, &wasmtypes.MsgStoreCode{
					Sender:       randAddr,
					WASMByteCode: wasmContract,
				})
				s.Nil(resp)
				s.EqualError(err, errorsmod.Wrap(sdkerrors.ErrUnauthorized, "can not create code").Error())

				// onlu gov account can store new contracts
				respFromGov, err := wasmMsgServer.StoreCode(s.ctx, &wasmtypes.MsgStoreCode{
					Sender:       appparams.AccGov.String(),
					WASMByteCode: wasmContract,
				})
				s.NoError(err)
				s.EqualValues(respFromGov.CodeID, 1)
				s.Equal(stakingWasmChecksum[:], wasmvmtypes.Checksum(respFromGov.Checksum))

				// anyone can instantiate if it was stored already
				respInst, err := wasmMsgServer.InstantiateContract(s.ctx, &wasmtypes.MsgInstantiateContract{
					Sender: randAddr,
					CodeID: respFromGov.CodeID,
					Label:  "xxxx",
					Msg:    []byte(`{}`),
					Funds:  sdk.Coins{},
				})
				s.NoError(err)
				s.NotNil(respInst.Address)
			},
		},
		{
			"Test launch software upgrade v1 testnet",
			sample.SignetBtcHeader195552(s.T()),
			UpgradeV1DataTestnet,
			testnetdata.ParamUpgrade,
			s.PreUpgrade,
			s.Upgrade,
			func() {
				s.PostUpgrade()

				// checks that anyone can instantiate a contract
				wasmMsgServer := wasmkeeper.NewMsgServerImpl(&s.app.WasmKeeper)
				resp, err := wasmMsgServer.StoreCode(s.ctx, &wasmtypes.MsgStoreCode{
					Sender:       datagen.GenRandomAddress().String(),
					WASMByteCode: wasmContract,
				})
				s.NoError(err)
				s.EqualValues(resp.CodeID, 1)
				s.Equal(stakingWasmChecksum[:], wasmvmtypes.Checksum(resp.Checksum))

				// check that the gov params were updated
				govParams, err := s.app.GovKeeper.Params.Get(s.ctx)
				s.NoError(err)
				s.EqualValues(testnetdata.VotingPeriod, *govParams.VotingPeriod)
				s.EqualValues(testnetdata.ExpeditedVotingPeriod, *govParams.ExpeditedVotingPeriod)
				s.EqualValues([]sdk.Coin{testnetdata.MinDeposit}, govParams.MinDeposit)
				s.EqualValues([]sdk.Coin{testnetdata.ExpeditedMinDeposit}, govParams.ExpeditedMinDeposit)

				// check that the consensus params were updated
				consensusParams, err := s.app.ConsensusParamsKeeper.ParamsStore.Get(s.ctx)
				s.NoError(err)
				s.EqualValues(testnetdata.BlockGasLimit, consensusParams.Block.MaxGas)

				// check that the staking params were updated
				stakingParams, err := s.app.StakingKeeper.GetParams(s.ctx)
				s.NoError(err)
				s.EqualValues(testnetdata.MinCommissionRate, stakingParams.MinCommissionRate)

				// check that the distribution params were updated
				distributionParams, err := s.app.DistrKeeper.Params.Get(s.ctx)
				s.NoError(err)
				s.EqualValues(testnetdata.CommunityTax, distributionParams.CommunityTax)

				// check that the btc checkpoint params were updated
				btcCheckpointParams := s.app.BtcCheckpointKeeper.GetParams(s.ctx)
				s.EqualValues(testnetdata.BTCCheckpointTag, btcCheckpointParams.CheckpointTag)

				// check that the btc light client params were updated
				btcLCParams := s.app.BTCLightClientKeeper.GetParams(s.ctx)
				s.True(slices.Contains(btcLCParams.InsertHeadersAllowList, testnetdata.ReporterAllowAddress))
			},
		},
	}

	for _, tc := range testCases {
		s.Run(fmt.Sprintf("Case %s", tc.msg), func() {
			s.SetupTest(tc.upgradeDataStr, tc.upgradeParams, tc.baseBtcHeader) // reset

			tc.preUpgrade()
			tc.upgrade()
			tc.postUpgrade()
		})
	}
}

func (s *UpgradeTestSuite) SetupTest(upgradeDataStr v1.UpgradeDataString, upgradeParams v1.ParamUpgradeFn, baseBtcHeader *btclighttypes.BTCHeaderInfo) {
	s.upgradeDataStr = upgradeDataStr

	// add the upgrade plan
	app.Upgrades = []upgrades.Upgrade{v1.CreateUpgrade(upgradeDataStr, upgradeParams)}

	// set up app
	s.app = app.SetupWithBitcoinConf(s.T(), false, bbn.BtcSignet)
	s.ctx = s.app.BaseApp.NewContextLegacy(false, tmproto.Header{Height: 1, ChainID: "babylon-1", Time: time.Now().UTC()})
	s.preModule = upgrade.NewAppModule(s.app.UpgradeKeeper, s.app.AccountKeeper.AddressCodec())

	btcLightK := s.app.BTCLightClientKeeper
	btclightclient.InitGenesis(s.ctx, s.app.BTCLightClientKeeper, btclighttypes.GenesisState{
		Params:     btcLightK.GetParams(s.ctx),
		BtcHeaders: []*btclighttypes.BTCHeaderInfo{baseBtcHeader},
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

	// Before upgrade, the params should be different
	bsParamsFromUpgrade, err := v1.LoadBtcStakingParamsFromData(s.upgradeDataStr.BtcStakingParamsStr)
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
		s.Require().NoError(err)
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

	// ensure the incentive params were set as expected
	incentiveParamsFromUpgrade, err := v1.LoadIncentiveParamsFromData(s.app.AppCodec(), s.upgradeDataStr.IncentiveParamStr)
	s.NoError(err)
	incentiveParams := s.app.IncentiveKeeper.GetParams(s.ctx)
	s.EqualValues(incentiveParamsFromUpgrade, incentiveParams)

	// ensure the headers were inserted as expected
	for i, btcHeaderInserted := range btcHeadersInserted {
		btcHeaderInState := allBtcHeaders[s.btcHeadersLenPreUpgrade+i]

		s.EqualValues(btcHeaderInserted.Header.MarshalHex(), btcHeaderInState.Header.MarshalHex())
	}

	// After upgrade, the params should be the same
	bsParamsFromUpgrade, err := v1.LoadBtcStakingParamsFromData(s.upgradeDataStr.BtcStakingParamsStr)
	s.NoError(err)

	bsModuleParams := s.app.BTCStakingKeeper.GetParams(s.ctx)
	lastParamInUpgradeData := bsParamsFromUpgrade[len(bsParamsFromUpgrade)-1]
	s.EqualValues(bsModuleParams, lastParamInUpgradeData)

	for expVersion, paramsInUpgradeData := range bsParamsFromUpgrade {
		bsParamsAtBtcHeight, version, err := s.app.BTCStakingKeeper.GetParamsForBtcHeight(s.ctx, uint64(paramsInUpgradeData.BtcActivationHeight))
		s.NoError(err)
		s.Equal(uint32(expVersion), version)
		s.Equal(*bsParamsAtBtcHeight, paramsInUpgradeData)
	}

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

	chainWasmParams := s.app.WasmKeeper.GetParams(s.ctx)
	upgradeWasmParams, err := v1.LoadCosmWasmParamsFromData(s.app.AppCodec(), s.upgradeDataStr.CosmWasmParamStr)
	s.NoError(err)
	s.EqualValues(chainWasmParams, upgradeWasmParams)

	allowedStakingTxHashes, err := v1.LoadAllowedStakingTransactionHashesFromData(s.upgradeDataStr.AllowedStakingTxHashesStr)
	s.NoError(err)
	s.NotNil(allowedStakingTxHashes)
	s.Greater(len(allowedStakingTxHashes.TxHashes), 0)

	for _, txHash := range allowedStakingTxHashes.TxHashes {
		hash, err := chainhash.NewHashFromStr(txHash)
		s.NoError(err)

		s.True(s.app.BTCStakingKeeper.IsStakingTransactionAllowed(s.ctx, hash))
	}

	nonExistentTxHash := chainhash.Hash{}
	s.False(s.app.BTCStakingKeeper.IsStakingTransactionAllowed(s.ctx, &nonExistentTxHash))
}
