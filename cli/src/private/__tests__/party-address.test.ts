import { describe, expect, test } from "bun:test";

import { hexToBytes } from "../hex.js";
import { partyAddressFor } from "../recipient.js";
import { parseCantonPartyId, type CantonPartyId } from "../types.js";
import { RETIRED_CANTON_NAMESPACE } from "./fixtures/registry.js";

// partyAddressFor is the Canton SDK's partyAddress; these vectors pin it for the CLI's recipients.
describe("partyAddressFor", () => {
  // Golden vectors for the tag "wormhole:v1" (ntt 0.7.0), computed with an
  // independent keccak256 that also reproduces the 0.6.x vectors of the old
  // tag "wormhole:ntt-recipient:v1" (alice b57bfe41..., operator efd963b9...).
  test.each([
    [
      "alice",
      "642e20b7253475adf8c136f61e1258a70c3809ec07256f5adef300f181923413",
    ],
    [
      "operator",
      "c3732dd70aeb99e67e1029897e0e39c3616fff4369228df2452301eccbfaab31",
    ],
  ])("%s", (hint, expectedAddress) => {
    expect(
      partyAddressFor(
        parseCantonPartyId(`${hint}::${RETIRED_CANTON_NAMESPACE}`)
      ) as string
    ).toBe(expectedAddress);
  });

  // The vector that Daml TestNtt:testPartyAddressForVector pins.
  test("vector-recipient::1220deadbeef", () => {
    expect(
      partyAddressFor(
        "vector-recipient::1220deadbeef" as CantonPartyId
      ) as string
    ).toBe("4c3feac3d35c3b4bcd02a9b3faa309c8ed0b33163e2161908f691980319f7947");
  });
});

describe("hexToBytes", () => {
  test.each([
    ["0x0102", [1, 2]],
    ["", []],
  ])("%p", (hexText, expected) => {
    expect([...hexToBytes(hexText)]).toEqual(expected);
  });
  test.each([["0x123"], ["zz"]])("rejects %p", (hexText) => {
    expect(() => hexToBytes(hexText)).toThrow();
  });
});
