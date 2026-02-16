package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
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

	// CLI flags
	flag.StringVar(&cfg.Host, "host", "localhost", "HTTP server host")
	flag.IntVar(&cfg.Port, "port", 8080, "HTTP server port")
	flag.StringVar(&cfg.ValkeyURL, "url", "localhost:6379", "Valkey/Redis server URL")
	flag.StringVar(&cfg.ValkeyPassword, "password", "", "Valkey/Redis password")
	flag.IntVar(&cfg.ValkeyDB, "db", 0, "Valkey/Redis database number")
	flag.BoolVar(&cfg.OpenBrowser, "open", false, "Open browser on start")
	flag.BoolVar(&cfg.ReadOnly, "readonly", false, "Disable write operations (set, delete, flush)")
	flag.StringVar(&cfg.Prefix, "prefix", "", "Only show/allow keys matching this prefix")
	flag.BoolVar(&cfg.DisableFlush, "disable-flush", false, "Block FLUSHDB even in write mode")
	flag.Int64Var(&cfg.MaxKeys, "max-keys", 0, "Limit SCAN count per request (0 = no limit)")
	flag.BoolVar(&cfg.Notifications, "notifications", false, "Auto-enable Valkey keyspace notifications for live updates")
	flag.StringVar(&cfg.CORSOrigin, "cors-origin", "", "Allowed CORS origin (e.g. http://localhost:5173). Omit to disallow cross-origin requests")
	flag.BoolVar(&cfg.Dev, "dev", false, "Development mode (skip serving embedded frontend)")
	showVersion := flag.Bool("version", false, "Show version")
	help := flag.Bool("help", false, "Show help")
	flag.Parse()

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

	log.Printf("Connected to Valkey at %s", cfg.ValkeyURL)
	log.Printf("kvweb running at http://%s:%d", cfg.Host, cfg.Port)
	if err := srv.Start(); err != nil {
		log.Fatalf("Server error: %v", err)
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
