package keeper_test

import (
	"fmt"
	"math/rand"
	"os"
	"runtime/pprof"
	"testing"
	"time"

	"github.com/golang/mock/gomock"

	testutil "github.com/babylonlabs-io/babylon/v2/testutil/btcstaking-helper"
	"github.com/babylonlabs-io/babylon/v2/testutil/datagen"
	btclctypes "github.com/babylonlabs-io/babylon/v2/x/btclightclient/types"
	bsmodule "github.com/babylonlabs-io/babylon/v2/x/btcstaking"
	"github.com/babylonlabs-io/babylon/v2/x/btcstaking/types"
)

func benchBeginBlock(b *testing.B, numFPs int, numDelsUnderFP int) {
	r := rand.New(rand.NewSource(time.Now().Unix()))

	// helper
	ctrl := gomock.NewController(b)
	defer ctrl.Finish()
	btclcKeeper := types.NewMockBTCLightClientKeeper(ctrl)
	btccKeeper := types.NewMockBtcCheckpointKeeper(ctrl)
	h := testutil.NewHelper(b, btclcKeeper, btccKeeper)
	// set all parameters
	covenantSKs, _ := h.GenAndApplyParams(r)

	// generate new finality providers
	fps := []*types.FinalityProvider{}
	for i := 0; i < numFPs; i++ {
		fp, err := datagen.GenRandomFinalityProvider(r, h.FpPopContext())
		h.NoError(err)
		msg := &types.MsgCreateFinalityProvider{
			Addr:        fp.Addr,
			Description: fp.Description,
			Commission: types.NewCommissionRates(
				*fp.Commission,
				fp.CommissionInfo.MaxRate,
				fp.CommissionInfo.MaxChangeRate,
			),
			BtcPk: fp.BtcPk,
			Pop:   fp.Pop,
		}
		_, err = h.MsgServer.CreateFinalityProvider(h.Ctx, msg)
		h.NoError(err)
		fps = append(fps, fp)
	}

	// create new BTC delegations under each finality provider
	btcDelMap := map[string][]*types.BTCDelegation{}
	for _, fp := range fps {
		for i := 0; i < numDelsUnderFP; i++ {
			// generate and insert new BTC delegation
			stakingValue := int64(2 * 10e8)
			delSK, _, err := datagen.GenRandomBTCKeyPair(r)
			h.NoError(err)
			stakingTxHash, msgCreateBTCDel, actualDel, btcHeaderInfo, inclusionProof, _, err := h.CreateDelegationWithBtcBlockHeight(
				r,
				delSK,
				fp.BtcPk.MustToBTCPK(),
				stakingValue,
				1000,
				0,
				0,
				true,
				false,
				10,
				10,
			)
			h.NoError(err)
			// retrieve BTC delegation in DB
			btcDelMap[stakingTxHash] = append(btcDelMap[stakingTxHash], actualDel)
			// generate and insert new covenant signatures
			h.CreateCovenantSigs(r, covenantSKs, msgCreateBTCDel, actualDel, 10)
			// activate BTC delegation
			// after that, all BTC delegations will have voting power
			h.AddInclusionProof(stakingTxHash, btcHeaderInfo, inclusionProof, 30)
		}
	}

	// mock stuff
	h.BTCLightClientKeeper.EXPECT().GetTipInfo(gomock.Eq(h.Ctx)).Return(&btclctypes.BTCHeaderInfo{Height: 30}).AnyTimes()

	// Start the CPU profiler
	cpuProfileFile := fmt.Sprintf("/tmp/btcstaking-beginblock-%d-%d-cpu.pprof", numFPs, numDelsUnderFP)
	f, err := os.Create(cpuProfileFile)
	if err != nil {
		b.Fatal(err)
	}
	defer f.Close()
	if err := pprof.StartCPUProfile(f); err != nil {
		b.Fatal(err)
	}
	defer pprof.StopCPUProfile()

	// Reset timer before the benchmark loop starts
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		err = bsmodule.BeginBlocker(h.Ctx, *h.BTCStakingKeeper)
		h.NoError(err)
	}
}

func BenchmarkBeginBlock_10_1(b *testing.B)    { benchBeginBlock(b, 10, 1) }
func BenchmarkBeginBlock_10_10(b *testing.B)   { benchBeginBlock(b, 10, 10) }
func BenchmarkBeginBlock_10_100(b *testing.B)  { benchBeginBlock(b, 10, 100) }
func BenchmarkBeginBlock_100_1(b *testing.B)   { benchBeginBlock(b, 100, 1) }
func BenchmarkBeginBlock_100_10(b *testing.B)  { benchBeginBlock(b, 100, 10) }
func BenchmarkBeginBlock_100_100(b *testing.B) { benchBeginBlock(b, 100, 100) }
