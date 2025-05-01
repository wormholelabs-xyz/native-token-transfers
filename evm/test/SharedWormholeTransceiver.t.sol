// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.13;

import "forge-std/Test.sol";
import "wormhole-solidity-sdk/interfaces/IWormhole.sol";
import "../src/Transceiver/SharedTransceiver/SharedWormholeTransceiver.sol";
import "../src/interfaces/ISharedWormholeTransceiver.sol";
import "./mocks/MockWormhole.sol";

contract MyMsgReceiver {
    uint16 public immutable chainId;

    struct Message {
        uint16 sourceChainId;
        bytes32 sourceManagerAddress;
        TransceiverStructs.NttManagerMessage payload;
    }

    Message[] private messages;

    constructor(
        uint16 _chainId
    ) {
        chainId = _chainId;
    }

    function numMessages() public view returns (uint256) {
        return messages.length;
    }

    function getMessage(
        uint256 idx
    ) public view returns (Message memory) {
        return messages[idx];
    }

    function attestationReceived(
        uint16 sourceChainId,
        bytes32 sourceManagerAddress,
        TransceiverStructs.NttManagerMessage memory payload
    ) external {
        messages.push(Message(sourceChainId, sourceManagerAddress, payload));
    }
}

contract SharedWormholeTransceiverForTest is SharedWormholeTransceiver {
    constructor(
        uint16 _ourChain,
        address _admin,
        address _wormhole,
        uint8 _consistencyLevel
    ) SharedWormholeTransceiver(_ourChain, _admin, _wormhole, _consistencyLevel) {}
}

contract SharedWormholeTransceiverTest is Test {
    MyMsgReceiver receiver1;
    address receiverAddr1;
    MyMsgReceiver receiver2;
    address receiverAddr2;

    address admin = address(0xabcdef);
    address userA = address(0xabcdec);
    address userB = address(0xabcdeb);
    MockWormhole myWormhole;
    SharedWormholeTransceiverForTest public srcTransceiver;
    MockWormhole destWormhole;
    SharedWormholeTransceiverForTest public destTransceiver;
    SharedWormholeTransceiverForTest public otherDestTransceiver;
    uint8 consistencyLevel = 200;

    uint16 ourChain = 42;

    uint16 srcChain = 42;
    uint16 destChain = 43;

    uint16 peerChain1 = 1;
    uint16 peerChain2 = 2;
    uint16 peerChain3 = 3;

    TransceiverStructs.TransceiverInstruction emptyInstructions =
        TransceiverStructs.TransceiverInstruction(0, new bytes(0));

    function setUp() public {
        receiver1 = new MyMsgReceiver(peerChain1);
        receiverAddr1 = address(receiver1);
        receiver2 = new MyMsgReceiver(peerChain2);
        receiverAddr2 = address(receiver2);

        myWormhole = new MockWormhole(srcChain);
        srcTransceiver = new SharedWormholeTransceiverForTest(
            srcChain, admin, address(myWormhole), consistencyLevel
        );
        destWormhole = new MockWormhole(destChain);
        destTransceiver = new SharedWormholeTransceiverForTest(
            destChain, admin, address(destWormhole), consistencyLevel
        );
        otherDestTransceiver = new SharedWormholeTransceiverForTest(
            destChain, admin, address(destWormhole), consistencyLevel
        );

        // Give everyone some money to play with.
        vm.deal(receiverAddr1, 1 ether);
        vm.deal(receiverAddr2, 1 ether);
        vm.deal(admin, 1 ether);
        vm.deal(userA, 1 ether);
        vm.deal(userB, 1 ether);
    }

    function test_init() public view {
        require(srcTransceiver.ourChain() == ourChain, "ourChain is not right");
        require(srcTransceiver.admin() == admin, "admin is not right");
        require(
            address(srcTransceiver.wormhole()) == address(myWormhole), "myWormhole is not right"
        );
        require(
            srcTransceiver.consistencyLevel() == consistencyLevel, "consistencyLevel is not right"
        );
    }

    function test_invalidInit() public {
        // ourChain can't be zero.
        vm.expectRevert();
        new SharedWormholeTransceiver(0, admin, address(destWormhole), consistencyLevel);

        // admin can't be zero.
        vm.expectRevert();
        new SharedWormholeTransceiver(
            destChain, address(0), address(destWormhole), consistencyLevel
        );

        // wormhole can't be zero.
        vm.expectRevert();
        new SharedWormholeTransceiver(destChain, admin, address(0), consistencyLevel);
    }

    function test_updateAdmin() public {
        // Only the admin can initiate this call.
        vm.startPrank(userA);
        vm.expectRevert(
            abi.encodeWithSelector(ISharedWormholeTransceiver.CallerNotAdmin.selector, userA)
        );
        srcTransceiver.updateAdmin(userB);

        // Can't set the admin to zero.
        vm.startPrank(admin);
        vm.expectRevert(
            abi.encodeWithSelector(ISharedWormholeTransceiver.InvalidAdminZeroAddress.selector)
        );
        srcTransceiver.updateAdmin(address(0));

        // This should work.
        vm.startPrank(admin);
        srcTransceiver.updateAdmin(userA);
    }

    function test_transferAdmin() public {
        // Set up to do a receive below.
        vm.startPrank(admin);
        destTransceiver.setPeer(srcChain, toWormholeFormat(address(srcTransceiver)));

        // Only the admin can initiate this call.
        vm.startPrank(userA);
        vm.expectRevert(
            abi.encodeWithSelector(ISharedWormholeTransceiver.CallerNotAdmin.selector, userA)
        );
        srcTransceiver.transferAdmin(userB);

        // Transferring to address zero should revert.
        vm.startPrank(admin);
        vm.expectRevert(
            abi.encodeWithSelector(ISharedWormholeTransceiver.InvalidAdminZeroAddress.selector)
        );
        srcTransceiver.transferAdmin(address(0));

        // This should work.
        vm.startPrank(admin);
        srcTransceiver.transferAdmin(userA);

        // Attempting to do another transfer when one is in progress should revert.
        vm.startPrank(admin);
        vm.expectRevert(
            abi.encodeWithSelector(ISharedWormholeTransceiver.AdminTransferPending.selector)
        );
        srcTransceiver.transferAdmin(userB);

        // Attempting to update when a transfer is in progress should revert.
        vm.startPrank(admin);
        vm.expectRevert(
            abi.encodeWithSelector(ISharedWormholeTransceiver.AdminTransferPending.selector)
        );
        srcTransceiver.updateAdmin(userB);

        // Attempting to set a peer when a transfer is in progress should revert.
        vm.startPrank(admin);
        vm.expectRevert(
            abi.encodeWithSelector(ISharedWormholeTransceiver.AdminTransferPending.selector)
        );
        srcTransceiver.setPeer(0, toWormholeFormat(address(destTransceiver)));

        vm.startPrank(receiverAddr1);

        // But you can quote the delivery price while a transfer is pending.
        srcTransceiver.quoteDeliveryPrice(peerChain1, emptyInstructions);

        // And you can send a message while a transfer is pending.
        uint16 dstChain = destChain;
        bytes32 dstAddr = toWormholeFormat(address(receiverAddr2));
        bytes memory payload = "Hello, World!";

        srcTransceiver.sendMessage(
            dstChain,
            emptyInstructions,
            buildManagerMessage(42, receiverAddr1, payload),
            dstAddr,
            bytes32(0) // refundAddress
        );

        require(1 == myWormhole.messagesSent(), "VAA did not get sent");

        // And you can receive a message while a transfer is pending.
        destTransceiver.receiveMessage(myWormhole.lastVaa());
    }

    function test_claimAdmin() public {
        // Can't claim when a transfer is not pending.
        vm.startPrank(admin);
        vm.expectRevert(
            abi.encodeWithSelector(ISharedWormholeTransceiver.NoAdminUpdatePending.selector)
        );
        srcTransceiver.claimAdmin();

        // Start a transfer.
        srcTransceiver.transferAdmin(userA);

        // If someone other than the current or pending admin tries to claim, it should revert.
        vm.startPrank(userB);
        vm.expectRevert(
            abi.encodeWithSelector(ISharedWormholeTransceiver.CallerNotAdmin.selector, userB)
        );
        srcTransceiver.claimAdmin();

        // The admin claiming should cancel the transfer.
        vm.startPrank(admin);
        srcTransceiver.claimAdmin();
        require(srcTransceiver.admin() == admin, "cancel set the admin incorrectly");
        require(
            srcTransceiver.pendingAdmin() == address(0), "cancel did not clear the pending admin"
        );

        // The new admin claiming it should work.
        srcTransceiver.transferAdmin(userA);
        vm.startPrank(userA);
        srcTransceiver.claimAdmin();
        require(srcTransceiver.admin() == userA, "transfer set the admin incorrectly");
        require(
            srcTransceiver.pendingAdmin() == address(0), "transfer did not clear the pending admin"
        );
    }

    function test_discardAdmin() public {
        // Only the admin can initiate this call.
        vm.startPrank(userA);
        vm.expectRevert(
            abi.encodeWithSelector(ISharedWormholeTransceiver.CallerNotAdmin.selector, userA)
        );
        srcTransceiver.discardAdmin();

        // This should work.
        vm.startPrank(admin);
        srcTransceiver.discardAdmin();
        require(srcTransceiver.admin() == address(0), "transfer set the admin incorrectly");
        require(
            srcTransceiver.pendingAdmin() == address(0), "transfer did not clear the pending admin"
        );

        // So now the old admin can't do anything.
        vm.expectRevert(
            abi.encodeWithSelector(ISharedWormholeTransceiver.CallerNotAdmin.selector, admin)
        );
        srcTransceiver.updateAdmin(userB);
    }

    function test_setPeer() public {
        // Only the admin can set a peer.
        vm.startPrank(userB);
        vm.expectRevert(
            abi.encodeWithSelector(ISharedWormholeTransceiver.CallerNotAdmin.selector, userB)
        );
        srcTransceiver.setPeer(0, toWormholeFormat(receiverAddr1));

        // Peer chain can't be zero.
        vm.startPrank(admin);
        vm.expectRevert(abi.encodeWithSelector(ISharedWormholeTransceiver.InvalidChain.selector, 0));
        srcTransceiver.setPeer(0, toWormholeFormat(receiverAddr1));

        // Peer contract can't be zero.
        vm.expectRevert(
            abi.encodeWithSelector(ISharedWormholeTransceiver.InvalidPeerZeroAddress.selector)
        );
        srcTransceiver.setPeer(peerChain1, toWormholeFormat(address(0)));

        // This should work.
        srcTransceiver.setPeer(peerChain1, toWormholeFormat(address(receiverAddr1)));

        // You can't set a peer when it's already set.
        vm.expectRevert(
            abi.encodeWithSelector(
                ISharedWormholeTransceiver.PeerAlreadySet.selector, peerChain1, receiverAddr1
            )
        );
        srcTransceiver.setPeer(peerChain1, toWormholeFormat(address(receiverAddr2)));

        // But you can set the peer for another chain.
        srcTransceiver.setPeer(peerChain2, toWormholeFormat(address(receiverAddr2)));

        // Test the getter.
        require(
            srcTransceiver.getPeer(peerChain1) == toWormholeFormat(address(receiverAddr1)),
            "Peer for chain one is wrong"
        );
        require(
            srcTransceiver.getPeer(peerChain2) == toWormholeFormat(address(receiverAddr2)),
            "Peer for chain two is wrong"
        );

        // If you get a peer for a chain that's not set, it reverts.
        vm.expectRevert(
            abi.encodeWithSelector(ISharedWormholeTransceiver.UnregisteredPeer.selector, peerChain3)
        );
        srcTransceiver.getPeer(peerChain3);
    }

    function test_getTransceiverType() public view {
        require(
            keccak256(abi.encodePacked(srcTransceiver.getTransceiverType()))
                == keccak256(abi.encodePacked(sharedWormholeTransceiverVersionString)),
            "transceiver type mismatch"
        );
    }

    function test_quoteDeliveryPrice() public view {
        require(
            srcTransceiver.quoteDeliveryPrice(peerChain1, emptyInstructions)
                == myWormhole.fixedMessageFee(),
            "message fee is wrong"
        );
    }

    function test_sendMessage() public {
        vm.startPrank(admin);
        destTransceiver.setPeer(srcChain, toWormholeFormat(address(srcTransceiver)));

        uint16 dstChain = destChain;
        bytes32 dstAddr = toWormholeFormat(receiverAddr2);
        bytes memory payload = "Hello, World!";
        uint256 deliverPrice = 382;
        bytes memory managerMsg = buildManagerMessage(42, userA, payload);

        // Send the message from receiver1 to receiver2.
        vm.startPrank(receiverAddr1);
        srcTransceiver.sendMessage{value: deliverPrice}(
            dstChain,
            emptyInstructions,
            managerMsg,
            dstAddr,
            bytes32(0) // refundAddress
        );

        // Make sure the VAA went out as expected.
        require(myWormhole.messagesSent() == 1, "Message count is wrong");
        require(myWormhole.lastNonce() == 0, "Nonce is wrong");
        require(myWormhole.lastConsistencyLevel() == consistencyLevel, "Consistency level is wrong");
        require(myWormhole.lastDeliveryPrice() == deliverPrice, "Deliver price is wrong");

        // Receive the message on the destination.
        destTransceiver.receiveMessage(myWormhole.lastVaa());

        // Make sure the destination received the message.
        require(1 == receiver2.numMessages(), "Message not received");

        // Make sure the received message is what we expect.
        MyMsgReceiver.Message memory msg1 = receiver2.getMessage(0);
        require(srcChain == msg1.sourceChainId, "Unexpected sourceChainId");
        require(
            toWormholeFormat(receiverAddr1) == msg1.sourceManagerAddress,
            "Unexpected sourceManagerAddress"
        );
        require(bytes32(uint256(42)) == msg1.payload.id, "Unexpected id");
        require(toWormholeFormat(userA) == msg1.payload.sender, "Unexpected sender");
        require(keccak256(payload) == keccak256(msg1.payload.payload), "Unexpected payload");
    }

    function test_receiveMessage() public {
        // Set the peers on the transceivers.
        vm.startPrank(admin);
        srcTransceiver.setPeer(destChain, toWormholeFormat(address(destTransceiver)));
        destTransceiver.setPeer(srcChain, toWormholeFormat(address(srcTransceiver)));
        otherDestTransceiver.setPeer(srcChain, bytes32(uint256(1)));

        uint16 dstChain = destChain;
        bytes32 dstAddr = toWormholeFormat(receiverAddr2);
        bytes memory payload = "Hello, World!";
        uint256 deliverPrice = 382;
        bytes memory managerMsg = buildManagerMessage(42, userA, payload);

        // Send the message from receiver1 to receiver2.
        vm.startPrank(receiverAddr1);
        srcTransceiver.sendMessage{value: deliverPrice}(
            dstChain,
            emptyInstructions,
            managerMsg,
            dstAddr,
            bytes32(0) // refundAddress
        );

        require(myWormhole.messagesSent() == 1, "Message count is wrong");
        bytes memory vaa = myWormhole.lastVaa();

        // This should work.
        destTransceiver.receiveMessage(vaa);

        // Make sure the destination received the message.
        require(1 == receiver2.numMessages(), "Message not received");

        // Make sure the received message is what we expect.
        MyMsgReceiver.Message memory msg1 = receiver2.getMessage(0);
        require(srcChain == msg1.sourceChainId, "Unexpected sourceChainId");
        require(
            toWormholeFormat(receiverAddr1) == msg1.sourceManagerAddress,
            "Unexpected sourceManagerAddress"
        );
        require(bytes32(uint256(42)) == msg1.payload.id, "Unexpected id");
        require(toWormholeFormat(userA) == msg1.payload.sender, "Unexpected sender");
        require(keccak256(payload) == keccak256(msg1.payload.payload), "Unexpected payload");

        // Can't post it from a chain that isn't registered
        vm.expectRevert(
            abi.encodeWithSelector(ISharedWormholeTransceiver.UnregisteredPeer.selector, srcChain)
        );
        srcTransceiver.receiveMessage(vaa);

        // Can't post it to the wrong transceiver.
        vm.expectRevert(
            abi.encodeWithSelector(
                ISharedWormholeTransceiver.InvalidPeer.selector, srcChain, address(srcTransceiver)
            )
        );
        otherDestTransceiver.receiveMessage(vaa);

        // An invalid VAA should revert.
        destWormhole.setValidFlag(false, "This is bad!");
        vm.expectRevert(
            abi.encodeWithSelector(ISharedWormholeTransceiver.InvalidVaa.selector, "This is bad!")
        );
        destTransceiver.receiveMessage(vaa);
        destWormhole.setValidFlag(true, "");
    }

    function test_getPeers() public {
        vm.startPrank(admin);

        require(0 == srcTransceiver.getPeers().length, "Initial peers should be zero");

        bytes32 peerAddr1 = toWormholeFormat(address(receiverAddr1));
        bytes32 peerAddr2 = toWormholeFormat(address(receiverAddr2));

        srcTransceiver.setPeer(peerChain1, peerAddr1);
        ISharedWormholeTransceiver.PeerEntry[] memory peers = srcTransceiver.getPeers();
        require(1 == peers.length, "Should be one peer");
        require(peers[0].chain == peerChain1, "Chain is wrong");
        require(peers[0].addr == peerAddr1, "Address is wrong");

        srcTransceiver.setPeer(peerChain2, peerAddr2);
        peers = srcTransceiver.getPeers();
        require(2 == peers.length, "Should be one peer");
        require(peers[0].chain == peerChain1, "First chain is wrong");
        require(peers[0].addr == peerAddr1, "First address is wrong");
        require(peers[1].chain == peerChain2, "Second chain is wrong");
        require(peers[1].addr == peerAddr2, "Second address is wrong");
    }

    function buildManagerMessage(
        uint64 sequence,
        address sender,
        bytes memory payload
    ) public pure returns (bytes memory) {
        return TransceiverStructs.encodeNttManagerMessage(
            TransceiverStructs.NttManagerMessage(
                bytes32(uint256(sequence)), toWormholeFormat(sender), payload
            )
        );
    }
}
