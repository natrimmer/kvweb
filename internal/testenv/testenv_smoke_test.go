package testenv_test

import (
	"testing"

	"github.com/natrimmer/kvweb/internal/testenv"
)

func TestMain(m *testing.M) { testenv.Main(m) }

// TestHarnessBoots is the harness's own smoke test: every discovered engine
// launches, answers over the client, and serves the API.
func TestHarnessBoots(t *testing.T) {
	testenv.Run(t, func(t *testing.T, e *testenv.Engine) {
		h := testenv.New(t, e)

		if err := h.Client.Ping(h.Ctx()); err != nil {
			t.Fatalf("PING: %v", err)
		}

		h.SeedString("smoke", "ok")
		if got := h.GetString("smoke"); got != "ok" {
			t.Errorf("value = %q, want %q", got, "ok")
		}

		h.Get("/api/health").ExpectOK()

		if e.Version == "" {
			t.Error("engine version was not detected")
		}
		t.Logf("%s %s on %s (db %d)", e.Name, e.Version, h.Addr, h.DB)
	})
}

// TestDatabaseIsolation proves two harnesses on the same shared server cannot
// see each other's keys, which is what makes parallel tests safe.
func TestDatabaseIsolation(t *testing.T) {
	testenv.Run(t, func(t *testing.T, e *testenv.Engine) {
		a := testenv.New(t, e)
		b := testenv.New(t, e)

		if a.DB == b.DB {
			t.Fatalf("both harnesses got database %d", a.DB)
		}

		a.SeedString("only-in-a", "1")
		if keys := b.AllKeys(); len(keys) != 0 {
			t.Errorf("second harness sees %v, want an empty database", keys)
		}
	})
}

// TestExclusiveServer checks that an exclusive harness gets its own process, so
// server-global changes cannot leak into the shared server.
func TestExclusiveServer(t *testing.T) {
	testenv.Run(t, func(t *testing.T, e *testenv.Engine) {
		shared := testenv.New(t, e)
		exclusive := testenv.New(t, e, testenv.Exclusive())

		if shared.Addr == exclusive.Addr {
			t.Fatalf("exclusive harness reused the shared server at %s", shared.Addr)
		}
	})
}
