package wasmbinding

import (
	"encoding/json"
	"fmt"

	errorsmod "cosmossdk.io/errors"
	wasmkeeper "github.com/CosmWasm/wasmd/x/wasm/keeper"
	wasmvmtypes "github.com/CosmWasm/wasmvm/v2/types"
	bbn "github.com/babylonlabs-io/babylon/types"
	"github.com/babylonlabs-io/babylon/wasmbinding/bindings"
	lcKeeper "github.com/babylonlabs-io/babylon/x/btclightclient/keeper"
	epochingkeeper "github.com/babylonlabs-io/babylon/x/epoching/keeper"
	zckeeper "github.com/babylonlabs-io/babylon/x/zoneconcierge/keeper"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

type QueryPlugin struct {
	epochingKeeper *epochingkeeper.Keeper
	zcKeeper       *zckeeper.Keeper
	lcKeeper       *lcKeeper.Keeper
}

// NewQueryPlugin returns a reference to a new QueryPlugin.
func NewQueryPlugin(
	ek *epochingkeeper.Keeper,
	zcKeeper *zckeeper.Keeper,
	lcKeeper *lcKeeper.Keeper,
) *QueryPlugin {
	return &QueryPlugin{
		epochingKeeper: ek,
		zcKeeper:       zcKeeper,
		lcKeeper:       lcKeeper,
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
			epoch := qp.zcKeeper.GetLastFinalizedEpoch(ctx)
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

			headerInfo := qp.lcKeeper.GetHeaderByHash(ctx, &headerHash)

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
	ek *epochingkeeper.Keeper,
	zcKeeper *zckeeper.Keeper,
	lcKeeper *lcKeeper.Keeper,
) []wasmkeeper.Option {
	wasmQueryPlugin := NewQueryPlugin(ek, zcKeeper, lcKeeper)

	queryPluginOpt := wasmkeeper.WithQueryPlugins(&wasmkeeper.QueryPlugins{
		Custom: CustomQuerier(wasmQueryPlugin),
	})

	return []wasmkeeper.Option{
		queryPluginOpt,
	}
}
