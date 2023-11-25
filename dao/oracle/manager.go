package oracle

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"

	batch "github.com/rocket-pool/batch-query"
	"github.com/rocket-pool/rocketpool-go/core"
	"github.com/rocket-pool/rocketpool-go/rocketpool"
	"github.com/rocket-pool/rocketpool-go/utils"
	"github.com/rocket-pool/rocketpool-go/utils/strings"
)

const (
	oracleDaoMemberBatchSize int = 200
)

// ===============
// === Structs ===
// ===============

// Binding for RocketDAONodeTrusted
type OracleDaoManager struct {
	// The number of members in the Oracle DAO
	MemberCount *core.FormattedUint256Field[uint64]

	// The minimum number of members allowed in the Oracle DAO
	MinimumMemberCount *core.FormattedUint256Field[uint64]

	// Settings for the Oracle DAO
	Settings *OracleDaoSettings

	// === Internal fields ===
	rp   *rocketpool.RocketPool
	dnt  *core.Contract
	dnta *core.Contract
	dntp *core.Contract
}

// ====================
// === Constructors ===
// ====================

// Creates a new OracleDaoManager contract binding
func NewOracleDaoManager(rp *rocketpool.RocketPool) (*OracleDaoManager, error) {
	// Create the contracts
	dnt, err := rp.GetContract(rocketpool.ContractName_RocketDAONodeTrusted)
	if err != nil {
		return nil, fmt.Errorf("error getting DAO node trusted contract: %w", err)
	}
	dnta, err := rp.GetContract(rocketpool.ContractName_RocketDAONodeTrustedActions)
	if err != nil {
		return nil, fmt.Errorf("error getting DAO node trusted actions contract: %w", err)
	}
	dntp, err := rp.GetContract(rocketpool.ContractName_RocketDAONodeTrustedProposals)
	if err != nil {
		return nil, fmt.Errorf("error getting DAO node trusted proposals contract: %w", err)
	}

	odaoMgr := &OracleDaoManager{
		MemberCount:        core.NewFormattedUint256Field[uint64](dnt, "getMemberCount"),
		MinimumMemberCount: core.NewFormattedUint256Field[uint64](dnt, "getMemberMinRequired"),

		rp:   rp,
		dnt:  dnt,
		dnta: dnta,
		dntp: dntp,
	}
	settings, err := newOracleDaoSettings(odaoMgr)
	if err != nil {
		return nil, fmt.Errorf("error creating Oracle DAO settings binding: %w", err)
	}
	odaoMgr.Settings = settings
	return odaoMgr, nil
}

// ====================
// === Transactions ===
// ====================

// === DAONodeTrusted ===

// Bootstrap a bool setting
func (c *OracleDaoManager) BootstrapBool(contractName rocketpool.ContractName, settingPath string, value bool, opts *bind.TransactOpts) (*core.TransactionInfo, error) {
	return core.NewTransactionInfo(c.dnt, "bootstrapSettingBool", opts, contractName, settingPath, value)
}

// Bootstrap a uint setting
func (c *OracleDaoManager) BootstrapUint(contractName rocketpool.ContractName, settingPath string, value *big.Int, opts *bind.TransactOpts) (*core.TransactionInfo, error) {
	return core.NewTransactionInfo(c.dnt, "bootstrapSettingUint", opts, contractName, settingPath, value)
}

// Bootstrap a member into the Oracle DAO
func (c *OracleDaoManager) BootstrapMember(id string, url string, nodeAddress common.Address, opts *bind.TransactOpts) (*core.TransactionInfo, error) {
	return core.NewTransactionInfo(c.dnt, "bootstrapMember", opts, id, url, nodeAddress)
}

// Bootstrap a contract upgrade
func (c *OracleDaoManager) BootstrapUpgrade(upgradeType string, contractName rocketpool.ContractName, contractAbi string, contractAddress common.Address, opts *bind.TransactOpts) (*core.TransactionInfo, error) {
	compressedAbi, err := core.EncodeAbiStr(contractAbi)
	if err != nil {
		return nil, fmt.Errorf("error compressing ABI: %w", err)
	}
	return core.NewTransactionInfo(c.dnt, "bootstrapUpgrade", opts, upgradeType, contractName, compressedAbi, contractAddress)
}

// === DAONodeTrustedActions ===

// Get info for joining the Oracle DAO
func (c *OracleDaoManager) Join(opts *bind.TransactOpts) (*core.TransactionInfo, error) {
	return core.NewTransactionInfo(c.dnta, "actionJoin", opts)
}

// Get info for leaving the Oracle DAO
func (c *OracleDaoManager) Leave(rplBondRefundAddress common.Address, opts *bind.TransactOpts) (*core.TransactionInfo, error) {
	return core.NewTransactionInfo(c.dnta, "actionLeave", opts, rplBondRefundAddress)
}

// Get info for making a challenge to an Oracle DAO member
func (c *OracleDaoManager) MakeChallenge(memberAddress common.Address, opts *bind.TransactOpts) (*core.TransactionInfo, error) {
	return core.NewTransactionInfo(c.dnta, "actionChallengeMake", opts, memberAddress)
}

// Get info for deciding a challenge to an Oracle DAO member
func (c *OracleDaoManager) DecideChallenge(memberAddress common.Address, opts *bind.TransactOpts) (*core.TransactionInfo, error) {
	return core.NewTransactionInfo(c.dnta, "actionChallengeDecide", opts, memberAddress)
}

// === DAONodeTrustedProposals ===

// Get info for proposing to invite a new member to the Oracle DAO
func (c *OracleDaoManager) ProposeInviteMember(message string, newMemberAddress common.Address, newMemberId string, newMemberUrl string, opts *bind.TransactOpts) (*core.TransactionInfo, error) {
	newMemberUrl = strings.Sanitize(newMemberUrl)
	if message == "" {
		message = fmt.Sprintf("invite %s (%s)", newMemberId, newMemberAddress.Hex())
	}
	return c.submitProposal(opts, message, "proposalInvite", newMemberId, newMemberUrl, newMemberAddress)
}

// Get info for proposing to leave the Oracle DAO
func (c *OracleDaoManager) ProposeMemberLeave(message string, memberAddress common.Address, opts *bind.TransactOpts) (*core.TransactionInfo, error) {
	return c.submitProposal(opts, message, "proposalLeave", memberAddress)
}

// Get info for proposing to kick a member from the Oracle DAO
func (c *OracleDaoManager) ProposeKickMember(message string, memberAddress common.Address, rplFineAmount *big.Int, opts *bind.TransactOpts) (*core.TransactionInfo, error) {
	if message == "" {
		message = fmt.Sprintf("kick %s", memberAddress.Hex())
	}
	return c.submitProposal(opts, message, "proposalKick", memberAddress, rplFineAmount)
}

// Get info for proposing a bool setting
func (c *OracleDaoManager) ProposeSetBool(message string, contractName rocketpool.ContractName, settingPath string, value bool, opts *bind.TransactOpts) (*core.TransactionInfo, error) {
	if message == "" {
		message = fmt.Sprintf("set %s", settingPath)
	}
	return c.submitProposal(opts, message, "proposalSettingBool", contractName, settingPath, value)
}

// Get info for proposing a uint setting
func (c *OracleDaoManager) ProposeSetUint(message string, contractName rocketpool.ContractName, settingPath string, value *big.Int, opts *bind.TransactOpts) (*core.TransactionInfo, error) {
	if message == "" {
		message = fmt.Sprintf("set %s", settingPath)
	}
	return c.submitProposal(opts, message, "proposalSettingUint", contractName, settingPath, value)
}

// Get info for proposing a contract upgrade
func (c *OracleDaoManager) ProposeUpgradeContract(message string, upgradeType string, contractName rocketpool.ContractName, contractAbi string, contractAddress common.Address, opts *bind.TransactOpts) (*core.TransactionInfo, error) {
	compressedAbi, err := core.EncodeAbiStr(contractAbi)
	if err != nil {
		return nil, fmt.Errorf("error compressing ABI: %w", err)
	}
	return c.submitProposal(opts, message, "proposalUpgrade", upgradeType, contractName, compressedAbi, contractAddress)
}

// Internal method used for actually constructing and submitting a proposal
func (c *OracleDaoManager) submitProposal(opts *bind.TransactOpts, message string, method string, args ...interface{}) (*core.TransactionInfo, error) {
	payload, err := c.dntp.ABI.Pack(method, args...)
	if err != nil {
		return nil, fmt.Errorf("error encoding payload: %w", err)
	}
	return core.NewTransactionInfo(c.dntp, "propose", opts, message, payload)
}

// =================
// === Addresses ===
// =================

// Get an Oracle DAO member address by index
func (c *OracleDaoManager) GetMemberAddress(mc *batch.MultiCaller, address_Out *common.Address, index uint64) {
	core.AddCall(mc, c.dnt, address_Out, "getMemberAt", big.NewInt(int64(index)))
}

// Get the list of Oracle DAO member addresses.
func (c *OracleDaoManager) GetMemberAddresses(memberCount uint64, opts *bind.CallOpts) ([]common.Address, error) {
	addresses := make([]common.Address, memberCount)

	// Run the multicall query for each address
	err := c.rp.BatchQuery(int(memberCount), c.rp.AddressBatchSize,
		func(mc *batch.MultiCaller, index int) error {
			c.GetMemberAddress(mc, &addresses[index], uint64(index))
			return nil
		},
		opts,
	)
	if err != nil {
		return nil, fmt.Errorf("error getting Oracle DAO member addresses: %w", err)
	}

	// Return
	return addresses, nil
}

// Get an Oracle DAO member by address.
func (c *OracleDaoManager) CreateMemberFromAddress(address common.Address, includeDetails bool, opts *bind.CallOpts) (*OracleDaoMember, error) {
	// Create the member binding
	member, err := NewOracleDaoMember(c.rp, address)
	if err != nil {
		return nil, fmt.Errorf("error creating Oracle DAO member binding for %s: %w", address.Hex(), err)
	}

	if includeDetails {
		err = c.rp.Query(func(mc *batch.MultiCaller) error {
			core.QueryAllFields(member, mc)
			return nil
		}, opts)
		if err != nil {
			return nil, fmt.Errorf("error getting Oracle DAO member details: %w", err)
		}
	}

	// Return
	return member, nil
}

// Get the list of all Oracle DAO members.
func (c *OracleDaoManager) CreateMembersFromAddresses(addresses []common.Address, includeDetails bool, opts *bind.CallOpts) ([]*OracleDaoMember, error) {
	// Create the member bindings
	memberCount := len(addresses)
	members := make([]*OracleDaoMember, memberCount)
	for i, address := range addresses {
		member, err := NewOracleDaoMember(c.rp, address)
		if err != nil {
			return nil, fmt.Errorf("error creating Oracle DAO member binding for %s: %w", address.Hex(), err)
		}
		members[i] = member
	}

	if includeDetails {
		err := c.rp.BatchQuery(int(memberCount), oracleDaoMemberBatchSize,
			func(mc *batch.MultiCaller, index int) error {
				member := members[index]
				core.QueryAllFields(member, mc)
				return nil
			},
			opts,
		)
		if err != nil {
			return nil, fmt.Errorf("error getting Oracle DAO member details: %w", err)
		}
	}

	// Return
	return members, nil
}

// =============
// === Utils ===
// =============

// Returns the most recent block number that the number of trusted nodes changed since fromBlock
func (c *OracleDaoManager) GetLatestMemberCountChangedBlock(fromBlock uint64, intervalSize *big.Int, opts *bind.CallOpts) (uint64, error) {
	// Construct a filter query for relevant logs
	addressFilter := []common.Address{*c.dnta.Address}
	topicFilter := [][]common.Hash{{
		c.dnta.ABI.Events["ActionJoined"].ID,
		c.dnta.ABI.Events["ActionLeave"].ID,
		c.dnta.ABI.Events["ActionKick"].ID,
		c.dnta.ABI.Events["ActionChallengeDecided"].ID,
	}}

	// Get the event logs
	logs, err := utils.GetLogs(c.rp, addressFilter, topicFilter, intervalSize, big.NewInt(int64(fromBlock)), nil, nil)
	if err != nil {
		return 0, err
	}

	for i := range logs {
		log := logs[len(logs)-i-1]
		if log.Topics[0] == c.dnta.ABI.Events["ActionChallengeDecided"].ID {
			values := make(map[string]interface{})
			// Decode the event
			if c.dnta.ABI.Events["ActionChallengeDecided"].Inputs.UnpackIntoMap(values, log.Data) != nil {
				return 0, err
			}
			if values["success"].(bool) {
				return log.BlockNumber, nil
			}
		} else {
			return log.BlockNumber, nil
		}
	}
	return fromBlock, nil
}
