// Field readers for the private-deployments registry. Each throws with the JSON path.

export function failRegistryField(context: string, message: string): never {
  throw new Error(`private-deployments registry: ${context}: ${message}`);
}

export function readObject(
  value: unknown,
  context: string
): Record<string, unknown> {
  if (typeof value !== "object" || value === null || Array.isArray(value)) {
    failRegistryField(
      context,
      `expected an object, got ${JSON.stringify(value)}`
    );
  }
  return value as Record<string, unknown>;
}

/** Rejects keys outside allowedKeys, so a stale schema field fails loudly. */
export function checkOnlyKnownKeys(
  registryObject: Record<string, unknown>,
  allowedKeys: readonly string[],
  context: string
): void {
  for (const key of Object.keys(registryObject)) {
    if (!allowedKeys.includes(key))
      failRegistryField(context, `unknown field "${key}"`);
  }
}

export function readNonEmptyString(
  value: unknown,
  context: string,
  maxLength = 256
): string {
  if (typeof value !== "string" || value.length === 0) {
    failRegistryField(
      context,
      `expected a non-empty string, got ${JSON.stringify(value)}`
    );
  }
  if (value.length > maxLength)
    failRegistryField(context, `longer than ${maxLength} characters`);
  return value;
}

export function readWithParser<Parsed>(
  value: unknown,
  context: string,
  parser: (fieldValue: never) => Parsed
): Parsed {
  try {
    return parser(value as never);
  } catch (err) {
    failRegistryField(context, (err as Error).message);
  }
}

const LOOPBACK_HOSTNAMES = new Set(["localhost", "127.0.0.1", "[::1]"]);

/** https URL without user info, query or fragment. http is accepted only for a loopback host. */
export function parseEndpointUrl(text: string, context: string): URL {
  let url: URL;
  try {
    url = new URL(text);
  } catch {
    failRegistryField(context, `not a URL: "${text}"`);
  }
  const isLoopbackHttp =
    url.protocol === "http:" && LOOPBACK_HOSTNAMES.has(url.hostname);
  if (url.protocol !== "https:" && !isLoopbackHttp) {
    failRegistryField(
      context,
      `protocol must be https: (http: only for localhost), got "${url.protocol}"`
    );
  }
  if (url.username !== "" || url.password !== "")
    failRegistryField(context, "URL has a user info part");
  if (url.search !== "") failRegistryField(context, "URL has a query part");
  if (url.hash !== "") failRegistryField(context, "URL has a fragment part");
  return url;
}
