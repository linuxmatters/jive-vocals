# Managed by Tailor: nix/go-ffmpeg-statigo.nix
{ pkgs, ... }:
with pkgs;
[
  pkg-config
  git
  curl
  jq
]
++ lib.optionals stdenv.isLinux [
  vulkan-loader
  intel-media-driver
  vpl-gpu-rt
]
