// SPDX-License-Identifier: Apache 2
pragma solidity >=0.8.8 <0.9.0;

import "wormhole-solidity-sdk/Utils.sol";
import "wormhole-solidity-sdk/libraries/BytesParsing.sol";

import "../interfaces/IMsgReceiver.sol";
import "../interfaces/ITransceiver.sol";
import "../libraries/TransceiverHelpers.sol";

import "./ManagerBase.sol";

abstract contract MsgManagerBase is ManagerBase, IMsgReceiver {
    // =============== Setup =================================================================

    constructor(address _token, Mode _mode, uint16 _chainId) ManagerBase(_token, _mode, _chainId) {}

    // ==================== External Interface ===============================================

    function attestationReceived(
        uint16 sourceChainId,
        bytes32 sourceManagerAddress,
        TransceiverStructs.NttManagerMessage memory payload
    ) external onlyTransceiver whenNotPaused {
        _verifyPeer(sourceChainId, sourceManagerAddress);

        // Compute manager message digest and record transceiver attestation.
        bytes32 nttManagerMessageHash = _recordTransceiverAttestation(sourceChainId, payload);

        if (isMessageApproved(nttManagerMessageHash)) {
            executeMsg(sourceChainId, sourceManagerAddress, payload);
        }
    }

    function executeMsg(
        uint16 sourceChainId,
        bytes32 sourceNttManagerAddress,
        TransceiverStructs.NttManagerMessage memory message
    ) public whenNotPaused {
        (bytes32 digest, bool alreadyExecuted) =
            _isMessageExecuted(sourceChainId, sourceNttManagerAddress, message);

        if (alreadyExecuted) {
            return;
        }

        _handleMsg(sourceChainId, sourceNttManagerAddress, message, digest);
    }

    // ==================== Internal Helpers ===============================================

    /// @dev Override this function to handle your messages.
    function _handleMsg(
        uint16 sourceChainId,
        bytes32 sourceManagerAddress,
        TransceiverStructs.NttManagerMessage memory message,
        bytes32 digest
    ) internal virtual {}

    function _sendMessage(
        uint16 recipientChain,
        bytes32 recipientManagerAddress,
        uint64 sequence,
        bytes memory payload,
        bytes memory transceiverInstructions
    ) internal returns (uint256 totalPriceQuote, bytes memory encodedNttManagerPayload) {
        // verify chain has not forked
        checkFork(evmChainId);

        address[] memory enabledTransceivers;
        TransceiverStructs.TransceiverInstruction[] memory instructions;
        uint256[] memory priceQuotes;
        (enabledTransceivers, instructions, priceQuotes, totalPriceQuote) =
            _prepareForTransfer(recipientChain, transceiverInstructions);
        (recipientChain, transceiverInstructions);

        // construct the NttManagerMessage payload
        encodedNttManagerPayload = TransceiverStructs.encodeNttManagerMessage(
            TransceiverStructs.NttManagerMessage(
                bytes32(uint256(sequence)), toWormholeFormat(msg.sender), payload
            )
        );

        // send the message
        _sendMessageToTransceivers(
            recipientChain,
            recipientManagerAddress, // refundAddress
            recipientManagerAddress,
            priceQuotes,
            instructions,
            enabledTransceivers,
            encodedNttManagerPayload
        );
    }

    /// @dev Verify that the peer address saved for `sourceChainId` matches the `peerAddress`.
    function _verifyPeer(uint16 sourceChainId, bytes32 peerAddress) internal view virtual;
}
