// SPDX-License-Identifier: Apache 2
pragma solidity >=0.8.8 <0.9.0;

import "../libraries/TransceiverHelpers.sol";

/// @dev TransceiverRegistryBase is a base class shared between TransceiverRegistry and TransceiverRegistryAdmin.
///      It defines all of the state for the transceiver registry. This facilitates using delegate calls to implement
///      the admin functionality in a separate contract (TransceiverRegistryAdmin).
abstract contract TransceiverRegistryBase {
    /// @dev Information about registered transceivers.
    struct TransceiverInfo {
        // whether this transceiver is registered
        bool registered;
        // whether this transceiver is enabled
        bool enabled;
        uint8 index;
    }

    /// @dev Bitmap encoding the enabled transceivers.
    /// invariant: forall (i: uint8), enabledTransceiverBitmap & i == 1 <=> transceiverInfos[i].enabled
    struct _EnabledTransceiverBitmap {
        uint64 bitmap;
    }

    /// @dev Total number of registered transceivers. This number can only increase.
    /// invariant: numRegisteredTransceivers <= MAX_TRANSCEIVERS
    /// invariant: forall (i: uint8),
    ///   i < numRegisteredTransceivers <=> exists (a: address), transceiverInfos[a].index == i
    struct _NumTransceivers {
        uint8 registered;
        uint8 enabled;
    }

    uint8 public constant MAX_TRANSCEIVERS = 64;

    bytes32 internal constant TRANSCEIVER_INFOS_SLOT =
        bytes32(uint256(keccak256("ntt.transceiverInfos")) - 1);

    bytes32 internal constant TRANSCEIVER_BITMAP_SLOT =
        bytes32(uint256(keccak256("ntt.transceiverBitmap")) - 1);

    bytes32 internal constant ENABLED_TRANSCEIVERS_SLOT =
        bytes32(uint256(keccak256("ntt.enabledTransceivers")) - 1);

    bytes32 internal constant REGISTERED_TRANSCEIVERS_SLOT =
        bytes32(uint256(keccak256("ntt.registeredTransceivers")) - 1);

    bytes32 internal constant NUM_REGISTERED_TRANSCEIVERS_SLOT =
        bytes32(uint256(keccak256("ntt.numRegisteredTransceivers")) - 1);

    // =============== Storage slot accessor functions ========================================

    function _getTransceiverInfosStorage()
        internal
        pure
        returns (mapping(address => TransceiverInfo) storage $)
    {
        uint256 slot = uint256(TRANSCEIVER_INFOS_SLOT);
        assembly ("memory-safe") {
            $.slot := slot
        }
    }

    function _getTransceiverBitmapStorage()
        internal
        pure
        returns (_EnabledTransceiverBitmap storage $)
    {
        uint256 slot = uint256(TRANSCEIVER_BITMAP_SLOT);
        assembly ("memory-safe") {
            $.slot := slot
        }
    }

    function _getRegisteredTransceiversStorage() internal pure returns (address[] storage $) {
        uint256 slot = uint256(REGISTERED_TRANSCEIVERS_SLOT);
        assembly ("memory-safe") {
            $.slot := slot
        }
    }

    function _getNumTransceiversStorage() internal pure returns (_NumTransceivers storage $) {
        uint256 slot = uint256(NUM_REGISTERED_TRANSCEIVERS_SLOT);
        assembly ("memory-safe") {
            $.slot := slot
        }
    }

    function _getEnabledTransceiversStorage() internal pure returns (address[] storage $) {
        uint256 slot = uint256(ENABLED_TRANSCEIVERS_SLOT);
        assembly ("memory-safe") {
            $.slot := slot
        }
    }

    //
    // =============== Per-chain transceiver storage ===========================================
    //
    struct _PerChainTransceiverData {
        uint64 bitmap;
        /// @dev By putting this here, we can implement the admin code in TransceiverRegistryBase rather than ManagerBase.
        uint8 threshold;
    }

    // =============== Storage slot for per-chain transceivers, send side ======================

    /// @dev Holds Chain ID => Enabled send side transceiver address[] mapping.
    ///      mapping(uint16 => address[]).
    bytes32 internal constant ENABLED_SEND_TRANSCEIVER_ARRAY_SLOT =
        bytes32(uint256(keccak256("registry.sendTransceiverArray")) - 1);

    // =============== Storage slot for per-chain transceivers, receive side ==================

    /// @dev Holds Chain ID => Enabled transceiver receive side bitmap mapping.
    ///      mapping(uint16 => uint64).
    bytes32 internal constant ENABLED_RECV_TRANSCEIVER_BITMAP_SLOT =
        bytes32(uint256(keccak256("registry.recvTransceiverBitmap")) - 1);

    // =============== Storage slot for tracking enabled chains ===============================

    /// @dev Holds mapping of array of chains with transceivers enabled for sending.
    bytes32 internal constant SEND_ENABLED_CHAINS_SLOT =
        bytes32(uint256(keccak256("registry.sendEnabledChains")) - 1);

    /// @dev Holds mapping of array of chains with transceivers enabled for receiving.
    bytes32 internal constant RECV_ENABLED_CHAINS_SLOT =
        bytes32(uint256(keccak256("registry.recvEnabledChains")) - 1);

    /// @dev Chain ID => Enabled transceiver bitmap mapping.
    function _getPerChainSendTransceiverArrayStorage()
        internal
        pure
        returns (mapping(uint16 => address[]) storage $)
    {
        uint256 slot = uint256(ENABLED_SEND_TRANSCEIVER_ARRAY_SLOT);
        assembly ("memory-safe") {
            $.slot := slot
        }
    }

    /// @dev Chain ID => Enabled transceiver bitmap mapping.
    function _getPerChainRecvTransceiverDataStorage()
        internal
        pure
        returns (mapping(uint16 => _PerChainTransceiverData) storage $)
    {
        uint256 slot = uint256(ENABLED_RECV_TRANSCEIVER_BITMAP_SLOT);
        assembly ("memory-safe") {
            $.slot := slot
        }
    }

    /// @dev Contains all chains that have transceivers enabled.
    function _getChainsEnabledStorage(
        bytes32 tag
    ) internal pure returns (uint16[] storage $) {
        uint256 slot = uint256(tag);
        assembly ("memory-safe") {
            $.slot := slot
        }
    }
}

/// @title TransceiverRegistry
/// @author Wormhole Project Contributors.
/// @notice This contract is responsible for handling the registration of Transceivers.
/// @dev This contract checks that a few critical invariants hold when transceivers are added or removed,
///      including:
///         1. If a transceiver is not registered, it should be enabled.
///         2. The value set in the bitmap of trannsceivers
///            should directly correspond to the whether the transceiver is enabled
abstract contract TransceiverRegistry is TransceiverRegistryBase {
    // TODO: Tests fail if I remove the immutable. Could pass this into the constructor and make `TransceiverRegistryAdmin` upgradeable.
    address private immutable _admin;

    constructor() {
        _admin = address(new TransceiverRegistryAdmin());
        _checkTransceiversInvariants();
    }

    /// @notice Error when the caller is not the transceiver.
    /// @dev Selector 0xa0ae911d.
    /// @param caller The address of the caller.
    error CallerNotTransceiver(address caller);

    /// @notice Error when the transceiver is the zero address.
    /// @dev Selector 0x2f44bd77.
    error InvalidTransceiverZeroAddress();

    /// @notice Error when the transceiver is disabled.
    /// @dev Selector 0x1f61ba44.
    error DisabledTransceiver(address transceiver);

    /// @notice Error when the number of registered transceivers
    ///         exceeeds (MAX_TRANSCEIVERS = 64).
    /// @dev Selector 0x891684c3.
    error TooManyTransceivers();

    /// @notice Error when attempting to remove a transceiver
    ///         that is not registered.
    /// @dev Selector 0xd583f470.
    /// @param transceiver The address of the transceiver.
    error NonRegisteredTransceiver(address transceiver);

    /// @notice Error when attempting to use an incorrect chain.
    /// @dev Selector: 0x587c94c3.
    /// @param chain The id of the incorrect chain.
    error InvalidChain(uint16 chain);

    /// @notice Error when attempting to enable a transceiver that is already enabled.
    /// @dev Selector: 0x8d68f84d.
    /// @param transceiver The address of the transceiver.
    error TransceiverAlreadyEnabled(address transceiver);

    /// @notice Error when the transceiver is disabled.
    /// @dev Selector: 0xa64030ff.
    error TransceiverAlreadyDisabled(address transceiver);

    /// @notice Attempting to remove a transceiver when it is still enabled for receiving on at least one chain.
    /// @dev Selector: 0x7481293a.
    /// @param chain The first chain on which the transceiver is still registered.
    /// @param bitmap The bitmap of enabled transceivers for that chain.
    error TransceiverStillEnabledForRecv(uint16 chain, uint64 bitmap);

    /// @notice Attempting to remove a transceiver when it is still enabled for sending on at least one chain.
    /// @dev Selector: 0x2bb41527.
    /// @param chain The first chain on which the transceiver is still registered.
    error TransceiverStillEnabledForSend(uint16 chain);

    /// @notice The threshold for transceiver attestations is too high.
    /// @param chainId The chain with the invalid threshold.
    /// @param threshold The threshold.
    /// @param transceivers The number of transceivers.
    error ThresholdTooHigh(uint16 chainId, uint256 threshold, uint256 transceivers);

    /// @notice The number of thresholds should not be zero.
    /// @param chainId The chain with the invalid threshold.
    error ZeroThreshold(uint16 chainId);

    modifier onlyTransceiver() {
        if (!_getTransceiverInfosStorage()[msg.sender].enabled) {
            revert CallerNotTransceiver(msg.sender);
        }
        _;
    }

    // =============== Storage Getters/Setters ========================================

    function _setTransceiver(
        address transceiver
    ) internal returns (uint8 index) {
        (bool success, bytes memory returnData) = _admin.delegatecall(
            abi.encodeWithSelector(TransceiverRegistryAdmin._setTransceiver.selector, transceiver)
        );
        _checkDelegateCallRevert(success, returnData);
        (index) = abi.decode(returnData, (uint8));
    }

    function _removeTransceiver(
        address transceiver
    ) internal {
        (bool success, bytes memory returnData) = _admin.delegatecall(
            abi.encodeWithSelector(
                TransceiverRegistryAdmin._removeTransceiver.selector, transceiver
            )
        );
        _checkDelegateCallRevert(success, returnData);
    }

    function enableSendTransceiverForChain(uint16 chain, address transceiver) public {
        (bool success, bytes memory returnData) = _admin.delegatecall(
            abi.encodeWithSelector(
                TransceiverRegistryAdmin._enableSendTransceiverForChain.selector, chain, transceiver
            )
        );
        _checkDelegateCallRevert(success, returnData);
    }

    function disableSendTransceiverForChain(uint16 chain, address transceiver) public {
        (bool success, bytes memory returnData) = _admin.delegatecall(
            abi.encodeWithSelector(
                TransceiverRegistryAdmin._disableSendTransceiverForChain.selector,
                chain,
                transceiver
            )
        );
        _checkDelegateCallRevert(success, returnData);
    }

    function enableRecvTransceiverForChain(uint16 chain, address transceiver) public {
        (bool success, bytes memory returnData) = _admin.delegatecall(
            abi.encodeWithSelector(
                TransceiverRegistryAdmin._enableRecvTransceiverForChain.selector, chain, transceiver
            )
        );
        _checkDelegateCallRevert(success, returnData);
    }

    function disableRecvTransceiverForChain(uint16 chain, address transceiver) public {
        (bool success, bytes memory returnData) = _admin.delegatecall(
            abi.encodeWithSelector(
                TransceiverRegistryAdmin._disableRecvTransceiverForChain.selector,
                chain,
                transceiver
            )
        );
        _checkDelegateCallRevert(success, returnData);
    }

    function _getEnabledTransceiversBitmap() internal view virtual returns (uint64 bitmap) {
        return _getTransceiverBitmapStorage().bitmap;
    }

    /// @notice Returns the Transceiver contracts that have been enabled via governance.
    function getTransceivers() external pure returns (address[] memory result) {
        result = _getEnabledTransceiversStorage();
    }

    /// @notice Returns the info for all enabled transceivers
    /// @dev moving this into `TransceiverRegistryAdmin` reduces the size of `NttManger` but increases the size of `NttManagerNoRateLimiting`.
    function getTransceiverInfo() external view returns (TransceiverInfo[] memory) {
        address[] memory enabledTransceivers = _getEnabledTransceiversStorage();
        uint256 numEnabledTransceivers = enabledTransceivers.length;
        TransceiverInfo[] memory result = new TransceiverInfo[](numEnabledTransceivers);

        for (uint256 i = 0; i < numEnabledTransceivers; ++i) {
            result[i] = _getTransceiverInfosStorage()[enabledTransceivers[i]];
        }

        return result;
    }

    /// @notice Returns the enabled send side transceiver addresses for the given chain.
    /// @param chain The Wormhole chain ID for the desired transceivers.
    /// @return result The enabled send side transceivers for the given chain.
    function getEnabledSendTransceiversForChain(
        uint16 chain
    ) public view returns (address[] memory result) {
        if (chain == 0) {
            revert InvalidChain(chain);
        }
        result = _getPerChainSendTransceiverArrayStorage()[chain];
    }

    /// @notice Returns the enabled receive side transceiver bitmap for the given chain.
    /// @param chain The Wormhole chain ID for the desired transceivers.
    /// @return result The enabled receive side transceiver bitmap for the given chain.
    function getEnabledRecvTransceiversBitmapForChain(
        uint16 chain
    ) public view returns (uint64 result) {
        if (chain == 0) {
            revert InvalidChain(chain);
        }
        result = _getPerChainRecvTransceiverDataStorage()[chain].bitmap;
    }

    /// @notice Returns whether or not the receive side transceiver is enabled for the given chain.
    /// @dev This function is private and should only be called by a function that checks the validity of chain and transceiver.
    /// @param chain The Wormhole chain ID.
    /// @param index The index of the transceiver.
    /// @return true if the transceiver is enabled, false otherwise.
    function _isRecvTransceiverEnabledForChain(
        uint16 chain,
        uint8 index
    ) internal view returns (bool) {
        uint64 bitmap = _getPerChainRecvTransceiverDataStorage()[chain].bitmap;
        return (bitmap & uint64(1 << index)) > 0;
    }

    /// @notice Sets the receive threshold for the specified chain.
    /// @param chain The Wormhole chain ID.
    /// @param threshold The updated threshold value.
    function _setThreshold(uint16 chain, uint8 threshold) internal {
        (bool success, bytes memory returnData) = _admin.delegatecall(
            abi.encodeWithSelector(
                TransceiverRegistryAdmin._setThreshold.selector, chain, threshold
            )
        );
        _checkDelegateCallRevert(success, returnData);
    }

    /// @notice Returns the set of chains for which sending is enabled.
    function getChainsEnabledForSending() external pure returns (uint16[] memory) {
        return _getChainsEnabledStorage(SEND_ENABLED_CHAINS_SLOT);
    }

    /// @notice Returns the set of chains for which receiving is enabled.
    function getChainsEnabledForReceiving() external pure returns (uint16[] memory) {
        return _getChainsEnabledStorage(RECV_ENABLED_CHAINS_SLOT);
    }

    // ============== Invariants =============================================

    /// @dev Check that the transceiver nttManager is in a valid state.
    /// Checking these invariants is somewhat costly, but we only need to do it
    /// when modifying the transceivers, which happens infrequently.
    function _checkTransceiversInvariants() internal view {
        (bool success, bytes memory returnData) = _admin.staticcall(
            abi.encodeWithSelector(TransceiverRegistryAdmin._checkTransceiversInvariants.selector)
        );
        _checkDelegateCallRevert(success, returnData);
    }

    function _checkDelegateCallRevert(bool success, bytes memory returnData) private pure {
        // if the function call reverted
        if (success == false) {
            // if there is a return reason string
            if (returnData.length > 0) {
                // bubble up any reason for revert
                assembly {
                    let returndata_size := mload(returnData)
                    revert(add(32, returnData), returndata_size)
                }
            } else {
                revert("delegate call reverted");
            }
        }
    }
}

/// @dev TransceiverRegistryAdmin is a helper contract to TransceiverRegistry.
///      It implements admin functionality and is called via `delegatecall`.
contract TransceiverRegistryAdmin is TransceiverRegistryBase {
    /// @notice Emitted when a send side transceiver is enabled for a chain.
    /// @dev Topic0
    ///      0x86c081420b3eb6721acf690f71cab5dea27b08f0be33f4319cdbb4a5733e7ac6.
    /// @param chain The Wormhole chain ID on which this transceiver is enabled.
    /// @param transceiver The address of the transceiver.
    event SendTransceiverEnabledForChain(uint16 chain, address transceiver);

    /// @notice Emitted when a receive side transceiver is enabled for a chain.
    /// @dev Topic0
    ///      0x6ceee5880439d670aa17a1428ce3f83fb3da492eb152aecef53eca06f0388bda.
    /// @param chain The Wormhole chain ID on which this transceiver is enabled.
    /// @param transceiver The address of the transceiver.
    event RecvTransceiverEnabledForChain(uint16 chain, address transceiver, uint8 threshold);

    /// @notice Emitted when a send side transceiver is disabled.
    /// @dev Topic0
    ///      0x6ceee5880439d670aa17a1428ce3f83fb3da492eb152aecef53eca06f0388bda.
    /// @param chain The Wormhole chain ID on which this transceiver is disabled.
    /// @param transceiver The address of the transceiver.
    event SendTransceiverDisabledForChain(uint16 chain, address transceiver);

    /// @notice Emitted when a receive side transceiver is disabled.
    /// @dev Topic0
    ///      0xdcad454c5c2805c34d9de195bc0f494aa9b7a73a7e1a3896d40004094dd9a499.
    /// @param chain The Wormhole chain ID on which this transceiver is disabled.
    /// @param transceiver The address of the transceiver.
    event RecvTransceiverDisabledForChain(uint16 chain, address transceiver);

    /// @notice Emmitted when the threshold required transceivers is changed.
    /// @dev Topic0
    ///      0x2a855b929b9a53c6fb5b5ed248b27e502b709c088e036a5aa17620c8fc5085a9.
    /// @param oldThreshold The old threshold.
    /// @param threshold The new threshold.
    event ThresholdChanged(uint8 oldThreshold, uint8 threshold);

    function _setTransceiver(
        address transceiver
    ) public returns (uint8 index) {
        mapping(address => TransceiverInfo) storage transceiverInfos = _getTransceiverInfosStorage();
        _EnabledTransceiverBitmap storage _enabledTransceiverBitmap = _getTransceiverBitmapStorage();
        address[] storage _enabledTransceivers = _getEnabledTransceiversStorage();

        _NumTransceivers storage _numTransceivers = _getNumTransceiversStorage();

        if (transceiver == address(0)) {
            revert TransceiverRegistry.InvalidTransceiverZeroAddress();
        }

        if (transceiverInfos[transceiver].registered) {
            transceiverInfos[transceiver].enabled = true;
        } else {
            if (_numTransceivers.registered >= MAX_TRANSCEIVERS) {
                revert TransceiverRegistry.TooManyTransceivers();
            }

            transceiverInfos[transceiver] = TransceiverInfo({
                registered: true,
                enabled: true,
                index: _numTransceivers.registered
            });
            _numTransceivers.registered++;
            _getRegisteredTransceiversStorage().push(transceiver);
        }

        _enabledTransceivers.push(transceiver);
        _numTransceivers.enabled++;

        uint64 updatedEnabledTransceiverBitmap =
            _enabledTransceiverBitmap.bitmap | uint64(1 << transceiverInfos[transceiver].index);
        // ensure that this actually changed the bitmap
        if (updatedEnabledTransceiverBitmap == _enabledTransceiverBitmap.bitmap) {
            revert TransceiverRegistry.TransceiverAlreadyEnabled(transceiver);
        }
        _enabledTransceiverBitmap.bitmap = updatedEnabledTransceiverBitmap;

        _checkTransceiversInvariants();

        return transceiverInfos[transceiver].index;
    }

    function _removeTransceiver(
        address transceiver
    ) public {
        mapping(address => TransceiverInfo) storage transceiverInfos = _getTransceiverInfosStorage();
        _EnabledTransceiverBitmap storage _enabledTransceiverBitmap = _getTransceiverBitmapStorage();
        address[] storage _enabledTransceivers = _getEnabledTransceiversStorage();

        if (transceiver == address(0)) {
            revert TransceiverRegistry.InvalidTransceiverZeroAddress();
        }

        if (!transceiverInfos[transceiver].registered) {
            revert TransceiverRegistry.NonRegisteredTransceiver(transceiver);
        }

        if (!transceiverInfos[transceiver].enabled) {
            revert TransceiverRegistry.DisabledTransceiver(transceiver);
        }

        // Reverts if the receiver is enabled for sending or receiving on any chain.
        _checkTransceiverNotEnabled(transceiver, transceiverInfos[transceiver].index);

        transceiverInfos[transceiver].enabled = false;
        _getNumTransceiversStorage().enabled--;

        uint64 updatedEnabledTransceiverBitmap =
            _enabledTransceiverBitmap.bitmap & uint64(~(1 << transceiverInfos[transceiver].index));
        // ensure that this actually changed the bitmap
        assert(updatedEnabledTransceiverBitmap < _enabledTransceiverBitmap.bitmap);
        _enabledTransceiverBitmap.bitmap = updatedEnabledTransceiverBitmap;

        bool removed = false;

        uint256 numEnabledTransceivers = _enabledTransceivers.length;
        for (uint256 i = 0; i < numEnabledTransceivers; i++) {
            if (_enabledTransceivers[i] == transceiver) {
                _enabledTransceivers[i] = _enabledTransceivers[numEnabledTransceivers - 1];
                _enabledTransceivers.pop();
                removed = true;
                break;
            }
        }
        assert(removed);

        _checkTransceiversInvariants();
        // we call the invariant check on the transceiver here as well, since
        // the above check only iterates through the enabled transceivers.
        _checkTransceiverInvariants(transceiver);
    }

    /// @dev Reverts if the transceiver is enabled on any chain.
    /// @param transceiver The transceiver being removed.
    /// @param index The index of the transceiver.
    function _checkTransceiverNotEnabled(address transceiver, uint8 index) private view {
        // Check the send side.
        uint16[] storage chains = _getChainsEnabledStorage(SEND_ENABLED_CHAINS_SLOT);
        uint256 numChains = chains.length;
        for (uint256 chainIdx = 0; (chainIdx < numChains);) {
            address[] storage transceivers =
                _getPerChainSendTransceiverArrayStorage()[chains[chainIdx]];
            uint256 numTransceivers = transceivers.length;
            for (uint256 transceiverIdx = 0; (transceiverIdx < numTransceivers);) {
                if (transceivers[transceiverIdx] == transceiver) {
                    revert TransceiverRegistry.TransceiverStillEnabledForSend(chains[chainIdx]);
                }
                unchecked {
                    ++transceiverIdx;
                }
            }

            unchecked {
                ++chainIdx;
            }
        }

        // Check the receive side.
        chains = _getChainsEnabledStorage(RECV_ENABLED_CHAINS_SLOT);
        numChains = chains.length;
        for (uint256 idx = 0; (idx < numChains);) {
            uint64 bitmap = _getPerChainRecvTransceiverDataStorage()[chains[idx]].bitmap;
            if (bitmap & uint64(1 << index) != 0) {
                revert TransceiverRegistry.TransceiverStillEnabledForRecv(chains[idx], bitmap);
            }
            unchecked {
                ++idx;
            }
        }
    }

    /// @dev This just enables the send side transceiver for a chain. It does not register it.
    /// @param chain The Wormhole chain ID.
    /// @param transceiver The transceiver address.
    function _enableSendTransceiverForChain(
        uint16 chain,
        address transceiver
    ) public onlyRegisteredTransceiver(chain, transceiver) {
        if (_isSendTransceiverEnabledForChain(chain, transceiver)) {
            revert TransceiverRegistry.TransceiverAlreadyEnabled(transceiver);
        }
        address[] storage sendTransceiverArray = _getPerChainSendTransceiverArrayStorage()[chain];
        if (sendTransceiverArray.length == 0) {
            _addEnabledChain(SEND_ENABLED_CHAINS_SLOT, chain);
        }
        sendTransceiverArray.push(transceiver);
        emit SendTransceiverEnabledForChain(chain, transceiver);
    }

    /// @notice Disables a send side transceiver for a chain.
    /// @param chain The chain ID.
    /// @param transceiver The transceiver address.
    function _disableSendTransceiverForChain(
        uint16 chain,
        address transceiver
    ) public onlyRegisteredTransceiver(chain, transceiver) {
        mapping(uint16 => address[]) storage enabledSendTransceivers =
            _getPerChainSendTransceiverArrayStorage();
        address[] storage transceivers = enabledSendTransceivers[chain];

        // Get the index of the disabled transceiver in the enabled transceivers array
        // and replace it with the last element in the array.
        uint256 len = transceivers.length;
        bool found = false;
        for (uint256 i = 0; i < len;) {
            if (transceivers[i] == transceiver) {
                // Swap the last element with the element to be removed
                transceivers[i] = transceivers[len - 1];
                // Remove the last element
                transceivers.pop();
                found = true;
                if (transceivers.length == 0) {
                    _removeEnabledChain(SEND_ENABLED_CHAINS_SLOT, chain);
                }
                break;
            }
            unchecked {
                ++i;
            }
        }
        if (!found) {
            revert TransceiverRegistry.TransceiverAlreadyDisabled(transceiver);
        }

        emit SendTransceiverDisabledForChain(chain, transceiver);
    }

    /// @dev This just enables the receive side transceiver for a chain. It does not register it.
    /// @param chain The Wormhole chain ID.
    /// @param transceiver The transceiver address.
    function _enableRecvTransceiverForChain(
        uint16 chain,
        address transceiver
    ) public onlyRegisteredTransceiver(chain, transceiver) {
        if (_isRecvTransceiverEnabledForChain(chain, transceiver)) {
            revert TransceiverRegistry.TransceiverAlreadyEnabled(transceiver);
        }
        uint8 index = _getTransceiverInfosStorage()[transceiver].index;
        _PerChainTransceiverData storage _bitmapEntry =
            _getPerChainRecvTransceiverDataStorage()[chain];
        if (_bitmapEntry.bitmap == 0) {
            _addEnabledChain(RECV_ENABLED_CHAINS_SLOT, chain);
            _bitmapEntry.threshold = 1;
        }
        _bitmapEntry.bitmap |= uint64(1 << index);
        emit RecvTransceiverEnabledForChain(chain, transceiver, _bitmapEntry.threshold);
    }

    /// @notice Disables a receive side transceiver for a chain.
    /// @dev Will revert under the following conditions:
    ///         - The transceiver is the zero address.
    ///         - The transceiver is not registered.
    /// @param chain The Wormhole chain ID.
    /// @param transceiver The transceiver address.
    function _disableRecvTransceiverForChain(
        uint16 chain,
        address transceiver
    ) public onlyRegisteredTransceiver(chain, transceiver) {
        mapping(address => TransceiverInfo) storage transceiverInfos = _getTransceiverInfosStorage();
        _PerChainTransceiverData storage _data = _getPerChainRecvTransceiverDataStorage()[chain];

        uint64 updatedEnabledTransceiverBitmap =
            _data.bitmap & uint64(~(1 << transceiverInfos[transceiver].index));
        // ensure that this actually changed the bitmap
        if (updatedEnabledTransceiverBitmap >= _data.bitmap) {
            revert TransceiverRegistry.TransceiverAlreadyDisabled(transceiver);
        }
        _data.bitmap = updatedEnabledTransceiverBitmap;
        if (_data.bitmap == 0) {
            _removeEnabledChain(RECV_ENABLED_CHAINS_SLOT, chain);
        }

        emit RecvTransceiverDisabledForChain(chain, transceiver);

        uint8 numEnabled = countSetBits(_data.bitmap);
        if (numEnabled < _data.threshold) {
            emit ThresholdChanged(_data.threshold, numEnabled);
            _data.threshold = numEnabled;
        }
    }

    /// @notice Returns whether or not the send side transceiver is enabled for the given chain.
    /// @dev This function is private and should only be called by a function that checks the validity of chain and transceiver.
    /// @param chain The Wormhole chain ID.
    /// @param transceiver The transceiver address.
    /// @return true if the transceiver is enabled, false otherwise.
    function _isSendTransceiverEnabledForChain(
        uint16 chain,
        address transceiver
    ) private view returns (bool) {
        address[] storage transceivers = _getPerChainSendTransceiverArrayStorage()[chain];
        uint256 length = transceivers.length;
        for (uint256 i = 0; i < length;) {
            if (transceivers[i] == transceiver) {
                return true;
            }
            unchecked {
                ++i;
            }
        }
        return false;
    }

    /// @notice Returns whether or not the receive side transceiver is enabled for the given chain.
    /// @dev This function is private and should only be called by a function that checks the validity of chain and transceiver.
    /// @param chain The Wormhole chain ID.
    /// @param transceiver The transceiver address.
    /// @return true if the transceiver is enabled, false otherwise.
    function _isRecvTransceiverEnabledForChain(
        uint16 chain,
        address transceiver
    ) private view returns (bool) {
        uint64 bitmap = _getPerChainRecvTransceiverDataStorage()[chain].bitmap;
        uint8 index = _getTransceiverInfosStorage()[transceiver].index;
        return (bitmap & uint64(1 << index)) > 0;
    }

    /// @dev It is assumed that the chain is not already in the list. We can get away with this because the function is internal.
    /// @dev Although this is a one line function, we have it for two reasons: (1) symmetry with remove, (2) simplifies testing.
    ///      The assumption is that the compiler will inline it anyway.
    function _addEnabledChain(bytes32 tag, uint16 chain) internal {
        _getChainsEnabledStorage(tag).push(chain);
    }

    /// @dev It's not an error if the chain is not in the list.
    function _removeEnabledChain(bytes32 tag, uint16 chain) internal {
        uint16[] storage chains = _getChainsEnabledStorage(tag);
        uint256 len = chains.length;
        for (uint256 idx = 0; (idx < len);) {
            if (chains[idx] == chain) {
                chains[idx] = chains[len - 1];
                chains.pop();
                return;
            }
            unchecked {
                ++idx;
            }
        }
    }

    /// @notice Sets the receive threshold for the specified chain.
    /// @param chain The Wormhole chain ID.
    /// @param threshold The updated threshold value.
    function _setThreshold(uint16 chain, uint8 threshold) public {
        if (threshold == 0) {
            revert TransceiverRegistry.ZeroThreshold(chain);
        }

        _PerChainTransceiverData storage _data = _getPerChainRecvTransceiverDataStorage()[chain];
        uint8 oldThreshold = _data.threshold;
        _data.threshold = threshold;
        _checkThresholdInvariant(chain);
        emit ThresholdChanged(oldThreshold, threshold);
    }

    // =============== Modifiers ======================================================

    /// @notice This modifier will revert if the transceiver is an invalid address, not registered, or the chain is invalid.
    /// @param chain The Wormhole chain ID.
    /// @param transceiver The transceiver address.
    modifier onlyRegisteredTransceiver(uint16 chain, address transceiver) {
        if (transceiver == address(0)) {
            revert TransceiverRegistry.InvalidTransceiverZeroAddress();
        }

        if (chain == 0) {
            revert TransceiverRegistry.InvalidChain(chain);
        }

        if (!_getTransceiverInfosStorage()[transceiver].registered) {
            revert TransceiverRegistry.NonRegisteredTransceiver(transceiver);
        }
        _;
    }

    /// @dev Check that the transceiver nttManager is in a valid state.
    /// Checking these invariants is somewhat costly, but we only need to do it
    /// when modifying the transceivers, which happens infrequently.
    function _checkTransceiversInvariants() public view {
        _NumTransceivers storage _numTransceivers = _getNumTransceiversStorage();
        address[] storage _enabledTransceivers = _getEnabledTransceiversStorage();

        uint256 numTransceiversEnabled = _numTransceivers.enabled;
        assert(numTransceiversEnabled == _enabledTransceivers.length);

        for (uint256 i = 0; i < numTransceiversEnabled; i++) {
            _checkTransceiverInvariants(_enabledTransceivers[i]);
        }

        // invariant: each transceiver is only enabled once
        for (uint256 i = 0; i < numTransceiversEnabled; i++) {
            for (uint256 j = i + 1; j < numTransceiversEnabled; j++) {
                assert(_enabledTransceivers[i] != _enabledTransceivers[j]);
            }
        }

        // invariant: numRegisteredTransceivers <= MAX_TRANSCEIVERS
        assert(_numTransceivers.registered <= MAX_TRANSCEIVERS);

        _checkPerChainTransceiversInvariants();
    }

    // @dev Check that the transceiver is in a valid state.
    function _checkTransceiverInvariants(
        address transceiver
    ) public view {
        mapping(address => TransceiverInfo) storage transceiverInfos = _getTransceiverInfosStorage();
        _EnabledTransceiverBitmap storage _enabledTransceiverBitmap = _getTransceiverBitmapStorage();
        _NumTransceivers storage _numTransceivers = _getNumTransceiversStorage();
        address[] storage _enabledTransceivers = _getEnabledTransceiversStorage();

        TransceiverInfo memory transceiverInfo = transceiverInfos[transceiver];

        // if a transceiver is not registered, it should not be enabled
        assert(
            transceiverInfo.registered || (!transceiverInfo.enabled && transceiverInfo.index == 0)
        );

        bool transceiverInEnabledBitmap =
            (_enabledTransceiverBitmap.bitmap & uint64(1 << transceiverInfo.index)) != 0;
        bool transceiverEnabled = transceiverInfo.enabled;

        bool transceiverInEnabledTransceivers = false;

        for (uint256 i = 0; i < _numTransceivers.enabled; i++) {
            if (_enabledTransceivers[i] == transceiver) {
                transceiverInEnabledTransceivers = true;
                break;
            }
        }

        // invariant: transceiverInfos[transceiver].enabled
        //            <=> enabledTransceiverBitmap & (1 << transceiverInfos[transceiver].index) != 0
        assert(transceiverInEnabledBitmap == transceiverEnabled);

        // invariant: transceiverInfos[transceiver].enabled <=> transceiver in _enabledTransceivers
        assert(transceiverInEnabledTransceivers == transceiverEnabled);

        assert(transceiverInfo.index < _numTransceivers.registered);

        _checkThresholdInvariants();
    }

    function _checkPerChainTransceiversInvariants() internal pure {
        // Send side
        uint16[] memory chains = _getChainsEnabledStorage(SEND_ENABLED_CHAINS_SLOT);
        uint256 len = chains.length;
        for (uint256 idx = 0; (idx < len);) {
            // Make sure there is an enabled transceiver for this chain.
            unchecked {
                ++idx;
            }
        }
    }

    function _checkThresholdInvariants() public view {
        uint16[] storage chains = _getChainsEnabledStorage(RECV_ENABLED_CHAINS_SLOT);
        uint256 len = chains.length;
        for (uint256 idx = 0; (idx < len);) {
            _checkThresholdInvariant(chains[idx]);
            unchecked {
                ++idx;
            }
        }
    }

    function _checkThresholdInvariant(
        uint16 chain
    ) internal view {
        // mapping(address => TransceiverInfo) storage transceiverInfos = _getTransceiverInfosStorage();
        _PerChainTransceiverData storage _data = _getPerChainRecvTransceiverDataStorage()[chain];

        uint8 numEnabled = countSetBits(_data.bitmap);

        // invariant: threshold <= enabledTransceivers.length
        if (_data.threshold > numEnabled) {
            revert TransceiverRegistry.ThresholdTooHigh(chain, _data.threshold, numEnabled);
        }

        if (numEnabled > 0) {
            if (_data.threshold == 0) {
                revert TransceiverRegistry.ZeroThreshold(chain);
            }
        }
    }
}
