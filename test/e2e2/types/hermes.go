package types

import (
	"time"
)

// HermesChainConfig defines Hermes configuration for a specific chain
type HermesChainConfig struct {
	ChainConfig    *ChainConfig
	RPCAddr        string
	GRPCAddr       string
	WebSocketAddr  string
	Account        *WalletSender // HermesAccount is now a WalletSender
	GasPrice       string
	KeyName        string
	TrustingPeriod time.Duration
}

// HermesGlobalConfig defines global Hermes configuration
type HermesGlobalConfig struct {
	LogLevel      string
	TelemetryHost string
	TelemetryPort int
	RestPort      int
}

// ChannelConfig defines IBC channel configuration
type ChannelConfig struct {
	ChainA      string
	ChainB      string
	PortA       string
	PortB       string
	ChannelA    string
	ChannelB    string
	ConnectionA string
	ConnectionB string
	ClientA     string
	ClientB     string
}

// HermesConfig holds complete Hermes relayer configuration
type HermesConfig struct {
	ChainConfigs map[string]*HermesChainConfig
	Channels     []ChannelConfig
	GlobalConfig *HermesGlobalConfig
}

// HermesRelayer manages Hermes IBC relayer
type HermesRelayer struct {
	Config    *HermesConfig
	Container *Container
	Tm        *TestManager
}

// NewHermesRelayer creates a new Hermes relayer
func NewHermesRelayer(tm *TestManager) *HermesRelayer {
	return &HermesRelayer{
		Config: &HermesConfig{
			ChainConfigs: make(map[string]*HermesChainConfig),
			GlobalConfig: &HermesGlobalConfig{
				LogLevel:      "info",
				TelemetryHost: "0.0.0.0",
				TelemetryPort: 3031,
				RestPort:      3000,
			},
		},
		Tm: tm,
	}
}

// AddChain adds a chain to the relayer configuration
func (hr *HermesRelayer) AddChain(chain *Chain, account *WalletSender) error {
	rpcAddr, err := hr.getRPCAddress(chain)
	if err != nil {
		return err
	}

	grpcAddr, err := hr.getGRPCAddress(chain)
	if err != nil {
		return err
	}

	hr.Config.ChainConfigs[chain.ID] = &HermesChainConfig{
		ChainConfig:    chain.Config,
		RPCAddr:        rpcAddr,
		GRPCAddr:       grpcAddr,
		WebSocketAddr:  rpcAddr + "/websocket",
		Account:        account,
		GasPrice:       "0.001ubbn",
		KeyName:        account.KeyName,
		TrustingPeriod: 14 * 24 * time.Hour,
	}

	return nil
}

// Start starts the Hermes relayer container
func (hr *HermesRelayer) Start() {
	// TODO: Implement Hermes container startup

}

// Stop stops the Hermes relayer container
func (hr *HermesRelayer) Stop() error {
	// TODO: Implement Hermes container shutdown
	return nil
}

// CreateChannel creates an IBC channel between two chains
func (hr *HermesRelayer) CreateChannel(chainA, chainB, portA, portB string) (*ChannelConfig, error) {
	channelConfig := &ChannelConfig{
		ChainA: chainA,
		ChainB: chainB,
		PortA:  portA,
		PortB:  portB,
	}

	// TODO: Implement channel creation logic
	hr.Config.Channels = append(hr.Config.Channels, *channelConfig)
	return channelConfig, nil
}

// RelayPackets relays pending packets between chains
func (hr *HermesRelayer) RelayPackets() error {
	// TODO: Implement packet relaying
	return nil
}

// getRPCAddress gets RPC address from the first node in the chain
func (hr *HermesRelayer) getRPCAddress(chain *Chain) (string, error) {
	if len(chain.Nodes) == 0 {
		return "", nil
	}
	return chain.Nodes[0].GetRPCAddress(), nil
}

// getGRPCAddress gets GRPC address from the first node in the chain
func (hr *HermesRelayer) getGRPCAddress(chain *Chain) (string, error) {
	if len(chain.Nodes) == 0 {
		return "", nil
	}
	return chain.Nodes[0].GetGRPCAddress(), nil
}
