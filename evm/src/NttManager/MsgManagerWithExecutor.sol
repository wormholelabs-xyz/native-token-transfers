// SPDX-License-Identifier: Apache 2
pragma solidity >=0.8.8 <0.9.0;

import "example-messaging-executor/evm/src/interfaces/IExecutor.sol";
import "example-messaging-executor/evm/src/libraries/ExecutorMessages.sol";
import "wormhole-solidity-sdk/Utils.sol";
import "wormhole-solidity-sdk/libraries/BytesParsing.sol";

import "../interfaces/IMsgManagerWithExecutor.sol";
import "../interfaces/ITransceiver.sol";
import "../libraries/TransceiverHelpers.sol";

import {ManagerBase} from "./ManagerBase.sol";

contract MsgManagerWithExecutor is IMsgManagerWithExecutor, ManagerBase {
    string public constant MSG_MANAGER_VERSION = "1.0.0";

    IExecutor public immutable executor;

    // =============== Setup =================================================================

    constructor(
        uint16 _chainId,
        address _executor
    ) ManagerBase(address(0), Mode.LOCKING, _chainId) {
        assert(_executor != address(0));
        executor = IExecutor(_executor);
    }

    function _initialize() internal virtual override {
        _init();
        _checkThresholdInvariants();
        _checkTransceiversInvariants();
    }

    function _init() internal onlyInitializing {
        // check if the owner is the deployer of this contract
        if (msg.sender != deployer) {
            revert UnexpectedDeployer(deployer, msg.sender);
        }
        if (msg.value != 0) {
            revert UnexpectedMsgValue();
        }
        __PausedOwnable_init(msg.sender, msg.sender);
        __ReentrancyGuard_init();
    }

    // =============== Storage ==============================================================

    bytes32 private constant PEERS_SLOT = bytes32(uint256(keccak256("mmgr.peers")) - 1);

    // =============== Storage Getters/Setters ==============================================

    function _getPeersStorage()
        internal
        pure
        returns (mapping(uint16 => MsgManagerPeer) storage $)
    {
        uint256 slot = uint256(PEERS_SLOT);
        assembly ("memory-safe") {
            $.slot := slot
        }
    }

    // =============== Public Getters ========================================================

    /// @inheritdoc IMsgManagerWithExecutor
    function getPeer(
        uint16 chainId_
    ) external view returns (MsgManagerPeer memory) {
        return _getPeersStorage()[chainId_];
    }

    // =============== Admin ==============================================================

    /// @inheritdoc IMsgManagerWithExecutor
    function setPeer(uint16 peerChainId, bytes32 peerAddress) public onlyOwner {
        if (peerChainId == 0) {
            revert InvalidPeerChainIdZero();
        }
        if (peerAddress == bytes32(0)) {
            revert InvalidPeerZeroAddress();
        }
        if (peerChainId == chainId) {
            revert InvalidPeerSameChainId();
        }

        MsgManagerPeer memory oldPeer = _getPeersStorage()[peerChainId];

        _getPeersStorage()[peerChainId].peerAddress = peerAddress;

        emit PeerUpdated(peerChainId, oldPeer.peerAddress, peerAddress);
    }

    /// ============== Invariants =============================================

    /// @dev When we add new immutables, this function should be updated
    function _checkImmutables() internal view virtual override {
        super._checkImmutables();
    }

    // ==================== External Interface ===============================================

    /// @inheritdoc IMsgManagerWithExecutor
    function sendMessage(
        uint16 recipientChain,
        bytes32 refundAddress,
        bytes calldata payload,
        bytes memory transceiverInstructions,
        ExecutorArgs calldata executorArgs
    ) external payable nonReentrant whenNotPaused returns (uint64 sequence) {
        sequence = _useMessageSequence();

        bytes32 recipientAddress = _getPeersStorage()[recipientChain].peerAddress;

        (uint256 totalPriceQuote,) = _sendMessage(
            sequence,
            recipientChain,
            recipientAddress,
            refundAddress,
            msg.sender,
            payload,
            transceiverInstructions
        );

        if (totalPriceQuote + executorArgs.value > msg.value) {
            revert InsufficientMsgValue(msg.value, totalPriceQuote, executorArgs.value);
        }

        // emit MessageSent(recipientChain, recipientAddress, sequence, totalPriceQuote);

        // Generate the executor event.
        // TODO: Not sure we want to use `makeNTTv1Request` since it doesn't have the payload.
        executor.requestExecution{value: executorArgs.value}(
            recipientChain,
            recipientAddress,
            executorArgs.refundAddress,
            executorArgs.signedQuote,
            ExecutorMessages.makeNTTv1Request(
                chainId, bytes32(uint256(uint160(address(this)))), bytes32(uint256(sequence))
            ),
            executorArgs.instructions
        );
    }

    /// @dev Override this function to handle your messages.
    function _handleMsg(
        uint16 sourceChainId,
        bytes32 sourceManagerAddress,
        TransceiverStructs.NttManagerMessage memory message,
        bytes32 digest
    ) internal virtual override {}

    // ==================== Internal Helpers ===============================================

    /// @dev Verify that the peer address saved for `sourceChainId` matches the `peerAddress`.
    function _verifyPeer(uint16 sourceChainId, bytes32 peerAddress) internal view override {
        if (_getPeersStorage()[sourceChainId].peerAddress != peerAddress) {
            revert InvalidPeer(sourceChainId, peerAddress);
        }
    }
}
