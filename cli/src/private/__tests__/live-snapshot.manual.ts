// Manual live check, not part of `bun test src` (bun only picks up *.test.ts).
// Usage: CANTON_KEYCLOAK_CLIENT_SECRET=... bun src/private/__tests__/live-snapshot.manual.ts [deployment]
import { loadConfig } from "../config.js";
import { CantonClient } from "../canton/client.js";
import { snapshot } from "../canton/acs.js";

const config = loadConfig();
const name = (process.argv[2] ?? "canburn") as keyof typeof config.deployments;
const dep = config.deployments[name];
if (!dep?.chains.Canton) throw new Error(`no Canton leg in ${String(name)}`);
const client = new CantonClient({
  jsonApi: config.canton.jsonApi,
  keycloak: config.canton.keycloak,
  clientSecret: process.env.CANTON_KEYCLOAK_CLIENT_SECRET!,
});
const state = await snapshot(client, config.canton, dep.chains.Canton);
for (const [k, v] of Object.entries(state))
  console.log(`  ${k.padEnd(22)} ${typeof v === "string" ? v.slice(0, 40) : JSON.stringify(v)?.slice(0, 60)}`);
