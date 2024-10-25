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
        zanbato = pkgs.buildGoModule {
          name = "zanbato";
          src = self;
          vendorHash = "sha256-HQMgcOmzI725cmQhDJKvxL00J0EMuBLa9Ji35LnohrY=";
        };
        format = pkgs.writeShellApplication {
          name = "format";

          runtimeInputs = [
            pkgs.nixfmt-rfc-style
            pkgs.yamlfmt
            pkgs.go
          ];

          text = ''
            if [[ $# -ne 1 || "$1" == "--help" ]]; then
              >&2 echo "Usage: $0 --check | --write"
              exit 0
            fi

            NIXFMT_ARGS=()
            YAMLFMT_ARGS=()

            case $1 in
              -w|--write)
                NIXFMT_ARGS+=("--verify")
                GOFMT_COMMAND="gofmt -w ."
                shift
                ;;
              -c|--check)
                NIXFMT_ARGS+=("--check")
                YAMLFMT_ARGS+=("-dry" "-lint")
                GOFMT_COMMAND="diff <(echo -n) <(gofmt -d .)"
                shift
                ;;
              *)
                >&2 echo "Unknown option $1"
                exit 1
                ;;
            esac

            >&2 echo "Running nixfmt."
            find . -not -path '*/.*' -not -path 'build' -iname '*.nix' -print0 | \
              xargs -0 nixfmt "''${NIXFMT_ARGS[@]}"

            >&2 echo "Running yamlfmt."
            yamlfmt "''${YAMLFMT_ARGS[@]}" '**.yml' .clang-format .clang-tidy

            >&2 echo "Running gofmt."
            bash -c "''${GOFMT_COMMAND}"
          '';
        };
      in
      {
        packages = {
          inherit zanbato format;
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
