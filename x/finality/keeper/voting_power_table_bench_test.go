package keeper_test

import (
	"fmt"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/babylonlabs-io/babylon/testutil/datagen"
	kt "github.com/babylonlabs-io/babylon/testutil/keeper"
	"github.com/babylonlabs-io/babylon/x/finality/keeper"
	"github.com/babylonlabs-io/babylon/x/finality/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
)

const (
	height = 100
)

func saveFinalityProviders(
	numFinalityProvidersPerBlock int,
	lastBlockHeight int,
	b *testing.B) (*keeper.Keeper, sdk.Context, []byte, func()) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	tempDir, err := os.MkdirTemp("", "bench-fin")
	fmt.Printf("tempDir: %s\n", tempDir)
	require.NoError(b, err)
	k, closeDb, commit, dbWriteAlot, ctx := kt.FinalityKeeperWithDb(b, tempDir, nil, nil, nil)
	randIdx := r.Intn(numFinalityProvidersPerBlock)

	var randKey []byte
	var activeKeys [][]byte

	for i := 0; i < numFinalityProvidersPerBlock; i++ {
		key, err := datagen.GenRandomBIP340PubKey(r)
		require.NoError(b, err)
		activeKeys = append(activeKeys, key.MustMarshal())
		if i == randIdx {
			randKey = key.MustMarshal()
		}
	}

	for block := 1; block <= lastBlockHeight; block++ {
		for _, key := range activeKeys {
			randVotingPower := uint64(r.Intn(1000000)) + 10
			k.SetVotingPower(ctx, key, uint64(block), randVotingPower)
		}
	}

	// first write to disk, this will actually flush the data to memory table
	// of the underlaying golang level db
	commit()
	// write a lot of data to db directly, this will force golab level db
	// to flush the data to disk
	dbWriteAlot()

	cleanup := func() {
		closeDb()
		os.RemoveAll(tempDir)
	}

	return k, ctx, randKey, cleanup
}

func getAllFinalityProviders(numFinalityProvidersPerBlock int, lastBlockHeight int, b *testing.B) {
	k, ctx, _, cleanup := saveFinalityProviders(numFinalityProvidersPerBlock, lastBlockHeight, b)
	defer cleanup()

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		table := k.GetVotingPowerTable(ctx, height)
		require.Equal(b, len(table), numFinalityProvidersPerBlock)
	}
}

func getOneFpVotingPower(numFinalityProviders int, lastBlockHeight int, b *testing.B) {
	k, ctx, fpKey, cleanup := saveFinalityProviders(numFinalityProviders, lastBlockHeight, b)
	defer cleanup()

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		vp := k.GetVotingPower(ctx, fpKey, height)
		require.NotZero(b, vp)
	}
}

func saveFinalityProvidersNew(
	numFinalityProvidersPerBlock int,
	lastBlockHeight int,
	b *testing.B) (*keeper.Keeper, sdk.Context, []byte, func()) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	tempDir, err := os.MkdirTemp("", "bench-fin")
	fmt.Printf("tempDir: %s\n", tempDir)
	require.NoError(b, err)
	k, closeDb, commit, dbWriteAlot, ctx := kt.FinalityKeeperWithDb(b, tempDir, nil, nil, nil)
	randIdx := r.Intn(numFinalityProvidersPerBlock)

	var activeFPs []*types.ActiveFinalityProvider

	var randKey []byte

	for i := 0; i < numFinalityProvidersPerBlock; i++ {
		key, err := datagen.GenRandomBIP340PubKey(r)
		require.NoError(b, err)
		randVotingPower := uint64(r.Intn(1000000)) + 10
		activeFPs = append(activeFPs, &types.ActiveFinalityProvider{FpBtcPk: key, VotingPower: randVotingPower})

		if i == randIdx {
			randKey = key.MustMarshal()
		}
	}

	for block := 1; block <= lastBlockHeight; block++ {
		k.SetVotingPowerAsList(ctx, uint64(block), activeFPs)
	}

	// first write to disk, this will actually flush the data to memory table
	// of the underlaying golang level db
	commit()

	// write a lot of data to db directly, this will force golab level db
	// to flush the data to disk
	dbWriteAlot()

	cleanup := func() {
		closeDb()
		os.RemoveAll(tempDir)
	}

	return k, ctx, randKey, cleanup
}

func getAllFinalityProvidersNew(numFinalityProvidersPerBlock int, lastBlockHeight int, b *testing.B) {
	k, ctx, _, cleanup := saveFinalityProvidersNew(numFinalityProvidersPerBlock, lastBlockHeight, b)
	defer cleanup()

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		table := k.GetVotingPowerAsList(ctx, height)
		require.Equal(b, len(table), numFinalityProvidersPerBlock)
	}
}

func getOneFpVotingPowerNew(numFinalityProviders int, lastBlockHeight int, b *testing.B) {
	k, ctx, fpKey, cleanup := saveFinalityProvidersNew(numFinalityProviders, lastBlockHeight, b)
	defer cleanup()

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		vp := k.GetVotingPowerNew(ctx, fpKey, height)
		require.NotZero(b, vp)
	}
}

// OLD ALL************************************************************************************

func BenchmarkGetAllFinalityProviders10(b *testing.B) {
	getAllFinalityProviders(10, height, b)
}

func BenchmarkGetAllFinalityProviders20(b *testing.B) {
	getAllFinalityProviders(20, height, b)
}

func BenchmarkGetAllFinalityProviders40(b *testing.B) {
	getAllFinalityProviders(40, height, b)
}

func BenchmarkGetAllFinalityProviders80(b *testing.B) {
	getAllFinalityProviders(80, height, b)
}

func BenchmarkGetAllFinalityProviders160(b *testing.B) {
	getAllFinalityProviders(160, height, b)
}

// New ALL************************************************************************************

func BenchmarkGetAllFinalityProvidersNew10(b *testing.B) {
	getAllFinalityProvidersNew(10, height, b)
}

func BenchmarkGetAllFinalityProvidersNew20(b *testing.B) {
	getAllFinalityProvidersNew(20, height, b)
}

func BenchmarkGetAllFinalityProvidersNew40(b *testing.B) {
	getAllFinalityProvidersNew(40, height, b)
}

func BenchmarkGetAllFinalityProvidersNew80(b *testing.B) {
	getAllFinalityProvidersNew(80, height, b)
}

func BenchmarkGetAllFinalityProvidersNew160(b *testing.B) {
	getAllFinalityProvidersNew(160, height, b)
}

// OLD ONE************************************************************************************

func BenchmarkGetOneFpVotingPower10(b *testing.B) {
	getOneFpVotingPower(10, height, b)
}

func BenchmarkGetOneFpVotingPower20(b *testing.B) {
	getOneFpVotingPower(20, height, b)
}

func BenchmarkGetOneFpVotingPower40(b *testing.B) {
	getOneFpVotingPower(40, height, b)
}

func BenchmarkGetOneFpVotingPower80(b *testing.B) {
	getOneFpVotingPower(80, height, b)
}

func BenchmarkGetOneFpVotingPower160(b *testing.B) {
	getOneFpVotingPower(160, height, b)
}

// NEW ONE************************************************************************************

func BenchmarkGetOneFpVotingPowerNew10(b *testing.B) {
	getOneFpVotingPowerNew(10, height, b)
}

func BenchmarkGetOneFpVotingPowerNew20(b *testing.B) {
	getOneFpVotingPowerNew(20, height, b)
}

func BenchmarkGetOneFpVotingPowerNew40(b *testing.B) {
	getOneFpVotingPowerNew(40, height, b)
}

func BenchmarkGetOneFpVotingPowerNew80(b *testing.B) {
	getOneFpVotingPowerNew(80, height, b)
}

func BenchmarkGetOneFpVotingPowerNew160(b *testing.B) {
	getOneFpVotingPowerNew(160, height, b)
}

// ************************************************************************************ New new

func saveFinalityProvidersNew1(
	numFinalityProvidersPerBlock int,
	lastBlockHeight int,
	t *testing.B) (*keeper.Keeper, sdk.Context, []byte, func()) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	tempDir, err := os.MkdirTemp("", "bench-fin")
	fmt.Printf("tempDir: %s\n", tempDir)
	require.NoError(t, err)
	k, closeDb, commit, dbWriteAlot, ctx := kt.FinalityKeeperWithDb(t, tempDir, nil, nil, nil)
	randIdx := r.Intn(numFinalityProvidersPerBlock)

	var activeFPs []*keeper.ActiveFp

	var randKey []byte

	for i := 0; i < numFinalityProvidersPerBlock; i++ {
		key, err := datagen.GenRandomBIP340PubKey(r)
		require.NoError(t, err)
		randVotingPower := uint64(r.Intn(1000000)) + 10
		activeFPs = append(activeFPs, &keeper.ActiveFp{FPBTCPK: key.MustMarshal(), VotingPower: randVotingPower})

		if i == randIdx {
			randKey = key.MustMarshal()
		}
	}

	for block := 1; block <= lastBlockHeight; block++ {
		k.SetVotingPowerAsListNew1(ctx, uint64(block), activeFPs)
	}

	// first write to disk, this will actually flush the data to memory table
	// of the underlaying golang level db
	commit()

	// write a lot of data to db directly, this will force golab level db
	// to flush the data to disk
	dbWriteAlot()

	cleanup := func() {
		closeDb()
		os.RemoveAll(tempDir)
	}

	return k, ctx, randKey, cleanup
}

func getOneFpVotingPowerNew1(numFinalityProviders int, lastBlockHeight int, b *testing.B) {
	k, ctx, fpKey, cleanup := saveFinalityProvidersNew1(numFinalityProviders, lastBlockHeight, b)
	defer cleanup()

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		vp := k.GetVotingPowerNew1(ctx, fpKey, height)
		require.NotZero(b, vp)
	}
}

func BenchmarkGetOneFpVotingPowerNew160New1(b *testing.B) {
	getOneFpVotingPowerNew1(160, height, b)
}
