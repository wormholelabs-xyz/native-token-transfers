import { describe, expect, test } from "bun:test";
import * as fs from "node:fs";
import * as os from "node:os";
import * as path from "node:path";

import {
  REPOSITORY_REGISTRY_PATH,
  loadRegistry,
  requireDeployment,
  resolveRegistryPath,
  validateRegistry,
} from "../config.js";
import { retiredCanburnRegistryJson } from "./fixtures/registry.js";

function withChange(change: (registryJson: any) => void): any {
  const registryJson = retiredCanburnRegistryJson();
  change(registryJson);
  return registryJson;
}

describe("validateRegistry", () => {
  test("loads the fixture with one deployment", () => {
    const registry = validateRegistry(retiredCanburnRegistryJson());
    expect([...registry.deployments.keys()]).toEqual(["canburn" as never]);
    expect(registry.canton.participant?.jsonApiUrl.href).toBe(
      "https://participant.example.test/"
    );
  });

  test("loads a registry with no deployments and no participant", () => {
    const registry = validateRegistry(
      withChange((registryJson) => {
        registryJson.deployments = {};
        delete registryJson.canton.participant;
      })
    );
    expect(registry.deployments.size).toBe(0);
    expect(() => requireDeployment(registry, "canburn")).toThrow(
      /no deployments/
    );
  });

  test("unknown --deployment lists the registry keys", () => {
    expect(() =>
      requireDeployment(
        validateRegistry(retiredCanburnRegistryJson()),
        "canhub"
      )
    ).toThrow(/canburn/);
  });

  test.each([
    ["schemaVersion 1", (r: any) => (r.schemaVersion = 1)],
    ["v1 namespace field", (r: any) => (r.canton.namespace = "1220")],
    ["v1 guardianPubkey field", (r: any) => (r.canton.guardianPubkey = "04")],
    [
      "http disclosure URL",
      (r: any) =>
        (r.canton.disclosureServiceUrl = "http://disclosure.example.test"),
    ],
    [
      "user info in the JSON API URL",
      (r: any) =>
        (r.canton.participant.jsonApiUrl =
          "https://user:pw@participant.example.test"),
    ],
    [
      "query in the token URL",
      (r: any) =>
        (r.canton.participant.oauthTokenUrl =
          "https://idp.example.test/token?x=1"),
    ],
    [
      "bare party hint",
      (r: any) => (r.canton.participant.userPartyId = "alice"),
    ],
    [
      "decimals 19",
      (r: any) => (r.deployments.canburn.chains.Canton.decimals = 19),
    ],
    [
      "decimals -1",
      (r: any) => (r.deployments.canburn.chains.Ethereum.decimals = -1),
    ],
    ["chain id 0", (r: any) => (r.chainIds.Solana = 0)],
    ["chain id 65536", (r: any) => (r.chainIds.Solana = 65536)],
    ["duplicate chain ids", (r: any) => (r.chainIds.Solana = 2)],
    [
      "bad EVM checksum",
      (r: any) =>
        (r.deployments.canburn.chains.Ethereum.manager =
          "0xFd2322c8b9D79c605A4263F1baCeC9a0B4Ff6d42"),
    ],
    [
      "31-byte Solana mint",
      (r: any) =>
        (r.deployments.canburn.chains.Solana.mint =
          "HxzGfiD91zPh6eYoP8363a8eDom4tpCoNWwSo7C1G"),
    ],
    [
      "uppercase manager hex",
      (r: any) =>
        (r.deployments.canburn.chains.Canton.manager =
          "D03C78C99B240126E5E18BF07BAE40847BB41B345B1850C71AFB75E809F24D08"),
    ],
    [
      "deployment name with a capital",
      (r: any) => (r.deployments.CanBurn = r.deployments.canburn),
    ],
    [
      "one-chain deployment",
      (r: any) =>
        (r.deployments.canburn.chains = {
          Canton: r.deployments.canburn.chains.Canton,
        }),
    ],
    ["network Devnet", (r: any) => (r.network = "Devnet")],
  ])("rejects %s", (_caseName, change) => {
    expect(() => validateRegistry(withChange(change))).toThrow();
  });

  test("accepts http for a loopback participant", () => {
    const registry = validateRegistry(
      withChange(
        (r) => (r.canton.participant.jsonApiUrl = "http://localhost:7575")
      )
    );
    expect(registry.canton.participant?.jsonApiUrl.hostname).toBe("localhost");
  });
});

describe("resolveRegistryPath", () => {
  test("explicit path wins", () => {
    expect(resolveRegistryPath("/tmp/a.json", "/tmp/b.json")).toBe(
      "/tmp/a.json"
    );
  });
  test("environment path is next", () => {
    expect(resolveRegistryPath(undefined, "/tmp/b.json")).toBe("/tmp/b.json");
  });
  test("a registry file in the working directory is ignored", () => {
    const workingDirectory = fs.mkdtempSync(
      path.join(os.tmpdir(), "ntt-registry-")
    );
    const previousDirectory = process.cwd();
    try {
      fs.writeFileSync(
        path.join(workingDirectory, "private-deployments.json"),
        "{}"
      );
      process.chdir(workingDirectory);
      expect(resolveRegistryPath(undefined, undefined)).toBe(
        REPOSITORY_REGISTRY_PATH
      );
    } finally {
      process.chdir(previousDirectory);
      fs.rmSync(workingDirectory, { recursive: true });
    }
  });
});

describe("loadRegistry", () => {
  test("the repository registry loads and is marked as the repository file", () => {
    const { registry, source } = loadRegistry(undefined, undefined);
    expect(source.isRepositoryFile).toBe(true);
    expect(source.sha256Hex).toMatch(/^[0-9a-f]{64}$/);
    expect(registry.network).toBe("Mainnet");
  });

  test("an explicit file is used, hashed, and marked as not the repository file", () => {
    const directory = fs.mkdtempSync(path.join(os.tmpdir(), "ntt-registry-"));
    try {
      const registryPath = path.join(directory, "local.json");
      fs.writeFileSync(
        registryPath,
        JSON.stringify(retiredCanburnRegistryJson())
      );
      const { source } = loadRegistry(registryPath, undefined);
      expect(source.absolutePath).toBe(registryPath);
      expect(source.isRepositoryFile).toBe(false);
    } finally {
      fs.rmSync(directory, { recursive: true });
    }
  });
});
