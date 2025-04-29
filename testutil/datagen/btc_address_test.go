package datagen_test

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/babylonlabs-io/babylon/v2/testutil/datagen"
	"github.com/babylonlabs-io/babylon/v2/types"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/stretchr/testify/require"
)

func FuzzGenRandomBTCAddress(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))
		net := &chaincfg.SimNetParams

		addr, err := datagen.GenRandomBTCAddress(r, net)
		require.NoError(t, err)

		// validate the address encoding/decoding
		addr2, err := btcutil.DecodeAddress(addr.EncodeAddress(), net)
		require.NoError(t, err)

		// ensure the address does not change after encoding/decoding
		require.Equal(t, addr.String(), addr2.String())
	})
}

func TestGenRandomBTCHeight(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	// this is signer header
	headerHex := "000000203bc383465c19c335119e45a12609337da3ab74cc42e1c8d3d352a45a0a0000003c77cf8aec371b218c1d8ff0280cb6a6df1483bb338e49483a5a70fa224dada04f581168b09a0e1d5514411b"

	// headerBytes, err := types.NewBTCHeaderBytesFromHex(headerHex)
	// require.NoError(t, err)
	// header := headerBytes.ToBlockHeader()

	// now := time.Now()

	// validheader := datagen.GenRandomBtcdValidHeader(r, header, nil, nil)
	// require.NotNil(t, validheader)
	// elapsed := time.Since(now)
	// fmt.Println("elapsed", elapsed)

	// vHeader := types.NewBTCHeaderBytesFromBlockHeader(validheader)
	// fmt.Println("vHeader", vHeader.MarshalHex())
	for i := 0; i < 3; i++ {
		headerBytes, err := types.NewBTCHeaderBytesFromHex(headerHex)
		require.NoError(t, err)
		header := headerBytes.ToBlockHeader()

		validheader := datagen.GenRandomBtcdValidHeader(r, header, nil, nil)
		require.NotNil(t, validheader)

		vHeader := types.NewBTCHeaderBytesFromBlockHeader(validheader)

		headerHex = vHeader.MarshalHex()
		fmt.Printf("\nheader %d: %s", i, headerHex)
	}
}

func TestSignetBlocks(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	headerHex := "000000203bc383465c19c335119e45a12609337da3ab74cc42e1c8d3d352a45a0a0000003c77cf8aec371b218c1d8ff0280cb6a6df1483bb338e49483a5a70fa224dada04f581168b09a0e1d5514411b"
	headerBytes, err := types.NewBTCHeaderBytesFromHex(headerHex)
	require.NoError(t, err)
	header := headerBytes.ToBlockHeader()

	chainExtension := datagen.GenRandomValidChainStartingFrom(
		r,
		header,
		nil,
		3,
	)

	for i, c := range chainExtension {
		vHeader := types.NewBTCHeaderBytesFromBlockHeader(c)
		headerHex = vHeader.MarshalHex()

		t.Logf("\nheader %d: %s", i, headerHex)
		fmt.Printf("\nheader %d: %s", i, headerHex)
	}
}
