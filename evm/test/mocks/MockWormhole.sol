// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.13;

import {Test, console} from "forge-std/Test.sol";
import "wormhole-solidity-sdk/Utils.sol";
import "wormhole-solidity-sdk/interfaces/IWormhole.sol";

contract MockWormhole {
    uint256 public constant fixedMessageFee = 250;

    uint16 public immutable ourChain;

    bool public validFlag;
    string public invalidReason;

    // These are incremented on calls.
    uint256 public messagesSent;
    uint32 public seqSent;

    // These are set on calls.
    uint256 public lastDeliveryPrice;
    uint32 public lastNonce;
    uint8 public lastConsistencyLevel;
    bytes public lastPayload;
    bytes public lastVaa;
    bytes32 public lastVaaHash;

    constructor(
        uint16 _ourChain
    ) {
        ourChain = _ourChain;
        validFlag = true;
    }

    function setValidFlag(bool v, string memory reason) external {
        validFlag = v;
        invalidReason = reason;
    }

    function messageFee() external pure returns (uint256) {
        return fixedMessageFee;
    }

    function publishMessage(
        uint32 nonce,
        bytes memory payload,
        uint8 consistencyLevel
    ) external payable returns (uint64 sequence) {
        seqSent += 1;
        sequence = seqSent;

        lastDeliveryPrice = msg.value;
        lastNonce = nonce;
        lastConsistencyLevel = consistencyLevel;
        lastPayload = payload;
        messagesSent += 1;

        bytes32 sender = toWormholeFormat(msg.sender);
        bytes32 hash = keccak256(payload);

        lastVaa = abi.encode(ourChain, sender, sequence, hash, payload);
        lastVaaHash = hash;
    }

    function parseAndVerifyVM(
        bytes calldata encodedVM
    ) external view returns (IWormhole.VM memory vm, bool valid, string memory reason) {
        valid = validFlag;
        reason = invalidReason;

        // These are the fields that the transceiver uses:
        // vm.emitterChainId
        // vm.emitterAddress
        // vm.hash
        // vm.payload

        (vm.emitterChainId, vm.emitterAddress, vm.sequence, vm.hash, vm.payload) =
            abi.decode(encodedVM, (uint16, bytes32, uint64, bytes32, bytes));
    }
}
