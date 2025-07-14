# Sui NTT

The project is structured into three packages:

- [`ntt`](./packages/ntt) (NTT manager)
- [`ntt_common`](./packages_ntt_common)
- [`wormhole_transciever`](./packages/wormhole_transceiver)

In the NTT architecture, the NTT manager implements the token transfer logic, and sends/receives messages via potentially multiple transceivers in a threshold configuration.
Here, we decouple the implementation of the manager from the transceiver, so they don't need to know about each other. Indeed, the `ntt` and `wormhole_transceiver` packages don't depend on each other.

Instead, they communicate via permissioned structs defined in [`ntt_common`](./packages/ntt_common).

## `ntt_common`

The [`ntt_common`](./packages/ntt_common) package contains common struct definitions that are shared between the NTT manager and transceivers.

There are two flavours of these structs: _unpermissioned_ and _permissioned_.

By unpermissioned, we mean that these can be created and destructed by any module.
These define the structs in the wire format, and as such come with (de)serialiser functions too. Holding a value of these types gives no guarantees about the provenance of the data, so they are exclusively used for structuring information. These structs are defined in the [`messages`](./packages/ntt_common/sources/messages) folder.

On the other hand, construction/consumption of permissioned structs is restricted, and thus provide specific gurantees about the information contained within.
The NTT manager sends messages by creating a value of type [`OutboundMessage`](./packages/ntt_common/sources/outbound_message.move), which the transceiver consumes and processes.
In the other direction, the NTT manager receives messages by consuming [`ValidatedTransceiverMessage`](./packages/ntt_common/sources/validated_transceiver_message.move) structs that are created by the transceiver. See the documentation in those modules for more details on the implementation.
These inbound/outbound permissioned structs are passed between the manager and transceivers in a programmable transaction block, meaning that the contracts don't directly call each other. As such, care has been taken to restrict the capabilities of these structs sufficiently to ensure that the client constructing the PTB can not do the wrong thing.

The intention is for a single `ntt_common` module to be deployed, and shared between all NTT manager & transceiver instances.

---

## Development and Testing

This implementation includes comprehensive testing infrastructure for local development and CI/CD.

### Prerequisites
- [Sui CLI](https://docs.sui.io/guides/developer/getting-started/sui-install) installed
- Node.js 18+ and npm
- Docker (optional, for containerized testing)

### Quick Start

1. **Start local Sui network:**
   ```bash
   make start-local        # Full featured with test wallets
   # OR
   make start-local-simple # Minimal setup for quick testing
   ```

2. **Test the network:**
   ```bash
   make test-network  # Validate running network functionality
   ```

3. **Build and test Move packages:**
   ```bash
   make build-packages
   make test
   ```

4. **Test TypeScript SDK:**
   ```bash
   make test-ts
   ```

5. **Run integration tests:**
   ```bash
   make test-integration
   ```

### Available Make Targets

- `make test` - Run Move contract tests
- `make test-ts` - Run TypeScript SDK tests  
- `make test-build` - Test build infrastructure (recommended)
- `make test-integration` - Run deployment integration tests
- `make test-docker` - Run tests in Docker environment
- `make start-local` - Start local Sui network (full featured)
- `make start-local-simple` - Start simple local Sui network
- `make test-network` - Test running local network functionality
- `make stop-local` - Stop local Sui network and cleanup
- `make build-packages` - Build all Move packages
- `make clean` - Clean all build artifacts
- `make help` - Show all available targets

### Directory Structure

```
sui/
├── packages/                    # Move contracts
│   ├── ntt/                    # Core NTT implementation
│   ├── ntt_common/             # Shared utilities and types
│   └── wormhole_transceiver/   # Wormhole message passing
├── ts/                         # TypeScript SDK
│   ├── src/ntt.ts             # Main SDK implementation
│   └── __tests__/             # Unit and integration tests
├── scripts/                    # Development and testing scripts
├── docker/                     # Docker testing environment
└── Makefile                    # Build and test automation
```

See `TESTING_PLAN.md` for comprehensive testing documentation.
