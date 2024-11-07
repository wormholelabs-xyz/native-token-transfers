// SPDX-License-Identifier: Apache 2
pragma solidity ^0.8.19;

import "example-gmp-router/evm/src/Router.sol";

contract MockRouter is Router {
    constructor(
        uint16 _chainId
    ) Router(_chainId) {}
}
