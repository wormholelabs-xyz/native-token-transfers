import { describe, expect, test } from "bun:test";
import * as fs from "node:fs";
import * as os from "node:os";
import * as path from "node:path";
import { fileURLToPath } from "node:url";
import {
  DEPLOYMENT_NAMES,
  loadConfig,
  parseChainAlias,
  resolveConfigPath,
  validateConfig,
} from "../config.js";

const REPO_ROOT = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  "../../../.."
);
const REGISTRY_PATH = path.join(REPO_ROOT, "private-deployments.json");

function loadValidRaw(): any {
  return JSON.parse(fs.readFileSync(REGISTRY_PATH, "utf8"));
}

describe("committed private-deployments.json", () => {
  test("parses and validates", () => {
    const config = validateConfig(loadValidRaw());
    expect(config.network).toBe("Mainnet");
    expect(config.canton.namespace).toBe(
      "122085bf64156420a95663eb8b8cce516858de1d531d95850debe9ec8d15e3f6263e"
    );
  });

  test("all four deployments are present", () => {
    const config = validateConfig(loadValidRaw());
    for (const name of DEPLOYMENT_NAMES) {
      expect(config.deployments[name]).toBeDefined();
    }
  });

  test("canlock has no Solana leg; the other three do", () => {
    const config = validateConfig(loadValidRaw());
    expect(config.deployments.canlock.chains.Solana).toBeUndefined();
    expect(config.deployments.canhub.chains.Solana).toBeDefined();
    expect(config.deployments.ethlock.chains.Solana).toBeDefined();
    expect(config.deployments.canburn.chains.Solana).toBeDefined();
  });

  test("loadConfig resolves the same file via explicit path", () => {
    const config = loadConfig(REGISTRY_PATH);
    expect(config.deployments.canburn.chains.Canton.mode).toBe("burning");
  });
});

describe("validateConfig rejects broken registries", () => {
  test("bad hex in a 32-byte field", () => {
    const raw = loadValidRaw();
    raw.deployments.canlock.chains.Canton.manager = "not-hex";
    expect(() => validateConfig(raw)).toThrow(/32-byte hex/);
  });

  test("missing chain (Ethereum) in a deployment", () => {
    const raw = loadValidRaw();
    delete raw.deployments.canlock.chains.Ethereum;
    expect(() => validateConfig(raw)).toThrow(/missing chain "Ethereum"/);
  });

  test("unknown mode value", () => {
    const raw = loadValidRaw();
    raw.deployments.canhub.chains.Ethereum.mode = "wrapping";
    expect(() => validateConfig(raw)).toThrow(/mode "locking" or "burning"/);
  });

  test("missing deployment entirely", () => {
    const raw = loadValidRaw();
    delete raw.deployments.ethlock;
    expect(() => validateConfig(raw)).toThrow(
      /missing required deployment "ethlock"/
    );
  });

  test("non-integer decimals", () => {
    const raw = loadValidRaw();
    raw.deployments.canburn.chains.Solana.decimals = "9";
    expect(() => validateConfig(raw)).toThrow(/expected an integer/);
  });

  test("malformed Ethereum address", () => {
    const raw = loadValidRaw();
    raw.deployments.canlock.chains.Ethereum.token = "0xnotanaddress";
    expect(() => validateConfig(raw)).toThrow(/20-byte address/);
  });
});

describe("parseChainAlias", () => {
  test("accepts canonical names and short aliases, case-insensitively", () => {
    expect(parseChainAlias("Canton")).toBe("Canton");
    expect(parseChainAlias("canton")).toBe("Canton");
    expect(parseChainAlias("ETHEREUM")).toBe("Ethereum");
    expect(parseChainAlias("eth")).toBe("Ethereum");
    expect(parseChainAlias("Solana")).toBe("Solana");
    expect(parseChainAlias("SOL")).toBe("Solana");
  });

  test("rejects unknown chains", () => {
    expect(() => parseChainAlias("sui")).toThrow(/unknown chain/);
  });
});

describe("resolveConfigPath precedence", () => {
  test("explicit path wins over everything else", () => {
    expect(resolveConfigPath("/some/explicit/path.json")).toBe(
      "/some/explicit/path.json"
    );
  });

  test("NTT_PRIVATE_DEPLOYMENTS env var is used when no explicit path is given", () => {
    const prev = process.env.NTT_PRIVATE_DEPLOYMENTS;
    process.env.NTT_PRIVATE_DEPLOYMENTS = "/env/path.json";
    try {
      expect(resolveConfigPath()).toBe("/env/path.json");
    } finally {
      if (prev === undefined) delete process.env.NTT_PRIVATE_DEPLOYMENTS;
      else process.env.NTT_PRIVATE_DEPLOYMENTS = prev;
    }
  });

  test("falls back to walking up from the module directory to the repo root", () => {
    const prevEnv = process.env.NTT_PRIVATE_DEPLOYMENTS;
    const prevCwd = process.cwd();
    delete process.env.NTT_PRIVATE_DEPLOYMENTS;
    process.chdir(os.tmpdir());
    try {
      expect(resolveConfigPath()).toBe(REGISTRY_PATH);
    } finally {
      process.chdir(prevCwd);
      if (prevEnv !== undefined) process.env.NTT_PRIVATE_DEPLOYMENTS = prevEnv;
    }
  });
});
