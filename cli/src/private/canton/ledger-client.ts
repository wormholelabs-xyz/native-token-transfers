// JSON Ledger API v2 client for the user's own participant. The token acts as the user only.
import type { CantonInstrumentId } from "../config.js";
import {
  parseCantonContractId,
  parseCantonPartyId,
  type CantonContractId,
  type CantonPartyId,
} from "../types.js";
import {
  DEFAULT_HTTP_BOUNDS,
  errorBodyExcerpt,
  fetchJsonBounded,
  type HttpBounds,
} from "./http.js";

export const LEDGER_REQUEST_MAX_ATTEMPTS = 2;
// submit-and-wait returns after the synchronizer commits; confirmation by all
// stakeholder participants can take longer than a read.
export const LEDGER_SUBMIT_TIMEOUT_MS = 120_000;
export const USER_HOLDINGS_MAX = 1000;
const TOKEN_EXPIRY_MARGIN_MS = 30_000;
const TOKEN_LIFETIME_DEFAULT_SECONDS = 300;
// Package-name reference: resolves to the vetted holding interface package.
const HOLDING_INTERFACE_ID =
  "#splice-api-token-holding-v1:Splice.Api.Token.HoldingV1:Holding";

export interface UserHolding {
  contractId: CantonContractId;
  amountDecimal: string;
  instrumentId: CantonInstrumentId;
  locked: boolean;
}

export interface LedgerTransaction {
  events: unknown[];
}

export interface CantonLedger {
  readonly userPartyId: CantonPartyId;
  readUserHoldings(): Promise<UserHolding[]>;
  submitAndWaitForTransaction(request: unknown): Promise<LedgerTransaction>;
}

export interface CantonLedgerClientOptions {
  jsonApiUrl: URL;
  oauthTokenUrl: URL;
  oauthClientId: string;
  oauthClientSecret: string;
  userPartyId: CantonPartyId;
  /** HTTP function; defaults to fetchJsonBounded. Tests inject a recorder. */
  fetchJson?: typeof fetchJsonBounded;
}

interface CachedAccessToken {
  accessToken: string;
  expiresAtMs: number;
}

export class CantonLedgerClient implements CantonLedger {
  readonly userPartyId: CantonPartyId;
  private cachedAccessToken?: CachedAccessToken;

  private readonly fetchJson: typeof fetchJsonBounded;

  constructor(private readonly options: CantonLedgerClientOptions) {
    this.userPartyId = options.userPartyId;
    this.fetchJson = options.fetchJson ?? fetchJsonBounded;
  }

  private async fetchAccessToken(): Promise<CachedAccessToken> {
    const response = await this.fetchJson(this.options.oauthTokenUrl.href, {
      method: "POST",
      headers: { "Content-Type": "application/x-www-form-urlencoded" },
      body: new URLSearchParams({
        grant_type: "client_credentials",
        client_id: this.options.oauthClientId,
        client_secret: this.options.oauthClientSecret,
      }),
    });
    const tokenJson = response.json as
      | { access_token?: unknown; expires_in?: unknown }
      | undefined;
    if (
      response.status !== 200 ||
      typeof tokenJson?.access_token !== "string" ||
      tokenJson.access_token === ""
    ) {
      throw new Error(
        `OAuth token request to ${this.options.oauthTokenUrl.host} failed with HTTP ${response.status}`
      );
    }
    const lifetimeSeconds =
      typeof tokenJson.expires_in === "number" && tokenJson.expires_in > 0
        ? tokenJson.expires_in
        : TOKEN_LIFETIME_DEFAULT_SECONDS;
    return {
      accessToken: tokenJson.access_token,
      expiresAtMs: Date.now() + lifetimeSeconds * 1000 - TOKEN_EXPIRY_MARGIN_MS,
    };
  }

  private async accessToken(): Promise<string> {
    if (
      !this.cachedAccessToken ||
      Date.now() >= this.cachedAccessToken.expiresAtMs
    ) {
      this.cachedAccessToken = await this.fetchAccessToken();
    }
    return this.cachedAccessToken.accessToken;
  }

  /** One retry with a new token after a 401. */
  private async requestJson(
    method: "GET" | "POST",
    ledgerPath: string,
    body?: unknown,
    bounds: HttpBounds = DEFAULT_HTTP_BOUNDS
  ): Promise<unknown> {
    const url = new URL(ledgerPath, this.options.jsonApiUrl).href;
    for (
      let attemptNumber = 1;
      attemptNumber <= LEDGER_REQUEST_MAX_ATTEMPTS;
      attemptNumber++
    ) {
      const response = await this.fetchJson(
        url,
        {
          method,
          headers: {
            Authorization: `Bearer ${await this.accessToken()}`,
            ...(body === undefined
              ? {}
              : { "Content-Type": "application/json" }),
          },
          body: body === undefined ? undefined : JSON.stringify(body),
        },
        bounds
      );
      if (
        response.status === 401 &&
        attemptNumber < LEDGER_REQUEST_MAX_ATTEMPTS
      ) {
        this.cachedAccessToken = undefined;
        continue;
      }
      if (response.status < 200 || response.status > 299) {
        throw new Error(
          `${method} ${ledgerPath} failed with HTTP ${response.status}: ${errorBodyExcerpt(response.text)}`
        );
      }
      return response.json;
    }
    throw new Error(`${method} ${ledgerPath}: no attempt left`);
  }

  async ledgerEndOffset(): Promise<number> {
    const ledgerEnd = (await this.requestJson(
      "GET",
      "/v2/state/ledger-end"
    )) as { offset?: unknown };
    if (
      typeof ledgerEnd?.offset !== "number" ||
      !Number.isSafeInteger(ledgerEnd.offset) ||
      ledgerEnd.offset < 0
    ) {
      throw new Error("ledger-end response has no valid offset");
    }
    return ledgerEnd.offset;
  }

  async readUserHoldings(): Promise<UserHolding[]> {
    const offset = await this.ledgerEndOffset();
    const activeContracts = await this.requestJson(
      "POST",
      "/v2/state/active-contracts",
      {
        eventFormat: {
          filtersByParty: {
            [this.userPartyId]: {
              cumulative: [
                {
                  identifierFilter: {
                    InterfaceFilter: {
                      value: {
                        interfaceId: HOLDING_INTERFACE_ID,
                        includeInterfaceView: true,
                        includeCreatedEventBlob: false,
                      },
                    },
                  },
                },
              ],
            },
          },
          verbose: false,
        },
        activeAtOffset: offset,
      }
    );
    return parseUserHoldings(activeContracts, this.userPartyId);
  }

  async submitAndWaitForTransaction(
    request: unknown
  ): Promise<LedgerTransaction> {
    const response = (await this.requestJson(
      "POST",
      "/v2/commands/submit-and-wait-for-transaction",
      request,
      { ...DEFAULT_HTTP_BOUNDS, timeoutMs: LEDGER_SUBMIT_TIMEOUT_MS }
    )) as {
      transaction?: { events?: unknown };
    };
    if (!Array.isArray(response?.transaction?.events))
      throw new Error("submit response has no transaction events");
    return { events: response.transaction.events };
  }
}

/** Holding interface views owned by the user, from an active-contracts response. */
export function parseUserHoldings(
  activeContracts: unknown,
  userPartyId: CantonPartyId
): UserHolding[] {
  if (!Array.isArray(activeContracts))
    throw new Error("active-contracts response is not an array");
  if (activeContracts.length > USER_HOLDINGS_MAX) {
    throw new Error(
      `the user has more than ${USER_HOLDINGS_MAX} holdings; merge holdings first`
    );
  }
  const holdings: UserHolding[] = [];
  for (const entry of activeContracts) {
    const createdEvent = (entry as any)?.contractEntry?.JsActiveContract
      ?.createdEvent;
    if (!createdEvent) continue;
    const views = Array.isArray(createdEvent.interfaceViews)
      ? createdEvent.interfaceViews
      : [];
    const holdingView = views.find((view: any) =>
      String(view?.interfaceId ?? "").endsWith(
        ":Splice.Api.Token.HoldingV1:Holding"
      )
    );
    const viewValue = holdingView?.viewValue;
    if (!viewValue || viewValue.owner !== userPartyId) continue;
    if (
      typeof viewValue.amount !== "string" ||
      typeof viewValue.instrumentId?.id !== "string"
    )
      continue;
    holdings.push({
      contractId: parseCantonContractId(createdEvent.contractId),
      amountDecimal: viewValue.amount,
      instrumentId: {
        admin: parseCantonPartyId(viewValue.instrumentId.admin),
        id: viewValue.instrumentId.id,
      },
      locked: viewValue.lock !== null && viewValue.lock !== undefined,
    });
  }
  return holdings;
}
