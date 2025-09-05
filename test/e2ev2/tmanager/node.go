package tmanager

import (
	"context"
	"fmt"
	"github.com/babylonlabs-io/babylon/v3/testutil/datagen"
	blc "github.com/babylonlabs-io/babylon/v3/x/btclightclient/types"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"encoding/json"

	sdkmath "cosmossdk.io/math"
	appsigner "github.com/babylonlabs-io/babylon/v3/app/signer"
	cmtconfig "github.com/cometbft/cometbft/config"
	"github.com/cometbft/cometbft/p2p"
	rpchttp "github.com/cometbft/cometbft/rpc/client/http"
	cmttypes "github.com/cometbft/cometbft/types"
	"github.com/cosmos/cosmos-sdk/codec/unknownproto"
	cryptocodec "github.com/cosmos/cosmos-sdk/crypto/codec"
	"github.com/cosmos/cosmos-sdk/server"
	srvconfig "github.com/cosmos/cosmos-sdk/server/config"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdktx "github.com/cosmos/cosmos-sdk/types/tx"
	sdksigning "github.com/cosmos/cosmos-sdk/types/tx/signing"
	authsigning "github.com/cosmos/cosmos-sdk/x/auth/signing"
	"github.com/cosmos/cosmos-sdk/x/genutil"
	genutiltypes "github.com/cosmos/cosmos-sdk/x/genutil/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"

	bbnapp "github.com/babylonlabs-io/babylon/v3/app"
	appparams "github.com/babylonlabs-io/babylon/v3/app/params"
	"github.com/babylonlabs-io/babylon/v3/cmd/babylond/cmd"
	"github.com/babylonlabs-io/babylon/v3/test/e2e/util"
	bbn "github.com/babylonlabs-io/babylon/v3/types"
	checkpointingtypes "github.com/babylonlabs-io/babylon/v3/x/checkpointing/types"
)

const (
	MinGasPrice                = "0.002"
	BabylonHomePathInContainer = "/home/babylon/babylondata"
	FlagHome                   = "--home=" + BabylonHomePathInContainer

	// waitUntilRepeatPauseTime is the time to wait between each check of the node status.
	waitUntilRepeatPauseTime = 2 * time.Second
	// waitUntilrepeatMax is the maximum number of times to repeat the wait until condition.
	waitUntilrepeatMax   = 60
	DefaultNodeWalletKey = "node-key"
)

// Node represents a blockchain node environment in a docker container
type Node struct {
	Tm          *TestManager
	ChainConfig *ChainConfig

	Name string
	Home string

	Ports     *NodePorts
	Container *Container

	NodeKeyP2P *p2p.NodeKey
	PeerID     string

	// Values are set when creating the babylon node container
	RpcClient    *rpchttp.HTTP
	GrpcEndpoint string

	// Wallets all the wallets where the keyring files were created inside this node
	// where the key is the wallet name
	Wallets map[string]*WalletSender
}

// ValidatorNode represents a validator node with additional capabilities
type ValidatorNode struct {
	*Node
	Wallet *ValidatorWallet
}

// NewNode creates a new regular node and necessary files with default wallet
func NewNode(tm *TestManager, name string, cfg *ChainConfig) *Node {
	n := NewNodeWithoutBls(tm, name, cfg)
	// even regular nodes needs bls keys
	// to avoid erros in signer.LoadOrGenBlsKey
	_, err := GenBlsKey(n.Home)
	require.NoError(n.T(), err)
	return n
}

// NewNodeWithoutBls creates a new regular node with simple ID generation
func NewNodeWithoutBls(tm *TestManager, name string, cfg *ChainConfig) *Node {
	nPorts, err := tm.PortMgr.AllocateNodePorts()
	require.NoError(tm.T, err)

	cointanerName := fmt.Sprintf("%s-%s-%s", cfg.ChainID, name, tm.NetworkID()[:4])
	n := &Node{
		Tm:          tm,
		ChainConfig: cfg,
		Name:        name,
		Home:        filepath.Join(cfg.Home, name),
		Container:   NewContainerBbnNode(cointanerName),
		Ports:       nPorts,
		Wallets:     make(map[string]*WalletSender, 0),
	}

	// each node starts with at least one wallet
	n.CreateWallet(DefaultNodeWalletKey)
	n.CreateConfigDir()
	n.WriteConfigAndGenesis()
	n.CreateNodeKeyP2P()
	n.CreateAppConfig()
	return n
}

// NewValidatorNode creates a new validator node with simple ID generation
func NewValidatorNode(tm *TestManager, name string, cfg *ChainConfig) *ValidatorNode {
	n := NewNodeWithoutBls(tm, name, cfg)

	valW := n.CreateWallet(name)
	consKey, err := CreateConsensusBlsKey(valW.Mnemonic, n.Home)
	require.NoError(n.T(), err)

	return &ValidatorNode{
		Node: n,
		Wallet: &ValidatorWallet{
			WalletSender:     valW,
			ConsKey:          consKey,
			ValidatorAddress: sdk.ValAddress(valW.Address),
			ConsensusAddress: sdk.GetConsAddress(valW.PrivKey.PubKey()),
		},
	}
}

// Start runs the container
func (n *Node) Start() {
	resource := n.RunNodeResource()

	rpcHostPort := resource.GetHostPort(fmt.Sprintf("%d/tcp", n.Ports.RPC))
	rpcClient, err := rpchttp.New("tcp://"+rpcHostPort, "/websocket")
	require.NoError(n.T(), err)
	n.RpcClient = rpcClient

	grpcHostPort := resource.GetHostPort(fmt.Sprintf("%d/tcp", n.Ports.GRPC))
	n.GrpcEndpoint = grpcHostPort
}

func (n *Node) HostPort(portID int) string {
	resource := n.ContainerResource()
	return resource.GetHostPort(fmt.Sprintf("%d/tcp", portID))
}

func (n *Node) ContainerResource() *dockertest.Resource {
	return n.Tm.ContainerManager.Resources[n.Container.Name]
}

func (n *ValidatorNode) CreateValidatorMsg(selfDelegationAmt sdk.Coin) sdk.Msg {
	description := stakingtypes.NewDescription(n.Name, "", "", "", "")
	commissionRates := stakingtypes.CommissionRates{
		Rate:          sdkmath.LegacyMustNewDecFromStr("0.1"),
		MaxRate:       sdkmath.LegacyMustNewDecFromStr("0.2"),
		MaxChangeRate: sdkmath.LegacyMustNewDecFromStr("0.01"),
	}

	// get the initial validator min self delegation
	minSelfDelegation, _ := sdkmath.NewIntFromString("1")

	valPubKey, err := cryptocodec.FromCmtPubKeyInterface(n.Wallet.ConsKey.Comet.PubKey)
	require.NoError(n.T(), err)

	stkMsgCreateVal, err := stakingtypes.NewMsgCreateValidator(
		n.Wallet.ValidatorAddress.String(),
		valPubKey,
		selfDelegationAmt,
		description,
		commissionRates,
		minSelfDelegation,
	)
	require.NoError(n.T(), err)

	proofOfPossession, err := appsigner.BuildPoP(n.Wallet.ConsKey.Comet.PrivKey, n.Wallet.ConsKey.Bls.PrivKey)
	require.NoError(n.T(), err)

	msg, err := checkpointingtypes.NewMsgWrappedCreateValidator(stkMsgCreateVal, &n.Wallet.ConsKey.Bls.PubKey, proofOfPossession)
	require.NoError(n.T(), err)
	return msg
}

// signMsg returns a signed tx of the provided messages,
// signed by the validator, using 0 fees, a high gas limit, and a common memo.
func (n *ValidatorNode) SignMsg(msgs ...sdk.Msg) *sdktx.Tx {
	txBuilder := util.EncodingConfig.TxConfig.NewTxBuilder()
	err := txBuilder.SetMsgs(msgs...)
	require.NoError(n.T(), err, "err building msg")

	// txBuilder.SetMemo(fmt.Sprintf("%s@%s:26656", n.nodeKey.ID(), n.moniker))
	txBuilder.SetFeeAmount(sdk.NewCoins(sdk.NewCoin(appparams.DefaultBondDenom, sdkmath.NewInt(20000))))

	pubKey := n.Wallet.PrivKey.PubKey()
	signerData := authsigning.SignerData{
		ChainID:       n.ChainConfig.ChainID,
		AccountNumber: 0,
		Sequence:      0,
		Address:       n.Wallet.Address.String(),
		PubKey:        pubKey,
	}

	// For SIGN_MODE_DIRECT, calling SetSignatures calls setSignerInfos on
	// TxBuilder under the hood, and SignerInfos is needed to generate the sign
	// bytes. This is the reason for setting SetSignatures here, with a nil
	// signature.
	//
	// Note: This line is not needed for SIGN_MODE_LEGACY_AMINO, but putting it
	// also doesn't affect its generated sign bytes, so for code's simplicity
	// sake, we put it here.
	sig := sdksigning.SignatureV2{
		PubKey: pubKey,
		Data: &sdksigning.SingleSignatureData{
			SignMode:  sdksigning.SignMode_SIGN_MODE_DIRECT,
			Signature: nil,
		},
		Sequence: 0,
	}

	err = txBuilder.SetSignatures(sig)
	require.NoError(n.T(), err, "err setting sigs")

	bytesToSign, err := authsigning.GetSignBytesAdapter(
		sdk.Context{}, // TODO: this is an empty context
		util.EncodingConfig.TxConfig.SignModeHandler(),
		sdksigning.SignMode_SIGN_MODE_DIRECT,
		signerData,
		txBuilder.GetTx(),
	)
	require.NoError(n.T(), err, "err get sign bytes")

	sigBytes, err := n.Wallet.PrivKey.Sign(bytesToSign)
	require.NoError(n.T(), err, "err private key sign bytes")

	sig = sdksigning.SignatureV2{
		PubKey: pubKey,
		Data: &sdksigning.SingleSignatureData{
			SignMode:  sdksigning.SignMode_SIGN_MODE_DIRECT,
			Signature: sigBytes,
		},
		Sequence: 0,
	}

	err = txBuilder.SetSignatures(sig)
	require.NoError(n.T(), err, "err setting signatures")

	signedTx := txBuilder.GetTx()
	bz, err := util.EncodingConfig.TxConfig.TxEncoder()(signedTx)
	require.NoError(n.T(), err, "err encoding tx")

	txDecoded, err := DecodeTx(bz)
	require.NoError(n.T(), err, "err decoding tx")

	return txDecoded
}

func (n *Node) WriteGenesis(genDoc *genutiltypes.AppGenesis) {
	path := filepath.Join(n.ConfigDirPath(), "genesis.json")
	err := genutil.ExportGenesisFile(genDoc, path)
	require.NoError(n.T(), err)
}

func DecodeTx(txBytes []byte) (*sdktx.Tx, error) {
	var raw sdktx.TxRaw

	// reject all unknown proto fields in the root TxRaw
	err := unknownproto.RejectUnknownFieldsStrict(txBytes, &raw, util.EncodingConfig.InterfaceRegistry)
	if err != nil {
		return nil, fmt.Errorf("failed to reject unknown fields: %w", err)
	}

	if err := util.Cdc.Unmarshal(txBytes, &raw); err != nil {
		return nil, err
	}

	var body sdktx.TxBody
	if err := util.Cdc.Unmarshal(raw.BodyBytes, &body); err != nil {
		return nil, fmt.Errorf("failed to decode tx: %w", err)
	}

	var authInfo sdktx.AuthInfo

	// reject all unknown proto fields in AuthInfo
	err = unknownproto.RejectUnknownFieldsStrict(raw.AuthInfoBytes, &authInfo, util.EncodingConfig.InterfaceRegistry)
	if err != nil {
		return nil, fmt.Errorf("failed to reject unknown fields: %w", err)
	}

	if err := util.Cdc.Unmarshal(raw.AuthInfoBytes, &authInfo); err != nil {
		return nil, fmt.Errorf("failed to decode auth info: %w", err)
	}

	return &sdktx.Tx{
		Body:       &body,
		AuthInfo:   &authInfo,
		Signatures: raw.Signatures,
	}, nil
}

func (n *Node) ConfigDirPath() string {
	return filepath.Join(n.Home, "config")
}

func (n *Node) CreateConfigDir() {
	err := os.MkdirAll(n.ConfigDirPath(), 0o755)
	require.NoError(n.T(), err)
}

func (n *Node) CreateNodeKeyP2P() {
	p2pKey, err := CreateNodeKey(n.Home, n.Name)
	require.NoError(n.T(), err)
	n.NodeKeyP2P = p2pKey

	n.PeerID = fmt.Sprintf("%s@%s:%d", n.NodeKeyP2P.ID(), n.Container.Name, n.Ports.P2P)
}

func (n *Node) CreateAppConfig() {
	appCfgPath := filepath.Join(n.ConfigDirPath(), "app.toml")

	appConfig := cmd.DefaultBabylonAppConfig()
	appConfig.BaseConfig.Pruning = "default"
	appConfig.BaseConfig.PruningKeepRecent = "0"
	appConfig.BaseConfig.PruningInterval = "0"

	appConfig.API.Enable = true
	appConfig.API.Address = n.GetRESTAddress()

	appConfig.MinGasPrices = fmt.Sprintf("%s%s", MinGasPrice, appparams.DefaultBondDenom)
	appConfig.StateSync.SnapshotInterval = 1500
	appConfig.StateSync.SnapshotKeepRecent = 2
	appConfig.BtcConfig.Network = string(bbn.BtcSimnet)

	appConfig.GRPC.Enable = true
	appConfig.GRPC.Address = n.GetGRPCAddress()

	customTemplate := cmd.DefaultBabylonTemplate()

	srvconfig.SetConfigTemplate(customTemplate)
	srvconfig.WriteConfigFile(appCfgPath, appConfig)
}

func (n *Node) WriteConfigAndGenesis() {
	serverCtx := server.NewDefaultContext()
	config := serverCtx.Config

	config.SetRoot(n.Home)
	config.Moniker = n.Name

	appGenesis, err := AppGenesisFromConfig(n.Home)
	require.NoError(n.T(), err)

	// Create a temp app to get the default genesis state
	tempApp := bbnapp.NewTmpBabylonApp()
	appState, err := json.MarshalIndent(tempApp.DefaultGenesis(), "", " ")
	require.NoError(n.T(), err)

	appGenesis.ChainID = n.ChainConfig.ChainID
	appGenesis.AppState = appState
	appGenesis.Consensus = &genutiltypes.ConsensusGenesis{
		Params: cmttypes.DefaultConsensusParams(),
	}
	appGenesis.Consensus.Params.Block.MaxGas = n.ChainConfig.GasLimit
	appGenesis.Consensus.Params.ABCI.VoteExtensionsEnableHeight = bbnapp.DefaultVoteExtensionsEnableHeight

	err = genutil.ExportGenesisFile(appGenesis, config.GenesisFile())
	require.NoError(n.T(), err)
	cmtconfig.WriteConfigFile(filepath.Join(n.ConfigDirPath(), "config.toml"), config)
}

func (n *Node) InitConfigWithPeers(persistentPeers []string) {
	cmtCfgPath := filepath.Join(n.ConfigDirPath(), "config.toml")

	vpr := viper.New()
	vpr.SetConfigFile(cmtCfgPath)
	err := vpr.ReadInConfig()
	require.NoError(n.T(), err)

	valConfig := cmtconfig.DefaultConfig()
	err = vpr.Unmarshal(valConfig)
	require.NoError(n.T(), err)

	valConfig.P2P.ListenAddress = n.GetP2PAddress()
	valConfig.P2P.AddrBookStrict = false
	valConfig.P2P.ExternalAddress = fmt.Sprintf("%s:%d", n.Container.Name, n.Ports.P2P)
	valConfig.RPC.ListenAddress = n.GetRPCAddress()
	valConfig.StateSync.Enable = false
	valConfig.LogLevel = "info"
	valConfig.P2P.PersistentPeers = strings.Join(persistentPeers, ",")
	valConfig.Storage.DiscardABCIResponses = false

	cmtconfig.WriteConfigFile(cmtCfgPath, valConfig)
}

func (n *Node) T() *testing.T {
	return n.Tm.T
}

func (n *Node) CreateWallet(keyName string) *WalletSender {
	nw := NewWalletSender(keyName, n)
	n.Wallets[keyName] = nw
	if n.IsChainRunning() {
		// set seq and acc number
		n.UpdateWalletAccSeqNumber(keyName)
	}
	return nw
}

// Wallet returns an existing wallet by name
func (n *Node) Wallet(keyName string) *WalletSender {
	return n.Wallets[keyName]
}

// SendFromDefaultWallet sends coins from the default wallet to an address
func (n *Node) SendFromDefaultWallet(recipient string, coin sdk.Coin) {
	n.SendCoins(recipient, sdk.NewCoins(coin))
}

func (n *Node) DefaultWallet() *WalletSender {
	return n.Wallets[DefaultNodeWalletKey]
}

func (n *Node) IsChainRunning() bool {
	return false
}

func (n *Node) RunNodeResource() *dockertest.Resource {
	pwd, err := os.Getwd()
	require.NoError(n.T(), err)

	if !n.Container.ImageExistsLocally() { // builds it locally if it doesn't have
		// needs to be in the path where the makefile is located '-'
		err := RunMakeCommand(filepath.Join(pwd, "../../"), "build-docker-e2e")
		require.NoError(n.T(), err)
	}

	exposedPorts := n.Ports.ContainerExposedPorts()

	// Get current user info to avoid permission issues
	currentUser, err := user.Current()
	require.NoError(n.T(), err)
	userSpec := fmt.Sprintf("%s:%s", currentUser.Uid, currentUser.Gid)

	runOpts := &dockertest.RunOptions{
		Name:       n.Container.Name,
		Repository: n.Container.Repository,
		Tag:        n.Container.Tag,
		NetworkID:  n.Tm.NetworkID(),
		User:       userSpec,
		Entrypoint: []string{
			"sh",
			"-c",
			// Use the following for debugging purposes:x
			// "export BABYLON_BLS_PASSWORD=password && babylond start " + FlagHome + " --log_level trace --trace",
			"export BABYLON_BLS_PASSWORD=password && babylond start " + FlagHome,
		},
		ExposedPorts: exposedPorts,
		Mounts: []string{
			fmt.Sprintf("%s/:%s", n.Home, BabylonHomePathInContainer),
			fmt.Sprintf("%s/bytecode:/bytecode", pwd),
		},
	}

	resource, err := n.Tm.ContainerManager.Pool.RunWithOptions(runOpts, NoRestart)
	require.NoError(n.T(), err)

	n.Tm.ContainerManager.Resources[n.Container.Name] = resource
	return resource
}

func (n *Node) WaitForNextBlock() {
	n.WaitForNextBlocks(1)
}

func (n *Node) WaitForNextBlocks(numberOfBlocks uint64) {
	latest, err := n.LatestBlockNumber()
	require.NoError(n.T(), err)
	blockToWait := latest + numberOfBlocks
	n.WaitForCondition(func() bool {
		newLatest, err := n.LatestBlockNumber()
		require.NoError(n.T(), err)
		return newLatest > blockToWait
	}, fmt.Sprintf("Timed out waiting for block %d. Current height is: %d", latest, blockToWait))
}

func (n *Node) WaitUntilBlkHeight(blkHeight uint32) {
	var (
		latestBlockHeight uint64
	)
	for i := 0; i < waitUntilrepeatMax; i++ {
		var err error
		latestBlockHeight, err = n.LatestBlockNumber()
		if err != nil {
			n.T().Logf("node %s error %s waiting for blk height %d", n.Name, err.Error(), blkHeight)
		}

		if latestBlockHeight >= uint64(blkHeight) {
			return
		}
		time.Sleep(waitUntilRepeatPauseTime)
	}
	n.T().Errorf("node %s timed out waiting for blk height %d, latest block height was %d", n.Name, blkHeight, latestBlockHeight)
}

func (n *Node) WaitForCondition(doneCondition func() bool, errorMsg string) {
	n.WaitForConditionWithPause(doneCondition, errorMsg, waitUntilRepeatPauseTime)
}

func (n *Node) WaitForConditionWithPause(doneCondition func() bool, errorMsg string, pause time.Duration) {
	for i := 0; i < waitUntilrepeatMax; i++ {
		if !doneCondition() {
			time.Sleep(pause)
			continue
		}
		return
	}
	n.T().Errorf("node %s timed out waiting for condition. Msg: %s", n.Name, errorMsg)
}

func (n *Node) Stop() error {
	return nil
}

func (n *Node) GetRPCAddress() string {
	if n.Ports == nil {
		return ""
	}
	return fmt.Sprintf("tcp://0.0.0.0:%d", n.Ports.RPC)
}

func (n *Node) GetP2PAddress() string {
	if n.Ports == nil {
		return ""
	}
	return fmt.Sprintf("tcp://0.0.0.0:%d", n.Ports.P2P)
}

func (n *Node) GetGRPCAddress() string {
	if n.Ports == nil {
		return ""
	}
	return fmt.Sprintf("0.0.0.0:%d", n.Ports.GRPC)
}

func (n *Node) GetRESTAddress() string {
	if n.Ports == nil {
		return ""
	}
	return fmt.Sprintf("tcp://0.0.0.0:%d", n.Ports.REST)
}

func (n *Node) IsHealthy() bool {
	// Implementation will be added later
	return true
}

func (n *Node) WaitForHeight(height int64) error {
	// Implementation will be added later
	return nil
}

// QueryGRPCGateway performs a query via the gRPC gateway
func (n *Node) QueryGRPCGateway(path string, params url.Values) ([]byte, error) {
	if n.Ports == nil {
		return nil, fmt.Errorf("node ports not initialized")
	}

	restHost := n.HostPort(n.Ports.REST)
	baseURL := fmt.Sprintf("http://%s", restHost)
	if len(params) > 0 {
		baseURL += "?" + params.Encode()
	}

	fullURL := baseURL + path
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to query gRPC gateway: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gRPC gateway returned status %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

// SubmitTx submits a signed transaction to the network via RPC client
func (n *Node) SubmitTx(tx *sdktx.Tx) (string, error) {
	// Convert *sdktx.Tx back to transaction bytes for broadcasting
	// We need to encode the transaction using the raw message approach
	rawTx := &sdktx.TxRaw{
		BodyBytes:     make([]byte, 0),
		AuthInfoBytes: make([]byte, 0),
		Signatures:    tx.Signatures,
	}

	// Marshal body and auth info
	if tx.Body != nil {
		bodyBytes, err := util.Cdc.Marshal(tx.Body)
		if err != nil {
			return "", fmt.Errorf("failed to marshal tx body: %w", err)
		}
		rawTx.BodyBytes = bodyBytes
	}

	if tx.AuthInfo != nil {
		authInfoBytes, err := util.Cdc.Marshal(tx.AuthInfo)
		if err != nil {
			return "", fmt.Errorf("failed to marshal auth info: %w", err)
		}
		rawTx.AuthInfoBytes = authInfoBytes
	}

	// Marshal the raw transaction
	txBytes, err := util.Cdc.Marshal(rawTx)
	if err != nil {
		return "", fmt.Errorf("failed to marshal raw transaction: %w", err)
	}

	// Submit transaction via RPC client using BroadcastTxSync
	result, err := n.RpcClient.BroadcastTxSync(context.Background(), txBytes)
	if err != nil {
		return "", fmt.Errorf("failed to broadcast transaction: %w", err)
	}

	if result.Code != 0 {
		return "", fmt.Errorf("transaction failed with code %d: %s", result.Code, result.Log)
	}

	return result.Hash.String(), nil
}

// RequireTxSuccess queries a transaction by hash and requires it to have code 0 (success)
func (n *Node) RequireTxSuccess(txHash string) {
	txResp := n.QueryTxByHash(txHash)
	require.Equal(n.T(), uint32(0), txResp.TxResponse.Code, "Transaction %s failed with code %d: %s", txHash, txResp.TxResponse.Code, txResp.TxResponse.RawLog)
}

// UpdateWalletsAccSeqNumber updates all wallets in a node by querying the chain
func (n *Node) UpdateWalletsAccSeqNumber() {
	addrs := make([]string, 0)
	keyNameByAddr := make(map[string]string, 0)
	for kName, w := range n.Wallets {
		addr := w.Addr()
		addrs = append(addrs, addr)
		keyNameByAddr[addr] = kName
	}

	accByAddr := n.QueryAllAccountInfo(addrs...)
	for addr, acc := range accByAddr {
		kName := keyNameByAddr[addr]
		num, seq := acc.GetAccountNumber(), acc.GetSequence()
		n.Wallets[kName].UpdateAccNumberAndSeq(num, seq)
	}
}

// UpdateWalletAccSeqNumber updates one wallet seq and acc number by querying the chain
func (n *Node) UpdateWalletAccSeqNumber(walletKeyName string) {
	w := n.Wallets[walletKeyName]
	num, seq := n.QueryAccountInfo(w.Addr())
	w.UpdateAccNumberAndSeq(num, seq)
}

func NoRestart(config *docker.HostConfig) {
	// in this case we don't want the nodes to restart on failure
	config.RestartPolicy = docker.RestartPolicy{
		Name: "no",
	}
}

func RunCommand(command string) error {
	fmt.Printf("Running command %s...\n", command)
	cmd := exec.Command(command)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func RunMakeCommand(path, command string) error {
	makePath, err := exec.LookPath("make")
	if err != nil {
		return fmt.Errorf("make command not found: %w", err)
	}

	fmt.Printf("Running make in path %s command %s...\n", path, command)
	cmd := exec.Command(makePath, command)
	cmd.Dir = path

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to build image: %w\nOutput: %s", err, string(output))
	}

	fmt.Printf("✓ Successfully built\n")
	return nil
}

func AppGenesisFromConfig(rootPath string) (*genutiltypes.AppGenesis, error) {
	serverCtx := server.NewDefaultContext()
	config := serverCtx.Config
	config.SetRoot(rootPath)

	genFile := config.GenesisFile()

	_, err := os.Stat(genFile)
	if err == nil {
		_, appGenesis, err := genutiltypes.GenesisStateFromGenFile(genFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read genesis doc from file: %w", err)
		}

		return appGenesis, nil
	}

	// if it doesn't exist just returns it empty
	if !os.IsNotExist(err) {
		return nil, err
	}
	return &genutiltypes.AppGenesis{}, nil
}

func CreateNodeKey(rootDir, moniker string) (*p2p.NodeKey, error) {
	serverCtx := server.NewDefaultContext()
	config := serverCtx.Config

	config.SetRoot(rootDir)
	config.Moniker = moniker

	nodeKey, err := p2p.LoadOrGenNodeKey(config.NodeKeyFile())
	if err != nil {
		return nil, err
	}

	return nodeKey, nil
}

// QueryTip queries the current tip of the BTC light client
func (n *Node) QueryTip() (*blc.BTCHeaderInfoResponse, error) {
	bz, err := n.QueryGRPCGateway("/babylon/btclightclient/v1/tip", url.Values{})
	if err != nil {
		return nil, err
	}

	var blcResponse blc.QueryTipResponse
	if err := util.Cdc.UnmarshalJSON(bz, &blcResponse); err != nil {
		return nil, err
	}

	return blcResponse.Header, nil
}

// WaitUntilBtcHeight waits until the BTC height reaches the specified height
func (n *Node) WaitUntilBtcHeight(height uint32) {
	var latestBlockHeight uint32
	n.WaitForCondition(func() bool {
		btcTip, err := n.QueryTip()
		require.NoError(n.T(), err)
		latestBlockHeight = btcTip.Height

		return latestBlockHeight >= height
	}, fmt.Sprintf("Timed out waiting for btc height %d", height))
}

// InsertNewEmptyBtcHeader inserts a new BTC header to the chain
func (n *Node) InsertNewEmptyBtcHeader(r *rand.Rand) *blc.BTCHeaderInfo {
	tipResp, err := n.QueryTip()
	require.NoError(n.T(), err)
	n.T().Logf("Retrieved current tip of btc headerchain. Height: %d", tipResp.Height)

	tip, err := ParseBTCHeaderInfoResponseToInfo(tipResp)
	require.NoError(n.T(), err)

	child := datagen.GenRandomValidBTCHeaderInfoWithParent(r, *tip)
	n.SendHeaderHex(child.Header.MarshalHex())
	n.WaitUntilBtcHeight(tipResp.Height + 1)
	return child
}

// SendHeaderHex sends a BTC header in hex format to the node
func (n *Node) SendHeaderHex(headerHex string) {
	wallet := n.Wallet("node-key")

	headerBytes, err := bbn.NewBTCHeaderBytesFromHex(headerHex)
	require.NoError(n.T(), err)

	msg := &blc.MsgInsertHeaders{
		Signer:  wallet.Address.String(),
		Headers: []bbn.BTCHeaderBytes{headerBytes},
	}

	_, tx := wallet.SubmitMsgs(msg)
	require.NotNil(n.T(), tx, "RegisterConsumerChain transaction should not be nil")
}
