#!/usr/bin/env bash
# Canton <-> Ethereum NTT end-to-end test, run by the ntt-canton-evm-e2e Tilt resource
# (Tiltfile.canton-evm). README.md in this directory describes the flow.
#
# Ethereum (Wormhole chain 2) is the hub: a DummyToken (18 decimals) in a LOCKING NttManager.
# Canton (Wormhole chain 75) is the spoke: a BURN/MINT deployment (8 decimals).
# The devnet guardian signs both directions. This script relays the VAAs itself.
#
#   1. EVM: deploy the token, NttManager and WormholeTransceiver.
#   2. Canton: deploy core, NTT and the burn/mint manager, with the EVM peer.
#   3. EVM: register the Canton peer.
#   4. EVM -> Canton: lock 2.5 tokens on EVM, mint 2.5 on Canton. A replay must fail.
#   5. Canton -> EVM: burn 1.0 token on Canton, release 1.0 on EVM.
set -euo pipefail

ETH_RPC="${ETH_RPC:-http://eth-devnet:8545}"
GUARDIAN_REST="${GUARDIAN_REST:-http://guardian:7071}"
# Not CANTON_HOST/CANTON_PORT: Kubernetes sets CANTON_PORT=tcp://<ip>:6865 for the canton Service.
CANTON_LEDGER_HOST="${CANTON_LEDGER_HOST:-canton}"
CANTON_LEDGER_PORT="${CANTON_LEDGER_PORT:-6865}"
# The participant's JSON Ledger API, only used to print update ids. Set it empty to skip them.
CANTON_JSON_API="${CANTON_JSON_API-http://canton:6864}"
EVM_DIR="${EVM_DIR:-/ntt/evm}"
NTT_TEST_DAR="${NTT_TEST_DAR:-/ntt/canton/test/.daml/dist/ntt-test-0.1.0.dar}"
WORK_DIR="${WORK_DIR:-$(mktemp -d)}"
RUN_LABEL="${RUN_LABEL:-$(date +%s)}"
WAIT_TIMEOUT_SECONDS="${WAIT_TIMEOUT_SECONDS:-300}"
VAA_TIMEOUT_SECONDS="${VAA_TIMEOUT_SECONDS:-120}"

# Devnet anvil account 15, which no other devnet test uses: deployer, owner and sender.
EVM_PRIVATE_KEY="${EVM_PRIVATE_KEY:-0x47b65307d0d654fd4f786b908c04af8fface7710fc998b37d219de19c39ee58c}"
# Receives the Canton -> EVM transfer. A fresh address per run, so its balance starts at 0.
EVM_RECIPIENT="${EVM_RECIPIENT:-$(cast wallet new --json | jq -er '.[0].address')}"
# The Wormhole core bridge on eth-devnet (scripts/devnet-consts.json in the wormhole repo).
CORE_BRIDGE="${CORE_BRIDGE:-0xC89Ce4735882C9F0f0FE26686c53074E09B0D550}"

readonly EVM_CHAIN_ID=2
readonly CANTON_CHAIN_ID=75
readonly EVM_TOKEN_DECIMALS=18
readonly CANTON_TOKEN_DECIMALS=8
# 200 = instant finality on the EVM watcher.
readonly EVM_CONSISTENCY_LEVEL=200
# 1,000,000 tokens at 18 decimals: no transfer in this test is rate limited.
readonly EVM_INBOUND_LIMIT=1000000000000000000000000
# EVM -> Canton: 2.5 tokens.
readonly TO_CANTON_AMOUNT=2500000000000000000
readonly TO_CANTON_EXPECTED_BALANCE="2.5"
# Canton -> EVM: 1.0 token, as integer units at Canton's 8 decimals and at EVM's 18.
readonly TO_EVM_RAW_AMOUNT=100000000
readonly TO_EVM_EXPECTED_AMOUNT=1000000000000000000

# The default profile builds an NttManager above the 24 KB contract size limit. The prod
# profile (via-IR) is the one canton/e2e/Dockerfile and evm/Dockerfile build with.
export FOUNDRY_PROFILE="${FOUNDRY_PROFILE:-prod}"

log() { echo "== $*"; }
fail() {
  echo "FAIL: $*" >&2
  exit 1
}

# lower <hex>: lowercase, without a 0x prefix.
lower() {
  local hex="${1#0x}"
  echo "${hex,,}"
}

# pad32 <hex address>: left-pad to 32 bytes, lowercase, without a 0x prefix.
pad32() {
  local hex
  hex="$(lower "$1")"
  printf '%064s' "$hex" | tr ' ' 0
}

# uint <cast output>: the number alone. cast prints large numbers as "1000 [1e3]".
uint() { echo "${1%% *}"; }

# wait_for <description> <command...>: retry the command until it succeeds.
wait_for() {
  local description="$1"
  shift
  local start=$SECONDS
  local deadline=$((SECONDS + WAIT_TIMEOUT_SECONDS))
  local next_report=$((SECONDS + 30))
  until "$@" >/dev/null 2>&1; do
    ((SECONDS < deadline)) || fail "timed out after ${WAIT_TIMEOUT_SECONDS}s waiting for $description"
    if ((SECONDS >= next_report)); then
      echo "$description: still waiting ($((SECONDS - start))s)"
      next_report=$((SECONDS + 30))
    fi
    sleep 2
  done
  echo "$description: ready"
}

core_bridge_deployed() {
  local code
  code="$(cast code --rpc-url "$ETH_RPC" "$CORE_BRIDGE")"
  [[ "$code" != "0x" ]]
}

# A TCP connect to the Ledger API, bounded so a dropped SYN cannot outlast the wait deadline.
canton_ledger_open() {
  # shellcheck disable=SC2016 # $0 and $1 expand in the child bash.
  timeout 5 bash -c 'exec 3<>"/dev/tcp/$0/$1"' "$CANTON_LEDGER_HOST" "$CANTON_LEDGER_PORT" 2>/dev/null
}

# first_json: the first JSON object on stdin, compacted. forge and cast --json can print
# other text (warnings) around the JSON, which jq would reject.
first_json() { sed -n '/^{/,$p' | jq -nc input; }

# evm_send <to> <signature> [args...]: send a transaction from EVM_PRIVATE_KEY, and fail
# unless it succeeds. Prints the receipt JSON. Set EVM_VALUE to attach wei.
evm_send() {
  local receipt
  receipt="$(cast send --rpc-url "$ETH_RPC" --private-key "$EVM_PRIVATE_KEY" --json \
    --value "${EVM_VALUE:-0}" "$@" | first_json)"
  [[ "$(jq -r .status <<<"$receipt")" == "0x1" ]] || fail "transaction $2 to $1 reverted: $receipt"
  # On stderr: callers capture the receipt from stdout.
  echo "evm tx $(jq -r .transactionHash <<<"$receipt") $2" >&2
  echo "$receipt"
}

evm_call() { cast call --rpc-url "$ETH_RPC" "$@"; }

# message_fee: the core bridge message fee, in wei.
message_fee() { uint "$(evm_call "$CORE_BRIDGE" "messageFee()(uint256)")"; }

# published_sequence <receipt JSON> <emitter>: the sequence of the core bridge message
# that <emitter> published in the transaction.
published_sequence() {
  local topic sequence_hex
  topic="$(cast keccak "LogMessagePublished(address,uint64,uint32,bytes,uint8)")"
  sequence_hex="$(jq -er --arg core "$(lower "$CORE_BRIDGE")" --arg topic "$topic" \
    --arg emitter "0x$(pad32 "$2")" '
      [.logs[] | select((.address | ascii_downcase | ltrimstr("0x")) == $core
                        and .topics[0] == $topic and .topics[1] == $emitter)]
      | if length == 1 then .[0].data[2:66] else error("expected one message, got \(length)") end
    ' <<<"$1")" || fail "no LogMessagePublished from $2 in the transaction"
  echo $((16#$sequence_hex))
}

# fetch_vaa <chain> <emitter> <sequence>: poll the guardian until the VAA is signed.
# Prints it as lowercase hex.
fetch_vaa() {
  local url deadline=$((SECONDS + VAA_TIMEOUT_SECONDS)) response
  url="$GUARDIAN_REST/v1/signed_vaa/$1/$(pad32 "$2")/$3"
  echo "waiting for VAA $1/$(pad32 "$2")/$3" >&2
  until response="$(curl -sf "$url")"; do
    ((SECONDS < deadline)) || fail "no VAA after ${VAA_TIMEOUT_SECONDS}s: $url"
    sleep 2
  done
  jq -er .vaaBytes <<<"$response" | base64 -d | od -An -v -tx1 | tr -d ' \n'
}

# canton_ledger_end: the participant's ledger end offset. Prints nothing without the JSON API.
canton_ledger_end() {
  [[ -n "$CANTON_JSON_API" ]] || return 0
  curl -sf "$CANTON_JSON_API/v2/state/ledger-end" | jq -er .offset
}

# canton_updates <from offset> <to offset>: print the update id and the root command of every
# transaction after <from> up to <to>, whichever parties it involves.
canton_updates() {
  [[ -n "$CANTON_JSON_API" && -n "$1" && -n "$2" ]] || return 0
  (($2 > $1)) || return 0
  jq -nc --argjson from "$1" --argjson to "$2" '{
      beginExclusive: $from, endInclusive: $to,
      updateFormat: {includeTransactions: {
        eventFormat: {filtersForAnyParty: {cumulative: [{identifierFilter: {WildcardFilter: {value: {includeCreatedEventBlob: false}}}}]}},
        transactionShape: "TRANSACTION_SHAPE_LEDGER_EFFECTS"}}}' |
    curl -sf -X POST -H 'content-type: application/json' --data-binary @- "$CANTON_JSON_API/v2/updates" |
    jq -r '
      .[].update.Transaction.value // empty | select(.events | length > 0)
      | (.events | min_by(.[].nodeId) | to_entries[0]) as $root
      | ($root.value.templateId | sub("^[^:]*:"; "")) as $template
      | "canton update \(.updateId) "
        + if $root.key == "ExercisedEvent" then "exercise \($template) \($root.value.choice)" else "create \($template)" end'
}

# canton_script <entry point> <input JSON file> <output JSON file> [upload]
# Prints the update ids of the transactions the script submitted.
canton_script() {
  local upload=no before after
  [[ "${4:-}" == upload ]] && upload=yes
  before="$(canton_ledger_end)" || before=""
  if ! dpm script \
    --dar "$NTT_TEST_DAR" \
    --script-name "Test.TiltE2E:$1" \
    --ledger-host "$CANTON_LEDGER_HOST" \
    --ledger-port "$CANTON_LEDGER_PORT" \
    --upload-dar "$upload" \
    --input-file "$2" \
    --output-file "$3" >"$WORK_DIR/$1.log" 2>&1; then
    grep -v '^[[:space:]]*at ' "$WORK_DIR/$1.log" >&2
    # A participant vets one package per name and version, so a changed ntt-test DAR
    # cannot replace the one an earlier run uploaded.
    if grep -q KNOWN_PACKAGE_VERSION "$WORK_DIR/$1.log"; then
      fail "this sandbox already has a different ntt-test 0.1.0 (Test.TiltE2E changed since the last run): restart the canton resource for a fresh sandbox"
    fi
    # The devnet bootstrap uploaded its own wormhole-core; NTT's must be an upgrade-compatible version.
    if grep -q 'NOT_VALID_UPGRADE_PACKAGE.*wormhole-core' "$WORK_DIR/$1.log"; then
      fail "the wormhole devnet's wormhole-core and the one NTT is built on (canton/dars) are not upgrade-compatible: use a wormhole checkout whose canton/core version matches NTT's (NTT_WORMHOLE_DIR, see README.md)"
    fi
    fail "Test.TiltE2E:$1 failed"
  fi
  after="$(canton_ledger_end)" || after=""
  canton_updates "$before" "$after" || echo "Test.TiltE2E:$1: could not list its Canton updates"
}

evm_address="$(cast wallet address --private-key "$EVM_PRIVATE_KEY")"
log "Run $RUN_LABEL: EVM account $evm_address, work dir $WORK_DIR"

log "0. Wait for eth-devnet, the guardian and Canton"
wait_for "eth-devnet core bridge" core_bridge_deployed
wait_for "guardian REST API" curl -sf "$GUARDIAN_REST/v1/heartbeats"
wait_for "Canton Ledger API" canton_ledger_open
if [[ -n "$CANTON_JSON_API" ]]; then
  if curl -sf "$CANTON_JSON_API/v2/version" >/dev/null; then
    echo "Canton JSON API: ready"
  else
    echo "Canton JSON API: not reachable at $CANTON_JSON_API, so no Canton update ids"
    CANTON_JSON_API=""
  fi
fi

log "1. EVM: deploy DummyToken, NttManager (locking) and WormholeTransceiver"
cd "$EVM_DIR"
forge create --rpc-url "$ETH_RPC" --private-key "$EVM_PRIVATE_KEY" --broadcast --json \
  src/mocks/DummyToken.sol:DummyToken >"$WORK_DIR/forge-create.log" 2>&1 ||
  {
    cat "$WORK_DIR/forge-create.log"
    fail "forge create DummyToken failed"
  }
token="$(first_json <"$WORK_DIR/forge-create.log" | jq -er .deployedTo)" ||
  {
    cat "$WORK_DIR/forge-create.log"
    fail "no deployedTo in the forge create output"
  }
# Anything besides the JSON, for the record.
grep -v '^[[:space:]]*[{}"]' "$WORK_DIR/forge-create.log" | sed 's/^/forge: /' || true
echo "evm tx $(first_json <"$WORK_DIR/forge-create.log" | jq -r .transactionHash) create DummyToken"
echo "token $token"
RELEASE_CORE_BRIDGE_ADDRESS="$CORE_BRIDGE" \
  RELEASE_TOKEN_ADDRESS="$token" \
  RELEASE_DECIMALS="$EVM_TOKEN_DECIMALS" \
  RELEASE_MODE=0 \
  RELEASE_CONSISTENCY_LEVEL="$EVM_CONSISTENCY_LEVEL" \
  forge script script/DeployWormholeNtt.s.sol \
  --rpc-url "$ETH_RPC" --private-key "$EVM_PRIVATE_KEY" --broadcast --slow >"$WORK_DIR/forge-deploy.log" 2>&1 ||
  {
    cat "$WORK_DIR/forge-deploy.log"
    fail "DeployWormholeNtt failed"
  }
evm_chain_id="$(cast chain-id --rpc-url "$ETH_RPC")"
deploy_broadcast="broadcast/DeployWormholeNtt.s.sol/$evm_chain_id/run-latest.json"
jq -r '.transactions[]
  | "evm tx \(.hash) " + if .transactionType == "CREATE" then "create \(.contractName // "contract")"
                         else "\(.function // "call") on \(.contractName // .contractAddress)" end' \
  "$deploy_broadcast"
# The script deploys two ERC1967Proxy contracts: the NttManager's, then the transceiver's.
evm_manager="$(jq -er '
    [.transactions[] | select(.transactionType == "CREATE" and .contractName == "ERC1967Proxy")]
    | if length == 2 then .[0].contractAddress else error("expected two proxies, got \(length)") end
  ' "$deploy_broadcast")"
evm_transceiver="$(uint "$(evm_call "$evm_manager" "getTransceivers()(address[])" | tr -d '[]' | cut -d, -f1)")"
echo "manager $evm_manager"
echo "transceiver $evm_transceiver"
[[ "$(uint "$(evm_call "$evm_manager" "chainId()(uint16)")")" == "$EVM_CHAIN_ID" ]] ||
  fail "the NttManager is not on Wormhole chain $EVM_CHAIN_ID"

log "2. Canton: core, NTT governance, burn/mint manager and the EVM peer"
jq -n --arg runLabel "$RUN_LABEL" --argjson chain "$EVM_CHAIN_ID" --argjson decimals "$EVM_TOKEN_DECIMALS" \
  --arg manager "$(pad32 "$evm_manager")" --arg transceiver "$(pad32 "$evm_transceiver")" \
  '{runLabel: $runLabel, peerChainId: $chain, peerManager: $manager, peerTransceiver: $transceiver,
    peerDecimals: $decimals}' >"$WORK_DIR/setup-in.json"
canton_script tiltE2eSetup "$WORK_DIR/setup-in.json" "$WORK_DIR/setup-out.json" upload
canton_manager="$(jq -er .managerAddress "$WORK_DIR/setup-out.json")"
canton_transceiver="$(jq -er .transceiverAddress "$WORK_DIR/setup-out.json")"
canton_recipient_address="$(jq -er .recipientAddress "$WORK_DIR/setup-out.json")"
echo "manager $canton_manager"
echo "transceiver $canton_transceiver"
echo "recipient $(jq -er .parties.recipient "$WORK_DIR/setup-out.json") (address $canton_recipient_address)"

log "3. EVM: register the Canton peer"
evm_send "$evm_manager" "setPeer(uint16,bytes32,uint8,uint256)" \
  "$CANTON_CHAIN_ID" "0x$canton_manager" "$CANTON_TOKEN_DECIMALS" "$EVM_INBOUND_LIMIT" >/dev/null
EVM_VALUE="$(message_fee)" evm_send "$evm_transceiver" "setWormholePeer(uint16,bytes32)" \
  "$CANTON_CHAIN_ID" "0x$canton_transceiver" >/dev/null

log "4. EVM -> Canton: lock 2.5 tokens on EVM, mint them on Canton"
evm_send "$token" "mintDummy(address,uint256)" "$evm_address" "$TO_CANTON_AMOUNT" >/dev/null
evm_send "$token" "approve(address,uint256)" "$evm_manager" "$TO_CANTON_AMOUNT" >/dev/null
delivery_price="$(evm_call "$evm_manager" "quoteDeliveryPrice(uint16,bytes)(uint256[],uint256)" \
  "$CANTON_CHAIN_ID" 0x00 | tail -n 1)"
receipt="$(EVM_VALUE="$(uint "$delivery_price")" evm_send "$evm_manager" "transfer(uint256,uint16,bytes32)" \
  "$TO_CANTON_AMOUNT" "$CANTON_CHAIN_ID" "0x$canton_recipient_address")"
sequence="$(published_sequence "$receipt" "$evm_transceiver")"
locked="$(uint "$(evm_call "$token" "balanceOf(address)(uint256)" "$evm_manager")")"
[[ "$locked" == "$TO_CANTON_AMOUNT" ]] || fail "the EVM NttManager holds $locked, expected $TO_CANTON_AMOUNT"
vaa="$(fetch_vaa "$EVM_CHAIN_ID" "$evm_transceiver" "$sequence")"
jq -n --slurpfile setup "$WORK_DIR/setup-out.json" --arg vaa "$vaa" --arg expected "$TO_CANTON_EXPECTED_BALANCE" \
  '{parties: $setup[0].parties, vaaBytes: $vaa, expectedAmount: $expected}' >"$WORK_DIR/receive-in.json"
canton_script tiltE2eReceive "$WORK_DIR/receive-in.json" "$WORK_DIR/receive-out.json"
echo "Canton recipient balance $(jq -r . "$WORK_DIR/receive-out.json"); replaying the VAA was rejected"

log "5. Canton -> EVM: burn 1.0 token on Canton, release it on EVM"
jq -n --slurpfile setup "$WORK_DIR/setup-out.json" --argjson chain "$EVM_CHAIN_ID" \
  --arg recipient "$(pad32 "$EVM_RECIPIENT")" --argjson amount "$TO_EVM_RAW_AMOUNT" \
  '{parties: $setup[0].parties, recipientChain: $chain, recipientAddress: $recipient, rawAmount: $amount}' \
  >"$WORK_DIR/send-in.json"
canton_script tiltE2eSend "$WORK_DIR/send-in.json" "$WORK_DIR/send-out.json"
emitter="$(jq -er .emitterAddress "$WORK_DIR/send-out.json")"
sequence="$(jq -er .sequence "$WORK_DIR/send-out.json")"
[[ "$emitter" == "$canton_transceiver" ]] || fail "the Canton send came from emitter $emitter, expected $canton_transceiver"
echo "sequence $sequence, payload $(jq -er .payload "$WORK_DIR/send-out.json")"
echo "Canton recipient balance after the burn $(jq -r .remainingBalance "$WORK_DIR/send-out.json")"
vaa="$(fetch_vaa "$CANTON_CHAIN_ID" "$emitter" "$sequence")"
evm_send "$evm_transceiver" "receiveMessage(bytes)" "0x$vaa" >/dev/null
received="$(uint "$(evm_call "$token" "balanceOf(address)(uint256)" "$EVM_RECIPIENT")")"
[[ "$received" == "$TO_EVM_EXPECTED_AMOUNT" ]] ||
  fail "the EVM recipient received $received, expected $TO_EVM_EXPECTED_AMOUNT"
locked="$(uint "$(evm_call "$token" "balanceOf(address)(uint256)" "$evm_manager")")"
[[ "$locked" == "$((TO_CANTON_AMOUNT - TO_EVM_EXPECTED_AMOUNT))" ]] ||
  fail "the EVM NttManager holds $locked after the release"
echo "EVM recipient $EVM_RECIPIENT received $received"

log "Canton <-> Ethereum NTT end-to-end test passed"
