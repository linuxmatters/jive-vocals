{
  description = "Professional podcast audio pre-processor";

  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs/nixos-unstable";
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
        basePkgs = import nixpkgs { inherit system; };
        pkgs = basePkgs // { go = basePkgs.go_1_26; };
      in
      {
        devShells.default = pkgs.mkShell {
          shellHook = import ./nix/hooks.nix { inherit pkgs; };
          packages = with pkgs; [
            actionlint
            curl
            ffmpeg
            gnugrep
            gcc
            go_1_26
            gocyclo
            golangci-lint
            ineffassign
            jq
            just
            mediainfo
          ] ++ import ./nix/loader.nix { inherit pkgs; };
        };
      }
    );
}
