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
            new DummyTransceiver(nttManager.chainId(), address(nttManager.endpoint()));
        DummyTransceiver e2 =
            new DummyTransceiver(nttManager.chainId(), address(nttManager.endpoint()));
        nttManager.setTransceiver(address(e1));
        nttManager.enableSendTransceiver(peerChainId, address(e1));
        nttManager.enableRecvTransceiver(peerChainId, address(e1));
        nttManager.setTransceiver(address(e2));
        nttManager.enableSendTransceiver(peerChainId, address(e2));
        nttManager.enableRecvTransceiver(peerChainId, address(e2));
        nttManager.setThreshold(peerChainId, 2);
        return (e1, e2);
    }

    function addTransceiver(
        NttManager nttManager,
        DummyTransceiver e1,
        uint16 peerChainId
    ) internal returns (DummyTransceiver, DummyTransceiver) {
        DummyTransceiver e2 =
            new DummyTransceiver(nttManager.chainId(), address(nttManager.endpoint()));
        nttManager.setTransceiver(address(e2));
        nttManager.enableSendTransceiver(peerChainId, address(e2));
        nttManager.enableRecvTransceiver(peerChainId, address(e2));
        nttManager.setThreshold(peerChainId, 2);
        return (e1, e2);
    }

    function transferAndAttest(
        address to,
        bytes32 id,
        NttManager nttManager,
        NttManager recipientNttManager,
        TrimmedAmount amount,
        TrimmedAmount inboundLimit,
        DummyTransceiver[] memory transceivers
    )
        internal
        returns (
            TransceiverStructs.NttManagerMessage memory m,
            bytes memory encodedM,
            DummyTransceiver.Message memory rmsg
        )
    {
        m = buildNttManagerMessage(to, id, recipientNttManager.chainId(), nttManager, amount);
        encodedM = TransceiverStructs.encodeNttManagerMessage(m);
        prepTokenReceive(nttManager, recipientNttManager, amount, inboundLimit);
        rmsg = attestMsg(nttManager, recipientNttManager, 0, transceivers, encodedM);
    }

    function transferAttestAndReceive(
        address to,
        bytes32 id,
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
        m = buildNttManagerMessage(to, id, recipientNttManager.chainId(), nttManager, amount);
        bytes memory encodedM = TransceiverStructs.encodeNttManagerMessage(m);
        prepTokenReceive(nttManager, recipientNttManager, amount, inboundLimit);
        rmsg = attestAndReceiveMsg(nttManager, recipientNttManager, 0, transceivers, encodedM);
    }

    function attestMsg(
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
    }

    function attestAndReceiveMsg(
        NttManager srcNttManager,
        NttManager dstNttManager,
        uint64 sequence,
        DummyTransceiver[] memory transceivers,
        bytes memory encodedM
    ) internal returns (DummyTransceiver.Message memory rmsg) {
        rmsg = attestMsg(srcNttManager, dstNttManager, sequence, transceivers, encodedM);

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

    error TransferSentEventNotFoundInLogs(uint64 nttSeqNo);
    error ExecutorEventNotFoundInLogs(uint64 nttSeqNo, bytes32 payloadHash);

    function getExecutionSent(
        Vm.Log[] memory events,
        address nttManager,
        uint64 nttSeqNo
    ) public pure returns (bytes memory payload) {
        // To find the payload bytes from the logs we need to do it in two steps:
        // 1. Look for the TransferSent event for our NttManager and sequence number to get the payload hash.
        // 2. Look at the RequestForExecution events to find one where the hash of its payload matches what we are looking for.
        for (uint256 idx = 0; idx < events.length; ++idx) {
            if (
                events[idx].topics[0]
                    == bytes32(0x75eb8927cc7c4810b30fa2e8011fce37da6da7d18eb82c642c367ae4445c3625)
                    && events[idx].emitter == nttManager
            ) {
                (,,, uint64 sequence, bytes32 msgHash) =
                    abi.decode(events[idx].data, (uint256, uint256, uint16, uint64, bytes32));

                if (sequence == nttSeqNo) {
                    for (idx = 0; idx < events.length; ++idx) {
                        if (
                            events[idx].topics[0]
                                == bytes32(
                                    0xd870d87e4a7c33d0943b0a3d2822b174e239cc55c169af14cc56467a4489e3b5
                                )
                        ) {
                            (,,,,, payload,) = abi.decode(
                                events[idx].data,
                                (uint256, uint16, bytes32, address, bytes, bytes, bytes)
                            );
                            bytes32 payloadHash = keccak256(payload);
                            if (payloadHash == msgHash) {
                                return payload;
                            }
                        }
                    }
                    revert ExecutorEventNotFoundInLogs(nttSeqNo, msgHash);
                }
            }
        }
        revert TransferSentEventNotFoundInLogs(nttSeqNo);
    }
}
