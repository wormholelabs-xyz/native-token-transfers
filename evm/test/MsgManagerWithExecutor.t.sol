// SPDX-License-Identifier: Apache 2
pragma solidity >=0.8.8 <0.9.0;

import "forge-std/Test.sol";
import "forge-std/console.sol";
import "openzeppelin-contracts/contracts/proxy/ERC1967/ERC1967Proxy.sol";
import "example-messaging-executor/evm/src/Executor.sol";
import "example-messaging-executor/evm/src/interfaces/IExecutor.sol";
import "wormhole-solidity-sdk/Utils.sol";

import "./libraries/TransceiverHelpers.sol";
import "../src/interfaces/IMsgManagerWithExecutor.sol";
import "../src/libraries/TransceiverStructs.sol";
import "../src/NttManager/MsgManagerWithExecutor.sol";

/// @dev MyMsgManagerWithExecutor is what an integrator might implement. They would override _handleMsg().
contract MyMsgManagerWithExecutor is MsgManagerWithExecutor {
    struct Message {
        uint16 sourceChainId;
        bytes32 sourceManagerAddress;
        bytes payload;
        bytes32 digest;
    }

    Message[] private messages;

    constructor(uint16 chainId, address executor) MsgManagerWithExecutor(chainId, executor) {}

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

contract MockExecutor is IExecutor {
    struct Request {
        uint16 dstChain;
        bytes32 dstAddr;
        address refundAddr;
        bytes signedQuote;
        bytes requestBytes;
        bytes relayInstructions;
    }

    uint16 public immutable chainId;
    Request[] private requests;

    constructor(
        uint16 _chainId
    ) {
        chainId = _chainId;
    }

    // NOTE: This was copied from the tests in the executor repo.
    function encodeSignedQuoteHeader(
        Executor.SignedQuoteHeader memory signedQuote
    ) public pure returns (bytes memory) {
        return abi.encodePacked(
            signedQuote.prefix,
            signedQuote.quoterAddress,
            signedQuote.payeeAddress,
            signedQuote.srcChain,
            signedQuote.dstChain,
            signedQuote.expiryTime
        );
    }

    function createSignedQuote(
        uint16 dstChain
    ) public view returns (bytes memory) {
        return createSignedQuote(dstChain, 60);
    }

    function createSignedQuote(
        uint16 dstChain,
        uint64 quoteLife
    ) public view returns (bytes memory) {
        Executor.SignedQuoteHeader memory signedQuote = IExecutor.SignedQuoteHeader({
            prefix: "EQ01",
            quoterAddress: address(0),
            payeeAddress: bytes32(0),
            srcChain: chainId,
            dstChain: dstChain,
            expiryTime: uint64(block.timestamp + quoteLife)
        });
        return encodeSignedQuoteHeader(signedQuote);
    }

    function createExecutorInstructions() public pure returns (bytes memory) {
        return new bytes(0);
    }

    function createArgs(
        uint16 dstChain
    ) public view returns (IMsgManagerWithExecutor.ExecutorArgs memory args) {
        args.refundAddress = msg.sender;
        args.signedQuote = createSignedQuote(dstChain);
        args.instructions = createExecutorInstructions();
    }

    function numRequests() public view returns (uint256) {
        return requests.length;
    }

    function geRequest(
        uint256 idx
    ) public view returns (Request memory) {
        return requests[idx];
    }

    function requestExecution(
        uint16 dstChain,
        bytes32 dstAddr,
        address refundAddr,
        bytes calldata signedQuote,
        bytes calldata requestBytes,
        bytes calldata relayInstructions
    ) external payable {
        requests.push(
            Request(dstChain, dstAddr, refundAddr, signedQuote, requestBytes, relayInstructions)
        );
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

        IMsgManagerWithExecutor(
            fromWormholeFormat(parsedTransceiverMessage.recipientNttManagerAddress)
        ).attestationReceived(
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

contract TestMsgManagerWithExecutor is Test {
    MockExecutor executor;
    MockExecutor peerExecutor;
    MyMsgManagerWithExecutor msgManagerWithExecutor;
    MyMsgManagerWithExecutor peerMsgManagerWithExecutor;

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

        executor = new MockExecutor(chainId1);
        peerExecutor = new MockExecutor(chainId2);

        MsgManagerWithExecutor implementation =
            new MyMsgManagerWithExecutor(chainId1, address(executor));
        msgManagerWithExecutor =
            MyMsgManagerWithExecutor(address(new ERC1967Proxy(address(implementation), "")));
        msgManagerWithExecutor.initialize();
        transceiver = new MockTransceiver(address(msgManagerWithExecutor));
        msgManagerWithExecutor.setTransceiver(address(transceiver));

        MsgManagerWithExecutor peerImplementation =
            new MyMsgManagerWithExecutor(chainId2, address(peerExecutor));
        peerMsgManagerWithExecutor =
            MyMsgManagerWithExecutor(address(new ERC1967Proxy(address(peerImplementation), "")));
        peerMsgManagerWithExecutor.initialize();
        peerTransceiver = new MockTransceiver(address(peerMsgManagerWithExecutor));
        peerMsgManagerWithExecutor.setTransceiver(address(peerTransceiver));

        msgManagerWithExecutor.setPeer(
            chainId2, toWormholeFormat(address(peerMsgManagerWithExecutor))
        );
        peerMsgManagerWithExecutor.setPeer(
            chainId1, toWormholeFormat(address(msgManagerWithExecutor))
        );
    }

    function testMsgManagerWithExecutorBasic() public {
        vm.startPrank(user_A);

        bytes memory transceiverInstructions = encodeEmptyTransceiverInstructions();
        bytes memory payload1 = "Hi, Mom!";
        bytes memory payload2 = "Hello, World!";
        bytes memory payload3 = "Farewell, Cruel World!";
        IMsgManagerWithExecutor.ExecutorArgs memory executorArgs = executor.createArgs(chainId2);
        uint64 s1 = msgManagerWithExecutor.sendMessage(
            chainId2, payload1, transceiverInstructions, executorArgs
        );
        uint64 s2 = msgManagerWithExecutor.sendMessage(
            chainId2, payload2, transceiverInstructions, executorArgs
        );
        uint64 s3 = msgManagerWithExecutor.sendMessage(
            chainId2, payload3, transceiverInstructions, executorArgs
        );
        vm.stopPrank();

        // Verify our sequence number increases as expected.
        assertEq(s1, 0);
        assertEq(s2, 1);
        assertEq(s3, 2);

        // Verify we sent the messages.
        assertEq(transceiver.numMessages(), 3);

        // Verify the executor received three requests.
        assertEq(executor.numRequests(), 3);

        // Receive the first message on the transceiver. (The executor would do this. . .)
        bytes memory msg1 = transceiver.getMessage(0);
        peerTransceiver.receiveMessage(chainId1, msg1);
        assertEq(peerMsgManagerWithExecutor.numMessages(), 1);
        assertEq(keccak256(payload1), keccak256(peerMsgManagerWithExecutor.getMessage(0).payload));

        // Receive and verify the second message.
        bytes memory msg2 = transceiver.getMessage(1);
        peerTransceiver.receiveMessage(chainId1, msg2);
        assertEq(peerMsgManagerWithExecutor.numMessages(), 2);
        assertEq(keccak256(payload2), keccak256(peerMsgManagerWithExecutor.getMessage(1).payload));

        // Receive and verify the third message.
        bytes memory msg3 = transceiver.getMessage(2);
        peerTransceiver.receiveMessage(chainId1, msg3);
        assertEq(peerMsgManagerWithExecutor.numMessages(), 3);
        assertEq(keccak256(payload3), keccak256(peerMsgManagerWithExecutor.getMessage(2).payload));
    }

    function encodeEmptyTransceiverInstructions() internal pure returns (bytes memory) {
        TransceiverStructs.TransceiverInstruction[] memory instructions =
            new TransceiverStructs.TransceiverInstruction[](0);
        return TransceiverStructs.encodeTransceiverInstructions(instructions);
    }
}
