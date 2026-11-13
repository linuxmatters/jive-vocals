# Managed by Tailor: nix/go.nix
{ pkgs, ... }:
let
  goTools = pkgs.go-tools.override {
    buildGoModule = pkgs.buildGoModule.override { go = pkgs.go; };
  };
  staticcheck =
    if builtins.compareVersions pkgs.go-tools.version "2026.2.2" >= 0 then
      goTools
    else
      goTools.overrideAttrs (_: {
        version = "2026.2.2";
        src = pkgs.fetchFromGitHub {
          owner = "dominikh";
          repo = "go-tools";
          tag = "2026.2.2";
          hash = "sha256-nmcXIQf9Mws4CMQDKQ1NUYHLYkm+ggSnh5sDiuvdnK4=";
        };
        vendorHash = "sha256-YC+DQZ3xjLGzhUXq/KBYhi1PGE63hBFaFor+XUZE1Ng=";
      });
in
with pkgs;
[
  go
  golangci-lint
  govulncheck
  goreleaser
  gocyclo
  goperf
  staticcheck
]
