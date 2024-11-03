package types_test

import (
	"github.com/babylonlabs-io/babylon/crypto/ecdsa"
	"github.com/cometbft/cometbft/crypto/tmhash"

	"encoding/base64"
	"encoding/hex"
	"fmt"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/babylonlabs-io/babylon/crypto/bip322"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/chaincfg"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/babylonlabs-io/babylon/testutil/datagen"
	bbn "github.com/babylonlabs-io/babylon/types"
	"github.com/babylonlabs-io/babylon/x/btcstaking/types"
)

var (
	net = &chaincfg.TestNet3Params
)

func newInvalidBIP340PoP(r *rand.Rand) *types.ProofOfPossessionBTC {
	return &types.ProofOfPossessionBTC{
		BtcSigType: types.BTCSigType_BIP340,
		BtcSig:     datagen.GenRandomByteArray(r, 32), // fake sig hash
	}
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

		// generate and verify PoP, correct case
		pop, err := types.NewPoPBTC(accAddr, btcSK)
		require.NoError(t, err)
		err = pop.VerifyBIP340(accAddr, bip340PK)
		require.NoError(t, err)

		// generate and verify PoP, invalid case
		invalidPoP := newInvalidBIP340PoP(r)
		err = invalidPoP.VerifyBIP340(accAddr, bip340PK)
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

		// generate and verify PoP, correct case
		pop, err := types.NewPoPBTCWithECDSABTCSig(accAddr, btcSK)
		require.NoError(t, err)
		err = pop.VerifyECDSA(accAddr, bip340PK)
		require.NoError(t, err)
	})
}

func TestPopFt(t *testing.T) {
	privateKeyStr := "cUiuX4MkxHvLxpteByWDe4CAADpZMBzZFRzfiXvyf8sQFZLZH7fF"
	privateKeyBytes, err := base64.StdEncoding.DecodeString(privateKeyStr)
	require.NoError(t, err)

	bbnAddrStr := "bbn1usyvyrep4qvkpe9pj9hku39sz6fsxkm0pd38wu"
	bbnAddr, err := sdk.AccAddressFromBech32(bbnAddrStr)
	require.NoError(t, err)

	bbnAddrBz := bbnAddr.Bytes()
	// require.Equal(t, "x", fmt.Sprintf("%s", bbnAddrBz))
	bbnAddrBzHex := hex.EncodeToString(bbnAddrBz)
	require.Equal(t, "e408c20f21a81960e4a1916f6e44b01693035b6f", bbnAddrBzHex)

	bbnAddrSha256 := tmhash.Sum(bbnAddr.Bytes())
	bbnAddrSha256Hex := hex.EncodeToString(bbnAddrSha256)
	require.Equal(t, "765cf28f2bb99786473e892bf29588f9078625cee1a0f57edb4c0b08c868efa0", bbnAddrSha256Hex)

	privKeyBtcCec, pubKeyBtcCec := btcec.PrivKeyFromBytes(privateKeyBytes)
	signed, err := ecdsa.Sign(privKeyBtcCec, bbnAddrBzHex)
	require.NoError(t, err)

	signedHex := hex.EncodeToString(signed)
	require.Equal(t, "207ba0d0a3f761d86573a96c08184232fbfa3ff830a04f38738069185f5518ab984870fc59e55b8861ab90881e260331977a98ee8e319e3871a4298056a9e60818", signedHex)

	popEcdsaBtc, err := types.NewPoPBTCWithECDSABTCSig(bbnAddr, privKeyBtcCec)
	require.NoError(t, err)

	popEcdsaBtcHexOfMarshal, err := popEcdsaBtc.ToHexStr()
	require.NoError(t, err)

	require.Equal(t, "08021241207ba0d0a3f761d86573a96c08184232fbfa3ff830a04f38738069185f5518ab984870fc59e55b8861ab90881e260331977a98ee8e319e3871a4298056a9e60818", popEcdsaBtcHexOfMarshal)

	bip322EncodedFromFront, err := base64.StdEncoding.DecodeString("AkcwRAIgVl1CWJAw3SIF7CLj1+iMnJ9mAFaVPNRgVyFPnqryQjgCIGlJEt/YwLUf81flhNcdlw3QpObk83CRgTqwHMENfGheASEDtLJdctLYE77nPGeUc9219HlWrpPkGj4WtgpxkLzLeK8=")
	require.NoError(t, err)

	pop, err := types.NewPoPBTCWithBIP322P2WPKHSig(bbnAddr, privKeyBtcCec, &chaincfg.SigNetParams)
	require.NoError(t, err)
	pop.BtcSig = bip322EncodedFromFront
	err = pop.VerifyBIP322(bbnAddr, bbn.NewBIP340PubKeyFromBTCPK(pubKeyBtcCec), net)
	require.NoError(t, err)

	// schSig, err := schnorr.ParseSignature(bip322EncodedFromFront)
	// require.NoError(t, err)
	// require.Equal(t, "x", schSig)

	bip322SignedAddrBz, err := types.NewBIP322Sig(bbnAddrSha256, privKeyBtcCec, &chaincfg.SigNetParams, bip322.SignWithP2WPKHAddress)
	require.NoError(t, err)

	bip322SignedAddrHex := hex.EncodeToString(bip322SignedAddrBz)
	require.Equal(t, "0a2a746231713377787473743437786535636e387234766d71356c636672377874776a6c386461716b7a3861126c02483045022100cf3db1f6db234e0bb375d5ae38dd1180b7b298a12244eeadaa27a57540996ddb02206ff6a44f1ab495d799a76f4722916991718ec8c142bced33b2e28339fec5233f012103e64d9214bfdee7adabb3f4b701d2dcf8112419699ba50e8134533a8c8054f8d5", bip322SignedAddrHex)
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

		// generate and verify PoP, correct case
		pop, err := types.NewPoPBTCWithBIP322P2WPKHSig(accAddr, btcSK, net)
		require.NoError(t, err)
		err = pop.VerifyBIP322(accAddr, bip340PK, net)
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

		// generate and verify PoP, correct case
		pop, err := types.NewPoPBTCWithBIP322P2TRBIP86Sig(accAddr, btcSK, net)
		require.NoError(t, err)
		err = pop.VerifyBIP322(accAddr, bip340PK, net)
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

		// generate valid bip322 P2WPKH pop
		pop, err := types.NewPoPBTCWithBIP322P2WPKHSig(accAddr, btcSK, net)
		require.NoError(t, err)

		// verify bip322 pop with incorrect staker key
		err = pop.VerifyBIP322(accAddr, bip340PK1, net)
		require.Error(t, err)

		// generate valid bip322 P2Tr pop
		pop, err = types.NewPoPBTCWithBIP322P2TRBIP86Sig(accAddr, btcSK, net)
		require.NoError(t, err)

		// verify bip322 pop with incorrect staker key
		err = pop.VerifyBIP322(accAddr, bip340PK1, net)
		require.Error(t, err)
	})
}

func TestPoPBTCValidateBasic(t *testing.T) {
	r := rand.New(rand.NewSource(10))

	btcSK, _, err := datagen.GenRandomBTCKeyPair(r)
	require.NoError(t, err)

	addrToSign := sdk.MustAccAddressFromBech32(datagen.GenRandomAccount().Address)

	popBip340, err := types.NewPoPBTC(addrToSign, btcSK)
	require.NoError(t, err)

	popBip322, err := types.NewPoPBTCWithBIP322P2WPKHSig(addrToSign, btcSK, &chaincfg.MainNetParams)
	require.NoError(t, err)

	popECDSA, err := types.NewPoPBTCWithECDSABTCSig(addrToSign, btcSK)
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

	popBip340, err := types.NewPoPBTC(addrToSign, btcSK)
	require.NoError(t, err)

	popBip322, err := types.NewPoPBTCWithBIP322P2WPKHSig(addrToSign, btcSK, netParams)
	require.NoError(t, err)

	popECDSA, err := types.NewPoPBTCWithECDSABTCSig(addrToSign, btcSK)
	require.NoError(t, err)

	tcs := []struct {
		title  string
		staker sdk.AccAddress
		btcPK  *bbn.BIP340PubKey
		pop    *types.ProofOfPossessionBTC
		expErr error
	}{
		{
			"valid: BIP340",
			addrToSign,
			bip340PK,
			popBip340,
			nil,
		},
		{
			"valid: BIP322",
			addrToSign,
			bip340PK,
			popBip322,
			nil,
		},
		{
			"valid: ECDSA",
			addrToSign,
			bip340PK,
			popECDSA,
			nil,
		},
		{
			"invalid: BIP340 - bad addr",
			randomAddr,
			bip340PK,
			popBip340,
			fmt.Errorf("failed to verify pop.BtcSig"),
		},
		{
			"invalid: BIP322 - bad addr",
			randomAddr,
			bip340PK,
			popBip322,
			fmt.Errorf("failed to verify possession of babylon sig by the BTC key: signature not empty on failed checksig"),
		},
		{
			"invalid: ECDSA - bad addr",
			randomAddr,
			bip340PK,
			popECDSA,
			fmt.Errorf("failed to verify btcSigRaw"),
		},
		{
			"invalid: SigType",
			nil,
			nil,
			&types.ProofOfPossessionBTC{
				BtcSigType: types.BTCSigType(123),
			},
			fmt.Errorf("invalid BTC signature type"),
		},
		{
			"invalid: nil sig",
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
			nil,
			bip340PK,
			popBip340,
			fmt.Errorf("failed to verify pop.BtcSig"),
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.title, func(t *testing.T) {
			actErr := tc.pop.Verify(tc.staker, tc.btcPK, netParams)
			if tc.expErr != nil {
				require.EqualError(t, actErr, tc.expErr.Error())
				return
			}
			require.NoError(t, actErr)
		})
	}
}
