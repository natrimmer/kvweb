{ pkgs, ... }:

let
  ports = {
    valkey = 6379;
    backend = 8080;
    frontend = 5173;
  };
in
{
  env = {
    KVWEB_VERSION = "dev";
    PORT_VALKEY = toString ports.valkey;
    PORT_BACKEND = toString ports.backend;
    PORT_FRONTEND = toString ports.frontend;
  };

  packages = [
    pkgs.git
    pkgs.gopls
    pkgs.golangci-lint
    pkgs.nodejs
    pkgs.pnpm
    pkgs.nixd
    pkgs.nil
    pkgs.valkey
  ];

  languages = {
    go.enable = true;
    typescript.enable = true;
  };

  processes = {
    valkey = {
      exec = "valkey-server --port ${toString ports.valkey} --save '' --appendonly no";
      process-compose = {
        log_location = "./logs/valkey.log";
        availability = {
          restart = "on_failure";
          max_restarts = 5;
        };
        readiness_probe = {
          exec = {
            command = "valkey-cli -p ${toString ports.valkey} ping";
          };
          initial_delay_seconds = 1;
          period_seconds = 1;
          timeout_seconds = 2;
          success_threshold = 1;
          failure_threshold = 5;
        };
      };
    };

    backend = {
      exec = "go run ./cmd/kvweb --port ${toString ports.backend} --url localhost:${toString ports.valkey}";
      process-compose = {
        log_location = "./logs/backend.log";
        availability = {
          restart = "on_failure";
          max_restarts = 5;
        };
        depends_on = {
          valkey = {
            condition = "process_healthy";
          };
        };
        readiness_probe = {
          http_get = {
            host = "127.0.0.1";
            port = ports.backend;
            scheme = "http";
            path = "/api/info";
          };
          initial_delay_seconds = 2;
          period_seconds = 1;
          timeout_seconds = 2;
          success_threshold = 1;
          failure_threshold = 10;
        };
      };
    };

    frontend = {
      exec = "cd web && pnpm install --frozen-lockfile 2>/dev/null || pnpm install && pnpm dev";
      process-compose = {
        log_location = "./logs/frontend.log";
        availability = {
          restart = "on_failure";
          max_restarts = 5;
        };
        depends_on = {
          backend = {
            condition = "process_healthy";
          };
        };
      };
    };
  };

  scripts = {
    dev.exec = ''
      echo "Starting development environment..."
      devenv up
    '';

    build-web.exec = ''
      echo "Building web frontend..."
      cd $DEVENV_ROOT/web
      pnpm install --frozen-lockfile 2>/dev/null || pnpm install
      pnpm build
      echo "Copying dist to static/dist..."
      rm -rf $DEVENV_ROOT/static/dist
      cp -r dist $DEVENV_ROOT/static/
      echo "Web build complete!"
    '';

    build.exec = ''
      build-web
      echo "Building kvweb binary..."
      go build -ldflags "-X main.version=$KVWEB_VERSION" -o kvweb ./cmd/kvweb
      echo "Build complete! Run ./kvweb to start"
    '';

    test.exec = ''
      echo "Running Go tests..."
      go test ./...
      echo ""
      echo "Running Svelte checks..."
      cd $DEVENV_ROOT/web
      pnpm check
    '';

    lint.exec = ''
      echo "Linting Go code..."
      golangci-lint run
      echo ""
      echo "Checking Svelte..."
      cd $DEVENV_ROOT/web
      pnpm check
    '';

    deps.exec = ''
      echo "Updating Go dependencies..."
      go mod tidy
      echo ""
      echo "Installing web dependencies..."
      cd $DEVENV_ROOT/web
      pnpm install
    '';

    ports.exec = ''
      echo "Service Ports:"
      echo ""
      echo "  Valkey      localhost:$PORT_VALKEY"
      echo "  Backend     http://localhost:$PORT_BACKEND"
      echo "  Frontend    http://localhost:$PORT_FRONTEND"
    '';

    seed.exec = ''
      echo "Seeding valkey with sample data..."
      CLI="valkey-cli -p $PORT_VALKEY"

      # String examples
      $CLI SET "user:1:name" "Alice Johnson"
      $CLI SET "user:1:email" "alice@example.com"
      $CLI SET "user:2:name" "Bob Smith"
      $CLI SET "user:2:email" "bob@example.com"
      $CLI SET "config:app:version" "1.0.0"
      $CLI SET "config:app:environment" "development"
      $CLI SET "session:abc123" '{"userId":1,"createdAt":"2024-01-15T10:30:00Z"}'

      # Hash examples
      $CLI HSET "user:1" name "Alice Johnson" email "alice@example.com" age 28 role "admin"
      $CLI HSET "user:2" name "Bob Smith" email "bob@example.com" age 34 role "user"
      $CLI HSET "product:1001" name "Widget Pro" price "29.99" stock 150 category "tools"
      $CLI HSET "product:1002" name "Gadget Plus" price "49.99" stock 75 category "electronics"

      # List examples
      $CLI RPUSH "queue:emails" "welcome@example.com" "newsletter@example.com" "alert@example.com"
      $CLI RPUSH "recent:searches" "redis tutorial" "key-value stores" "caching strategies"
      $CLI RPUSH "logs:app" '{"level":"info","msg":"App started"}' '{"level":"warn","msg":"High memory"}' '{"level":"error","msg":"Connection failed"}'

      # Set examples
      $CLI SADD "tags:post:1" "redis" "database" "nosql" "tutorial"
      $CLI SADD "tags:post:2" "golang" "backend" "api"
      $CLI SADD "online:users" "user:1" "user:2" "user:5" "user:8"

      # Sorted set examples
      $CLI ZADD "leaderboard:game1" 1500 "player:alice" 1350 "player:bob" 1200 "player:charlie" 980 "player:diana"
      $CLI ZADD "trending:articles" 342 "article:1001" 256 "article:1002" 189 "article:1003"

      # Stream example
      $CLI XADD "events:user" "*" action "login" userId 1 ip "192.168.1.1"
      $CLI XADD "events:user" "*" action "page_view" userId 1 page "/dashboard"
      $CLI XADD "events:user" "*" action "logout" userId 1

      # Keys with TTL
      $CLI SET "cache:weather:nyc" '{"temp":72,"conditions":"sunny"}' EX 3600
      $CLI SET "cache:weather:la" '{"temp":85,"conditions":"clear"}' EX 3600

      # Nested namespace examples
      $CLI SET "api:v1:users:count" "1542"
      $CLI SET "api:v1:requests:today" "28493"
      $CLI SET "api:v2:users:count" "892"

      echo ""
      echo "Seeded $(valkey-cli -p $PORT_VALKEY DBSIZE | cut -d' ' -f2) keys"
      echo "Run 'valkey-cli -p $PORT_VALKEY KEYS \"*\"' to see all keys"
    '';

    commands.exec = ''
      echo "Available commands:"
      echo ""
      echo "  dev        - Start dev environment (valkey + backend + frontend)"
      echo "  build      - Build production binary with embedded frontend"
      echo "  build-web  - Build frontend only"
      echo "  test       - Run all tests"
      echo "  lint       - Run linters"
      echo "  deps       - Update dependencies"
      echo "  seed       - Populate valkey with sample data"
      echo "  ports      - Show service ports"
      echo "  commands   - Show this help"
    '';
  };

  enterShell = ''
    echo ""
    echo "kvweb development environment"
    echo ""
    commands
    echo ""
    ports
    echo ""
  '';

  git-hooks.hooks = {
    #----------------------------------------
    # Formatting Hooks - Run First
    #----------------------------------------
    beautysh.enable = true; # Format shell files
    nixfmt-rfc-style.enable = true; # Format Nix code
    gofmt.enable = true; # Format Go code
    ts-fmt = {
      enable = true;
      name = "TypeScript Format";
      entry = "${pkgs.bash}/bin/bash -c 'cd web && ${pkgs.pnpm}/bin/pnpm format'";
      files = "\\.(ts|js|svelte)$";
      language = "system";
      pass_filenames = false;
    };

    #----------------------------------------
    # Linting Hooks - Run After Formatting
    #----------------------------------------
    shellcheck.enable = true; # Lint shell scripts
    statix.enable = true; # Lint Nix code
    deadnix.enable = true; # Find unused Nix code
    golangci-lint.enable = true; # Lint Go code
    eslint = {
      enable = true;
      name = "ESLint";
      entry = "${pkgs.bash}/bin/bash -c 'cd web && ${pkgs.pnpm}/bin/pnpm check'";
      files = "\\.(js|ts|svelte)$";
      language = "system";
      pass_filenames = false;
    };

    #----------------------------------------
    # Security & Safety Hooks
    #----------------------------------------
    detect-private-keys.enable = true; # Prevent committing private keys
    check-added-large-files.enable = true; # Prevent committing large files
    check-case-conflicts.enable = true; # Check for case-insensitive conflicts
    check-merge-conflicts.enable = true; # Check for merge conflict markers
    check-executables-have-shebangs.enable = true; # Ensure executables have shebangs
    check-shebang-scripts-are-executable.enable = true; # Ensure scripts with shebangs are executable
  };
}
