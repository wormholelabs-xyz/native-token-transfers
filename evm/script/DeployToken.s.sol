// SPDX-License-Identifier: Apache 2
pragma solidity >=0.8.8 <0.9.0;

import {Script, console} from "forge-std/Script.sol";
import "openzeppelin-contracts/contracts/token/ERC20/ERC20.sol";
import "./helpers/ERC20Mintable.sol";

interface IWormhole {
    function chainId() external view returns (uint16);
}

contract DeployToken is Script {
    function deployMintableToken(string memory name, string memory symbol) public {
        vm.startBroadcast();

        ERC20Mintable token = new ERC20Mintable(name, symbol);
        console.log("Token deployed at: ", address(token));

        vm.stopBroadcast();
    }

    function setMinter(address token, address minter) public {
        vm.startBroadcast();

        ERC20Mintable(token).setMinter(minter);

        vm.stopBroadcast();
    }
}
