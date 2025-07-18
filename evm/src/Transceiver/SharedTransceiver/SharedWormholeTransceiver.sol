// SPDX-License-Identifier: Apache 2
pragma solidity ^0.8.19;

import "wormhole-solidity-sdk/libraries/BytesParsing.sol";
import "wormhole-solidity-sdk/interfaces/IWormhole.sol";
import "wormhole-solidity-sdk/Utils.sol";

import "../../interfaces/IMsgReceiver.sol";
import "../../interfaces/ISharedWormholeTransceiver.sol";

string constant sharedWormholeTransceiverVersionString = "SharedWormholeTransceiver-0.0.1";

/// @title SharedWormholeTransceiver
///
/// @author Wormhole Project Contributors.
///
/// @notice The SharedWormholeTransceiver is a Wormhole transceiver implementation that can
///         be shared between multiple managers. It implements the ITransceiver interface.
///
///         The SharedWormholeTransceiver assumes the use of the Executor at the manager level,
///         so it has no internal relayer support. It currently requires no transceiver instructions,
///         so the parameter required by the interface is not used.
///
///         Since the ITransceiver interface requires NttManager and token addresses, this transceiver
///         implements those interfaces, but they are not used, and the values returned are zero.
///
///         Since this transceiver is not owned by a specific manager, some of the interface
///         functions are stubbed off (transferring ownership, for instance). Additionally,
///         this transceiver is immutable, so the upgrade function reverts.
///
///         This transceiver has an admin who is responsible for provisioning peers. There are
///         a number of admin functions for provisioning peers and transferring the admin.
///         Additionally, there is a function to discard the admin, making the contract
///         truly immutable.
///
///         This transceiver maintains the Wormhole transceiver wire format, so in theory
///         it should be able to peer with instances of the standard `WormholeTransceiver`.
///
contract SharedWormholeTransceiver is ISharedWormholeTransceiver {
    using BytesParsing for bytes; // Used by _decodePayload

    // ==================== Constants ================================================
    // TODO: These are in `WormholeTranceiverState.sol` but I can't access them for some reason.
    // TODO: Do we need to publish `WH_TRANSCEIVER_INIT_PREFIX` and `WH_PEER_REGISTRATION_PREFIX`?

    /// @dev Prefix for all TransceiverMessage payloads
    /// @notice Magic string (constant value set by messaging provider) that idenfies the payload as an transceiver-emitted payload.
    ///         Note that this is not a security critical field. It's meant to be used by messaging providers to identify which messages are Transceiver-related.
    bytes4 public constant WH_TRANSCEIVER_PAYLOAD_PREFIX = 0x9945FF10;

    /// @dev Prefix for all Wormhole transceiver initialisation payloads
    ///      This is bytes4(keccak256("WormholeTransceiverInit"))
    bytes4 constant WH_TRANSCEIVER_INIT_PREFIX = 0x9c23bd3b;

    /// @dev Prefix for all Wormhole peer registration payloads
    ///      This is bytes4(keccak256("WormholePeerRegistration"))
    bytes4 constant WH_PEER_REGISTRATION_PREFIX = 0x18fc67c2;

    // ==================== Immutables ===============================================

    address public admin;
    address public pendingAdmin;
    uint16 public immutable ourChain;
    IWormhole public immutable wormhole;
    uint8 public immutable consistencyLevel;

    // ==================== Constructor ==============================================

    constructor(uint16 _ourChain, address _admin, address _wormhole, uint8 _consistencyLevel) {
        assert(_ourChain != 0);
        assert(_admin != address(0));
        assert(_wormhole != address(0));
        // Not checking consistency level since maybe zero is valid?
        ourChain = _ourChain;
        admin = _admin;
        wormhole = IWormhole(_wormhole);
        consistencyLevel = _consistencyLevel;
    }

    // =============== Storage Keys =============================================

    bytes32 private constant WORMHOLE_PEERS_SLOT = bytes32(uint256(keccak256("swt.peers")) - 1);
    bytes32 private constant CHAINS_SLOT = bytes32(uint256(keccak256("swt.chains")) - 1);

    // =============== Storage Accessors ========================================

    function _getPeersStorage() internal pure returns (mapping(uint16 => bytes32) storage $) {
        uint256 slot = uint256(WORMHOLE_PEERS_SLOT);
        assembly ("memory-safe") {
            $.slot := slot
        }
    }

    function _getChainsStorage() internal pure returns (uint16[] storage $) {
        uint256 slot = uint256(CHAINS_SLOT);
        assembly ("memory-safe") {
            $.slot := slot
        }
    }

    // =============== Public Getters ======================================================

    /// @inheritdoc ISharedWormholeTransceiver
    function getPeer(
        uint16 chainId
    ) public view returns (bytes32 peerContract) {
        peerContract = _getPeersStorage()[chainId];
        if (peerContract == bytes32(0)) {
            revert UnregisteredPeer(chainId);
        }
    }

    /// @inheritdoc ISharedWormholeTransceiver
    function getPeers() public view returns (PeerEntry[] memory results) {
        uint16[] storage chains = _getChainsStorage();
        uint256 len = chains.length;
        results = new PeerEntry[](len);
        for (uint256 idx = 0; idx < len;) {
            results[idx].chain = chains[idx];
            results[idx].addr = getPeer(chains[idx]);
            unchecked {
                ++idx;
            }
        }
    }

    // =============== Admin ===============================================================

    /// @inheritdoc ISharedWormholeTransceiver
    function updateAdmin(
        address newAdmin
    ) external onlyAdmin {
        // SPEC: MUST check that the caller is the current admin and there is not a pending transfer.
        // - This is handled by onlyAdmin.

        // SPEC: If possible, MUST NOT allow the admin to discard admin via this command (e.g. newAdmin != address(0) on EVM)
        if (newAdmin == address(0)) {
            revert InvalidAdminZeroAddress();
        }

        // SPEC: Immediately sets newAdmin as the admin of the integrator.
        admin = newAdmin;
        emit AdminUpdated(msg.sender, newAdmin);
    }

    /// @inheritdoc ISharedWormholeTransceiver
    function transferAdmin(
        address newAdmin
    ) external onlyAdmin {
        // SPEC: MUST check that the caller is the current admin and there is not a pending transfer.
        // - This is handled by onlyAdmin.

        // SPEC: If possible, MUST NOT allow the admin to discard admin via this command (e.g. `newAdmin != address(0)` on EVM).
        if (newAdmin == address(0)) {
            revert InvalidAdminZeroAddress();
        }

        // SPEC: Initiates the first step of a two-step process in which the current admin (to cancel) or new admin must claim.
        pendingAdmin = newAdmin;
        emit AdminUpdateRequested(msg.sender, newAdmin);
    }

    /// @inheritdoc ISharedWormholeTransceiver
    function claimAdmin() external {
        // This doesn't use onlyAdmin because the pending admin must be non-zero.

        // SPEC: MUST check that the caller is the current admin OR the pending admin.
        if ((admin != msg.sender) && (pendingAdmin != msg.sender)) {
            revert CallerNotAdmin(msg.sender);
        }

        // SPEC: MUST check that there is an admin transfer pending (e. g. pendingAdmin != address(0) on EVM).
        if (pendingAdmin == address(0)) {
            revert NoAdminUpdatePending();
        }

        // SPEC: Cancels / Completes the second step of the two-step transfer. Sets the admin to the caller and clears the pending admin.
        address oldAdmin = admin;
        admin = msg.sender;
        pendingAdmin = address(0);
        emit AdminUpdated(oldAdmin, msg.sender);
    }

    /// @inheritdoc ISharedWormholeTransceiver
    function discardAdmin() external onlyAdmin {
        // SPEC: MUST check that the caller is the current admin and there is not a pending transfer.
        // - This is handled by onlyAdmin.

        // SPEC: Clears the current admin. THIS IS NOT REVERSIBLE. This ensures that the Integrator configuration becomes immutable.
        admin = address(0);
        emit AdminDiscarded(msg.sender);
    }

    /// @inheritdoc ISharedWormholeTransceiver
    function setPeer(uint16 peerChain, bytes32 peerContract) external onlyAdmin {
        if (peerChain == 0 || peerChain == ourChain) {
            revert InvalidChain(peerChain);
        }
        if (peerContract == bytes32(0)) {
            revert InvalidPeerZeroAddress();
        }

        bytes32 oldPeerContract = _getPeersStorage()[peerChain];

        // SPEC: MUST not set the peer if it is already set.
        if (oldPeerContract != bytes32(0)) {
            revert PeerAlreadySet(peerChain, oldPeerContract);
        }

        _getPeersStorage()[peerChain] = peerContract;
        _getChainsStorage().push(peerChain);
        emit PeerAdded(peerChain, peerContract);
    }

    // =============== ITransceiver Interface ==============================================

    /// @inheritdoc ITransceiver
    function getTransceiverType() external pure virtual returns (string memory) {
        return sharedWormholeTransceiverVersionString;
    }

    /// @inheritdoc ITransceiver
    function quoteDeliveryPrice(
        uint16, // recipientChain
        TransceiverStructs.TransceiverInstruction calldata // instruction
    ) external view virtual returns (uint256) {
        return wormhole.messageFee();
    }

    /// @inheritdoc ITransceiver
    /// @dev The caller should set the delivery price in msg.value.
    /// @dev This transceiver does not use instructions, so that parameter is ignored.
    /// @dev This transceiver does not use refundAddress, so that parameter is ignored.
    function sendMessage(
        uint16 recipientChain,
        TransceiverStructs.TransceiverInstruction memory, // instruction,
        bytes memory nttManagerMessage,
        bytes32 recipientNttManagerAddress,
        bytes32 // refundAddress
    ) external payable virtual {
        (
            TransceiverStructs.TransceiverMessage memory transceiverMessage,
            bytes memory encodedTransceiverPayload
        ) = TransceiverStructs.buildAndEncodeTransceiverMessage(
            WH_TRANSCEIVER_PAYLOAD_PREFIX,
            toWormholeFormat(msg.sender),
            recipientNttManagerAddress,
            nttManagerMessage,
            new bytes(0)
        );

        wormhole.publishMessage{value: msg.value}(0, encodedTransceiverPayload, consistencyLevel);
        emit SendTransceiverMessage(recipientChain, transceiverMessage);
    }

    /// @inheritdoc ITransceiver
    /// @dev This transciever does not have a specific NttManager, so this function just reverts.
    function getNttManagerOwner() external pure returns (address) {
        revert NotImplemented();
    }

    /// @inheritdoc ITransceiver
    /// @dev This transciever does not have a specific NttManager or token, so this function just reverts.
    function getNttManagerToken() external pure returns (address) {
        revert NotImplemented();
    }

    /// @inheritdoc ITransceiver
    /// @dev Since shared transceivers are not owned by the manager, this function does nothing.
    ///      We don't want to revert because a manager may have both shared and unshared transceivers.
    ///      It should be able to call this without without worrying about the transceiver type.
    function transferTransceiverOwnership(
        address newOwner
    ) external {}

    /// @inheritdoc ITransceiver
    /// @dev Since shared transceivers are immutable, this just reverts.
    function upgrade(
        address // newImplementation
    ) external pure {
        revert NotUpgradable();
    }

    // =============== ISharedWormholeTransceiver Interface ================================

    /// @inheritdoc ISharedWormholeTransceiver
    function receiveMessage(
        bytes calldata encodedMessage
    ) external {
        // Verify the wormhole message and extract the source chain and payload.
        (uint16 sourceChainId, bytes memory payload) = _verifyMessage(encodedMessage);

        // TODO: There is no check that this message is intended for this chain, and I don't see how to do it!

        // Parse the encoded Transceiver payload and the encapsulated manager message.
        TransceiverStructs.TransceiverMessage memory parsedTransceiverMessage;
        TransceiverStructs.NttManagerMessage memory parsedNttManagerMessage;
        (parsedTransceiverMessage, parsedNttManagerMessage) = TransceiverStructs
            .parseTransceiverAndNttManagerMessage(WH_TRANSCEIVER_PAYLOAD_PREFIX, payload);

        // We use recipientNttManagerAddress to deliver the message so it must be set.
        if (parsedTransceiverMessage.recipientNttManagerAddress == bytes32(0)) {
            revert RecipientManagerAddressIsZero();
        }

        // Forward the message to the specified manager.
        IMsgReceiver(fromWormholeFormat(parsedTransceiverMessage.recipientNttManagerAddress))
            .attestationReceived(
            sourceChainId, parsedTransceiverMessage.sourceNttManagerAddress, parsedNttManagerMessage
        );

        // We don't need to emit an event here because _verifyMessage already did.
    }

    // ============= Internal ===============================================================

    function _verifyMessage(
        bytes memory encodedMessage
    ) internal returns (uint16, bytes memory) {
        // Verify VAA against Wormhole Core Bridge contract.
        (IWormhole.VM memory vm, bool valid, string memory reason) =
            wormhole.parseAndVerifyVM(encodedMessage);
        if (!valid) {
            revert InvalidVaa(reason);
        }

        // Ensure that the message came from the registered peer contract.
        if (getPeer(vm.emitterChainId) != vm.emitterAddress) {
            revert InvalidPeer(vm.emitterChainId, vm.emitterAddress);
        }

        emit ReceivedMessage(vm.hash, vm.emitterChainId, vm.emitterAddress, vm.sequence);
        return (vm.emitterChainId, vm.payload);
    }

    // =============== MODIFIERS ===============================================

    modifier onlyAdmin() {
        if (admin != msg.sender) {
            revert CallerNotAdmin(msg.sender);
        }
        if (pendingAdmin != address(0)) {
            revert AdminTransferPending();
        }
        _;
    }
}
