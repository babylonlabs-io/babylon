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

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"

	appsigner "github.com/babylonlabs-io/babylon/v4/app/signer"
	"github.com/babylonlabs-io/babylon/v4/crypto/bls12381"
	"github.com/babylonlabs-io/babylon/v4/precompiles/epoching"
	"github.com/babylonlabs-io/babylon/v4/testutil/precompiles"
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
	_, err = ts.InitAndFundEVMAccount(ts.validatorPriv, math.NewInt(100_000_000))
	ts.Require().NoError(err)
	_, err = ts.InitAndFundEVMAccount(ts.delegatorPriv, math.NewInt(100_000_000))
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
		valHex    common.Address
		valBech32 string
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

		// get validator
		vals, err := s.App.StakingKeeper.GetAllValidators(s.Ctx)
		Expect(err).To(BeNil())
		Expect(vals).NotTo(BeNil())
		valBech32 = vals[0].OperatorAddress
		valAddr, err := sdk.ValAddressFromBech32(valBech32)
		Expect(err).To(BeNil())
		valHex = common.BytesToAddress(valAddr.Bytes())

		// delegate 1 bbn from pre-defined delegator
		amount := big.NewInt(1_000_000) // 1 bbn
		resp, err := s.CallContract(
			s.delegatorPriv,
			s.addr,
			s.abi,
			epoching.WrappedDelegateMethod,
			common.Address(s.delegatorPriv.PubKey().Address().Bytes()),
			valHex,
			amount,
		)
		Expect(err).To(BeNil(), "error while calling the contract")
		Expect(resp.VmError).To(Equal(""))

		s.AdvanceToNextEpoch()

		res, err := s.QueryClientStaking.Delegation(s.Ctx, &stakingtypes.QueryDelegationRequest{
			DelegatorAddr: sdk.AccAddress(s.delegatorPriv.PubKey().Address().Bytes()).String(),
			ValidatorAddr: valBech32,
		})
		Expect(err).To(BeNil())
		Expect(res.DelegationResponse).NotTo(BeNil())
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
		var prevDelegation stakingtypes.Delegation

		BeforeEach(func() {
			// prevDelegation is the delegation that is available prior to the test
			res, err := s.QueryClientStaking.Delegation(s.Ctx, &stakingtypes.QueryDelegationRequest{
				DelegatorAddr: sdk.AccAddress(s.delegatorPriv.PubKey().Address().Bytes()).String(),
				ValidatorAddr: valBech32,
			})
			Expect(err).To(BeNil())
			Expect(res.DelegationResponse).NotTo(BeNil())

			prevDelegation = res.DelegationResponse.Delegation
		})

		Context("as the token owner", func() {
			It("should delegate", func() {
				delAddr := common.Address(s.delegatorPriv.PubKey().Address().Bytes())

				resp, err := s.CallContract(
					s.delegatorPriv,
					s.addr,
					s.abi,
					epoching.WrappedDelegateMethod,
					delAddr,
					valHex,
					big.NewInt(1_000_000),
				)
				Expect(err).To(BeNil(), "error while calling the contract")
				Expect(resp.VmError).To(Equal(""))

				s.AdvanceToNextEpoch()

				res, err := s.QueryClientStaking.Delegation(s.Ctx, &stakingtypes.QueryDelegationRequest{
					DelegatorAddr: sdk.AccAddress(s.delegatorPriv.PubKey().Address().Bytes()).String(),
					ValidatorAddr: valBech32,
				})
				Expect(err).To(BeNil())
				Expect(res.DelegationResponse).NotTo(BeNil())
				expShares := prevDelegation.GetShares().Add(math.LegacyNewDecWithPrec(1, 3))
				Expect(res.DelegationResponse.Delegation.GetShares().Equal(expShares)).To(BeTrue(), "expected delegation shares to be updated")
			})

			It("should not delegate if the account doesn't have sufficient balance", func() {
				newPriv, err := ethsecp256k1.GenerateKey()
				Expect(err).To(BeNil())
				_, err = s.InitAndFundEVMAccount(newPriv, math.NewInt(10_000))
				Expect(err).To(BeNil())

				_, err = s.CallContract(
					s.delegatorPriv,
					s.addr,
					s.abi,
					epoching.WrappedDelegateMethod,
					common.Address(newPriv.PubKey().Address().Bytes()),
					valHex,
					big.NewInt(1_000_000),
				)
				Expect(err).NotTo(BeNil(), "expected error while calling the contract")
			})

			It("should not delegate if the validator doesn't exist", func() {
				_, err := s.CallContract(
					s.delegatorPriv,
					s.addr,
					s.abi,
					epoching.WrappedDelegateMethod,
					common.Address(s.delegatorPriv.PubKey().Address().Bytes()),
					common.Address(s.validatorPriv.PubKey().Address().Bytes()),
					big.NewInt(1_000_000),
				)
				Expect(err).NotTo(BeNil(), "expected error while calling the contract")
			})
		})

		Context("on behalf of another account", func() {
			It("should not delegate if delegator address is not the msg.sender", func() {
				differentAddr := common.Address(s.validatorPriv.PubKey().Address().Bytes())
				_, err := s.CallContract(
					s.delegatorPriv,
					s.addr,
					s.abi,
					epoching.WrappedDelegateMethod,
					differentAddr,
					valHex,
					big.NewInt(1_000_000),
				)
				Expect(err).NotTo(BeNil(), "expected error while calling the contract")
			})
		})
	})

	Describe("to undelegate", func() {
		Context("as the token owner", func() {
			It("should undelegate", func() {
				delAddr := common.Address(s.delegatorPriv.PubKey().Address().Bytes())
				valAddr := sdk.ValAddress(valHex.Bytes())

				res, err := s.QueryClientStaking.ValidatorUnbondingDelegations(s.Ctx, &stakingtypes.QueryValidatorUnbondingDelegationsRequest{ValidatorAddr: valBech32})
				Expect(err).To(BeNil())
				Expect(res.UnbondingResponses).To(BeNil())
				Expect(res.UnbondingResponses).To(HaveLen(0), "expected no unbonding delegations before test")

				_, err = s.CallContract(
					s.delegatorPriv,
					s.addr,
					s.abi,
					epoching.WrappedUndelegateMethod,
					delAddr,
					valHex,
					big.NewInt(1_000_000),
				)
				Expect(err).To(BeNil(), "error while calling the contract")

				s.AdvanceToNextEpoch()

				delUbdRes, err := s.QueryClientStaking.ValidatorUnbondingDelegations(s.Ctx, &stakingtypes.QueryValidatorUnbondingDelegationsRequest{ValidatorAddr: valBech32})
				Expect(err).To(BeNil())
				Expect(delUbdRes.UnbondingResponses).To(HaveLen(1), "expected one delegation")
				Expect(delUbdRes.UnbondingResponses[0].ValidatorAddress).To(Equal(valAddr.String()), "expected validator address to be %s", valAddr)
			})

			It("should not undelegate if the amount exceeds the delegation", func() {
				delAddr := common.Address(s.delegatorPriv.PubKey().Address().Bytes())

				_, err := s.CallContract(
					s.delegatorPriv,
					s.addr,
					s.abi,
					epoching.WrappedUndelegateMethod,
					delAddr,
					valHex,
					big.NewInt(2_000_000), // 2 bbn
				)
				Expect(err).NotTo(BeNil(), "error while calling the contract")
			})

			It("should not undelegate if the validator doesn't exist", func() {
				delAddr := common.Address(s.delegatorPriv.PubKey().Address().Bytes())
				_, err := s.CallContract(
					s.delegatorPriv,
					s.addr,
					s.abi,
					epoching.WrappedUndelegateMethod,
					delAddr,
					common.Address(s.validatorPriv.PubKey().Address().Bytes()),
					big.NewInt(1_000_000),
				)
				Expect(err).NotTo(BeNil())
			})
		})

		Context("on behalf of another account", func() {
			It("should not undelegate if delegator address is not the msg.sender", func() {
				diffDelAddr := common.Address(s.validatorPriv.PubKey().Address().Bytes())
				_, err := s.CallContract(
					s.delegatorPriv,
					s.addr,
					s.abi,
					epoching.WrappedUndelegateMethod,
					diffDelAddr,
					valHex,
					big.NewInt(1_000_000),
				)
				Expect(err).NotTo(BeNil())
			})
		})
	})

	Describe("to redelegate", func() {
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
			newValHex                common.Address
			newValAddr               sdk.ValAddress
		)

		BeforeEach(func() {
			// create a new validator for a destination of the redelegation
			newValHex = common.Address(s.validatorPriv.PubKey().Address().Bytes())
			resp, err := s.CallContract(
				s.validatorPriv, s.addr, s.abi, epoching.WrappedCreateValidatorMethod,
				blsKey, defaultDescription, defaultCommission, defaultMinSelfDelegation, newValHex, consPkB64, defaultValue,
			)
			Expect(err).To(BeNil(), "error while calling the contract")
			Expect(resp.VmError).To(Equal(""))

			s.AdvanceToNextEpoch()

			newValAddr = sdk.ValAddress(newValHex.Bytes())
			v, err := s.App.StakingKeeper.GetValidator(s.Ctx, newValAddr)
			Expect(err).To(BeNil())
			Expect(newValAddr.String()).To(Equal(v.OperatorAddress))
			Expect(v.Tokens.IsPositive()).To(BeTrue())
		})

		Context("as the token owner", func() {
			It("should redelegate", func() {
				delAddr := common.Address(s.delegatorPriv.PubKey().Address().Bytes())

				// redelegate from valAddr -> newValAddr
				resp, err := s.CallContract(
					s.delegatorPriv, s.addr, s.abi, epoching.WrappedRedelegateMethod,
					delAddr, valHex, newValHex, big.NewInt(1_000_000),
				)
				Expect(err).To(BeNil(), "error while calling the contract")
				Expect(resp.VmError).To(Equal(""))

				s.AdvanceToNextEpoch()

				res, err := s.QueryClientStaking.Redelegations(s.Ctx, &stakingtypes.QueryRedelegationsRequest{
					DelegatorAddr:    sdk.AccAddress(delAddr.Bytes()).String(),
					SrcValidatorAddr: valBech32,
					DstValidatorAddr: sdk.ValAddress(newValAddr.Bytes()).String(),
				})
				Expect(err).To(BeNil())
				Expect(res.RedelegationResponses).To(HaveLen(1), "expected one redelegation to be found")
				Expect(res.RedelegationResponses[0].Redelegation.DelegatorAddress).To(Equal(sdk.AccAddress(delAddr.Bytes()).String()), "expected delegator address to be %s", sdk.AccAddress(delAddr.Bytes()).String())
				Expect(res.RedelegationResponses[0].Redelegation.ValidatorSrcAddress).To(Equal(valBech32), "expected source validator address to be %s", valBech32)
				Expect(res.RedelegationResponses[0].Redelegation.ValidatorDstAddress).To(Equal(sdk.ValAddress(newValAddr.Bytes()).String()), "expected destination validator address to be %s", sdk.ValAddress(newValAddr.Bytes()).String())
			})

			It("should not redelegate if the amount exceeds the delegation", func() {
				delAddr := common.Address(s.delegatorPriv.PubKey().Address().Bytes())
				_, err := s.CallContract(
					s.delegatorPriv, s.addr, s.abi, epoching.WrappedRedelegateMethod,
					delAddr, valHex, newValHex, big.NewInt(2_000_000),
				)
				Expect(err).NotTo(BeNil())
			})
		})

		Context("on behalf of another account", func() {
			It("should not redelegate if delegator address is not the msg.sender", func() {
				diffDelAddr := common.Address(s.validatorPriv.PubKey().Address().Bytes())
				_, err := s.CallContract(
					s.delegatorPriv, s.addr, s.abi, epoching.WrappedRedelegateMethod,
					diffDelAddr, valHex, newValHex, big.NewInt(1_000_000),
				)
				Expect(err).NotTo(BeNil())
			})
		})
	})

	Describe("to cancel an unbonding delegation", func() {
		var creationHeight int64

		BeforeEach(func() {
			// set up an unbonding delegation
			delAddr := common.Address(s.delegatorPriv.PubKey().Address().Bytes())
			valAddr := sdk.ValAddress(valHex.Bytes())
			_, err := s.CallContract(
				s.delegatorPriv,
				s.addr,
				s.abi,
				epoching.WrappedUndelegateMethod,
				delAddr,
				valHex,
				big.NewInt(1_000_000),
			)
			Expect(err).To(BeNil(), "error while calling the contract")

			s.AdvanceToNextEpoch()
			// undelegate is executed at current block - 1 since every message that is queued
			// is executed at the last block of the current epoch
			creationHeight = s.Ctx.BlockHeight() - 1

			delUbdRes, err := s.QueryClientStaking.ValidatorUnbondingDelegations(s.Ctx, &stakingtypes.QueryValidatorUnbondingDelegationsRequest{ValidatorAddr: valBech32})
			Expect(err).To(BeNil())
			Expect(delUbdRes.UnbondingResponses).To(HaveLen(1), "expected one delegation")
			Expect(delUbdRes.UnbondingResponses[0].DelegatorAddress).To(Equal(sdk.AccAddress(delAddr.Bytes()).String()), "expected delegator address to be %s", sdk.AccAddress(delAddr.Bytes()).String())
			Expect(delUbdRes.UnbondingResponses[0].ValidatorAddress).To(Equal(valAddr.String()), "expected validator address to be %s", valAddr)
			Expect(delUbdRes.UnbondingResponses[0].Entries[0].CreationHeight).To(Equal(creationHeight), "expected different creation height")
			Expect(delUbdRes.UnbondingResponses[0].Entries).To(HaveLen(1), "expected one unbonding delegation entry to be found")
			Expect(delUbdRes.UnbondingResponses[0].Entries[0].Balance).To(Equal(math.NewInt(1_000_000)), "expected different balance")
		})

		Context("as the token owner", func() {
			It("should cancel the unbonding delegation", func() {
				delAddr := common.Address(s.delegatorPriv.PubKey().Address().Bytes())

				valDelRes, err := s.QueryClientStaking.ValidatorDelegations(s.Ctx, &stakingtypes.QueryValidatorDelegationsRequest{
					ValidatorAddr: valBech32,
				})
				Expect(err).To(BeNil())
				Expect(valDelRes.DelegationResponses).To(HaveLen(1), "only one self delegation should be found")

				resp, err := s.CallContract(
					s.delegatorPriv,
					s.addr,
					s.abi,
					epoching.WrappedCancelUnbondingDelegationMethod,
					delAddr,
					valHex,
					big.NewInt(1_000_000),
					big.NewInt(creationHeight),
				)
				Expect(err).To(BeNil(), "error while calling the contract")
				Expect(resp.VmError).To(Equal(""))

				s.AdvanceToNextEpoch()

				res, err := s.QueryClientStaking.DelegatorUnbondingDelegations(s.Ctx, &stakingtypes.QueryDelegatorUnbondingDelegationsRequest{
					DelegatorAddr: sdk.AccAddress(delAddr.Bytes()).String(),
				})
				Expect(err).To(BeNil())
				Expect(res.UnbondingResponses).To(HaveLen(0), "expected unbonding delegation to be canceled")

				valDelRes, err = s.QueryClientStaking.ValidatorDelegations(s.Ctx, &stakingtypes.QueryValidatorDelegationsRequest{
					ValidatorAddr: valBech32,
				})
				Expect(err).To(BeNil())
				Expect(valDelRes.DelegationResponses).To(HaveLen(2), "expect two delegation to be found")
			})

			It("should not cancel an unbonding delegation if the amount is not correct", func() {
				delAddr := common.Address(s.delegatorPriv.PubKey().Address().Bytes())
				_, err := s.CallContract(
					s.delegatorPriv,
					s.addr,
					s.abi,
					epoching.WrappedCancelUnbondingDelegationMethod,
					delAddr,
					valHex,
					big.NewInt(2_000_000),
					big.NewInt(creationHeight),
				)
				// unlike CancelUnbondingDelegation from staking msgServer, WrappedCancelUnbondingDelegation
				// successfully enqueue msgs even if the amount is incorrect. but this will be
				// failed after epoch boundary
				Expect(err).To(BeNil(), "error while calling the contract")

				s.AdvanceToNextEpoch()

				res, err := s.QueryClientStaking.DelegatorUnbondingDelegations(s.Ctx, &stakingtypes.QueryDelegatorUnbondingDelegationsRequest{
					DelegatorAddr: sdk.AccAddress(delAddr.Bytes()).String(),
				})
				Expect(err).To(BeNil())
				Expect(res.UnbondingResponses).To(HaveLen(1), "expected unbonding delegation not to have be canceled")
			})

			It("should not cancel an unbonding delegation if the creation height is not correct", func() {
				delAddr := common.Address(s.delegatorPriv.PubKey().Address().Bytes())
				_, err := s.CallContract(
					s.delegatorPriv,
					s.addr,
					s.abi,
					epoching.WrappedCancelUnbondingDelegationMethod,
					delAddr,
					valHex,
					big.NewInt(1_000_000),
				)
				Expect(err).NotTo(BeNil(), "error while calling the contract")

				s.AdvanceToNextEpoch()

				res, err := s.QueryClientStaking.DelegatorUnbondingDelegations(s.Ctx, &stakingtypes.QueryDelegatorUnbondingDelegationsRequest{
					DelegatorAddr: sdk.AccAddress(delAddr.Bytes()).String(),
				})
				Expect(err).To(BeNil())
				Expect(res.UnbondingResponses).To(HaveLen(1), "expected unbonding delegation not to have be canceled")
			})
		})
	})
})
