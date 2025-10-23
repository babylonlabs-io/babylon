package types_test

import (
	"math/big"
	"testing"
	"time"

	"github.com/babylonlabs-io/babylon/v4/types"
	btcchaincfg "github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/wire"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type btcutilsTestSuite struct {
	suite.Suite
	mainnetHeader, testnetHeader, mainnetHeaderInvalidTs *wire.BlockHeader
	mainnetPowLimit, testnetPowLimit                     *big.Int
}

func TestBtcutilsTestSuite(t *testing.T) {
	suite.Run(t, new(btcutilsTestSuite))
}

func (s *btcutilsTestSuite) SetupSuite() {
	s.T().Parallel()
	mainnetHeaderHex := "00006020c6c5a20e29da938a252c945411eba594cbeba021a1e20000000000000000000039e4bd0cd0b5232bb380a9576fcfe7d8fb043523f7a158187d9473e44c1740e6b4fa7c62ba01091789c24c22"
	testnetHeaderHex := "0000202015f76d6c7147a1cd3cca4ec31b75ac0218199e863ebd040c6000000000000000d321bbe2d2e323781b4ab89abc24d37c6ad70d92a5169f9775b650447548711162957a626fa4001a90376af4"
	mainnetHeaderBytes, _ := types.NewBTCHeaderBytesFromHex(mainnetHeaderHex)
	testnetHeaderBytes, _ := types.NewBTCHeaderBytesFromHex(testnetHeaderHex)

	s.mainnetHeader = mainnetHeaderBytes.ToBlockHeader()
	s.testnetHeader = testnetHeaderBytes.ToBlockHeader()

	mainnetHeader := *s.mainnetHeader
	s.mainnetHeaderInvalidTs = &mainnetHeader
	s.mainnetHeaderInvalidTs.Timestamp = time.Now()

	s.mainnetPowLimit = btcchaincfg.MainNetParams.PowLimit
	s.testnetPowLimit = btcchaincfg.TestNet3Params.PowLimit
}

func (s *btcutilsTestSuite) TestValidateBTCHeader() {
	data := []struct {
		name     string
		header   *wire.BlockHeader
		powLimit *big.Int
		hasErr   bool
	}{
		{"valid mainnet", s.mainnetHeader, s.mainnetPowLimit, false},
		{"valid testnet", s.testnetHeader, s.testnetPowLimit, false},
		{"mainnet invalid limit", s.mainnetHeader, big.NewInt(0), true},
		{"testnet invalid limit", s.testnetHeader, big.NewInt(0), true},
		{"mainnet invalid timestamp", s.mainnetHeaderInvalidTs, s.mainnetPowLimit, true},
	}

	for _, d := range data {
		err := types.ValidateBTCHeader(d.header, d.powLimit)
		if d.hasErr {
			s.Require().Error(err, d.name)
		} else {
			s.Require().NoError(err, d.name)
		}
	}
}

func TestGetOutputIdxInBTCTx(t *testing.T) {
	pkScript1 := []byte{0x01, 0x02, 0x03}
	pkScript2 := []byte{0x04, 0x05, 0x06}

	tcs := []struct {
		name   string
		tx     *wire.MsgTx
		output *wire.TxOut
		expIdx uint32
		expErr error
	}{
		{
			name: "single matching output",
			tx: &wire.MsgTx{
				TxOut: []*wire.TxOut{
					{Value: 1000, PkScript: pkScript1},
					{Value: 2000, PkScript: pkScript2},
				},
			},
			output: &wire.TxOut{Value: 1000, PkScript: pkScript1},
			expIdx: 0,
		},
		{
			name: "matching output at second position",
			tx: &wire.MsgTx{
				TxOut: []*wire.TxOut{
					{Value: 1000, PkScript: pkScript1},
					{Value: 2000, PkScript: pkScript2},
				},
			},
			output: &wire.TxOut{Value: 2000, PkScript: pkScript2},
			expIdx: 1,
		},
		{
			name: "output not found",
			tx: &wire.MsgTx{
				TxOut: []*wire.TxOut{
					{Value: 1000, PkScript: pkScript1},
				},
			},
			output: &wire.TxOut{Value: 2000, PkScript: pkScript2},
			expErr: types.ErrOutputNotFound,
		},
		{
			name: "multiple outputs with same value but different script",
			tx: &wire.MsgTx{
				TxOut: []*wire.TxOut{
					{Value: 1000, PkScript: pkScript1},
					{Value: 1000, PkScript: pkScript2},
				},
			},
			output: &wire.TxOut{Value: 1000, PkScript: pkScript1},
			expIdx: 0,
		},
		{
			name: "multiple outputs with same script but different value",
			tx: &wire.MsgTx{
				TxOut: []*wire.TxOut{
					{Value: 1000, PkScript: pkScript1},
					{Value: 2000, PkScript: pkScript1},
				},
			},
			output: &wire.TxOut{Value: 1000, PkScript: pkScript1},
			expIdx: 0,
		},
		{
			name: "duplicate outputs - multiple matches error",
			tx: &wire.MsgTx{
				TxOut: []*wire.TxOut{
					{Value: 1000, PkScript: pkScript1},
					{Value: 1000, PkScript: pkScript1},
				},
			},
			output: &wire.TxOut{Value: 1000, PkScript: pkScript1},
			expErr: types.ErrMultipleOutputsMatch,
		},
		{
			name: "duplicate outputs at different positions",
			tx: &wire.MsgTx{
				TxOut: []*wire.TxOut{
					{Value: 1000, PkScript: pkScript1},
					{Value: 2000, PkScript: pkScript2},
					{Value: 1000, PkScript: pkScript1},
				},
			},
			output: &wire.TxOut{Value: 1000, PkScript: pkScript1},
			expErr: types.ErrMultipleOutputsMatch,
		},
		{
			name: "empty tx outputs",
			tx: &wire.MsgTx{
				TxOut: []*wire.TxOut{},
			},
			output: &wire.TxOut{Value: 1000, PkScript: pkScript1},
			expErr: types.ErrOutputNotFound,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			actIdx, actErr := types.GetOutputIdxInBTCTx(tc.tx, tc.output)
			if tc.expErr != nil {
				require.EqualError(t, actErr, tc.expErr.Error())
				return
			}
			require.NoError(t, actErr)
			require.Equal(t, tc.expIdx, actIdx)
		})
	}
}
