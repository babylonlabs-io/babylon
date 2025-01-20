package relayerclient

import (
	"context"
	"fmt"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/cosmos/gogoproto/proto"
	commitmenttypes "github.com/cosmos/ibc-go/v8/modules/core/23-commitment/types"
	"github.com/cosmos/relayer/v2/relayer/codecs/ethermint"
	"github.com/strangelove-ventures/cometbft-client/client"
	"go.uber.org/zap"
	"io"
	"os"
	"path"
	"sync"
	"time"
)

type CosmosProviderConfig struct {
	KeyDirectory     string                  `json:"key-directory" yaml:"key-directory"`
	Key              string                  `json:"key" yaml:"key"`
	ChainName        string                  `json:"-" yaml:"-"`
	ChainID          string                  `json:"chain-id" yaml:"chain-id"`
	RPCAddr          string                  `json:"rpc-addr" yaml:"rpc-addr"`
	AccountPrefix    string                  `json:"account-prefix" yaml:"account-prefix"`
	KeyringBackend   string                  `json:"keyring-backend" yaml:"keyring-backend"`
	GasAdjustment    float64                 `json:"gas-adjustment" yaml:"gas-adjustment"`
	GasPrices        string                  `json:"gas-prices" yaml:"gas-prices"`
	MinGasAmount     uint64                  `json:"min-gas-amount" yaml:"min-gas-amount"`
	MaxGasAmount     uint64                  `json:"max-gas-amount" yaml:"max-gas-amount"`
	Debug            bool                    `json:"debug" yaml:"debug"`
	Timeout          string                  `json:"timeout" yaml:"timeout"`
	BlockTimeout     string                  `json:"block-timeout" yaml:"block-timeout"`
	OutputFormat     string                  `json:"output-format" yaml:"output-format"`
	SignModeStr      string                  `json:"sign-mode" yaml:"sign-mode"`
	ExtraCodecs      []string                `json:"extra-codecs" yaml:"extra-codecs"`
	Modules          []module.AppModuleBasic `json:"-" yaml:"-"`
	Slip44           *int                    `json:"coin-type" yaml:"coin-type"`
	SigningAlgorithm string                  `json:"signing-algorithm" yaml:"signing-algorithm"`
	Broadcast        BroadcastMode           `json:"broadcast-mode" yaml:"broadcast-mode"`
	MinLoopDuration  time.Duration           `json:"min-loop-duration" yaml:"min-loop-duration"`
	ExtensionOptions []ExtensionOption       `json:"extension-options" yaml:"extension-options"`

	// If FeeGrantConfiguration is set, TXs submitted by the ChainClient will be signed by the FeeGrantees in a round-robin fashion by default.
	FeeGrants *FeeGrantConfiguration `json:"feegrants" yaml:"feegrants"`
}

// FeeGrantConfiguration By default, TXs will be signed by the feegrantees 'ManagedGrantees' keys in a round robin fashion.
// Clients can use other signing keys by invoking 'tx.SendMsgsWith' and specifying the signing key.
type FeeGrantConfiguration struct {
	GranteesWanted int `json:"num_grantees" yaml:"num_grantees"`
	// Normally this is the default ChainClient key
	GranterKeyOrAddr string `json:"granter" yaml:"granter"`
	// Whether we control the granter private key (if not, someone else must authorize our feegrants)
	IsExternalGranter bool `json:"external_granter" yaml:"external_granter"`
	// List of keys (by name) that this FeeGranter manages
	ManagedGrantees []string `json:"grantees" yaml:"grantees"`
	// Last checked on chain (0 means grants never checked and may not exist)
	BlockHeightVerified int64 `json:"block_last_verified" yaml:"block_last_verified"`
	// Index of the last ManagedGrantee used as a TX signer
	GranteeLastSignerIndex int
}

type CosmosProvider struct {
	log *zap.Logger

	PCfg           CosmosProviderConfig
	Keybase        keyring.Keyring
	KeyringOptions []keyring.Option
	RPCClient      RPCClient
	//LightProvider  provtypes.Provider
	Input  io.Reader
	Output io.Writer
	Cdc    Codec

	//nextAccountSeq uint64
	feegrantMu sync.Mutex

	// the map key is the TX signer, which can either be 'default' (provider key) or a feegrantee
	// the purpose of the map is to lock on the signer from TX creation through submission,
	// thus making TX sequencing errors less likely.
	walletStateMap map[string]*WalletState

	// metrics to monitor the provider
	TotalFees   sdk.Coins
	totalFeesMu sync.Mutex

	// for comet < v0.37, decode tm events as base64 todo remove this
	cometLegacyEncoding bool

	// for comet < v0.38, use legacy RPC client for ResultsBlockResults todo remove this
	cometLegacyBlockResults bool
}

func (pc CosmosProviderConfig) BroadcastMode() BroadcastMode {
	return pc.Broadcast
}

type WalletState struct {
	NextAccountSequence uint64
	Mu                  sync.Mutex
}

// NewProvider validates the CosmosProviderConfig, instantiates a ChainClient and then instantiates a CosmosProvider
func (pc CosmosProviderConfig) NewProvider(log *zap.Logger, homepath string, chainName string) (ChainProvider, error) {
	if err := pc.Validate(); err != nil {
		return nil, err
	}

	pc.KeyDirectory = keysDir(homepath, pc.ChainID)

	pc.ChainName = chainName
	pc.Modules = append([]module.AppModuleBasic{}, ModuleBasics...)

	if pc.Broadcast == "" {
		pc.Broadcast = BroadcastModeBatch
	}

	cp := &CosmosProvider{
		log:            log,
		PCfg:           pc,
		KeyringOptions: []keyring.Option{ethermint.EthSecp256k1Option()},
		Input:          os.Stdin,
		Output:         os.Stdout,
		walletStateMap: map[string]*WalletState{},

		// TODO: this is a bit of a hack, we should probably have a better way to inject modules
		Cdc: MakeCodec(pc.Modules, pc.ExtraCodecs, pc.AccountPrefix, pc.AccountPrefix+"valoper"),
	}

	return cp, nil
}

func (pc CosmosProviderConfig) Validate() error {
	if _, err := time.ParseDuration(pc.Timeout); err != nil {
		return fmt.Errorf("invalid Timeout: %w", err)
	}
	return nil
}

// keysDir returns a string representing the path on the local filesystem where the keystore will be initialized.
func keysDir(home, chainID string) string {
	return path.Join(home, "keys", chainID)
}

func (cc *CosmosProvider) ProviderConfig() ProviderConfig {
	return cc.PCfg
}

func (cc *CosmosProvider) ChainId() string {
	return cc.PCfg.ChainID
}

func (cc *CosmosProvider) ChainName() string {
	return cc.PCfg.ChainName
}

func (cc *CosmosProvider) Type() string {
	return "cosmos"
}

func (cc *CosmosProvider) Key() string {
	return cc.PCfg.Key
}

func (cc *CosmosProvider) Timeout() string {
	return cc.PCfg.Timeout
}

// CommitmentPrefix returns the commitment prefix for Cosmos
func (cc *CosmosProvider) CommitmentPrefix() commitmenttypes.MerklePrefix {
	return defaultChainPrefix
}

// Address returns the chains configured address as a string
func (cc *CosmosProvider) Address() (string, error) {
	info, err := cc.Keybase.Key(cc.PCfg.Key)
	if err != nil {
		return "", err
	}

	acc, err := info.GetAddress()
	if err != nil {
		return "", err
	}

	out, err := cc.EncodeBech32AccAddr(acc)
	if err != nil {
		return "", err
	}

	return out, err
}

func (cc *CosmosProvider) MustEncodeAccAddr(addr sdk.AccAddress) string {
	enc, err := cc.EncodeBech32AccAddr(addr)
	if err != nil {
		panic(err)
	}
	return enc
}

// AccountFromKeyOrAddress returns an account from either a key or an address.
// If 'keyOrAddress' is the empty string, this returns the default key's address.
func (cc *CosmosProvider) AccountFromKeyOrAddress(keyOrAddress string) (out sdk.AccAddress, err error) {
	switch {
	case keyOrAddress == "":
		out, err = cc.GetKeyAddress(cc.PCfg.Key)
	case cc.KeyExists(keyOrAddress):
		out, err = cc.GetKeyAddress(keyOrAddress)
	default:
		out, err = sdk.GetFromBech32(keyOrAddress, cc.PCfg.AccountPrefix)
	}
	return
}

// Sprint returns the json representation of the specified proto message.
func (cc *CosmosProvider) Sprint(toPrint proto.Message) (string, error) {
	out, err := cc.Cdc.Marshaler.MarshalJSON(toPrint)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// SetPCAddr sets the rpc-addr for the chain.
// It will fail if the rpcAddr is invalid(not a url).
func (cc *CosmosProvider) SetRpcAddr(rpcAddr string) error {
	cc.PCfg.RPCAddr = rpcAddr
	return nil
}

// Init initializes the keystore, RPC client, amd light client provider.
// Once initialization is complete an attempt to query the underlying node's tendermint version is performed.
// NOTE: Init must be called after creating a new instance of CosmosProvider.
func (cc *CosmosProvider) Init(ctx context.Context) error {
	keybase, err := keyring.New(
		cc.PCfg.ChainID,
		cc.PCfg.KeyringBackend,
		cc.PCfg.KeyDirectory,
		cc.Input,
		cc.Cdc.Marshaler,
		cc.KeyringOptions...,
	)
	if err != nil {
		return err
	}
	// TODO: figure out how to deal with input or maybe just make all keyring backends test?

	timeout, err := time.ParseDuration(cc.PCfg.Timeout)
	if err != nil {
		return err
	}

	c, err := client.NewClient(cc.PCfg.RPCAddr, timeout)
	if err != nil {
		return err
	}

	//lightprovider, err := prov.New(cc.PCfg.ChainID, cc.PCfg.RPCAddr) todo check if needed
	//if err != nil {
	//	return err
	//}

	rpcClient := NewRPCClient(c)

	cc.RPCClient = rpcClient
	//cc.LightProvider = lightprovider
	cc.Keybase = keybase

	return nil
}

// WaitForNBlocks blocks until the next block on a given chain
func (cc *CosmosProvider) WaitForNBlocks(ctx context.Context, n int64) error {
	var initial int64
	h, err := cc.RPCClient.Status(ctx)
	if err != nil {
		return err
	}
	if h.SyncInfo.CatchingUp {
		return fmt.Errorf("chain catching up")
	}
	initial = h.SyncInfo.LatestBlockHeight
	for {
		h, err = cc.RPCClient.Status(ctx)
		if err != nil {
			return err
		}
		if h.SyncInfo.LatestBlockHeight > initial+n {
			return nil
		}
		select {
		case <-time.After(10 * time.Millisecond):
			// Nothing to do.
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (cc *CosmosProvider) BlockTime(ctx context.Context, height int64) (time.Time, error) {
	resultBlock, err := cc.RPCClient.Block(ctx, &height)
	if err != nil {
		return time.Time{}, err
	}
	return resultBlock.Block.Time, nil
}
