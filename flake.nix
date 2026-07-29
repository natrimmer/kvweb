{
  description = "kvweb — web-based GUI for browsing Valkey/Redis databases";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";

  outputs =
    { self, nixpkgs }:
    let
      systems = [
        "x86_64-linux"
        "aarch64-linux"
        "x86_64-darwin"
        "aarch64-darwin"
      ];
      forAllSystems = nixpkgs.lib.genAttrs systems;
    in
    {
      packages = forAllSystems (
        system:
        let
          pkgs = nixpkgs.legacyPackages.${system};

          version = self.shortRev or "dirty";

          kvweb-ui = pkgs.stdenvNoCC.mkDerivation {
            pname = "kvweb-ui";
            inherit version;
            src = ./web;

            nativeBuildInputs = [
              pkgs.nodejs
              pkgs.pnpm_10
              pkgs.pnpmConfigHook
            ];

            pnpmDeps = pkgs.fetchPnpmDeps {
              pname = "kvweb-ui";
              inherit version;
              src = ./web;
              pnpm = pkgs.pnpm_10;
              fetcherVersion = 3;
              hash = "sha256-HnEIzvrn8ofqCH+rRA8lnXY6lS5vmEnpu6KBMvzojtU=";
            };

            buildPhase = ''
              runHook preBuild
              pnpm build
              runHook postBuild
            '';

            installPhase = ''
              runHook preInstall
              cp -r dist $out
              runHook postInstall
            '';
          };

          kvweb = pkgs.buildGoModule {
            pname = "kvweb";
            inherit version;
            src = ./.;

            vendorHash = "sha256-i1ZxT/pvJMnRSccLb53mlQNdyvzrVqKTKndfzUKqrHU=";

            env.CGO_ENABLED = 0;

            subPackages = [ "cmd/kvweb" ];

            ldflags = [
              "-X main.version=${version}"
              "-X main.commit=${version}"
            ];

            preBuild = ''
              rm -rf static/dist
              cp -r ${kvweb-ui} static/dist
            '';

            meta = {
              description = "Web-based GUI for browsing Valkey/Redis databases";
              homepage = "https://github.com/natrimmer/kvweb";
              mainProgram = "kvweb";
            };
          };
        in
        {
          inherit kvweb;
          default = kvweb;
        }
      );
    };
}
