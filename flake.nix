{
  description = "Professional podcast audio pre-processor";

  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs/nixos-unstable";
    tailor.url = "github:wimpysworld/tailor/latest";
  };

  outputs =
    {
      self,
      nixpkgs,
      tailor,
      ...
    }:
    let
      supportedSystems = [
        "x86_64-linux"
        "aarch64-darwin"
        "aarch64-linux"
      ];
      forAllSystems = nixpkgs.lib.genAttrs supportedSystems;
    in
    {
      packages = forAllSystems (
        system:
        let
          pkgs = import nixpkgs { inherit system; };
          package = import ./nix/package.nix { inherit pkgs self; };
        in
        # Expose the project package under its pname and as the default package.
        pkgs.lib.optionalAttrs
          (
            builtins.pathExists ./nix/package.nix && pkgs.lib.meta.availableOn pkgs.stdenv.hostPlatform package
          )
          {
            ${package.pname} = package;
            default = package;
          }
      );
      devShells = forAllSystems (
        system:
        let
          basePkgs = import nixpkgs { inherit system; };
          pkgs = basePkgs // {
            go = basePkgs.go_1_26;
          };
          # Keep the development shell available when Tailor has no package for this system.
          tailorPkgs = tailor.packages.${system} or { };
        in
        {
          default = pkgs.mkShell {
            shellHook = import ./nix/hooks.nix { inherit pkgs; };
            packages =
              with pkgs;
              [
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
              ]
              ++ (if tailorPkgs ? tailor then [ tailorPkgs.tailor ] else [ ])
              ++ import ./nix/loader.nix { inherit pkgs; };
          };
        }
      );
    };
}
