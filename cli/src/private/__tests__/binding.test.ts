import { describe, expect, test } from "bun:test";

import { hexToBytes, recipientBinding } from "../binding.js";
import { parseCantonPartyId } from "../types.js";
import { RETIRED_CANTON_NAMESPACE } from "./fixtures/registry.js";

describe("recipientBinding", () => {
  // Golden vectors from `recipient_binding` on the guardian VM (2026-08).
  test.each([
    [
      "alice",
      "b57bfe41337237d696834be81e491c78099971fc2bc1ad6eedd032583350a516",
    ],
    [
      "operator",
      "efd963b986b976980dc83bfc06974761a4fb4498683abe194242da93222e4051",
    ],
  ])("%s", (hint, expectedBinding) => {
    expect(
      recipientBinding(
        parseCantonPartyId(`${hint}::${RETIRED_CANTON_NAMESPACE}`)
      ) as string
    ).toBe(expectedBinding);
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
