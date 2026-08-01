// Package testenv discovers and launches real Redis and Valkey servers for
// integration tests, then wires them to kvweb's config, client and API handler.
//
// Servers are found on PATH (or via the env vars below) and launched on an
// ephemeral port with a scratch directory, persistence off and keyspace
// notifications off. Each engine gets one shared server per `go test` process;
// tests that mutate server-global state (notify-keyspace-events, slowlog
// config) ask for an exclusive one instead.
//
// Environment:
//
//	KVWEB_TEST_ENGINES          comma-separated subset to run, e.g. "valkey"
//	KVWEB_TEST_REQUIRE_ENGINES  fail instead of skip when an engine is missing
//	KVWEB_TEST_VALKEY_SERVER    path to valkey-server
//	KVWEB_TEST_REDIS_SERVER     path to redis-server
//	KVWEB_TEST_VALKEY_URL       host:port of an already-running Valkey
//	KVWEB_TEST_REDIS_URL        host:port of an already-running Redis
//
// Pointing at an already-running server (the _URL vars) covers Docker and CI
// service containers. Tests that need an exclusive server skip in that mode,
// because there is no safe way to isolate server-global state.
package testenv

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// dbCount is the number of logical databases each launched server exposes.
// It also bounds how many harnesses can run in parallel against one engine.
const dbCount = 16

// Engine is a discovered server implementation.
type Engine struct {
	Name    string // "valkey" or "redis"
	Version string // e.g. "9.1.0"
	Major   int
	Minor   int
	Patch   int

	binary   string // absolute path to the server binary; empty when external
	external string // host:port of an externally managed server, if any

	dbs chan int // pool of free logical databases on the shared server

	once   sync.Once
	shared *instance
	err    error
}

// IsRedis reports whether the engine is Redis.
func (e *Engine) IsRedis() bool { return e.Name == "redis" }

// IsValkey reports whether the engine is Valkey.
func (e *Engine) IsValkey() bool { return e.Name == "valkey" }

// External reports whether the engine is an already-running server that this
// process does not manage.
func (e *Engine) External() bool { return e.external != "" }

// AtLeast reports whether the engine version is at least major.minor.
func (e *Engine) AtLeast(major, minor int) bool {
	if e.Major != major {
		return e.Major > major
	}
	return e.Minor >= minor
}

func (e *Engine) String() string { return e.Name + " " + e.Version }

// discovery state, computed once per process.
var (
	discoverOnce sync.Once
	discovered   []*Engine
	unavailable  = map[string]string{} // engine name -> why it could not be used
	excluded     = map[string]bool{}   // engines deselected via KVWEB_TEST_ENGINES
)

type spec struct {
	name    string
	product string // first word of the binary's --version output
	binEnv  string
	urlEnv  string
	binName string
}

var specs = []spec{
	{"valkey", "Valkey", "KVWEB_TEST_VALKEY_SERVER", "KVWEB_TEST_VALKEY_URL", "valkey-server"},
	{"redis", "Redis", "KVWEB_TEST_REDIS_SERVER", "KVWEB_TEST_REDIS_URL", "redis-server"},
}

// versionRE matches the "v=9.1.0" field in a server's --version banner.
var versionRE = regexp.MustCompile(`v=(\d+)\.(\d+)\.(\d+)`)

func discover() {
	discoverOnce.Do(func() {
		want := requestedEngines()
		for _, s := range specs {
			// A deselected engine is a choice, not a missing dependency, so it
			// does not count against KVWEB_TEST_REQUIRE_ENGINES.
			if len(want) > 0 && !want[s.name] {
				excluded[s.name] = true
				continue
			}
			e, why := discoverEngine(s)
			if e == nil {
				unavailable[s.name] = why
				continue
			}
			e.dbs = make(chan int, dbCount)
			for i := range dbCount {
				e.dbs <- i
			}
			discovered = append(discovered, e)
		}
	})
}

// requestedEngines parses KVWEB_TEST_ENGINES. An empty result means "all".
func requestedEngines() map[string]bool {
	raw := strings.TrimSpace(os.Getenv("KVWEB_TEST_ENGINES"))
	if raw == "" {
		return nil
	}
	want := map[string]bool{}
	for _, name := range strings.Split(raw, ",") {
		if name = strings.ToLower(strings.TrimSpace(name)); name != "" {
			want[name] = true
		}
	}
	return want
}

func discoverEngine(s spec) (*Engine, string) {
	if addr := strings.TrimSpace(os.Getenv(s.urlEnv)); addr != "" {
		e := &Engine{Name: s.name, external: addr}
		if err := e.readExternalVersion(); err != nil {
			return nil, fmt.Sprintf("%s=%s is not reachable: %v", s.urlEnv, addr, err)
		}
		return e, ""
	}

	var candidates []string
	if p := strings.TrimSpace(os.Getenv(s.binEnv)); p != "" {
		candidates = []string{p}
	} else {
		candidates = lookPathAll(s.binName)
	}

	if len(candidates) == 0 {
		return nil, fmt.Sprintf("no %s on PATH (set %s or %s)", s.binName, s.binEnv, s.urlEnv)
	}

	// Valkey ships a redis-server alias, so identity is decided by the banner,
	// not the filename. Take the first candidate that actually is this product.
	var reasons []string
	for _, path := range candidates {
		product, major, minor, patch, err := probeBinary(path)
		if err != nil {
			reasons = append(reasons, fmt.Sprintf("%s: %v", path, err))
			continue
		}
		if product != s.product {
			reasons = append(reasons, fmt.Sprintf("%s is %s, not %s", path, product, s.product))
			continue
		}
		return &Engine{
			Name:    s.name,
			Version: fmt.Sprintf("%d.%d.%d", major, minor, patch),
			Major:   major,
			Minor:   minor,
			Patch:   patch,
			binary:  path,
		}, ""
	}
	return nil, fmt.Sprintf("no usable %s binary (%s)", s.product, strings.Join(reasons, "; "))
}

// lookPathAll returns every executable named name across PATH, in PATH order.
// exec.LookPath only returns the first, which is not enough when Valkey's
// redis-server alias shadows a real Redis install.
func lookPathAll(name string) []string {
	var found []string
	seen := map[string]bool{}
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		if dir == "" {
			dir = "."
		}
		path := filepath.Join(dir, name)
		info, err := os.Stat(path)
		if err != nil || info.IsDir() || info.Mode()&0o111 == 0 {
			continue
		}
		resolved, err := filepath.EvalSymlinks(path)
		if err != nil {
			resolved = path
		}
		if seen[resolved] {
			continue
		}
		seen[resolved] = true
		found = append(found, path)
	}
	return found
}

// probeBinary runs `<path> --version` and parses the product and version out of
// banners like "Valkey server v=9.1.0 sha=... bits=64 build=...".
func probeBinary(path string) (product string, major, minor, patch int, err error) {
	out, err := runVersion(path)
	if err != nil {
		return "", 0, 0, 0, err
	}
	fields := strings.Fields(out)
	if len(fields) == 0 {
		return "", 0, 0, 0, fmt.Errorf("empty --version output")
	}
	m := versionRE.FindStringSubmatch(out)
	if m == nil {
		return "", 0, 0, 0, fmt.Errorf("could not parse version from %q", strings.TrimSpace(out))
	}
	major, _ = strconv.Atoi(m[1])
	minor, _ = strconv.Atoi(m[2])
	patch, _ = strconv.Atoi(m[3])
	return fields[0], major, minor, patch, nil
}

// Engines returns every engine available to this test run.
//
// It skips the calling test when no engine is available, unless
// KVWEB_TEST_REQUIRE_ENGINES is set, in which case a missing engine is a
// failure. That flag is how CI asserts the matrix really ran.
func Engines(t *testing.T) []*Engine {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping integration test in -short mode")
	}
	discover()

	if requireEngines() {
		var missing []string
		for name, why := range unavailable {
			missing = append(missing, fmt.Sprintf("%s (%s)", name, why))
		}
		if len(missing) > 0 {
			sort.Strings(missing)
			t.Fatalf("KVWEB_TEST_REQUIRE_ENGINES is set but these engines are unavailable: %s",
				strings.Join(missing, ", "))
		}
	}

	if len(discovered) == 0 {
		t.Skipf("no Redis or Valkey server available: %s", describeUnavailable())
	}
	return discovered
}

func describeUnavailable() string {
	parts := make([]string, 0, len(unavailable)+len(excluded))
	for name, why := range unavailable {
		parts = append(parts, name+": "+why)
	}
	for name := range excluded {
		parts = append(parts, name+": excluded by KVWEB_TEST_ENGINES")
	}
	sort.Strings(parts)
	return strings.Join(parts, "; ")
}

func requireEngines() bool {
	v := strings.TrimSpace(os.Getenv("KVWEB_TEST_REQUIRE_ENGINES"))
	return v != "" && v != "0" && !strings.EqualFold(v, "false")
}

// Run runs fn as a subtest against every available engine.
func Run(t *testing.T, fn func(t *testing.T, e *Engine)) {
	t.Helper()
	for _, e := range Engines(t) {
		t.Run(e.Name, func(t *testing.T) {
			fn(t, e)
		})
	}
}

// Main is the TestMain body for packages that use this harness. It runs the
// tests and then shuts down every shared server this process started.
func Main(m *testing.M) {
	code := m.Run()
	shutdownShared()
	os.Exit(code)
}

func shutdownShared() {
	for _, e := range discovered {
		if e.shared != nil {
			e.shared.stop()
		}
	}
}
