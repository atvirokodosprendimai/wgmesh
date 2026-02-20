{
  description = "Chimney Docker image built with Nix";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-24.11";
    flake-utils.url = "github:numtide/flake-utils";
    nix2container.url = "github:nlewo/nix2container";
    nix2container.inputs.nixpkgs.follows = "nixpkgs";
  };

  outputs = { self, nixpkgs, flake-utils, nix2container }:
    flake-utils.lib.eachSystem [ "x86_64-linux" "aarch64-linux" ] (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
        n2c = nix2container.packages.${system}.nix2container;

        # The chimney binary is injected at image-build time via --argstr or
        # by placing it at ./chimney relative to this flake. CI cross-compiles
        # chimney for the target arch and passes CHIMNEY_BIN env var.
        chimneyBin = builtins.getEnv "CHIMNEY_BIN";
        chimneyBinPath = if chimneyBin != "" then chimneyBin else "./chimney";

        # Static chimney binary package
        chimneyPkg = pkgs.stdenvNoCC.mkDerivation {
          name = "chimney";
          src = builtins.path { path = /. + chimneyBinPath; name = "chimney-bin"; };
          dontUnpack = true;
          installPhase = ''
            mkdir -p $out/bin
            cp $src $out/bin/chimney
            chmod +x $out/bin/chimney
          '';
        };

      in {
        packages = {
          # Minimal chimney container image
          # Layers:
          #   1. cacert (TLS root CAs — chimney calls GitHub API)
          #   2. chimney binary
          # No shell, no package manager, no debug tools in prod image.
          chimneyImage = n2c.buildImage {
            name = "ghcr.io/atvirokodosprendimai/wgmesh/chimney";
            tag = "latest";

            layers = [
              # Layer 1: TLS certs (changes rarely — good for caching)
              (n2c.buildLayer {
                deps = [ pkgs.cacert ];
              })
              # Layer 2: chimney binary (changes every build)
              (n2c.buildLayer {
                deps = [ chimneyPkg ];
              })
            ];

            config = {
              Entrypoint = [ "${chimneyPkg}/bin/chimney" ];
              Env = [
                "SSL_CERT_FILE=${pkgs.cacert}/etc/ssl/certs/ca-bundle.crt"
                "NIX_SSL_CERT_FILE=${pkgs.cacert}/etc/ssl/certs/ca-bundle.crt"
              ];
              ExposedPorts = { "8080/tcp" = {}; };
              Labels = {
                "org.opencontainers.image.source" = "https://github.com/atvirokodosprendimai/wgmesh";
                "org.opencontainers.image.description" = "chimney GitHub API proxy";
              };
            };
          };
        };

        # `nix run .#push` to push the image (used in CI)
        apps.push = {
          type = "app";
          program = toString (pkgs.writeShellScript "push-chimney" ''
            ${self.packages.${system}.chimneyImage.copyToDockerDaemon}/bin/copy-to-docker-daemon
          '');
        };
      }
    );
}
