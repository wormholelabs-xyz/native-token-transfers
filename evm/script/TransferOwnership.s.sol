// SPDX-License-Identifier: Apache 2
pragma solidity >=0.8.8 <0.9.0;

import {Script, console2} from "forge-std/Script.sol";
import "../src/interfaces/ITransceiver.sol";
import "../src/interfaces/INttManager.sol";

contract TransferOwnership is Script {
    struct Env {
        ITransceiver transceiverAddress;
    }

    function _readEnvVariables() internal view returns (Env memory params) {
        params.transceiverAddress = ITransceiver(vm.envAddress("NTT_NEW_TRANSCEIVER_ADDRESS"));
        require(address(params.managerAddress) != address(0x0), "Invalid manager address");
    }

    function run() public {
        vm.startBroadcast();
        Env memory params = _readEnvVariables();
        console2.log("Manager address:", address(params.managerAddress));
        console2.log("Transceiver address:", address(params.transceiverAddress));

        // TODO: `getTransceiverType` does not exist for the wormhole
        // transceiver, so we should default to wormhole if it fails
        string memory transceiverType = params.transceiverAddress.getTransceiverType();
        console2.log("Type of transceiver:", transceiverType);
        params.managerAddress.setTransceiver(address(params.transceiverAddress));

        uint8 threshold = params.managerAddress.getThreshold();
        params.managerAddress.setThreshold(threshold + 1);
        vm.stopBroadcast();
    }
}
