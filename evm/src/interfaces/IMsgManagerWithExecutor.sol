// SPDX-License-Identifier: Apache 2
pragma solidity >=0.8.8 <0.9.0;

import "../libraries/TrimmedAmount.sol";
import "../libraries/TransceiverStructs.sol";

import "./IManagerBase.sol";

interface IMsgManagerWithExecutor is IManagerBase {
    /// @dev The peer on another chain.
    struct MsgManagerPeer {
        bytes32 peerAddress;
    }

    /// @dev Instructions for the executor to perform relaying.
    struct ExecutorArgs {
        // The msg value to be passed into the Executor.
        uint256 value;
        // The refund address used by the Executor.
        address refundAddress;
        // The signed quote to be passed into the Executor.
        bytes signedQuote;
        // The relay instructions to be passed into the Executor.
        bytes instructions;
    }

    /// @notice Emitted when the peer contract is updated.
    /// @dev Topic0
    ///      TODO.
    /// @param chainId_ The chain ID of the peer contract.
    /// @param oldPeerContract The old peer contract address.
    /// @param peerContract The new peer contract address.
    event PeerUpdated(uint16 indexed chainId_, bytes32 oldPeerContract, bytes32 peerContract);

    /// @notice Emitted when a message is sent from the manager.
    /// @dev Topic0
    ///      0xe54e51e42099622516fa3b48e9733581c9dbdcb771cafb093f745a0532a35982.
    /// @param recipientChain The chain ID of the recipient.
    /// @param recipientAddress The recipient of the message.
    /// @param sequence The unique sequence ID of the message.
    /// @param fee The amount of ether sent along with the tx to cover the delivery fee.
    event MessageSent(
        uint16 recipientChain, bytes32 indexed recipientAddress, uint64 sequence, uint256 fee
    );

    /// @notice Error when trying to execute a message on an unintended target chain.
    /// @dev Selector 0x3dcb204a.
    /// @param targetChain The target chain.
    /// @param thisChain The current chain.
    error InvalidTargetChain(uint16 targetChain, uint16 thisChain);

    /// @notice Peer for the chain does not match the configuration.
    /// @param chainId ChainId of the source chain.
    /// @param peerAddress Address of the peer nttManager contract.
    error InvalidPeer(uint16 chainId, bytes32 peerAddress);

    /// @notice Peer chain ID cannot be zero.
    error InvalidPeerChainIdZero();

    /// @notice Peer cannot be the zero address.
    error InvalidPeerZeroAddress();

    /// @notice Peer cannot be on the same chain
    /// @dev Selector 0x20371f2a.
    error InvalidPeerSameChainId();

    /// @notice The caller is not the deployer.
    error UnexpectedDeployer(address expectedOwner, address owner);

    /// @notice An unexpected msg.value was passed with the call
    /// @dev Selector 0xbd28e889.
    error UnexpectedMsgValue();

    /// @notice The message value is insufficient.
    /// @dev Selector TODO.
    error InsufficientMsgValue(uint256 value, uint256 totalPriceQuote, uint256 executorValue);

    /// @notice Sends a message to the remote peer on the specified recipient chain.
    /// @dev This function enforces attestation threshold and replay logic for messages. Once all
    ///      validations are complete, this function calls `executeMsg` to execute the command specified
    ///      by the message.
    /// @param recipientChain The Wormhole chain id of the recipient.
    /// @param refundAddress The refund address on the recipient chain.
    /// @param transceiverInstructions Instructions to be passed to the transceiver, if any.
    /// @param payload The message to be sent.
    function sendMessage(
        uint16 recipientChain,
        bytes32 refundAddress,
        bytes calldata payload,
        bytes memory transceiverInstructions,
        ExecutorArgs calldata executorArgs
    ) external payable returns (uint64);

    /// @notice Returns registered peer contract for a given chain.
    /// @param chainId_ Wormhole chain ID.
    function getPeer(
        uint16 chainId_
    ) external view returns (MsgManagerPeer memory);

    /// @notice Sets the corresponding peer.
    /// @dev The msgManager that executes the message sets the source msgManager as the peer.
    /// @param peerChainId The Wormhole chain ID of the peer.
    /// @param peerContract The address of the peer nttManager contract.
    ///        Set to zero if not needed.
    function setPeer(uint16 peerChainId, bytes32 peerContract) external;
}
