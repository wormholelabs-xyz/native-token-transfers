// SPDX-License-Identifier: Apache 2
pragma solidity >=0.8.8 <0.9.0;

import "../libraries/TransceiverStructs.sol";

interface IMsgReceiver {
    /// @notice Called by an Endpoint contract to deliver a verified attestation.
    /// @dev This function enforces attestation threshold and replay logic for messages. Once all
    ///      validations are complete, this function calls `executeMsg` to execute the command specified
    ///      by the message.
    /// @param sourceChainId The Wormhole chain id of the sender.
    /// @param sourceNttManagerAddress The address of the sender's NTT Manager contract.
    /// @param payload The VAA payload.
    function attestationReceived(
        uint16 sourceChainId,
        bytes32 sourceNttManagerAddress,
        TransceiverStructs.NttManagerMessage memory payload
    ) external;

    /// @notice Called after a message has been sufficiently verified to execute
    ///         the command in the message. This function will decode the payload
    ///         as an NttManagerMessage to extract the sequence, msgType, and other parameters.
    /// @dev This function is exposed as a fallback for when an `Transceiver` is deregistered
    ///      when a message is in flight.
    /// @param sourceChainId The Wormhole chain id of the sender.
    /// @param sourceNttManagerAddress The address of the sender's nttManager contract.
    /// @param message The message to execute.
    function executeMsg(
        uint16 sourceChainId,
        bytes32 sourceNttManagerAddress,
        TransceiverStructs.NttManagerMessage memory message
    ) external;
}
