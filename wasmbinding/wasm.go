package wasmbinding

import (
	"encoding/json"
	"fmt"

	tokenfactorykeeper "github.com/strangelove-ventures/tokenfactory/x/tokenfactory/keeper"

	errorsmod "cosmossdk.io/errors"
	wasmkeeper "github.com/CosmWasm/wasmd/x/wasm/keeper"
	wasmvmtypes "github.com/CosmWasm/wasmvm/v2/types"
	bbn "github.com/babylonlabs-io/babylon/v4/types"
	"github.com/babylonlabs-io/babylon/v4/wasmbinding/bindings"
	lcKeeper "github.com/babylonlabs-io/babylon/v4/x/btclightclient/keeper"
	checkpointingkeeper "github.com/babylonlabs-io/babylon/v4/x/checkpointing/keeper"
	epochingkeeper "github.com/babylonlabs-io/babylon/v4/x/epoching/keeper"
	zckeeper "github.com/babylonlabs-io/babylon/v4/x/zoneconcierge/keeper"
	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

type QueryPlugin struct {
	tokenfactoryKeeper  *tokenfactorykeeper.Keeper
	epochingKeeper      *epochingkeeper.Keeper
	checkpointingkeeper *checkpointingkeeper.Keeper
	lcKeeper            *lcKeeper.Keeper
	zcKeeper            *zckeeper.Keeper
}

// NewQueryPlugin returns a reference to a new QueryPlugin.
func NewQueryPlugin(
	tk *tokenfactorykeeper.Keeper,
	ek *epochingkeeper.Keeper,
	ch *checkpointingkeeper.Keeper,
	lcKeeper *lcKeeper.Keeper,
	zcKeeper *zckeeper.Keeper,
) *QueryPlugin {
	return &QueryPlugin{
		tokenfactoryKeeper:  tk,
		epochingKeeper:      ek,
		checkpointingkeeper: ch,
		lcKeeper:            lcKeeper,
		zcKeeper:            zcKeeper,
	}
}

// CustomQuerier dispatches custom CosmWasm bindings queries.
func CustomQuerier(qp *QueryPlugin) func(ctx sdk.Context, request json.RawMessage) ([]byte, error) {
	return func(ctx sdk.Context, request json.RawMessage) ([]byte, error) {
		var contractQuery bindings.BabylonQuery
		if err := json.Unmarshal(request, &contractQuery); err != nil {
			return nil, errorsmod.Wrap(err, "failed to unarshall request ")
		}

		switch {
		case contractQuery.Epoch != nil:
			epoch := qp.epochingKeeper.GetEpoch(ctx)
			res := bindings.CurrentEpochResponse{
				Epoch: epoch.EpochNumber,
			}

			bz, err := json.Marshal(res)
			if err != nil {
				return nil, errorsmod.Wrap(err, "failed marshaling")
			}

			return bz, nil

		case contractQuery.LatestFinalizedEpochInfo != nil:
			epoch := qp.checkpointingkeeper.GetLastFinalizedEpoch(ctx)

			epochInfo, err := qp.epochingKeeper.GetHistoricalEpoch(ctx, epoch)

			if err != nil {
				// Here something went really wrong with our data model. If epoch is finalized
				// it should always be known by epoching module
				panic(fmt.Sprintf("Finalized epoch %d not known by epoching module", epoch))
			}

			res := bindings.LatestFinalizedEpochInfoResponse{
				EpochInfo: &bindings.FinalizedEpochInfo{
					EpochNumber:     epoch,
					LastBlockHeight: epochInfo.GetLastBlockHeight(),
				},
			}

			bz, err := json.Marshal(res)
			if err != nil {
				return nil, errorsmod.Wrap(err, "failed marshaling")
			}

			return bz, nil
		case contractQuery.BtcTip != nil:
			tip := qp.lcKeeper.GetTipInfo(ctx)
			if tip == nil {
				return nil, fmt.Errorf("no tip info found")
			}

			res := bindings.BtcTipResponse{
				HeaderInfo: bindings.AsBtcBlockHeaderInfo(tip),
			}

			bz, err := json.Marshal(res)

			if err != nil {
				return nil, errorsmod.Wrap(err, "failed marshaling")
			}

			return bz, nil
		case contractQuery.BtcBaseHeader != nil:
			baseHeader := qp.lcKeeper.GetBaseBTCHeader(ctx)

			if baseHeader == nil {
				return nil, fmt.Errorf("no base header found")
			}

			res := bindings.BtcBaseHeaderResponse{
				HeaderInfo: bindings.AsBtcBlockHeaderInfo(baseHeader),
			}

			bz, err := json.Marshal(res)

			if err != nil {
				return nil, errorsmod.Wrap(err, "failed marshaling")
			}

			return bz, nil
		case contractQuery.BtcHeaderByHash != nil:
			headerHash, err := bbn.NewBTCHeaderHashBytesFromHex(contractQuery.BtcHeaderByHash.Hash)

			if err != nil {
				return nil, errorsmod.Wrap(err, "failed to parse header hash")
			}

			headerInfo, err := qp.lcKeeper.GetHeaderByHash(ctx, &headerHash)
			if err != nil {
				return nil, errorsmod.Wrapf(err, "failed to get header hash: %s", err.Error())
			}

			res := bindings.BtcHeaderQueryResponse{
				HeaderInfo: bindings.AsBtcBlockHeaderInfo(headerInfo),
			}
			bz, err := json.Marshal(res)

			if err != nil {
				return nil, errorsmod.Wrap(err, "failed marshaling")
			}

			return bz, nil
		case contractQuery.BtcHeaderByHeight != nil:
			headerInfo := qp.lcKeeper.GetHeaderByHeight(ctx, contractQuery.BtcHeaderByHeight.Height)

			res := bindings.BtcHeaderQueryResponse{
				HeaderInfo: bindings.AsBtcBlockHeaderInfo(headerInfo),
			}
			bz, err := json.Marshal(res)

			if err != nil {
				return nil, errorsmod.Wrap(err, "failed marshaling")
			}

			return bz, nil
		default:
			return nil, wasmvmtypes.UnsupportedRequest{Kind: "unknown babylon query variant"}
		}
	}
}

func RegisterCustomPlugins(
	tk *tokenfactorykeeper.Keeper,
	ek *epochingkeeper.Keeper,
	ck *checkpointingkeeper.Keeper,
	lcKeeper *lcKeeper.Keeper,
	zcKeeper *zckeeper.Keeper,
) []wasmkeeper.Option {
	wasmQueryPlugin := NewQueryPlugin(tk, ek, ck, lcKeeper, zcKeeper)

	queryPluginOpt := wasmkeeper.WithQueryPlugins(&wasmkeeper.QueryPlugins{
		Custom: CustomQuerier(wasmQueryPlugin),
	})

	return []wasmkeeper.Option{
		queryPluginOpt,
	}
}

func RegisterGrpcQueries(queryRouter baseapp.GRPCQueryRouter, codec codec.Codec) []wasmkeeper.Option {
	queryPluginOpt := wasmkeeper.WithQueryPlugins(
		&wasmkeeper.QueryPlugins{
			Stargate: wasmkeeper.AcceptListStargateQuerier(WhitelistedGrpcQuery(), &queryRouter, codec),
			Grpc:     wasmkeeper.AcceptListGrpcQuerier(WhitelistedGrpcQuery(), &queryRouter, codec),
		})

	return []wasmkeeper.Option{
		queryPluginOpt,
	}
}
