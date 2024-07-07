{
  description = "Go implementation of Kaitai Struct.";

  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs =
    {
      self,
      nixpkgs,
      flake-utils,
    }:
    flake-utils.lib.eachDefaultSystem (
      system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
        zanbato = {
          default = pkgs.buildGoModule {
            name = "zanbato";
            src = self;
            vendorHash = pkgs.lib.fakeHash;
          };
        };
      in
      {
        packages = {
          inherit zanbato;
          default = zanbato;
        };
        devShell = pkgs.mkShell {
          inputsFrom = [ zanbato ];
          nativeBuildInputs = [
            pkgs.go
            pkgs.gopls
          ];
        };
      }
    );
}
