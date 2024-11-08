// SPDX-License-Identifier: Apache 2

pragma solidity >=0.8.8 <0.9.0;

import "./NttManagerHelpers.sol";
import "../mocks/DummyTransceiver.sol";
import "../../src/mocks/DummyToken.sol";
import "../../src/NttManager/NttManager.sol";
import "../../src/libraries/TrimmedAmount.sol";

library TransceiverHelpersLib {
    using TrimmedAmountLib for TrimmedAmount;

    // 0x99'E''T''T'
    bytes4 constant TEST_TRANSCEIVER_PAYLOAD_PREFIX = 0x99455454;

    function setup_transceivers(
        NttManager nttManager,
        uint16 peerChainId
    ) internal returns (DummyTransceiver, DummyTransceiver) {
        DummyTransceiver e1 =
            new DummyTransceiver(nttManager.chainId(), address(nttManager.router()));
        DummyTransceiver e2 =
            new DummyTransceiver(nttManager.chainId(), address(nttManager.router()));
        nttManager.setTransceiver(address(e1));
        nttManager.enableSendTransceiver(peerChainId, address(e1));
        nttManager.enableRecvTransceiver(peerChainId, address(e1));
        nttManager.setTransceiver(address(e2));
        nttManager.enableSendTransceiver(peerChainId, address(e2));
        nttManager.enableRecvTransceiver(peerChainId, address(e2));
        nttManager.setThreshold(2);
        return (e1, e2);
    }

    function transferAttestAndReceive(
        address to,
        bytes32 id,
        uint16 toChain,
        NttManager nttManager,
        NttManager recipientNttManager,
        TrimmedAmount amount,
        TrimmedAmount inboundLimit,
        DummyTransceiver[] memory transceivers
    )
        internal
        returns (
            TransceiverStructs.NttManagerMessage memory m,
            DummyTransceiver.Message memory rmsg
        )
    {
        m = buildNttManagerMessage(to, id, toChain, nttManager, amount);
        bytes memory encodedM = TransceiverStructs.encodeNttManagerMessage(m);
        prepTokenReceive(nttManager, recipientNttManager, amount, inboundLimit);
        rmsg = attestAndReceiveMsg(nttManager, recipientNttManager, 0, transceivers, encodedM);
    }

    function attestAndReceiveMsg(
        NttManager srcNttManager,
        NttManager dstNttManager,
        uint64 sequence,
        DummyTransceiver[] memory transceivers,
        bytes memory encodedM
    ) internal returns (DummyTransceiver.Message memory rmsg) {
        rmsg = DummyTransceiver.Message({
            srcChain: srcNttManager.chainId(),
            srcAddr: UniversalAddressLibrary.fromAddress(address(srcNttManager)),
            sequence: sequence,
            dstChain: dstNttManager.chainId(),
            dstAddr: UniversalAddressLibrary.fromAddress(address(dstNttManager)),
            payloadHash: keccak256(encodedM),
            refundAddr: address(0)
        });

        // Attest the message on all the transceivers.
        uint8 numTrans = uint8(transceivers.length);
        for (uint8 i; i < numTrans; i++) {
            transceivers[i].receiveMessage(rmsg);
        }

        // Execute the message.
        dstNttManager.executeMsg(
            srcNttManager.chainId(),
            UniversalAddressLibrary.fromAddress(address(srcNttManager)),
            0,
            encodedM
        );
    }

    function buildNttManagerMessage(
        address to,
        bytes32 id,
        uint16 toChain,
        NttManager nttManager,
        TrimmedAmount amount
    ) internal view returns (TransceiverStructs.NttManagerMessage memory) {
        DummyToken token = DummyToken(nttManager.token());

        return TransceiverStructs.NttManagerMessage(
            id,
            bytes32(0),
            TransceiverStructs.encodeNativeTokenTransfer(
                TransceiverStructs.NativeTokenTransfer({
                    amount: amount,
                    sourceToken: toWormholeFormat(address(token)),
                    to: toWormholeFormat(to),
                    toChain: toChain,
                    additionalPayload: ""
                })
            )
        );
    }

    function prepTokenReceive(
        NttManager nttManager,
        NttManager recipientNttManager,
        TrimmedAmount amount,
        TrimmedAmount inboundLimit
    ) internal {
        DummyToken token = DummyToken(nttManager.token());
        token.mintDummy(address(recipientNttManager), amount.untrim(token.decimals()));
        NttManagerHelpersLib.setConfigs(
            inboundLimit, nttManager, recipientNttManager, token.decimals()
        );
    }

    function buildTransceiverMessageWithNttManagerPayload(
        bytes32 id,
        bytes32 sender,
        bytes32 sourceNttManager,
        bytes32 recipientNttManager,
        bytes memory payload
    ) internal pure returns (TransceiverStructs.NttManagerMessage memory, bytes memory) {
        TransceiverStructs.NttManagerMessage memory m =
            TransceiverStructs.NttManagerMessage(id, sender, payload);
        bytes memory nttManagerMessage = TransceiverStructs.encodeNttManagerMessage(m);
        bytes memory transceiverMessage;
        (, transceiverMessage) = TransceiverStructs.buildAndEncodeTransceiverMessage(
            TEST_TRANSCEIVER_PAYLOAD_PREFIX,
            sourceNttManager,
            recipientNttManager,
            nttManagerMessage,
            new bytes(0)
        );
        return (m, transceiverMessage);
    }
}
