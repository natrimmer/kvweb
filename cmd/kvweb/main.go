package main

import (
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"
	"syscall"

	"github.com/natrimmer/kvweb/internal/config"
	"github.com/natrimmer/kvweb/internal/server"
	"github.com/natrimmer/kvweb/internal/valkey"
)

var (
	version = "dev"
	commit  = "none"
)

func main() {
	cfg := config.New()
	parseBuildInfo(cfg)

	// CLI flags
	flag.StringVar(&cfg.Host, "host", "localhost", "HTTP server host")
	flag.IntVar(&cfg.Port, "port", 8080, "HTTP server port")
	flag.StringVar(&cfg.ValkeyURL, "url", "localhost:6379", "Valkey/Redis server address or URL (e.g. localhost:6379, redis://user:pass@host:6379/0, rediss://host:6380)")
	flag.StringVar(&cfg.ValkeyPassword, "password", "", "Valkey/Redis password (prefer VALKEY_PASSWORD env var)")
	flag.IntVar(&cfg.ValkeyDB, "db", 0, "Valkey/Redis database number")
	flag.BoolVar(&cfg.OpenBrowser, "open", false, "Open browser on start")
	flag.BoolVar(&cfg.ReadOnly, "readonly", false, "Disable write operations (set, delete, flush)")
	flag.StringVar(&cfg.Prefix, "prefix", "", "Only show/allow keys matching this prefix")
	flag.BoolVar(&cfg.DisableFlush, "disable-flush", true, "Block FLUSHDB even in write mode (use --disable-flush=false to allow)")
	flag.Int64Var(&cfg.MaxKeys, "max-keys", 0, "Limit SCAN count per request (0 = no limit)")
	flag.BoolVar(&cfg.Notifications, "notifications", false, "Auto-enable Valkey keyspace notifications for live updates")
	flag.StringVar(&cfg.CORSOrigin, "cors-origin", "", "Allowed CORS origin (e.g. http://localhost:5173). Omit to disallow cross-origin requests")
	flag.BoolVar(&cfg.Dev, "dev", false, "Development mode (skip serving embedded frontend)")
	showVersion := flag.Bool("version", false, "Show version")
	help := flag.Bool("help", false, "Show help")
	flag.Parse()

	// Prefer env var for password to avoid process list exposure
	if cfg.ValkeyPassword == "" {
		cfg.ValkeyPassword = os.Getenv("VALKEY_PASSWORD")
	}

	if *showVersion {
		fmt.Printf("kvweb %s (%s)\n", version, commit)
		os.Exit(0)
	}

	if *help {
		flag.Usage()
		os.Exit(0)
	}

	// Initialize Valkey client
	client, err := valkey.New(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to Valkey: %v", err)
	}
	defer client.Close()

	// Create and start server
	srv := server.New(cfg, client)

	// Open browser if requested
	if cfg.OpenBrowser {
		url := fmt.Sprintf("http://%s:%d", cfg.Host, cfg.Port)
		go func() {
			if err := openBrowser(url); err != nil {
				log.Printf("Failed to open browser: %v", err)
			}
		}()
	}

	// Graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-stop
		log.Println("Shutting down...")
		if err := srv.Shutdown(); err != nil {
			log.Fatalf("Error during shutdown: %v", err)
		}
	}()

	log.Printf("Connected to Valkey at %s", redactURL(cfg.ValkeyURL))
	if cfg.Host == "0.0.0.0" || cfg.Host == "" {
		log.Printf("WARNING: Binding to all interfaces — server will be accessible on your network")
	}
	log.Printf("kvweb running at http://%s:%d", cfg.Host, cfg.Port)
	if err := srv.Start(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

// parseBuildInfo extracts version, commit, and dirty state from the build-time
// variables. version may be a plain semver ("0.1.2" from goreleaser) or a full
// git describe output ("v0.1.2-2-g914ab42-dirty" from local builds).
// When no ldflags are set (dev mode), version stays "dev" with no commit/dirty.
func parseBuildInfo(cfg *config.Config) {
	if version == "dev" {
		cfg.Version = "dev"
		return
	}

	v := strings.TrimPrefix(version, "v")

	// Strip "-dirty" suffix
	if strings.HasSuffix(v, "-dirty") {
		cfg.Dirty = true
		v = strings.TrimSuffix(v, "-dirty")
	}

	// git describe format: "0.1.2-2-g914ab42" → version "0.1.2", commit "914ab42"
	// If commit was set separately (goreleaser), v is already clean like "0.1.2"
	if parts := strings.SplitN(v, "-", 2); len(parts) == 2 && strings.Contains(parts[1], "g") {
		cfg.Version = parts[0]
		hashParts := strings.Split(parts[1], "-")
		for _, p := range hashParts {
			if strings.HasPrefix(p, "g") {
				cfg.Commit = p[1:]
			}
		}
	} else {
		cfg.Version = v
	}

	// Use separately-set commit if version didn't embed one (goreleaser sets it)
	if cfg.Commit == "" && commit != "none" {
		cfg.Commit = commit
	}
}

func openBrowser(url string) error {
	var cmd string
	var args []string

	switch os := runtime.GOOS; os {
	case "darwin":
		cmd = "open"
		args = []string{url}
	case "windows":
		cmd = "rundll32"
		args = []string{"url.dll,FileProtocolHandler", url}
	default: // Linux and others
		cmd = "xdg-open"
		args = []string{url}
	}

	return exec.Command(cmd, args...).Start()
}

// redactURL strips the password from a URL for safe logging.
// Plain host:port strings are returned as-is.
func redactURL(raw string) string {
	if !strings.Contains(raw, "://") {
		return raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	if u.User != nil {
		if _, hasPass := u.User.Password(); hasPass {
			u.User = url.UserPassword(u.User.Username(), "xxxxx")
		}
	}
	return u.String()
}
