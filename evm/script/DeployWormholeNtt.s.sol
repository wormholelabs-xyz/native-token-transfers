// SPDX-License-Identifier: Apache 2
pragma solidity >=0.8.8 <0.9.0;

import {Script, console} from "forge-std/Script.sol";
import {DeployWormholeNttBase} from "./helpers/DeployWormholeNttBase.sol";
import {INttManager} from "../src/interfaces/INttManager.sol";
import {IWormholeTransceiver} from "../src/interfaces/IWormholeTransceiver.sol";
import "../src/interfaces/IManagerBase.sol";
import "openzeppelin-contracts/contracts/token/ERC20/ERC20.sol";

interface IWormhole {
    function chainId() external view returns (uint16);
}

contract DeployWormholeNtt is Script, DeployWormholeNttBase {
    function run(
        address wormhole,
        address token,
        address wormholeRelayer,
        address specialRelayer,
        IManagerBase.Mode mode
    ) public {
        vm.startBroadcast();

        console.log("Deploying Wormhole Ntt...");
        IWormhole wh = IWormhole(wormhole);

        uint16 chainId = wh.chainId();

        console.log("Chain ID: ", chainId);

        DeploymentParams memory params = DeploymentParams({
            token: token,
            mode: mode,
            wormholeChainId: chainId,
            rateLimitDuration: 86400,
            shouldSkipRatelimiter: false,
            wormholeCoreBridge: wormhole,
            wormholeRelayerAddr: wormholeRelayer,
            specialRelayerAddr: specialRelayer,
            consistencyLevel: 202,
            gasLimit: 500000,
            outboundLimit: uint256(type(uint64).max) * 10 ** 10
        });

        // Deploy NttManager.
        address manager = deployNttManager(params);

        // Deploy Wormhole Transceiver.
        address transceiver = deployWormholeTransceiver(params, manager);

        // // Configure NttManager.
        configureNttManager(
            manager,
            transceiver,
            params.outboundLimit,
            params.shouldSkipRatelimiter
        );

        // INttManager manager = INttManager(
        //     address()
        // );
        // IWormholeTransceiver transceiver = IWormholeTransceiver(
        //     address()
        // );

        // manager.setPeer(
        //     1, // solana
        //     bytes32(
        //         0x059b6298b0d6e28628a114a7393bd357aabe27e05426337dbc3b480f91dbda90
        //     ),
        //     9, // decimals
        //     1000000000000000000 // inbound limit
        // );

        // transceiver.setWormholePeer(
        //     1, // solana
        //     bytes32(
        //         0x016df8fbfa17f59eee38e9347e78ad241060918b96f58691cf5739b004897a8b
        //     ) // transceiver PDA
        // );

        // uint256 amount = 5 * 10 ** 17;

        // set allowance
        // IERC20(manager.token()).approve(
        //     address(manager),
        //     amount
        // );

        // manager.transfer(
        //     amount,
        //     1, // solana
        //     bytes32(
        //         0x05ae102afb09ea97b8b04bb904abac11ecdb97191d18f688c3ed10fa2b470840
        //     ) // PAytVxxSUkQDDT69XG2mECPixpMAzQ7hg9gm2pmdFKu
        // );

        // bytes memory vaa = hex"010000000001002cf49b354f69c9e888c6d41258d8a3d6d564c0c71785e2aef4844ada415d532c6dd298e55b727e9233d466c4cb3d614513d75157b6215f30d504e3ea070dcc0300662bf537000000000001016df8fbfa17f59eee38e9347e78ad241060918b96f58691cf5739b004897a8b0000000000000003209945ff10059b6298b0d6e28628a114a7393bd357aabe27e05426337dbc3b480f91dbda9000000000000000000000000046475d067f7c2a388a7bb7fd5a9a4a68d7fa45c5009131139ec2b73da6b9ff78d02dd8657fc4b7c73c71ff08e35577f11ea90264719405ae102afb09ea97b8b04bb904abac11ecdb97191d18f688c3ed10fa2b470840004f994e5454080000000002faf0809715fd3a4e9c76698410c3d977e05a26e61bd07bb865ae4a22dec2a60e47c91f0000000000000000000000003d6cb9d16fd5ff33511d630fc1e98a8f5f93dd8527120000";
        // transceiver.receiveMessage(vaa);

        vm.stopBroadcast();
    }

    // invoke with
    function register(
        address managerAddress,
        address transceiverAddress,
        uint16 chainId,
        address otherManager,
        address otherTransceiver,
        uint8 decimals
    ) public {
        vm.startBroadcast();
        console.log("Registering Wormhole Ntt...");

        INttManager manager = INttManager(managerAddress);
        IWormholeTransceiver transceiver = IWormholeTransceiver(
            transceiverAddress
        );

        manager.setPeer(
            chainId,
            toUniversalAddress(otherManager),
            decimals,
            uint256(type(uint64).max) * 10 ** 10 // inbound limit
        );

        transceiver.setWormholePeer(
            chainId,
            toUniversalAddress(otherTransceiver)
        );
        vm.stopBroadcast();
    }
}
