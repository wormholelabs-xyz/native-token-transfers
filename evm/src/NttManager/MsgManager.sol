// SPDX-License-Identifier: Apache 2
pragma solidity >=0.8.8 <0.9.0;

import "wormhole-solidity-sdk/Utils.sol";
import "wormhole-solidity-sdk/libraries/BytesParsing.sol";

import "../interfaces/IMsgManager.sol";
import "../interfaces/ITransceiver.sol";
import "../libraries/TransceiverHelpers.sol";

import {ManagerBase} from "./ManagerBase.sol";

contract MsgManager is IMsgManager, ManagerBase {
    string public constant MSG_MANAGER_VERSION = "1.0.0";

    // =============== Setup =================================================================

    constructor(
        uint16 _chainId
    ) ManagerBase(address(0), Mode.LOCKING, _chainId) {}

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

    /// @inheritdoc IMsgManager
    function getPeer(
        uint16 chainId_
    ) external view returns (MsgManagerPeer memory) {
        return _getPeersStorage()[chainId_];
    }

    // =============== Admin ==============================================================

    /// @inheritdoc IMsgManager
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

    /// @inheritdoc IMsgManager
    function sendMessage(
        uint16 recipientChain,
        bytes32 refundAddress,
        bytes calldata payload,
        bytes memory transceiverInstructions
    ) external payable nonReentrant whenNotPaused returns (uint64 sequence) {
        sequence = _useMessageSequence();

        bytes32 recipientAddress = _getPeersStorage()[recipientChain].peerAddress;

        (uint256 totalPriceQuote, bytes memory encodedNttManagerPayload) = _sendMessage(
            sequence,
            recipientChain,
            recipientAddress,
            refundAddress,
            msg.sender,
            payload,
            transceiverInstructions
        );

        emit MessageSent(
            recipientChain, recipientAddress, sequence, totalPriceQuote, encodedNttManagerPayload
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
