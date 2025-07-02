package cli

import (
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	sdkmath "cosmossdk.io/math"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/tx"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	asig "github.com/babylonlabs-io/babylon/v3/crypto/schnorr-adaptor-signature"
	bbn "github.com/babylonlabs-io/babylon/v3/types"
	"github.com/babylonlabs-io/babylon/v3/x/btcstaking/types"
)

const (
	FlagConsumerId      = "consumer-id"
	FlagMoniker         = "moniker"
	FlagIdentity        = "identity"
	FlagWebsite         = "website"
	FlagSecurityContact = "security-contact"
	FlagDetails         = "details"

	FlagCommissionRate          = "commission-rate"
	FlagCommissionMaxRate       = "commission-max-rate"
	FlagCommissionMaxChangeRate = "commission-max-change-rate"
)

// GetTxCmd returns the transaction commands for this module
func GetTxCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      fmt.Sprintf("%s transactions subcommands", types.ModuleName),
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	cmd.AddCommand(
		NewCreateFinalityProviderCmd(),
		NewEditFinalityProviderCmd(),
		NewCreateBTCDelegationCmd(),
		NewAddCovenantSigsCmd(),
		NewBTCUndelegateCmd(),
		NewSelectiveSlashingEvidenceCmd(),
		NewAddBTCDelegationInclusionProofCmd(),
		NewBTCStakeExpandCmd(),
	)

	return cmd
}

func NewCreateFinalityProviderCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create-finality-provider [btc_pk] [pop]",
		Args:  cobra.ExactArgs(2),
		Short: "Create a finality provider",
		Long: strings.TrimSpace(
			`Creates a finality provider for Babylon or a Consumer chain.`, // TODO: example
		),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			fs := cmd.Flags()

			// get description
			consumerID, _ := fs.GetString(FlagConsumerId)
			moniker, _ := fs.GetString(FlagMoniker)
			identity, _ := fs.GetString(FlagIdentity)
			website, _ := fs.GetString(FlagWebsite)
			security, _ := fs.GetString(FlagSecurityContact)
			details, _ := fs.GetString(FlagDetails)
			description := stakingtypes.NewDescription(
				moniker,
				identity,
				website,
				security,
				details,
			)
			// get commission rate information
			commission, err := getCommissionRates(fs)
			if err != nil {
				return err
			}

			// get BTC PK
			btcPK, err := bbn.NewBIP340PubKeyFromHex(args[0])
			if err != nil {
				return err
			}

			// get PoP
			pop, err := types.NewPoPBTCFromHex(args[1])
			if err != nil {
				return err
			}

			msg := types.MsgCreateFinalityProvider{
				Addr:        clientCtx.FromAddress.String(),
				Description: &description,
				Commission:  commission,
				BtcPk:       btcPK,
				Pop:         pop,
				ConsumerId:  consumerID,
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), &msg)
		},
	}

	fs := cmd.Flags()
	fs.String(FlagConsumerId, "", "The finality provider's consumer ID, if any")
	fs.String(FlagMoniker, "", "The finality provider's (optional) moniker")
	fs.String(FlagWebsite, "", "The finality provider's (optional) website")
	fs.String(FlagSecurityContact, "", "The finality provider's (optional) security contact email")
	fs.String(FlagDetails, "", "The finality provider's (optional) details")
	fs.String(FlagIdentity, "", "The (optional) identity signature (ex. UPort or Keybase)")
	// commission-related flags
	fs.String(FlagCommissionRate, "0", "The initial commission rate percentage")
	fs.String(FlagCommissionMaxRate, "", "The maximum commission rate percentage")
	fs.String(FlagCommissionMaxChangeRate, "", "The maximum commission change rate percentage (per day)")

	flags.AddTxFlagsToCmd(cmd)

	return cmd
}

func NewEditFinalityProviderCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "edit-finality-provider [btc_pk]",
		Args:  cobra.ExactArgs(1),
		Short: "Edit an existing finality provider",
		Long: strings.TrimSpace(
			`Edit an existing finality provider.`, // TODO: example
		),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			fs := cmd.Flags()

			// get description
			moniker, _ := fs.GetString(FlagMoniker)
			identity, _ := fs.GetString(FlagIdentity)
			website, _ := fs.GetString(FlagWebsite)
			security, _ := fs.GetString(FlagSecurityContact)
			details, _ := fs.GetString(FlagDetails)
			description := stakingtypes.NewDescription(
				moniker,
				identity,
				website,
				security,
				details,
			)
			// get commission
			rateStr, _ := fs.GetString(FlagCommissionRate)
			rate, err := sdkmath.LegacyNewDecFromStr(rateStr)
			if err != nil {
				return err
			}

			// get BTC PK
			btcPK, err := hex.DecodeString(args[0])
			if err != nil {
				return err
			}

			msg := types.MsgEditFinalityProvider{
				Addr:        clientCtx.FromAddress.String(),
				BtcPk:       btcPK,
				Description: &description,
				Commission:  &rate,
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), &msg)
		},
	}

	fs := cmd.Flags()
	fs.String(FlagMoniker, "", "The finality provider's (optional) moniker")
	fs.String(FlagWebsite, "", "The finality provider's (optional) website")
	fs.String(FlagSecurityContact, "", "The finality provider's (optional) security contact email")
	fs.String(FlagDetails, "", "The finality provider's (optional) details")
	fs.String(FlagIdentity, "", "The (optional) identity signature (ex. UPort or Keybase)")
	fs.String(FlagCommissionRate, "0", "The initial commission rate percentage")

	flags.AddTxFlagsToCmd(cmd)

	return cmd
}

func NewCreateBTCDelegationCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create-btc-delegation [btc_pk] [pop_hex] [staking_tx] [inclusion_proof] [fp_pk1],[fp_pk2],... [staking_time] [staking_value] [slashing_tx] [delegator_slashing_sig] [unbonding_tx] [unbonding_slashing_tx] [unbonding_time] [unbonding_value] [delegator_unbonding_slashing_sig]",
		Args:  cobra.ExactArgs(14),
		Short: "Create a BTC delegation",
		Long: strings.TrimSpace(
			`Create a BTC delegation.`, // TODO: example
		),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			msg, err := parseArgsIntoMsgCreateBTCDelegation(args)
			if err != nil {
				return err
			}

			msg.StakerAddr = clientCtx.FromAddress.String()

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)

	return cmd
}

func NewBTCStakeExpandCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "btc-stake-expand [btc_pk] [pop_hex] [staking_tx] [inclusion_proof] [fp_pk1],[fp_pk2],... [staking_time] [staking_value] [slashing_tx] [delegator_slashing_sig] [unbonding_tx] [unbonding_slashing_tx] [unbonding_time] [unbonding_value] [delegator_unbonding_slashing_sig] [previous_staking_tx_hash] [funding_tx]",
		Args:  cobra.ExactArgs(16),
		Short: "Expand a BTC delegation",
		Long: strings.TrimSpace(
			`Expand a BTC delegation.`,
		),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			parsed, err := parseArgsIntoMsgCreateBTCDelegation(args)
			if err != nil {
				return err
			}

			fundingTx, err := hex.DecodeString(args[15])
			if err != nil {
				return err
			}

			msg := &types.MsgBtcStakeExpand{
				StakerAddr:                    clientCtx.FromAddress.String(),
				BtcPk:                         parsed.BtcPk,
				FpBtcPkList:                   parsed.FpBtcPkList,
				Pop:                           parsed.Pop,
				StakingTime:                   parsed.StakingTime,
				StakingValue:                  parsed.StakingValue,
				StakingTx:                     parsed.StakingTx,
				StakingTxInclusionProof:       parsed.StakingTxInclusionProof,
				SlashingTx:                    parsed.SlashingTx,
				DelegatorSlashingSig:          parsed.DelegatorSlashingSig,
				UnbondingTx:                   parsed.UnbondingTx,
				UnbondingTime:                 parsed.UnbondingTime,
				UnbondingValue:                parsed.UnbondingValue,
				UnbondingSlashingTx:           parsed.UnbondingSlashingTx,
				DelegatorUnbondingSlashingSig: parsed.DelegatorUnbondingSlashingSig,
				PreviousStakingTxHash:         args[14],
				FundingTx:                     fundingTx,
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)

	return cmd
}

func NewAddBTCDelegationInclusionProofCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add-btc-inclusion-proof [staking_tx_hash] [inclusion_proof]",
		Args:  cobra.ExactArgs(2),
		Short: "Add a signature on the unbonding tx of a BTC delegation identified by a given staking tx hash. ",
		Long: strings.TrimSpace(
			`Add a signature on the unbonding tx of a BTC delegation identified by a given staking tx hash signed by the delegator. The signature proves that delegator wants to unbond, and Babylon will consider the BTC delegation unbonded.`, // TODO: example
		),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			// get staking tx hash
			stakingTxHash := args[0]

			inclusionProof, err := types.NewInclusionProofFromHex(args[1])
			if err != nil {
				return err
			}

			msg := types.MsgAddBTCDelegationInclusionProof{
				Signer:                  clientCtx.FromAddress.String(),
				StakingTxHash:           stakingTxHash,
				StakingTxInclusionProof: inclusionProof,
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), &msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)

	return cmd
}

func NewAddCovenantSigsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add-covenant-sigs [covenant_pk] [staking_tx_hash] [slashing_tx_sig1],[slashing_tx_sig2],... [unbonding_tx_sig] [slashing_unbonding_tx_sig1],[slashing_unbonding_tx_sig2],... [stake_expansion_tx_sig]",
		Args:  cobra.RangeArgs(5, 6),
		Short: "Add a covenant signature",
		Long: strings.TrimSpace(
			`Add a covenant signature.`, // TODO: example
		),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			covPK, err := bbn.NewBIP340PubKeyFromHex(args[0])
			if err != nil {
				return fmt.Errorf("invalid public key: %w", err)
			}

			// get staking tx hash
			stakingTxHash := args[1]

			// parse slashing tx sigs
			slashingTxSigs := [][]byte{}
			for _, sigHex := range strings.Split(args[2], ",") {
				sig, err := asig.NewAdaptorSignatureFromHex(sigHex)
				if err != nil {
					return fmt.Errorf("invalid covenant signature: %w", err)
				}
				slashingTxSigs = append(slashingTxSigs, sig.MustMarshal())
			}

			// get covenant signature for unbonding tx
			unbondingTxSig, err := bbn.NewBIP340SignatureFromHex(args[3])
			if err != nil {
				return err
			}

			// parse unbonding slashing tx sigs
			unbondingSlashingSigs := [][]byte{}
			for _, sigHex := range strings.Split(args[4], ",") {
				slashingSig, err := asig.NewAdaptorSignatureFromHex(sigHex)
				if err != nil {
					return fmt.Errorf("invalid covenant signature: %w", err)
				}
				unbondingSlashingSigs = append(unbondingSlashingSigs, slashingSig.MustMarshal())
			}

			msg := types.MsgAddCovenantSigs{
				Signer:                  clientCtx.FromAddress.String(),
				Pk:                      covPK,
				StakingTxHash:           stakingTxHash,
				SlashingTxSigs:          slashingTxSigs,
				UnbondingTxSig:          unbondingTxSig,
				SlashingUnbondingTxSigs: unbondingSlashingSigs,
			}

			// stake expansion
			if len(args) == 6 {
				stkExpSig, err := bbn.NewBIP340SignatureFromHex(args[5])
				if err != nil {
					return err
				}
				msg.StakeExpansionTxSig = stkExpSig
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), &msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)

	return cmd
}

func NewBTCUndelegateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "btc-undelegate [staking_tx_hash] [spend_stake_tx] [spend_stake_tx_inclusion_proof] [funding_tx1],[funding_tx2],...",
		Args:  cobra.ExactArgs(4),
		Short: "Add unbonding information about a BTC delegation identified by a given staking tx hash.",
		Long: strings.TrimSpace(
			`Add unbonding information about a BTC delegation identified by a given staking tx hash. Proof of inclusion proves stake was spent on BTC chain`, // TODO: example
		),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			// get staking tx hash
			stakingTxHash := args[0]

			_, bytes, err := bbn.NewBTCTxFromHex(args[1])
			if err != nil {
				return err
			}

			inclusionProof, err := types.NewInclusionProofFromHex(args[2])
			if err != nil {
				return err
			}

			// parse funding txs
			fundingTxs := [][]byte{}
			for _, fundingTxHex := range strings.Split(args[3], ",") {
				_, fundingTxBytes, err := bbn.NewBTCTxFromHex(fundingTxHex)
				if err != nil {
					return fmt.Errorf("invalid funding tx: %w", err)
				}

				fundingTxs = append(fundingTxs, fundingTxBytes)
			}

			msg := types.MsgBTCUndelegate{
				Signer:                        clientCtx.FromAddress.String(),
				StakingTxHash:                 stakingTxHash,
				StakeSpendingTx:               bytes,
				StakeSpendingTxInclusionProof: inclusionProof,
				FundingTransactions:           fundingTxs,
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), &msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)

	return cmd
}

func NewSelectiveSlashingEvidenceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "selective-slashing-evidence [staking_tx_hash] [recovered_fp_btc_sk]",
		Args:  cobra.ExactArgs(2),
		Short: "Add the recovered BTC SK of a finality provider that launched selective slashing offence.",
		Long: strings.TrimSpace(
			`Add the recovered BTC SK of a finality provider that launched selective slashing offence. The SK is recovered from a pair of Schnorr/adaptor signatures`, // TODO: example
		),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			// get staking tx hash
			stakingTxHash := args[0]

			// get delegator signature for unbonding tx
			fpSKBytes, err := hex.DecodeString(args[1])
			if err != nil {
				return err
			}

			msg := types.MsgSelectiveSlashingEvidence{
				Signer:           clientCtx.FromAddress.String(),
				StakingTxHash:    stakingTxHash,
				RecoveredFpBtcSk: fpSKBytes,
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), &msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)

	return cmd
}

// getCommissionRates retrieves the commission rates information
// from the corresponding flags. If the flag value is empty, uses default values
func getCommissionRates(fs *pflag.FlagSet) (commission types.CommissionRates, err error) {
	rateStr, _ := fs.GetString(FlagCommissionRate)
	maxRateStr, _ := fs.GetString(FlagCommissionMaxRate)
	maxRateChangeStr, _ := fs.GetString(FlagCommissionMaxChangeRate)

	if rateStr == "" || maxRateStr == "" || maxRateChangeStr == "" {
		return commission, errors.New("must specify all validator commission parameters")
	}

	rate, err := sdkmath.LegacyNewDecFromStr(rateStr)
	if err != nil {
		return commission, fmt.Errorf("invalid commission-rate: %w", err)
	}

	maxRate, err := sdkmath.LegacyNewDecFromStr(maxRateStr)
	if err != nil {
		return commission, fmt.Errorf("invalid commission-max-rate: %w", err)
	}

	maxRateChange, err := sdkmath.LegacyNewDecFromStr(maxRateChangeStr)
	if err != nil {
		return commission, fmt.Errorf("invalid commission-max-change-rate: %w", err)
	}
	return types.NewCommissionRates(rate, maxRate, maxRateChange), nil
}

func parseArgsIntoMsgCreateBTCDelegation(args []string) (*types.MsgCreateBTCDelegation, error) {
	// staker pk
	btcPK, err := bbn.NewBIP340PubKeyFromHex(args[0])

	if err != nil {
		return nil, err
	}

	// get PoP
	pop, err := types.NewPoPBTCFromHex(args[1])
	if err != nil {
		return nil, err
	}

	// get staking tx bytes
	stakingTx, err := hex.DecodeString(args[2])
	if err != nil {
		return nil, err
	}

	var inclusionProof *types.InclusionProof
	// inclusionProof can be nil if empty argument is provided
	if len(args[3]) > 0 {
		inclusionProof, err = types.NewInclusionProofFromHex(args[3])
		if err != nil {
			return nil, err
		}
	}

	// get finality provider PKs
	fpPKStrs := strings.Split(args[4], ",")
	fpPKs := make([]bbn.BIP340PubKey, len(fpPKStrs))
	for i := range fpPKStrs {
		fpPK, err := bbn.NewBIP340PubKeyFromHex(fpPKStrs[i])
		if err != nil {
			return nil, err
		}
		fpPKs[i] = *fpPK
	}

	// get staking time
	stakingTime, err := parseLockTime(args[5])
	if err != nil {
		return nil, err
	}

	stakingValue, err := parseBtcAmount(args[6])
	if err != nil {
		return nil, err
	}

	// get slashing tx
	slashingTx, err := types.NewBTCSlashingTxFromHex(args[7])
	if err != nil {
		return nil, err
	}

	// get delegator sig on slashing tx
	delegatorSlashingSig, err := bbn.NewBIP340SignatureFromHex(args[8])
	if err != nil {
		return nil, err
	}

	// get unbonding tx
	_, unbondingTxBytes, err := bbn.NewBTCTxFromHex(args[9])
	if err != nil {
		return nil, err
	}

	// get unbonding slashing tx
	unbondingSlashingTx, err := types.NewBTCSlashingTxFromHex(args[10])
	if err != nil {
		return nil, err
	}

	// get staking time
	unbondingTime, err := parseLockTime(args[11])
	if err != nil {
		return nil, err
	}

	unbondingValue, err := parseBtcAmount(args[12])
	if err != nil {
		return nil, err
	}

	// get delegator sig on unbonding slashing tx
	delegatorUnbondingSlashingSig, err := bbn.NewBIP340SignatureFromHex(args[13])
	if err != nil {
		return nil, err
	}

	return &types.MsgCreateBTCDelegation{
		BtcPk:                         btcPK,
		FpBtcPkList:                   fpPKs,
		Pop:                           pop,
		StakingTime:                   uint32(stakingTime),
		StakingValue:                  int64(stakingValue),
		StakingTx:                     stakingTx,
		StakingTxInclusionProof:       inclusionProof,
		SlashingTx:                    slashingTx,
		DelegatorSlashingSig:          delegatorSlashingSig,
		UnbondingTx:                   unbondingTxBytes,
		UnbondingTime:                 uint32(unbondingTime),
		UnbondingValue:                int64(unbondingValue),
		UnbondingSlashingTx:           unbondingSlashingTx,
		DelegatorUnbondingSlashingSig: delegatorUnbondingSlashingSig,
	}, nil
}
