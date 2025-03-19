// SPDX-License-Identifier: Apache 2
pragma solidity >=0.8.8 <0.9.0;

import "openzeppelin-contracts/contracts/token/ERC20/ERC20.sol";

// This is a simple token used for testing purposes.
contract ERC20Mintable is ERC20 {
    address public minter;

    constructor(string memory name, string memory symbol) ERC20(name, symbol) {
        minter = msg.sender;
    }

    function setMinter(address _minter) public {
        require(msg.sender == minter, "ERC20Mintable: only minter can set minter");
        minter = _minter;
    }

    function mint(address to, uint256 amount) public {
        require(msg.sender == minter, "ERC20Mintable: only minter can mint");
        _mint(to, amount);
    }

    function burn(uint256 amount) public {
        _burn(msg.sender, amount);
    }
}
