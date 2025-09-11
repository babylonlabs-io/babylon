package epoching_test

import (
	"encoding/base64"
	"math/big"
	"slices"
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
	"github.com/cosmos/cosmos-sdk/types/query"
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
				newValHex := common.Address(s.validatorPriv.PubKey().Address().Bytes())
				resp, err := s.CallContract(
					s.validatorPriv, s.addr, s.abi, epoching.WrappedCreateValidatorMethod,
					blsKey, defaultDescription, defaultCommission, defaultMinSelfDelegation, newValHex, consPkB64, defaultValue,
				)
				Expect(err).To(BeNil(), "error while calling the contract")
				Expect(resp.VmError).To(Equal(""))

				s.AdvanceToNextEpoch()

				newValAddr := sdk.ValAddress(newValHex.Bytes())
				v, err := s.App.StakingKeeper.GetValidator(s.Ctx, newValAddr)
				Expect(err).To(BeNil())
				Expect(newValAddr.String()).To(Equal(v.OperatorAddress))
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

	Describe("Validator queries", func() {
		It("should return validator", func() {
			resp, err := s.QueryContract(
				s.addr,
				s.abi,
				epoching.ValidatorMethod,
				valHex,
			)
			Expect(err).To(BeNil(), "error while calling the contract %v", err)
			Expect(resp.VmError).To(Equal(""))

			var valOut epoching.ValidatorOutput
			err = s.abi.UnpackIntoInterface(&valOut, epoching.ValidatorMethod, resp.Ret)
			Expect(err).To(BeNil(), "error while unpacking the validator output: %v", err)
			Expect(valOut.Validator.OperatorAddress.String()).To(Equal(valHex.String()), "expected validator address to match")
			Expect(valOut.Validator.DelegatorShares).To(Equal(big.NewInt(1001000000000000000)), "expected different delegator shares")
		})

		It("should return an empty validator if the validator is not found", func() {
			newValHex := common.Address(s.validatorPriv.PubKey().Address().Bytes())
			resp, err := s.QueryContract(
				s.addr,
				s.abi,
				epoching.ValidatorMethod,
				newValHex,
			)
			Expect(err).To(BeNil(), "error while calling the contract %v", err)
			Expect(resp.VmError).To(Equal(""))

			var valOut epoching.ValidatorOutput
			err = s.abi.UnpackIntoInterface(&valOut, epoching.ValidatorMethod, resp.Ret)
			Expect(err).To(BeNil(), "error while unpacking the validator output: %v", err)
			Expect(valOut.Validator.OperatorAddress.String()).To(Equal("0x0000000000000000000000000000000000000000"), "expected validator address to be 0x0000000000000000000000000000000000000000")
			Expect(valOut.Validator.Status).To(BeZero(), "expected unspecified bonding status")
		})
	})

	Describe("Validators queries", func() {
		It("should return validators (default pagination", func() {
			resp, err := s.QueryContract(
				s.addr,
				s.abi,
				epoching.ValidatorsMethod,
				stakingtypes.Bonded.String(),
				query.PageRequest{},
			)
			Expect(err).To(BeNil(), "error while calling the contract %v", err)
			Expect(resp.VmError).To(Equal(""))

			vals, err := s.App.StakingKeeper.GetAllValidators(s.Ctx)
			Expect(err).To(BeNil())
			Expect(vals).NotTo(BeNil())

			var valOut epoching.ValidatorsOutput
			err = s.abi.UnpackIntoInterface(&valOut, epoching.ValidatorsMethod, resp.Ret)
			Expect(err).To(BeNil())

			Expect(valOut.PageResponse.NextKey).To(BeEmpty())
			Expect(valOut.PageResponse.Total).To(Equal(uint64(len(vals))))

			Expect(valOut.Validators).To(HaveLen(len(vals)))
			for _, val := range valOut.Validators {
				validatorAddrs := make([]string, len(vals))
				for i, v := range vals {
					validatorAddrs[i] = v.OperatorAddress
				}
				operatorAddress := sdk.ValAddress(val.OperatorAddress.Bytes()).String()

				Expect(slices.Contains(validatorAddrs, operatorAddress)).To(BeTrue(), "operator address not found in test suite validators")
				Expect(val.DelegatorShares).To(Equal(big.NewInt(1001000000000000000)), "expected different delegator shares")
			}
		})

		It("should return validators with pagination limit = 1", func() {
			const limit = 1
			resp, err := s.QueryContract(
				s.addr,
				s.abi,
				epoching.ValidatorsMethod,
				stakingtypes.Bonded.String(),
				query.PageRequest{Limit: limit, CountTotal: true},
			)
			Expect(err).To(BeNil(), "error while calling the contract %v", err)
			Expect(resp.VmError).To(Equal(""))

			var valOut epoching.ValidatorsOutput
			err = s.abi.UnpackIntoInterface(&valOut, epoching.ValidatorsMethod, resp.Ret)
			Expect(err).To(BeNil())

			// next_key is empty since there are no more data rather than one validators output
			Expect(valOut.PageResponse.NextKey).To(BeEmpty())
			Expect(valOut.PageResponse.Total).To(Equal(uint64(len(valOut.Validators))))

			Expect(valOut.Validators).To(HaveLen(limit))
		})

		It("should return an error if the bonding type is not known", func() {
			_, err := s.QueryContract(
				s.addr,
				s.abi,
				epoching.ValidatorsMethod,
				"15", // invalid bonding type
				query.PageRequest{},
			)
			Expect(err).NotTo(BeNil(), "error while calling the contract %v", err)
		})

		It("should return an empty array if there are no validators with the given bonding type", func() {
			resp, err := s.QueryContract(
				s.addr,
				s.abi,
				epoching.ValidatorsMethod,
				stakingtypes.Unbonded.String(),
				query.PageRequest{},
			)
			Expect(err).To(BeNil(), "error while calling the contract %v", err)
			Expect(resp.VmError).To(Equal(""))

			var valOut epoching.ValidatorsOutput
			err = s.abi.UnpackIntoInterface(&valOut, epoching.ValidatorsMethod, resp.Ret)
			Expect(err).To(BeNil())

			Expect(valOut.PageResponse.NextKey).To(BeEmpty())
			Expect(valOut.PageResponse.Total).To(Equal(uint64(0)))
			Expect(valOut.Validators).To(HaveLen(0), "expected no validators to be returned")
		})
	})

	Describe("Delegation queries", func() {
		It("should return a delegation if it is found", func() {
			delAddr := common.Address(s.delegatorPriv.PubKey().Address().Bytes())

			resp, err := s.QueryContract(
				s.addr,
				s.abi,
				epoching.DelegationMethod,
				delAddr,
				valHex,
			)
			Expect(err).To(BeNil(), "error while calling the contract %v", err)
			Expect(resp.VmError).To(Equal(""))

			var delOut epoching.DelegationOutput
			err = s.abi.UnpackIntoInterface(&delOut, epoching.DelegationMethod, resp.Ret)
			Expect(err).To(BeNil(), "error while unpacking the delegation output: %v", err)
			Expect(delOut.Shares.Cmp(big.NewInt(0))).To(BeNumerically(">", 0), "expected delegation shares to be greater than 0")
			Expect(delOut.Balance.Amount.Cmp(big.NewInt(0))).To(BeNumerically(">", 0), "expected delegation balance to be greater than 0")
		})

		It("should return an empty delegation if it is not found", func() {
			newDelAddr := common.Address(s.validatorPriv.PubKey().Address().Bytes())

			resp, err := s.QueryContract(
				s.addr,
				s.abi,
				epoching.DelegationMethod,
				newDelAddr,
				valHex,
			)
			Expect(err).To(BeNil(), "error while calling the contract %v", err)
			Expect(resp.VmError).To(Equal(""))

			var delOut epoching.DelegationOutput
			err = s.abi.UnpackIntoInterface(&delOut, epoching.DelegationMethod, resp.Ret)
			Expect(err).To(BeNil(), "error while unpacking the delegation output: %v", err)
			Expect(delOut.Shares.Int64()).To(BeZero(), "expected no delegation shares")
			Expect(delOut.Balance.Amount.Int64()).To(BeZero(), "expected zero delegation balance")
		})
	})

	Describe("UnbondingDelegation queries", func() {
		var creationHeight int64

		BeforeEach(func() {
			// Create an unbonding delegation first
			delAddr := common.Address(s.delegatorPriv.PubKey().Address().Bytes())
			_, err := s.CallContract(
				s.delegatorPriv,
				s.addr,
				s.abi,
				epoching.WrappedUndelegateMethod,
				delAddr,
				valHex,
				big.NewInt(1_000_000), // 1 bbn
			)
			Expect(err).To(BeNil(), "error while calling the contract")

			s.AdvanceToNextEpoch()
			creationHeight = s.Ctx.BlockHeight() - 1

			// Verify unbonding delegation was created
			res, err := s.QueryClientStaking.ValidatorUnbondingDelegations(s.Ctx, &stakingtypes.QueryValidatorUnbondingDelegationsRequest{ValidatorAddr: valBech32})
			Expect(err).To(BeNil())
			Expect(res.UnbondingResponses).To(HaveLen(1), "expected one unbonding delegation")
		})

		It("should return an unbonding delegation if it is found", func() {
			delAddr := common.Address(s.delegatorPriv.PubKey().Address().Bytes())

			resp, err := s.QueryContract(
				s.addr,
				s.abi,
				epoching.UnbondingDelegationMethod,
				delAddr,
				valHex,
			)
			Expect(err).To(BeNil(), "error while calling the contract %v", err)
			Expect(resp.VmError).To(Equal(""))

			var ubdOut epoching.UnbondingDelegationOutput
			err = s.abi.UnpackIntoInterface(&ubdOut, epoching.UnbondingDelegationMethod, resp.Ret)
			Expect(err).To(BeNil(), "error while unpacking the unbonding delegation output: %v", err)
			Expect(ubdOut.UnbondingDelegation.Entries).To(HaveLen(1), "expected one unbonding delegation entry")
			Expect(ubdOut.UnbondingDelegation.Entries[0].Balance).To(Equal(big.NewInt(1_000_000)), "expected different balance")
			Expect(ubdOut.UnbondingDelegation.Entries[0].CreationHeight).To(Equal(creationHeight), "expected different creation height")
		})

		It("should return an empty slice if the unbonding delegation is not found", func() {
			newDelAddr := common.Address(s.validatorPriv.PubKey().Address().Bytes())

			resp, err := s.QueryContract(
				s.addr,
				s.abi,
				epoching.UnbondingDelegationMethod,
				newDelAddr,
				valHex,
			)
			Expect(err).To(BeNil(), "error while calling the contract %v", err)
			Expect(resp.VmError).To(Equal(""))

			var ubdOut epoching.UnbondingDelegationOutput
			err = s.abi.UnpackIntoInterface(&ubdOut, epoching.UnbondingDelegationMethod, resp.Ret)
			Expect(err).To(BeNil(), "error while unpacking the unbonding delegation output: %v", err)
			Expect(ubdOut.UnbondingDelegation.Entries).To(HaveLen(0), "expected no unbonding delegation entries")
		})
	})

	Describe("Redelegation queries", func() {
		var newValHex common.Address

		BeforeEach(func() {
			// Create a second validator for redelegation
			newValHex = common.Address(s.validatorPriv.PubKey().Address().Bytes())
			description := epoching.Description{
				Moniker:         "second validator",
				Identity:        "",
				Website:         "",
				SecurityContact: "",
				Details:         "",
			}
			commission := epoching.Commission{
				Rate:          big.NewInt(100000000000000000),
				MaxRate:       big.NewInt(100000000000000000),
				MaxChangeRate: big.NewInt(100000000000000000),
			}
			minSelfDelegation := big.NewInt(1)
			value := big.NewInt(1_000_000) // 1bbn

			resp, err := s.CallContract(
				s.validatorPriv, s.addr, s.abi, epoching.WrappedCreateValidatorMethod,
				blsKey, description, commission, minSelfDelegation, newValHex, consPkB64, value,
			)
			Expect(err).To(BeNil(), "error while calling the contract")
			Expect(resp.VmError).To(Equal(""))

			s.AdvanceToNextEpoch()

			// Create a redelegation
			delAddr := common.Address(s.delegatorPriv.PubKey().Address().Bytes())
			_, err = s.CallContract(
				s.delegatorPriv, s.addr, s.abi, epoching.WrappedRedelegateMethod,
				delAddr, valHex, newValHex, big.NewInt(500_000), // 0.5 bbn
			)
			Expect(err).To(BeNil(), "error while calling the contract")

			s.AdvanceToNextEpoch()
		})

		It("should return the redelegation if it exists", func() {
			delAddr := common.Address(s.delegatorPriv.PubKey().Address().Bytes())

			resp, err := s.QueryContract(
				s.addr,
				s.abi,
				epoching.RedelegationMethod,
				delAddr,
				valHex,
				newValHex,
			)
			Expect(err).To(BeNil(), "error while calling the contract %v", err)
			Expect(resp.VmError).To(Equal(""))

			var redelOut epoching.RedelegationOutput
			err = s.abi.UnpackIntoInterface(&redelOut, epoching.RedelegationMethod, resp.Ret)
			Expect(err).To(BeNil(), "error while unpacking the redelegation output: %v", err)
			Expect(redelOut.Redelegation.Entries).To(HaveLen(1), "expected one redelegation entry")
			Expect(redelOut.Redelegation.Entries[0].InitialBalance).To(Equal(big.NewInt(500_000)), "expected different initial balance")
			Expect(redelOut.Redelegation.DelegatorAddress).To(Equal(delAddr), "expected different delegator address")
			Expect(redelOut.Redelegation.ValidatorSrcAddress).To(Equal(valHex), "expected different source validator address")
			Expect(redelOut.Redelegation.ValidatorDstAddress).To(Equal(newValHex), "expected different destination validator address")
		})

		It("should return an empty output if the redelegation is not found", func() {
			// Use a different delegator that hasn't redelegated
			newDelAddr := common.Address(s.validatorPriv.PubKey().Address().Bytes())

			resp, err := s.QueryContract(
				s.addr,
				s.abi,
				epoching.RedelegationMethod,
				newDelAddr,
				valHex,
				newValHex,
			)
			Expect(err).To(BeNil(), "error while calling the contract %v", err)
			Expect(resp.VmError).To(Equal(""))

			var redelOut epoching.RedelegationOutput
			err = s.abi.UnpackIntoInterface(&redelOut, epoching.RedelegationMethod, resp.Ret)
			Expect(err).To(BeNil(), "error while unpacking the redelegation output: %v", err)
			Expect(redelOut.Redelegation.Entries).To(HaveLen(0), "expected no redelegation entries")
		})
	})

	Describe("Redelegations queries", func() {
		var newValHex common.Address

		BeforeEach(func() {
			// Create a second validator for redelegation
			newValHex = common.Address(s.validatorPriv.PubKey().Address().Bytes())
			description := epoching.Description{
				Moniker:         "second validator",
				Identity:        "",
				Website:         "",
				SecurityContact: "",
				Details:         "",
			}
			commission := epoching.Commission{
				Rate:          big.NewInt(100000000000000000),
				MaxRate:       big.NewInt(100000000000000000),
				MaxChangeRate: big.NewInt(100000000000000000),
			}
			minSelfDelegation := big.NewInt(1)
			value := big.NewInt(1_000_000) // 1bbn

			resp, err := s.CallContract(
				s.validatorPriv, s.addr, s.abi, epoching.WrappedCreateValidatorMethod,
				blsKey, description, commission, minSelfDelegation, newValHex, consPkB64, value,
			)
			Expect(err).To(BeNil(), "error while calling the contract")
			Expect(resp.VmError).To(Equal(""))

			s.AdvanceToNextEpoch()

			// Create redelegations
			delAddr := common.Address(s.delegatorPriv.PubKey().Address().Bytes())
			_, err = s.CallContract(
				s.delegatorPriv, s.addr, s.abi, epoching.WrappedRedelegateMethod,
				delAddr, valHex, newValHex, big.NewInt(500_000), // 0.5 bbn
			)
			Expect(err).To(BeNil(), "error while calling the contract")

			s.AdvanceToNextEpoch()
		})

		It("should return all redelegations for delegator (default pagination)", func() {
			delAddr := common.Address(s.delegatorPriv.PubKey().Address().Bytes())

			resp, err := s.QueryContract(
				s.addr,
				s.abi,
				epoching.RedelegationsMethod,
				delAddr,
				common.Address{},
				common.Address{},
				query.PageRequest{},
			)
			Expect(err).To(BeNil(), "error while calling the contract %v", err)
			Expect(resp.VmError).To(Equal(""))

			var redelsOut epoching.RedelegationsOutput
			err = s.abi.UnpackIntoInterface(&redelsOut, epoching.RedelegationsMethod, resp.Ret)
			Expect(err).To(BeNil(), "error while unpacking the redelegations output: %v", err)
			Expect(redelsOut.PageResponse.NextKey).To(BeEmpty())
			Expect(redelsOut.Response).To(HaveLen(1), "expected one redelegation to be returned")
			Expect(redelsOut.Response[0].Entries).To(HaveLen(1), "expected one redelegation entry")
		})

		It("should return empty array if no redelegations found", func() {
			// Use a different delegator that hasn't redelegated
			newDelAddr := common.Address(s.validatorPriv.PubKey().Address().Bytes())

			resp, err := s.QueryContract(
				s.addr,
				s.abi,
				epoching.RedelegationsMethod,
				newDelAddr,
				common.Address{},
				common.Address{},
				query.PageRequest{},
			)
			Expect(err).To(BeNil(), "error while calling the contract %v", err)
			Expect(resp.VmError).To(Equal(""))

			var redelsOut epoching.RedelegationsOutput
			err = s.abi.UnpackIntoInterface(&redelsOut, epoching.RedelegationsMethod, resp.Ret)
			Expect(err).To(BeNil(), "error while unpacking the redelegations output: %v", err)
			Expect(redelsOut.Response).To(HaveLen(0), "expected no redelegations to be returned")
		})
	})

	Describe("epochInfo queries", func() {
		It("should return the epoch info", func() {
			resp, err := s.QueryContract(
				s.addr,
				s.abi,
				epoching.EpochInfoMethod,
				uint64(1),
			)
			Expect(err).To(BeNil(), "error while calling the contract %v", err)
			Expect(resp.VmError).To(Equal(""))

			var epochInfo epoching.EpochInfoOutput
			err = s.abi.UnpackIntoInterface(&epochInfo, epoching.EpochInfoMethod, resp.Ret)
			Expect(err).To(BeNil(), "error while unpacking the epoch info output: %v", err)
			Expect(epochInfo.Epoch.EpochNumber).To(Equal(uint64(1)))
		})
	})

	Describe("currentEpoch queries", func() {
		It("should return the current epoch", func() {
			// NOTE: BeginBlocker is called only when Commit block, so by quering at block height 11,
			// this will result in non-incremented epoch which is 1, to get epoch 2, we should query
			// at block height 12
			s.Commit(nil)
			resp, err := s.QueryContract(
				s.addr,
				s.abi,
				epoching.CurrentEpochMethod,
			)
			Expect(err).To(BeNil(), "error while calling the contract %v", err)
			Expect(resp.VmError).To(Equal(""))

			var currentEpoch epoching.CurrentEpochOutput
			err = s.abi.UnpackIntoInterface(&currentEpoch, epoching.CurrentEpochMethod, resp.Ret)
			Expect(err).To(BeNil(), "error while unpacking the current epoch output: %v", err)
			Expect(currentEpoch.Response.CurrentEpoch).To(Equal(uint64(2)))
			Expect(currentEpoch.Response.EpochBoundary).To(Equal(uint64(20)))
		})
	})

	Describe("epochMsgs queries", func() {
		BeforeEach(func() {
			// make MsgWrappedDelegate and remain this msg in the queue
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
		})

		It("should return the epoch messages", func() {
			resp, err := s.QueryContract(
				s.addr,
				s.abi,
				epoching.EpochMsgsMethod,
				uint64(2),
				query.PageRequest{CountTotal: true},
			)
			Expect(err).To(BeNil(), "error while calling the contract %v", err)
			Expect(resp.VmError).To(Equal(""))

			var epochMsgs epoching.EpochMsgsOutput
			err = s.abi.UnpackIntoInterface(&epochMsgs, epoching.EpochMsgsMethod, resp.Ret)
			Expect(err).To(BeNil(), "error while unpacking the epoch msgs: %v", err)
			Expect(epochMsgs.PageResponse.Total).To(Equal(uint64(1)))
			Expect(epochMsgs.QueuedMsgs[0].BlockHeight).To(Equal(uint64(11)))
		})

		It("should return empty array if no epoch messages found", func() {
			s.AdvanceToNextEpoch()
			s.Commit(nil)

			resp, err := s.QueryContract(
				s.addr,
				s.abi,
				epoching.EpochMsgsMethod,
				uint64(3),
				query.PageRequest{CountTotal: true},
			)
			Expect(err).To(BeNil(), "error while calling the contract %v", err)
			Expect(resp.VmError).To(Equal(""))

			var epochMsgs epoching.EpochMsgsOutput
			err = s.abi.UnpackIntoInterface(&epochMsgs, epoching.EpochMsgsMethod, resp.Ret)
			Expect(err).To(BeNil(), "error while unpacking the epoch msgs: %v", err)
			Expect(epochMsgs.PageResponse.Total).To(Equal(uint64(0)))
			Expect(epochMsgs.QueuedMsgs).To(HaveLen(0))
		})
	})

	Describe("latestEpochMsgs queries", func() {
		BeforeEach(func() {
			// make two MsgWrappedDelegate and remain these msgs in the queue
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

			resp, err = s.CallContract(
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
		})

		It("should return the latest epoch messages", func() {
			resp, err := s.QueryContract(
				s.addr,
				s.abi,
				epoching.LatestEpochMsgsMethod,
				uint64(0),
				uint64(2),
				query.PageRequest{CountTotal: true},
			)
			Expect(err).To(BeNil(), "error while calling the contract %v", err)
			Expect(resp.VmError).To(Equal(""))

			var latestEpochMsgs epoching.LatestEpochMsgsOutput
			err = s.abi.UnpackIntoInterface(&latestEpochMsgs, epoching.LatestEpochMsgsMethod, resp.Ret)
			Expect(err).To(BeNil(), "error while unpacking the latest epoch msgs: %v", err)
			Expect(latestEpochMsgs.PageResponse.Total).To(Equal(uint64(2)))
			// two msgs in the epoch number two
			Expect(latestEpochMsgs.LatestEpochMsgs[1].Msgs).To(HaveLen(2))
		})
	})

	Describe("validatorLifecycle queries", func() {
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

		BeforeEach(func() {
			// for querying validator life cycle, create new validator
			newValHex := common.Address(s.validatorPriv.PubKey().Address().Bytes())
			resp, err := s.CallContract(
				s.validatorPriv, s.addr, s.abi, epoching.WrappedCreateValidatorMethod,
				blsKey, defaultDescription, defaultCommission, defaultMinSelfDelegation, newValHex, consPkB64, defaultValue,
			)
			Expect(err).To(BeNil(), "error while calling the contract")
			Expect(resp.VmError).To(Equal(""))

			s.AdvanceToNextEpoch()

			newValAddr := sdk.ValAddress(newValHex.Bytes())
			v, err := s.App.StakingKeeper.GetValidator(s.Ctx, newValAddr)
			Expect(err).To(BeNil())
			Expect(newValAddr.String()).To(Equal(v.OperatorAddress))
			Expect(v.Tokens.IsPositive()).To(BeTrue())
		})

		It("should return the validator lifecycle", func() {
			newValHex := common.Address(s.validatorPriv.PubKey().Address().Bytes())
			resp, err := s.QueryContract(
				s.addr,
				s.abi,
				epoching.ValidatorLifecycleMethod,
				newValHex,
			)
			Expect(err).To(BeNil(), "error while calling the contract %v", err)
			Expect(resp.VmError).To(Equal(""))

			var validatorLifecycle epoching.ValidatorLifecycleOutput
			err = s.abi.UnpackIntoInterface(&validatorLifecycle, epoching.ValidatorLifecycleMethod, resp.Ret)
			Expect(err).To(BeNil(), "error while unpacking the validator lifecycle: %v", err)
			Expect(validatorLifecycle.ValidatorLife[0].BlockHeight).To(Equal(uint64(20)))
			Expect(validatorLifecycle.ValidatorLife[0].StateDesc).To(Equal("CREATED"))
			Expect(validatorLifecycle.ValidatorLife[1].StateDesc).To(Equal("BONDED"))
		})
	})

	Describe("delegationLifecycle queries", func() {
		It("should return the delegation lifecycle", func() {
			delAddr := common.Address(s.delegatorPriv.PubKey().Address().Bytes())
			resp, err := s.QueryContract(
				s.addr,
				s.abi,
				epoching.DelegationLifecycleMethod,
				delAddr,
			)
			Expect(err).To(BeNil(), "error while calling the contract %v", err)
			Expect(resp.VmError).To(Equal(""))

			var delegationLifecycle epoching.DelegationLifecycleOutput
			err = s.abi.UnpackIntoInterface(&delegationLifecycle, epoching.DelegationLifecycleMethod, resp.Ret)
			Expect(err).To(BeNil(), "error while unpacking the delegation lifecycle: %v", err)
			Expect(delegationLifecycle.DelegationLifecycle.DelAddr.String()).To(Equal(delAddr.String()))
			Expect(delegationLifecycle.DelegationLifecycle.DelLife[0].BlockHeight).To(Equal(uint64(10)))
			Expect(delegationLifecycle.DelegationLifecycle.DelLife[0].State).To(Equal(uint8(0)))
			Expect(delegationLifecycle.DelegationLifecycle.DelLife[1].State).To(Equal(uint8(1)))
		})
	})

	Describe("epochValSet queries", func() {
		It("should return the epoch validator set", func() {
			resp, err := s.QueryContract(
				s.addr,
				s.abi,
				epoching.EpochValSetMethod,
				uint64(1),
				query.PageRequest{CountTotal: true},
			)
			Expect(err).To(BeNil(), "error while calling the contract %v", err)
			Expect(resp.VmError).To(Equal(""))

			var epochValSet epoching.EpochValSetOutput
			err = s.abi.UnpackIntoInterface(&epochValSet, epoching.EpochValSetMethod, resp.Ret)
			Expect(err).To(BeNil(), "error while unpacking the epoch val set: %v", err)
			Expect(epochValSet.Validators[0].Addr).To(Equal(valHex))
		})
	})
})
