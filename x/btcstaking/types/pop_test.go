package types_test

import (
	"fmt"
	"math/rand"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/require"

	"github.com/btcsuite/btcd/chaincfg"

	sdk "github.com/cosmos/cosmos-sdk/types"

<<<<<<< HEAD
	"github.com/babylonlabs-io/babylon/v2/testutil/datagen"
	bbn "github.com/babylonlabs-io/babylon/v2/types"
	"github.com/babylonlabs-io/babylon/v2/x/btcstaking/types"
=======
	"github.com/babylonlabs-io/babylon/v3/app/signingcontext"
	"github.com/babylonlabs-io/babylon/v3/testutil/datagen"
	bbn "github.com/babylonlabs-io/babylon/v3/types"
	"github.com/babylonlabs-io/babylon/v3/x/btcstaking/types"
>>>>>>> 2b02d75 (Implement context separator signing (#1252))
)

var (
	net         = &chaincfg.TestNet3Params
	testChainID = "test-5"
)

func newInvalidBIP340PoP(r *rand.Rand) *types.ProofOfPossessionBTC {
	return &types.ProofOfPossessionBTC{
		BtcSigType: types.BTCSigType_BIP340,
		BtcSig:     datagen.GenRandomByteArray(r, 32), // fake sig hash
	}
}

func RandomSigningContext(r *rand.Rand) string {
	randomModuleAddress := datagen.GenRandomAccount().GetAddress().String()
	ruint32 := datagen.RandomUInt32(r, 4)

	switch ruint32 {
	case 0:
		return signingcontext.FpFinVoteContextV0(testChainID, randomModuleAddress)
	case 1:
		return signingcontext.FpRandCommitContextV0(testChainID, randomModuleAddress)
	case 2:
		return signingcontext.StakerPopContextV0(testChainID, randomModuleAddress)
	default:
		return signingcontext.FpPopContextV0(testChainID, randomModuleAddress)
	}
}

func Fuzz_MsgToSignBIP322(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		accAddr := datagen.GenRandomAccount().GetAddress()
		addrStr := accAddr.String()
		strUtf8Valid := utf8.ValidString(addrStr)
		require.True(t, strUtf8Valid)

		radnomModuleAddress := datagen.GenRandomAccount().GetAddress().String()

		signingContext := signingcontext.StakerPopContextV0(testChainID, radnomModuleAddress)

		bz := types.MsgToSignBIP322(signingContext, accAddr)
		require.Equal(t, []byte(signingContext+addrStr), bz)

		bzUtf8Valid := utf8.Valid(bz)
		require.True(t, bzUtf8Valid)
	})
}

func FuzzPoP_BIP340(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		// generate BTC key pair
		btcSK, btcPK, err := datagen.GenRandomBTCKeyPair(r)
		require.NoError(t, err)
		bip340PK := bbn.NewBIP340PubKeyFromBTCPK(btcPK)

		accAddr := datagen.GenRandomAccount().GetAddress()
		signingContext := RandomSigningContext(r)
		// generate and verify PoP, correct case
		pop, err := datagen.NewPoPBTC(signingContext, accAddr, btcSK)
		require.NoError(t, err)
		err = pop.VerifyBIP340(signingContext, accAddr, bip340PK)
		require.NoError(t, err)

		// generate and verify PoP, invalid case
		invalidPoP := newInvalidBIP340PoP(r)
		err = invalidPoP.VerifyBIP340(signingContext, accAddr, bip340PK)
		require.Error(t, err)
	})
}

func FuzzPoP_ECDSA(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		// generate BTC key pair
		btcSK, btcPK, err := datagen.GenRandomBTCKeyPair(r)
		require.NoError(t, err)
		bip340PK := bbn.NewBIP340PubKeyFromBTCPK(btcPK)

		accAddr := datagen.GenRandomAccount().GetAddress()
		signingContext := RandomSigningContext(r)

		// generate and verify PoP, correct case
		pop, err := datagen.NewPoPBTCWithECDSABTCSig(signingContext, accAddr, btcSK)
		require.NoError(t, err)
		err = pop.VerifyECDSA(signingContext, accAddr.String(), bip340PK)
		require.NoError(t, err)
	})
}

func FuzzPoP_BIP322_P2WPKH(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		// generate BTC key pair
		btcSK, btcPK, err := datagen.GenRandomBTCKeyPair(r)
		require.NoError(t, err)
		bip340PK := bbn.NewBIP340PubKeyFromBTCPK(btcPK)

		accAddr := datagen.GenRandomAccount().GetAddress()

		signingContext := RandomSigningContext(r)

		// generate and verify PoP, correct case
		pop, err := datagen.NewPoPBTCWithBIP322P2WPKHSig(signingContext, accAddr, btcSK, net)
		require.NoError(t, err)
		err = pop.VerifyBIP322(signingContext, accAddr, bip340PK, net)
		require.NoError(t, err)
	})
}

func FuzzPoP_BIP322_P2Tr_BIP86(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		// generate BTC key pair
		btcSK, btcPK, err := datagen.GenRandomBTCKeyPair(r)
		require.NoError(t, err)
		bip340PK := bbn.NewBIP340PubKeyFromBTCPK(btcPK)

		accAddr := datagen.GenRandomAccount().GetAddress()

		signingContext := RandomSigningContext(r)

		// generate and verify PoP, correct case
		pop, err := datagen.NewPoPBTCWithBIP322P2TRBIP86Sig(signingContext, accAddr, btcSK, net)
		require.NoError(t, err)
		err = pop.VerifyBIP322(signingContext, accAddr, bip340PK, net)
		require.NoError(t, err)
	})
}

// TODO: Add more negative cases
func FuzzPop_ValidBip322SigNotMatchingBip340PubKey(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		// generate two BTC key pairs
		btcSK, _, err := datagen.GenRandomBTCKeyPair(r)
		require.NoError(t, err)

		_, btcPK1, err := datagen.GenRandomBTCKeyPair(r)
		require.NoError(t, err)
		bip340PK1 := bbn.NewBIP340PubKeyFromBTCPK(btcPK1)

		accAddr := datagen.GenRandomAccount().GetAddress()

		signingContext := RandomSigningContext(r)

		// generate valid bip322 P2WPKH pop
		pop, err := datagen.NewPoPBTCWithBIP322P2WPKHSig(signingContext, accAddr, btcSK, net)
		require.NoError(t, err)

		// verify bip322 pop with incorrect staker key
		err = pop.VerifyBIP322(signingContext, accAddr, bip340PK1, net)
		require.Error(t, err)

		// generate valid bip322 P2Tr pop
		pop, err = datagen.NewPoPBTCWithBIP322P2TRBIP86Sig(signingContext, accAddr, btcSK, net)
		require.NoError(t, err)

		// verify bip322 pop with incorrect staker key
		err = pop.VerifyBIP322(signingContext, accAddr, bip340PK1, net)
		require.Error(t, err)
	})
}

func TestPoPBTCValidateBasic(t *testing.T) {
	r := rand.New(rand.NewSource(10))

	btcSK, _, err := datagen.GenRandomBTCKeyPair(r)
	require.NoError(t, err)

	addrToSign := sdk.MustAccAddressFromBech32(datagen.GenRandomAccount().Address)

	signingContext := RandomSigningContext(r)

	popBip340, err := datagen.NewPoPBTC(signingContext, addrToSign, btcSK)
	require.NoError(t, err)

	popBip322, err := datagen.NewPoPBTCWithBIP322P2WPKHSig(signingContext, addrToSign, btcSK, &chaincfg.MainNetParams)
	require.NoError(t, err)

	popECDSA, err := datagen.NewPoPBTCWithECDSABTCSig(signingContext, addrToSign, btcSK)
	require.NoError(t, err)

	tcs := []struct {
		title  string
		pop    *types.ProofOfPossessionBTC
		expErr error
	}{
		{
			"valid: BIP 340",
			popBip340,
			nil,
		},
		{
			"valid: BIP 322",
			popBip322,
			nil,
		},
		{
			"valid: ECDSA",
			popECDSA,
			nil,
		},
		{
			"invalid: nil sig",
			&types.ProofOfPossessionBTC{},
			fmt.Errorf("empty BTC signature"),
		},
		{
			"invalid: BIP 340 - bad sig",
			&types.ProofOfPossessionBTC{
				BtcSigType: types.BTCSigType_BIP340,
				BtcSig:     popBip322.BtcSig,
			},
			fmt.Errorf("invalid BTC BIP340 signature: bytes cannot be converted to a *schnorr.Signature object"),
		},
		{
			"invalid: BIP 322 - bad sig",
			&types.ProofOfPossessionBTC{
				BtcSigType: types.BTCSigType_BIP322,
				BtcSig:     []byte("ss"),
			},
			fmt.Errorf("invalid BTC BIP322 signature: unexpected EOF"),
		},
		{
			"invalid: ECDSA - bad sig",
			&types.ProofOfPossessionBTC{
				BtcSigType: types.BTCSigType_ECDSA,
				BtcSig:     popBip340.BtcSig,
			},
			fmt.Errorf("invalid BTC ECDSA signature size"),
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.title, func(t *testing.T) {
			actErr := tc.pop.ValidateBasic()
			if tc.expErr != nil {
				require.EqualError(t, actErr, tc.expErr.Error())
				return
			}
			require.NoError(t, actErr)
		})
	}
}

func TestPoPBTCVerify(t *testing.T) {
	r := rand.New(rand.NewSource(10))

	addrToSign := sdk.MustAccAddressFromBech32(datagen.GenRandomAccount().Address)
	randomAddr := sdk.MustAccAddressFromBech32(datagen.GenRandomAccount().Address)

	// generate BTC key pair
	btcSK, btcPK, err := datagen.GenRandomBTCKeyPair(r)
	require.NoError(t, err)
	bip340PK := bbn.NewBIP340PubKeyFromBTCPK(btcPK)

	netParams := &chaincfg.MainNetParams

	signingContext := RandomSigningContext(r)

	popBip340, err := datagen.NewPoPBTC(signingContext, addrToSign, btcSK)
	require.NoError(t, err)

	popBip322, err := datagen.NewPoPBTCWithBIP322P2WPKHSig(signingContext, addrToSign, btcSK, netParams)
	require.NoError(t, err)

	popECDSA, err := datagen.NewPoPBTCWithECDSABTCSig(signingContext, addrToSign, btcSK)
	require.NoError(t, err)

	tcs := []struct {
		title          string
		signingContext string
		staker         sdk.AccAddress
		btcPK          *bbn.BIP340PubKey
		pop            *types.ProofOfPossessionBTC
		expErr         error
	}{
		{
			"valid: BIP340",
			signingContext,
			addrToSign,
			bip340PK,
			popBip340,
			nil,
		},
		{
			"valid: BIP322",
			signingContext,
			addrToSign,
			bip340PK,
			popBip322,
			nil,
		},
		{
			"valid: ECDSA",
			signingContext,
			addrToSign,
			bip340PK,
			popECDSA,
			nil,
		},
		{
			"invalid: BIP340 - bad addr",
			signingContext,
			randomAddr,
			bip340PK,
			popBip340,
			fmt.Errorf("failed to verify pop.BtcSig"),
		},
		{
			"invalid: BIP322 - bad addr",
			signingContext,
			randomAddr,
			bip340PK,
			popBip322,
			fmt.Errorf("failed to verify possession of babylon sig by the BTC key: signature not empty on failed checksig"),
		},
		{
			"invalid: ECDSA - bad addr",
			signingContext,
			randomAddr,
			bip340PK,
			popECDSA,
			fmt.Errorf("the recovered PK does not match the given PK"),
		},
		{
			"invalid: SigType",
			signingContext,
			nil,
			nil,
			&types.ProofOfPossessionBTC{
				BtcSigType: types.BTCSigType(123),
			},
			fmt.Errorf("invalid BTC signature type"),
		},
		{
			"invalid: nil sig",
			signingContext,
			randomAddr,
			bip340PK,
			&types.ProofOfPossessionBTC{
				BtcSigType: types.BTCSigType_BIP322,
				BtcSig:     nil,
			},
			fmt.Errorf("failed to verify possession of babylon sig by the BTC key: cannot verify bip322 signature. One of the required parameters is empty"),
		},
		{
			"invalid: nil signed msg",
			signingContext,
			nil,
			bip340PK,
			popBip340,
			fmt.Errorf("failed to verify pop.BtcSig"),
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.title, func(t *testing.T) {
			actErr := tc.pop.Verify(tc.signingContext, tc.staker, tc.btcPK, netParams)
			if tc.expErr != nil {
				require.EqualError(t, actErr, tc.expErr.Error())
				return
			}
			require.NoError(t, actErr)
		})
	}
}
