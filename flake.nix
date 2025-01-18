{
  description = "Go implementation of Kaitai Struct.";

  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs";
    flake-utils.url = "github:numtide/flake-utils";
    kaitai_struct_tests = {
      url = "github:kaitai-io/kaitai_struct_tests";
      flake = false;
    };
  };

  outputs =
    {
      self,
      nixpkgs,
      flake-utils,
      kaitai_struct_tests,
    }:
    flake-utils.lib.eachDefaultSystem (
      system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
        zanbato = pkgs.buildGoModule {
          name = "zanbato";
          src = self;
          postPatch = ''
            mkdir -p internal/third_party
            ln -s ${kaitai_struct_tests} internal/third_party/kaitai_struct_tests
            find .
          '';
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
        checks = {
          format = pkgs.runCommandLocal "check-format" { } ''
            cd ${self}
            ${pkgs.lib.getExe format} --check
            touch $out
          '';
          inherit zanbato;
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
