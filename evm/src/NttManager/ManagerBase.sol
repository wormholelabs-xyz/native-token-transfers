// SPDX-License-Identifier: Apache 2
pragma solidity >=0.8.8 <0.9.0;

import "wormhole-solidity-sdk/Utils.sol";
import "wormhole-solidity-sdk/libraries/BytesParsing.sol";

import "example-gmp-router/evm/src/interfaces/IRouterAdmin.sol";
import "example-gmp-router/evm/src/interfaces/IRouterIntegrator.sol";
import "example-gmp-router/evm/src/Router.sol"; // TODO: Doing this to access TransceiverRegistry publics. Should there be an interface??

import "../libraries/external/OwnableUpgradeable.sol";
import "../libraries/external/ReentrancyGuardUpgradeable.sol";
import "../libraries/TransceiverStructs.sol";
import "../libraries/TransceiverHelpers.sol";
import "../libraries/PausableOwnable.sol";
import "../libraries/Implementation.sol";

import "../interfaces/IManagerBase.sol";

abstract contract ManagerBase is
    IManagerBase,
    PausableOwnable,
    ReentrancyGuardUpgradeable,
    Implementation
{
    // =============== Immutables ============================================================

    /// @dev Address of the token that this NTT Manager is tied to
    address public immutable token;
    /// @dev Contract deployer address
    address immutable deployer;
    /// @dev Mode of the NTT Manager -- this is either LOCKING (Mode = 0) or BURNING (Mode = 1)
    /// In LOCKING mode, tokens are locked/unlocked by the NTT Manager contract when sending/redeeming cross-chain transfers.
    /// In BURNING mode, tokens are burned/minted by the NTT Manager contract when sending/redeeming cross-chain transfers.
    Mode public immutable mode;
    /// @dev Wormhole chain ID that the NTT Manager is deployed on.
    /// This chain ID is formatted Wormhole Chain IDs -- https://docs.wormhole.com/wormhole/reference/constants
    uint16 public immutable chainId;
    /// @dev EVM chain ID that the NTT Manager is deployed on.
    /// This chain ID is formatted based on standardized chain IDs, e.g. Ethereum mainnet is 1, Sepolia is 11155111, etc.
    uint256 immutable evmChainId;

    IRouterIntegrator public immutable router;

    // =============== Setup =================================================================

    constructor(address _router, address _token, Mode _mode, uint16 _chainId) {
        router = IRouterIntegrator(_router);
        token = _token;
        mode = _mode;
        chainId = _chainId;
        evmChainId = block.chainid;
        // save the deployer (check this on initialization)
        deployer = msg.sender;

        // TODO: Doing this here registered the wrong integrator. I think because the proxy stuff made `this` not what we want.
        // Register this contract as an integrator with the router. For now we are assuming this contract is the admin for the router. TODO: Is that okay?
        // router.register(address(this));
    }

    function _migrate() internal virtual override {
        _checkThresholdInvariants();
        // TODO: Check that the router doesn't change.
    }

    // =============== Storage ==============================================================

    bytes32 private constant MESSAGE_SEQUENCE_SLOT =
        bytes32(uint256(keccak256("ntt.messageSequence")) - 1);

    bytes32 private constant THRESHOLD_SLOT = bytes32(uint256(keccak256("ntt.threshold")) - 1);

    // =============== Storage Getters/Setters ==============================================

    function _getThresholdStorage() private pure returns (_Threshold storage $) {
        uint256 slot = uint256(THRESHOLD_SLOT);
        assembly ("memory-safe") {
            $.slot := slot
        }
    }

    // TODO: Do we still need this? The code assigns a sequence number before enqueuing, so I don't think we can just use the router one.
    function _getMessageSequenceStorage() internal pure returns (_Sequence storage $) {
        uint256 slot = uint256(MESSAGE_SEQUENCE_SLOT);
        assembly ("memory-safe") {
            $.slot := slot
        }
    }

    // =============== External Logic =============================================================

    /// @inheritdoc IManagerBase
    function quoteDeliveryPrice(
        uint16 recipientChain
    ) public view returns (uint256) {
        return router.quoteDeliveryPrice(recipientChain); // TODO: Add in executor delivery price.
    }

    // =============== Internal Logic ===========================================================

    function _refundToSender(
        uint256 refundAmount
    ) internal {
        // refund the price quote back to sender
        (bool refundSuccessful,) = payable(msg.sender).call{value: refundAmount}("");

        // check success
        if (!refundSuccessful) {
            revert RefundFailed(refundAmount);
        }
    }

    // =============== Public Getters ========================================================

    /// @inheritdoc IManagerBase
    function getMode() public view returns (uint8) {
        return uint8(mode);
    }

    /// @inheritdoc IManagerBase
    function getThreshold() public view returns (uint8) {
        return _getThresholdStorage().num;
    }

    /// @inheritdoc IManagerBase
    function isMessageApproved(
        uint16 srcChain,
        UniversalAddress srcAddr,
        uint64 sequence,
        UniversalAddress dstAddr,
        bytes32 payloadHash
    ) public view returns (bool) {
        (uint128 enabled, uint128 attested,) =
            router.getMessageStatus(srcChain, srcAddr, sequence, dstAddr, payloadHash);

        return (enabled == attested);
    }

    /// @inheritdoc IManagerBase
    function nextMessageSequence() external view returns (uint64) {
        return _getMessageSequenceStorage().num;
    }

    // TODO: What's the difference between this and isMessageApproved?
    /// @inheritdoc IManagerBase
    function isMessageExecuted(
        uint16 srcChain,
        UniversalAddress srcAddr,
        uint64 sequence,
        UniversalAddress dstAddr,
        bytes32 payloadHash
    ) public view returns (bool) {
        (,, bool executed) =
            router.getMessageStatus(srcChain, srcAddr, sequence, dstAddr, payloadHash);

        return executed;
    }

    /// @inheritdoc IManagerBase
    function transceiverAttestedToMessage(
        uint16 srcChain,
        UniversalAddress srcAddr,
        uint64 sequence,
        UniversalAddress dstAddr,
        bytes32 payloadHash,
        uint8 index
    ) public view returns (bool) {
        (, uint128 attested,) =
            router.getMessageStatus(srcChain, srcAddr, sequence, dstAddr, payloadHash);

        return attested & uint64(1 << index) > 0;
    }

    /// @inheritdoc IManagerBase
    function messageAttestations(
        uint16 srcChain,
        UniversalAddress srcAddr,
        uint64 sequence,
        UniversalAddress dstAddr,
        bytes32 payloadHash
    ) public view returns (uint8 count) {
        (, uint128 attested,) =
            router.getMessageStatus(srcChain, srcAddr, sequence, dstAddr, payloadHash);

        return countSetBits128(attested);
    }

    // =============== Admin ==============================================================

    /// @inheritdoc IManagerBase
    function upgrade(
        address newImplementation
    ) external onlyOwner {
        _upgrade(newImplementation);
    }

    /// @inheritdoc IManagerBase
    function pause() public onlyOwnerOrPauser {
        _pause();
    }

    function unpause() public onlyOwner {
        _unpause();
    }

    /// @notice Transfer ownership of the Manager contract and all Transceiver contracts to a new owner.
    function transferOwnership(
        address newOwner
    ) public override onlyOwner {
        // TODO: Just delete this function and let the Ownable one be called directly?
        super.transferOwnership(newOwner);
    }

    /// @inheritdoc IManagerBase
    function setTransceiver(
        address transceiver
    ) external onlyOwner {
        uint8 index = IRouterAdmin(address(router)).addTransceiver(address(this), transceiver);

        _Threshold storage _threshold = _getThresholdStorage();
        // We do not automatically increase the threshold here.
        // Automatically increasing the threshold can result in a scenario
        // where in-flight messages can't be redeemed.
        // For example: Assume there is 1 Transceiver and the threshold is 1.
        // If we were to add a new Transceiver, the threshold would increase to 2.
        // However, all messages that are either in-flight or that are sent on
        // a source chain that does not yet have 2 Transceivers will only have been
        // sent from a single transceiver, so they would never be able to get
        // redeemed.
        // Instead, we leave it up to the owner to manually update the threshold
        // after some period of time, ideally once all chains have the new Transceiver
        // and transfers that were sent via the old configuration are all complete.
        // However if the threshold is 0 (the initial case) we do increment to 1.
        if (_threshold.num == 0) {
            _threshold.num = 1;
        }

        emit TransceiverAdded(transceiver, index, _threshold.num); // TODO

        _checkThresholdInvariants();
    }

    /// @inheritdoc IManagerBase
    function enableSendTransceiver(uint16 chain, address transceiver) external {
        IRouterAdmin(address(router)).enableSendTransceiver(address(this), chain, transceiver);
    }

    /// @inheritdoc IManagerBase
    function enableRecvTransceiver(uint16 chain, address transceiver) external {
        IRouterAdmin(address(router)).enableRecvTransceiver(address(this), chain, transceiver);
    }

    /// @inheritdoc IManagerBase
    function disableSendTransceiver(uint16 chain, address transceiver) external {
        IRouterAdmin(address(router)).disableSendTransceiver(address(this), chain, transceiver);
    }

    /// @inheritdoc IManagerBase
    function disableRecvTransceiver(uint16 chain, address transceiver) external {
        IRouterAdmin(address(router)).disableRecvTransceiver(address(this), chain, transceiver);
    }

    /// @inheritdoc IManagerBase
    function setThreshold(
        uint8 threshold
    ) external onlyOwner {
        if (threshold == 0) {
            revert ZeroThreshold();
        }

        _Threshold storage _threshold = _getThresholdStorage();
        uint8 oldThreshold = _threshold.num;

        _threshold.num = threshold;
        _checkThresholdInvariants();

        emit ThresholdChanged(oldThreshold, threshold);
    }

    // =============== Internal ==============================================================

    function _useMessageSequence() internal returns (uint64 currentSequence) {
        currentSequence = _getMessageSequenceStorage().num;
        _getMessageSequenceStorage().num++;
    }

    /// ============== Invariants =============================================

    /// @dev When we add new immutables, this function should be updated
    function _checkImmutables() internal view virtual override {
        assert(this.token() == token);
        assert(this.mode() == mode);
        assert(this.chainId() == chainId);
    }

    function _checkThresholdInvariants() internal view {
        // TODO: Need to have per-chain thresholds and make sure the threshold is not greater than the enabled on any chain.
        // Question: Should TransceiverRegistry be able to return the list of chains enabled for receiving for an integrator?

        uint8 threshold = _getThresholdStorage().num;

        // TODO: This is not right since some of these may be disabled. Really need to do per-chain transceivers anyway.
        address[] memory enabledTransceivers =
            Router(address(router)).getTransceivers(address(this));

        // invariant: threshold <= enabledTransceivers.length
        if (threshold > enabledTransceivers.length) {
            revert ThresholdTooHigh(threshold, enabledTransceivers.length);
        }

        if (enabledTransceivers.length > 0) {
            if (threshold == 0) {
                revert ZeroThreshold();
            }
        }
    }
}
