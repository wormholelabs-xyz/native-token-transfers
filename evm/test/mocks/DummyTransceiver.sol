// SPDX-License-Identifier: Apache 2

pragma solidity >=0.8.8 <0.9.0;

import "forge-std/Test.sol";

import "example-gmp-router/evm/src/interfaces/IRouterTransceiver.sol";
import "example-gmp-router/evm/src/interfaces/ITransceiver.sol";

contract DummyTransceiver is ITransceiver {
    uint16 public immutable chainId;
    address public immutable router;
    bytes4 constant TEST_TRANSCEIVER_PAYLOAD_PREFIX = 0x99455454;

    constructor(uint16 _chainId, address _router) {
        chainId = _chainId;
        router = _router;
    }

    function getTransceiverType() external pure returns (string memory) {
        return "dummy";
    }

    function quoteDeliveryPrice(
        uint16 /* recipientChain */
    ) external pure returns (uint256) {
        return 0;
    }

    struct Message {
        uint16 srcChain;
        UniversalAddress srcAddr;
        uint64 sequence;
        uint16 dstChain;
        UniversalAddress dstAddr;
        bytes32 payloadHash;
        address refundAddr;
    }

    Message[] public messages;

    function getMessages() external view returns (Message[] memory) {
        return messages;
    }

    function sendMessage(
        UniversalAddress srcAddr,
        uint64 sequence,
        uint16 dstChain,
        UniversalAddress dstAddr,
        bytes32 payloadHash,
        address refundAddr
    ) external payable {
        Message memory m = Message({
            srcChain: chainId,
            srcAddr: srcAddr,
            sequence: sequence,
            dstChain: dstChain,
            dstAddr: dstAddr,
            payloadHash: payloadHash,
            refundAddr: refundAddr
        });
        messages.push(m);
    }

    function receiveMessage(
        Message memory m
    ) external {
        IRouterTransceiver(router).attestMessage(
            m.srcChain, m.srcAddr, m.sequence, m.dstChain, m.dstAddr, m.payloadHash
        );
    }
}
