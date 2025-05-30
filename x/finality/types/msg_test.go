package types_test

import (
	"math/rand"
	"testing"

	"github.com/babylonlabs-io/babylon/v3/crypto/eots"
	"github.com/babylonlabs-io/babylon/v3/testutil/datagen"
	bbntypes "github.com/babylonlabs-io/babylon/v3/types"
	"github.com/babylonlabs-io/babylon/v3/x/finality/types"
	"github.com/stretchr/testify/require"
)

func FuzzMsgAddFinalitySig(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		sk, err := eots.KeyGen(r)
		require.NoError(t, err)

		numPubRand := uint64(100)
		randListInfo, err := datagen.GenRandomPubRandList(r, numPubRand)
		require.NoError(t, err)

		startHeight := datagen.RandomInt(r, 10)
		blockHeight := startHeight + datagen.RandomInt(r, 10)
		blockHash := datagen.GenRandomByteArray(r, 32)

		signer := datagen.GenRandomAccount().Address
		msg, err := datagen.NewMsgAddFinalitySig(signer, sk, startHeight, blockHeight, randListInfo, blockHash)
		require.NoError(t, err)

		prCommit := &types.PubRandCommit{
			StartHeight: startHeight,
			NumPubRand:  numPubRand,
			Commitment:  randListInfo.Commitment,
		}

		// verify the finality signature message
		err = types.VerifyFinalitySig(msg, prCommit)
		require.NoError(t, err)
	})
}

func FuzzMsgCommitPubRandList(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		sk, _, err := datagen.GenRandomBTCKeyPair(r)
		require.NoError(t, err)

		startHeight := datagen.RandomInt(r, 10)
		numPubRand := datagen.RandomInt(r, 100) + 1
		_, msg, err := datagen.GenRandomMsgCommitPubRandList(r, sk, startHeight, numPubRand)
		require.NoError(t, err)

		// sanity checks, including verifying signature
		err = msg.VerifySig()
		require.NoError(t, err)
	})
}

func TestMsgCommitPubRandListValidateBasic(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	sk, _, err := datagen.GenRandomBTCKeyPair(r)
	require.NoError(t, err)

	tests := []struct {
		name        string
		msgModifier func(*types.MsgCommitPubRandList)
		expectErr   bool
		errString   string
	}{
		{
			name:        "valid message",
			msgModifier: func(msg *types.MsgCommitPubRandList) {},
			expectErr:   false,
		},
		{
			name: "invalid signer",
			msgModifier: func(msg *types.MsgCommitPubRandList) {
				msg.Signer = "invalid-address"
			},
			expectErr: true,
			errString: "invalid signer address",
		},
		{
			name: "invalid commitment size",
			msgModifier: func(msg *types.MsgCommitPubRandList) {
				msg.Commitment = []byte("too-short")
			},
			expectErr: true,
			errString: "commitment must be 32 bytes",
		},
		{
			name: "overflow in block height",
			msgModifier: func(msg *types.MsgCommitPubRandList) {
				msg.NumPubRand = 0
			},
			expectErr: true,
			errString: "public rand commit start block height",
		},
		{
			name: "empty FP BTC PubKey",
			msgModifier: func(msg *types.MsgCommitPubRandList) {
				msg.FpBtcPk = nil
			},
			expectErr: true,
			errString: "empty FP BTC PubKey",
		},
		{
			name: "empty signature",
			msgModifier: func(msg *types.MsgCommitPubRandList) {
				msg.Sig = nil
			},
			expectErr: true,
			errString: "empty signature",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			startHeight := datagen.RandomInt(r, 10)
			numPubRand := datagen.RandomInt(r, 100) + 1
			_, msg, err := datagen.GenRandomMsgCommitPubRandList(r, sk, startHeight, numPubRand)
			require.NoError(t, err)

			tc.msgModifier(msg)

			err = msg.ValidateBasic()

			if tc.expectErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.errString)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestMsgAddFinalitySig_ValidateBasic(t *testing.T) {
	r := rand.New(rand.NewSource(10))

	sk, err := eots.KeyGen(r)
	require.NoError(t, err)

	numPubRand := uint64(100)
	randListInfo, err := datagen.GenRandomPubRandList(r, numPubRand)
	require.NoError(t, err)

	startHeight := datagen.RandomInt(r, 10)
	blockHeight := startHeight + datagen.RandomInt(r, 10)
	blockHash := datagen.GenRandomByteArray(r, 32)

	signer := datagen.GenRandomAccount().Address

	testCases := []struct {
		name        string
		msgModifier func(*types.MsgAddFinalitySig)
		expErr      string
	}{
		{
			name:        "valid message",
			msgModifier: func(*types.MsgAddFinalitySig) {},
		},
		{
			name: "invalid signer address",
			msgModifier: func(m *types.MsgAddFinalitySig) {
				m.Signer = "invalid-address"
			},
			expErr: "invalid signer address",
		},
		{
			name: "nil BTC public key",
			msgModifier: func(m *types.MsgAddFinalitySig) {
				m.FpBtcPk = nil
			},
			expErr: "empty Finality Provider BTC PubKey",
		},
		{
			name: "invalid BTC public key size",
			msgModifier: func(m *types.MsgAddFinalitySig) {
				k := bbntypes.BIP340PubKey([]byte{0x01})
				m.FpBtcPk = &k
			},
			expErr: "invalid finality provider BTC public key length",
		},
		{
			name: "nil PubRand",
			msgModifier: func(m *types.MsgAddFinalitySig) {
				m.PubRand = nil
			},
			expErr: "empty Public Randomness",
		},
		{
			name: "invalid PubRand length",
			msgModifier: func(m *types.MsgAddFinalitySig) {
				pr := bbntypes.SchnorrPubRand([]byte{0x02})
				m.PubRand = &pr
			},
			expErr: "invalind public randomness length",
		},
		{
			name: "nil proof",
			msgModifier: func(m *types.MsgAddFinalitySig) {
				m.Proof = nil
			},
			expErr: "empty inclusion proof",
		},
		{
			name: "nil finality sig",
			msgModifier: func(m *types.MsgAddFinalitySig) {
				m.FinalitySig = nil
			},
			expErr: "empty finality signature",
		},
		{
			name: "invalid finality sig length",
			msgModifier: func(m *types.MsgAddFinalitySig) {
				sig := bbntypes.SchnorrEOTSSig([]byte{0x03})
				m.FinalitySig = &sig
			},
			expErr: "invalid finality signature length",
		},
		{
			name: "invalid block app hash length",
			msgModifier: func(m *types.MsgAddFinalitySig) {
				m.BlockAppHash = []byte{0x01}
			},
			expErr: "invalid block app hash length",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			msg, err := datagen.NewMsgAddFinalitySig(signer, sk, startHeight, blockHeight, randListInfo, blockHash)
			require.NoError(t, err)
			tc.msgModifier(msg)
			err = msg.ValidateBasic()
			if tc.expErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.expErr)
		})
	}
}

func TestMsgUnjailFinalityProvider_ValidateBasic(t *testing.T) {
	validAddr := datagen.GenRandomAddress().String()
	validPk := bbntypes.BIP340PubKey(make([]byte, bbntypes.BIP340PubKeyLen))

	testCases := []struct {
		name   string
		msg    types.MsgUnjailFinalityProvider
		expErr string
	}{
		{
			name: "valid message",
			msg: types.MsgUnjailFinalityProvider{
				Signer:  validAddr,
				FpBtcPk: &validPk,
			},
			expErr: "",
		},
		{
			name: "invalid signer address",
			msg: types.MsgUnjailFinalityProvider{
				Signer:  "invalid-address",
				FpBtcPk: &validPk,
			},
			expErr: "invalid signer address",
		},
		{
			name: "nil BTC public key",
			msg: types.MsgUnjailFinalityProvider{
				Signer:  validAddr,
				FpBtcPk: nil,
			},
			expErr: "empty FP BTC PubKey",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.msg.ValidateBasic()
			if tc.expErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.expErr)
		})
	}
}
