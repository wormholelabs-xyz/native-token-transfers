// SPDX-License-Identifier: Apache 2
pragma solidity >=0.8.8 <0.9.0;

import "forge-std/Test.sol";

import "../src/libraries/TransceiverStructs.sol";
import "../src/interfaces/IManagerBase.sol";
import "../src/interfaces/INttManager.sol";
import "./libraries/TransceiverHelpers.sol";

contract TestTransceiverStructs is Test {
    using TrimmedAmountLib for uint256;
    using TrimmedAmountLib for TrimmedAmount;

    // TODO: add some negative tests for unknown message types etc

    function test_serialize_NttManagerMessage() public {
        TransceiverStructs.NativeTokenTransfer memory ntt = TransceiverStructs.NativeTokenTransfer({
            amount: packTrimmedAmount(uint64(1234567), 7),
            sourceToken: hex"BEEFFACE",
            to: hex"FEEBCAFE",
            toChain: 17,
            additionalPayload: ""
        });

        TransceiverStructs.NttManagerMessage memory mm = TransceiverStructs.NttManagerMessage({
            id: hex"128434bafe23430000000000000000000000000000000000ce00aa0000000000",
            sender: hex"46679213412343",
            payload: TransceiverStructs.encodeNativeTokenTransfer(ntt)
        });

        bytes memory encoded = TransceiverStructs.encodeNttManagerMessage(mm);

        TransceiverStructs.NttManagerMessage memory mmParsed =
            TransceiverStructs.parseNttManagerMessage(encoded);

        // deep equality check
        assertEq(abi.encode(mmParsed), abi.encode(mm));

        TransceiverStructs.NativeTokenTransfer memory nttParsed =
            TransceiverStructs.parseNativeTokenTransfer(mmParsed.payload);

        // deep equality check
        assertEq(abi.encode(nttParsed), abi.encode(ntt));
    }

    function test_serialize_NttManagerMessageWithAdditionalPayload() public {
        TransceiverStructs.NativeTokenTransfer memory ntt = TransceiverStructs.NativeTokenTransfer({
            amount: packTrimmedAmount(uint64(1234567), 7),
            sourceToken: hex"BEEFFACE",
            to: hex"FEEBCAFE",
            toChain: 17,
            additionalPayload: hex"deadbeef000000000000000000000000000000000000000000000000deadbeef"
        });

        TransceiverStructs.NttManagerMessage memory mm = TransceiverStructs.NttManagerMessage({
            id: hex"128434bafe23430000000000000000000000000000000000ce00aa0000000000",
            sender: hex"46679213412343",
            payload: TransceiverStructs.encodeNativeTokenTransfer(ntt)
        });

        bytes memory encoded = TransceiverStructs.encodeNttManagerMessage(mm);

        TransceiverStructs.NttManagerMessage memory mmParsed =
            TransceiverStructs.parseNttManagerMessage(encoded);

        // deep equality check
        assertEq(abi.encode(mmParsed), abi.encode(mm));

        TransceiverStructs.NativeTokenTransfer memory nttParsed =
            TransceiverStructs.parseNativeTokenTransfer(mmParsed.payload);

        // deep equality check
        assertEq(abi.encode(nttParsed), abi.encode(ntt));
    }

    function test_SerdeRoundtrip_NttManagerMessage(
        TransceiverStructs.NttManagerMessage memory m
    ) public {
        bytes memory message = TransceiverStructs.encodeNttManagerMessage(m);

        TransceiverStructs.NttManagerMessage memory parsed =
            TransceiverStructs.parseNttManagerMessage(message);

        assertEq(m.id, parsed.id);
        assertEq(m.sender, parsed.sender);
        assertEq(m.payload, parsed.payload);
    }

    function test_SerdeJunk_NttManagerMessage(
        TransceiverStructs.NttManagerMessage memory m
    ) public {
        bytes memory message = TransceiverStructs.encodeNttManagerMessage(m);

        bytes memory junk = "junk";

        vm.expectRevert(
            abi.encodeWithSelector(
                BytesParsing.LengthMismatch.selector, message.length + junk.length, message.length
            )
        );
        TransceiverStructs.parseNttManagerMessage(abi.encodePacked(message, junk));
    }

    function test_SerdeRoundtrip_NativeTokenTransfer(
        TransceiverStructs.NativeTokenTransfer memory m
    ) public {
        bytes memory message = TransceiverStructs.encodeNativeTokenTransfer(m);

        TransceiverStructs.NativeTokenTransfer memory parsed =
            TransceiverStructs.parseNativeTokenTransfer(message);

        assertEq(m.amount.getAmount(), parsed.amount.getAmount());
        assertEq(m.to, parsed.to);
        assertEq(m.toChain, parsed.toChain);
    }

    function test_SerdeJunk_NativeTokenTransfer(
        TransceiverStructs.NativeTokenTransfer memory m
    ) public {
        bytes memory message = TransceiverStructs.encodeNativeTokenTransfer(m);

        bytes memory junk = "junk";

        uint256 expectedRead = message.length;
        if (m.additionalPayload.length == 0) {
            // when there isn't an additionalPayload, "ju" is interpreted as the length
            expectedRead = message.length + 2 + 0x6A75;
        }

        vm.expectRevert(
            abi.encodeWithSelector(
                BytesParsing.LengthMismatch.selector, message.length + junk.length, expectedRead
            )
        );
        TransceiverStructs.parseNativeTokenTransfer(abi.encodePacked(message, junk));
    }
}
