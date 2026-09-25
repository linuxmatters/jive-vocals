# Managed by Tailor: nix/hooks/core.nix
{ pkgs, ... }:
pkgs.lib.optionalString pkgs.stdenv.isLinux ''
  if tailor_nixos_drivers && [ -d /run/opengl-driver/lib ]; then
    tailor_prepend_path LD_LIBRARY_PATH /run/opengl-driver/lib
  fi
''
