package protocol

import (
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/rocket-pool/node-manager-core/eth"
	"github.com/rocket-pool/rocketpool-go/core"
	"github.com/rocket-pool/rocketpool-go/rocketpool"
	"github.com/rocket-pool/rocketpool-go/types"
)

/// ==================
/// === Interfaces ===
/// ==================

// A general interface for settings, parameterized by the type required for proposals and boostrapping
type IProtocolDaoSetting[ProposeType core.CallReturnType] interface {
	eth.IQueryable
	GetContract() rocketpool.ContractName
	GetSettingName() SettingName
	ProposeSet(value ProposeType, blockNumber uint32, treeNodes []types.VotingTreeNode, opts *bind.TransactOpts) (*eth.TransactionInfo, error)
	Bootstrap(value ProposeType, opts *bind.TransactOpts) (*eth.TransactionInfo, error)
}

/// ===================
/// === BoolSetting ===
/// ===================

// A simple boolean setting
type ProtocolDaoBoolSetting struct {
	*core.SimpleField[bool]

	// === Internal fields ===
	settingContract rocketpool.ContractName
	pdaoMgr         *ProtocolDaoManager
	settingName     SettingName
}

// Creates a new bool setting
func newBoolSetting(settingContract *core.Contract, pdaoMgr *ProtocolDaoManager, settingName SettingName) *ProtocolDaoBoolSetting {
	return &ProtocolDaoBoolSetting{
		SimpleField:     core.NewSimpleField[bool](settingContract, "getSettingBool", settingName),
		settingContract: rocketpool.ContractName(settingContract.Name),
		pdaoMgr:         pdaoMgr,
		settingName:     settingName,
	}
}

// Gets the owning contract of this setting
func (s *ProtocolDaoBoolSetting) GetContract() rocketpool.ContractName {
	return s.settingContract
}

// Gets the underlying path for the setting within the contracts
func (s *ProtocolDaoBoolSetting) GetSettingName() SettingName {
	return s.settingName
}

// Creates a proposal to change the setting
func (s *ProtocolDaoBoolSetting) ProposeSet(value bool, blockNumber uint32, treeNodes []types.VotingTreeNode, opts *bind.TransactOpts) (*eth.TransactionInfo, error) {
	return s.pdaoMgr.ProposeSetBool("", s.settingContract, s.settingName, value, blockNumber, treeNodes, opts)
}

// Bootstraps the setting with a new value
func (s *ProtocolDaoBoolSetting) Bootstrap(value bool, opts *bind.TransactOpts) (*eth.TransactionInfo, error) {
	return s.pdaoMgr.BootstrapBool(s.settingContract, s.settingName, value, opts)
}

/// ===================
/// === UintSetting ===
/// ===================

// A simple uint setting
type ProtocolDaoUintSetting struct {
	*core.SimpleField[*big.Int]

	// === Internal fields ===
	settingContract rocketpool.ContractName
	pdaoMgr         *ProtocolDaoManager
	settingName     SettingName
}

// Creates a new uint setting
func newUintSetting(settingContract *core.Contract, pdaoMgr *ProtocolDaoManager, settingName SettingName) *ProtocolDaoUintSetting {
	return &ProtocolDaoUintSetting{
		SimpleField:     core.NewSimpleField[*big.Int](settingContract, "getSettingUint", settingName),
		settingContract: rocketpool.ContractName(settingContract.Name),
		pdaoMgr:         pdaoMgr,
		settingName:     settingName,
	}
}

// Gets the owning contract of this setting
func (s *ProtocolDaoUintSetting) GetContract() rocketpool.ContractName {
	return s.settingContract
}

// Gets the underlying path for the setting within the contracts
func (s *ProtocolDaoUintSetting) GetSettingName() SettingName {
	return s.settingName
}

// Creates a proposal to change the setting
func (s *ProtocolDaoUintSetting) ProposeSet(value *big.Int, blockNumber uint32, treeNodes []types.VotingTreeNode, opts *bind.TransactOpts) (*eth.TransactionInfo, error) {
	return s.pdaoMgr.ProposeSetUint("", s.settingContract, s.settingName, value, blockNumber, treeNodes, opts)
}

// Bootstraps the setting with a new value
func (s *ProtocolDaoUintSetting) Bootstrap(value *big.Int, opts *bind.TransactOpts) (*eth.TransactionInfo, error) {
	return s.pdaoMgr.BootstrapUint(s.settingContract, s.settingName, value, opts)
}

/// =======================
/// === CompoundSetting ===
/// =======================

// A uint256 setting that can be formatted to a more well-defined type
type ProtocolDaoCompoundSetting[DataType core.FormattedUint256Type] struct {
	*core.FormattedUint256Field[DataType]

	// === Internal fields ===
	settingContract rocketpool.ContractName
	pdaoMgr         *ProtocolDaoManager
	settingName     SettingName
}

// Creates a new compound setting
func newCompoundSetting[DataType core.FormattedUint256Type](settingContract *core.Contract, pdaoMgr *ProtocolDaoManager, settingName SettingName) *ProtocolDaoCompoundSetting[DataType] {
	s := &ProtocolDaoCompoundSetting[DataType]{
		FormattedUint256Field: core.NewFormattedUint256Field[DataType](settingContract, "getSettingUint", settingName),
		settingContract:       rocketpool.ContractName(settingContract.Name),
		pdaoMgr:               pdaoMgr,
		settingName:           settingName,
	}

	return s
}

// Gets the owning contract of this setting
func (s *ProtocolDaoCompoundSetting[DataType]) GetContract() rocketpool.ContractName {
	return s.settingContract
}

// Gets the underlying path for the setting within the contracts
func (s *ProtocolDaoCompoundSetting[DataType]) GetSettingName() SettingName {
	return s.settingName
}

// Creates a proposal to change the setting
func (s *ProtocolDaoCompoundSetting[DataType]) ProposeSet(value *big.Int, blockNumber uint32, treeNodes []types.VotingTreeNode, opts *bind.TransactOpts) (*eth.TransactionInfo, error) {
	return s.pdaoMgr.ProposeSetUint("", s.settingContract, s.settingName, value, blockNumber, treeNodes, opts)
}

// Bootstraps the setting with a new value
func (s *ProtocolDaoCompoundSetting[DataType]) Bootstrap(value *big.Int, opts *bind.TransactOpts) (*eth.TransactionInfo, error) {
	return s.pdaoMgr.BootstrapUint(s.settingContract, s.settingName, value, opts)
}