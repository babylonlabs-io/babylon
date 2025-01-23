package client

import (
	"github.com/babylonlabs-io/babylon/client/babylonclient"
	"time"

	bbn "github.com/babylonlabs-io/babylon/app"
	"github.com/babylonlabs-io/babylon/client/config"
	"github.com/babylonlabs-io/babylon/client/query"
	rpchttp "github.com/cometbft/cometbft/rpc/client/http"
)

type Client struct {
	*query.QueryClient

	provider *babylonclient.CosmosProvider
	timeout  time.Duration
	cfg      *config.BabylonConfig
}

func (c *Client) Provider() *babylonclient.CosmosProvider {
	return c.provider
}

func New(cfg *config.BabylonConfig) (*Client, error) {
	// ensure cfg is valid
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	provider, err := cfg.ToCosmosProviderConfig().NewProvider(
		"", // TODO: set home path
		"babylon",
	)
	if err != nil {
		return nil, err
	}

	cp := provider.(*babylonclient.CosmosProvider)
	cp.PCfg.KeyDirectory = cfg.KeyDirectory

	// Create tmp Babylon app to retrieve and register codecs
	// Need to override this manually as otherwise option from config is ignored
	cp.Cdc = bbn.GetEncodingConfig()

	// initialise Cosmos provider
	// NOTE: this will create a RPC client. The RPC client will be used for
	// submitting txs and making ad hoc queries. It won't create WebSocket
	// connection with Babylon node
	if err = cp.Init(); err != nil {
		return nil, err
	}

	// create a queryClient so that the Client inherits all query functions
	// TODO: merge this RPC client with the one in `cp` after Cosmos side
	// finishes the migration to new RPC client
	// see https://github.com/strangelove-ventures/cometbft-client
	c, err := rpchttp.NewWithTimeout(cp.PCfg.RPCAddr, "/websocket", uint(cfg.Timeout.Seconds()))
	if err != nil {
		return nil, err
	}
	queryClient, err := query.NewWithClient(c, cfg.Timeout)
	if err != nil {
		return nil, err
	}

	return &Client{
		queryClient,
		cp,
		cfg.Timeout,
		cfg,
	}, nil
}

func (c *Client) GetConfig() *config.BabylonConfig {
	return c.cfg
}
