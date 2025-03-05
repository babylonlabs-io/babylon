package datagen

import (
	"math/rand"
	"time"

	"cosmossdk.io/core/header"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	ibctmtypes "github.com/cosmos/ibc-go/v8/modules/light-clients/07-tendermint"
)

func GenRandomTMHeader(r *rand.Rand, chainID string, height uint64) *cmtproto.Header {
	return &cmtproto.Header{
		ChainID: chainID,
		Height:  int64(height),
		Time:    time.Now(),
		AppHash: GenRandomByteArray(r, 32),
	}
}

func GenRandomIBCTMHeader(r *rand.Rand, height uint64) *ibctmtypes.Header {
	return &ibctmtypes.Header{
		SignedHeader: &cmtproto.SignedHeader{
			Header: &cmtproto.Header{
				ChainID: GenRandomHexStr(r, 10),
				Height:  int64(height),
				AppHash: GenRandomByteArray(r, 32),
			},
		},
	}
}

func GenRandomTMHeaderInfo(r *rand.Rand, chainID string, height uint64) *header.Info {
	return &header.Info{
		Height:  int64(height),
		ChainID: chainID,
		AppHash: GenRandomByteArray(r, 32),
	}
}

func WithCtxHeight(ctx sdk.Context, height uint64) sdk.Context {
	headerInfo := ctx.HeaderInfo()
	headerInfo.Height = int64(height)
	ctx = ctx.WithHeaderInfo(headerInfo)
	return ctx
}
