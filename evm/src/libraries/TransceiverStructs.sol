// SPDX-License-Identifier: Apache 2
pragma solidity >=0.8.8 <0.9.0;

import "wormhole-solidity-sdk/libraries/BytesParsing.sol";
import "./TrimmedAmount.sol";

library TransceiverStructs {
    using BytesParsing for bytes;
    using TrimmedAmountLib for TrimmedAmount;

    /// @notice Error thrown when the payload length exceeds the allowed maximum.
    /// @dev Selector 0xa3419691.
    /// @param size The size of the payload.
    error PayloadTooLong(uint256 size);

    /// @notice Error thrown when the prefix of an encoded message
    ///         does not match the expected value.
    /// @dev Selector 0x56d2569d.
    /// @param prefix The prefix that was found in the encoded message.
    error IncorrectPrefix(bytes4 prefix);

    /// @dev Prefix for all NativeTokenTransfer payloads
    ///      This is 0x99'N''T''T'
    bytes4 constant NTT_PREFIX = 0x994E5454;

    /// @dev Message emitted and received by the nttManager contract.
    ///      The wire format is as follows:
    ///      - id - 32 bytes
    ///      - sender - 32 bytes
    ///      - payloadLength - 2 bytes
    ///      - payload - `payloadLength` bytes
    struct NttManagerMessage {
        /// @notice unique message identifier
        /// @dev This is incrementally assigned on EVM chains, but this is not
        /// guaranteed on other runtimes.
        bytes32 id;
        /// @notice original message sender address.
        bytes32 sender;
        /// @notice payload that corresponds to the type.
        bytes payload;
    }

    function nttManagerMessageDigest(
        uint16 sourceChainId,
        NttManagerMessage memory m
    ) public pure returns (bytes32) {
        return _nttManagerMessageDigest(sourceChainId, encodeNttManagerMessage(m));
    }

    function _nttManagerMessageDigest(
        uint16 sourceChainId,
        bytes memory encodedNttManagerMessage
    ) internal pure returns (bytes32) {
        return keccak256(abi.encodePacked(sourceChainId, encodedNttManagerMessage));
    }

    function encodeNttManagerMessage(
        NttManagerMessage memory m
    ) public pure returns (bytes memory encoded) {
        if (m.payload.length > type(uint16).max) {
            revert PayloadTooLong(m.payload.length);
        }
        uint16 payloadLength = uint16(m.payload.length);
        return abi.encodePacked(m.id, m.sender, payloadLength, m.payload);
    }

    /// @notice Parse a NttManagerMessage.
    /// @param encoded The byte array corresponding to the encoded message
    /// @return nttManagerMessage The parsed NttManagerMessage struct.
    function parseNttManagerMessage(
        bytes memory encoded
    ) public pure returns (NttManagerMessage memory nttManagerMessage) {
        uint256 offset = 0;
        (nttManagerMessage.id, offset) = encoded.asBytes32Unchecked(offset);
        (nttManagerMessage.sender, offset) = encoded.asBytes32Unchecked(offset);
        uint256 payloadLength;
        (payloadLength, offset) = encoded.asUint16Unchecked(offset);
        (nttManagerMessage.payload, offset) = encoded.sliceUnchecked(offset, payloadLength);
        encoded.checkLength(offset);
    }

    /// @dev Native Token Transfer payload.
    ///      The wire format is as follows:
    ///      - NTT_PREFIX - 4 bytes
    ///      - numDecimals - 1 byte
    ///      - amount - 8 bytes
    ///      - sourceToken - 32 bytes
    ///      - to - 32 bytes
    ///      - toChain - 2 bytes
    ///      - additionalPayloadLength - 2 bytes, optional
    ///      - additionalPayload - `additionalPayloadLength` bytes
    struct NativeTokenTransfer {
        /// @notice Amount being transferred (big-endian u64 and u8 for decimals)
        TrimmedAmount amount;
        /// @notice Source chain token address.
        bytes32 sourceToken;
        /// @notice Address of the recipient.
        bytes32 to;
        /// @notice Chain ID of the recipient
        uint16 toChain;
        /// @notice Custom payload
        /// @dev Recommended that the first 4 bytes are a unique prefix
        bytes additionalPayload;
    }

    function encodeNativeTokenTransfer(
        NativeTokenTransfer memory m
    ) public pure returns (bytes memory encoded) {
        // The `amount` and `decimals` fields are encoded in reverse order compared to how they are declared in the
        // `TrimmedAmount` type. This is consistent with the Rust NTT implementation.
        TrimmedAmount transferAmount = m.amount;
        if (m.additionalPayload.length > 0) {
            if (m.additionalPayload.length > type(uint16).max) {
                revert PayloadTooLong(m.additionalPayload.length);
            }
            uint16 additionalPayloadLength = uint16(m.additionalPayload.length);
            return abi.encodePacked(
                NTT_PREFIX,
                transferAmount.getDecimals(),
                transferAmount.getAmount(),
                m.sourceToken,
                m.to,
                m.toChain,
                additionalPayloadLength,
                m.additionalPayload
            );
        }
        return abi.encodePacked(
            NTT_PREFIX,
            transferAmount.getDecimals(),
            transferAmount.getAmount(),
            m.sourceToken,
            m.to,
            m.toChain
        );
    }

    /// @dev Parse a NativeTokenTransfer.
    /// @param encoded The byte array corresponding to the encoded message
    /// @return nativeTokenTransfer The parsed NativeTokenTransfer struct.
    function parseNativeTokenTransfer(
        bytes memory encoded
    ) public pure returns (NativeTokenTransfer memory nativeTokenTransfer) {
        uint256 offset = 0;
        bytes4 prefix;
        (prefix, offset) = encoded.asBytes4Unchecked(offset);
        if (prefix != NTT_PREFIX) {
            revert IncorrectPrefix(prefix);
        }

        // The `amount` and `decimals` fields are parsed in reverse order compared to how they are declared in the
        // `TrimmedAmount` struct. This is consistent with the Rust NTT implementation.
        uint8 numDecimals;
        (numDecimals, offset) = encoded.asUint8Unchecked(offset);
        uint64 amount;
        (amount, offset) = encoded.asUint64Unchecked(offset);
        nativeTokenTransfer.amount = packTrimmedAmount(amount, numDecimals);

        (nativeTokenTransfer.sourceToken, offset) = encoded.asBytes32Unchecked(offset);
        (nativeTokenTransfer.to, offset) = encoded.asBytes32Unchecked(offset);
        (nativeTokenTransfer.toChain, offset) = encoded.asUint16Unchecked(offset);
        // The additional payload may be omitted, but if it is included, it is prefixed by a u16 for its length.
        // If there are at least 2 bytes remaining, attempt to parse the additional payload.
        if (encoded.length >= offset + 2) {
            uint256 payloadLength;
            (payloadLength, offset) = encoded.asUint16Unchecked(offset);
            (nativeTokenTransfer.additionalPayload, offset) =
                encoded.sliceUnchecked(offset, payloadLength);
        }
        encoded.checkLength(offset);
    }
}
