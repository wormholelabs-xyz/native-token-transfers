// SPDX-License-Identifier: Apache 2
pragma solidity ^0.8.19;

import "example-messaging-endpoint/evm/src/Endpoint.sol";

contract MockEndpoint is Endpoint {
    constructor(
        uint16 _chainId
    ) Endpoint(_chainId) {}
}
