package integration_test

import (
	"math/rand"
	"testing"

	"cosmossdk.io/math"
	"github.com/stretchr/testify/require"

	abci "github.com/cometbft/cometbft/abci/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdksigning "github.com/cosmos/cosmos-sdk/types/tx/signing"
	cmtcrypto "github.com/cometbft/cometbft/proto/tendermint/crypto"
	authsigning "github.com/cosmos/cosmos-sdk/x/auth/signing"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	"github.com/cosmos/cosmos-sdk/client"

	"github.com/babylonlabs-io/babylon/v4/app"
	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	testhelper "github.com/babylonlabs-io/babylon/v4/testutil/helper"
	bbn "github.com/babylonlabs-io/babylon/v4/types"
	btclightclienttypes "github.com/babylonlabs-io/babylon/v4/x/btclightclient/types"
	finalitytypes "github.com/babylonlabs-io/babylon/v4/x/finality/types"
)

// TestValidateBasic_AppIntegration tests ValidateBasic at baseapp level via CheckTx
// This verifies that ValidateBasic is properly called during baseapp's transaction processing
// Tests focus on message-specific validation rules rather than generic errors
func TestValidateBasic_AppIntegration(t *testing.T) {
	helper := testhelper.NewHelper(t)
	app := helper.App

	t.Run("MsgInsertHeaders_CheckTx_ValidateBasic_Flow", func(t *testing.T) {
		t.Run("ValidTransaction_ShouldPassValidateBasic", func(t *testing.T) {
			r := rand.New(rand.NewSource(1))

			// Get existing tip to build on it
			initTip := app.BTCLightClientKeeper.GetTipInfo(helper.Ctx)
			require.NotNil(t, initTip)

			// Generate valid chain extension
			chainExtension := datagen.GenRandomValidChainStartingFrom(
				r,
				initTip.Header.ToBlockHeader(),
				nil,
				1, // Just 1 header
			)

			// Create a proper secp256k1 account key for signing
			accountPrivKey := secp256k1.GenPrivKey()
			signerAddr := sdk.AccAddress(accountPrivKey.PubKey().Address())

			// Create basic transaction
			txBuilder := app.TxConfig().NewTxBuilder()
			msg := &btclightclienttypes.MsgInsertHeaders{
				Signer:  signerAddr.String(),
				Headers: []bbn.BTCHeaderBytes{bbn.NewBTCHeaderBytesFromBlockHeader(chainExtension[0])},
			}

			err := txBuilder.SetMsgs(msg)
			require.NoError(t, err)

			txBuilder.SetGasLimit(300000)
			fee := sdk.NewCoins(sdk.NewCoin("ubbn", math.NewInt(1000)))
			txBuilder.SetFeeAmount(fee)

			// Sign the transaction once
			txBytes := signTransactionWithSecp256k1(t, helper, app, txBuilder, accountPrivKey)

			// First verify ValidateBasic passes using SDK logic
			err = testhelper.ValidateBasicTxMsgs([]sdk.Msg{msg})
			require.NoError(t, err, "Valid message should pass ValidateBasic")

			// Use CheckTx which calls validateBasicTxMsgs internally with a fully signed transaction
			resp, _ := app.BaseApp.CheckTx(&abci.RequestCheckTx{Tx: txBytes})

			// Code 9 = "unknown address" means ValidateBasic passed but account doesn't exist
			// This proves ValidateBasic was called and succeeded during CheckTx
			if resp.Code == 9 && resp.Log != "" && resp.Log != "0" {
				t.Logf("CheckTx passed ValidateBasic and reached account validation (code %d: %s)", resp.Code, resp.Log)
				require.Contains(t, resp.Log, "does not exist", "Should reach account validation, proving ValidateBasic passed")
			} else {
				t.Logf("CheckTx response - code %d: %s, info: %s", resp.Code, resp.Log, resp.Info)
				require.Equal(t, abci.CodeTypeOK, resp.Code, "Transaction should pass ValidateBasic and succeed completely")
			}

			t.Log("Valid signed MsgInsertHeaders passed baseapp CheckTx (ValidateBasic) completely")
		})

		t.Run("InvalidTransaction_EmptyHeaders_ValidateBasic_Fails", func(t *testing.T) {
			signer := helper.GenAccs[0]
			signerAddr := signer.GetAddress()

			// Create message with empty headers list (specific ValidateBasic check)
			msg := &btclightclienttypes.MsgInsertHeaders{
				Signer:  signerAddr.String(),
				Headers: []bbn.BTCHeaderBytes{}, // Empty headers - specific ValidateBasic validation
			}

			// First verify ValidateBasic fails using SDK logic
			err := testhelper.ValidateBasicTxMsgs([]sdk.Msg{msg})
			require.Error(t, err, "Invalid message should fail ValidateBasic")
			require.Contains(t, err.Error(), "empty headers list")

			// Create basic transaction
			txBuilder := app.TxConfig().NewTxBuilder()
			err = txBuilder.SetMsgs(msg)
			require.NoError(t, err)

			txBuilder.SetGasLimit(300000)
			fee := sdk.NewCoins(sdk.NewCoin("ubbn", math.NewInt(1000)))
			txBuilder.SetFeeAmount(fee)

			tx := txBuilder.GetTx()
			txBytes, err := app.TxConfig().TxEncoder()(tx)
			require.NoError(t, err)

			// Use CheckTx which should fail at ValidateBasic level
			resp, _ := app.BaseApp.CheckTx(&abci.RequestCheckTx{Tx: txBytes})
			require.NotEqual(t, abci.CodeTypeOK, resp.Code, "Invalid transaction should fail CheckTx at ValidateBasic level")
			require.Contains(t, resp.Log, "empty headers list")

			t.Log("Invalid MsgInsertHeaders (empty headers) correctly rejected at baseapp CheckTx ValidateBasic level")
		})
	})

	t.Run("MsgCommitPubRandList_CheckTx_ValidateBasic_Flow", func(t *testing.T) {
		t.Run("ValidTransaction_ShouldPassValidateBasic", func(t *testing.T) {
			r := rand.New(rand.NewSource(3))
			fpBTCPK, err := datagen.GenRandomBIP340PubKey(r)
			require.NoError(t, err)

			// Create a valid BIP340 signature (64 bytes hex)
			sig, err := bbn.NewBIP340SignatureFromHex("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
			require.NoError(t, err)

			// Create a proper secp256k1 account key for signing
			accountPrivKey := secp256k1.GenPrivKey()
			signerAddr := sdk.AccAddress(accountPrivKey.PubKey().Address())

			// Create basic transaction
			txBuilder := app.TxConfig().NewTxBuilder()
			msg := &finalitytypes.MsgCommitPubRandList{
				Signer:      signerAddr.String(),
				FpBtcPk:     fpBTCPK,
				StartHeight: 1,
				NumPubRand:  100,
				Commitment:  datagen.GenRandomByteArray(r, 32), // Exactly 32 bytes - ValidateBasic requirement
				Sig:         sig,
			}

			err = txBuilder.SetMsgs(msg)
			require.NoError(t, err)

			txBuilder.SetGasLimit(300000)
			fee := sdk.NewCoins(sdk.NewCoin("ubbn", math.NewInt(1000)))
			txBuilder.SetFeeAmount(fee)

			// Sign the transaction once
			txBytes := signTransactionWithSecp256k1(t, helper, app, txBuilder, accountPrivKey)

			// First verify ValidateBasic passes using SDK logic
			err = testhelper.ValidateBasicTxMsgs([]sdk.Msg{msg})
			require.NoError(t, err, "Valid message should pass ValidateBasic")

			// Use CheckTx which calls validateBasicTxMsgs internally with a fully signed transaction
			resp, _ := app.BaseApp.CheckTx(&abci.RequestCheckTx{Tx: txBytes})

			// Code 9 = "unknown address" means ValidateBasic passed but account doesn't exist
			// This proves ValidateBasic was called and succeeded during CheckTx
			if resp.Code == 9 && resp.Log != "" && resp.Log != "0" {
				t.Logf("CheckTx passed ValidateBasic and reached account validation (code %d: %s)", resp.Code, resp.Log)
				require.Contains(t, resp.Log, "does not exist", "Should reach account validation, proving ValidateBasic passed")
			} else {
				t.Logf("CheckTx response - code %d: %s, info: %s", resp.Code, resp.Log, resp.Info)
				require.Equal(t, abci.CodeTypeOK, resp.Code, "Transaction should pass ValidateBasic and succeed completely")
			}

			t.Log("Valid signed MsgCommitPubRandList passed baseapp CheckTx (ValidateBasic) completely")
		})

		t.Run("InvalidTransaction_InvalidCommitmentSize_ValidateBasic_Fails", func(t *testing.T) {
			signer := helper.GenAccs[0]
			signerAddr := signer.GetAddress()

			r := rand.New(rand.NewSource(5))
			fpBTCPK, err := datagen.GenRandomBIP340PubKey(r)
			require.NoError(t, err)

			sig, err := bbn.NewBIP340SignatureFromHex("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
			require.NoError(t, err)

			msg := &finalitytypes.MsgCommitPubRandList{
				Signer:      signerAddr.String(),
				FpBtcPk:     fpBTCPK,
				StartHeight: 1,
				NumPubRand:  100,
				Commitment:  []byte{0x01, 0x02}, // Wrong size - should be exactly 32 bytes (ValidateBasic check)
				Sig:         sig,
			}

			// First verify ValidateBasic fails using SDK logic
			err = testhelper.ValidateBasicTxMsgs([]sdk.Msg{msg})
			require.Error(t, err, "Invalid message should fail ValidateBasic")
			require.Contains(t, err.Error(), "commitment must be 32 bytes")

			// Create basic transaction
			txBuilder := app.TxConfig().NewTxBuilder()
			err = txBuilder.SetMsgs(msg)
			require.NoError(t, err)

			txBuilder.SetGasLimit(300000)
			fee := sdk.NewCoins(sdk.NewCoin("ubbn", math.NewInt(1000)))
			txBuilder.SetFeeAmount(fee)

			tx := txBuilder.GetTx()
			txBytes, err := app.TxConfig().TxEncoder()(tx)
			require.NoError(t, err)

			// Use CheckTx which should fail at ValidateBasic level
			resp, _ := app.BaseApp.CheckTx(&abci.RequestCheckTx{Tx: txBytes})
			require.NotEqual(t, abci.CodeTypeOK, resp.Code, "Invalid transaction should fail CheckTx at ValidateBasic level")
			require.Contains(t, resp.Log, "commitment must be 32 bytes")

			t.Log("Invalid MsgCommitPubRandList (invalid commitment size) correctly rejected at baseapp CheckTx ValidateBasic level")
		})

		t.Run("InvalidTransaction_HeightOverflow_ValidateBasic_Fails", func(t *testing.T) {
			signer := helper.GenAccs[0]
			signerAddr := signer.GetAddress()

			r := rand.New(rand.NewSource(6))
			fpBTCPK, err := datagen.GenRandomBIP340PubKey(r)
			require.NoError(t, err)

			sig, err := bbn.NewBIP340SignatureFromHex("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
			require.NoError(t, err)

			msg := &finalitytypes.MsgCommitPubRandList{
				Signer:      signerAddr.String(),
				FpBtcPk:     fpBTCPK,
				StartHeight: 18446744073709551615, // Max uint64 - will cause overflow in ValidateBasic
				NumPubRand:  1,                    // Any positive number will cause overflow
				Commitment:  datagen.GenRandomByteArray(r, 32),
				Sig:         sig,
			}

			// First verify ValidateBasic fails using SDK logic
			err = testhelper.ValidateBasicTxMsgs([]sdk.Msg{msg})
			require.Error(t, err, "Invalid message should fail ValidateBasic")
			require.Contains(t, err.Error(), "overflow")

			// Create basic transaction
			txBuilder := app.TxConfig().NewTxBuilder()
			err = txBuilder.SetMsgs(msg)
			require.NoError(t, err)

			txBuilder.SetGasLimit(300000)
			fee := sdk.NewCoins(sdk.NewCoin("ubbn", math.NewInt(1000)))
			txBuilder.SetFeeAmount(fee)

			tx := txBuilder.GetTx()
			txBytes, err := app.TxConfig().TxEncoder()(tx)
			require.NoError(t, err)

			// Use CheckTx which should fail at ValidateBasic level
			resp, _ := app.BaseApp.CheckTx(&abci.RequestCheckTx{Tx: txBytes})
			require.NotEqual(t, abci.CodeTypeOK, resp.Code, "Invalid transaction should fail CheckTx at ValidateBasic level")
			require.Contains(t, resp.Log, "overflow")

			t.Log("Invalid MsgCommitPubRandList (height overflow) correctly rejected at baseapp CheckTx ValidateBasic level")
		})
	})

	t.Run("MsgAddFinalitySig_CheckTx_ValidateBasic_Flow", func(t *testing.T) {
		t.Run("ValidTransaction_ShouldPassValidateBasic", func(t *testing.T) {
			r := rand.New(rand.NewSource(7))

			fpBTCPK, err := datagen.GenRandomBIP340PubKey(r)
			require.NoError(t, err)

			// Create valid 32-byte Schnorr pubrand (ValidateBasic requirement)
			pubRand, err := bbn.NewSchnorrPubRandFromHex("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
			require.NoError(t, err)

			finalitySig, err := datagen.GenRandomFinalitySig(r)
			require.NoError(t, err)

			proof := &cmtcrypto.Proof{
				Total:    1,
				Index:    0,
				LeafHash: datagen.GenRandomByteArray(r, 32),
				Aunts:    [][]byte{},
			}

			// Create a proper secp256k1 account key for signing
			accountPrivKey := secp256k1.GenPrivKey()
			signerAddr := sdk.AccAddress(accountPrivKey.PubKey().Address())

			// Create basic transaction
			txBuilder := app.TxConfig().NewTxBuilder()
			msg := &finalitytypes.MsgAddFinalitySig{
				Signer:       signerAddr.String(),
				FpBtcPk:      fpBTCPK,
				BlockHeight:  100,
				PubRand:      pubRand,
				Proof:        proof,
				BlockAppHash: datagen.GenRandomByteArray(r, 32), // Exactly 32 bytes - ValidateBasic requirement
				FinalitySig:  finalitySig,
			}

			err = txBuilder.SetMsgs(msg)
			require.NoError(t, err)

			txBuilder.SetGasLimit(300000)
			fee := sdk.NewCoins(sdk.NewCoin("ubbn", math.NewInt(1000)))
			txBuilder.SetFeeAmount(fee)

			// Sign the transaction once
			txBytes := signTransactionWithSecp256k1(t, helper, app, txBuilder, accountPrivKey)

			// First verify ValidateBasic passes using SDK logic
			err = testhelper.ValidateBasicTxMsgs([]sdk.Msg{msg})
			require.NoError(t, err, "Valid message should pass ValidateBasic")

			// Use CheckTx which calls validateBasicTxMsgs internally with a fully signed transaction
			resp, _ := app.BaseApp.CheckTx(&abci.RequestCheckTx{Tx: txBytes})

			// Code 9 = "unknown address" means ValidateBasic passed but account doesn't exist
			// This proves ValidateBasic was called and succeeded during CheckTx
			if resp.Code == 9 && resp.Log != "" && resp.Log != "0" {
				t.Logf("CheckTx passed ValidateBasic and reached account validation (code %d: %s)", resp.Code, resp.Log)
				require.Contains(t, resp.Log, "does not exist", "Should reach account validation, proving ValidateBasic passed")
			} else {
				t.Logf("CheckTx response - code %d: %s, info: %s", resp.Code, resp.Log, resp.Info)
				require.Equal(t, abci.CodeTypeOK, resp.Code, "Transaction should pass ValidateBasic and succeed completely")
			}

			t.Log("Valid signed MsgAddFinalitySig passed baseapp CheckTx (ValidateBasic) completely")
		})

		t.Run("InvalidTransaction_InvalidAppHashSize_ValidateBasic_Fails", func(t *testing.T) {
			signer := helper.GenAccs[0]
			signerAddr := signer.GetAddress()

			r := rand.New(rand.NewSource(8))

			fpBTCPK, err := datagen.GenRandomBIP340PubKey(r)
			require.NoError(t, err)

			// Create valid 32-byte Schnorr pubrand
			pubRand, err := bbn.NewSchnorrPubRandFromHex("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
			require.NoError(t, err)

			finalitySig, err := datagen.GenRandomFinalitySig(r)
			require.NoError(t, err)

			proof := &cmtcrypto.Proof{
				Total:    1,
				Index:    0,
				LeafHash: datagen.GenRandomByteArray(r, 32),
				Aunts:    [][]byte{},
			}

			msg := &finalitytypes.MsgAddFinalitySig{
				Signer:       signerAddr.String(),
				FpBtcPk:      fpBTCPK,
				BlockHeight:  100,
				PubRand:      pubRand,
				Proof:        proof,
				BlockAppHash: []byte{0x01, 0x02}, // Wrong size - should be exactly 32 bytes (ValidateBasic check)
				FinalitySig:  finalitySig,
			}

			// First verify ValidateBasic fails using SDK logic
			err = testhelper.ValidateBasicTxMsgs([]sdk.Msg{msg})
			require.Error(t, err, "Invalid message should fail ValidateBasic")
			require.Contains(t, err.Error(), "invalid block app hash length")

			// Create basic transaction
			txBuilder := app.TxConfig().NewTxBuilder()
			err = txBuilder.SetMsgs(msg)
			require.NoError(t, err)

			txBuilder.SetGasLimit(300000)
			fee := sdk.NewCoins(sdk.NewCoin("ubbn", math.NewInt(1000)))
			txBuilder.SetFeeAmount(fee)

			tx := txBuilder.GetTx()
			txBytes, err := app.TxConfig().TxEncoder()(tx)
			require.NoError(t, err)

			// Use CheckTx which should fail at ValidateBasic level
			resp, _ := app.BaseApp.CheckTx(&abci.RequestCheckTx{Tx: txBytes})
			require.NotEqual(t, abci.CodeTypeOK, resp.Code, "Invalid transaction should fail CheckTx at ValidateBasic level")
			require.Contains(t, resp.Log, "invalid block app hash length")

			t.Log("Invalid MsgAddFinalitySig (invalid app hash size) correctly rejected at baseapp CheckTx ValidateBasic level")
		})

		t.Run("InvalidTransaction_NilPubRand_ValidateBasic_Fails", func(t *testing.T) {
			signer := helper.GenAccs[0]
			signerAddr := signer.GetAddress()

			r := rand.New(rand.NewSource(9))

			fpBTCPK, err := datagen.GenRandomBIP340PubKey(r)
			require.NoError(t, err)

			finalitySig, err := datagen.GenRandomFinalitySig(r)
			require.NoError(t, err)

			proof := &cmtcrypto.Proof{
				Total:    1,
				Index:    0,
				LeafHash: datagen.GenRandomByteArray(r, 32),
				Aunts:    [][]byte{},
			}

			msg := &finalitytypes.MsgAddFinalitySig{
				Signer:       signerAddr.String(),
				FpBtcPk:      fpBTCPK,
				BlockHeight:  100,
				PubRand:      nil, // Nil PubRand - ValidateBasic check
				Proof:        proof,
				BlockAppHash: datagen.GenRandomByteArray(r, 32),
				FinalitySig:  finalitySig,
			}

			// First verify ValidateBasic fails using SDK logic
			err = testhelper.ValidateBasicTxMsgs([]sdk.Msg{msg})
			require.Error(t, err, "Invalid message should fail ValidateBasic")
			require.Contains(t, err.Error(), "empty Public Randomness")

			// Create basic transaction
			txBuilder := app.TxConfig().NewTxBuilder()
			err = txBuilder.SetMsgs(msg)
			require.NoError(t, err)

			txBuilder.SetGasLimit(300000)
			fee := sdk.NewCoins(sdk.NewCoin("ubbn", math.NewInt(1000)))
			txBuilder.SetFeeAmount(fee)

			tx := txBuilder.GetTx()
			txBytes, err := app.TxConfig().TxEncoder()(tx)
			require.NoError(t, err)

			// Use CheckTx which should fail at ValidateBasic level
			resp, _ := app.BaseApp.CheckTx(&abci.RequestCheckTx{Tx: txBytes})
			require.NotEqual(t, abci.CodeTypeOK, resp.Code, "Invalid transaction should fail CheckTx at ValidateBasic level")
			require.Contains(t, resp.Log, "empty Public Randomness")

			t.Log("Invalid MsgAddFinalitySig (nil pubrand) correctly rejected at baseapp CheckTx ValidateBasic level")
		})
	})

	t.Run("TransactionLevelValidation_CheckTx_Flow", func(t *testing.T) {
		t.Log("=== Testing Transaction-Level ValidateBasic Flow via CheckTx ===")

		signer := helper.GenAccs[0]
		signerAddr := signer.GetAddress()

		r := rand.New(rand.NewSource(10))

		// Get existing tip for valid header
		initTip := app.BTCLightClientKeeper.GetTipInfo(helper.Ctx)
		require.NotNil(t, initTip)
		chainExtension := datagen.GenRandomValidChainStartingFrom(
			r,
			initTip.Header.ToBlockHeader(),
			nil,
			1,
		)

		// Create valid message
		validMsg := &btclightclienttypes.MsgInsertHeaders{
			Signer:  signerAddr.String(),
			Headers: []bbn.BTCHeaderBytes{bbn.NewBTCHeaderBytesFromBlockHeader(chainExtension[0])},
		}

		// Create invalid message with empty headers (specific ValidateBasic failure)
		invalidMsg := &btclightclienttypes.MsgInsertHeaders{
			Signer:  signerAddr.String(),
			Headers: []bbn.BTCHeaderBytes{}, // Empty headers - ValidateBasic check
		}

		// Test mixed valid/invalid messages in single transaction
		msgs := []sdk.Msg{validMsg, invalidMsg}

		// First verify ValidateBasic fails using SDK logic
		err := testhelper.ValidateBasicTxMsgs(msgs)
		require.Error(t, err, "Transaction with invalid message should fail ValidateBasic")
		require.Contains(t, err.Error(), "empty headers list")

		// Create transaction with mixed messages
		txBuilder := app.TxConfig().NewTxBuilder()
		err = txBuilder.SetMsgs(msgs...)
		require.NoError(t, err)

		txBuilder.SetGasLimit(300000)
		fee := sdk.NewCoins(sdk.NewCoin("ubbn", math.NewInt(1000)))
		txBuilder.SetFeeAmount(fee)

		tx := txBuilder.GetTx()
		txBytes, err := app.TxConfig().TxEncoder()(tx)
		require.NoError(t, err)

		// Use CheckTx which should fail at ValidateBasic level due to invalid message
		resp, _ := app.BaseApp.CheckTx(&abci.RequestCheckTx{Tx: txBytes})
		require.NotEqual(t, abci.CodeTypeOK, resp.Code, "Transaction with invalid message should fail CheckTx at ValidateBasic level")
		require.Contains(t, resp.Log, "empty headers list")

		t.Log("Transaction with mixed valid/invalid messages correctly rejected at baseapp CheckTx ValidateBasic level")
	})

	t.Log("All baseapp CheckTx integration tests passed - ValidateBasic properly called during baseapp transaction processing")
}

// signTransactionWithSecp256k1 signs a transaction using a provided secp256k1 account key
// Returns the signed transaction bytes and the signer address
func signTransactionWithSecp256k1(t *testing.T, helper *testhelper.Helper, app *app.BabylonApp, txBuilder client.TxBuilder, accountPrivKey *secp256k1.PrivKey) []byte {
	signerAddr := sdk.AccAddress(accountPrivKey.PubKey().Address())
	pubKey := accountPrivKey.PubKey()

	// Set up signing info
	sigV2 := sdksigning.SignatureV2{
		PubKey: pubKey,
		Data: &sdksigning.SingleSignatureData{
			SignMode:  sdksigning.SignMode_SIGN_MODE_DIRECT,
			Signature: nil,
		},
		Sequence: 0,
	}

	err := txBuilder.SetSignatures(sigV2)
	require.NoError(t, err)

	// Generate signature bytes for signing
	signerData := authsigning.SignerData{
		Address:       signerAddr.String(),
		ChainID:       helper.Ctx.ChainID(),
		AccountNumber: 0,
		Sequence:      0,
		PubKey:        pubKey,
	}

	bytesToSign, err := authsigning.GetSignBytesAdapter(
		helper.Ctx,
		app.TxConfig().SignModeHandler(),
		sdksigning.SignMode_SIGN_MODE_DIRECT,
		signerData,
		txBuilder.GetTx(),
	)
	require.NoError(t, err)

	// Sign the bytes
	signature, err := accountPrivKey.Sign(bytesToSign)
	require.NoError(t, err)

	// Set the actual signature
	sigV2.Data.(*sdksigning.SingleSignatureData).Signature = signature
	err = txBuilder.SetSignatures(sigV2)
	require.NoError(t, err)

	// Get the final signed transaction
	signedTx := txBuilder.GetTx()
	txBytes, err := app.TxConfig().TxEncoder()(signedTx)
	require.NoError(t, err)

	return txBytes
}
