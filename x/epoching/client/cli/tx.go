package cli

import (
	"fmt"
	"strconv"
	"strings"

	errorsmod "cosmossdk.io/errors"
	"cosmossdk.io/math"
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"

	"github.com/babylonlabs-io/babylon/app/params"
	"github.com/babylonlabs-io/babylon/x/epoching/types"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/tx"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/cosmos/cosmos-sdk/version"
	stakingcli "github.com/cosmos/cosmos-sdk/x/staking/client/cli"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
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
		NewDelegateCmd(),
		NewRedelegateCmd(),
		NewUnbondCmd(),
		NewCancelUnbondingCmd(),
		NewEditValidatorCmd(),
	)

	return cmd
}

func NewDelegateCmd() *cobra.Command {
	bech32PrefixValAddr := params.Bech32PrefixAccAddr
	denom := params.DefaultBondDenom

	cmd := &cobra.Command{
		Use:   "delegate [validator-addr] [amount]",
		Args:  cobra.ExactArgs(2),
		Short: "Delegate liquid tokens to a validator",
		Long: strings.TrimSpace(
			fmt.Sprintf(`Delegate an amount of liquid coins to a validator from your wallet.

Example:
$ %s tx epoching delegate %s1l2rsakp388kuv9k8qzq6lrm9taddae7fpx59wm 1000%s --from mykey
`,
				version.AppName, bech32PrefixValAddr, denom,
			),
		),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}
			amount, err := sdk.ParseCoinNormalized(args[1])
			if err != nil {
				return err
			}

			delAddr := clientCtx.GetFromAddress()
			valAddr, err := sdk.ValAddressFromBech32(args[0])
			if err != nil {
				return err
			}

			stakingMsg := stakingtypes.NewMsgDelegate(delAddr.String(), valAddr.String(), amount)
			msg := types.NewMsgWrappedDelegate(stakingMsg)

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)

	return cmd
}

func NewRedelegateCmd() *cobra.Command {
	bech32PrefixValAddr := params.Bech32PrefixAccAddr
	denom := params.DefaultBondDenom

	cmd := &cobra.Command{
		Use:   "redelegate [src-validator-addr] [dst-validator-addr] [amount]",
		Short: "Redelegate illiquid tokens from one validator to another",
		Args:  cobra.ExactArgs(3),
		Long: strings.TrimSpace(
			fmt.Sprintf(`Redelegate an amount of illiquid staking tokens from one validator to another.

Example:
$ %s tx epoching redelegate %s1gghjut3ccd8ay0zduzj64hwre2fxs9ldmqhffj %s1l2rsakp388kuv9k8qzq6lrm9taddae7fpx59wm 100%s --from mykey
`,
				version.AppName, bech32PrefixValAddr, bech32PrefixValAddr, denom,
			),
		),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}
			delAddr := clientCtx.GetFromAddress()
			valSrcAddr, err := sdk.ValAddressFromBech32(args[0])
			if err != nil {
				return err
			}

			valDstAddr, err := sdk.ValAddressFromBech32(args[1])
			if err != nil {
				return err
			}

			amount, err := sdk.ParseCoinNormalized(args[2])
			if err != nil {
				return err
			}

			stakingMsg := stakingtypes.NewMsgBeginRedelegate(delAddr.String(), valSrcAddr.String(), valDstAddr.String(), amount)
			msg := types.NewMsgWrappedBeginRedelegate(stakingMsg)

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)

	return cmd
}

func NewCancelUnbondingCmd() *cobra.Command {
	bech32PrefixValAddr := params.Bech32PrefixAccAddr
	denom := params.DefaultBondDenom

	cmd := &cobra.Command{
		Use:   "cancel-unbond [validator-addr] [amount] [creation-height]",
		Short: "Cancel unbonding delegation and delegate back to the validator",
		Args:  cobra.ExactArgs(3),
		Long: strings.TrimSpace(
			fmt.Sprintf(`Cancel Unbonding Delegation and delegate back to the validator.

Example:
$ %s tx staking cancel-unbond %s1gghjut3ccd8ay0zduzj64hwre2fxs9ldmqhffj 100%s 2 --from mykey
`,
				version.AppName, bech32PrefixValAddr, denom,
			),
		),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}
			delAddr := clientCtx.GetFromAddress()
			valAddr, err := sdk.ValAddressFromBech32(args[0])
			if err != nil {
				return err
			}

			amount, err := sdk.ParseCoinNormalized(args[1])
			if err != nil {
				return err
			}

			creationHeight, err := strconv.ParseInt(args[2], 10, 64)
			if err != nil {
				return err
			}

			stakingMsg := stakingtypes.NewMsgCancelUnbondingDelegation(delAddr.String(), valAddr.String(), creationHeight, amount)
			msg := types.NewMsgWrappedCancelUnbondingDelegation(stakingMsg)

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)

	return cmd
}

func NewUnbondCmd() *cobra.Command {
	bech32PrefixValAddr := params.Bech32PrefixAccAddr
	denom := params.DefaultBondDenom

	cmd := &cobra.Command{
		Use:   "unbond [validator-addr] [amount]",
		Short: "Unbond shares from a validator",
		Args:  cobra.ExactArgs(2),
		Long: strings.TrimSpace(
			fmt.Sprintf(`Unbond an amount of bonded shares from a validator.

Example:
$ %s tx epoching unbond %s1gghjut3ccd8ay0zduzj64hwre2fxs9ldmqhffj 100%s --from mykey
`,
				version.AppName, bech32PrefixValAddr, denom,
			),
		),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}
			delAddr := clientCtx.GetFromAddress()
			valAddr, err := sdk.ValAddressFromBech32(args[0])
			if err != nil {
				return err
			}

			amount, err := sdk.ParseCoinNormalized(args[1])
			if err != nil {
				return err
			}

			stakingMsg := stakingtypes.NewMsgUndelegate(delAddr.String(), valAddr.String(), amount)
			msg := types.NewMsgWrappedUndelegate(stakingMsg)

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)

	return cmd
}

// NewEditValidatorCmd returns a CLI command handler for creating a MsgWrappedEditValidator transaction.
func NewEditValidatorCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "edit-validator",
		Short: "edit an existing validator account",
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			moniker, _ := cmd.Flags().GetString(stakingcli.FlagEditMoniker)
			identity, _ := cmd.Flags().GetString(stakingcli.FlagIdentity)
			website, _ := cmd.Flags().GetString(stakingcli.FlagWebsite)
			security, _ := cmd.Flags().GetString(stakingcli.FlagSecurityContact)
			details, _ := cmd.Flags().GetString(stakingcli.FlagDetails)
			description := stakingtypes.NewDescription(moniker, identity, website, security, details)

			var newRate *math.LegacyDec

			commissionRate, _ := cmd.Flags().GetString(stakingcli.FlagCommissionRate)
			if commissionRate != "" {
				rate, err := math.LegacyNewDecFromStr(commissionRate)
				if err != nil {
					return fmt.Errorf("invalid new commission rate: %v", err)
				}

				newRate = &rate
			}

			var newMinSelfDelegation *math.Int

			minSelfDelegationString, _ := cmd.Flags().GetString(stakingcli.FlagMinSelfDelegation)
			if minSelfDelegationString != "" {
				msb, ok := math.NewIntFromString(minSelfDelegationString)
				if !ok {
					return errorsmod.Wrap(sdkerrors.ErrInvalidRequest, "minimum self delegation must be a positive integer")
				}

				newMinSelfDelegation = &msb
			}

			valAddr := sdk.ValAddress(clientCtx.GetFromAddress())

			msg := stakingtypes.NewMsgEditValidator(valAddr.String(), description, newRate, newMinSelfDelegation)
			wMsg := types.NewMsgWrappedEditValidator(msg)
			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), wMsg)
		},
	}

	cmd.Flags().AddFlagSet(flagSetDescriptionEdit())
	cmd.Flags().AddFlagSet(flagSetCommissionUpdate())
	cmd.Flags().AddFlagSet(stakingcli.FlagSetMinSelfDelegation())
	flags.AddTxFlagsToCmd(cmd)

	return cmd
}

func flagSetDescriptionEdit() *flag.FlagSet {
	fs := flag.NewFlagSet("", flag.ContinueOnError)

	fs.String(stakingcli.FlagEditMoniker, stakingtypes.DoNotModifyDesc, "The validator's name")
	fs.String(stakingcli.FlagIdentity, stakingtypes.DoNotModifyDesc, "The (optional) identity signature (ex. UPort or Keybase)")
	fs.String(stakingcli.FlagWebsite, stakingtypes.DoNotModifyDesc, "The validator's (optional) website")
	fs.String(stakingcli.FlagSecurityContact, stakingtypes.DoNotModifyDesc, "The validator's (optional) security contact email")
	fs.String(stakingcli.FlagDetails, stakingtypes.DoNotModifyDesc, "The validator's (optional) details")

	return fs
}

func flagSetCommissionUpdate() *flag.FlagSet {
	fs := flag.NewFlagSet("", flag.ContinueOnError)

	fs.String(stakingcli.FlagCommissionRate, "", "The new commission rate percentage")

	return fs
}
