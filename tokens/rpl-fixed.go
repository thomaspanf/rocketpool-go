package tokens

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"

	"github.com/rocket-pool/rocketpool-go/core"
	"github.com/rocket-pool/rocketpool-go/rocketpool"
	"github.com/rocket-pool/rocketpool-go/utils/multicall"
)

// ===============
// === Structs ===
// ===============

// Binding for RocketTokenRPLFixedSupply
type TokenRplFixedSupply struct {
	Details  TokenRplFixedSupplyDetails
	rp       *rocketpool.RocketPool
	contract *core.Contract
}

// Details for RocketTokenRPLFixedSupply
type TokenRplFixedSupplyDetails struct {
	TotalSupply *big.Int `json:"totalSupply"`
}

// ====================
// === Constructors ===
// ====================

// Creates a new TokenRplFixedSupply contract binding
func NewTokenRplFixedSupply(rp *rocketpool.RocketPool) (*TokenRplFixedSupply, error) {
	// Create the contract
	contract, err := rp.GetContract(rocketpool.ContractName_RocketTokenRPLFixedSupply)
	if err != nil {
		return nil, fmt.Errorf("error getting RPL fixed supply contract: %w", err)
	}

	return &TokenRplFixedSupply{
		Details:  TokenRplFixedSupplyDetails{},
		rp:       rp,
		contract: contract,
	}, nil
}

// =============
// === Calls ===
// =============

// === Core ERC-20 functions ===

// Get the fixed-supply RPL total supply
func (c *TokenRplFixedSupply) GetTotalSupply(mc *multicall.MultiCaller) {
	multicall.AddCall(mc, c.contract, &c.Details.TotalSupply, "totalSupply")
}

// Get the fixed-supply RPL balance of an address
func (c *TokenRplFixedSupply) GetBalance(mc *multicall.MultiCaller, balance_Out **big.Int, address common.Address) {
	multicall.AddCall(mc, c.contract, balance_Out, "balanceOf", address)
}

// Get the fixed-supply RPL spending allowance of an address and spender
func (c *TokenRplFixedSupply) GetAllowance(mc *multicall.MultiCaller, allowance_Out **big.Int, owner common.Address, spender common.Address) {
	multicall.AddCall(mc, c.contract, allowance_Out, "allowance", owner, spender)
}

// ====================
// === Transactions ===
// ====================

// === Core ERC-20 functions ===

// Get info for approving fixed-supply RPL's usage by a spender
func (c *TokenRplFixedSupply) Approve(spender common.Address, amount *big.Int, opts *bind.TransactOpts) (*core.TransactionInfo, error) {
	return core.NewTransactionInfo(c.contract, "approve", opts, spender, amount)
}

// Get info for transferring fixed-supply RPL
func (c *TokenRplFixedSupply) Transfer(to common.Address, amount *big.Int, opts *bind.TransactOpts) (*core.TransactionInfo, error) {
	return core.NewTransactionInfo(c.contract, "transfer", opts, to, amount)
}

// Get info for transferring fixed-supply RPL from a sender
func (c *TokenRplFixedSupply) TransferFrom(from common.Address, to common.Address, amount *big.Int, opts *bind.TransactOpts) (*core.TransactionInfo, error) {
	return core.NewTransactionInfo(c.contract, "transferFrom", opts, from, to, amount)
}
