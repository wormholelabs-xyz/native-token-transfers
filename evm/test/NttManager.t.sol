// SPDX-License-Identifier: Apache 2
pragma solidity >=0.8.8 <0.9.0;

import "forge-std/Test.sol";
import "forge-std/console.sol";

import "../src/NttManager/NttManager.sol";
import "../src/interfaces/INttManager.sol";
import "../src/interfaces/IRateLimiter.sol";
import "../src/interfaces/IManagerBase.sol";
import "../src/interfaces/IRateLimiterEvents.sol";
import "../src/libraries/external/OwnableUpgradeable.sol";
import "../src/libraries/PausableUpgradeable.sol";
import "../src/libraries/TransceiverHelpers.sol";
import {Utils} from "./libraries/Utils.sol";

import "openzeppelin-contracts/contracts/token/ERC20/ERC20.sol";
import "openzeppelin-contracts/contracts/proxy/ERC1967/ERC1967Proxy.sol";
import "wormhole-solidity-sdk/interfaces/IWormhole.sol";
import "wormhole-solidity-sdk/testing/helpers/WormholeSimulator.sol";
import "wormhole-solidity-sdk/Utils.sol";
import "example-gmp-router/evm/src/TransceiverRegistry.sol";
import "./libraries/TransceiverHelpers.sol";
import "./libraries/NttManagerHelpers.sol";
import "./mocks/DummyTransceiver.sol";
import "../src/mocks/DummyToken.sol";
import "./mocks/MockNttManager.sol";
import "./mocks/MockRouter.sol";

// TODO: set this up so the common functionality tests can be run against both
contract TestNttManager is Test, IRateLimiterEvents {
    MockNttManagerContract nttManager;
    MockNttManagerContract nttManagerOther;
    MockNttManagerContract nttManagerZeroRateLimiter;
    MockNttManagerContract nttManagerZeroRateLimiterOther;
    MockRouter router;
    MockRouter routerOther;
    DummyTransceiver transceiver;
    DummyTransceiver transceiverOther;

    using TrimmedAmountLib for uint256;
    using TrimmedAmountLib for TrimmedAmount;

    // 0x99'E''T''T'
    uint16 constant chainId = 7;
    uint16 constant chainId2 = 8;

    address user_A = address(0x123);
    address user_B = address(0x456);

    uint256 constant DEVNET_GUARDIAN_PK =
        0xcfb12303a19cde580bb4dd771639b0d26bc68353645571a8cff516ab2ee113a0;
    WormholeSimulator guardian;
    uint256 initialBlockTimestamp;

    function setUp() public {
        string memory url = "https://ethereum-sepolia-rpc.publicnode.com";
        IWormhole wormhole = IWormhole(0x4a8bc80Ed5a4067f1CCf107057b8270E0cC11A78);
        vm.createSelectFork(url);
        initialBlockTimestamp = vm.getBlockTimestamp();

        guardian = new WormholeSimulator(address(wormhole), DEVNET_GUARDIAN_PK);

        router = new MockRouter(chainId);
        routerOther = new MockRouter(chainId2);

        DummyToken t = new DummyToken();
        NttManager implementation = new MockNttManagerContract(
            address(router), address(t), IManagerBase.Mode.LOCKING, chainId, 1 days, false
        );

        NttManager otherImplementation = new MockNttManagerContract(
            address(routerOther), address(t), IManagerBase.Mode.LOCKING, chainId2, 1 days, false
        );

        nttManager = MockNttManagerContract(address(new ERC1967Proxy(address(implementation), "")));
        nttManager.initialize();

        nttManagerOther =
            MockNttManagerContract(address(new ERC1967Proxy(address(otherImplementation), "")));
        nttManagerOther.initialize();

        nttManager.setPeer(
            chainId2, toWormholeFormat(address(nttManagerOther)), t.decimals(), type(uint64).max
        );

        nttManagerOther.setPeer(
            chainId, toWormholeFormat(address(nttManager)), t.decimals(), type(uint64).max
        );

        transceiver = new DummyTransceiver(chainId, address(router));
        nttManager.setTransceiver(address(transceiver));
        nttManager.enableSendTransceiver(chainId2, address(transceiver));
        nttManager.enableRecvTransceiver(chainId2, address(transceiver));

        transceiverOther = new DummyTransceiver(chainId2, address(routerOther));
        nttManagerOther.setTransceiver(address(transceiverOther));
        nttManagerOther.enableSendTransceiver(chainId, address(transceiverOther));
        nttManagerOther.enableRecvTransceiver(chainId, address(transceiverOther));
    }

    function test_setUp() public {}

    // === pure unit tests

    // naive implementation of countSetBits to test against
    function simpleCount(
        uint64 n
    ) public pure returns (uint8) {
        uint8 count;

        while (n > 0) {
            count += uint8(n & 1);
            n >>= 1;
        }

        return count;
    }

    function testFuzz_countSetBits(
        uint64 n
    ) public {
        assertEq(simpleCount(n), countSetBits(n));
    }

    // === Deployments with rate limiter disabled

    function test_disabledRateLimiter() public {
        // Set things up.
        DummyToken t = new DummyToken();
        {
            NttManager implementation1 = new MockNttManagerContract(
                address(router), address(t), IManagerBase.Mode.LOCKING, chainId, 0, true
            );

            nttManagerZeroRateLimiter =
                MockNttManagerContract(address(new ERC1967Proxy(address(implementation1), "")));
            nttManagerZeroRateLimiter.initialize();

            nttManagerZeroRateLimiter.setTransceiver(address(transceiver));
            nttManagerZeroRateLimiter.enableSendTransceiver(chainId2, address(transceiver));

            routerOther = new MockRouter(chainId2);
            NttManager implementation2 = new MockNttManagerContract(
                address(routerOther), address(t), IManagerBase.Mode.LOCKING, chainId2, 0, true
            );

            nttManagerZeroRateLimiterOther =
                MockNttManagerContract(address(new ERC1967Proxy(address(implementation2), "")));
            nttManagerZeroRateLimiterOther.initialize();

            transceiverOther = new DummyTransceiver(chainId2, address(routerOther));
            nttManagerZeroRateLimiterOther.setTransceiver(address(transceiverOther));
            nttManagerZeroRateLimiterOther.enableRecvTransceiver(chainId, address(transceiverOther));

            nttManagerZeroRateLimiter.setPeer(
                chainId2,
                toWormholeFormat(address(nttManagerZeroRateLimiterOther)),
                9,
                type(uint64).max
            );

            nttManagerZeroRateLimiterOther.setPeer(
                chainId, toWormholeFormat(address(nttManagerZeroRateLimiter)), 9, type(uint64).max
            );

            t.mintDummy(address(user_A), 5 * 10 ** t.decimals());
        }

        // Test outgoing transfers complete successfully with rate limit disabled
        vm.startPrank(user_A);
        t.approve(address(nttManagerZeroRateLimiter), 3 * 10 ** t.decimals());

        vm.recordLogs();

        uint64 s1 = nttManagerZeroRateLimiter.transfer(
            1 * 10 ** t.decimals(), chainId2, toWormholeFormat(user_B)
        );
        uint64 s2 = nttManagerZeroRateLimiter.transfer(
            1 * 10 ** t.decimals(), chainId2, toWormholeFormat(user_B)
        );
        uint64 s3 = nttManagerZeroRateLimiter.transfer(
            1 * 10 ** t.decimals(), chainId2, toWormholeFormat(user_B)
        );
        vm.stopPrank();

        assertEq(s1, 0);
        assertEq(s2, 1);
        assertEq(s3, 2);

        {
            // Verify we actually sent out three messages.
            DummyTransceiver.Message[] memory routerMsgs = transceiver.getMessages();
            assert(3 == routerMsgs.length);

            DummyTransceiver.Message memory rmsg0 = routerMsgs[0];
            DummyTransceiver.Message memory rmsg1 = routerMsgs[1];
            DummyTransceiver.Message memory rmsg2 = routerMsgs[2];

            // Attest all three messages on the other side.
            transceiverOther.receiveMessage(rmsg0);
            transceiverOther.receiveMessage(rmsg1);
            transceiverOther.receiveMessage(rmsg2);

            // Verify that the messages attested.
            require(
                1
                    == nttManagerZeroRateLimiterOther.messageAttestations(
                        rmsg0.srcChain, rmsg0.srcAddr, rmsg0.sequence, rmsg0.dstAddr, rmsg0.payloadHash
                    ),
                "Message 0 did not attest"
            );
            require(
                1
                    == nttManagerZeroRateLimiterOther.messageAttestations(
                        rmsg1.srcChain, rmsg1.srcAddr, rmsg1.sequence, rmsg1.dstAddr, rmsg1.payloadHash
                    ),
                "Message 1 did not attest"
            );
            require(
                1
                    == nttManagerZeroRateLimiterOther.messageAttestations(
                        rmsg2.srcChain, rmsg2.srcAddr, rmsg2.sequence, rmsg2.dstAddr, rmsg2.payloadHash
                    ),
                "Message 2 did not attest"
            );

            // Verify that the right transceiver attested.
            require(
                nttManagerZeroRateLimiterOther.transceiverAttestedToMessage(
                    rmsg0.srcChain,
                    rmsg0.srcAddr,
                    rmsg0.sequence,
                    rmsg0.dstAddr,
                    rmsg0.payloadHash,
                    0
                ),
                "Transceiver did not attest to message 0"
            );
            require(
                nttManagerZeroRateLimiterOther.transceiverAttestedToMessage(
                    rmsg1.srcChain,
                    rmsg1.srcAddr,
                    rmsg1.sequence,
                    rmsg1.dstAddr,
                    rmsg1.payloadHash,
                    0
                ),
                "Transceiver did not attest to message 1"
            );
            require(
                nttManagerZeroRateLimiterOther.transceiverAttestedToMessage(
                    rmsg2.srcChain,
                    rmsg2.srcAddr,
                    rmsg2.sequence,
                    rmsg2.dstAddr,
                    rmsg2.payloadHash,
                    0
                ),
                "Transceiver did not attest to message 2"
            );

            // But the messages should not be marked executed.
            require(
                !nttManagerZeroRateLimiterOther.isMessageExecuted(
                    rmsg0.srcChain, rmsg0.srcAddr, rmsg0.sequence, rmsg0.dstAddr, rmsg0.payloadHash
                ),
                "Message 0 should not be marked executed yet"
            );
            require(
                !nttManagerZeroRateLimiterOther.isMessageExecuted(
                    rmsg1.srcChain, rmsg1.srcAddr, rmsg1.sequence, rmsg1.dstAddr, rmsg1.payloadHash
                ),
                "Message 1 should not be marked executed yet"
            );
            require(
                !nttManagerZeroRateLimiterOther.isMessageExecuted(
                    rmsg2.srcChain, rmsg2.srcAddr, rmsg2.sequence, rmsg2.dstAddr, rmsg2.payloadHash
                ),
                "Message 2 should not be marked executed yet"
            );
        }

        // {
        //     // Get the execution events from the logs.
        //     Vm.Log[] memory logEvents = vm.getRecordedLogs();
        //     (, bytes memory payload1) =
        //         getExecutionSent(logEvents, chainId, address(nttManagerZeroRateLimiter), s1);
        //     (, bytes memory payload2) =
        //         getExecutionSent(logEvents, chainId, address(nttManagerZeroRateLimiter), s2);
        //     (, bytes memory payload3) =
        //         getExecutionSent(logEvents, chainId, address(nttManagerZeroRateLimiter), s3);

        //     nttManagerZeroRateLimiterOther.executeMsg(
        //         chainId,
        //         UniversalAddressLibrary.fromAddress(address(nttManagerZeroRateLimiter)),
        //         s1,
        //         payload1
        //     );
        // }
    }

    // === ownership

    function test_owner() public {
        // TODO: implement separate governance contract
        assertEq(nttManager.owner(), address(this));
    }

    function test_transferOwnership() public {
        address newOwner = address(0x123);
        nttManager.transferOwnership(newOwner);
        assertEq(nttManager.owner(), newOwner);
    }

    function test_onlyOwnerCanTransferOwnership() public {
        address notOwner = address(0x123);
        vm.startPrank(notOwner);

        vm.expectRevert(
            abi.encodeWithSelector(OwnableUpgradeable.OwnableUnauthorizedAccount.selector, notOwner)
        );
        nttManager.transferOwnership(address(0x456));
    }

    function test_pauseUnpause() public {
        assertEq(nttManager.isPaused(), false);
        nttManager.pause();
        assertEq(nttManager.isPaused(), true);

        // When the NttManager is paused, initiating transfers, completing queued transfers on both source and destination chains,
        // executing transfers and attesting to transfers should all revert
        vm.expectRevert(
            abi.encodeWithSelector(PausableUpgradeable.RequireContractIsNotPaused.selector)
        );
        nttManager.transfer(0, 0, bytes32(0));

        vm.expectRevert(
            abi.encodeWithSelector(PausableUpgradeable.RequireContractIsNotPaused.selector)
        );
        nttManager.completeOutboundQueuedTransfer(0);

        vm.expectRevert(
            abi.encodeWithSelector(PausableUpgradeable.RequireContractIsNotPaused.selector)
        );
        nttManager.completeInboundQueuedTransfer(bytes32(0));

        // The router and transceiver are not pausable, so calling receiveMessage should still work.
        DummyTransceiver.Message memory rmsg = DummyTransceiver.Message({
            srcChain: chainId2,
            srcAddr: UniversalAddressLibrary.fromAddress(address(nttManagerOther)),
            sequence: 0,
            dstChain: chainId,
            dstAddr: UniversalAddressLibrary.fromAddress(address(nttManager)),
            payloadHash: keccak256("Hello, World"),
            refundAddr: address(user_A)
        });

        transceiver.receiveMessage(rmsg);

        bytes memory message = "Hello, World";

        // But executeMsg should be paused in the NttManager.
        vm.expectRevert(
            abi.encodeWithSelector(PausableUpgradeable.RequireContractIsNotPaused.selector)
        );
        nttManager.executeMsg(0, UniversalAddressLibrary.fromAddress(address(0)), 0, message);
    }

    function test_pausePauserUnpauseOnlyOwner() public {
        // transfer pauser to another address
        address pauser = address(0x123);
        nttManager.transferPauserCapability(pauser);

        // execute from pauser context
        vm.startPrank(pauser);
        assertEq(nttManager.isPaused(), false);
        nttManager.pause();
        assertEq(nttManager.isPaused(), true);

        vm.expectRevert(
            abi.encodeWithSelector(OwnableUpgradeable.OwnableUnauthorizedAccount.selector, pauser)
        );
        nttManager.unpause();

        // execute from owner context
        // ensures that owner can still unpause
        vm.startPrank(address(this));
        nttManager.unpause();
        assertEq(nttManager.isPaused(), false);
    }

    // === deployment with invalid token
    function test_brokenToken() public {
        DummyToken t = new DummyTokenBroken();
        NttManager implementation = new MockNttManagerContract(
            address(router), address(t), IManagerBase.Mode.LOCKING, chainId, 1 days, false
        );

        NttManager newNttManager =
            MockNttManagerContract(address(new ERC1967Proxy(address(implementation), "")));
        vm.expectRevert(abi.encodeWithSelector(INttManager.StaticcallFailed.selector));
        newNttManager.initialize();

        vm.expectRevert(abi.encodeWithSelector(INttManager.StaticcallFailed.selector));
        newNttManager.transfer(1, 1, bytes32("1"));
    }

    // === transceiver registration

    function test_registerTransceiver() public {
        DummyTransceiver e = new DummyTransceiver(chainId, address(router));
        nttManager.setTransceiver(address(e));
    }

    function test_onlyOwnerCanModifyTransceivers() public {
        DummyTransceiver e = new DummyTransceiver(chainId, address(router));
        nttManager.setTransceiver(address(e));

        address notOwner = address(0x123);
        vm.startPrank(notOwner);

        vm.expectRevert(
            abi.encodeWithSelector(OwnableUpgradeable.OwnableUnauthorizedAccount.selector, notOwner)
        );
        nttManager.setTransceiver(address(e));
    }

    function test_cantEnableTransceiverTwice() public {
        DummyTransceiver e = new DummyTransceiver(chainId, address(router));
        nttManager.setTransceiver(address(e));

        vm.expectRevert(
            abi.encodeWithSelector(
                TransceiverRegistry.TransceiverAlreadyRegistered.selector, address(e)
            )
        );
        nttManager.setTransceiver(address(e));
    }

    // TODO: Do we need these two tests?
    // function test_disableReenableTransceiver() public {
    //     DummyTransceiver e = new DummyTransceiver(chainId, address(router));
    //     nttManager.setTransceiver(address(e));
    //     nttManager.removeTransceiver(address(e));
    //     nttManager.setTransceiver(address(e));
    // }

    // function test_disableAllTransceiversFails() public {
    //     vm.expectRevert(abi.encodeWithSelector(IManagerBase.ZeroThreshold.selector));
    //     nttManager.removeTransceiver(address(transceiver));
    // }

    function test_multipleTransceivers() public {
        // Setup already added one transceiver for chainId2 so we'll add a couple more.
        DummyTransceiver e1 = new DummyTransceiver(chainId2, address(router));
        DummyTransceiver e2 = new DummyTransceiver(chainId2, address(router));

        nttManager.setTransceiver(address(e1));
        nttManager.setTransceiver(address(e2));
    }

    function test_noEnabledTransceivers() public {
        DummyToken token = new DummyToken();
        NttManager implementation = new MockNttManagerContract(
            address(router), address(token), IManagerBase.Mode.LOCKING, chainId, 1 days, false
        );

        MockNttManagerContract newNttManager =
            MockNttManagerContract(address(new ERC1967Proxy(address(implementation), "")));
        newNttManager.initialize();

        user_A = address(0x123);
        user_B = address(0x456);

        uint8 decimals = token.decimals();

        newNttManager.setPeer(chainId2, toWormholeFormat(address(0x1)), 9, type(uint64).max);
        newNttManager.setOutboundLimit(packTrimmedAmount(type(uint64).max, 8).untrim(decimals));

        token.mintDummy(address(user_A), 5 * 10 ** decimals);

        vm.startPrank(user_A);

        token.approve(address(newNttManager), 3 * 10 ** decimals);

        vm.expectRevert(abi.encodeWithSelector(Router.TransceiverNotEnabled.selector));
        newNttManager.transfer(
            1 * 10 ** decimals,
            chainId2,
            toWormholeFormat(user_B),
            toWormholeFormat(user_A),
            false,
            new bytes(1)
        );
    }

    function test_notTransceiver() public {
        // TODO: this is accepted currently. should we include a check to ensure
        // only transceivers can be registered? (this would be a convenience check, not a security one)
        nttManager.setTransceiver(address(0x123));
    }

    ///////////////////////////////////////// TODO: This test takes a long time so I'm disabling it for now. Put it back!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!
    // function test_maxOutTransceivers() public {
    //     // We should be able to register 128 transceivers total. We registered one in set up, so go with one less than the max.
    //     uint256 numTransceivers = router.maxTransceivers() - 1;
    //     for (uint256 i = 0; i < numTransceivers; ++i) {
    //         DummyTransceiver d = new DummyTransceiver(chainId, address(router));
    //         nttManager.setTransceiver(address(d));
    //     }

    //     // Registering a new transceiver should fail as we've hit the cap
    //     DummyTransceiver c = new DummyTransceiver(chainId, address(router));
    //     vm.expectRevert(TransceiverRegistry.TooManyTransceivers.selector);
    //     nttManager.setTransceiver(address(c));
    // }

    // TODO: The new transceiver doesn't support instructions.
    // function test_passingInstructionsToTransceivers() public {
    //     // Let's register a transceiver and then disable the original transceiver. We now have 2 registered transceivers
    //     // since we register 1 in the setup
    //     DummyTransceiver e = new DummyTransceiver(chainId, address(router));
    //     nttManager.setTransceiver(address(e));
    //     nttManager.removeTransceiver(address(transceiver));

    //     user_A = address(0x123);
    //     user_B = address(0x456);

    //     DummyToken token = DummyToken(nttManager.token());

    //     uint8 decimals = token.decimals();

    //     nttManager.setPeer(chainId2, toWormholeFormat(address(0x1)), 9, type(uint64).max);
    //     nttManager.setOutboundLimit(packTrimmedAmount(type(uint64).max, 8).untrim(decimals));

    //     token.mintDummy(address(user_A), 5 * 10 ** decimals);

    //     vm.startPrank(user_A);

    //     token.approve(address(nttManager), 3 * 10 ** decimals);

    //     // Pass some instructions for the enabled transceiver
    //     TransceiverStructs.TransceiverInstruction memory transceiverInstruction =
    //         TransceiverStructs.TransceiverInstruction({index: 1, payload: new bytes(1)});
    //     TransceiverStructs.TransceiverInstruction[] memory transceiverInstructions =
    //         new TransceiverStructs.TransceiverInstruction[](1);
    //     transceiverInstructions[0] = transceiverInstruction;
    //     bytes memory instructions =
    //         TransceiverStructs.encodeTransceiverInstructions(transceiverInstructions);

    //     nttManager.transfer(
    //         1 * 10 ** decimals,
    //         chainId2,
    //         toWormholeFormat(user_B),
    //         toWormholeFormat(user_A),
    //         false,
    //         instructions
    //     );
    // }

    function test_cancellingOutboundQueuedTransfers() public {
        DummyToken token = DummyToken(nttManager.token());

        uint8 decimals = token.decimals();

        nttManager.setPeer(chainId2, toWormholeFormat(address(0x1)), 9, type(uint64).max);
        nttManager.setOutboundLimit(0);

        token.mintDummy(address(user_A), 5 * 10 ** decimals);

        vm.startPrank(user_A);

        token.approve(address(nttManager), 3 * 10 ** decimals);

        uint256 userBalanceBefore = token.balanceOf(user_A);
        uint256 nttManagerBalanceBefore = token.balanceOf(address(nttManager));

        uint64 s1 = nttManager.transfer(
            1 * 10 ** decimals,
            chainId2,
            toWormholeFormat(user_B),
            toWormholeFormat(user_A),
            true,
            new bytes(1)
        );
        vm.stopPrank();

        // Another user should not be able to cancel the transfer
        vm.prank(user_B);
        vm.expectRevert(
            abi.encodeWithSelector(INttManager.CancellerNotSender.selector, user_B, user_A)
        );
        nttManager.cancelOutboundQueuedTransfer(s1);

        vm.startPrank(user_A);
        nttManager.cancelOutboundQueuedTransfer(s1);

        // The balance before and after the cancel should be identical
        assertEq(userBalanceBefore, token.balanceOf(user_A));
        assertEq(nttManagerBalanceBefore, token.balanceOf(address(nttManager)));

        // We cannot cancel a queued transfer more than once
        vm.expectRevert(
            abi.encodeWithSelector(IRateLimiter.OutboundQueuedTransferNotFound.selector, s1)
        );
        nttManager.cancelOutboundQueuedTransfer(s1);

        // We cannot complete an outbound transfer that has already been cancelled
        vm.expectRevert(
            abi.encodeWithSelector(IRateLimiter.OutboundQueuedTransferNotFound.selector, s1)
        );
        nttManager.completeOutboundQueuedTransfer(s1);

        // The next transfer has previous sequence number + 1
        uint64 s2 = nttManager.transfer(
            1 * 10 ** decimals,
            chainId2,
            toWormholeFormat(user_B),
            toWormholeFormat(user_A),
            true,
            new bytes(1)
        );

        assertEq(s2, s1 + 1);
    }

    // == threshold

    function test_cantSetThresholdTooHigh() public {
        // 1 transceiver set, so can't set threshold to 2
        vm.expectRevert(abi.encodeWithSelector(IManagerBase.ThresholdTooHigh.selector, 2, 1));
        nttManager.setThreshold(2);
    }

    function test_canSetThreshold() public {
        DummyTransceiver e1 = new DummyTransceiver(chainId, address(router));
        DummyTransceiver e2 = new DummyTransceiver(chainId, address(router));
        nttManager.setTransceiver(address(e1));
        nttManager.setTransceiver(address(e2));

        nttManager.setThreshold(1);
        nttManager.setThreshold(2);
        nttManager.setThreshold(1);
    }

    function test_cantSetThresholdToZero() public {
        DummyTransceiver e = new DummyTransceiver(chainId, address(router));
        nttManager.setTransceiver(address(e));

        vm.expectRevert(abi.encodeWithSelector(IManagerBase.ZeroThreshold.selector));
        nttManager.setThreshold(0);
    }

    function test_onlyOwnerCanSetThreshold() public {
        address notOwner = address(0x123);
        vm.startPrank(notOwner);

        vm.expectRevert(
            abi.encodeWithSelector(OwnableUpgradeable.OwnableUnauthorizedAccount.selector, notOwner)
        );
        nttManager.setThreshold(1);
    }

    // == threshold

    function test_peerRegistrationLimitsCanBeUpdated() public {
        bytes32 peer = toWormholeFormat(address(nttManager));
        nttManager.setPeer(nttManagerOther.chainId(), peer, 9, 0);

        IRateLimiter.RateLimitParams memory params =
            nttManager.getInboundLimitParams(nttManagerOther.chainId());
        assertEq(params.limit.getAmount(), 0);
        assertEq(params.limit.getDecimals(), 8);

        nttManager.setInboundLimit(type(uint64).max, nttManagerOther.chainId());
        params = nttManager.getInboundLimitParams(nttManagerOther.chainId());
        assertEq(params.limit.getAmount(), type(uint64).max / 10 ** (18 - 8));
        assertEq(params.limit.getDecimals(), 8);
    }

    // === attestation

    function test_onlyEnabledTransceiversCanAttest() public {
        // Setup created two managers, each with one enabled transceiver pointed at the other.

        DummyTransceiver.Message memory rmsg = DummyTransceiver.Message({
            srcChain: chainId2,
            srcAddr: UniversalAddressLibrary.fromAddress(address(nttManagerOther)),
            sequence: 0,
            dstChain: chainId,
            dstAddr: UniversalAddressLibrary.fromAddress(address(nttManager)),
            payloadHash: keccak256("Hello, World"),
            refundAddr: address(user_A)
        });

        // This should work.
        transceiver.receiveMessage(rmsg);
        checkAttestation(nttManager, 0, rmsg);

        // But if we disable the transceiver for receiving, it should fail.
        nttManager.disableRecvTransceiver(chainId2, address(transceiver));

        vm.expectRevert(abi.encodeWithSelector(Router.TransceiverNotEnabled.selector));
        transceiver.receiveMessage(rmsg);
    }

    function test_onlyPeerNttManagerCanAttest() public {
        DummyTransceiver.Message memory rmsg = DummyTransceiver.Message({
            srcChain: chainId2,
            srcAddr: UniversalAddressLibrary.fromAddress(address(0xdeadbeef)), // This is not the peer NttManager.
            sequence: 0,
            dstChain: chainId,
            dstAddr: UniversalAddressLibrary.fromAddress(address(nttManager)),
            payloadHash: keccak256("Hello, World"),
            refundAddr: address(user_A)
        });

        // The router and transceiver don't block this, so this should succeed.
        transceiver.receiveMessage(rmsg);
        checkAttestation(nttManager, 0, rmsg);

        // But the call to executeMsg should check the peer.
        bytes memory message = "Hello, World";
        vm.expectRevert(
            abi.encodeWithSelector(
                INttManager.InvalidPeer.selector,
                chainId2,
                UniversalAddressLibrary.fromAddress(address(0xdeadbeef)).toBytes32()
            )
        );
        nttManager.executeMsg(
            chainId2, UniversalAddressLibrary.fromAddress(address(0xdeadbeef)), 0, message
        );
    }

    function test_attestSimple() public {
        DummyTransceiver.Message memory rmsg = DummyTransceiver.Message({
            srcChain: chainId2,
            srcAddr: UniversalAddressLibrary.fromAddress(address(nttManagerOther)),
            sequence: 0,
            dstChain: chainId,
            dstAddr: UniversalAddressLibrary.fromAddress(address(nttManager)),
            payloadHash: keccak256("Hello, World"),
            refundAddr: address(user_A)
        });

        // Attest the message. This should work.
        transceiver.receiveMessage(rmsg);
        checkAttestation(nttManager, 0, rmsg);

        // Can't attest the same message twice.
        vm.expectRevert(abi.encodeWithSelector(Router.DuplicateMessageAttestation.selector));
        transceiver.receiveMessage(rmsg);

        // Can't attest when the transceiver is disabled.
        nttManager.disableRecvTransceiver(chainId2, address(transceiver));

        rmsg = DummyTransceiver.Message({
            srcChain: chainId2,
            srcAddr: UniversalAddressLibrary.fromAddress(address(nttManagerOther)),
            sequence: 1,
            dstChain: chainId,
            dstAddr: UniversalAddressLibrary.fromAddress(address(nttManager)),
            payloadHash: keccak256("Hello, World"),
            refundAddr: address(user_A)
        });

        vm.expectRevert(abi.encodeWithSelector(Router.TransceiverNotEnabled.selector));
        transceiver.receiveMessage(rmsg);
    }

    function test_transfer_sequences() public {
        DummyToken token = DummyToken(nttManager.token());

        uint8 decimals = token.decimals();

        nttManager.setPeer(chainId2, toWormholeFormat(address(0x1)), 9, type(uint64).max);
        nttManager.setOutboundLimit(packTrimmedAmount(type(uint64).max, 8).untrim(decimals));

        token.mintDummy(address(user_A), 5 * 10 ** decimals);

        vm.startPrank(user_A);

        token.approve(address(nttManager), 3 * 10 ** decimals);

        uint64 s1 = nttManager.transfer(
            1 * 10 ** decimals,
            chainId2,
            toWormholeFormat(user_B),
            toWormholeFormat(user_A),
            false,
            new bytes(1)
        );
        uint64 s2 = nttManager.transfer(
            1 * 10 ** decimals,
            chainId2,
            toWormholeFormat(user_B),
            toWormholeFormat(user_A),
            false,
            new bytes(1)
        );
        uint64 s3 = nttManager.transfer(
            1 * 10 ** decimals,
            chainId2,
            toWormholeFormat(user_B),
            toWormholeFormat(user_A),
            false,
            new bytes(1)
        );

        assertEq(s1, 0);
        assertEq(s2, 1);
        assertEq(s3, 2);
    }

    function test_transferWithAmountAndDecimalsThatCouldOverflow() public {
        // The source chain has 18 decimals trimmed to 8, and the peer has 6 decimals trimmed to 6
        nttManager.setPeer(chainId2, toWormholeFormat(address(0x1)), 6, type(uint64).max);

        DummyToken token = DummyToken(nttManager.token());
        uint8 decimals = token.decimals();
        assertEq(decimals, 18);

        token.mintDummy(address(user_A), type(uint256).max);

        vm.startPrank(user_A);
        token.approve(address(nttManager), type(uint256).max);

        // When transferring to a chain with 6 decimals the amount will get trimmed to 6 decimals
        // and then scaled back up to 8 for local accounting. If we get the trimmed amount to be
        // type(uint64).max, then when scaling up we could overflow. We safely cast to prevent this.

        uint256 amount = type(uint64).max * 10 ** (decimals - 6);

        vm.expectRevert("SafeCast: value doesn't fit in 64 bits");
        nttManager.transfer(
            amount,
            chainId2,
            toWormholeFormat(user_B),
            toWormholeFormat(user_A),
            false,
            new bytes(1)
        );

        // A (slightly) more sensible amount should work normally
        amount = (type(uint64).max * 10 ** (decimals - 6 - 2)) - 150000000000; // Subtract this to make sure we don't have dust
        nttManager.transfer(
            amount,
            chainId2,
            toWormholeFormat(user_B),
            toWormholeFormat(user_A),
            false,
            new bytes(1)
        );
    }

    // function test_attestationQuorum() public {
    //     user_B = address(0x456);

    //     (DummyTransceiver e1, DummyTransceiver e2) =
    //         TransceiverHelpersLib.setup_transceivers(nttManagerOther);

    //     TrimmedAmount transferAmount = packTrimmedAmount(50, 8);

    //     TransceiverStructs.NttManagerMessage memory m;
    //     bytes memory encodedEm;
    //     {
    //         DummyTransceiver[] memory transceivers = new DummyTransceiver[](2);
    //         transceivers[0] = e1;
    //         transceivers[1] = e2;

    //         TransceiverStructs.TransceiverMessage memory em;
    //         (m, em) = TransceiverHelpersLib.attestTransceiversHelper(
    //             user_B,
    //             0,
    //             chainId,
    //             nttManager,
    //             nttManagerOther,
    //             transferAmount,
    //             packTrimmedAmount(type(uint64).max, 8),
    //             transceivers
    //         );
    //         encodedEm = TransceiverStructs.encodeTransceiverMessage(
    //             TransceiverHelpersLib.TEST_TRANSCEIVER_PAYLOAD_PREFIX, em
    //         );
    //     }

    //     {
    //         token = DummyToken(nttManager.token());
    //         assertEq(token.balanceOf(address(user_B)), transferAmount.untrim(token.decimals()));
    //     }

    //     // replay protection for transceiver
    //     vm.recordLogs();
    //     vm.expectRevert(
    //         abi.encodeWithSelector(
    //             IManagerBase.TransceiverAlreadyAttestedToMessage.selector,
    //             TransceiverStructs.nttManagerMessageDigest(
    //                 TransceiverHelpersLib.SENDING_CHAIN_ID, m
    //             )
    //         )
    //     );
    //     e2.receiveMessage(encodedEm);
    // }

    // function test_transfersOnForkedChains() public {
    //     uint256 evmChainId = block.chainid;

    //     uint8 decimals = token.decimals();

    //     nttManager.setPeer(
    //         TransceiverHelpersLib.SENDING_CHAIN_ID,
    //         toWormholeFormat(address(nttManagerOther)),
    //         9,
    //         type(uint64).max
    //     );
    //     nttManager.setOutboundLimit(0);

    //     token.mintDummy(address(user_A), 5 * 10 ** decimals);

    //     vm.startPrank(user_A);

    //     token.approve(address(nttManager), 3 * 10 ** decimals);

    //     uint64 sequence = nttManager.transfer(
    //         1 * 10 ** decimals,
    //         TransceiverHelpersLib.SENDING_CHAIN_ID,
    //         toWormholeFormat(user_B),
    //         toWormholeFormat(user_A),
    //         true,
    //         new bytes(1)
    //     );

    //     vm.warp(vm.getBlockTimestamp() + 1 days);

    //     vm.chainId(chainId);

    //     // Queued outbound transfers can't be completed
    //     vm.expectRevert(abi.encodeWithSelector(InvalidFork.selector, evmChainId, chainId));
    //     nttManager.completeOutboundQueuedTransfer(sequence);

    //     // Queued outbound transfers can't be cancelled
    //     vm.expectRevert(abi.encodeWithSelector(InvalidFork.selector, evmChainId, chainId));
    //     nttManager.cancelOutboundQueuedTransfer(sequence);

    //     // Outbound transfers fail when queued
    //     vm.expectRevert(abi.encodeWithSelector(InvalidFork.selector, evmChainId, chainId));
    //     nttManager.transfer(
    //         1 * 10 ** decimals,
    //         TransceiverHelpersLib.SENDING_CHAIN_ID,
    //         toWormholeFormat(user_B),
    //         toWormholeFormat(user_A),
    //         true,
    //         new bytes(1)
    //     );
    //     vm.stopPrank();

    //     nttManager.setOutboundLimit(packTrimmedAmount(type(uint64).max, 8).untrim(decimals));
    //     // Outbound transfers fail when not queued
    //     vm.prank(user_A);
    //     vm.expectRevert(abi.encodeWithSelector(InvalidFork.selector, evmChainId, chainId));
    //     nttManager.transfer(
    //         1 * 10 ** decimals,
    //         TransceiverHelpersLib.SENDING_CHAIN_ID,
    //         toWormholeFormat(user_B),
    //         toWormholeFormat(user_A),
    //         false,
    //         new bytes(1)
    //     );

    //     // INBOUND

    //     bytes memory tokenTransferMessage = TransceiverStructs.encodeNativeTokenTransfer(
    //         TransceiverStructs.NativeTokenTransfer({
    //             amount: packTrimmedAmount(100, 8),
    //             sourceToken: toWormholeFormat(address(token)),
    //             to: toWormholeFormat(user_B),
    //             toChain: chainId
    //         })
    //     );

    //     bytes memory transceiverMessage;
    //     TransceiverStructs.NttManagerMessage memory nttManagerMessage;
    //     (nttManagerMessage, transceiverMessage) = TransceiverHelpersLib
    //         .buildTransceiverMessageWithNttManagerPayload(
    //         0,
    //         toWormholeFormat(address(0x1)),
    //         toWormholeFormat(address(nttManagerOther)),
    //         toWormholeFormat(address(nttManager)),
    //         tokenTransferMessage
    //     );

    //     // Inbound transfers can't be completed
    //     vm.expectRevert(abi.encodeWithSelector(InvalidFork.selector, evmChainId, chainId));
    //     transceiver.receiveMessage(transceiverMessage);

    //     // Inbound queued transfers can't be completed
    //     nttManager.setInboundLimit(0, TransceiverHelpersLib.SENDING_CHAIN_ID);

    //     vm.chainId(evmChainId);

    //     bytes32 hash = TransceiverStructs.nttManagerMessageDigest(
    //         TransceiverHelpersLib.SENDING_CHAIN_ID, nttManagerMessage
    //     );
    //     transceiver.receiveMessage(transceiverMessage);

    //     vm.chainId(chainId);

    //     vm.warp(vm.getBlockTimestamp() + 1 days);

    //     vm.expectRevert(abi.encodeWithSelector(InvalidFork.selector, evmChainId, chainId));
    //     nttManager.completeInboundQueuedTransfer(hash);
    // }

    // TODO:
    // currently there is no way to test the threshold logic and the duplicate
    // protection logic without setting up the business logic as well.
    //
    // we should separate the business logic out from the transceiver handling.
    // that way the functionality could be tested separately (and the contracts
    // would also be more reusable)

    // === storage

    function test_noAutomaticSlot() public {
        DummyToken t = new DummyToken();
        MockNttManagerContract c = new MockNttManagerContract(
            address(router), address(t), IManagerBase.Mode.LOCKING, 1, 1 days, false
        );
        assertEq(c.lastSlot(), 0x0);
    }

    function test_constructor() public {
        DummyToken t = new DummyToken();

        vm.startStateDiffRecording();

        new MockNttManagerContract(
            address(router), address(t), IManagerBase.Mode.LOCKING, 1, 1 days, false
        );

        Utils.assertSafeUpgradeableConstructor(vm.stopAndReturnStateDiff());
    }

    // === token transfer logic

    function test_dustReverts() public {
        // transfer 3 tokens
        address from = address(0x123);
        address to = address(0x456);

        DummyToken token = DummyToken(nttManager.token());
        uint8 decimals = token.decimals();

        uint256 maxAmount = 5 * 10 ** decimals;
        token.mintDummy(from, maxAmount);
        nttManager.setPeer(chainId2, toWormholeFormat(address(0x1)), 9, type(uint64).max);
        nttManager.setOutboundLimit(packTrimmedAmount(type(uint64).max, 8).untrim(decimals));
        nttManager.setInboundLimit(
            packTrimmedAmount(type(uint64).max, 8).untrim(decimals), nttManagerOther.chainId()
        );

        vm.startPrank(from);

        uint256 transferAmount = 3 * 10 ** decimals;
        assertEq(
            transferAmount < maxAmount - 500, true, "Transferring more tokens than what exists"
        );

        uint256 dustAmount = 500;
        uint256 amountWithDust = transferAmount + dustAmount; // An amount with 19 digits, which will result in dust due to 18 decimals
        token.approve(address(nttManager), amountWithDust);

        vm.expectRevert(
            abi.encodeWithSelector(
                INttManager.TransferAmountHasDust.selector, amountWithDust, dustAmount
            )
        );
        nttManager.transfer(
            amountWithDust,
            chainId2,
            toWormholeFormat(to),
            toWormholeFormat(from),
            false,
            new bytes(1)
        );

        vm.stopPrank();
    }

    // === upgradeability
    function expectRevert(
        address contractAddress,
        bytes memory encodedSignature,
        bytes memory expectedRevert
    ) internal {
        (bool success, bytes memory result) = contractAddress.call(encodedSignature);
        require(!success, "call did not revert");

        require(keccak256(result) == keccak256(expectedRevert), "call did not revert as expected");
    }

    function test_upgradeNttManager() public {
        // The testing strategy here is as follows:
        // Step 1: Deploy the nttManager contract with two transceivers and
        //         receive a message through it.
        // Step 2: Upgrade it to a new nttManager contract an use the same transceivers to receive
        //         a new message through it.
        // Step 3: Upgrade back to the standalone contract (with two
        //           transceivers) and receive a message through it.
        // This ensures that the storage slots don't get clobbered through the upgrades.

        // Get rid of the original transceiver so we can add two new ones.
        nttManagerOther.disableSendTransceiver(chainId, address(transceiverOther));
        nttManagerOther.disableRecvTransceiver(chainId, address(transceiverOther));

        DummyToken token = DummyToken(nttManager.token());
        TrimmedAmount transferAmount = packTrimmedAmount(50, 8);
        (DummyTransceiver e1, DummyTransceiver e2) =
            TransceiverHelpersLib.setup_transceivers(nttManagerOther, chainId);

        // Step 1 (contract is deployed by setUp())
        DummyTransceiver[] memory transceivers = new DummyTransceiver[](2);
        transceivers[0] = e1;
        transceivers[1] = e2;

        TransceiverStructs.NttManagerMessage memory m = TransceiverHelpersLib
            .attestTransceiversHelper(
            user_B,
            0,
            chainId2,
            nttManager,
            nttManagerOther,
            transferAmount,
            packTrimmedAmount(type(uint64).max, 8),
            transceivers
        );

        assertEq(token.balanceOf(address(user_B)), transferAmount.untrim(token.decimals()));

        /*
        // Step 2 (upgrade to a new nttManager)
        MockNttManagerContract newNttManager = new MockNttManagerContract(
            address(router), nttManager.token(), IManagerBase.Mode.LOCKING, chainId, 1 days, false
        );
        nttManagerOther.upgrade(address(newNttManager));

        TransceiverHelpersLib.attestTransceiversHelper(
            user_B,
            bytes32(uint256(1)),
            chainId,
            nttManager, // this is the proxy
            nttManagerOther, // this is the proxy
            transferAmount,
            packTrimmedAmount(type(uint64).max, 8),
            transceivers
        );

        assertEq(token.balanceOf(address(user_B)), transferAmount.untrim(token.decimals()) * 2);
        */
    }

    /*
    function test_tokenUpgradedAndDecimalsChanged() public {
        DummyToken dummy1 = new DummyTokenMintAndBurn();

        // Make the token an upgradeable token
        DummyTokenMintAndBurn t =
            DummyTokenMintAndBurn(address(new ERC1967Proxy(address(dummy1), "")));

        NttManager implementation = new MockNttManagerContract(
            address(router), address(t), IManagerBase.Mode.LOCKING, chainId, 1 days, false
        );

        MockNttManagerContract newNttManager =
            MockNttManagerContract(address(new ERC1967Proxy(address(implementation), "")));
        newNttManager.initialize();

        // register nttManager peer and transceiver
        bytes32 peer = toWormholeFormat(address(nttManager));
        newNttManager.setPeer(TransceiverHelpersLib.SENDING_CHAIN_ID, peer, 9, type(uint64).max);
        {
            DummyTransceiver e = new DummyTransceiver(chainId, address(newNttManager));
            newNttManager.setTransceiver(address(e));
        }

        t.mintDummy(address(user_A), 5 * 10 ** t.decimals());

        // Check that we can initiate a transfer
        vm.startPrank(user_A);
        t.approve(address(newNttManager), 3 * 10 ** t.decimals());
        newNttManager.transfer(
            1 * 10 ** t.decimals(),
            TransceiverHelpersLib.SENDING_CHAIN_ID,
            toWormholeFormat(user_B),
            toWormholeFormat(user_A),
            false,
            new bytes(1)
        );
        vm.stopPrank();

        // Check that we can receive a transfer
        (DummyTransceiver e1,) = TransceiverHelpersLib.setup_transceivers(newNttManager);
        newNttManager.setThreshold(1);

        bytes memory transceiverMessage;
        bytes memory tokenTransferMessage;

        TrimmedAmount transferAmount = packTrimmedAmount(100, 8);

        tokenTransferMessage = TransceiverStructs.encodeNativeTokenTransfer(
            TransceiverStructs.NativeTokenTransfer({
                amount: transferAmount,
                sourceToken: toWormholeFormat(address(t)),
                to: toWormholeFormat(user_B),
                toChain: chainId
            })
        );

        (, transceiverMessage) = TransceiverHelpersLib.buildTransceiverMessageWithNttManagerPayload(
            0, bytes32(0), peer, toWormholeFormat(address(newNttManager)), tokenTransferMessage
        );

        e1.receiveMessage(transceiverMessage);
        uint256 userBBalanceBefore = t.balanceOf(address(user_B));
        assertEq(userBBalanceBefore, transferAmount.untrim(t.decimals()));

        // If the token decimals change to the same trimmed amount, we should safely receive the correct number of tokens
        DummyTokenDifferentDecimals dummy2 = new DummyTokenDifferentDecimals(10); // 10 gets trimmed to 8
        t.upgrade(address(dummy2));

        vm.startPrank(user_A);
        newNttManager.transfer(
            1 * 10 ** 10,
            TransceiverHelpersLib.SENDING_CHAIN_ID,
            toWormholeFormat(user_B),
            toWormholeFormat(user_A),
            false,
            new bytes(1)
        );
        vm.stopPrank();

        (, transceiverMessage) = TransceiverHelpersLib.buildTransceiverMessageWithNttManagerPayload(
            bytes32("1"),
            bytes32(0),
            peer,
            toWormholeFormat(address(newNttManager)),
            tokenTransferMessage
        );
        e1.receiveMessage(transceiverMessage);
        assertEq(
            t.balanceOf(address(user_B)), userBBalanceBefore + transferAmount.untrim(t.decimals())
        );

        // Now if the token decimals change to a different trimmed amount, we shouldn't be able to send or receive
        DummyTokenDifferentDecimals dummy3 = new DummyTokenDifferentDecimals(7); // 7 is 7 trimmed
        t.upgrade(address(dummy3));

        vm.startPrank(user_A);
        vm.expectRevert(abi.encodeWithSelector(NumberOfDecimalsNotEqual.selector, 8, 7));
        newNttManager.transfer(
            1 * 10 ** 7,
            TransceiverHelpersLib.SENDING_CHAIN_ID,
            toWormholeFormat(user_B),
            toWormholeFormat(user_A),
            false,
            new bytes(1)
        );
        vm.stopPrank();

        (, transceiverMessage) = TransceiverHelpersLib.buildTransceiverMessageWithNttManagerPayload(
            bytes32("2"),
            bytes32(0),
            peer,
            toWormholeFormat(address(newNttManager)),
            tokenTransferMessage
        );
        vm.expectRevert(abi.encodeWithSelector(NumberOfDecimalsNotEqual.selector, 8, 7));
        e1.receiveMessage(transceiverMessage);
    }

    function test_transferWithInstructionIndexOutOfBounds() public {
        TransceiverStructs.TransceiverInstruction memory TransceiverInstruction =
            TransceiverStructs.TransceiverInstruction({index: 100, payload: new bytes(1)});
        TransceiverStructs.TransceiverInstruction[] memory TransceiverInstructions =
            new TransceiverStructs.TransceiverInstruction[](1);
        TransceiverInstructions[0] = TransceiverInstruction;
        bytes memory encodedInstructions =
            TransceiverStructs.encodeTransceiverInstructions(TransceiverInstructions);

        DummyToken token = DummyToken(nttManager.token());

        uint8 decimals = token.decimals();

        nttManager.setPeer(chainId2, toWormholeFormat(address(0x1)), 9, type(uint64).max);
        nttManager.setOutboundLimit(packTrimmedAmount(type(uint64).max, 8).untrim(decimals));

        token.mintDummy(address(user_A), 5 * 10 ** decimals);

        vm.startPrank(user_A);

        token.approve(address(nttManager), 3 * 10 ** decimals);

        vm.expectRevert(
            abi.encodeWithSelector(TransceiverStructs.InvalidInstructionIndex.selector, 100, 1)
        );
        nttManager.transfer(
            1 * 10 ** decimals,
            chainId2,
            toWormholeFormat(user_B),
            toWormholeFormat(user_A),
            false,
            encodedInstructions
        );
    }
    */

    error ExecutionEventNotFoundInLogs(uint64 nttSeqNo);

    function getExecutionSent(
        Vm.Log[] memory events,
        uint16 srcChain,
        address integrator,
        uint64 nttSeqNo
    ) public pure returns (uint64 routerSeq, bytes memory payload) {
        for (uint256 idx = 0; idx < events.length; ++idx) {
            if (
                events[idx].topics[0]
                    == bytes32(0x34a042d85c0b260d1be6cd4bf178c0a3f85c6cf64868e6d64ec7a11027449d5a)
                    && events[idx].topics[1] == bytes32(uint256(srcChain))
                    && events[idx].emitter == integrator
                    && events[idx].topics[3] == bytes32(uint256(nttSeqNo))
            ) {
                (routerSeq,,, payload) =
                    abi.decode(events[idx].data, (uint64, uint16, bytes32, bytes));
                return (routerSeq, payload);
            }
        }
        revert ExecutionEventNotFoundInLogs(nttSeqNo);
    }

    function checkAttestation(
        MockNttManagerContract nttm,
        uint8 transceiverIdx,
        DummyTransceiver.Message memory rmsg
    ) public view {
        // Verify that it shows as attested.
        require(
            1 << transceiverIdx
                == nttm.messageAttestations(
                    rmsg.srcChain, rmsg.srcAddr, rmsg.sequence, rmsg.dstAddr, rmsg.payloadHash
                ),
            "Message did not attest"
        );

        // Verify that the right transceiver attested.
        require(
            nttm.transceiverAttestedToMessage(
                rmsg.srcChain,
                rmsg.srcAddr,
                rmsg.sequence,
                rmsg.dstAddr,
                rmsg.payloadHash,
                transceiverIdx
            ),
            "Transceiver did not attest to message"
        );

        // But the message should not be marked as executed.
        require(
            !nttm.isMessageExecuted(
                rmsg.srcChain, rmsg.srcAddr, rmsg.sequence, rmsg.dstAddr, rmsg.payloadHash
            ),
            "Message should not be marked executed yet"
        );
    }
}
