package sample

import (
	"testing"

	"github.com/babylonlabs-io/babylon/v4/types"
	bbn "github.com/babylonlabs-io/babylon/v4/types"
	btclightck "github.com/babylonlabs-io/babylon/v4/x/btclightclient/keeper"
	btclighttypes "github.com/babylonlabs-io/babylon/v4/x/btclightclient/types"
	"github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
)

// AccAddress returns a sample account address
func AccAddress() string {
	pk := ed25519.GenPrivKey().PubKey()
	addr := pk.Address()
	return sdk.AccAddress(addr).String()
}

// SignetBtcHeader195552 returns the BTC Header block 195552 from signet bbn-test-4.
func SignetBtcHeader195552(t *testing.T) *btclighttypes.BTCHeaderInfo {
	var btcHeader btclighttypes.BTCHeaderInfo

	// signet btc header of height 195552
	btcHeaderHash, err := types.NewBTCHeaderBytesFromHex("00000020c8710c5662ab0a4680963697765a390cba4814f95f0556fc5fb3b446b2000000fa9b80e52653455e5d4a4648fbe1f62854a07dbec0633a42ef595431de9be36dccb64366934f011ef3d98200")
	require.NoError(t, err)

	wireHeaders := btclightck.BtcHeadersBytesToBlockHeader([]types.BTCHeaderBytes{btcHeaderHash})
	wireHeader := wireHeaders[0]

	blockHash := wireHeader.BlockHash()
	headerHash := bbn.NewBTCHeaderHashBytesFromChainhash(&blockHash)
	work := btclighttypes.CalcWork(&btcHeaderHash)
	btcHeader = btclighttypes.BTCHeaderInfo{
		Header: &btcHeaderHash,
		Height: uint32(195552),
		Hash:   &headerHash,
		Work:   &work,
	}

	return &btcHeader
}

// MainnetBtcHeader854784 returns the BTC Header block 854784 from mainnet.
func MainnetBtcHeader854784(t *testing.T) *btclighttypes.BTCHeaderInfo {
	var btcHeader btclighttypes.BTCHeaderInfo

	// mainnet btc header of height 854784
	btcHeaderHash, err := types.NewBTCHeaderBytesFromHex("0000c020f382af1f6d228721b49f3da2f5b831587803b16597b301000000000000000000e4f76aae64d8316d195a92424871b74168b58d1c3c6988548e0e9890b15fc2fc3c00aa66be1a0317082e4bc7")
	require.NoError(t, err)

	wireHeaders := btclightck.BtcHeadersBytesToBlockHeader([]types.BTCHeaderBytes{btcHeaderHash})
	wireHeader := wireHeaders[0]

	blockHash := wireHeader.BlockHash()
	headerHash := bbn.NewBTCHeaderHashBytesFromChainhash(&blockHash)
	work := btclighttypes.CalcWork(&btcHeaderHash)
	btcHeader = btclighttypes.BTCHeaderInfo{
		Header: &btcHeaderHash,
		Height: uint32(854784),
		Hash:   &headerHash,
		Work:   &work,
	}

	return &btcHeader
}
