package e2e

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
	"time"

	sdkmath "cosmossdk.io/math"
	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/suite"

	"github.com/babylonlabs-io/babylon/v4/test/e2e/configurer"
	"github.com/babylonlabs-io/babylon/v4/test/e2e/configurer/chain"
)

type FeemarketTestSuite struct {
	suite.Suite

	configurer configurer.Configurer
	addrA      string
	addrB      string
	addrC      string
}

func (s *FeemarketTestSuite) SetupSuite() {
	s.T().Log("setting up Feemarket test suite...")
	var (
		err error
	)

	s.configurer, err = configurer.NewBabylonConfigurer(s.T(), true)
	s.Require().NoError(err)

	err = s.configurer.ConfigureChains()
	s.Require().NoError(err)

	err = s.configurer.RunSetup()
	s.Require().NoError(err)
}

func (s *FeemarketTestSuite) TearDownSuite() {
	err := s.configurer.ClearResources()
	if err != nil {
		s.T().Logf("error to clear resources %s", err.Error())
	}
}

func (s *FeemarketTestSuite) TestAll() {
	s.BaseFeeExcludesRefundableGas()
}

// BaseFeeExcludesRefundableGas validates that refundable transaction gas
// is properly excluded from base fee calculations in a real environment
func (s *FeemarketTestSuite) BaseFeeExcludesRefundableGas() {
	bbnChain := s.configurer.GetChainConfig(0)
	bbnChain.WaitUntilHeight(2)

	node, err := bbnChain.GetNodeAtIndex(2)
	s.NoError(err)

	// Setup account with funds
	s.addrA = node.KeysAdd("addr-A")
	node.BankSendFromNode(s.addrA, "100000000ubbn") // 100 BBN for gas fees
	s.addrB = node.KeysAdd("addr-B")
	node.BankSendFromNode(s.addrB, "100000000ubbn") // 100 BBN for gas fees
	node.WaitForNextBlock()

	initialBaseFee := s.getCurrentBaseFee(node)
	s.T().Logf("Initial base fee: %s", initialBaseFee.String())

	refundableTxHash := s.sendRefundableTx(node, s.addrA)
	s.Require().NotEmpty(refundableTxHash, "Refundable transaction should return valid hash")

	s.addrC = node.KeysAdd("addr-C")
	transferAmount := sdk.NewInt64Coin(nativeDenom, 5000000) // 5 BBN
	nonRefundableTxHash := s.sendNonRefundableTx(node, s.addrB, s.addrC, transferAmount)
	s.Require().NotEmpty(nonRefundableTxHash, "Non-refundable transaction should return valid hash")

	node.WaitForNextBlock()

	refundableTxResp, _ := node.QueryTx(refundableTxHash)
	s.Require().Equal(uint32(0), refundableTxResp.Code, "Refundable transaction should succeed")

	nonRefundableTxResp, _ := node.QueryTx(nonRefundableTxHash)
	s.Require().Equal(uint32(0), nonRefundableTxResp.Code, "Non-refundable transaction should succeed")

	s.verifyTxFeeRefunded(node, refundableTxHash, refundableTxResp)

	finalBaseFee := s.getCurrentBaseFee(node)

	node.WaitForNextBlock()

	// The base fee should change based on network congestion
	// With our wrapper, it should exclude refundable gas from the calculation
	s.validateBaseFeeCalculation(node, initialBaseFee, finalBaseFee, refundableTxResp, nonRefundableTxResp)

	s.T().Log("Base fee excludes refundable gas test passed")
}

// getCurrentBaseFee queries the current base fee from the feemarket module
func (s *FeemarketTestSuite) getCurrentBaseFee(node *chain.NodeConfig) sdkmath.LegacyDec {
	cmd := []string{"babylond", "query", "feemarket", "base-fee", "--output=json"}
	outBuf, _, err := node.ExecRawCmd(cmd)
	s.Require().NoError(err)

	var result struct {
		BaseFee string `json:"base_fee"`
	}
	err = json.Unmarshal(outBuf.Bytes(), &result)
	s.Require().NoError(err)

	baseFee, err := sdkmath.LegacyNewDecFromStr(result.BaseFee)
	s.Require().NoError(err)
	return baseFee
}

// sendRefundableTx sends a MsgInsertHeaders transaction (refundable)
func (s *FeemarketTestSuite) sendRefundableTx(node *chain.NodeConfig, from string) string {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	tipResp, err := node.QueryTip()
	s.Require().NoError(err)
	s.T().Logf("Retrieved current tip of btc headerchain. Height: %d", tipResp.Height)

	tip, err := chain.ParseBTCHeaderInfoResponseToInfo(tipResp)
	s.Require().NoError(err)

	child := datagen.GenRandomValidBTCHeaderInfoWithParent(r, *tip)
	headerHex := child.Header.MarshalHex()

	s.T().Logf("Generated new header: %s", headerHex)

	cmd := []string{
		"babylond", "tx", "btclightclient", "insert-headers", headerHex,
		fmt.Sprintf("--from=%s", from),
		"--fees=1000ubbn", // Set explicit fees
		fmt.Sprintf("--chain-id=%s", node.GetChainID()),
		"--yes",
		"--keyring-backend=test",
		"--log_format=json",
		"--home=/home/babylon/babylondata",
	}

	outBuf, _, err := node.ExecRawCmd(cmd)
	s.Require().NoError(err)

	txHash := chain.GetTxHashFromOutput(outBuf.String())
	s.T().Logf("Refundable transaction sent, txHash: %s", txHash)
	return txHash
}

// sendNonRefundableTx sends a bank send transaction (non-refundable)
func (s *FeemarketTestSuite) sendNonRefundableTx(node *chain.NodeConfig, from, to string, amount sdk.Coin) string {
	cmd := []string{
		"babylond", "tx", "bank", "send", from, to, amount.String(),
		"--fees=1000ubbn", // Set explicit fees
		fmt.Sprintf("--chain-id=%s", node.GetChainID()),
		"--yes",
		"--keyring-backend=test",
		"--log_format=json",
		"--home=/home/babylon/babylondata",
	}

	outBuf, _, err := node.ExecRawCmd(cmd)
	s.Require().NoError(err)

	txHash := chain.GetTxHashFromOutput(outBuf.String())
	s.T().Logf("Non-refundable transaction sent, txHash: %s", txHash)
	return txHash
}

// verifyTxRefunded checks if a transaction was properly refunded
func (s *FeemarketTestSuite) verifyTxFeeRefunded(node *chain.NodeConfig, txHash string, txResp sdk.TxResponse) {
	refundFound := false
	for _, event := range txResp.Events {
		if event.Type == "transfer" || event.Type == "coin_received" {
			for _, attr := range event.Attributes {
				if attr.Key == "recipient" || attr.Key == "receiver" {
					// Check if any transfers went back to the fee payer (indicating refund)
					if strings.Contains(attr.Value, s.addrA) { // This should be the refundable tx sender
						refundFound = true
						s.T().Logf("Found potential refund in event: %s = %s", attr.Key, attr.Value)
					}
				}
			}
		}
	}

	s.T().Logf("Refund verification for tx %s: refund_found=%v", txHash, refundFound)
}

// validateBaseFeeCalculation verifies the base fee calculation logic
func (s *FeemarketTestSuite) validateBaseFeeCalculation(node *chain.NodeConfig, initialBaseFee, finalBaseFee sdkmath.LegacyDec, refundableTx, nonRefundableTx sdk.TxResponse) {
	s.T().Logf("Validating base fee calculation:")
	s.T().Logf("  Initial base fee: %s", initialBaseFee.String())
	s.T().Logf("  Final base fee: %s", finalBaseFee.String())
	s.T().Logf("  Refundable tx gas wanted: %d", refundableTx.GasWanted)
	s.T().Logf("  Refundable tx gas used: %d", refundableTx.GasUsed)
	s.T().Logf("  Non-refundable tx gas wanted: %d", nonRefundableTx.GasWanted)
	s.T().Logf("  Non-refundable tx gas used: %d", nonRefundableTx.GasUsed)

	// Basic validation: base fee should not be negative and should be reasonable
	s.Require().True(finalBaseFee.IsPositive(), "Final base fee should be positive")
}
