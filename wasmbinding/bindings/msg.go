package bindings

import ftypes "github.com/babylonlabs-io/babylon/v3/x/finality/types"

type BabylonMsg struct {
	MsgEquivocationEvidence *ftypes.MsgEquivocationEvidence `json:"msg_equivocation_evidence,omitempty"`
}
