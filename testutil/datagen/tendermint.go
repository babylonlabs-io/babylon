package datagen

import (
	zctypes "github.com/babylonlabs-io/babylon/v3/x/zoneconcierge/types"
	"math/rand"
	"time"

	"cosmossdk.io/core/header"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	ibctmtypes "github.com/cosmos/ibc-go/v10/modules/light-clients/07-tendermint"
)

func GenRandomTMHeader(r *rand.Rand, chainID string, height uint64) *cmtproto.Header {
	return &cmtproto.Header{
		ChainID: chainID,
		Height:  int64(height),
		Time:    time.Now().UTC(),
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

func NewZCHeaderInfo(header *ibctmtypes.Header, clientID string) *zctypes.HeaderInfo {
	return &zctypes.HeaderInfo{
		ClientId: clientID,
		AppHash:  header.Header.AppHash,
		ChainId:  header.Header.ChainID,
		Time:     header.Header.Time,
		Height:   uint64(header.Header.Height),
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
