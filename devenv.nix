{ pkgs, ... }:

{
  # https://devenv.sh/basics/
  env.GREET = "kvweb";

  # https://devenv.sh/packages/
  packages = [
    pkgs.git
    pkgs.gopls
    pkgs.golangci-lint
    pkgs.nodejs
    pkgs.pnpm
    pkgs.nixd
    pkgs.nil
  ];

  # https://devenv.sh/languages/
  languages = {
    go.enable = true;
    typescript.enable = true;
  };

  scripts.hello.exec = ''
    echo hello from $GREET
  '';

  # https://devenv.sh/basics/
  enterShell = ''
    hello         # Run scripts directly
  '';

  enterTest = ''
    echo "Running tests"
    git --version | grep --color=auto "${pkgs.git.version}"
  '';
}
