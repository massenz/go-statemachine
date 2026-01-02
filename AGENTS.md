# AGENTS.md

This file provides guidance to AI agents (Warp, Claude, Copilot, and others) when working with code in this repository.

## Project overview

This repository implements a gRPC-based finite state machine (FSM) server (`fsm-server`) backed by Redis, with asynchronous event processing via AWS SQS (or LocalStack) and a separate CLI client (`fsm-cli`) for sending configurations, FSMs, and events using YAML.

Key external dependencies and assumptions:
- Go 1.24.x
- Redis (single-node or cluster)
- AWS SQS or LocalStack for queueing events and error notifications
- Docker / Docker Compose for local stacks and many integration tests
- Ginkgo v1 CLI (`github.com/onsi/ginkgo/ginkgo`) for running tests

### Protocol Buffers

This service relies on the `statemachine-proto` [repository](https://github.com/massenz/statemachine-proto) for its protobuf definitions.

The definitions are all in  `api/statemachine.proto` (and can be found locally at `../../statemachine-proto/api/statemachine.proto`).

### Documentation

Relevant documentation:
- High-level design and data model: `README.md`
- CLI usage: `docs/cli.md` and `docs/examples/examples.yaml`
- TLS details: `docs/tls.md`
- Helm deployment: `helm/README.md`

## Build, test, and common commands

All commands below are assumed to run from the repository root.

All operations on the main server module (building, testing, etc.) are driven by the provided `Makefile`, which adds teh `make help` command to display all available commands and their descriptions.

Whenever validating your changes, please use either of `make build` or `make test` to ensure correctness.

All code changes ought to be made in their own branch: if the repo is not already in a separete branch, create one via `git checkout -b <branch-name>`, asking for a descriptive name, unless the instructions make it obvious what name to use.

### Tooling prerequisites

- Install Ginkgo v1 CLI (the Makefile and tests assume v1, not v2):
  - `go install github.com/onsi/ginkgo/ginkgo@v1.16.5` (or the version declared in `go.mod`)
- Ensure `yq` is installed; the shared Makefile (`thirdparty/common.mk`) reads `settings.yaml` via `yq` to determine the app name and version.
- Docker and Docker Compose must be available for integration tests and `make start` / `make cli-test`.

### Core Make targets

From `Makefile` and `thirdparty/common.mk`:

- Format code:
  - `make fmt`
- Build the gRPC server binary:
  - `make build`
  - Output: `build/bin/fsm-server-<release>_<GOOS>-<GOARCH>` (release and platform are derived from `settings.yaml` and `git rev-parse`).
- Run all tests (Ginkgo, with coverage):
  - `make test`
  - This will ensure TLS certs exist via the `check_certs` dependency and write coverage data under `build/reports`.
- Open coverage report in a browser:
  - `make coverage`
- Full local validation (build + certs + tests):
  - `make all`
- Generate TLS certs (used by both server and CLI):
  - `make certs`
- Clean build artifacts, coverage files, TLS certs, and container images:
  - `make clean`

Useful additional targets:

- Build Docker image for the server:
  - `make container`
- Start local integration stack (Redis, LocalStack, and the server container):
  - `make start`
  - Brings up Redis and LocalStack via `docker/compose.yaml`, creates `events` and `notifications` SQS queues via `aws` CLI, then runs the `fsm-server` container.
- Stop local stack:
  - `make stop`
- Build CLI client:
  - `make cli`
  - Output is placed under `build/bin/` with a name derived from `settings.yaml`.
- Run CLI client tests (requires Docker; uses its own compose file):
  - `make cli-test`

### Running tests with Ginkgo directly

Tests are written with Ginkgo v1 and usually live under `pkg/**` and `fsm-cli/client`.

Common patterns (outside of the Makefile):
- Run all tests under a single package:
  - `ginkgo ./pkg/storage`
- Focused specs within a package (helpful when iterating on a failing test):
  - `ginkgo -focus 'RedisStore' ./pkg/storage`

Many tests rely on Docker via `testcontainers-go` or LocalStack/Redis containers (see `pkg/internal/testing/containers.go`), so ensure Docker is running before invoking tests directly.

### Running the gRPC server locally

The canonical way to build and run the server for local development is:

1. Build the binary:
   - `make build`
2. Ensure Redis and (optionally) SQS/LocalStack are running.
3. Run the server binary from `build/bin/` with appropriate flags. Important flags (see `cmd/main.go`):
   - `-redis` (required):
     - Single-node Redis: `-redis host:port`
     - Cluster: comma-separated list of nodes and `-cluster=true` if you are using Redis cluster mode.
   - `-grpc-port` (default `7398`): gRPC listen port.
   - `-events`: SQS queue name for inbound events.
   - `-notifications`: SQS queue name for error/outcome notifications (optional; enables publisher if set).
   - `-endpoint-url`: HTTP URL for the SQS endpoint (e.g. `http://localhost:4566` for LocalStack).
   - `-timeout`, `-max-retries`: Redis timeout and retry settings.
   - `-debug` / `-trace`: enable verbose / very verbose logging across all components.
   - `-insecure`: disable TLS on the gRPC server.

The server logs the release string from `pkg/api.Release`, which is injected at build time via `-ldflags` in the Makefile.

### TLS configuration

TLS is enabled by default for the gRPC server and expected by the CLI unless explicitly disabled.

Key points:
- Cert generation:
  - `make certs` creates `ca.pem`, `server.pem`, and `server-key.pem` in the local `certs/` directory using CFSSL.
- Server-side:
  - gRPC TLS configuration is driven by `pkg/grpc.SetupTLSConfig` and `pkg/internal/config/tls.go`.
  - If `cfg.TlsCerts` is unset, the server looks for certs in:
    - `$TLS_CONFIG_DIR` if set, otherwise
    - `/etc/statemachine/certs` (the default).
  - The `-insecure` flag to `fsm-server` disables TLS entirely.
- Client-side (CLI):
  - By default, the CLI expects TLS and uses the CA certificate from its `CertsDir` (see `fsm-cli/client/tls` usage via `grpc.ParseCAFile`).
  - Use `-insecure` on the CLI to connect to a non-TLS server.

When running the server inside the provided Docker image, use volume mounts to supply certs at `/etc/statemachine/certs` and ensure the environment matches what `docs/tls.md` describes.

## Code architecture

This section captures the key architectural relationships so that non-trivial changes can be made without re-deriving the design from scratch.

### Top-level layout

- `cmd/main.go` – composition root for the `fsm-server` service.
- `pkg/api` – core FSM domain logic (config validation, event creation, simple in-memory FSM modeling).
- `pkg/storage` – storage abstractions (`StoreManager` and friends) plus the Redis-backed implementation.
- `pkg/pubsub` – SQS integration and the `EventsListener` that coordinates event processing and outcomes.
- `pkg/grpc` – gRPC service implementation and TLS setup, exposing the statemachine API from `statemachine-proto`.
- `pkg/internal` – internal helpers not meant to be part of the public API (currently TLS config files and test-only helpers).
- `fsm-cli` – separate Go module providing the CLI client binary, including YAML marshalling/unmarshalling and gRPC client wiring.
- `docker/` – Dockerfiles and compose definitions for local stacks.
- `helm/` – Helm chart for Kubernetes deployments.

### Runtime data flow

At a high level, the system is built around a single logical pipeline:

1. **Event ingress**
   - **gRPC**: clients call `StatemachineService.SendEvent` (implemented in `pkg/grpc/grpc_server.go`).
     - The server performs basic validation (destination ID and event name must be non-empty).
     - It enriches the event (ID and timestamp) via `pkg/api.UpdateEvent`.
     - The entire `EventRequest` is pushed onto a process-wide `eventsCh` channel.
   - **SQS**: an optional `SqsSubscriber` (`pkg/pubsub`) polls an SQS queue and converts messages into `EventRequest` instances, which are also written onto `eventsCh`.

2. **Event processing**
   - `cmd/main.go` constructs a single `EventsListener` from `pkg/pubsub` with:
     - `EventsChannel`: the shared `eventsCh` channel from above.
     - `StatemachinesStore`: a `StoreManager` (currently `RedisStore`).
     - `NotificationsChannel`: an optional channel for error outcomes when `-notifications` is configured.
   - `EventsListener.ListenForMessages` is the central consumer:
     - Validates each incoming `EventRequest` (must have FSM ID and configuration name).
     - Persists the raw event via `StoreManager.PutEvent`.
     - Calls `StoreManager.TxProcessEvent`, which:
       - Fetches the target FSM and its configuration from Redis.
       - Applies the event to the FSM using `api.ConfiguredStateMachine.SendEvent` (pure in-memory logic) inside a Redis transaction.
       - Updates the FSM record and the “FSMs-by-state” Redis sets in a single transactional block.
     - Builds an `EventResponse` with an appropriate `EventOutcome_StatusCode` (e.g. `Ok`, `FsmNotFound`, `InternalError`) and hands it off to `reportOutcome`.

3. **Outcome recording and notification**
   - `reportOutcome` stores event outcomes in Redis via `StoreManager.AddEventOutcome`, keyed by event ID and configuration name.
   - If `NotificationsChannel` is configured, `PostNotificationAndReportOutcome` also forwards outcomes to the channel, where an `SqsPublisher` picks them up and publishes them to the configured SQS notifications queue.
   - Clients retrieve outcomes via the gRPC `GetEventOutcome` method, which looks up the outcome in Redis by event ID + config.

4. **Configuration and FSM lifecycle**
   - gRPC methods in `pkg/grpc/grpc_server.go` manage configurations and FSMs:
     - `PutConfiguration` validates config objects using `api.CheckValid` (states, starting state, reachability) and then persists them via `StoreManager.PutConfig`. It also maintains Redis sets for configs and their versions.
     - `GetAllConfigurations` and `GetConfiguration` read configuration names and specific `name:version` pairs from Redis.
     - `PutFiniteStateMachine` ensures the referenced configuration exists, sets default state from `Configuration.StartingState` when omitted, persists the FSM, and records the FSM’s presence in “FSMs-by-state” sets via `UpdateState`.
     - `GetFiniteStateMachine`, `GetAllInState`, and `StreamAllInstate` expose different query paths for FSMs.

### Storage layer

The `pkg/storage` package encapsulates all access to Redis. The key design points:

- `StoreManager` is a composite interface that includes config, FSM, and event stores, plus health checks and logging.
- `RedisStore` provides the implementation, with a shared `redis.UniversalClient` and configurable timeout and retry settings.
- All protobufs are serialized with `proto.Marshal` / `proto.Unmarshal` and stored as binary values.
- Internal helpers (`get`, `put`) implement retry loops with basic backoff for timeout-related errors.
- FSM state is duplicated in two places:
  - The FSM object itself (current state).
  - Redis sets keyed by configuration + state (`GetAllInState` / `UpdateState`), allowing efficient reverse lookups.
- Event outcomes are stored separately from events so that outcome queries do not require scanning history.

Understanding and respecting this abstraction boundary is important when adding new persistence features: new behaviors should generally be implemented via `StoreManager` rather than direct Redis access from higher layers.

### TLS and configuration management

- TLS-specific configuration constants live in `pkg/internal/config/tls.go` and are consumed by `pkg/grpc.SetupTLSConfig`.
- The server expects certs and keys to exist at a directory determined by `TLS_CONFIG_DIR` or the default `/etc/statemachine/certs`.
- The CLI mirrors this behavior by loading the CA certificate from its own certs directory, using the same `ParseCAFile` helper from the gRPC package.

When modifying TLS behavior, keep the server and CLI in sync and update `docs/tls.md` accordingly.

### CLI client architecture

The `fsm-cli` submodule encapsulates all client-side concerns and should be kept decoupled from server internals beyond the public gRPC surface:

- `fsm-cli/cmd/main.go` handles CLI flags (`-addr`, `-insecure`), command dispatch (`send`, `get`, `version`), and prints user-facing messages.
- `fsm-cli/client/client.go` wraps the generated gRPC client and exposes higher-level operations:
  - `NewClient` sets up TLS or insecure transport based on flags and CLI configuration.
  - `Send` reads YAML from a file or stdin, determines `kind` (`Configuration`, `FiniteStateMachine`, `EventRequest`, etc.), and dispatches through `SendHandlers` to call the correct gRPC method(s).
  - `sendEvent` sends an event and then polls `GetEventOutcome` with retries until the outcome is stored in Redis.
  - `Get` retrieves either configurations or FSMs and prints them back as YAML.

The CLI’s YAML schema and examples live under `docs/examples/examples.yaml` and `data/*.yaml`. New user-facing behaviors should be reflected in these files.

### Testing strategy

- The main server module uses Ginkgo + Gomega tests per package (see `*_suite_test.go` files under `pkg/`).
- Integration tests rely heavily on `testcontainers-go` to spin up ephemeral Redis and LocalStack containers (see `pkg/internal/testing/containers.go`).
- The CLI has its own test suite under `fsm-cli/client`, executed via `make cli-test`, which uses a dedicated Docker Compose file (`docker/cli-test-compose.yaml`) and environment-specific configuration.

When introducing new features, prefer to:
- Extend or add package-level Ginkgo suites under the appropriate `pkg/**` directory.
- For any behavior that touches Redis, SQS, or the event pipeline, add or extend tests that use the existing `testcontainers` helpers rather than baking in assumptions about local services.
