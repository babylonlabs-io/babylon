package client

import (
	"github.com/cosmos/cosmos-sdk/client"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/address"
	"github.com/cosmos/cosmos-sdk/x/gov/client/cli"
	"github.com/spf13/cobra"
)

// adapted from
// https://github.com/osmosis-labs/osmosis/blob/2e85d1ee3e15e3f74898395d37b455af48649268/osmoutils/osmocli/parsers.go

var DefaultGovAuthority = sdk.AccAddress(address.Module("gov"))

const (
	FlagIsExpedited = "is-expedited"
	FlagAuthority   = "authority"
)

func GetProposalInfo(cmd *cobra.Command) (client.Context, string, string, sdk.Coins, bool, sdk.AccAddress, error) {
	clientCtx, err := client.GetClientTxContext(cmd)
	if err != nil {
		return client.Context{}, "", "", nil, false, nil, err
	}

	proposalTitle, err := cmd.Flags().GetString(cli.FlagTitle)
	if err != nil {
		return clientCtx, proposalTitle, "", nil, false, nil, err
	}

	summary, err := cmd.Flags().GetString(cli.FlagSummary)
	if err != nil {
		return client.Context{}, proposalTitle, summary, nil, false, nil, err
	}

	depositArg, err := cmd.Flags().GetString(cli.FlagDeposit)
	if err != nil {
		return client.Context{}, proposalTitle, summary, nil, false, nil, err
	}

	deposit, err := sdk.ParseCoinsNormalized(depositArg)
	if err != nil {
		return client.Context{}, proposalTitle, summary, deposit, false, nil, err
	}

	isExpedited, err := cmd.Flags().GetBool(FlagIsExpedited)
	if err != nil {
		return client.Context{}, proposalTitle, summary, deposit, false, nil, err
	}

	authorityString, err := cmd.Flags().GetString(FlagAuthority)
	if err != nil {
		return client.Context{}, proposalTitle, summary, deposit, false, nil, err
	}
	authority, err := sdk.AccAddressFromBech32(authorityString)
	if err != nil {
		return client.Context{}, proposalTitle, summary, deposit, false, nil, err
	}

	return clientCtx, proposalTitle, summary, deposit, isExpedited, authority, nil
}

func AddCommonProposalFlags(cmd *cobra.Command) {
	cmd.Flags().String(cli.FlagTitle, "", "Title of proposal")
	cmd.Flags().String(cli.FlagSummary, "", "Summary of proposal")
	cmd.Flags().String(cli.FlagDeposit, "", "Deposit of proposal")
	cmd.Flags().Bool(FlagIsExpedited, false, "Whether the proposal is expedited")
	cmd.Flags().String(FlagAuthority, DefaultGovAuthority.String(), "The address of the governance account. Default is the sdk gov module account")
}
