package initialization

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/cometbft/cometbft/crypto/ed25519"

	"cosmossdk.io/math"
	cmtconfig "github.com/cometbft/cometbft/config"
	"github.com/cometbft/cometbft/p2p"
	cmttypes "github.com/cometbft/cometbft/types"
	sdkcrypto "github.com/cosmos/cosmos-sdk/crypto"
	cryptocodec "github.com/cosmos/cosmos-sdk/crypto/codec"
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/cosmos/cosmos-sdk/server"
	srvconfig "github.com/cosmos/cosmos-sdk/server/config"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdktx "github.com/cosmos/cosmos-sdk/types/tx"
	sdksigning "github.com/cosmos/cosmos-sdk/types/tx/signing"
	authsigning "github.com/cosmos/cosmos-sdk/x/auth/signing"
	"github.com/cosmos/cosmos-sdk/x/genutil"
	genutiltypes "github.com/cosmos/cosmos-sdk/x/genutil/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/cosmos/go-bip39"
	"github.com/spf13/viper"

	checkpointingtypes "github.com/babylonlabs-io/babylon/v3/x/checkpointing/types"

	"github.com/cometbft/cometbft/privval"

	babylonApp "github.com/babylonlabs-io/babylon/v3/app"
	appparams "github.com/babylonlabs-io/babylon/v3/app/params"
	appsigner "github.com/babylonlabs-io/babylon/v3/app/signer"
	"github.com/babylonlabs-io/babylon/v3/cmd/babylond/cmd"
	"github.com/babylonlabs-io/babylon/v3/test/e2e/util"
)

type internalNode struct {
	chain        *internalChain
	moniker      string
	mnemonic     string
	keyInfo      *keyring.Record
	privateKey   cryptotypes.PrivKey
	consensusKey appsigner.ConsensusKey
	nodeKey      p2p.NodeKey
	peerId       string
	isValidator  bool
}

func newNode(chain *internalChain, nodeConfig *NodeConfig) (*internalNode, error) {
	node := &internalNode{
		chain:       chain,
		moniker:     fmt.Sprintf("%s-node-%s", chain.chainMeta.Id, nodeConfig.Name),
		isValidator: nodeConfig.IsValidator,
	}
	// creating keys comes before init
	if err := node.createKey(ValidatorWalletName); err != nil {
		return nil, err
	}
	if err := node.createConsensusKey(); err != nil {
		return nil, err
	}
	// generate genesis files
	if err := node.init(); err != nil {
		return nil, err
	}
	if err := node.createNodeKey(); err != nil {
		return nil, err
	}
	node.createAppConfig(nodeConfig)
	return node, nil
}

func (n *internalNode) configDir() string {
	return fmt.Sprintf("%s/%s", n.chain.chainMeta.configDir(), n.moniker)
}

func (n *internalNode) buildCreateValidatorMsg(amount sdk.Coin, consensusKey appsigner.ConsensusKey) (sdk.Msg, error) {
	description := stakingtypes.NewDescription(n.moniker, "", "", "", "")
	commissionRates := stakingtypes.CommissionRates{
		Rate:          math.LegacyMustNewDecFromStr("0.1"),
		MaxRate:       math.LegacyMustNewDecFromStr("0.2"),
		MaxChangeRate: math.LegacyMustNewDecFromStr("0.01"),
	}

	// get the initial validator min self delegation
	minSelfDelegation, _ := math.NewIntFromString("1")

	valPubKey, err := cryptocodec.FromCmtPubKeyInterface(n.consensusKey.Comet.PubKey)
	if err != nil {
		return nil, err
	}

	addr, err := n.keyInfo.GetAddress()
	if err != nil {
		return nil, err
	}

	stkMsgCreateVal, err := stakingtypes.NewMsgCreateValidator(
		sdk.ValAddress(addr).String(),
		valPubKey,
		amount,
		description,
		commissionRates,
		minSelfDelegation,
	)
	if err != nil {
		return nil, err
	}

	proofOfPossession, err := appsigner.BuildPoP(consensusKey.Comet.PrivKey, consensusKey.Bls.PrivKey)
	if err != nil {
		return nil, err
	}

	return checkpointingtypes.NewMsgWrappedCreateValidator(stkMsgCreateVal, &consensusKey.Bls.PubKey, proofOfPossession)
}

func (n *internalNode) createConfig() error {
	p := path.Join(n.configDir(), "config")
	return os.MkdirAll(p, 0o755)
}

func (n *internalNode) createAppConfig(nodeConfig *NodeConfig) {
	// set application configuration
	appCfgPath := filepath.Join(n.configDir(), "config", "app.toml")

	appConfig := cmd.DefaultBabylonAppConfig()

	appConfig.BaseConfig.Pruning = nodeConfig.Pruning
	appConfig.BaseConfig.PruningKeepRecent = nodeConfig.PruningKeepRecent
	appConfig.BaseConfig.PruningInterval = nodeConfig.PruningInterval
	appConfig.API.Enable = true
	appConfig.API.Address = "tcp://0.0.0.0:1317"
	appConfig.MinGasPrices = fmt.Sprintf("%s%s", MinGasPrice, BabylonDenom)
	appConfig.StateSync.SnapshotInterval = nodeConfig.SnapshotInterval
	appConfig.StateSync.SnapshotKeepRecent = nodeConfig.SnapshotKeepRecent
	appConfig.BtcConfig.Network = nodeConfig.BtcNetwork
	appConfig.GRPC.Enable = true
	appConfig.GRPC.Address = "0.0.0.0:9090"

	customTemplate := cmd.DefaultBabylonTemplate()

	srvconfig.SetConfigTemplate(customTemplate)
	srvconfig.WriteConfigFile(appCfgPath, appConfig)
}

func (n *internalNode) createNodeKey() error {
	serverCtx := server.NewDefaultContext()
	config := serverCtx.Config

	config.SetRoot(n.configDir())
	config.Moniker = n.moniker

	nodeKey, err := p2p.LoadOrGenNodeKey(config.NodeKeyFile())
	if err != nil {
		return err
	}

	n.nodeKey = *nodeKey
	return nil
}

func (n *internalNode) createConsensusKey() error {
	serverCtx := server.NewDefaultContext()
	config := serverCtx.Config
	config.SetRoot(n.configDir())
	config.Moniker = n.moniker

	pvKeyFile := config.PrivValidatorKeyFile()
	pvStateFile := config.PrivValidatorStateFile()
	blsKeyFile := appsigner.DefaultBlsKeyFile(n.configDir())
	blsPasswordFile := appsigner.DefaultBlsPasswordFile(n.configDir())

	if err := appsigner.EnsureDirs(pvKeyFile, pvStateFile, blsKeyFile, blsPasswordFile); err != nil {
		return fmt.Errorf("failed to ensure dirs: %w", err)
	}

	// create file pv
	var privKey ed25519.PrivKey
	if n.mnemonic == "" {
		privKey = ed25519.GenPrivKey()
	} else {
		privKey = ed25519.GenPrivKeyFromSecret([]byte(n.mnemonic))
	}
	filePV := privval.NewFilePV(privKey, pvKeyFile, pvStateFile)
	filePV.Key.Save()
	filePV.LastSignState.Save()

	// create bls pv
	bls := appsigner.GenBls(blsKeyFile, blsPasswordFile, "password")

	n.consensusKey = appsigner.ConsensusKey{
		Comet: &filePV.Key,
		Bls:   &bls.Key,
	}
	return nil
}

func (n *internalNode) createKeyFromMnemonic(name, mnemonic string) error {
	kb, err := keyring.New(keyringAppName, keyring.BackendTest, n.configDir(), nil, util.Cdc)

	if err != nil {
		return err
	}

	keyringAlgos, _ := kb.SupportedAlgorithms()
	algo, err := keyring.NewSigningAlgoFromString(string(hd.Secp256k1Type), keyringAlgos)
	if err != nil {
		return err
	}

	info, err := kb.NewAccount(name, mnemonic, "", sdk.FullFundraiserPath, algo)
	if err != nil {
		return err
	}

	privKeyArmor, err := kb.ExportPrivKeyArmor(name, keyringPassphrase)
	if err != nil {
		return err
	}

	privKey, _, err := sdkcrypto.UnarmorDecryptPrivKey(privKeyArmor, keyringPassphrase)
	if err != nil {
		return err
	}

	n.keyInfo = info
	n.mnemonic = mnemonic
	n.privateKey = privKey

	return nil
}

func (n *internalNode) createKey(name string) error {
	mnemonic, err := n.createMnemonic()
	if err != nil {
		return err
	}

	return n.createKeyFromMnemonic(name, mnemonic)
}

func (n *internalNode) export() *Node {
	addr, err := n.keyInfo.GetAddress()

	if err != nil {
		panic("address should be correct")
	}

	pub, err := n.keyInfo.GetPubKey()
	if err != nil {
		panic("pub key should be correct")
	}

	return &Node{
		Name:          n.moniker,
		ConfigDir:     n.configDir(),
		Mnemonic:      n.mnemonic,
		PublicAddress: addr.String(),
		WalletName:    n.keyInfo.Name,
		PublicKey:     pub.Bytes(),
		PrivateKey:    n.privateKey.Bytes(),
		PeerId:        n.peerId,
		IsValidator:   n.isValidator,
		CometPrivKey:  n.consensusKey.Comet.PrivKey.Bytes(),
	}
}

func (n *internalNode) getNodeKey() *p2p.NodeKey {
	return &n.nodeKey
}

func (n *internalNode) getAppGenesis() (*genutiltypes.AppGenesis, error) {
	serverCtx := server.NewDefaultContext()
	config := serverCtx.Config
	config.SetRoot(n.configDir())

	genFile := config.GenesisFile()
	appGenesis := &genutiltypes.AppGenesis{}

	if _, err := os.Stat(genFile); err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
	} else {
		var err error

		_, appGenesis, err = genutiltypes.GenesisStateFromGenFile(genFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read genesis doc from file: %w", err)
		}
	}

	return appGenesis, nil
}

func (n *internalNode) init() error {
	if err := n.createConfig(); err != nil {
		return err
	}

	serverCtx := server.NewDefaultContext()
	config := serverCtx.Config

	config.SetRoot(n.configDir())
	config.Moniker = n.moniker

	appGenesis, err := n.getAppGenesis()
	if err != nil {
		return err
	}

	// Create a temp app to get the default genesis state
	tempApp := babylonApp.NewTmpBabylonApp()
	appState, err := json.MarshalIndent(tempApp.DefaultGenesis(), "", " ")
	if err != nil {
		return fmt.Errorf("failed to JSON encode app genesis state: %w", err)
	}

	appGenesis.ChainID = n.chain.chainMeta.Id
	appGenesis.AppState = appState
	appGenesis.Consensus = &genutiltypes.ConsensusGenesis{
		Params: cmttypes.DefaultConsensusParams(),
	}
	appGenesis.Consensus.Params.Block.MaxGas = babylonApp.DefaultGasLimit
	appGenesis.Consensus.Params.ABCI.VoteExtensionsEnableHeight = babylonApp.DefaultVoteExtensionsEnableHeight

	if err = genutil.ExportGenesisFile(appGenesis, config.GenesisFile()); err != nil {
		return fmt.Errorf("failed to export app genesis state: %w", err)
	}

	cmtconfig.WriteConfigFile(filepath.Join(config.RootDir, "config", "config.toml"), config)
	return nil
}

func (n *internalNode) createMnemonic() (string, error) {
	entropySeed, err := bip39.NewEntropy(256)
	if err != nil {
		return "", err
	}

	mnemonic, err := bip39.NewMnemonic(entropySeed)
	if err != nil {
		return "", err
	}

	return mnemonic, nil
}

func (n *internalNode) initNodeConfigs(persistentPeers []string) error {
	cmtCfgPath := filepath.Join(n.configDir(), "config", "config.toml")

	vpr := viper.New()
	vpr.SetConfigFile(cmtCfgPath)
	if err := vpr.ReadInConfig(); err != nil {
		return err
	}

	valConfig := cmtconfig.DefaultConfig()
	if err := vpr.Unmarshal(valConfig); err != nil {
		return err
	}

	valConfig.P2P.ListenAddress = "tcp://0.0.0.0:26656"
	valConfig.P2P.AddrBookStrict = false
	valConfig.P2P.ExternalAddress = fmt.Sprintf("%s:%d", n.moniker, 26656)
	valConfig.RPC.ListenAddress = "tcp://0.0.0.0:26657"
	valConfig.StateSync.Enable = false
	valConfig.LogLevel = "debug"
	valConfig.P2P.PersistentPeers = strings.Join(persistentPeers, ",")
	valConfig.Storage.DiscardABCIResponses = false

	cmtconfig.WriteConfigFile(cmtCfgPath, valConfig)
	return nil
}

// signMsg returns a signed tx of the provided messages,
// signed by the validator, using 0 fees, a high gas limit, and a common memo.
func (n *internalNode) signMsg(msgs ...sdk.Msg) (*sdktx.Tx, error) {
	txBuilder := util.EncodingConfig.TxConfig.NewTxBuilder()

	if err := txBuilder.SetMsgs(msgs...); err != nil {
		return nil, err
	}

	txBuilder.SetMemo(fmt.Sprintf("%s@%s:26656", n.nodeKey.ID(), n.moniker))
	txBuilder.SetFeeAmount(sdk.NewCoins(sdk.NewCoin(appparams.DefaultBondDenom, math.NewInt(20000))))

	addr, err := n.keyInfo.GetAddress()
	if err != nil {
		return nil, err
	}
	pub, err := n.keyInfo.GetPubKey()
	if err != nil {
		return nil, err
	}
	// TODO: Find a better way to sign this tx with less code.
	signerData := authsigning.SignerData{
		ChainID:       n.chain.chainMeta.Id,
		AccountNumber: 0,
		Sequence:      0,
		Address:       addr.String(),
		PubKey:        pub,
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
		PubKey: pub,
		Data: &sdksigning.SingleSignatureData{
			SignMode:  sdksigning.SignMode_SIGN_MODE_DIRECT,
			Signature: nil,
		},
		Sequence: 0,
	}

	if err := txBuilder.SetSignatures(sig); err != nil {
		return nil, err
	}

	bytesToSign, err := authsigning.GetSignBytesAdapter(
		sdk.Context{}, // TODO: this is an empty context
		util.EncodingConfig.TxConfig.SignModeHandler(),
		sdksigning.SignMode_SIGN_MODE_DIRECT,
		signerData,
		txBuilder.GetTx(),
	)
	if err != nil {
		return nil, err
	}

	sigBytes, err := n.privateKey.Sign(bytesToSign)
	if err != nil {
		return nil, err
	}

	if err != nil {
		return nil, err
	}

	sig = sdksigning.SignatureV2{
		PubKey: pub,
		Data: &sdksigning.SingleSignatureData{
			SignMode:  sdksigning.SignMode_SIGN_MODE_DIRECT,
			Signature: sigBytes,
		},
		Sequence: 0,
	}
	if err := txBuilder.SetSignatures(sig); err != nil {
		return nil, err
	}

	signedTx := txBuilder.GetTx()
	bz, err := util.EncodingConfig.TxConfig.TxEncoder()(signedTx)
	if err != nil {
		return nil, err
	}

	return decodeTx(bz)
}
