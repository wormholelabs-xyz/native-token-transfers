// SPDX-License-Identifier: Apache 2

pragma solidity >=0.8.8 <0.9.0;

import "forge-std/Test.sol";
import "./mocks/MockNttManager.sol";
import "../src/interfaces/IManagerBase.sol";
import "openzeppelin-contracts/contracts/proxy/ERC1967/ERC1967Proxy.sol";
import {DummyTransceiver} from "./NttManager.t.sol";
import {DummyToken} from "./NttManager.t.sol";
import "./mocks/MockRouter.sol";

contract OwnershipTests is Test {
    MockRouter router;
    NttManager nttManager;
    uint16 constant chainId = 7;

    function setUp() public {
        DummyToken t = new DummyToken();
        router = new MockRouter(chainId);

        NttManager implementation = new MockNttManagerContract(
            address(router), address(t), IManagerBase.Mode.LOCKING, chainId, 1 days, false
        );

        nttManager = MockNttManagerContract(address(new ERC1967Proxy(address(implementation), "")));
        nttManager.initialize();
    }

    function test_setUp() public {}

    // The transceiver is no longer owned by the NttManager. It's owned by the router. Nothing to test here.
    // function checkOwnership(DummyTransceiver e, address nttManagerOwner) public {
    //     address transceiverNttManager = e.getNttManagerOwner();
    //     assertEq(transceiverNttManager, nttManagerOwner);
    // }

    // /// transceiver retrieves the nttManager owner correctly
    // function testTransceiverOwnership() public {
    //     // TODO: use setup_transceivers here
    //     DummyTransceiver e1 = new DummyTransceiver(address(nttManager));
    //     nttManager.setTransceiver(address(e1));
    //     nttManager.setThreshold(1);

    //     checkOwnership(e1, nttManager.owner());
    // }
}
