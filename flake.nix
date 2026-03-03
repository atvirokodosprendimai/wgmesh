{
  description = "wgmesh — WireGuard mesh network with decentralized peer discovery";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
      in
      {
        packages.default = pkgs.buildGoModule {
          pname = "wgmesh";
          version = "dev";
          src = ./.;
          vendorHash = "sha256-bFoaLnxaURN0P27d7BV3O17sgqSBvfZCCgDJ3KiERQM=";
          subPackages = [ "." ];
          env.CGO_ENABLED = 0;
          ldflags = [ "-s" "-w" "-X main.version=dev" ];

          meta = with pkgs.lib; {
            description = "WireGuard mesh network — decentralized peer discovery and encrypted networking";
            homepage = "https://github.com/atvirokodosprendimai/wgmesh";
            license = licenses.mit;
            mainProgram = "wgmesh";
          };
        };
      }
    ) // {
      nixosModules.default = { config, lib, pkgs, ... }:
        let
          cfg = config.services.wgmesh;
        in
        {
          options.services.wgmesh = {
            enable = lib.mkEnableOption "wgmesh mesh network";

            secretFile = lib.mkOption {
              type = lib.types.path;
              description = "Path to file containing the mesh secret";
              example = "/etc/wgmesh/secret";
            };

            extraArgs = lib.mkOption {
              type = lib.types.listOf lib.types.str;
              default = [ ];
              description = "Extra arguments to pass to wgmesh join";
              example = [ "--advertise-routes" "10.0.0.0/24" "--gossip" ];
            };
          };

          config = lib.mkIf cfg.enable {
            systemd.services.wgmesh = {
              description = "WireGuard Mesh Network (wgmesh)";
              after = [ "network-online.target" ];
              wants = [ "network-online.target" ];
              wantedBy = [ "multi-user.target" ];

              script = ''
                set -eu
                export WGMESH_SECRET_FILE=${lib.escapeShellArg cfg.secretFile}
                exec ${self.packages.${pkgs.system}.default}/bin/wgmesh join ${lib.escapeShellArgs cfg.extraArgs}
              '';

              serviceConfig = {
                Type = "simple";
                Restart = "always";
                RestartSec = 5;
                LimitNOFILE = 65535;
                NoNewPrivileges = true;
                ProtectSystem = "full";
                ProtectHome = true;
                ReadWritePaths = [ "/var/lib/wgmesh" ];
              };
            };
          };
        };
    };
}
