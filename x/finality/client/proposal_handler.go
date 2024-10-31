package client

import (
	govclient "github.com/cosmos/cosmos-sdk/x/gov/client"

	"github.com/babylonlabs-io/babylon/x/finality/client/cli"
)

var (
	ResumeFinalityHandler = govclient.NewProposalHandler(cli.NewCmdSubmitResumeFinalityProposal)
)
