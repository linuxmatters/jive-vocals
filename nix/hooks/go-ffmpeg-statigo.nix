# Managed by Tailor: nix/hooks/go-ffmpeg-statigo.nix
{ pkgs, ... }:
pkgs.lib.optionalString pkgs.stdenv.isLinux ''
  tailor_prepend_path LD_LIBRARY_PATH "${pkgs.lib.optionalString pkgs.stdenv.hostPlatform.isx86_64 "${pkgs.vpl-gpu-rt}/lib:${pkgs.intel-media-driver}/lib:"}${pkgs.vulkan-loader}/lib"
  if tailor_nixos_drivers; then
    ${pkgs.lib.optionalString pkgs.stdenv.hostPlatform.isx86_64 ''
      if [ "''${ONEVPL_SEARCH_PATH+x}" != x ]; then
        export ONEVPL_SEARCH_PATH="${pkgs.vpl-gpu-rt}/lib"
      fi
    ''}
    if [ "''${LIBVA_DRIVERS_PATH+x}" != x ]; then
      ${pkgs.lib.optionalString pkgs.stdenv.hostPlatform.isx86_64 ''
        export LIBVA_DRIVERS_PATH="${pkgs.intel-media-driver}/lib/dri"
      ''}
      for tailor_va_driver in /run/opengl-driver/lib/dri/${
        if pkgs.stdenv.hostPlatform.isx86_64 then "iHD_drv_video.so" else "*_drv_video.so"
      }; do
        if [ -f "$tailor_va_driver" ] && [ -r "$tailor_va_driver" ]; then
          export LIBVA_DRIVERS_PATH=/run/opengl-driver/lib/dri
          break
        fi
      done
      unset tailor_va_driver
    fi
  fi
''
