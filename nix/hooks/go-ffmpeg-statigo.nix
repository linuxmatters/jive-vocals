# Managed by Tailor: nix/hooks/go-ffmpeg-statigo.nix
{ pkgs, ... }:
pkgs.lib.optionalString pkgs.stdenv.isLinux ''
  if tailor_nixos_drivers; then
    tailor_prepend_path LD_LIBRARY_PATH "${pkgs.vpl-gpu-rt}/lib:${pkgs.intel-media-driver}/lib:${pkgs.vulkan-loader}/lib"
    if [ "''${ONEVPL_SEARCH_PATH+x}" != x ]; then
      export ONEVPL_SEARCH_PATH="${pkgs.vpl-gpu-rt}/lib"
    fi
    if [ "''${LIBVA_DRIVERS_PATH+x}" != x ]; then
      if [ -d /run/opengl-driver/lib/dri ]; then
        export LIBVA_DRIVERS_PATH=/run/opengl-driver/lib/dri
      else
        export LIBVA_DRIVERS_PATH="${pkgs.intel-media-driver}/lib/dri"
      fi
    fi
    if [ "''${VK_DRIVER_FILES+x}" != x ] && [ -d /run/opengl-driver/share/vulkan/icd.d ]; then
      tailor_icds=""
      while IFS= read -r tailor_icd; do
        tailor_icds="''${tailor_icds:+$tailor_icds:}$tailor_icd"
      done < <(find -L /run/opengl-driver/share/vulkan/icd.d -type f -name '*.json' -readable 2>/dev/null | LC_ALL=C sort)
      if [ -n "$tailor_icds" ]; then export VK_DRIVER_FILES="$tailor_icds"; fi
      unset tailor_icds tailor_icd
    fi
  fi
''
