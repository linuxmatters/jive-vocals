# Managed by Tailor: nix/go.nix
{ pkgs, ... }:
with pkgs;
[
  go
  golangci-lint
  govulncheck
  goreleaser
  gocyclo
  goperf
]
