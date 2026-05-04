# OpenAPI / Web UI Investigation for go-statemachine

**Date:** 2026-05-04  
**Status:** Recommendation (not yet implemented)

---

## Summary Recommendation

**Adopt gRPC server reflection + grpcui as the primary interactive tooling layer.**

Enable gRPC server reflection in `pkg/grpc/grpc_server.go` (a single import and one line in
`NewGrpcServer`) and run `grpcui` as a sidecar or development tool. This gives an interactive,
Swagger-UI-like web interface for exploring and calling every API method with zero REST-layer
complexity, no proto-file annotations, and no upstream coordination with the
`statemachine-proto` repository.

As a companion for hosted, human-readable API documentation, publish the `statemachine-proto`
module to the **buf schema registry (buf.build)** from within the proto repo's CI pipeline.
This is purely a proto-repo concern and requires no changes here.

**Do not adopt grpc-gateway at this time.** The cost (upstream proto-file annotations,
streaming-RPC limitations, an extra HTTP multiplexer, OpenAPI spec maintenance) is not
justified for a service that already has a well-typed gRPC surface and a CLI client.
Revisit if a REST API is specifically required by HTTP-only consumers.

---

## Option Comparison

| Criterion | grpc-gateway + Swagger UI | buf.build BSR | grpcui | gRPC Reflection + grpcurl |
|---|---|---|---|---|
| **Provides web UI** | Yes (Swagger UI) | Yes (hosted, read-only) | Yes (interactive, self-hosted) | No |
| **Provides OpenAPI spec** | Yes (protoc-gen-openapiv2) | No | No | No |
| **REST layer added** | Yes | No | No | No |
| **Proto annotation changes needed** | Yes (in separate repo) | No | No | No |
| **Works with protos in a separate repo** | Painful — needs upstream PR or fork | Easy — add buf.build module config to proto repo | Easy — server reflection or compiled descriptor file | Easy — server reflection |
| **Streaming RPC support** | Limited — SSE or WebSocket workarounds needed | N/A (docs only) | Full — maps streams to UI | Full — grpcurl supports streaming |
| **TLS complexity impact** | Adds HTTP listener; must expose two ports or add mux | None | Connects to existing gRPC port | Connects to existing gRPC port |
| **Code changes in this repo** | Large — gateway mux, HTTP router, Swagger UI serving | None | Small — enable reflection | Tiny — enable reflection |
| **Maintenance burden** | High — OpenAPI spec drifts if annotations lag behind proto changes | Low — managed by proto repo CI | Low — reflection is always in sync | Minimal |
| **Interactive testing** | Yes (Swagger UI) | No | Yes | CLI only |
| **Requires running extra process** | Yes (gateway binary or middleware) | No (cloud-hosted) | Yes (grpcui binary, optional) | No |
| **Offline / air-gapped capable** | Yes | No | Yes | Yes |
| **Go module changes in this repo** | Large | None | Small (one dependency) | Tiny (one import from `google.golang.org/grpc`) |

---

## Detailed Analysis

### 1. grpc-gateway + protoc-gen-openapiv2 + Swagger UI

[github.com/grpc-ecosystem/grpc-gateway](https://github.com/grpc-ecosystem/grpc-gateway) generates
a reverse-proxy that translates HTTP/JSON calls into gRPC calls and vice versa.
`protoc-gen-openapiv2` generates a Swagger/OpenAPI 2.0 spec from annotated proto files.

**How it would work here:**

1. Add `google.api.http` option annotations to every RPC method in
   `api/statemachine.proto` (in the **`statemachine-proto` repo**).
2. Run `protoc-gen-openapiv2` in the proto repo's build to emit a `.swagger.json` file.
3. In **this repo**, add a gateway mux (e.g. `runtime.NewServeMux`) inside `cmd/main.go`,
   register the generated `RegisterStatemachineServiceHandlerFromEndpoint` function, and
   start an HTTP listener alongside the existing gRPC listener.
4. Optionally serve the Swagger UI static assets from the HTTP listener.

**Why this is a poor fit right now:**

- The proto definitions live in `massenz/statemachine-proto`, a separate repo with its own
  release cycle. Every API annotation must be added there and a new Go module version
  published before this repo can consume it. That is two-repo coordination for every API
  change.
- Two of the current RPC methods — `StreamAllInstate` and `StreamAllConfigurations` — are
  server-streaming RPCs. grpc-gateway can represent these as chunked HTTP responses but the
  Swagger UI has no native streaming support, producing a degraded experience.
- `SendEvent` is intentionally asynchronous: the response contains only the event ID, not the
  outcome. Exposing this via a REST endpoint with a synchronous HTTP model is misleading to
  consumers who expect HTTP to be request-response.
- The TLS configuration is already non-trivial (`SetupTLSConfig`, mutual-TLS option,
  `TLS_CONFIG_DIR`). Adding an HTTP listener means either sharing the port (via `cmux`) or
  opening a second one, both of which add operational surface area.
- OpenAPI specs generated from proto files require the proto annotations to remain in sync.
  When they lag behind, the spec silently becomes inaccurate — a maintenance trap.

**When to revisit:** If a third-party HTTP-only client integration is required (e.g., a
webhook consumer that cannot speak gRPC), grpc-gateway is the right answer. At that point,
coordinate annotation changes with the `statemachine-proto` maintainers.

---

### 2. buf.build Schema Registry + API Explorer

[buf.build](https://buf.build) is a cloud-hosted schema registry and API explorer for
Protocol Buffers. Publishing a module to the BSR gives a rendered, browsable view of all
messages and RPC methods — think auto-generated HTML docs from proto comments.

**How it would work here:**

This is entirely a **`statemachine-proto` repo concern**. Add a `buf.yaml` module file to
that repo pointing to a `buf.build/<owner>/statemachine-proto` module name, then configure
its CI to run `buf push` on each release tag. The BSR then shows the API explorer at
`buf.build/<owner>/statemachine-proto`.

**No changes are needed in this repo.**

**Limitations:**

- The BSR API explorer is read-only and cloud-hosted; it cannot send real requests to a
  running server.
- Requires a buf.build account and intentional publishing of proto definitions. If the proto
  definitions are meant to stay private, the BSR free tier is public-only; private modules
  require a paid plan.
- Adds value as documentation (especially for external API consumers) but does not replace
  interactive tooling for developers.

**Verdict:** Worth doing in the `statemachine-proto` repo independently, but does not satisfy
the interactive testing / "Swagger UI" goal on its own.

---

### 3. grpcui — Interactive Web UI for gRPC

[github.com/fullstorydev/grpcui](https://github.com/fullstorydev/grpcui) is a standalone
binary (and embeddable Go library) that connects to a gRPC server and renders an interactive
HTML form-based UI — essentially Postman / Swagger UI for gRPC. It uses either **server
reflection** or a compiled proto descriptor file to discover the service schema at runtime.

**How it would work here:**

**Step A: Enable server reflection** (small change in this repo):

```go
// pkg/grpc/grpc_server.go (inside NewGrpcServer, after grpc.NewServer)
import "google.golang.org/grpc/reflection"

server := grpc.NewServer(grpc.Creds(creds))
protos.RegisterStatemachineServiceServer(server, &grpcSubscriber{Config: cfg})
reflection.Register(server)  // add this line
```

`google.golang.org/grpc/reflection` is already transitively present in the module graph via
`google.golang.org/grpc`; no new dependency is needed.

**Step B: Run grpcui** (no code changes; it is an external binary):

```bash
# Insecure (development)
grpcui -plaintext localhost:7398

# With TLS
grpcui -cacert certs/ca.pem -cert certs/server.pem -key certs/server-key.pem localhost:7398
```

This opens a browser tab at `http://localhost:8080` with a form for every RPC method,
including streaming ones. It requires a live, running `fsm-server` instance.

**grpcui can also be embedded** in the server binary itself as an additional HTTP handler on
a separate port, but that is optional and increases binary size.

**Strengths:**

- The UI is always perfectly in sync with the actual service — reflection is a runtime query,
  not a build-time artifact.
- No OpenAPI spec, no proto annotations, no second HTTP port for the core service.
- Works with streaming RPCs natively.
- Developer-only tool — it can be restricted to dev environments with a build tag or flag,
  keeping the production binary lean.
- TLS: `grpcui` understands gRPC TLS certificates natively.

**Limitations:**

- Requires a running server to be useful. Cannot be used as static API documentation.
- The web UI is functional but not as polished as a curated Swagger UI with rich
  descriptions. Proto comments do appear in the UI if reflection returns them, but
  presentation is minimal.
- Enabling reflection in production exposes the full service schema to anyone who can reach
  the gRPC port. This is usually acceptable for internal services but worth noting if the
  port is publicly reachable. Reflection can be gated behind a flag (e.g., `-enable-reflection`).

---

### 4. gRPC Server Reflection + grpcurl (CLI tooling, no web UI)

Enabling server reflection (same one-liner as in Option 3) also unlocks
[grpcurl](https://github.com/fullstorydev/grpcurl) — a `curl`-like CLI for gRPC — without
any web browser dependency.

```bash
# List available services
grpcurl -plaintext localhost:7398 list

# List methods of the statemachine service
grpcurl -plaintext localhost:7398 list sm.statemachine.v1.StatemachineService

# Call a method
grpcurl -plaintext -d '{"value": ""}' localhost:7398 \
  sm.statemachine.v1.StatemachineService/GetAllConfigurations
```

This is already useful for debugging and scripting, but does not satisfy the "web UI"
requirement. It is best treated as a prerequisite or companion to grpcui rather than a
standalone option.

---

## Recommended Implementation Path

If the decision is made to adopt the grpcui + reflection approach, the implementation is
small and low-risk:

### Step 1: Enable server reflection (this repo)

In `pkg/grpc/grpc_server.go`, add the reflection import and register the reflection service
immediately after `RegisterStatemachineServiceServer`. Add a boolean field (e.g.,
`EnableReflection bool`) to `Config` so that reflection can be disabled in production
deployments via a `-no-reflection` flag.

Estimated diff: ~5 lines in `grpc_server.go`, ~3 lines in `cmd/main.go`.

### Step 2: Add `-reflect` flag to the server binary

In `cmd/main.go`, add:

```go
var enableReflection = flag.Bool("reflect", false,
    "Enable gRPC server reflection (exposes the full API schema; do not use on public-facing ports)")
```

Pass the flag value through `grpc.Config.EnableReflection`.

### Step 3: Document grpcui usage in `docs/`

Add a `docs/grpcui.md` (or a section in `docs/cli.md`) that describes:
- How to install grpcui: `go install github.com/fullstorydev/grpcui/cmd/grpcui@latest`
- How to start the server with reflection: `fsm-server -reflect ...`
- How to launch the UI with and without TLS

### Step 4 (optional, proto repo): Publish to buf.build

In the `statemachine-proto` repository, add a `buf.yaml` and configure its CI to push
releases to the BSR. This provides browsable, version-tagged documentation for external API
consumers.

---

## Risks and Mitigations

| Risk | Mitigation |
|---|---|
| Reflection leaks schema to unauthorized clients on public ports | Gate with `-reflect` flag (off by default); document that it should only be enabled on internal/dev deployments |
| grpcui becomes stale if not updated | It is an external dev tool, not embedded; version constraints are not a concern for the server binary itself |
| buf.build requires a paid plan for private modules | The proto repo already appears to be public (`massenz/statemachine-proto`); the BSR free tier is sufficient |
| Future grpc-gateway adoption is blocked | Reflection and grpcui are fully compatible with grpc-gateway if it is added later; they are not mutually exclusive |

---

## Not Recommended at This Time

- **Embedding Swagger UI / serving static HTML from the gRPC server**: Adds binary bloat and
  a second HTTP listener without significant benefit over grpcui.
- **Hand-written OpenAPI specs**: High maintenance cost, immediately drifts from the proto
  definitions, no automation possible without grpc-gateway.
- **gRPC transcoding via Envoy**: Architecturally correct at scale but introduces an
  infrastructure dependency (Envoy proxy) that is out of proportion for a project of this
  size.
