package trustednode

import (
    "bytes"
    "fmt"
    "testing"

    "github.com/ethereum/go-ethereum/common"

    "github.com/rocket-pool/rocketpool-go/dao"
    trustednodedao "github.com/rocket-pool/rocketpool-go/dao/trustednode"
    "github.com/rocket-pool/rocketpool-go/node"
    "github.com/rocket-pool/rocketpool-go/rocketpool"
    trustednodesettings "github.com/rocket-pool/rocketpool-go/settings/trustednode"
    "github.com/rocket-pool/rocketpool-go/utils/eth"

    "github.com/rocket-pool/rocketpool-go/tests/testutils/accounts"
    daoutils "github.com/rocket-pool/rocketpool-go/tests/testutils/dao"
    "github.com/rocket-pool/rocketpool-go/tests/testutils/evm"
    nodeutils "github.com/rocket-pool/rocketpool-go/tests/testutils/node"
)


func TestProposeInviteMember(t *testing.T) {

    // State snapshotting
    if err := evm.TakeSnapshot(); err != nil { t.Fatal(err) }
    t.Cleanup(func() { if err := evm.RevertSnapshot(); err != nil { t.Fatal(err) } })

    // Set proposal cooldown
    if _, err := trustednodesettings.BootstrapProposalCooldown(rp, 0, ownerAccount.GetTransactor()); err != nil { t.Fatal(err) }

    // Register nodes
    if _, err := node.RegisterNode(rp, "Australia/Brisbane", nodeAccount.GetTransactor()); err != nil { t.Fatal(err) }
    if err := nodeutils.RegisterTrustedNode(rp, ownerAccount, trustedNodeAccount1); err != nil { t.Fatal(err) }

    // Submit, pass & execute invite member proposal
    proposalMemberAddress := nodeAccount.Address
    proposalMemberId := "coolguy"
    proposalMemberEmail := "coolguy@rocketpool.net"
    proposalId, _, err := trustednodedao.ProposeInviteMember(rp, "invite coolguy", proposalMemberAddress, proposalMemberId, proposalMemberEmail, trustedNodeAccount1.GetTransactor())
    if err != nil { t.Fatal(err) }
    if err := daoutils.PassAndExecuteProposal(rp, proposalId, []*accounts.Account{trustedNodeAccount1}); err != nil { t.Fatal(err) }

    // Get & check initial member exists status
    if exists, err := trustednodedao.GetMemberExists(rp, nodeAccount.Address, nil); err != nil {
        t.Error(err)
    } else if exists {
        t.Error("Incorrect initial member exists status")
    }

    // Mint trusted node RPL bond & join trusted node DAO
    if err := nodeutils.MintTrustedNodeBond(rp, ownerAccount, nodeAccount); err != nil { t.Fatal(err) }
    if _, err := trustednodedao.Join(rp, nodeAccount.GetTransactor()); err != nil {
        t.Fatal(err)
    }

    // Get & check updated member exists status
    if exists, err := trustednodedao.GetMemberExists(rp, nodeAccount.Address, nil); err != nil {
        t.Error(err)
    } else if !exists {
        t.Error("Incorrect updated member exists status")
    }

    // Get & check proposal payload string
    if payloadStr, err := dao.GetProposalPayloadStr(rp, proposalId, nil); err != nil {
        t.Error(err)
    } else if payloadStr != fmt.Sprintf("proposalInvite(%s,%s,%s)", proposalMemberId, proposalMemberEmail, proposalMemberAddress.Hex()) {
        t.Errorf("Incorrect proposal payload string %s", payloadStr)
    }

    // Get & check member invite executed block
    if inviteExecutedBlock, err := trustednodedao.GetMemberInviteProposalExecutedBlock(rp, proposalMemberAddress, nil); err != nil {
        t.Error(err)
    } else if inviteExecutedBlock == 0 {
        t.Errorf("Incorrect member invite proposal executed block %d", inviteExecutedBlock)
    }

}


func TestProposeMemberLeave(t *testing.T) {

    // State snapshotting
    if err := evm.TakeSnapshot(); err != nil { t.Fatal(err) }
    t.Cleanup(func() { if err := evm.RevertSnapshot(); err != nil { t.Fatal(err) } })

    // Set proposal cooldown
    if _, err := trustednodesettings.BootstrapProposalCooldown(rp, 0, ownerAccount.GetTransactor()); err != nil { t.Fatal(err) }

    // Register nodes
    if err := nodeutils.RegisterTrustedNode(rp, ownerAccount, trustedNodeAccount1); err != nil { t.Fatal(err) }
    if err := nodeutils.RegisterTrustedNode(rp, ownerAccount, trustedNodeAccount2); err != nil { t.Fatal(err) }
    if err := nodeutils.RegisterTrustedNode(rp, ownerAccount, trustedNodeAccount3); err != nil { t.Fatal(err) }
    if err := nodeutils.RegisterTrustedNode(rp, ownerAccount, trustedNodeAccount4); err != nil { t.Fatal(err) }

    // Submit, pass & execute member leave proposal
    proposalMemberAddress := trustedNodeAccount1.Address
    proposalId, _, err := trustednodedao.ProposeMemberLeave(rp, "node 1 leave", proposalMemberAddress, trustedNodeAccount1.GetTransactor())
    if err != nil { t.Fatal(err) }
    if err := daoutils.PassAndExecuteProposal(rp, proposalId, []*accounts.Account{
        trustedNodeAccount1,
        trustedNodeAccount2,
        trustedNodeAccount3,
        trustedNodeAccount4,
    }); err != nil { t.Fatal(err) }

    // Get & check initial member exists status
    if exists, err := trustednodedao.GetMemberExists(rp, trustedNodeAccount1.Address, nil); err != nil {
        t.Error(err)
    } else if !exists {
        t.Error("Incorrect initial member exists status")
    }

    // Leave trusted node DAO
    if _, err := trustednodedao.Leave(rp, trustedNodeAccount1.Address, trustedNodeAccount1.GetTransactor()); err != nil {
        t.Fatal(err)
    }

    // Get & check updated member exists status
    if exists, err := trustednodedao.GetMemberExists(rp, trustedNodeAccount1.Address, nil); err != nil {
        t.Error(err)
    } else if exists {
        t.Error("Incorrect updated member exists status")
    }

    // Get & check proposal payload string
    if payloadStr, err := dao.GetProposalPayloadStr(rp, proposalId, nil); err != nil {
        t.Error(err)
    } else if payloadStr != fmt.Sprintf("proposalLeave(%s)", proposalMemberAddress.Hex()) {
        t.Errorf("Incorrect proposal payload string %s", payloadStr)
    }

    // Get & check member leave executed block
    if leaveExecutedBlock, err := trustednodedao.GetMemberLeaveProposalExecutedBlock(rp, proposalMemberAddress, nil); err != nil {
        t.Error(err)
    } else if leaveExecutedBlock == 0 {
        t.Errorf("Incorrect member leave proposal executed block %d", leaveExecutedBlock)
    }

}


func TestProposeReplaceMember(t *testing.T) {

    // State snapshotting
    if err := evm.TakeSnapshot(); err != nil { t.Fatal(err) }
    t.Cleanup(func() { if err := evm.RevertSnapshot(); err != nil { t.Fatal(err) } })

    // Set proposal cooldown
    if _, err := trustednodesettings.BootstrapProposalCooldown(rp, 0, ownerAccount.GetTransactor()); err != nil { t.Fatal(err) }

    // Register nodes
    if _, err := node.RegisterNode(rp, "Australia/Brisbane", nodeAccount.GetTransactor()); err != nil { t.Fatal(err) }
    if err := nodeutils.RegisterTrustedNode(rp, ownerAccount, trustedNodeAccount1); err != nil { t.Fatal(err) }

    // Submit, pass & execute replace member proposal
    proposalOldMemberAddress := trustedNodeAccount1.Address
    proposalNewMemberAddress := nodeAccount.Address
    proposalNewMemberId := "coolguy"
    proposalNewMemberEmail := "coolguy@rocketpool.net"
    proposalId, _, err := trustednodedao.ProposeReplaceMember(rp, "replace node 1", proposalOldMemberAddress, proposalNewMemberAddress, proposalNewMemberId, proposalNewMemberEmail, trustedNodeAccount1.GetTransactor())
    if err != nil { t.Fatal(err) }
    if err := daoutils.PassAndExecuteProposal(rp, proposalId, []*accounts.Account{trustedNodeAccount1}); err != nil { t.Fatal(err) }

    // Get & check initial member exists statuses
    if oldMemberExists, err := trustednodedao.GetMemberExists(rp, trustedNodeAccount1.Address, nil); err != nil {
        t.Error(err)
    } else if !oldMemberExists {
        t.Error("Incorrect initial old member exists status")
    }
    if newMemberExists, err := trustednodedao.GetMemberExists(rp, nodeAccount.Address, nil); err != nil {
        t.Error(err)
    } else if newMemberExists {
        t.Error("Incorrect initial new member exists status")
    }

    // Replace position in trusted node DAO
    if _, err := trustednodedao.Replace(rp, trustedNodeAccount1.GetTransactor()); err != nil {
        t.Fatal(err)
    }

    // Get & check updated member exists status
    if oldMemberExists, err := trustednodedao.GetMemberExists(rp, trustedNodeAccount1.Address, nil); err != nil {
        t.Error(err)
    } else if oldMemberExists {
        t.Error("Incorrect updated old member exists status")
    }
    if newMemberExists, err := trustednodedao.GetMemberExists(rp, nodeAccount.Address, nil); err != nil {
        t.Error(err)
    } else if !newMemberExists {
        t.Error("Incorrect updated new member exists status")
    }

    // Get & check proposal payload string
    if payloadStr, err := dao.GetProposalPayloadStr(rp, proposalId, nil); err != nil {
        t.Error(err)
    } else if payloadStr != fmt.Sprintf("proposalReplace(%s,%s,%s,%s)", proposalOldMemberAddress.Hex(), proposalNewMemberId, proposalNewMemberEmail, proposalNewMemberAddress.Hex()) {
        t.Errorf("Incorrect proposal payload string %s", payloadStr)
    }

    // Get & check member replace executed block
    if replaceExecutedBlock, err := trustednodedao.GetMemberReplaceProposalExecutedBlock(rp, proposalOldMemberAddress, nil); err != nil {
        t.Error(err)
    } else if replaceExecutedBlock == 0 {
        t.Errorf("Incorrect member replace proposal executed block %d", replaceExecutedBlock)
    }

    // Get & check member replacement address
    if replacementAddress, err := trustednodedao.GetMemberReplacementAddress(rp, proposalOldMemberAddress, nil); err != nil {
        t.Error(err)
    } else if !bytes.Equal(replacementAddress.Bytes(), proposalNewMemberAddress.Bytes()) {
        t.Errorf("Incorrect member replacement address %s", replacementAddress.Hex())
    }

}


func TestProposeKickMember(t *testing.T) {

    // State snapshotting
    if err := evm.TakeSnapshot(); err != nil { t.Fatal(err) }
    t.Cleanup(func() { if err := evm.RevertSnapshot(); err != nil { t.Fatal(err) } })

    // Set proposal cooldown
    if _, err := trustednodesettings.BootstrapProposalCooldown(rp, 0, ownerAccount.GetTransactor()); err != nil { t.Fatal(err) }

    // Register nodes
    if err := nodeutils.RegisterTrustedNode(rp, ownerAccount, trustedNodeAccount1); err != nil { t.Fatal(err) }
    if err := nodeutils.RegisterTrustedNode(rp, ownerAccount, trustedNodeAccount2); err != nil { t.Fatal(err) }

    // Get & check initial member exists status
    if exists, err := trustednodedao.GetMemberExists(rp, trustedNodeAccount2.Address, nil); err != nil {
        t.Error(err)
    } else if !exists {
        t.Error("Incorrect initial member exists status")
    }

    // Submit, pass & execute kick member proposal
    proposalMemberAddress := trustedNodeAccount2.Address
    proposalFineAmount := eth.EthToWei(1000)
    proposalId, _, err := trustednodedao.ProposeKickMember(rp, "kick node 2", proposalMemberAddress, proposalFineAmount, trustedNodeAccount1.GetTransactor())
    if err != nil { t.Fatal(err) }
    if err := daoutils.PassAndExecuteProposal(rp, proposalId, []*accounts.Account{trustedNodeAccount1, trustedNodeAccount2}); err != nil { t.Fatal(err) }

    // Get & check updated member exists status
    if exists, err := trustednodedao.GetMemberExists(rp, trustedNodeAccount2.Address, nil); err != nil {
        t.Error(err)
    } else if exists {
        t.Error("Incorrect updated member exists status")
    }

    // Get & check proposal payload string
    if payloadStr, err := dao.GetProposalPayloadStr(rp, proposalId, nil); err != nil {
        t.Error(err)
    } else if payloadStr != fmt.Sprintf("proposalKick(%s,%s)", proposalMemberAddress.Hex(), proposalFineAmount.String()) {
        t.Errorf("Incorrect proposal payload string %s", payloadStr)
    }

}


func TestProposeUpgradeContract(t *testing.T) {

    // State snapshotting
    if err := evm.TakeSnapshot(); err != nil { t.Fatal(err) }
    t.Cleanup(func() { if err := evm.RevertSnapshot(); err != nil { t.Fatal(err) } })

    // Set proposal cooldown
    if _, err := trustednodesettings.BootstrapProposalCooldown(rp, 0, ownerAccount.GetTransactor()); err != nil { t.Fatal(err) }

    // Register node
    if err := nodeutils.RegisterTrustedNode(rp, ownerAccount, trustedNodeAccount1); err != nil { t.Fatal(err) }

    // Submit, pass & execute upgrade contract proposal
    proposalUpgradeType := "upgradeContract"
    proposalContractName := "rocketDepositPool"
    proposalContractAddress := common.HexToAddress("0x1111111111111111111111111111111111111111")
    proposalContractAbi := "[{\"name\":\"foo\",\"type\":\"function\",\"inputs\":[],\"outputs\":[]}]"
    proposalId, _, err := trustednodedao.ProposeUpgradeContract(rp, "upgrade rocketDepositPool", proposalUpgradeType, proposalContractName, proposalContractAbi, proposalContractAddress, trustedNodeAccount1.GetTransactor())
    if err != nil { t.Fatal(err) }
    if err := daoutils.PassAndExecuteProposal(rp, proposalId, []*accounts.Account{trustedNodeAccount1, trustedNodeAccount2}); err != nil { t.Fatal(err) }

    // Get & check updated contract details
    if contractAddress, err := rp.GetAddress(proposalContractName); err != nil {
        t.Error(err)
    } else if !bytes.Equal(contractAddress.Bytes(), proposalContractAddress.Bytes()) {
        t.Errorf("Incorrect updated contract address %s", contractAddress.Hex())
    }
    if contractAbi, err := rp.GetABI(proposalContractName); err != nil {
        t.Error(err)
    } else if _, ok := contractAbi.Methods["foo"]; !ok {
        t.Errorf("Incorrect updated contract ABI")
    }

    // Get & check proposal payload string
    if payloadStr, err := dao.GetProposalPayloadStr(rp, proposalId, nil); err != nil {
        t.Error(err)
    } else if encodedAbi, err := rocketpool.EncodeAbiStr(proposalContractAbi); err != nil {
        t.Error(err)
    } else if payloadStr != fmt.Sprintf("proposalUpgrade(%s,%s,%s,%s)", proposalUpgradeType, proposalContractName, encodedAbi, proposalContractAddress.Hex()) {
        t.Errorf("Incorrect proposal payload string %s", payloadStr)
    }

}
