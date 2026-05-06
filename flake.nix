{
  description = "Agent-first CLI for Grafana and Grafana Cloud";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
  inputs.flake-utils.url = "github:numtide/flake-utils";

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs { inherit system; };
        version = "0.13.0";
      in {
        packages = {
          default = pkgs.buildGoModule {
            pname = "grafana-cli";
            inherit version;
            src = ./.;
            vendorHash = "sha256-2tOLpD86san6YKPm/4uu8dioBvA7rhrR7ZmSgn3QmEc=";
            subPackages = [ "cmd/grafana" ];

            ldflags = [ "-s" "-w" ];

            meta = {
              description = "Agent-first CLI for Grafana and Grafana Cloud";
              homepage = "https://github.com/matiasvillaverde/grafana-cli";
              license = {
                spdxId = "FSL-1.1-ALv2";
                fullName = "Functional Source License, Version 1.1, ALv2 Future License";
                free = false;
              };
              mainProgram = "grafana";
            };
          };
        };

        devShells.default = pkgs.mkShell {
          buildInputs = [ self.packages.${system}.default ];
        };
      });
}
