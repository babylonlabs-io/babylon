package types_test

import (
	"math/rand"
	"testing"
	"time"

	errorsmod "cosmossdk.io/errors"
	"github.com/babylonlabs-io/babylon/v4/crypto/eots"
	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	"github.com/babylonlabs-io/babylon/v4/x/finality/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
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
