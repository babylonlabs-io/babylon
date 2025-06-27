package types_test

import (
	"math/rand"
	"testing"
	"time"

	errorsmod "cosmossdk.io/errors"
	"github.com/babylonlabs-io/babylon/v3/app/signingcontext"
	"github.com/babylonlabs-io/babylon/v3/crypto/eots"
	"github.com/babylonlabs-io/babylon/v3/testutil/datagen"
	bbntypes "github.com/babylonlabs-io/babylon/v3/types"
	"github.com/babylonlabs-io/babylon/v3/x/finality/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/stretchr/testify/require"
)

const (
	testChainID = "test-1"
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

		randomModuleAddress := datagen.GenRandomAddress().String()

		voteContext := signingcontext.FpFinVoteContextV0(testChainID, randomModuleAddress)

		msg, err := datagen.NewMsgAddFinalitySig(signer, sk, voteContext, startHeight, blockHeight, randListInfo, blockHash)
		require.NoError(t, err)

		prCommit := &types.PubRandCommit{
			StartHeight: startHeight,
			NumPubRand:  numPubRand,
			Commitment:  randListInfo.Commitment,
		}

		// verify the finality signature message
		err = types.VerifyFinalitySig(msg, prCommit, voteContext)
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

		randomModuleAddress := datagen.GenRandomAddress().String()
		commitContext := signingcontext.FpRandCommitContextV0(testChainID, randomModuleAddress)

		_, msg, err := datagen.GenRandomMsgCommitPubRandList(r, sk, commitContext, startHeight, numPubRand)
		require.NoError(t, err)

		// sanity checks, including verifying signature
		err = msg.VerifySig(commitContext)
		require.NoError(t, err)
	})
}

func TestMsgCommitPubRandListValidateBasic(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	sk, _, err := datagen.GenRandomBTCKeyPair(r)
	require.NoError(t, err)

	randomModuleAddress := datagen.GenRandomAddress().String()
	commitContext := signingcontext.FpRandCommitContextV0(testChainID, randomModuleAddress)

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
			_, msg, err := datagen.GenRandomMsgCommitPubRandList(r, sk, commitContext, startHeight, numPubRand)
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

	randomModuleAddress := datagen.GenRandomAddress().String()
	voteContext := signingcontext.FpFinVoteContextV0(testChainID, randomModuleAddress)

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
			msg, err := datagen.NewMsgAddFinalitySig(signer, sk, voteContext, startHeight, blockHeight, randListInfo, blockHash)
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

func TestMsgEquivocationEvidence_ValidateBasic(t *testing.T) {
	var (
		validAddr        = datagen.GenRandomAddress().String()
		validPk          = bbntypes.BIP340PubKey(make([]byte, bbntypes.BIP340PubKeyLen))
		validPubRand     = bbntypes.SchnorrPubRand(make([]byte, bbntypes.SchnorrPubRandLen))
		validFinalitySig = bbntypes.SchnorrEOTSSig(make([]byte, bbntypes.SchnorrEOTSSigLen))
		validHash        = make([]byte, 32)
	)

	testCases := []struct {
		name   string
		msg    types.MsgEquivocationEvidence
		expErr string
	}{
		{
			name: "valid message",
			msg: types.MsgEquivocationEvidence{
				Signer:               validAddr,
				FpBtcPk:              &validPk,
				PubRand:              &validPubRand,
				CanonicalAppHash:     validHash,
				ForkAppHash:          validHash,
				CanonicalFinalitySig: &validFinalitySig,
				ForkFinalitySig:      &validFinalitySig,
			},
			expErr: "",
		},
		{
			name: "invalid signer",
			msg: types.MsgEquivocationEvidence{
				Signer:               "invalid-address",
				FpBtcPk:              &validPk,
				PubRand:              &validPubRand,
				CanonicalAppHash:     validHash,
				ForkAppHash:          validHash,
				CanonicalFinalitySig: &validFinalitySig,
				ForkFinalitySig:      &validFinalitySig,
			},
			expErr: "invalid signer address",
		},
		{
			name: "nil FpBtcPk",
			msg: types.MsgEquivocationEvidence{
				Signer:               validAddr,
				FpBtcPk:              nil,
				PubRand:              &validPubRand,
				CanonicalAppHash:     validHash,
				ForkAppHash:          validHash,
				CanonicalFinalitySig: &validFinalitySig,
				ForkFinalitySig:      &validFinalitySig,
			},
			expErr: "empty FpBtcPk",
		},
		{
			name: "nil PubRand",
			msg: types.MsgEquivocationEvidence{
				Signer:               validAddr,
				FpBtcPk:              &validPk,
				PubRand:              nil,
				CanonicalAppHash:     validHash,
				ForkAppHash:          validHash,
				CanonicalFinalitySig: &validFinalitySig,
				ForkFinalitySig:      &validFinalitySig,
			},
			expErr: "empty PubRand",
		},
		{
			name: "invalid CanonicalAppHash length",
			msg: types.MsgEquivocationEvidence{
				Signer:               validAddr,
				FpBtcPk:              &validPk,
				PubRand:              &validPubRand,
				CanonicalAppHash:     []byte("short"),
				ForkAppHash:          validHash,
				CanonicalFinalitySig: &validFinalitySig,
				ForkFinalitySig:      &validFinalitySig,
			},
			expErr: "malformed CanonicalAppHash",
		},
		{
			name: "invalid ForkAppHash length",
			msg: types.MsgEquivocationEvidence{
				Signer:               validAddr,
				FpBtcPk:              &validPk,
				PubRand:              &validPubRand,
				CanonicalAppHash:     validHash,
				ForkAppHash:          []byte("short"),
				CanonicalFinalitySig: &validFinalitySig,
				ForkFinalitySig:      &validFinalitySig,
			},
			expErr: "malformed ForkAppHash",
		},
		{
			name: "nil ForkFinalitySig",
			msg: types.MsgEquivocationEvidence{
				Signer:               validAddr,
				FpBtcPk:              &validPk,
				PubRand:              &validPubRand,
				CanonicalAppHash:     validHash,
				ForkAppHash:          validHash,
				CanonicalFinalitySig: &validFinalitySig,
				ForkFinalitySig:      nil,
			},
			expErr: "empty ForkFinalitySig",
		},
		{
			name: "nil CanonicalFinalitySig",
			msg: types.MsgEquivocationEvidence{
				Signer:               validAddr,
				FpBtcPk:              &validPk,
				PubRand:              &validPubRand,
				CanonicalAppHash:     validHash,
				ForkAppHash:          validHash,
				CanonicalFinalitySig: nil,
				ForkFinalitySig:      &validFinalitySig,
			},
			expErr: "empty CanonicalFinalitySig",
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

func TestMsgResumeFinalityProposal_ValidateBasic(t *testing.T) {
	t.Parallel()
	r := rand.New(rand.NewSource(time.Now().Unix()))

	validHalting := uint32(10)
	validAddr := datagen.GenRandomAddress().String()
	validPk1, err := datagen.GenRandomBIP340PubKey(r)
	require.NoError(t, err)
	validPk2, err := datagen.GenRandomBIP340PubKey(r)
	require.NoError(t, err)

	validPk1Hex := validPk1.MarshalHex()
	validPk2Hex := validPk2.MarshalHex()

	tcs := []struct {
		name   string
		msg    types.MsgResumeFinalityProposal
		expErr error
	}{
		{
			name: "valid message",
			msg: types.MsgResumeFinalityProposal{
				Authority:     validAddr,
				FpPksHex:      []string{validPk1Hex},
				HaltingHeight: validHalting,
			},
			expErr: nil,
		},
		{
			name: "valid: multiple fps",
			msg: types.MsgResumeFinalityProposal{
				Authority:     validAddr,
				FpPksHex:      []string{validPk1Hex, validPk2Hex},
				HaltingHeight: validHalting,
			},
			expErr: nil,
		},
		{
			name: "invalid: bad authority",
			msg: types.MsgResumeFinalityProposal{
				Authority:     "xxx",
				FpPksHex:      []string{validPk1Hex},
				HaltingHeight: validHalting,
			},
			expErr: errorsmod.Wrap(sdkerrors.ErrInvalidAddress, "invalid authority address (decoding bech32 failed: invalid bech32 string length 3)"),
		},
		{
			name: "invalid: halt zero",
			msg: types.MsgResumeFinalityProposal{
				Authority:     validAddr,
				FpPksHex:      []string{validPk1Hex},
				HaltingHeight: 0,
			},
			expErr: types.ErrInvalidResumeFinality.Wrap("halting height is zero"),
		},
		{
			name: "invalid: no fp pk",
			msg: types.MsgResumeFinalityProposal{
				Authority:     validAddr,
				FpPksHex:      []string{},
				HaltingHeight: validHalting,
			},
			expErr: types.ErrInvalidResumeFinality.Wrap("no fp pk hex set"),
		},
		{
			name: "invalid: bad pk",
			msg: types.MsgResumeFinalityProposal{
				Authority:     validAddr,
				FpPksHex:      []string{"xxxx"},
				HaltingHeight: validHalting,
			},
			expErr: types.ErrInvalidResumeFinality.Wrapf("failed to parse FP BTC PK Hex (xxxx) into BIP-340"),
		},
		{
			name: "invalid: duplicate fp",
			msg: types.MsgResumeFinalityProposal{
				Authority:     validAddr,
				FpPksHex:      []string{validPk1Hex, validPk2Hex, validPk1Hex},
				HaltingHeight: validHalting,
			},
			expErr: types.ErrInvalidResumeFinality.Wrapf("duplicated FP BTC PK Hex (%s)", validPk1Hex),
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			actErr := tc.msg.ValidateBasic()
			if tc.expErr == nil {
				require.NoError(t, actErr)
				return
			}

			require.Error(t, actErr)
			require.EqualError(t, actErr, tc.expErr.Error())
		})
	}
}
