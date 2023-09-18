package proposals

import (
	"fmt"

	batch "github.com/rocket-pool/batch-query"
	"github.com/rocket-pool/rocketpool-go/core"
	"github.com/rocket-pool/rocketpool-go/rocketpool"
)

// ===============
// === Structs ===
// ===============

// Binding for Protocol DAO proposals
type ProtocolDaoProposal struct {
	*proposalCommon
	Details ProtocolDaoProposalDetails
	rp      *rocketpool.RocketPool
	mgr     *core.Contract
}

// Details for proposals
type ProtocolDaoProposalDetails struct {
	*proposalCommonDetails
}

// ====================
// === Constructors ===
// ====================

// Creates a new ProtocolDaoProposal contract binding
func newProtocolDaoProposal(rp *rocketpool.RocketPool, base *proposalCommon) (*ProtocolDaoProposal, error) {
	// Create the contract
	contract, err := rp.GetContract(rocketpool.ContractName_RocketDAOProposal)
	if err != nil {
		return nil, fmt.Errorf("error getting DAO proposal contract: %w", err)
	}

	return &ProtocolDaoProposal{
		proposalCommon: base,
		Details: ProtocolDaoProposalDetails{
			proposalCommonDetails: &base.Details,
		},
		rp:  rp,
		mgr: contract,
	}, nil
}

// =============
// === Calls ===
// =============

// Get the basic details
func (c *ProtocolDaoProposal) QueryAllDetails(mc *batch.MultiCaller) {
	c.proposalCommon.QueryAllDetails(mc)
}
