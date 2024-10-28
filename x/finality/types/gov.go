package types

import (
	"fmt"
	"strings"

	govtypesv1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1beta1"

	"github.com/babylonlabs-io/babylon/types"
)

const (
	ProposalResumeFinality = "ResumeFinality"
)

// Init registers proposals to update and replace pool incentives.
func init() {
	govtypesv1.RegisterProposalType(ProposalResumeFinality)
}

var (
	_ govtypesv1.Content = &ResumeFinalityProposal{}
)

// NewResumeFinalityProposal returns a new instance of a resume finality proposal struct.
func NewResumeFinalityProposal(title, description string, fps []types.BIP340PubKey, haltingHeight uint32) govtypesv1.Content {
	return &ResumeFinalityProposal{
		Title:         title,
		Description:   description,
		FpPks:         fps,
		HaltingHeight: haltingHeight,
	}
}

// GetTitle gets the title of the proposal
func (p *ResumeFinalityProposal) GetTitle() string { return p.Title }

// GetDescription gets the description of the proposal
func (p *ResumeFinalityProposal) GetDescription() string { return p.Description }

// ProposalRoute returns the router key for the proposal
func (p *ResumeFinalityProposal) ProposalRoute() string { return RouterKey }

// ProposalType returns the type of the proposal
func (p *ResumeFinalityProposal) ProposalType() string {
	return ProposalResumeFinality
}

// ValidateBasic validates a governance proposal's abstract and basic contents
func (p *ResumeFinalityProposal) ValidateBasic() error {
	err := govtypesv1.ValidateAbstract(p)
	if err != nil {
		return err
	}
	if len(p.FpPks) == 0 {
		return ErrEmptyProposalFinalityProviders
	}

	if p.HaltingHeight == 0 {
		return ErrEmptyProposalHaltingHeight
	}

	return nil
}

// String returns a string containing the jail finality providers proposal.
func (p *ResumeFinalityProposal) String() string {
	fpsStr := fmt.Sprintln("Finality providers to jail:")
	for i, pk := range p.FpPks {
		fpsStr = fpsStr + fmt.Sprintf("%d. %s\n", i+1, pk.MarshalHex())
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf(`Resume Finality Proposal:
  Title:       %s
  Description: %s
  Halting height: %d
  %s
`, p.Title, p.Description, p.HaltingHeight, fpsStr))
	return b.String()
}
