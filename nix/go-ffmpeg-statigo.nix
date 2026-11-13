# Managed by Tailor: nix/go-ffmpeg-statigo.nix
{ pkgs, ... }:
with pkgs;
[
  pkg-config
  git
  curl
  jq
]
++ lib.optionals stdenv.isLinux (
  [ vulkan-loader ]
  ++ lib.optionals stdenv.hostPlatform.isx86_64 [
    intel-media-driver
    vpl-gpu-rt
  ]
)
