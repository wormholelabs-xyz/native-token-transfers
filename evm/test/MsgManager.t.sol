// SPDX-License-Identifier: Apache 2
pragma solidity >=0.8.8 <0.9.0;

import "forge-std/Test.sol";
import "forge-std/console.sol";
import "openzeppelin-contracts/contracts/proxy/ERC1967/ERC1967Proxy.sol";
import "wormhole-solidity-sdk/Utils.sol";

import "./libraries/TransceiverHelpers.sol";
import "../src/interfaces/IMsgManager.sol";
import "../src/libraries/TransceiverStructs.sol";
import "../src/NttManager/MsgManager.sol";

/// @dev MyMsgManager is what an integrator might implement. They would override _handleMsg().
contract MyMsgManager is MsgManager {
    struct Message {
        uint16 sourceChainId;
        bytes32 sourceManagerAddress;
        bytes payload;
        bytes32 digest;
    }

    Message[] private messages;

    constructor(
        uint16 chainId
    ) MsgManager(chainId) {}

    function numMessages() public view returns (uint256) {
        return messages.length;
    }

    function getMessage(
        uint256 idx
    ) public view returns (Message memory) {
        return messages[idx];
    }

    function _handleMsg(
        uint16 sourceChainId,
        bytes32 sourceManagerAddress,
        TransceiverStructs.NttManagerMessage memory message,
        bytes32 digest
    ) internal virtual override {
        messages.push(Message(sourceChainId, sourceManagerAddress, message.payload, digest));
    }
}

contract MockTransceiver is Transceiver {
    bytes4 constant TEST_TRANSCEIVER_PAYLOAD_PREFIX = 0x99455454;

    bytes[] private messages;

    constructor(
        address nttManager
    ) Transceiver(nttManager) {}

    function getTransceiverType() external pure override returns (string memory) {
        return "dummy";
    }

    function _quoteDeliveryPrice(
        uint16, /* recipientChain */
        TransceiverStructs.TransceiverInstruction memory /* transceiverInstruction */
    ) internal pure override returns (uint256) {
        return 0;
    }

    function _sendMessage(
        uint16, /* recipientChain */
        uint256, /* deliveryPayment */
        address, /* caller */
        bytes32 recipientNttManagerAddress,
        bytes32, /* refundAddres */
        TransceiverStructs.TransceiverInstruction memory, /* instruction */
        bytes memory nttManagerMessage
    ) internal override {
        bytes memory encodedEm;
        (, encodedEm) = TransceiverStructs.buildAndEncodeTransceiverMessage(
            TEST_TRANSCEIVER_PAYLOAD_PREFIX,
            toWormholeFormat(address(nttManager)),
            recipientNttManagerAddress,
            nttManagerMessage,
            new bytes(0) // TODO: encode instructions
        );

        messages.push(encodedEm);
    }

    function receiveMessage(uint16 sourceChainId, bytes memory encodedMessage) external {
        TransceiverStructs.TransceiverMessage memory parsedTransceiverMessage;
        TransceiverStructs.NttManagerMessage memory parsedNttManagerMessage;
        (parsedTransceiverMessage, parsedNttManagerMessage) = TransceiverStructs
            .parseTransceiverAndNttManagerMessage(TEST_TRANSCEIVER_PAYLOAD_PREFIX, encodedMessage);

        IMsgManager(fromWormholeFormat(parsedTransceiverMessage.recipientNttManagerAddress))
            .attestationReceived(
            sourceChainId, parsedTransceiverMessage.sourceNttManagerAddress, parsedNttManagerMessage
        );
    }

    function numMessages() public view returns (uint256) {
        return messages.length;
    }

    function getMessage(
        uint256 idx
    ) public view returns (bytes memory) {
        return messages[idx];
    }
}

contract TestMsgManager is Test {
    MyMsgManager msgManager;
    MyMsgManager peerMsgManager;

    uint16 constant chainId1 = 7;
    uint16 constant chainId2 = 8;

    address user_A = address(0x123);
    address user_B = address(0x456);

    uint256 initialBlockTimestamp;
    MockTransceiver transceiver;
    MockTransceiver peerTransceiver;

    function setUp() public {
        string memory url = "https://ethereum-sepolia-rpc.publicnode.com";
        vm.createSelectFork(url);
        initialBlockTimestamp = vm.getBlockTimestamp();

        MsgManager implementation = new MyMsgManager(chainId1);
        msgManager = MyMsgManager(address(new ERC1967Proxy(address(implementation), "")));
        msgManager.initialize();
        transceiver = new MockTransceiver(address(msgManager));
        msgManager.setTransceiver(address(transceiver));

        MsgManager peerImplementation = new MyMsgManager(chainId2);
        peerMsgManager = MyMsgManager(address(new ERC1967Proxy(address(peerImplementation), "")));
        peerMsgManager.initialize();
        peerTransceiver = new MockTransceiver(address(peerMsgManager));
        peerMsgManager.setTransceiver(address(peerTransceiver));

        msgManager.setPeer(chainId2, toWormholeFormat(address(peerMsgManager)));
        peerMsgManager.setPeer(chainId1, toWormholeFormat(address(msgManager)));
    }

    function testMsgManagerBasic() public {
        vm.startPrank(user_A);

        bytes memory transceiverInstructions = encodeEmptyTransceiverInstructions();
        bytes memory payload1 = "Hi, Mom!";
        bytes memory payload2 = "Hello, World!";
        bytes memory payload3 = "Farewell, Cruel World!";
        uint64 s1 = msgManager.sendMessage(chainId2, payload1, transceiverInstructions);
        uint64 s2 = msgManager.sendMessage(chainId2, payload2, transceiverInstructions);
        uint64 s3 = msgManager.sendMessage(chainId2, payload3, transceiverInstructions);
        vm.stopPrank();

        // Verify our sequence number increases as expected.
        assertEq(s1, 0);
        assertEq(s2, 1);
        assertEq(s3, 2);

        // Verify we sent the messages.
        assertEq(transceiver.numMessages(), 3);

        // Receive and verify the first message.
        bytes memory msg1 = transceiver.getMessage(0);
        peerTransceiver.receiveMessage(chainId1, msg1);
        assertEq(peerMsgManager.numMessages(), 1);
        assertEq(keccak256(payload1), keccak256(peerMsgManager.getMessage(0).payload));

        // Receive and verify the second message.
        bytes memory msg2 = transceiver.getMessage(1);
        peerTransceiver.receiveMessage(chainId1, msg2);
        assertEq(peerMsgManager.numMessages(), 2);
        assertEq(keccak256(payload2), keccak256(peerMsgManager.getMessage(1).payload));

        // Receive and verify the third message.
        bytes memory msg3 = transceiver.getMessage(2);
        peerTransceiver.receiveMessage(chainId1, msg3);
        assertEq(peerMsgManager.numMessages(), 3);
        assertEq(keccak256(payload3), keccak256(peerMsgManager.getMessage(2).payload));
    }

    function testMsgManagerWithThreshold2() public {
        // Add a second transceiver to the receiver and increase the threshold.
        MockTransceiver peerTransceiver2 = new MockTransceiver(address(peerMsgManager));
        peerMsgManager.setTransceiver(address(peerTransceiver2));
        peerMsgManager.setThreshold(2);

        vm.startPrank(user_A);

        // Send a message.
        bytes memory transceiverInstructions = encodeEmptyTransceiverInstructions();
        bytes memory payload = "Hi, Mom!";
        assertEq(msgManager.sendMessage(chainId2, payload, transceiverInstructions), 0);
        assertEq(transceiver.numMessages(), 1);

        // Receive the message on the first transceiver and verify that the manager didn't receive it yet.
        bytes memory msg1 = transceiver.getMessage(0);
        peerTransceiver.receiveMessage(chainId1, msg1);
        assertEq(peerMsgManager.numMessages(), 0);

        // Receive the message on the second transceiver and verify that the manager did now receive it.
        peerTransceiver2.receiveMessage(chainId1, msg1);
        assertEq(peerMsgManager.numMessages(), 1);
        assertEq(keccak256(payload), keccak256(peerMsgManager.getMessage(0).payload));
    }

    function encodeEmptyTransceiverInstructions() internal pure returns (bytes memory) {
        TransceiverStructs.TransceiverInstruction[] memory instructions =
            new TransceiverStructs.TransceiverInstruction[](0);
        return TransceiverStructs.encodeTransceiverInstructions(instructions);
    }
}
