package oracle

import (
	"context"
	"fmt"
	"math"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/rocket-pool/rocketpool-go/dao/oracle"
	"github.com/rocket-pool/rocketpool-go/dao/protocol"
	"github.com/rocket-pool/rocketpool-go/network"
	"github.com/rocket-pool/rocketpool-go/rocketpool"
	"gonum.org/v1/gonum/mathext"
)

// ===============
// === Structs ===
// ===============

type TrustedNodeParticipationCalculator struct {
	rp      *rocketpool.RocketPool
	odaoMgr *oracle.OracleDaoManager
	pds     *protocol.ProtocolDaoSettings
	nm      *network.NetworkManager
}

// The results of the trusted node participation calculation
type TrustedNodeParticipation struct {
	StartBlock          uint64
	UpdateFrequency     uint64
	UpdateCount         uint64
	Probability         float64
	ExpectedSubmissions float64
	ActualSubmissions   map[common.Address]float64
	Participation       map[common.Address][]bool
}

// ====================
// === Constructors ===
// ====================

// Creates a new TrustedNodeParticipationCalculator
func NewTrustedNodeParticipationCalculator(rp *rocketpool.RocketPool) (*TrustedNodeParticipationCalculator, error) {
	odaoMgr, err := oracle.NewOracleDaoManager(rp)
	if err != nil {
		return nil, fmt.Errorf("error getting oDAO manager binding: %w", err)
	}
	pdaoMgr, err := protocol.NewProtocolDaoManager(rp)
	if err != nil {
		return nil, fmt.Errorf("error getting Protocol DAO manager binding: %w", err)
	}
	pds := pdaoMgr.Settings
	nm, err := network.NewNetworkManager(rp)
	if err != nil {
		return nil, fmt.Errorf("error getting NetworkBalances binding: %w", err)
	}

	return &TrustedNodeParticipationCalculator{
		rp:      rp,
		odaoMgr: odaoMgr,
		pds:     pds,
		nm:      nm,
	}, nil
}

// =============
// === Utils ===
// =============

// Calculates the participation rate of every trusted node on price submission since the last block that member count changed
func (c *TrustedNodeParticipationCalculator) CalculateTrustedNodePricesParticipation(intervalSize *big.Int, opts *bind.CallOpts) (*TrustedNodeParticipation, error) {
	// Create an opts with the current block if not specified
	if opts == nil {
		currentBlockNumber, err := c.rp.Client.BlockNumber(context.Background())
		if err != nil {
			return nil, fmt.Errorf("error getting current block number: %w", err)
		}

		opts = &bind.CallOpts{
			BlockNumber: big.NewInt(0).SetUint64(currentBlockNumber),
		}
	}
	blockNumber := opts.BlockNumber.Uint64()

	// Get the price frequency and member count
	err := c.rp.Query(nil, opts,
		c.pds.Network.SubmitPricesFrequency,
		c.odaoMgr.MemberCount,
	)
	if err != nil {
		return nil, fmt.Errorf("error during initial parameter update: %w", err)
	}
	updatePricesFrequency := c.pds.Network.SubmitPricesFrequency.Formatted()
	memberCount := c.odaoMgr.MemberCount.Formatted()

	// Get the block of the most recent member join (limiting to 50 intervals)
	minBlock := (blockNumber/updatePricesFrequency - 50) * updatePricesFrequency
	latestMemberCountChangedBlock, err := c.odaoMgr.GetLatestMemberCountChangedBlock(minBlock, intervalSize, opts)
	if err != nil {
		return nil, err
	}

	// Start block is the first interval after the latest join
	startBlock := (latestMemberCountChangedBlock/updatePricesFrequency + 1) * updatePricesFrequency
	// The number of members that have to submit each interval
	consensus := math.Floor(float64(memberCount)/2 + 1)

	// Check if any intervals have passed
	intervalsPassed := uint64(0)
	if blockNumber > startBlock {
		// The number of intervals passed
		intervalsPassed = (blockNumber-startBlock)/updatePricesFrequency + 1
	}

	// How many submissions would we expect per member given a random submission
	expected := float64(intervalsPassed) * consensus / float64(memberCount)

	// Get trusted members
	members, err := c.odaoMgr.GetMemberAddresses(memberCount, opts)
	if err != nil {
		return nil, fmt.Errorf("error getting Oracle DAO member addresses: %w", err)
	}

	// Construct the epoch map
	participationTable := make(map[common.Address][]bool)

	// Iterate members and sum chi-square
	submissions := make(map[common.Address]float64)
	chi := float64(0)
	for _, member := range members {
		participationTable[member] = make([]bool, intervalsPassed)
		actual := 0
		if intervalsPassed > 0 {
			blocks, err := c.nm.GetPricesSubmissions(member, startBlock, intervalSize, opts)
			if err != nil {
				return nil, err
			}
			actual = len(*blocks)
			delta := float64(actual) - expected
			chi += (delta * delta) / expected

			// Add to participation table
			for _, block := range *blocks {
				// Ignore out of step updates
				if block%updatePricesFrequency == 0 {
					index := block/updatePricesFrequency - startBlock/updatePricesFrequency
					participationTable[member][index] = true
				}
			}
		}

		// Save actual submission
		submissions[member] = float64(actual)
	}

	// Calculate inverse cumulative density function with members-1 DoF
	probability := float64(1)
	if intervalsPassed > 0 {
		probability = 1 - mathext.GammaIncReg(float64(len(members)-1)/2, chi/2)
	}

	// Construct return value
	participation := TrustedNodeParticipation{
		Probability:         probability,
		ExpectedSubmissions: expected,
		ActualSubmissions:   submissions,
		StartBlock:          startBlock,
		UpdateFrequency:     updatePricesFrequency,
		UpdateCount:         intervalsPassed,
		Participation:       participationTable,
	}
	return &participation, nil
}

// Calculates the participation rate of every trusted node on balance submission since the last block that member count changed
func (c *TrustedNodeParticipationCalculator) CalculateTrustedNodeBalancesParticipation(intervalSize *big.Int, opts *bind.CallOpts) (*TrustedNodeParticipation, error) {
	// Create an opts with the current block if not specified
	if opts == nil {
		currentBlockNumber, err := c.rp.Client.BlockNumber(context.Background())
		if err != nil {
			return nil, fmt.Errorf("error getting current block number: %w", err)
		}

		opts = &bind.CallOpts{
			BlockNumber: big.NewInt(0).SetUint64(currentBlockNumber),
		}
	}
	blockNumber := opts.BlockNumber.Uint64()

	// Get the balance frequency and member count
	err := c.rp.Query(nil, opts, c.pds.Network.SubmitBalancesFrequency,
		c.odaoMgr.MemberCount,
	)
	if err != nil {
		return nil, fmt.Errorf("error during initial parameter update: %w", err)
	}
	updateBalancesFrequency := c.pds.Network.SubmitBalancesFrequency.Formatted()
	memberCount := c.odaoMgr.MemberCount.Formatted()

	// Get the block of the most recent member join (limiting to 50 intervals)
	minBlock := (blockNumber/updateBalancesFrequency - 50) * updateBalancesFrequency
	latestMemberCountChangedBlock, err := c.odaoMgr.GetLatestMemberCountChangedBlock(minBlock, intervalSize, opts)
	if err != nil {
		return nil, err
	}

	// Start block is the first interval after the latest join
	startBlock := (latestMemberCountChangedBlock/updateBalancesFrequency + 1) * updateBalancesFrequency

	// The number of members that have to submit each interval
	consensus := math.Floor(float64(memberCount)/2 + 1)

	// Check if any intervals have passed
	intervalsPassed := uint64(0)
	if blockNumber > startBlock {
		// The number of intervals passed
		intervalsPassed = (blockNumber-startBlock)/updateBalancesFrequency + 1
	}

	// How many submissions would we expect per member given a random submission
	expected := float64(intervalsPassed) * consensus / float64(memberCount)

	// Get trusted members
	members, err := c.odaoMgr.GetMemberAddresses(memberCount, opts)
	if err != nil {
		return nil, fmt.Errorf("error getting Oracle DAO member addresses: %w", err)
	}

	// Construct the epoch map
	participationTable := make(map[common.Address][]bool)

	// Iterate members and sum chi-square
	submissions := make(map[common.Address]float64)
	chi := float64(0)
	for _, member := range members {
		participationTable[member] = make([]bool, intervalsPassed)
		actual := 0
		if intervalsPassed > 0 {
			blocks, err := c.nm.GetBalancesSubmissions(member, startBlock, intervalSize, opts)
			if err != nil {
				return nil, err
			}
			actual = len(*blocks)
			delta := float64(actual) - expected
			chi += (delta * delta) / expected

			// Add to participation table
			for _, block := range *blocks {
				// Ignore out of step updates
				if block%updateBalancesFrequency == 0 {
					index := block/updateBalancesFrequency - startBlock/updateBalancesFrequency
					participationTable[member][index] = true
				}
			}
		}

		// Save actual submission
		submissions[member] = float64(actual)
	}

	// Calculate inverse cumulative density function with members-1 DoF
	probability := float64(1)
	if intervalsPassed > 0 {
		probability = 1 - mathext.GammaIncReg(float64(len(members)-1)/2, chi/2)
	}

	// Construct return value
	participation := TrustedNodeParticipation{
		Probability:         probability,
		ExpectedSubmissions: expected,
		ActualSubmissions:   submissions,
		StartBlock:          startBlock,
		UpdateFrequency:     updateBalancesFrequency,
		UpdateCount:         intervalsPassed,
		Participation:       participationTable,
	}
	return &participation, nil
}

// Returns a mapping of members and whether they have submitted balances this interval or not
func (c *TrustedNodeParticipationCalculator) GetTrustedNodeLatestBalancesParticipation(rp *rocketpool.RocketPool, intervalSize *big.Int, opts *bind.CallOpts) (map[common.Address]bool, error) {
	// Create an opts with the current block if not specified
	if opts == nil {
		currentBlockNumber, err := c.rp.Client.BlockNumber(context.Background())
		if err != nil {
			return nil, fmt.Errorf("error getting current block number: %w", err)
		}

		opts = &bind.CallOpts{
			BlockNumber: big.NewInt(0).SetUint64(currentBlockNumber),
		}
	}
	blockNumber := opts.BlockNumber.Uint64()

	// Get the price frequency and member count
	err := c.rp.Query(nil, opts,
		c.pds.Network.SubmitBalancesFrequency,
		c.odaoMgr.MemberCount,
	)
	if err != nil {
		return nil, fmt.Errorf("error during initial parameter update: %w", err)
	}
	updateBalancesFrequency := c.pds.Network.SubmitBalancesFrequency.Formatted()
	memberCount := c.odaoMgr.MemberCount.Formatted()

	// Get trusted members
	members, err := c.odaoMgr.GetMemberAddresses(memberCount, opts)
	if err != nil {
		return nil, fmt.Errorf("error getting Oracle DAO member addresses: %w", err)
	}

	// Get submission within the current interval
	fromBlock := blockNumber / updateBalancesFrequency * updateBalancesFrequency
	submissions, err := c.nm.GetLatestBalancesSubmissions(fromBlock, intervalSize, opts)
	if err != nil {
		return nil, err
	}

	// Build and return result table
	participationTable := make(map[common.Address]bool)
	for _, member := range members {
		participationTable[member] = false
	}
	for _, submission := range submissions {
		participationTable[submission] = true
	}
	return participationTable, nil
}

// Returns a mapping of members and whether they have submitted prices this interval or not
func (c *TrustedNodeParticipationCalculator) GetTrustedNodeLatestPricesParticipation(rp *rocketpool.RocketPool, intervalSize *big.Int, opts *bind.CallOpts) (map[common.Address]bool, error) {
	// Create an opts with the current block if not specified
	if opts == nil {
		currentBlockNumber, err := c.rp.Client.BlockNumber(context.Background())
		if err != nil {
			return nil, fmt.Errorf("error getting current block number: %w", err)
		}

		opts = &bind.CallOpts{
			BlockNumber: big.NewInt(0).SetUint64(currentBlockNumber),
		}
	}
	blockNumber := opts.BlockNumber.Uint64()

	// Get the price frequency and member count
	err := c.rp.Query(nil, opts,
		c.pds.Network.SubmitPricesFrequency,
		c.odaoMgr.MemberCount,
	)
	if err != nil {
		return nil, fmt.Errorf("error during initial parameter update: %w", err)
	}
	updatePricesFrequency := c.pds.Network.SubmitPricesFrequency.Formatted()
	memberCount := c.odaoMgr.MemberCount.Formatted()

	// Get trusted members
	members, err := c.odaoMgr.GetMemberAddresses(memberCount, opts)
	if err != nil {
		return nil, fmt.Errorf("error getting Oracle DAO member addresses: %w", err)
	}

	// Get submission within the current interval
	fromBlock := blockNumber / updatePricesFrequency * updatePricesFrequency
	submissions, err := c.nm.GetLatestPricesSubmissions(fromBlock, intervalSize, opts)
	if err != nil {
		return nil, err
	}

	// Build and return result table
	participationTable := make(map[common.Address]bool)
	for _, member := range members {
		participationTable[member] = false
	}
	for _, submission := range submissions {
		participationTable[submission] = true
	}
	return participationTable, nil
}