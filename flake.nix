{
  description = "Intent-first autotiling for stacking compositors";

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
          pname = "intentile";
          version = "0.1.0";
          src = ./.;
          vendorHash = null;
          meta = {
            description = "Intent-first autotiling for stacking compositors";
            homepage = "https://github.com/jaycee1285/intentile";
            mainProgram = "intentile";
          };
        };

        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [
            go
            gopls
            gotools
            go-tools
            wtype
          ];

          shellHook = ''
            echo "intentile dev environment"
            echo "Go: $(go version)"
          '';
        };
      }
    );
}
