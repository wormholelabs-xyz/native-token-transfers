// SPDX-License-Identifier: Apache 2
pragma solidity >=0.8.8 <0.9.0;

import "forge-std/Test.sol";
import "forge-std/console.sol";

import "../src/NttManager/NttManagerNoRateLimiting.sol";
import "../src/Transceiver/Transceiver.sol";
import "../src/interfaces/INttManager.sol";
import "../src/interfaces/IRateLimiter.sol";
import "../src/interfaces/ITransceiver.sol";
import "../src/interfaces/IManagerBase.sol";
import "../src/interfaces/IRateLimiterEvents.sol";
import "../src/NttManager/TransceiverRegistry.sol";
import {Utils} from "./libraries/Utils.sol";
import {DummyToken, DummyTokenMintAndBurn} from "./NttManager.t.sol";
import "../src/interfaces/IWormholeTransceiver.sol";
import {WormholeTransceiver} from "../src/Transceiver/WormholeTransceiver/WormholeTransceiver.sol";
import "../src/libraries/TransceiverStructs.sol";
import "./mocks/MockNttManager.sol";
import "./mocks/MockTransceivers.sol";

import "openzeppelin-contracts/contracts/token/ERC20/ERC20.sol";
import "openzeppelin-contracts/contracts/proxy/ERC1967/ERC1967Proxy.sol";
import "wormhole-solidity-sdk/interfaces/IWormhole.sol";
import "wormhole-solidity-sdk/testing/helpers/WormholeSimulator.sol";
import "wormhole-solidity-sdk/Utils.sol";
//import "wormhole-solidity-sdk/testing/WormholeRelayerTest.sol";

contract TestPerChainTransceivers is Test, IRateLimiterEvents {
    MockNttManagerContract nttManagerChain1;
    NttManager nttManagerChain2;
    NttManager nttManagerChain3;

    using TrimmedAmountLib for uint256;
    using TrimmedAmountLib for TrimmedAmount;

    uint16 constant chainId1 = 7;
    uint16 constant chainId2 = 100;
    uint16 constant chainId3 = 101;
    uint8 constant FAST_CONSISTENCY_LEVEL = 200;
    uint256 constant GAS_LIMIT = 500000;

    uint16 constant SENDING_CHAIN_ID = 1;
    uint256 constant DEVNET_GUARDIAN_PK =
        0xcfb12303a19cde580bb4dd771639b0d26bc68353645571a8cff516ab2ee113a0;
    WormholeSimulator guardian;
    uint256 initialBlockTimestamp;

    WormholeTransceiver wormholeTransceiverChain1;
    WormholeTransceiver secondWormholeTransceiverChain1;
    WormholeTransceiver wormholeTransceiverChain2;
    WormholeTransceiver secondWormholeTransceiverChain2;
    WormholeTransceiver wormholeTransceiverChain3;
    WormholeTransceiver secondWormholeTransceiverChain3;
    address userA = address(0x123);
    address userB = address(0x456);
    address userC = address(0x789);
    address userD = address(0xABC);

    address relayer = address(0x28D8F1Be96f97C1387e94A53e00eCcFb4E75175a);
    IWormhole wormhole = IWormhole(0x4a8bc80Ed5a4067f1CCf107057b8270E0cC11A78);

    // This function sets up the following config:
    // - A manager on each of three chains.
    // - Two transceivers on each chain, all interconnected as peers.
    // - On chain one, it sets a default threshold of one and a per-chain threshold of two for chain three.
    // - On chain three, it sets a default threshold of one and a per-chain threshold of two for chain one.

    function setUp() public {
        string memory url = "https://ethereum-sepolia-rpc.publicnode.com";
        vm.createSelectFork(url);
        initialBlockTimestamp = vm.getBlockTimestamp();

        guardian = new WormholeSimulator(address(wormhole), DEVNET_GUARDIAN_PK);

        vm.chainId(chainId1);
        DummyToken t1 = new DummyToken();
        NttManager implementation = new MockNttManagerContract(
            address(t1), IManagerBase.Mode.LOCKING, chainId1, 1 days, false
        );

        nttManagerChain1 =
            MockNttManagerContract(address(new ERC1967Proxy(address(implementation), "")));
        nttManagerChain1.initialize();

        // Create the first transceiver, from chain 1 to chain 2.
        WormholeTransceiver wormholeTransceiverChain1Implementation = new MockWormholeTransceiverContract(
            address(nttManagerChain1),
            address(wormhole),
            address(relayer),
            address(0x0),
            FAST_CONSISTENCY_LEVEL,
            GAS_LIMIT
        );
        wormholeTransceiverChain1 = MockWormholeTransceiverContract(
            address(new ERC1967Proxy(address(wormholeTransceiverChain1Implementation), ""))
        );

        // Only the deployer should be able to initialize
        vm.prank(userA);
        vm.expectRevert(
            abi.encodeWithSelector(ITransceiver.UnexpectedDeployer.selector, address(this), userA)
        );
        wormholeTransceiverChain1.initialize();

        // Actually initialize properly now
        wormholeTransceiverChain1.initialize();

        nttManagerChain1.setTransceiver(address(wormholeTransceiverChain1));

        // Create the second transceiver for chain 1.
        WormholeTransceiver secondWormholeTransceiverChain1Implementation = new MockWormholeTransceiverContract(
            address(nttManagerChain1),
            address(wormhole),
            address(relayer),
            address(0x0),
            FAST_CONSISTENCY_LEVEL,
            GAS_LIMIT
        );
        secondWormholeTransceiverChain1 = MockWormholeTransceiverContract(
            address(new ERC1967Proxy(address(secondWormholeTransceiverChain1Implementation), ""))
        );

        secondWormholeTransceiverChain1.initialize();
        nttManagerChain1.setTransceiver(address(secondWormholeTransceiverChain1));

        // Chain 2 setup
        vm.chainId(chainId2);
        DummyToken t2 = new DummyTokenMintAndBurn();
        NttManager implementationChain2 = new MockNttManagerContract(
            address(t2), IManagerBase.Mode.BURNING, chainId2, 1 days, false
        );

        nttManagerChain2 =
            MockNttManagerContract(address(new ERC1967Proxy(address(implementationChain2), "")));
        nttManagerChain2.initialize();

        WormholeTransceiver wormholeTransceiverChain2Implementation = new MockWormholeTransceiverContract(
            address(nttManagerChain2),
            address(wormhole),
            address(relayer),
            address(0x0),
            FAST_CONSISTENCY_LEVEL,
            GAS_LIMIT
        );
        wormholeTransceiverChain2 = MockWormholeTransceiverContract(
            address(new ERC1967Proxy(address(wormholeTransceiverChain2Implementation), ""))
        );
        wormholeTransceiverChain2.initialize();

        nttManagerChain2.setTransceiver(address(wormholeTransceiverChain2));

        // Register peer contracts for the nttManager and transceiver. Transceivers and nttManager each have the concept of peers here.
        nttManagerChain1.setPeer(
            chainId2, bytes32(uint256(uint160(address(nttManagerChain2)))), 9, type(uint64).max
        );
        nttManagerChain2.setPeer(
            chainId1, bytes32(uint256(uint160(address(nttManagerChain1)))), 7, type(uint64).max
        );

        // Create the second transceiver for chain 2.
        WormholeTransceiver secondWormholeTransceiverChain2Implementation = new MockWormholeTransceiverContract(
            address(nttManagerChain2),
            address(wormhole),
            address(relayer),
            address(0x0),
            FAST_CONSISTENCY_LEVEL,
            GAS_LIMIT
        );
        secondWormholeTransceiverChain2 = MockWormholeTransceiverContract(
            address(new ERC1967Proxy(address(secondWormholeTransceiverChain2Implementation), ""))
        );

        secondWormholeTransceiverChain2.initialize();
        nttManagerChain2.setTransceiver(address(secondWormholeTransceiverChain2));

        // Set peers for the transceivers
        wormholeTransceiverChain1.setWormholePeer(
            chainId2, bytes32(uint256(uint160(address(wormholeTransceiverChain2))))
        );
        wormholeTransceiverChain2.setWormholePeer(
            chainId1, bytes32(uint256(uint160(address(wormholeTransceiverChain1))))
        );
        secondWormholeTransceiverChain1.setWormholePeer(
            chainId2, bytes32(uint256(uint160(address(secondWormholeTransceiverChain2))))
        );
        secondWormholeTransceiverChain2.setWormholePeer(
            chainId1, bytes32(uint256(uint160(address(secondWormholeTransceiverChain1))))
        );

        // Chain 3 setup
        vm.chainId(chainId3);
        DummyToken t3 = new DummyTokenMintAndBurn();
        NttManager implementationChain3 = new MockNttManagerContract(
            address(t3), IManagerBase.Mode.BURNING, chainId3, 1 days, false
        );

        nttManagerChain3 =
            MockNttManagerContract(address(new ERC1967Proxy(address(implementationChain3), "")));
        nttManagerChain3.initialize();

        WormholeTransceiver wormholeTransceiverChain3Implementation = new MockWormholeTransceiverContract(
            address(nttManagerChain3),
            address(wormhole),
            address(relayer),
            address(0x0),
            FAST_CONSISTENCY_LEVEL,
            GAS_LIMIT
        );
        wormholeTransceiverChain3 = MockWormholeTransceiverContract(
            address(new ERC1967Proxy(address(wormholeTransceiverChain3Implementation), ""))
        );
        wormholeTransceiverChain3.initialize();

        nttManagerChain3.setTransceiver(address(wormholeTransceiverChain3));

        // Register peer contracts for the nttManager and transceiver. Transceivers and nttManager each have the concept of peers here.
        nttManagerChain1.setPeer(
            chainId3, bytes32(uint256(uint160(address(nttManagerChain3)))), 9, type(uint64).max
        );
        nttManagerChain3.setPeer(
            chainId1, bytes32(uint256(uint160(address(nttManagerChain1)))), 7, type(uint64).max
        );

        // Create the second transceiver, from chain 3 to chain 1.
        WormholeTransceiver secondWormholeTransceiverChain3Implementation = new MockWormholeTransceiverContract(
            address(nttManagerChain3),
            address(wormhole),
            address(relayer),
            address(0x0),
            FAST_CONSISTENCY_LEVEL,
            GAS_LIMIT
        );
        secondWormholeTransceiverChain3 = MockWormholeTransceiverContract(
            address(new ERC1967Proxy(address(secondWormholeTransceiverChain3Implementation), ""))
        );

        // Actually initialize properly now
        secondWormholeTransceiverChain3.initialize();

        nttManagerChain3.setTransceiver(address(secondWormholeTransceiverChain3));

        // Set peers for the transceivers
        wormholeTransceiverChain1.setWormholePeer(
            chainId3, bytes32(uint256(uint160(address(wormholeTransceiverChain3))))
        );
        wormholeTransceiverChain3.setWormholePeer(
            chainId1, bytes32(uint256(uint160(address(wormholeTransceiverChain1))))
        );
        wormholeTransceiverChain3.setWormholePeer(
            chainId2, bytes32(uint256(uint160(address(wormholeTransceiverChain2))))
        );
        wormholeTransceiverChain2.setWormholePeer(
            chainId3, bytes32(uint256(uint160(address(wormholeTransceiverChain3))))
        );
        secondWormholeTransceiverChain1.setWormholePeer(
            chainId3, bytes32(uint256(uint160(address(secondWormholeTransceiverChain3))))
        );
        secondWormholeTransceiverChain3.setWormholePeer(
            chainId1, bytes32(uint256(uint160(address(secondWormholeTransceiverChain1))))
        );
        secondWormholeTransceiverChain2.setWormholePeer(
            chainId3, bytes32(uint256(uint160(address(secondWormholeTransceiverChain3))))
        );
        secondWormholeTransceiverChain3.setWormholePeer(
            chainId2, bytes32(uint256(uint160(address(secondWormholeTransceiverChain2))))
        );
    }

    function test_setUp() public {
        address[] memory transceivers = nttManagerChain1.getTransceivers();
        require(transceivers.length == 2, "Invalid number of transceivers");
        require(transceivers[0] == address(wormholeTransceiverChain1), "Transceiver one is invalid");
        require(
            transceivers[1] == address(secondWormholeTransceiverChain1),
            "Transceiver two is invalid"
        );

        TransceiverRegistry.TransceiverInfo[] memory info = nttManagerChain1.getTransceiverInfo();
        require(info.length == 2, "Invalid number of transceiver infos");
    }

    function test_transceiverSetters() public {
        // Make sure nothing is enabled for either sending or receiving.
        require(
            nttManagerChain1.getChainsEnabledForSending().length == 0,
            "There should be no chains enabled for sending to start with"
        );
        require(
            nttManagerChain1.getChainsEnabledForReceiving().length == 0,
            "There should be no chains enabled for receiving to start with"
        );

        // Chain 2
        require(
            nttManagerChain1.getEnabledSendTransceiversForChain(chainId2).length == 0,
            "There should be nothing enabled for sending on chain two to start with"
        );
        require(
            nttManagerChain1.getEnabledRecvTransceiversBitmapForChain(chainId2) == 0,
            "There should be nothing enabled for receiving on chain two to start with"
        );

        // Chain 3
        require(
            nttManagerChain1.getEnabledSendTransceiversForChain(chainId3).length == 0,
            "There should be nothing enabled for sending on chain three to start with"
        );
        require(
            nttManagerChain1.getEnabledRecvTransceiversBitmapForChain(chainId3) == 0,
            "There should be nothing enabled for receiving on chain three to start with"
        );

        // Enable a sender on chain two.
        nttManagerChain1.enableSendTransceiverForChain(chainId2, address(wormholeTransceiverChain1));
        address[] memory sendTrans = nttManagerChain1.getEnabledSendTransceiversForChain(chainId2);
        require(sendTrans.length == 1, "Sending chains length is wrong for chain two #1");
        require(
            sendTrans[0] == address(wormholeTransceiverChain1),
            "Sending chains is wrong for chain two #1"
        );
        uint16[] memory sendChains = nttManagerChain1.getChainsEnabledForSending();
        require(sendChains.length == 1, "There should be one chain enabled for sending");
        require(sendChains[0] == chainId2, "Chain two should be enabled for sending");

        // Enable a receiver on chain two.
        nttManagerChain1.enableRecvTransceiverForChain(chainId2, address(wormholeTransceiverChain1));
        require(
            nttManagerChain1.getEnabledRecvTransceiversBitmapForChain(chainId2) == 0x01,
            "Receiving bitmap is wrong for chain two #1"
        );
        uint16[] memory recvChains = nttManagerChain1.getChainsEnabledForReceiving();
        require(recvChains.length == 1, "There should be one chain enabled for receiving");
        require(recvChains[0] == chainId2, "Chain two should be enabled for receiving #1");

        // Enable a sender on chain three.
        nttManagerChain1.enableSendTransceiverForChain(chainId3, address(wormholeTransceiverChain1));
        sendTrans = nttManagerChain1.getEnabledSendTransceiversForChain(chainId3);
        require(sendTrans.length == 1, "Sending chains length is wrong for chain three #1");
        require(
            sendTrans[0] == address(wormholeTransceiverChain1),
            "Sending chains is wrong for chain three #1"
        );
        sendChains = nttManagerChain1.getChainsEnabledForSending();
        require(sendChains.length == 2, "There should be two chain enabled for sending");
        require(sendChains[0] == chainId2, "Chain two should be enabled for sending");
        require(sendChains[1] == chainId3, "Chain three should be enabled for sending");

        // Enable a receiver on chain three.
        nttManagerChain1.enableRecvTransceiverForChain(
            chainId3, address(secondWormholeTransceiverChain1)
        );
        require(
            nttManagerChain1.getEnabledRecvTransceiversBitmapForChain(chainId3) == 0x02,
            "Receiving bitmap is wrong for chain three #1"
        );
        recvChains = nttManagerChain1.getChainsEnabledForReceiving();
        require(recvChains.length == 2, "There should be two chains enabled for receiving");
        require(recvChains[0] == chainId2, "Chain two should be enabled for receiving #2");
        require(recvChains[1] == chainId3, "Chain three should be enabled for receiving #1");
        require(
            nttManagerChain1.getThreshold(chainId3) == 1, "Threshold is wrong for chain three #1"
        );

        // Enable two receivers on chain two. That should not change the threshold.
        nttManagerChain1.enableRecvTransceiverForChain(
            chainId2, address(secondWormholeTransceiverChain1)
        );
        require(
            nttManagerChain1.getEnabledRecvTransceiversBitmapForChain(chainId2) == 0x03,
            "Receiving bitmap is wrong for chain two #2"
        );
        recvChains = nttManagerChain1.getChainsEnabledForReceiving();
        require(recvChains.length == 2, "There should be two chains enabled for receiving");
        require(recvChains[0] == chainId2, "Chain two should be enabled for receiving #3");
        require(recvChains[1] == chainId3, "Chain three should be enabled for receiving #2");
        require(nttManagerChain1.getThreshold(chainId2) == 1, "Threshold is wrong for chain two #2");

        nttManagerChain1.setThreshold(chainId2, 2);
        require(nttManagerChain1.getThreshold(chainId2) == 2, "Threshold is wrong for chain two #3");

        // Disable one receiver on chain two. This should reduce the threshold.
        nttManagerChain1.disableRecvTransceiverForChain(
            chainId2, address(wormholeTransceiverChain1)
        );
        require(
            nttManagerChain1.getEnabledRecvTransceiversBitmapForChain(chainId2) == 0x02,
            "Receiving bitmap is wrong for chain two #3"
        );
        recvChains = nttManagerChain1.getChainsEnabledForReceiving();
        require(recvChains.length == 2, "There should be two chains enabled for receiving");
        require(recvChains[0] == chainId2, "Chain two should be enabled for receiving #4");
        require(recvChains[1] == chainId3, "Chain three should be enabled for receiving #3");
        require(nttManagerChain1.getThreshold(chainId2) == 1, "Threshold is wrong for chain two #4");

        // Disable the other receiver on chain two.
        nttManagerChain1.disableRecvTransceiverForChain(
            chainId2, address(secondWormholeTransceiverChain1)
        );
        require(
            nttManagerChain1.getEnabledRecvTransceiversBitmapForChain(chainId2) == 0x00,
            "Receiving bitmap is wrong for chain two #4"
        );
        recvChains = nttManagerChain1.getChainsEnabledForReceiving();
        require(recvChains.length == 1, "There should be only one chain enabled for receiving");
        require(recvChains[0] == chainId3, "Chain three should be enabled for receiving #5");
        require(nttManagerChain1.getThreshold(chainId2) == 0, "Threshold is wrong for chain two #4");

        // Disable one receiver on chain three.
        nttManagerChain1.disableRecvTransceiverForChain(
            chainId3, address(secondWormholeTransceiverChain1)
        );
        require(
            nttManagerChain1.getEnabledRecvTransceiversBitmapForChain(chainId3) == 0x00,
            "Receiving bitmap is wrong for chain three #3"
        );
        recvChains = nttManagerChain1.getChainsEnabledForReceiving();
        require(recvChains.length == 0, "There should be one chain enabled for receiving");
        require(
            nttManagerChain1.getThreshold(chainId3) == 0, "Threshold is wrong for chain three #3"
        );

        // Make sure our senders haven't changed on either chain.
        sendTrans = nttManagerChain1.getEnabledSendTransceiversForChain(chainId2);
        require(sendTrans.length == 1, "Sending chains length is wrong for chain two #2");
        require(
            sendTrans[0] == address(wormholeTransceiverChain1),
            "Sending chains is wrong for chain two #2"
        );

        sendTrans = nttManagerChain1.getEnabledSendTransceiversForChain(chainId3);
        require(sendTrans.length == 1, "Sending chains length is wrong for chain three #2");
        require(
            sendTrans[0] == address(wormholeTransceiverChain1),
            "Sending chains is wrong for chain three #2"
        );

        sendChains = nttManagerChain1.getChainsEnabledForSending();
        require(sendChains.length == 2, "There should be two chain enabled for sending #2");
        require(sendChains[0] == chainId2, "Chain two should be enabled for sending #2");
        require(sendChains[1] == chainId3, "Chain three should be enabled for sending #2");
    }

    // This test does a transfer between chain one and chain two.
    // Since the receive thresholds are set to one, posting a VAA from only one transceiver completes the transfer.
    function test_thresholdLessThanNumReceivers() public {
        // On manager one, enable two transceivers for sending and receiving on chain two. Set the threshold to one.
        nttManagerChain1.enableSendTransceiverForChain(chainId2, address(wormholeTransceiverChain1));
        nttManagerChain1.enableSendTransceiverForChain(
            chainId2, address(secondWormholeTransceiverChain1)
        );
        nttManagerChain1.enableRecvTransceiverForChain(chainId2, address(wormholeTransceiverChain1));
        nttManagerChain1.enableRecvTransceiverForChain(
            chainId2, address(secondWormholeTransceiverChain1)
        );
        nttManagerChain1.setThreshold(chainId2, 1);

        // On manager two, enable two transceivers for sending and receiving on chain two. Set the threshold to one.
        nttManagerChain2.enableSendTransceiverForChain(chainId1, address(wormholeTransceiverChain2));
        nttManagerChain2.enableSendTransceiverForChain(
            chainId1, address(secondWormholeTransceiverChain2)
        );
        nttManagerChain2.enableRecvTransceiverForChain(chainId1, address(wormholeTransceiverChain2));
        nttManagerChain2.enableRecvTransceiverForChain(
            chainId1, address(secondWormholeTransceiverChain2)
        );
        nttManagerChain2.setThreshold(chainId1, 1);

        vm.chainId(chainId1);

        // Setting up the transfer
        DummyToken token1 = DummyToken(nttManagerChain1.token());
        DummyToken token2 = DummyTokenMintAndBurn(nttManagerChain2.token());

        uint8 decimals = token1.decimals();
        uint256 sendingAmount = 5 * 10 ** decimals;
        token1.mintDummy(address(userA), 5 * 10 ** decimals);

        // Transfer tokens from chain one to chain two through standard means (not relayer)
        vm.startPrank(userA);
        token1.approve(address(nttManagerChain1), sendingAmount);
        vm.recordLogs();
        {
            uint256 nttManagerBalanceBefore = token1.balanceOf(address(nttManagerChain1));
            uint256 userBalanceBefore = token1.balanceOf(address(userA));
            nttManagerChain1.transfer(sendingAmount, chainId2, bytes32(uint256(uint160(userB))));

            // Balance check on funds going in and out working as expected
            uint256 nttManagerBalanceAfter = token1.balanceOf(address(nttManagerChain1));
            uint256 userBalanceAfter = token1.balanceOf(address(userB));
            require(
                nttManagerBalanceBefore + sendingAmount == nttManagerBalanceAfter,
                "Should be locking the tokens"
            );
            require(
                userBalanceBefore - sendingAmount == userBalanceAfter,
                "User should have sent tokens"
            );
        }

        vm.stopPrank();

        // Get and sign the log to go down the other pipes. There should be two messages since we have two transceivers.
        Vm.Log[] memory entries = guardian.fetchWormholeMessageFromLog(vm.getRecordedLogs());
        require(2 == entries.length, "Unexpected number of log entries 1");
        bytes[] memory encodedVMs = new bytes[](entries.length);
        for (uint256 i = 0; i < encodedVMs.length; i++) {
            encodedVMs[i] = guardian.fetchSignedMessageFromLogs(entries[i], chainId1);
        }

        // Chain2 verification and checks
        vm.chainId(chainId2);

        uint256 supplyBefore = token2.totalSupply();
        wormholeTransceiverChain2.receiveMessage(encodedVMs[0]);
        uint256 supplyAfter = token2.totalSupply();

        require(sendingAmount + supplyBefore == supplyAfter, "Supplies dont match #1");
        require(token2.balanceOf(userB) == sendingAmount, "User didn't receive tokens");
        require(token2.balanceOf(address(nttManagerChain2)) == 0, "NttManager has unintended funds");

        // Go back the other way from a THIRD user
        vm.prank(userB);
        token2.transfer(userC, sendingAmount);

        vm.startPrank(userC);
        token2.approve(address(nttManagerChain2), sendingAmount);
        vm.recordLogs();

        // Supply checks on the transfer
        supplyBefore = token2.totalSupply();
        nttManagerChain2.transfer(
            sendingAmount,
            chainId1,
            toWormholeFormat(userD),
            toWormholeFormat(userC),
            false,
            encodeTransceiverInstruction(true)
        );

        supplyAfter = token2.totalSupply();

        require(sendingAmount - supplyBefore == supplyAfter, "Supplies don't match");
        require(token2.balanceOf(userB) == 0, "OG user receive tokens");
        require(token2.balanceOf(userC) == 0, "Sending user didn't receive tokens");
        require(
            token2.balanceOf(address(nttManagerChain2)) == 0,
            "NttManager didn't receive unintended funds"
        );

        // Get and sign the log to go down the other pipe. Thank you to whoever wrote this code in the past!
        entries = guardian.fetchWormholeMessageFromLog(vm.getRecordedLogs());
        require(2 == entries.length, "Unexpected number of log entries 2");
        encodedVMs = new bytes[](entries.length);
        for (uint256 i = 0; i < encodedVMs.length; i++) {
            encodedVMs[i] = guardian.fetchSignedMessageFromLogs(entries[i], chainId2);
        }

        // Chain1 verification and checks with the receiving of the message
        vm.chainId(chainId1);

        supplyBefore = token1.totalSupply();
        wormholeTransceiverChain1.receiveMessage(encodedVMs[0]);
        supplyAfter = token1.totalSupply();

        require(supplyBefore == supplyAfter, "Supplies don't match between operations");
        require(token1.balanceOf(userB) == 0, "OG user receive tokens");
        require(token1.balanceOf(userC) == 0, "Sending user didn't receive tokens");
        require(token1.balanceOf(userD) == sendingAmount, "Transfer did not complete");

        // Submitting the second message back on chain one should not change anything.
        supplyBefore = token1.totalSupply();
        secondWormholeTransceiverChain1.receiveMessage(encodedVMs[1]);
        supplyAfter = token1.totalSupply();

        require(supplyBefore == supplyAfter, "Supplies don't match between operations");
        require(token1.balanceOf(userB) == 0, "OG user receive tokens");
        require(token1.balanceOf(userC) == 0, "Sending user didn't receive tokens");
        require(
            token1.balanceOf(userD) == sendingAmount,
            "Second message updated the balance when it shouldn't have"
        );
    }

    function test_someReverts() public {
        // Threshold too high (since nothing has been enabled yet).
        vm.expectRevert(
            abi.encodeWithSelector(TransceiverRegistry.ThresholdTooHigh.selector, chainId2, 1, 0)
        );
        nttManagerChain1.setThreshold(chainId2, 1);

        // Can't set the threshold to zero.
        nttManagerChain1.enableRecvTransceiverForChain(chainId2, address(wormholeTransceiverChain1));
        vm.expectRevert(
            abi.encodeWithSelector(TransceiverRegistry.ZeroThreshold.selector, chainId2)
        );
        nttManagerChain1.setThreshold(chainId2, 0);

        // Threshold too high when something is actually enabled.
        vm.expectRevert(
            abi.encodeWithSelector(TransceiverRegistry.ThresholdTooHigh.selector, chainId2, 2, 1)
        );
        nttManagerChain1.setThreshold(chainId2, 2);

        // Can't enable sending for chain zero.
        vm.expectRevert(abi.encodeWithSelector(TransceiverRegistry.InvalidChain.selector, 0));
        nttManagerChain1.getEnabledSendTransceiversForChain(0);

        // Can't enable receiving for chain zero.
        vm.expectRevert(abi.encodeWithSelector(TransceiverRegistry.InvalidChain.selector, 0));
        nttManagerChain1.getEnabledRecvTransceiversBitmapForChain(0);

        // Can't add transceiver with zero address.
        vm.expectRevert(
            abi.encodeWithSelector(TransceiverRegistry.InvalidTransceiverZeroAddress.selector)
        );
        nttManagerChain1.setTransceiver(address(0));

        // Can't add the same transceiver twice.
        vm.expectRevert(
            abi.encodeWithSelector(
                TransceiverRegistry.TransceiverAlreadyEnabled.selector,
                address(wormholeTransceiverChain1)
            )
        );
        nttManagerChain1.setTransceiver(address(wormholeTransceiverChain1));

        // Can't remove transceiver with zero address.
        vm.expectRevert(
            abi.encodeWithSelector(TransceiverRegistry.InvalidTransceiverZeroAddress.selector)
        );
        nttManagerChain1.removeTransceiver(address(0));

        // Can't remove a transceiver that hasn't been added.
        vm.expectRevert(
            abi.encodeWithSelector(
                TransceiverRegistry.NonRegisteredTransceiver.selector,
                address(wormholeTransceiverChain2)
            )
        );
        nttManagerChain1.removeTransceiver(address(wormholeTransceiverChain2));

        // Can't remove a transceiver that has already been removed.
        nttManagerChain1.disableRecvTransceiverForChain(
            chainId2, address(wormholeTransceiverChain1)
        );

        nttManagerChain1.removeTransceiver(address(wormholeTransceiverChain1));
        vm.expectRevert(
            abi.encodeWithSelector(
                TransceiverRegistry.DisabledTransceiver.selector, address(wormholeTransceiverChain1)
            )
        );
        nttManagerChain1.removeTransceiver(address(wormholeTransceiverChain1));

        nttManagerChain1.setTransceiver(address(wormholeTransceiverChain1));

        // Can't disable a send transcevier that's not enabled.
        vm.expectRevert(
            abi.encodeWithSelector(
                TransceiverRegistry.TransceiverAlreadyDisabled.selector,
                address(wormholeTransceiverChain1)
            )
        );
        nttManagerChain1.disableSendTransceiverForChain(
            chainId2, address(wormholeTransceiverChain1)
        );

        // Can't enable a send transcevier that is already enabled.
        nttManagerChain1.enableSendTransceiverForChain(chainId2, address(wormholeTransceiverChain1));
        vm.expectRevert(
            abi.encodeWithSelector(
                TransceiverRegistry.TransceiverAlreadyEnabled.selector,
                address(wormholeTransceiverChain1)
            )
        );
        nttManagerChain1.enableSendTransceiverForChain(chainId2, address(wormholeTransceiverChain1));

        // Can't disable a receive transcevier that's not enabled.
        vm.expectRevert(
            abi.encodeWithSelector(
                TransceiverRegistry.TransceiverAlreadyDisabled.selector,
                address(secondWormholeTransceiverChain1)
            )
        );
        nttManagerChain1.disableRecvTransceiverForChain(
            chainId2, address(secondWormholeTransceiverChain1)
        );

        // Can't enable a receive transceiver that's already enabled.
        nttManagerChain1.enableRecvTransceiverForChain(chainId3, address(wormholeTransceiverChain1));
        vm.expectRevert(
            abi.encodeWithSelector(
                TransceiverRegistry.TransceiverAlreadyEnabled.selector,
                address(wormholeTransceiverChain1)
            )
        );
        nttManagerChain1.enableRecvTransceiverForChain(chainId3, address(wormholeTransceiverChain1));

        // Can't enable transceiver with zero address.
        vm.expectRevert(
            abi.encodeWithSelector(TransceiverRegistry.InvalidTransceiverZeroAddress.selector)
        );
        nttManagerChain1.enableRecvTransceiverForChain(chainId3, address(0));

        // Can't enable transceiver with chain zero.
        vm.expectRevert(abi.encodeWithSelector(TransceiverRegistry.InvalidChain.selector, 0));
        nttManagerChain1.enableRecvTransceiverForChain(0, address(wormholeTransceiverChain1));
    }

    // This test does a transfer between chain one and chain three.
    // Since the threshold for these two chains is two, the transfer is not completed until both VAAs are posted.
    function test_thresholdEqualToNumberOfReceivers() public {
        // On manager one, enable two transceivers for sending and receiving on chain two. Set the threshold to one.
        nttManagerChain1.enableSendTransceiverForChain(chainId3, address(wormholeTransceiverChain1));
        nttManagerChain1.enableSendTransceiverForChain(
            chainId3, address(secondWormholeTransceiverChain1)
        );
        nttManagerChain1.enableRecvTransceiverForChain(chainId3, address(wormholeTransceiverChain1));
        nttManagerChain1.enableRecvTransceiverForChain(
            chainId3, address(secondWormholeTransceiverChain1)
        );
        nttManagerChain1.setThreshold(chainId3, 2);

        // On manager two, enable two transceivers for sending and receiving on chain two. Set the threshold to one.
        nttManagerChain3.enableSendTransceiverForChain(chainId1, address(wormholeTransceiverChain3));
        nttManagerChain3.enableSendTransceiverForChain(
            chainId1, address(secondWormholeTransceiverChain3)
        );
        nttManagerChain3.enableRecvTransceiverForChain(chainId1, address(wormholeTransceiverChain3));
        nttManagerChain3.enableRecvTransceiverForChain(
            chainId1, address(secondWormholeTransceiverChain3)
        );
        nttManagerChain3.setThreshold(chainId1, 2);

        vm.chainId(chainId1);

        // Setting up the transfer
        DummyToken token1 = DummyToken(nttManagerChain1.token());
        DummyToken token3 = DummyTokenMintAndBurn(nttManagerChain3.token());

        uint8 decimals = token1.decimals();
        uint256 sendingAmount = 5 * 10 ** decimals;
        token1.mintDummy(address(userA), 5 * 10 ** decimals);
        vm.startPrank(userA);
        token1.approve(address(nttManagerChain1), sendingAmount);

        vm.recordLogs();

        // Send token from chain 1 to chain 3, userB.
        {
            uint256 nttManagerBalanceBefore = token1.balanceOf(address(nttManagerChain1));
            uint256 userBalanceBefore = token1.balanceOf(address(userA));
            nttManagerChain1.transfer(sendingAmount, chainId3, bytes32(uint256(uint160(userB))));

            // Balance check on funds going in and out working as expected
            uint256 nttManagerBalanceAfter = token1.balanceOf(address(nttManagerChain1));
            uint256 userBalanceAfter = token1.balanceOf(address(userB));
            require(
                nttManagerBalanceBefore + sendingAmount == nttManagerBalanceAfter,
                "Should be locking the tokens"
            );
            require(
                userBalanceBefore - sendingAmount == userBalanceAfter,
                "User should have sent tokens"
            );
        }

        vm.stopPrank();

        // Get and sign the log to go down the other pipes. There should be two messages since we have two transceivers.
        Vm.Log[] memory entries = guardian.fetchWormholeMessageFromLog(vm.getRecordedLogs());
        require(2 == entries.length, "Unexpected number of log entries 3");
        bytes[] memory encodedVMs = new bytes[](entries.length);
        for (uint256 i = 0; i < encodedVMs.length; i++) {
            encodedVMs[i] = guardian.fetchSignedMessageFromLogs(entries[i], chainId1);
        }

        // Chain3 verification and checks
        vm.chainId(chainId3);

        uint256 supplyBefore = token3.totalSupply();

        // Submit the first message on chain 3. The numbers shouldn't change yet since the threshold is two.
        wormholeTransceiverChain3.receiveMessage(encodedVMs[0]);
        uint256 supplyAfter = token3.totalSupply();

        require(supplyBefore == supplyAfter, "Supplies changed early");
        require(token3.balanceOf(userB) == 0, "User receive tokens early");
        require(token3.balanceOf(address(nttManagerChain3)) == 0, "NttManager has unintended funds");

        // Submit the second message and the transfer should complete.
        secondWormholeTransceiverChain3.receiveMessage(encodedVMs[1]);
        supplyAfter = token3.totalSupply();

        require(sendingAmount + supplyBefore == supplyAfter, "Supplies dont match");
        require(token3.balanceOf(userB) == sendingAmount, "User didn't receive tokens");
        require(token3.balanceOf(address(nttManagerChain3)) == 0, "NttManager has unintended funds");

        // Go back the other way from a THIRD user
        vm.prank(userB);
        token3.transfer(userC, sendingAmount);

        vm.startPrank(userC);
        token3.approve(address(nttManagerChain3), sendingAmount);
        vm.recordLogs();

        // Supply checks on the transfer
        supplyBefore = token3.totalSupply();
        nttManagerChain3.transfer(
            sendingAmount,
            chainId1,
            toWormholeFormat(userD),
            toWormholeFormat(userC),
            false,
            encodeTransceiverInstruction(true)
        );

        supplyAfter = token3.totalSupply();

        require(sendingAmount - supplyBefore == supplyAfter, "Supplies don't match");
        require(token3.balanceOf(userB) == 0, "OG user receive tokens");
        require(token3.balanceOf(userC) == 0, "Sending user didn't receive tokens");
        require(
            token3.balanceOf(address(nttManagerChain3)) == 0,
            "NttManager didn't receive unintended funds"
        );

        // Get and sign the log to go down the other pipe. Thank you to whoever wrote this code in the past!
        entries = guardian.fetchWormholeMessageFromLog(vm.getRecordedLogs());
        require(2 == entries.length, "Unexpected number of log entries for response");
        encodedVMs = new bytes[](entries.length);
        for (uint256 i = 0; i < encodedVMs.length; i++) {
            encodedVMs[i] = guardian.fetchSignedMessageFromLogs(entries[i], chainId3);
        }

        // Chain1 verification and checks with the receiving of the message
        vm.chainId(chainId1);

        // Submit the first message back on chain one. Nothing should happen because our threshold is two.
        supplyBefore = token1.totalSupply();
        wormholeTransceiverChain1.receiveMessage(encodedVMs[0]);
        supplyAfter = token1.totalSupply();

        require(supplyBefore == supplyAfter, "Supplies don't match between operations");
        require(token1.balanceOf(userB) == 0, "OG user receive tokens");
        require(token1.balanceOf(userC) == 0, "Sending user didn't receive tokens");
        require(token1.balanceOf(userD) == 0, "User received funds before they should");

        // Submit the second message back on chain one. This should update the balance.
        supplyBefore = token1.totalSupply();
        secondWormholeTransceiverChain1.receiveMessage(encodedVMs[1]);
        supplyAfter = token1.totalSupply();

        require(supplyBefore == supplyAfter, "Supplies don't match between operations");
        require(token1.balanceOf(userB) == 0, "OG user receive tokens");
        require(token1.balanceOf(userC) == 0, "Sending user didn't receive tokens");
        require(token1.balanceOf(userD) == sendingAmount, "User received funds");
    }

    function test_onlyTransceiverCanCallAttestationReceived() public {
        // On manager one, enable two transceivers for sending and receiving on chain two. Set the threshold to one.
        nttManagerChain1.enableSendTransceiverForChain(chainId2, address(wormholeTransceiverChain1));
        nttManagerChain1.enableRecvTransceiverForChain(chainId2, address(wormholeTransceiverChain1));
        nttManagerChain1.setThreshold(chainId2, 1);

        TransceiverStructs.NttManagerMessage memory payload;

        vm.startPrank(userA);

        vm.expectRevert(
            abi.encodeWithSelector(
                TransceiverRegistry.CallerNotTransceiver.selector, address(userA)
            )
        );
        nttManagerChain1.attestationReceived(
            chainId2, toWormholeFormat(address(nttManagerChain2)), payload
        );
    }

    function encodeTransceiverInstruction(
        bool relayer_off
    ) public view returns (bytes memory) {
        WormholeTransceiver.WormholeTransceiverInstruction memory instruction =
            IWormholeTransceiver.WormholeTransceiverInstruction(relayer_off);
        bytes memory encodedInstructionWormhole =
            wormholeTransceiverChain1.encodeWormholeTransceiverInstruction(instruction);
        TransceiverStructs.TransceiverInstruction memory TransceiverInstruction = TransceiverStructs
            .TransceiverInstruction({index: 0, payload: encodedInstructionWormhole});
        TransceiverStructs.TransceiverInstruction[] memory TransceiverInstructions =
            new TransceiverStructs.TransceiverInstruction[](1);
        TransceiverInstructions[0] = TransceiverInstruction;
        return TransceiverStructs.encodeTransceiverInstructions(TransceiverInstructions);
    }

    // Encode an instruction for each of the relayers
    function encodeTransceiverInstructions(
        bool relayer_off
    ) public view returns (bytes memory) {
        WormholeTransceiver.WormholeTransceiverInstruction memory instruction =
            IWormholeTransceiver.WormholeTransceiverInstruction(relayer_off);

        bytes memory encodedInstructionWormhole =
            wormholeTransceiverChain1.encodeWormholeTransceiverInstruction(instruction);

        TransceiverStructs.TransceiverInstruction memory TransceiverInstruction1 =
        TransceiverStructs.TransceiverInstruction({index: 0, payload: encodedInstructionWormhole});
        TransceiverStructs.TransceiverInstruction memory TransceiverInstruction2 =
        TransceiverStructs.TransceiverInstruction({index: 1, payload: encodedInstructionWormhole});

        TransceiverStructs.TransceiverInstruction[] memory TransceiverInstructions =
            new TransceiverStructs.TransceiverInstruction[](2);

        TransceiverInstructions[0] = TransceiverInstruction1;
        TransceiverInstructions[1] = TransceiverInstruction2;

        return TransceiverStructs.encodeTransceiverInstructions(TransceiverInstructions);
    }
}
