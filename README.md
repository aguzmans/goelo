# goelo

`goelo` is a storage-free rating engine for applications that need to rate sources, authors,
insiders, managers, or other entities. It provides ELO (compatible with `ai-post-to-wp`'s current
severity K factors and clamps) and Glicko-2. Each caller owns identity, outcomes, opponent selection,
timestamps, and persistence.

## Build and run

```sh
go test ./...
go build -o bin/credrate ./cmd/credrate
./bin/credrate rate examples/ai-post-to-wp/request.json
```

After the first tagged release, install the CLI with `go install github.com/aguzmans/goelo/cmd/credrate@latest`.

The CLI reads one JSON request from a file or standard input and writes one JSON result to standard
output. Errors go to standard error and return a non-zero exit status.

```sh
credrate rate request.json
cat request.json | credrate rate
credrate version
```

## Request contract

ELO's outcomes are processed in array order. Each one includes `opponent_rating`, `correct`, and
`severity` (`critical`, `high`, `medium`, or `low`). ELO retains the existing behavior: K values
64/48/32/16, half-away-from-zero rounding, and an 800–2400 clamp. The returned `change` is the sum
of the rounded pre-clamp deltas, matching the existing helper even when the final rating hits a clamp.

Glicko-2's `outcomes` are one rating period and each includes opponent rating, opponent RD, and score
from 0 to 1. `periods_elapsed` counts complete idle periods before this batch. RD aging happens before
the batch using the prior volatility. With an empty batch, only explicit aging is applied: zero means
no change; one means one idle period. The core never reads a clock.

For either algorithm, omit `current` to use defaults or pass a full state. A seeded entity should pass
its caller-chosen initial state. ELO and Glicko-2 states cannot be switched in place. The core has no
database, network, ranking, seed list, identity normalization, or domain-specific tier labels.

## Consumers

### `ai-post-to-wp` (Go + MySQL)

Use `core.Rate` in process. MySQL remains the source of current state, average cited-source opponent,
seed selection, `audit_floor`, history, and audit attribution. See [Go example](examples/ai-post-to-wp/main.go).

The CLI equivalent is in [examples/ai-post-to-wp/request.json](examples/ai-post-to-wp/request.json):

```sh
credrate rate examples/ai-post-to-wp/request.json
```

### `long-stocks-advisor` (Python + Redis)

The example uses the CLI as a process boundary. The caller resolves prediction outcomes, chooses a
stable manager/entity key, uses the fixed 1500 opponent for v1, and persists the returned state to
Redis. See [Python example](examples/long-stocks-advisor/rate_entity.py) and its
[request JSON](examples/long-stocks-advisor/request.json).

```sh
credrate rate examples/long-stocks-advisor/request.json
python examples/long-stocks-advisor/rate_entity.py manager:buffett
```

The Python example expects the project's `redis` package and a reachable Redis instance (default
`REDIS_URL=redis://localhost:6379/0`). It demonstrates one winning resolved signal; production code
passes `score=0.0` for a loss and can provide idle periods since that entity's last update.

The stock example ships two adapters against the same request contract: a CLI-based one
([rate_entity.py](examples/long-stocks-advisor/rate_entity.py), a process per rating) and an
HTTP-based one ([rate_entity_http.py](examples/long-stocks-advisor/rate_entity_http.py), which posts
to a long-running `credserve`). Use the CLI when call volume is low; use HTTP when a long-lived
service is cheaper than spawning a process per rating. A Python-native implementation can instead
target the same vectors. Whichever boundary you pick, the JSON contract is identical.

## HTTP service (`credserve`)

`credserve` exposes the same stateless core over HTTP for callers that cannot import the Go package
(e.g. Python). It holds no state — each request is independent — and binds to **localhost only** by
default; put it behind your own auth/ingress before exposing it.

```sh
go build -o bin/credserve ./cmd/credserve
./bin/credserve                 # listens on 127.0.0.1:8080
./bin/credserve -addr :9090     # override the bind address
```

After the first tagged release: `go install github.com/aguzmans/goelo/cmd/credserve@latest`.

- `POST /rate` — body is exactly one `core.Request` JSON (same as the CLI); response is one
  `core.Result` JSON. Unknown fields and trailing data are rejected. A validation or
  algorithm-switch error returns `400`; a well-formed request the math cannot resolve returns `422`;
  the body on error is `{"error": "..."}`.
- `GET /healthz` — `{"status":"ok","contract_version":"1","version":"<build>"}`.
- Every response carries an `X-Goelo-Contract-Version` header so clients can pin compatibility.

```sh
curl -sS -X POST http://127.0.0.1:8080/rate \
  -H 'Content-Type: application/json' \
  --data @examples/ai-post-to-wp/request.json
```

## Conformance and versioning

The [ELO](core/vectors/elo_vectors.json) and [Glicko-2](core/vectors/glicko2_vectors.json) vectors
are executable contracts. ELO cases have zero tolerance; Glicko-2 checks the published example,
intermediate calculations, defaults, inactivity, and age-before-rating behavior. Run `go test ./...`
before changing the math.

Releases follow Go module semantic versioning: tags are `vMAJOR.MINOR.PATCH`. Conventional commits
drive the release-please version bump (`fix:` patch, `feat:` minor, breaking change major). The
GitHub workflow validates merges to `main`; release-please opens a version PR and, when that PR is
merged, creates the version tag/release and GoReleaser attaches platform CLI binaries and checksums.
Consumers should pin a released module version, for example:

```sh
go get github.com/aguzmans/goelo@v1.0.0
go install github.com/aguzmans/goelo/cmd/credrate@v1.0.0
```

Until the first published release, `ai-post-to-wp` can use a local `replace` directive for development.
