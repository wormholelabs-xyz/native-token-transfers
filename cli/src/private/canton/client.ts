// Thin Canton JSON Ledger API v2 client: Keycloak client-credentials token
// mint/cache and a bun/global-fetch wrapper. No pure logic lives here — that
// stays in acs.ts/ntt.ts so it's testable without a live participant.
import type { KeycloakConfig } from "../config.js";

export interface CantonClientOptions {
  jsonApi: string;
  keycloak: KeycloakConfig;
  clientSecret: string;
}

interface CachedToken {
  accessToken: string;
  expiresAt: number; // epoch ms
}

export class CantonClient {
  private readonly jsonApi: string;
  private readonly keycloak: KeycloakConfig;
  private readonly clientSecret: string;
  private cached?: CachedToken;

  constructor(opts: CantonClientOptions) {
    this.jsonApi = opts.jsonApi.replace(/\/+$/, "");
    this.keycloak = opts.keycloak;
    this.clientSecret = opts.clientSecret;
  }

  private tokenUrl(): string {
    return `${this.keycloak.url.replace(/\/+$/, "")}/auth/realms/${this.keycloak.realm}/protocol/openid-connect/token`;
  }

  private async mintToken(): Promise<CachedToken> {
    const body = new URLSearchParams({
      grant_type: "client_credentials",
      client_id: this.keycloak.clientId,
      client_secret: this.clientSecret,
    });
    const res = await fetch(this.tokenUrl(), { method: "POST", body });
    const text = await res.text();
    if (!res.ok) {
      throw new Error(`keycloak token mint failed: ${res.status} ${text}`);
    }
    const json = JSON.parse(text);
    if (!json.access_token) {
      throw new Error(`keycloak token mint failed: no access_token in response`);
    }
    // 30s skew so a token doesn't expire mid-flight against the participant.
    const expiresInMs = (json.expires_in ?? 300) * 1000;
    return {
      accessToken: json.access_token,
      expiresAt: Date.now() + expiresInMs - 30_000,
    };
  }

  /** Mints on first use / after expiry; callers never see raw token plumbing. */
  async token(): Promise<string> {
    if (!this.cached || Date.now() >= this.cached.expiresAt) {
      this.cached = await this.mintToken();
    }
    return this.cached.accessToken;
  }

  private async request(
    path: string,
    init: RequestInit,
    retryOn401 = true
  ): Promise<Response> {
    const token = await this.token();
    const res = await fetch(`${this.jsonApi}${path}`, {
      ...init,
      headers: { ...(init.headers ?? {}), Authorization: `Bearer ${token}` },
    });
    if (res.status === 401 && retryOn401) {
      this.cached = undefined;
      return this.request(path, init, false);
    }
    return res;
  }

  async get(path: string): Promise<any> {
    const res = await this.request(path, { method: "GET" });
    const text = await res.text();
    if (!res.ok) throw new Error(`GET ${path} failed: ${res.status} ${text}`);
    return text.length > 0 ? JSON.parse(text) : undefined;
  }

  async post(path: string, body: unknown): Promise<any> {
    const res = await this.request(path, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });
    const text = await res.text();
    const json = text.length > 0 ? JSON.parse(text) : undefined;
    if (!res.ok) {
      throw new Error(`POST ${path} failed: ${res.status} ${JSON.stringify(json ?? text)}`);
    }
    return json;
  }

  async ledgerEnd(): Promise<number> {
    const res = await this.get("/v2/state/ledger-end");
    return res.offset;
  }

  async activeContracts(parties: string[], offset: number): Promise<any> {
    const filtersByParty: Record<string, unknown> = {};
    for (const p of parties) {
      filtersByParty[p] = {
        cumulative: [
          { identifierFilter: { WildcardFilter: { value: { includeCreatedEventBlob: false } } } },
        ],
      };
    }
    return this.post("/v2/state/active-contracts", {
      filter: { filtersByParty },
      verbose: false,
      activeAtOffset: offset,
    });
  }

  async submitAndWaitForTransactionTree(body: unknown): Promise<any> {
    return this.post("/v2/commands/submit-and-wait-for-transaction-tree", body);
  }
}
