# kvweb

A web-based GUI for browsing and editing Valkey/Redis databases. Inspired by [pgweb](https://github.com/sosedoff/pgweb).

Single binary. Go backend with embedded Svelte frontend.

## Install

```
go install github.com/natrimmer/kvweb/cmd/kvweb@latest
```

Or build from source:

```
git clone https://github.com/natrimmer/kvweb
cd kvweb
build
```

## Usage

```
kvweb [flags]
```

| Flag | Default | Description |
|------|---------|-------------|
| `-url` | `localhost:6379` | Valkey/Redis server address |
| `-password` | | Server password |
| `-db` | `0` | Database number |
| `-host` | `localhost` | HTTP listen address |
| `-port` | `8080` | HTTP listen port |
| `-readonly` | `false` | Disable write operations |
| `-prefix` | | Only show keys matching this prefix |
| `-disable-flush` | `false` | Block FLUSHDB even in write mode |
| `-max-keys` | `0` | Limit SCAN count per request (0 = no limit) |
| `-notifications` | `false` | Auto-enable keyspace notifications for live updates |
| `-open` | `false` | Open browser on start |

## Supported Types

string, hash, list, set, sorted set, stream, HyperLogLog, geo
