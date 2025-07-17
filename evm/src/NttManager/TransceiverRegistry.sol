// SPDX-License-Identifier: Apache 2
pragma solidity >=0.8.8 <0.9.0;

/// @dev TransceiverRegistryBase is a base class shared between TransceiverRegistry and TransceiverRegistryAdmin.
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

    /// @notice Error when attempting to enable a transceiver that is already enabled.
    /// @dev Selector 0x8d68f84d.
    /// @param transceiver The address of the transceiver.
    error TransceiverAlreadyEnabled(address transceiver);

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

    // @dev Check that the transceiver is in a valid state.
    function _checkTransceiverInvariants(
        address transceiver
    ) private view {
        (bool success, bytes memory returnData) = _admin.staticcall(
            abi.encodeWithSelector(
                TransceiverRegistryAdmin._checkTransceiverInvariants.selector, transceiver
            )
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
                revert("_removeTransceiver reverted");
            }
        }
    }
}

/// @dev TransceiverRegistryAdmin is a helper contract to TransceiverRegistry.
///      It implements admin functionality and is called via `delegatecall`.
contract TransceiverRegistryAdmin is TransceiverRegistryBase {
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

        // if an transceiver is not registered, it should not be enabled
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
    }
}
