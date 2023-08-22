package rocketpool

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"golang.org/x/sync/errgroup"

	batch "github.com/rocket-pool/batch-query"
	"github.com/rocket-pool/rocketpool-go/core"
	"github.com/rocket-pool/rocketpool-go/storage"
)

const (
	defaultConcurrentCallLimit      int = 6
	defaultAddressBatchSize         int = 1000
	defaultContractVersionBatchSize int = 500
	defaultBalanceBatchSize         int = 1000
)

// Rocket Pool contract manager
type RocketPool struct {
	Client                   core.ExecutionClient
	Storage                  *storage.Storage
	DeployBlock              *big.Int
	MulticallAddress         *common.Address
	BalanceBatcher           *batch.BalanceBatcher
	VersionManager           *VersionManager
	ConcurrentCallLimit      int
	AddressBatchSize         int
	ContractVersionBatchSize int

	// Internal fields
	contracts    map[ContractName]*core.Contract
	instanceAbis map[ContractName]*abi.ABI // Used for instanced contracts like minipools or fee distributors
}

// Create new contract manager
func NewRocketPool(client core.ExecutionClient, rocketStorageAddress common.Address, multicallAddress common.Address, balanceBatcherAddress common.Address) (*RocketPool, error) {
	// Create the RocketStorage binding
	storage, err := storage.NewStorage(client, rocketStorageAddress)
	if err != nil {
		return nil, fmt.Errorf("error creating rocket storage binding: %w", err)
	}

	// Create the balance batcher
	balanceBatcher, err := batch.NewBalanceBatcher(client, balanceBatcherAddress, defaultBalanceBatchSize, defaultConcurrentCallLimit)
	if err != nil {
		return nil, fmt.Errorf("error creating balance batcher: %w", err)
	}

	// Create the binding
	rp := &RocketPool{
		Client:                   client,
		Storage:                  storage,
		MulticallAddress:         &multicallAddress,
		BalanceBatcher:           balanceBatcher,
		ConcurrentCallLimit:      defaultConcurrentCallLimit,
		AddressBatchSize:         defaultAddressBatchSize,
		ContractVersionBatchSize: defaultContractVersionBatchSize,
		contracts:                map[ContractName]*core.Contract{},
		instanceAbis:             map[ContractName]*abi.ABI{},
	}
	rp.VersionManager = NewVersionManager(rp)

	// Get the block the protocol was deployed on
	deployBlockHash := crypto.Keccak256Hash([]byte("deploy.block"))
	err = rp.Query(func(mc *batch.MultiCaller) error {
		rp.Storage.GetUint(mc, &rp.DeployBlock, deployBlockHash)
		return nil
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("error getting deployment block: %w", err)
	}

	return rp, nil
}

// Load all network contracts
func (rp *RocketPool) LoadAllContracts(opts *bind.CallOpts) error {
	err := rp.LoadContracts(opts, ContractNames...)
	if err != nil {
		return fmt.Errorf("error loading contracts: %w", err)
	}

	err = rp.LoadInstanceABIs(opts, InstanceContractNames...)
	if err != nil {
		return fmt.Errorf("error loading instance contract ABIs: %w", err)
	}
	return nil
}

// Load only the provided specific contracts by their name
func (rp *RocketPool) LoadContracts(opts *bind.CallOpts, contractNames ...ContractName) error {
	addresses := make([]common.Address, len(contractNames))
	abiStrings := make([]string, len(contractNames))

	// Load the details via multicall
	results, err := rp.FlexQuery(func(mc *batch.MultiCaller) error {
		for i, contractName := range contractNames {
			rp.Storage.GetAddress(mc, &addresses[i], string(contractName))
			rp.Storage.GetAbi(mc, &abiStrings[i], string(contractName))
		}
		return nil
	}, opts)
	if err != nil {
		return fmt.Errorf("error getting addresses and ABIs: %w", err)
	}
	for i, result := range results {
		if !result {
			contractName := contractNames[i]
			return fmt.Errorf("failed getting address and ABI for contract %s", contractName)
		}
	}

	// Create the contract objects
	for i, contractName := range contractNames {
		// Decode the ABI
		abi, err := core.DecodeAbi(abiStrings[i])
		if err != nil {
			return fmt.Errorf("error decoding contract %s ABI: %w", string(contractNames[i]), err)
		}

		// Make the contract binding
		contract := &core.Contract{
			Contract: bind.NewBoundContract(addresses[i], *abi, rp.Client, rp.Client, rp.Client),
			Address:  &addresses[i],
			ABI:      abi,
			Client:   rp.Client,
		}
		rp.contracts[contractName] = contract
	}

	// Get the versions of each contract
	results, err = rp.FlexQuery(func(mc *batch.MultiCaller) error {
		for _, contractName := range contractNames {
			contract := rp.contracts[contractName]
			err := GetContractVersion(mc, &contract.Version, *contract.Address)
			if err != nil {
				return fmt.Errorf("error getting version for contract %s: %w", string(contractName), err)
			}
		}
		return nil
	}, opts)
	if err != nil {
		return fmt.Errorf("error getting contract versions: %w", err)
	}
	for i, result := range results {
		if !result {
			contract := rp.contracts[contractNames[i]]
			contract.Version = 1 // If the contract doesn't have a version() in its ABI then it's v1
		}
	}

	return nil
}

// Load the ABIs for instances contracts (like minipools or fee distributors)
func (rp *RocketPool) LoadInstanceABIs(opts *bind.CallOpts, contractNames ...ContractName) error {
	abiStrings := make([]string, len(contractNames))

	// Load the details via multicall
	err := rp.Query(func(mc *batch.MultiCaller) error {
		for i, contractName := range contractNames {
			rp.Storage.GetAbi(mc, &abiStrings[i], string(contractName))
		}
		return nil
	}, opts)
	if err != nil {
		return fmt.Errorf("error getting instanced ABIs: %w", err)
	}

	// Create the contract objects
	for i, contractName := range contractNames {
		// Decode the ABI
		abi, err := core.DecodeAbi(abiStrings[i])
		if err != nil {
			return fmt.Errorf("error decoding contract %s ABI: %w", string(contractNames[i]), err)
		}
		rp.instanceAbis[contractName] = abi
	}

	return nil
}

// Get a network contract
func (rp *RocketPool) GetContract(contractName ContractName) (*core.Contract, error) {
	contract, exists := rp.contracts[contractName]
	if !exists {
		return nil, fmt.Errorf("contract %s has not been loaded yet", string(contractName))
	}
	return contract, nil
}

// Get several network contracts
func (rp *RocketPool) GetContracts(contractNames ...ContractName) ([]*core.Contract, error) {
	contracts := make([]*core.Contract, len(contractNames))
	for i, contractName := range contractNames {
		contract, exists := rp.contracts[contractName]
		if !exists {
			return nil, fmt.Errorf("contract %s has not been loaded yet", string(contractName))
		}
		contracts[i] = contract
	}
	return contracts, nil
}

// Create a binding for a network contract instance
func (rp *RocketPool) MakeContract(contractName ContractName, address common.Address) (*core.Contract, error) {
	abi, exists := rp.instanceAbis[contractName]
	if !exists {
		return nil, fmt.Errorf("ABI for contract %s has not been loaded yet", string(contractName))
	}

	// Create and return
	return &core.Contract{
		Contract: bind.NewBoundContract(address, *abi, rp.Client, rp.Client, rp.Client),
		Address:  &address,
		ABI:      abi,
		Client:   rp.Client,
	}, nil
}

// =============
// === Utils ===
// =============

// Create a contract directly from its ABI, encoded in string form
func (rp *RocketPool) CreateMinipoolContractFromEncodedAbi(address common.Address, encodedAbi string) (*core.Contract, error) {
	// Decode ABI
	abi, err := core.DecodeAbi(encodedAbi)
	if err != nil {
		return nil, fmt.Errorf("Could not decode minipool %s ABI: %w", address, err)
	}

	// Create and return
	return &core.Contract{
		Contract: bind.NewBoundContract(address, *abi, rp.Client, rp.Client, rp.Client),
		Address:  &address,
		ABI:      abi,
		Client:   rp.Client,
	}, nil
}

// Create a contract directly from its ABI
func (rp *RocketPool) CreateMinipoolContractFromAbi(address common.Address, abi *abi.ABI) (*core.Contract, error) {
	// Create and return
	return &core.Contract{
		Contract: bind.NewBoundContract(address, *abi, rp.Client, rp.Client, rp.Client),
		Address:  &address,
		ABI:      abi,
		Client:   rp.Client,
	}, nil
}

// Signs and submits a transaction to the network
func (rp *RocketPool) SubmitTransaction(txInfo *core.TransactionInfo, opts *bind.TransactOpts) (*types.Transaction, error) {
	return core.ExecuteTransaction(rp.Client, txInfo.Data, txInfo.To, opts)
}

// Signs and submits a bundle of transactions to the network.
// NOTE: this assumes the bundle is meant to be submitted sequentially.
// If the nonce has been set in opts, this will use it for the first transaction and automatically increment it for each subsequent transaction.
func (rp *RocketPool) SubmitTransactions(txInfos []*core.TransactionInfo, opts *bind.TransactOpts) ([]*types.Transaction, error) {
	txs := make([]*types.Transaction, len(txInfos))
	one := big.NewInt(1)
	for i, txInfo := range txInfos {
		tx, err := core.ExecuteTransaction(rp.Client, txInfo.Data, txInfo.To, opts)
		if err != nil {
			return nil, fmt.Errorf("error creating transaction %d in bundle: %w", i, err)
		}
		txs[i] = tx
		if opts.Nonce != nil {
			// Increment the nonce for the next TX if it's explicitly set
			opts.Nonce.Add(opts.Nonce, one)
		}
	}
	return txs, nil
}

// =========================
// === Multicall Helpers ===
// =========================

// Run a multicall query that doesn't perform any return type allocation
func (rp *RocketPool) Query(query func(*batch.MultiCaller) error, opts *bind.CallOpts) error {
	// Create the multicaller
	mc, err := batch.NewMultiCaller(rp.Client, *rp.MulticallAddress)
	if err != nil {
		return fmt.Errorf("error creating multicaller: %w", err)
	}

	// Run the query
	err = query(mc)
	if err != nil {
		return fmt.Errorf("error running multicall query: %w", err)
	}

	// Execute the multicall
	_, err = mc.FlexibleCall(true, opts)
	if err != nil {
		return fmt.Errorf("error executing multicall: %w", err)
	}

	return nil
}

// Run a multicall query that doesn't perform any return type allocation
// Use this if one of the calls is allowed to fail without interrupting the others; the returned result array provides information about the success of each call.
func (rp *RocketPool) FlexQuery(query func(*batch.MultiCaller) error, opts *bind.CallOpts) ([]bool, error) {
	// Create the multicaller
	mc, err := batch.NewMultiCaller(rp.Client, *rp.MulticallAddress)
	if err != nil {
		return nil, fmt.Errorf("error creating multicaller: %w", err)
	}

	// Run the query
	err = query(mc)
	if err != nil {
		return nil, fmt.Errorf("error running multicall query: %w", err)
	}

	// Execute the multicall
	return mc.FlexibleCall(false, opts)
}

// Create and execute a multicall query that is too big for one call and must be run in batches
func (rp *RocketPool) BatchQuery(count int, batchSize int, query func(*batch.MultiCaller, int) error, opts *bind.CallOpts) error {
	// Sync
	var wg errgroup.Group
	wg.SetLimit(rp.ConcurrentCallLimit)

	// Run getters in batches
	for i := 0; i < count; i += batchSize {
		i := i
		max := i + batchSize
		if max > count {
			max = count
		}

		// Load details
		wg.Go(func() error {
			mc, err := batch.NewMultiCaller(rp.Client, *rp.MulticallAddress)
			if err != nil {
				return err
			}
			for j := i; j < max; j++ {
				err := query(mc, j)
				if err != nil {
					return fmt.Errorf("error running query adder: %w", err)
				}
			}
			_, err = mc.FlexibleCall(true, opts)
			if err != nil {
				return fmt.Errorf("error executing multicall: %w", err)
			}
			return nil
		})
	}

	// Wait for them all to complete
	if err := wg.Wait(); err != nil {
		return fmt.Errorf("error during multicall query: %w", err)
	}

	return nil
}

// Create and execute a multicall query that is too big for one call and must be run in batches.
// Use this if one of the calls is allowed to fail without interrupting the others; the returned result array provides information about the success of each call.
func (rp *RocketPool) FlexBatchQuery(count int, batchSize int, query func(*batch.MultiCaller, int) error, handleResult func(bool, int) error, opts *bind.CallOpts) error {
	// Sync
	var wg errgroup.Group
	wg.SetLimit(rp.ConcurrentCallLimit)

	// Run getters in batches
	for i := 0; i < count; i += batchSize {
		i := i
		max := i + batchSize
		if max > count {
			max = count
		}

		// Load details
		wg.Go(func() error {
			mc, err := batch.NewMultiCaller(rp.Client, *rp.MulticallAddress)
			if err != nil {
				return err
			}
			for j := i; j < max; j++ {
				err := query(mc, j)
				if err != nil {
					return fmt.Errorf("error running query adder: %w", err)
				}
			}
			results, err := mc.FlexibleCall(false, opts)
			if err != nil {
				return fmt.Errorf("error executing multicall: %w", err)
			}
			for j, result := range results {
				err = handleResult(result, j+i)
				if err != nil {
					return fmt.Errorf("error running query result handler: %w", err)
				}
			}

			return nil
		})
	}

	// Wait for them all to complete
	if err := wg.Wait(); err != nil {
		return fmt.Errorf("error during multicall query: %w", err)
	}

	// Return
	return nil
}
