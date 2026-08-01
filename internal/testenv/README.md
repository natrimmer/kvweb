# Integration test harness

Every test in `internal/api`, `internal/server` and `internal/valkey` runs
against a real Redis **and** a real Valkey server. No mocks, no fakes — the
whole point is to catch the places where the two engines drift apart.

## Running

```bash
test-integration            # both engines, verbose
test-integration valkey     # one engine
test-unit                   # -short: skips everything that needs a server
tests                       # Go tests + Svelte checks (what CI runs)
```

Outside the devenv shell, `go test ./...` works too, as long as
`valkey-server` and `redis-server` are on `PATH`.

Servers start in tens of milliseconds each, so a full run is dominated by one
test that waits on the 5-second stats broadcast rather than by startup cost.

## How servers are provisioned

The harness launches its own servers on ephemeral ports with a scratch
directory, persistence off and keyspace notifications off. It finds them by
scanning `PATH` and running `--version` on each candidate, because Valkey ships
a `redis-server` alias and filename alone cannot tell the two apart. `devenv.nix`
short-circuits that by exporting exact store paths.

| Variable | Effect |
| --- | --- |
| `KVWEB_TEST_ENGINES` | Comma-separated subset, e.g. `valkey` |
| `KVWEB_TEST_REQUIRE_ENGINES` | Fail instead of skip when an engine is missing — CI sets this |
| `KVWEB_TEST_VALKEY_SERVER` / `KVWEB_TEST_REDIS_SERVER` | Exact binary paths |
| `KVWEB_TEST_VALKEY_URL` / `KVWEB_TEST_REDIS_URL` | Use an already-running server instead of launching one |

The `_URL` variables are how the CI compat job talks to Docker service
containers. Tests needing an exclusive server skip in that mode, since a shared
server offers no safe way to isolate server-global state.

## Writing a test

```go
func TestSomething(t *testing.T) {
    testenv.Run(t, func(t *testing.T, e *testenv.Engine) {
        h := testenv.New(t, e)

        h.SeedHash("user:1", map[string]string{"name": "Alice"})

        h.Get(h.KeyPath("user:1")).ExpectOK()
        h.Post(h.KeyPath("user:1")+"/hash", map[string]any{
            "field": "email", "value": "a@example.test",
        }).ExpectOK()
    })
}
```

`testenv.Run` turns the body into one subtest per engine. `testenv.New` gives
you a config, a connected client, and the API handler on an `httptest` server,
with helpers for seeding (`Seed*`), reading back (`GetString`, `HashFields`,
`ScanAll`) and asserting on responses (`ExpectOK`, `ExpectError`, `Decode`).

Options cover the security modes: `ReadOnly()`, `Prefix("app:")`,
`DisableFlush()`, `MaxKeys(n)`, `CORSOrigin(origin)`, and `WithConfig` for
anything else.

`KeyPath` percent-encodes each segment, so keys and members containing slashes,
spaces or unicode round-trip correctly. Always build key URLs with it.

### Shared vs exclusive servers

By default a test gets a **logical database** on the engine's shared server,
flushed before and after. Up to 16 tests per engine run in parallel this way.

Reach for `testenv.Exclusive()` when a test changes anything server-wide —
`notify-keyspace-events`, slowlog config, `SCRIPT FLUSH`, `CONFIG SET`:

```go
h := testenv.New(t, e, testenv.Exclusive("--slowlog-log-slower-than", "0"))
```

Forgetting this makes other tests flaky in ways that only show under load.

### Attributing failures

`Harness.T` is fixed when the harness is built. Inside a `t.Run` subtest, use
`h.With(t)` so a failed assertion is reported against that subtest instead of
its parent.

## What each package covers

- **`internal/valkey`** — client operations per type, Lua scripts and their exact
  error strings, scan pagination, compression round-trips through the wire, and
  a `divergence_test.go` pinning every place kvweb parses a reply by hand
  (INFO fields, `HYLL` header, SLOWLOG field order, notification channel names).
- **`internal/api`** — every HTTP route: types, pagination, error paths,
  read-only and prefix guards driven off a shared table of write routes, the
  console allowlist, and key names that need escaping.
- **`internal/server`** — a real server on a real port with a real WebSocket
  client: initial status and stats frames, live key events, the notifications
  toggle, prefix filtering of broadcasts, and clean shutdown.
