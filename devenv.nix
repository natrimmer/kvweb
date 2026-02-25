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
    PORT_VALKEY = toString ports.valkey;
    PORT_BACKEND = toString ports.backend;
    PORT_FRONTEND = toString ports.frontend;
  };

  packages = [
    pkgs.git
    pkgs.gopls
    pkgs.golangci-lint
    pkgs.goreleaser
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
      exec = "go run ./cmd/kvweb --port ${toString ports.backend} --url localhost:${toString ports.valkey} --dev";
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
      VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "dev")
      COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "none")
      go build -ldflags "-X main.version=$VERSION -X main.commit=$COMMIT" -o kvweb ./cmd/kvweb
      echo "Build complete! Run ./kvweb to start"
    '';

    tests.exec = ''
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
      chmod +x ./seed_valkey.sh
      TYPE="''${1:-all}"
      ./seed_valkey.sh "$TYPE"
    '';

    commands.exec = ''
      echo "Available commands:"
      echo ""
      echo "  dev        - Start dev environment (valkey + backend + frontend)"
      echo "  build      - Build production binary with embedded frontend"
      echo "  build-web  - Build frontend only"
      echo "  tests      - Run all tests"
      echo "  lint       - Run linters"
      echo "  deps       - Update dependencies"
      echo "  seed [TYPE]- Populate valkey with sample data"
      echo "               Types: all, string, hash, list, set, zset, geo, stream, hll, ttl"
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
