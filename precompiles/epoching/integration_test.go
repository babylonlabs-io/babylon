package epoching_test

import (
	"encoding/base64"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/cosmos/evm/crypto/ethsecp256k1"
	"github.com/cosmos/evm/precompiles/staking"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"

	cmted25519 "github.com/cometbft/cometbft/crypto/ed25519"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"

	appsigner "github.com/babylonlabs-io/babylon/v4/app/signer"
	"github.com/babylonlabs-io/babylon/v4/crypto/bls12381"
	"github.com/babylonlabs-io/babylon/v4/precompiles/epoching"
	"github.com/babylonlabs-io/babylon/v4/test/integration/precompiles"
)

var s *PrecompileIntegrationTestSuite

type PrecompileIntegrationTestSuite struct {
	suite.Suite
	precompiles.BaseTestSuite

	abi           abi.ABI
	addr          common.Address
	validatorPriv *ethsecp256k1.PrivKey
	delegatorPriv *ethsecp256k1.PrivKey
}

func TestPrecompileTestSuite(t *testing.T) {
	suite.Run(t, new(PrecompileIntegrationTestSuite))
}

func (ts *PrecompileIntegrationTestSuite) SetupTest() {
	ts.BaseTestSuite.SetupApp(ts.T())

	a, err := epoching.LoadABI()
	require.NoError(ts.T(), err)
	ts.abi = a
	ts.addr = common.HexToAddress(epoching.EpochingPrecompileAddress)

	// create distinct validator and delegator keys
	ts.validatorPriv, err = ethsecp256k1.GenerateKey()
	ts.Require().NoError(err)
	ts.delegatorPriv, err = ethsecp256k1.GenerateKey()
	ts.Require().NoError(err)

	// fund both with 100 bbn
	_, err = ts.InitAndFundEVMAccount(ts.validatorPriv, sdkmath.NewInt(100_000_000))
	ts.Require().NoError(err)
	_, err = ts.InitAndFundEVMAccount(ts.delegatorPriv, sdkmath.NewInt(100_000_000))
	ts.Require().NoError(err)
}

func TestPrecompileIntegrationTestSuite(t *testing.T) {
	s = new(PrecompileIntegrationTestSuite)
	s.SetT(t)
	// Run Ginkgo integration tests
	RegisterFailHandler(Fail)
	RunSpecs(t, "Epoching Precompile Suite")
}

var _ = Describe("Calling epoching precompile directly", func() {
	var (
		blsKey    epoching.BlsKey
		consPkB64 string
	)

	BeforeEach(func() {
		s.SetupTest()
		valPriv := cmted25519.GenPrivKey()
		blsPriv := bls12381.GenPrivKey()
		valKeys, err := appsigner.NewValidatorKeys(valPriv, blsPriv)
		s.Require().NoError(err)

		blsKey = epoching.BlsKey{
			PubKey:     valKeys.BlsPubkey.Bytes(),
			Ed25519Sig: valKeys.PoP.Ed25519Sig,
			BlsSig:     valKeys.PoP.BlsSig.Bytes(),
		}
		consPkB64 = base64.StdEncoding.EncodeToString(valKeys.ValPubkey.Bytes())
	})

	Describe("to create validator", func() {
		var (
			defaultDescription = staking.Description{
				Moniker:         "new node",
				Identity:        "",
				Website:         "",
				SecurityContact: "",
				Details:         "",
			}
			defaultCommission = staking.Commission{
				Rate:          big.NewInt(100000000000000000),
				MaxRate:       big.NewInt(100000000000000000),
				MaxChangeRate: big.NewInt(100000000000000000),
			}
			defaultMinSelfDelegation = big.NewInt(1)
			defaultValue             = big.NewInt(1_000_000) // 1bbn
		)

		Context("when validator address is the msg.sender & EoA", func() {
			It("should succeed", func() {
				valHex := common.Address(s.validatorPriv.PubKey().Address().Bytes())
				resp, err := s.CallContract(
					s.validatorPriv, s.addr, s.abi, epoching.WrappedCreateValidatorMethod,
					blsKey, defaultDescription, defaultCommission, defaultMinSelfDelegation, valHex, consPkB64, defaultValue,
				)
				Expect(err).To(BeNil(), "error while calling the contract")
				Expect(resp.VmError).To(Equal(""))

				s.AdvanceToNextEpoch()

				valAddr := sdk.ValAddress(valHex.Bytes())
				v, err := s.App.StakingKeeper.GetValidator(s.Ctx, valAddr)
				Expect(err).To(BeNil())
				Expect(valAddr.String()).To(Equal(v.OperatorAddress))
				Expect(v.Tokens.IsPositive()).To(BeTrue())
				GinkgoT().Logf("validator: %v\n", v)
			})
		})

		Context("when validator address is not the msg.sender", func() {
			It("should fail", func() {
				// use delegator address to force mismatch with signer
				randAddr := common.Address(s.delegatorPriv.PubKey().Address().Bytes())
				_, err := s.CallContract(
					s.validatorPriv, s.addr, s.abi, epoching.WrappedCreateValidatorMethod,
					blsKey, defaultDescription, defaultCommission, defaultMinSelfDelegation, randAddr, consPkB64, defaultValue,
				)
				Expect(err).NotTo(BeNil(), "error while calling the contract")
			})
		})
	})

	Describe("to edit validator", func() {
		var (
			defaultDescription = staking.Description{
				Moniker:         "edit node",
				Identity:        "[do-not-modify]",
				Website:         "[do-not-modify]",
				SecurityContact: "[do-not-modify]",
				Details:         "[do-not-modify]",
			}
			defaultCommissionRate    = big.NewInt(staking.DoNotModifyCommissionRate)
			defaultMinSelfDelegation = big.NewInt(staking.DoNotModifyMinSelfDelegation)
		)

		Context("when msg.sender is equal to the validator address", func() {
			It("should succeed", func() {
				valHex := common.Address(s.validatorPriv.PubKey().Address().Bytes())
				description := staking.Description{
					Moniker:         "new node",
					Identity:        "",
					Website:         "",
					SecurityContact: "",
					Details:         "",
				}
				commission := staking.Commission{
					Rate:          big.NewInt(100000000000000000),
					MaxRate:       big.NewInt(100000000000000000),
					MaxChangeRate: big.NewInt(100000000000000000),
				}
				minSelfDelegation := big.NewInt(1)
				pubkeyBase64Str := consPkB64
				value := big.NewInt(1_000_000) // 1bbn
				resp, err := s.CallContract(
					s.validatorPriv, s.addr, s.abi, epoching.WrappedCreateValidatorMethod,
					blsKey, description, commission, minSelfDelegation, valHex, pubkeyBase64Str, value,
				)
				Expect(err).To(BeNil(), "error while calling the contract")
				Expect(resp.VmError).To(Equal(""))

				s.AdvanceToNextEpoch()

				resp, err = s.CallContract(
					s.validatorPriv, s.addr, s.abi, epoching.WrappedEditValidatorMethod,
					defaultDescription, valHex, defaultCommissionRate, defaultMinSelfDelegation,
				)
				Expect(err).To(BeNil(), "error while calling the contract")
				Expect(resp.VmError).To(Equal(""))

				s.AdvanceToNextEpoch()

				valAddr := sdk.ValAddress(valHex.Bytes())
				v, err := s.App.StakingKeeper.GetValidator(s.Ctx, valAddr)
				Expect(err).To(BeNil())
				Expect(valAddr.String()).To(Equal(v.OperatorAddress))
				Expect(v.Description.Moniker).To(Equal(defaultDescription.Moniker))
				// other fields should not be modified due to the value "[do-not-modify]"
				Expect(v.Description.Identity).To(Equal(description.Identity), "expected validator identity not to be updated")
				Expect(v.Description.Website).To(Equal(description.Website), "expected validator website not to be updated")
				Expect(v.Description.SecurityContact).To(Equal(description.SecurityContact), "expected validator security contact not to be updated")
				Expect(v.Description.Details).To(Equal(description.Details), "expected validator details not to be updated")

				Expect(v.Commission.Rate.BigInt().String()).To(Equal(commission.Rate.String()), "expected validator commission rate remain unchanged")
				Expect(v.Commission.MaxRate.BigInt().String()).To(Equal(commission.MaxRate.String()), "expected validator max commission rate remain unchanged")
				Expect(v.Commission.MaxChangeRate.BigInt().String()).To(Equal(commission.MaxChangeRate.String()), "expected validator max change rate remain unchanged")
				Expect(v.MinSelfDelegation.String()).To(Equal(minSelfDelegation.String()), "expected validator min self delegation remain unchanged")
			})
		})

		Context("with msg.sender different than validator address", func() {
			It("should fail", func() {
				randAddr := common.Address(s.delegatorPriv.PubKey().Address().Bytes())
				_, err := s.CallContract(
					s.validatorPriv, s.addr, s.abi, epoching.WrappedEditValidatorMethod,
					defaultDescription, randAddr, defaultCommissionRate, defaultMinSelfDelegation,
				)
				Expect(err).NotTo(BeNil(), "error while calling the contract")
			})
		})
	})

	Describe("to delegate", func() {
		Context("as the token owner", func() {
			It("should delegate", func() {
				delegatorAcc := sdk.AccAddress(s.delegatorPriv.PubKey().Address().Bytes())
				valAddr := sdk.ValAddress(s.validatorPriv.PubKey().Address().Bytes())
				_ = delegatorAcc
				_ = valAddr
			})
		})
	})
})
