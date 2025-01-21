package relayerclient

import (
	"context"
	"fmt"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
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
	Input          io.Reader
	Output         io.Writer
	Cdc            Codec
	feegrantMu     sync.Mutex

	// the map key is the TX signer, which can either be 'default' (provider key) or a feegrantee
	// the purpose of the map is to lock on the signer from TX creation through submission,
	// thus making TX sequencing errors less likely.
	walletStateMap map[string]*WalletState
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
		Cdc: MakeCodec(pc.Modules, pc.AccountPrefix, pc.AccountPrefix+"valoper"),
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

// SetRpcAddr sets the rpc-addr for the chain.
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
		cc.Cdc.Marshaller,
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

	rpcClient := NewRPCClient(c)

	cc.RPCClient = rpcClient
	cc.Keybase = keybase

	return nil
}
