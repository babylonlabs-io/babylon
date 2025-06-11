package prepare_test

import (
	"bytes"
	"fmt"
	sdktestutil "github.com/cosmos/cosmos-sdk/types/module/testutil"
	"math/rand"
	"sort"
	"testing"
	"time"

	"cosmossdk.io/core/header"
	"cosmossdk.io/log"
	cbftt "github.com/cometbft/cometbft/abci/types"
	cmtprotocrypto "github.com/cometbft/cometbft/proto/tendermint/crypto"
	tendermintTypes "github.com/cometbft/cometbft/proto/tendermint/types"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/client"
	cryptocodec "github.com/cosmos/cosmos-sdk/crypto/codec"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdktestdata "github.com/cosmos/cosmos-sdk/testutil/testdata"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/mempool"
	signingtypes "github.com/cosmos/cosmos-sdk/types/tx/signing"
	protoio "github.com/cosmos/gogoproto/io"
	"github.com/cosmos/gogoproto/proto"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/babylonlabs-io/babylon/v4/app/ante"
	"github.com/babylonlabs-io/babylon/v4/crypto/bls12381"
	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	"github.com/babylonlabs-io/babylon/v4/testutil/helper"
	"github.com/babylonlabs-io/babylon/v4/testutil/mocks"
	"github.com/babylonlabs-io/babylon/v4/x/checkpointing/prepare"
	checkpointingtypes "github.com/babylonlabs-io/babylon/v4/x/checkpointing/types"
	et "github.com/babylonlabs-io/babylon/v4/x/epoching/types"
	ftypes "github.com/babylonlabs-io/babylon/v4/x/finality/types"
)

type TestValidator struct {
	Keys  *datagen.GenesisKeyWithBLS
	Power int64
}

func (v *TestValidator) CometValidator() *cbftt.Validator {
	return &cbftt.Validator{
		Address: v.Keys.GenesisKey.ValPubkey.Address(),
		Power:   v.Power,
	}
}

func (v *TestValidator) EpochingValidator() et.Validator {
	return et.Validator{
		Addr:  v.Keys.GenesisKey.ValPubkey.Address(),
		Power: v.Power,
	}
}

func (v *TestValidator) ProtoPubkey() cmtprotocrypto.PublicKey {
	validatorPubKey := cmtprotocrypto.PublicKey{
		Sum: &cmtprotocrypto.PublicKey_Ed25519{
			Ed25519: v.Keys.PrivKey.PubKey().Bytes(),
		},
	}
	return validatorPubKey
}

func (v *TestValidator) VoteExtension(
	bh *checkpointingtypes.BlockHash,
	epochNum uint64,
) checkpointingtypes.VoteExtension {
	signBytes := checkpointingtypes.GetSignBytes(epochNum, *bh)
	// Need valid bls signature for aggregation
	bls := bls12381.Sign(v.Keys.PrivateKey, signBytes)

	return checkpointingtypes.VoteExtension{
		Signer:    v.Keys.ValidatorAddress,
		BlockHash: bh,
		EpochNum:  epochNum,
		Height:    0,
		BlsSig:    &bls,
	}
}

func (v *TestValidator) SignVoteExtension(
	t *testing.T,
	bytes []byte,
	height int64,
	chainId string,
) cbftt.ExtendedVoteInfo {
	votExt := genVoteExt(t,
		bytes, height, 0, chainId)
	signature, err := v.Keys.PrivKey.Sign(votExt)
	require.NoError(t, err)

	evi := cbftt.ExtendedVoteInfo{
		Validator:          *v.CometValidator(),
		VoteExtension:      bytes,
		ExtensionSignature: signature,
		BlockIdFlag:        tendermintTypes.BlockIDFlagCommit,
	}

	return evi
}

func (v *TestValidator) ValidatorAddress(t *testing.T) sdk.ValAddress {
	valAddress, err := sdk.ValAddressFromBech32(v.Keys.ValidatorAddress)
	require.NoError(t, err)
	return valAddress
}

func (v *TestValidator) BlsPubKey() bls12381.PublicKey {
	return *v.Keys.BlsKey.Pubkey
}

func genNTestValidators(t *testing.T, n int) []TestValidator {
	if n == 0 {
		return []TestValidator{}
	}

	keys, err := datagen.GenesisValidatorSet(n)
	require.NoError(t, err)

	var vals []TestValidator
	for _, key := range keys.Keys {
		k := key
		vals = append(vals, TestValidator{
			Keys:  k,
			Power: 100,
		})
	}

	// below are copied from https://github.com/cosmos/cosmos-sdk/blob/v0.50.6/baseapp/abci_utils_test.go
	// Since v0.50.5 Cosmos SDK enforces certain order for vote extensions
	sort.SliceStable(vals, func(i, j int) bool {
		if vals[i].Power == vals[j].Power {
			valAddress1, err := sdk.ValAddressFromBech32(vals[i].Keys.ValidatorAddress)
			require.NoError(t, err)
			valAddress2, err := sdk.ValAddressFromBech32(vals[j].Keys.ValidatorAddress)
			require.NoError(t, err)
			return bytes.Compare(valAddress1, valAddress2) == -1
		}
		return vals[i].Power > vals[j].Power
	})

	return vals
}

func setupSdkCtx(height int64) sdk.Context {
	return sdk.Context{}.WithHeaderInfo(header.Info{
		Height:  height,
		Time:    time.Now(),
		ChainID: "test",
	}).WithConsensusParams(tendermintTypes.ConsensusParams{
		Abci: &tendermintTypes.ABCIParams{
			VoteExtensionsEnableHeight: 1,
		},
	}).WithChainID("test")
}

func firstEpoch() *et.Epoch {
	return &et.Epoch{
		EpochNumber:          1,
		CurrentEpochInterval: 10,
		FirstBlockHeight:     1,
	}
}

type EpochAndCtx struct {
	Epoch *et.Epoch
	Ctx   sdk.Context
}

func epochAndVoteExtensionCtx() *EpochAndCtx {
	epoch := firstEpoch()
	ctx := setupSdkCtx(int64(epoch.FirstBlockHeight) + int64(epoch.GetCurrentEpochInterval()))
	return &EpochAndCtx{
		Epoch: epoch,
		Ctx:   ctx,
	}
}

func genVoteExt(
	t *testing.T,
	ext []byte,
	height int64,
	round int64,
	chainID string,
) []byte {
	cve := tendermintTypes.CanonicalVoteExtension{
		Extension: ext,
		Height:    height, // the vote extension was signed in the previous height
		Round:     round,
		ChainId:   chainID,
	}

	marshalDelimitedFn := func(msg proto.Message) ([]byte, error) {
		var buf bytes.Buffer
		if err := protoio.NewDelimitedWriter(&buf).WriteMsg(msg); err != nil {
			return nil, err
		}

		return buf.Bytes(), nil
	}

	extSignBytes, err := marshalDelimitedFn(&cve)
	require.NoError(t, err)
	return extSignBytes
}

func requestPrepareProposal(height int64, commitInfo cbftt.ExtendedCommitInfo) *cbftt.RequestPrepareProposal {
	return &cbftt.RequestPrepareProposal{
		MaxTxBytes:      10000,
		Txs:             [][]byte{},
		LocalLastCommit: commitInfo,
		Height:          height,
	}
}

func randomBlockHash() checkpointingtypes.BlockHash {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	return datagen.GenRandomBlockHash(r)
}

// TODO There should be one function to verify the checkpoint against the validator set
// but currently there are different implementations in the codebase in checpointing module
// and zonecocierge module
func verifyCheckpoint(validators []TestValidator, rawCkpt *checkpointingtypes.RawCheckpoint) error {
	valsCopy := validators

	sort.Slice(valsCopy, func(i, j int) bool {
		return sdk.BigEndianToUint64(valsCopy[i].EpochingValidator().Addr) < sdk.BigEndianToUint64(valsCopy[j].EpochingValidator().Addr)
	})

	var validatorWithBls []*checkpointingtypes.ValidatorWithBlsKey

	for _, val := range valsCopy {
		validatorWithBls = append(validatorWithBls, &checkpointingtypes.ValidatorWithBlsKey{
			ValidatorAddress: val.Keys.ValidatorAddress,
			BlsPubKey:        val.BlsPubKey(),
			VotingPower:      uint64(val.Power),
		})
	}

	valSet := &checkpointingtypes.ValidatorWithBlsKeySet{ValSet: validatorWithBls}
	// filter validator set that contributes to the signature
	signerSet, signerSetPower, err := valSet.FindSubsetWithPowerSum(rawCkpt.Bitmap)
	if err != nil {
		return err
	}
	// ensure the signerSet has > 2/3 voting power
	if signerSetPower*3 <= valSet.GetTotalPower()*2 {
		return fmt.Errorf("failed")
	}
	// verify BLS multisig
	signedMsgBytes := rawCkpt.SignedMsg()
	ok, err := bls12381.VerifyMultiSig(*rawCkpt.BlsMultiSig, signerSet.GetBLSKeySet(), signedMsgBytes)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("BLS signature does not match the public key")
	}
	return nil
}

type Scenario struct {
	TotalPower   int64
	ValidatorSet []TestValidator
	Extensions   []cbftt.ExtendedVoteInfo
	Txs          []sdk.Tx
	TxVerifier   baseapp.ProposalTxVerifier
}

type ValidatorsAndExtensions struct {
	Vals       []TestValidator
	Extensions []checkpointingtypes.VoteExtension
}

func generateNValidatorAndVoteExtensions(t *testing.T, n int, bh *checkpointingtypes.BlockHash, epochNumber uint64) (*ValidatorsAndExtensions, int64) {
	validators := genNTestValidators(t, n)
	var extensions []checkpointingtypes.VoteExtension
	var power int64
	for _, val := range validators {
		validator := val
		ve := validator.VoteExtension(bh, epochNumber)
		extensions = append(extensions, ve)
		power += validator.Power
	}

	return &ValidatorsAndExtensions{
		Vals:       validators,
		Extensions: extensions,
	}, power
}

func ToValidatorSet(v []TestValidator) et.ValidatorSet {
	var cv []et.Validator
	for _, val := range v {
		cv = append(cv, val.EpochingValidator())
	}
	return et.NewSortedValidatorSet(cv)
}

// addTxsToMempool is a helper function to add the transactions to the
// provided mempool. Uses the Priority anteHandler decorator
// to set the tx priority
func addTxsToMempool(txs []sdk.Tx, mp mempool.Mempool) error {
	if len(txs) == 0 {
		return nil
	}
	ctx := sdk.Context{}.WithPriority(100)
	deco := ante.NewPriorityDecorator()

	next := func(ctx sdk.Context, tx sdk.Tx, simulate bool) (sdk.Context, error) {
		return ctx, nil
	}
	for _, tx := range txs {
		newCtx, err := deco.AnteHandle(ctx, tx, false, next)
		if err != nil {
			return err
		}
		if err := mp.Insert(newCtx, tx); err != nil {
			return err
		}
	}

	return nil
}

func buildTx(txConfig client.TxConfig, nonce uint64, msgs []sdk.Msg) (sdk.Tx, error) {
	builder := txConfig.NewTxBuilder()
	if err := builder.SetMsgs(msgs...); err != nil {
		return nil, err
	}
	builder.SetGasLimit(100)
	if err := setTxSignature(builder, nonce); err != nil {
		return nil, err
	}
	return builder.GetTx(), nil
}

func setTxSignature(builder client.TxBuilder, nonce uint64) error {
	h := randomBlockHash()
	privKey := secp256k1.GenPrivKeyFromSecret([]byte(h.String()))
	pubKey := privKey.PubKey()
	return builder.SetSignatures(
		signingtypes.SignatureV2{
			PubKey:   pubKey,
			Sequence: nonce,
			Data:     &signingtypes.SingleSignatureData{},
		},
	)
}

func regularTx(txConf client.TxConfig) (sdk.Tx, error) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	nonce := datagen.RandomUInt32(r, 10_000)
	msgs := []sdk.Msg{
		&sdktestdata.TestMsg{},
	}
	return buildTx(txConf, uint64(nonce), msgs)
}

func livenessTx(txConf client.TxConfig) (sdk.Tx, error) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	nonce := datagen.RandomUInt32(r, 10_000)
	msgs := []sdk.Msg{
		&ftypes.MsgAddFinalitySig{},
	}
	return buildTx(txConf, uint64(nonce), msgs)
}

func isRegularTx(txDecoder sdk.TxDecoder, txBz []byte) (bool, error) {
	tx, err := txDecoder(txBz)
	if err != nil {
		return false, err
	}
	msgs := tx.GetMsgs()
	for _, msg := range msgs {
		if _, ok := msg.(*sdktestdata.TestMsg); !ok {
			return false, nil
		}
	}
	return true, nil
}

func isLivenessTx(txDecoder sdk.TxDecoder, txBz []byte) (bool, error) {
	tx, err := txDecoder(txBz)
	if err != nil {
		return false, err
	}
	msgs := tx.GetMsgs()
	for _, msg := range msgs {
		// a tx with only one liveness-related msg is a livenessTx
		if _, ok := msg.(*ftypes.MsgAddFinalitySig); ok {
			return true, nil
		}
	}
	return false, nil
}

func genRandomTxs(t *testing.T, txConf client.TxConfig, regTxsCount, livenessTxsCount int) []sdk.Tx {
	var txs []sdk.Tx
	// Generate regular transactions
	for i := 0; i < regTxsCount; i++ {
		tx, err := regularTx(txConf)
		require.NoError(t, err)
		txs = append(txs, tx)
	}

	// Generate liveness transactions
	for i := 0; i < livenessTxsCount; i++ {
		tx, err := livenessTx(txConf)
		require.NoError(t, err)
		txs = append(txs, tx)
	}

	// Shuffle transactions randomly
	rand.Shuffle(len(txs), func(i, j int) { txs[i], txs[j] = txs[j], txs[i] })
	return txs
}

func verifyTxOrder(t *testing.T, txs [][]byte, txDecoder sdk.TxDecoder, regTxsCount, livenessTxsCount int) {
	if len(txs) == 1 {
		return
	}
	// Expect to have one extra tx, the injected checkpoint tx
	require.Equal(t, regTxsCount+livenessTxsCount+1, len(txs), "Unexpected number of transactions")
	// Skip the first transaction which is the injected checkpoint tx
	for i, txBz := range txs[1:] {
		isLiveness, err := isLivenessTx(txDecoder, txBz)
		require.NoError(t, err, "Error decoding transaction at index %d", i+1)

		if i < livenessTxsCount { // First transactions should be liveness txs
			require.True(t, isLiveness, "Expected a liveness transaction at index %d", i+1)
		} else { // The remaining transactions should be regular txs
			isRegular, err := isRegularTx(txDecoder, txBz)
			require.NoError(t, err, "Error decoding transaction at index %d", i+1)
			require.True(t, isRegular, "Expected a regular transaction at index %d", i+1)
		}
	}
}

// txVerifier is a dummy tx verifier used in tests
type txVerifier struct {
	txEncodConfig client.TxEncodingConfig
}

// PrepareProposalVerifyTx implements baseapp.ProposalTxVerifier.
func (m txVerifier) PrepareProposalVerifyTx(tx sdk.Tx) ([]byte, error) {
	return m.txEncodConfig.TxEncoder()(tx)
}

// ProcessProposalVerifyTx implements baseapp.ProposalTxVerifier.
func (m txVerifier) ProcessProposalVerifyTx(txBz []byte) (sdk.Tx, error) {
	return m.txEncodConfig.TxDecoder()(txBz)
}

// TxDecode implements baseapp.ProposalTxVerifier.
func (m txVerifier) TxDecode(txBz []byte) (sdk.Tx, error) {
	return m.txEncodConfig.TxDecoder()(txBz)
}

// TxEncode implements baseapp.ProposalTxVerifier.
func (m txVerifier) TxEncode(tx sdk.Tx) ([]byte, error) {
	return m.txEncodConfig.TxEncoder()(tx)
}

func newTxVerifier(txEnc client.TxEncodingConfig) baseapp.ProposalTxVerifier {
	return txVerifier{txEncodConfig: txEnc}
}

func TestPrepareProposalAtVoteExtensionHeight(t *testing.T) {
	var (
		r               = rand.New(rand.NewSource(time.Now().UnixNano()))
		encCfg          = sdktestutil.MakeTestEncodingConfig()
		regularTxCount  = int(datagen.RandomInt(r, 10))
		livenessTxCount = int(datagen.RandomInt(r, 10))
	)
	sdktestdata.RegisterInterfaces(encCfg.InterfaceRegistry)
	ftypes.RegisterInterfaces(encCfg.InterfaceRegistry)
	cryptocodec.RegisterInterfaces(encCfg.InterfaceRegistry)

	tests := []struct {
		name          string
		scenarioSetup func(ec *EpochAndCtx, ek *mocks.MockCheckpointingKeeper) *Scenario
		expectError   bool
	}{
		{
			name: "Empty vote extension list ",
			scenarioSetup: func(ec *EpochAndCtx, ek *mocks.MockCheckpointingKeeper) *Scenario {
				bh := randomBlockHash()
				validatorAndExtensions, totalPower := generateNValidatorAndVoteExtensions(t, 4, &bh, ec.Epoch.EpochNumber)
				return &Scenario{
					TotalPower:   totalPower,
					ValidatorSet: validatorAndExtensions.Vals,
					Extensions:   []cbftt.ExtendedVoteInfo{},
				}
			},
			expectError: true,
		},
		{
			name: "List with only empty vote extensions",
			scenarioSetup: func(ec *EpochAndCtx, ek *mocks.MockCheckpointingKeeper) *Scenario {
				bh := randomBlockHash()
				validatorAndExtensions, totalPower := generateNValidatorAndVoteExtensions(t, 4, &bh, ec.Epoch.EpochNumber)
				var signedVoteExtensions []cbftt.ExtendedVoteInfo
				for i, val := range validatorAndExtensions.Vals {
					validator := val
					ek.EXPECT().GetPubKeyByConsAddr(gomock.Any(), sdk.ConsAddress(validator.ValidatorAddress(t).Bytes())).Return(validator.ProtoPubkey(), nil).AnyTimes()
					ek.EXPECT().VerifyBLSSig(gomock.Any(), validatorAndExtensions.Extensions[i].ToBLSSig()).Return(nil).AnyTimes()
					ek.EXPECT().GetBlsPubKey(gomock.Any(), validator.ValidatorAddress(t)).Return(validator.BlsPubKey(), nil).AnyTimes()
					// empty vote extension
					signedExtension := validator.SignVoteExtension(t, []byte{}, ec.Ctx.HeaderInfo().Height-1, ec.Ctx.ChainID())
					signedVoteExtensions = append(signedVoteExtensions, signedExtension)
				}

				return &Scenario{
					TotalPower:   totalPower,
					ValidatorSet: validatorAndExtensions.Vals,
					Extensions:   signedVoteExtensions,
				}
			},
			expectError: true,
		},
		{
			name: "1/3 of validators provided invalid bls signature",
			scenarioSetup: func(ec *EpochAndCtx, ek *mocks.MockCheckpointingKeeper) *Scenario {
				bh := randomBlockHash()
				// each validator has the same voting power
				numValidators := 9
				invalidValidBlsSig := numValidators / 3

				validatorAndExtensions, totalPower := generateNValidatorAndVoteExtensions(t, numValidators, &bh, ec.Epoch.EpochNumber)

				var signedVoteExtensions []cbftt.ExtendedVoteInfo
				for i, val := range validatorAndExtensions.Vals {
					validator := val
					ek.EXPECT().GetPubKeyByConsAddr(gomock.Any(), sdk.ConsAddress(validator.ValidatorAddress(t).Bytes())).Return(validator.ProtoPubkey(), nil).AnyTimes()

					if i < invalidValidBlsSig {
						ek.EXPECT().VerifyBLSSig(gomock.Any(), validatorAndExtensions.Extensions[i].ToBLSSig()).Return(checkpointingtypes.ErrInvalidBlsSignature).AnyTimes()
					} else {
						ek.EXPECT().VerifyBLSSig(gomock.Any(), validatorAndExtensions.Extensions[i].ToBLSSig()).Return(nil).AnyTimes()
					}
					ek.EXPECT().GetBlsPubKey(gomock.Any(), validator.ValidatorAddress(t)).Return(validator.BlsPubKey(), nil).AnyTimes()
					marshaledExtension, err := validatorAndExtensions.Extensions[i].Marshal()
					require.NoError(t, err)
					signedExtension := validator.SignVoteExtension(t, marshaledExtension, ec.Ctx.HeaderInfo().Height-1, ec.Ctx.ChainID())
					signedVoteExtensions = append(signedVoteExtensions, signedExtension)
				}

				return &Scenario{
					TotalPower:   totalPower,
					ValidatorSet: validatorAndExtensions.Vals,
					Extensions:   signedVoteExtensions,
				}
			},
			expectError: true,
		},
		{
			name: "less than 1/3 of validators provided invalid bls signature",
			scenarioSetup: func(ec *EpochAndCtx, ek *mocks.MockCheckpointingKeeper) *Scenario {
				bh := randomBlockHash()
				// each validator has the same voting power
				numValidators := 9
				invalidBlsSig := numValidators/3 - 1

				validatorAndExtensions, totalPower := generateNValidatorAndVoteExtensions(t, numValidators, &bh, ec.Epoch.EpochNumber)

				var signedVoteExtensions []cbftt.ExtendedVoteInfo
				for i, val := range validatorAndExtensions.Vals {
					validator := val
					ek.EXPECT().GetPubKeyByConsAddr(gomock.Any(), sdk.ConsAddress(validator.ValidatorAddress(t).Bytes())).Return(validator.ProtoPubkey(), nil).AnyTimes()

					if i < invalidBlsSig {
						ek.EXPECT().VerifyBLSSig(gomock.Any(), validatorAndExtensions.Extensions[i].ToBLSSig()).Return(checkpointingtypes.ErrInvalidBlsSignature).AnyTimes()
					} else {
						ek.EXPECT().VerifyBLSSig(gomock.Any(), validatorAndExtensions.Extensions[i].ToBLSSig()).Return(nil).AnyTimes()
					}
					ek.EXPECT().GetBlsPubKey(gomock.Any(), validator.ValidatorAddress(t)).Return(validator.BlsPubKey(), nil).AnyTimes()
					marshaledExtension, err := validatorAndExtensions.Extensions[i].Marshal()
					require.NoError(t, err)
					signedExtension := validator.SignVoteExtension(t, marshaledExtension, ec.Ctx.HeaderInfo().Height-1, ec.Ctx.ChainID())
					signedVoteExtensions = append(signedVoteExtensions, signedExtension)
				}

				return &Scenario{
					TotalPower:   totalPower,
					ValidatorSet: validatorAndExtensions.Vals,
					Extensions:   signedVoteExtensions,
				}
			},
			expectError: false,
		},
		{
			name: "2/3 + 1 of validators voted for valid block hash, the rest voted for invalid block hash",
			scenarioSetup: func(ec *EpochAndCtx, ek *mocks.MockCheckpointingKeeper) *Scenario {
				bh := randomBlockHash()
				bh1 := randomBlockHash()

				validatorAndExtensionsValid, totalPowerValid := generateNValidatorAndVoteExtensions(t, 7, &bh, ec.Epoch.EpochNumber)
				validatorAndExtensionsInvalid, totalPowerInvalid := generateNValidatorAndVoteExtensions(t, 2, &bh1, ec.Epoch.EpochNumber)

				var allvalidators []TestValidator
				allvalidators = append(allvalidators, validatorAndExtensionsValid.Vals...)
				allvalidators = append(allvalidators, validatorAndExtensionsInvalid.Vals...)

				var allExtensions []checkpointingtypes.VoteExtension
				allExtensions = append(allExtensions, validatorAndExtensionsValid.Extensions...)
				allExtensions = append(allExtensions, validatorAndExtensionsInvalid.Extensions...)

				var signedVoteExtensions []cbftt.ExtendedVoteInfo
				for i, val := range allvalidators {
					validator := val
					ek.EXPECT().GetPubKeyByConsAddr(gomock.Any(), sdk.ConsAddress(validator.ValidatorAddress(t).Bytes())).Return(validator.ProtoPubkey(), nil).AnyTimes()
					ek.EXPECT().VerifyBLSSig(gomock.Any(), allExtensions[i].ToBLSSig()).Return(nil).AnyTimes()
					ek.EXPECT().GetBlsPubKey(gomock.Any(), validator.ValidatorAddress(t)).Return(validator.BlsPubKey(), nil).AnyTimes()
					marshaledExtension, err := allExtensions[i].Marshal()
					require.NoError(t, err)
					signedExtension := validator.SignVoteExtension(t, marshaledExtension, ec.Ctx.HeaderInfo().Height-1, ec.Ctx.ChainID())
					signedVoteExtensions = append(signedVoteExtensions, signedExtension)
				}

				return &Scenario{
					TotalPower:   totalPowerValid + totalPowerInvalid,
					ValidatorSet: allvalidators,
					Extensions:   signedVoteExtensions,
				}
			},
			expectError: false,
		},
		{
			name: "All valid vote extensions",
			scenarioSetup: func(ec *EpochAndCtx, ek *mocks.MockCheckpointingKeeper) *Scenario {
				bh := randomBlockHash()
				validatorAndExtensions, totalPower := generateNValidatorAndVoteExtensions(t, 4, &bh, ec.Epoch.EpochNumber)

				var signedVoteExtensions []cbftt.ExtendedVoteInfo
				for i, val := range validatorAndExtensions.Vals {
					validator := val
					ek.EXPECT().GetPubKeyByConsAddr(gomock.Any(), sdk.ConsAddress(validator.ValidatorAddress(t).Bytes())).Return(validator.ProtoPubkey(), nil).AnyTimes()
					ek.EXPECT().VerifyBLSSig(gomock.Any(), validatorAndExtensions.Extensions[i].ToBLSSig()).Return(nil).AnyTimes()
					ek.EXPECT().GetBlsPubKey(gomock.Any(), validator.ValidatorAddress(t)).Return(validator.BlsPubKey(), nil).AnyTimes()
					marshaledExtension, err := validatorAndExtensions.Extensions[i].Marshal()
					require.NoError(t, err)
					signedExtension := validator.SignVoteExtension(t, marshaledExtension, ec.Ctx.HeaderInfo().Height-1, ec.Ctx.ChainID())
					signedVoteExtensions = append(signedVoteExtensions, signedExtension)
				}

				return &Scenario{
					TotalPower:   totalPower,
					ValidatorSet: validatorAndExtensions.Vals,
					Extensions:   signedVoteExtensions,
				}
			},
			expectError: false,
		},
		{
			name: "All valid vote extensions and other transactions in block proposal",
			scenarioSetup: func(ec *EpochAndCtx, ek *mocks.MockCheckpointingKeeper) *Scenario {
				bh := randomBlockHash()
				validatorAndExtensions, totalPower := generateNValidatorAndVoteExtensions(t, 4, &bh, ec.Epoch.EpochNumber)

				var signedVoteExtensions []cbftt.ExtendedVoteInfo
				for i, val := range validatorAndExtensions.Vals {
					validator := val
					ek.EXPECT().GetPubKeyByConsAddr(gomock.Any(), sdk.ConsAddress(validator.ValidatorAddress(t).Bytes())).Return(validator.ProtoPubkey(), nil).AnyTimes()
					ek.EXPECT().VerifyBLSSig(gomock.Any(), validatorAndExtensions.Extensions[i].ToBLSSig()).Return(nil).AnyTimes()
					ek.EXPECT().GetBlsPubKey(gomock.Any(), validator.ValidatorAddress(t)).Return(validator.BlsPubKey(), nil).AnyTimes()
					marshaledExtension, err := validatorAndExtensions.Extensions[i].Marshal()
					require.NoError(t, err)
					signedExtension := validator.SignVoteExtension(t, marshaledExtension, ec.Ctx.HeaderInfo().Height-1, ec.Ctx.ChainID())
					signedVoteExtensions = append(signedVoteExtensions, signedExtension)
				}

				return &Scenario{
					TotalPower:   totalPower,
					ValidatorSet: validatorAndExtensions.Vals,
					Extensions:   signedVoteExtensions,
					Txs:          genRandomTxs(t, encCfg.TxConfig, regularTxCount, livenessTxCount),
					TxVerifier:   newTxVerifier(encCfg.TxConfig), // we need to use this dummy tx verifier, otherwise the baseApp is used and runs the transactions in 'execModePrepareProposal' and execModeProcessProposal'
				}
			},
			expectError: false,
		},
		// TODO: Add scenarios testing compatibility of prepareProposal, processProposal and preBlocker
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := gomock.NewController(t)
			ek := mocks.NewMockCheckpointingKeeper(c)
			mCfg := mempool.DefaultPriorityNonceMempoolConfig()
			mCfg.MaxTx = 0
			mem := mempool.NewPriorityMempool(mCfg)
			ec := epochAndVoteExtensionCtx()
			scenario := tt.scenarioSetup(ec, ek)
			// Those are true for every scenario
			ek.EXPECT().GetEpoch(gomock.Any()).Return(ec.Epoch).AnyTimes()
			ek.EXPECT().GetTotalVotingPower(gomock.Any(), ec.Epoch.EpochNumber).Return(scenario.TotalPower).AnyTimes()
			ek.EXPECT().GetValidatorSet(gomock.Any(), ec.Epoch.EpochNumber).Return(et.NewSortedValidatorSet(ToValidatorSet(scenario.ValidatorSet))).AnyTimes()

			// if there're txs in the scenario, add them to the mempool
			addTxsToMempool(scenario.Txs, mem)

			logger := log.NewTestLogger(t)
			db := dbm.NewMemDB()
			name := t.Name()
			bApp := baseapp.NewBaseApp(name, logger, db, encCfg.TxConfig.TxDecoder(), baseapp.SetChainID("chain-test"))
			h := prepare.NewProposalHandler(
				log.NewNopLogger(),
				ek,
				mem,
				bApp,
				encCfg,
			)

			if scenario.TxVerifier != nil {
				h = h.WithTxVerifier(scenario.TxVerifier)
			}

			commitInfo, _, cometInfo := helper.ExtendedCommitToLastCommit(cbftt.ExtendedCommitInfo{Round: 0, Votes: scenario.Extensions})
			scenario.Extensions = commitInfo.Votes
			ec.Ctx = ec.Ctx.WithCometInfo(cometInfo)

			req := requestPrepareProposal(ec.Ctx.HeaderInfo().Height, commitInfo)
			prop, err := h.PrepareProposal()(ec.Ctx, req)
			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				expTxCount := len(scenario.Txs) + 1 // Expecting to have all the txs in the mempool + the injected checkpoint tx
				require.Len(t, prop.Txs, expTxCount)
				checkpoint, err := h.ExtractInjectedCheckpoint(prop.Txs)
				require.NoError(t, err)
				err = verifyCheckpoint(scenario.ValidatorSet, checkpoint.Ckpt.Ckpt)
				require.NoError(t, err)
				verifyTxOrder(t, prop.Txs, encCfg.TxConfig.TxDecoder(), regularTxCount, livenessTxCount)
			}
		})
	}
}
