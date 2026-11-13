{ pkgs, self, ... }:
let
  version = "0.0.0-${self.sourceInfo.shortRev or (self.sourceInfo.dirtyShortRev or "dirty")}";

  # github: flakes omit submodules, so fetch ffmpeg-statigo at the commit that
  # third_party/ffmpeg-statigo pins, and the static library that its
  # lib.Version selects.
  ffmpeg-statigo = pkgs.fetchFromGitHub {
    owner = "linuxmatters";
    repo = "ffmpeg-statigo";
    rev = "47ba8e8d26bcace3020c2d58ea684ff6677798a8";
    hash = "sha256-AvjIUkfJk/4i65265och/j4x6y9ss0taULjwT7Z5gsA=";
  };
  libraries = {
    x86_64-linux = {
      target = "linux-amd64";
      hash = "sha256-R/CFjAAq4JWgFhcnauQMcZK8fQa/IDmzfXUE39vE6b0=";
    };
    aarch64-linux = {
      target = "linux-arm64";
      hash = "sha256-kTbh4dYxhbZX2/tr8ZWuV6LxMBgSd0OvxGh1ocwogDc=";
    };
  };
  library = libraries.${pkgs.stdenv.hostPlatform.system} or libraries.x86_64-linux;
  libffmpeg = pkgs.runCommand "libffmpeg-8.1.2.1-${library.target}" { } ''
    mkdir -p $out
    tar -xzf ${
      pkgs.fetchurl {
        url = "https://github.com/linuxmatters/ffmpeg-statigo/releases/download/lib-8.1.2.1/ffmpeg-${library.target}.tar.gz";
        inherit (library) hash;
      }
    } -C $out
  '';
in
pkgs.buildGo126Module {
  pname = "jive-vocals";
  inherit version;
  src = self;
  # Build from the module cache, so cgo reads the headers in the replaced
  # ffmpeg-statigo directory.
  proxyVendor = true;
  vendorHash = "sha256-nixsPY/TTC5r9C2obZOihwPap5U3Ws3dax6izmGHz4g=";
  subPackages = [ "cmd/jive-vocals" ];
  # Link libffmpeg.a from the store, because the sandbox cannot download it.
  env = {
    CGO_ENABLED = 1;
    CGO_LDFLAGS = "-L${libffmpeg}/${builtins.replaceStrings [ "-" ] [ "_" ] library.target}";
  };
  postPatch = ''
    mkdir -p third_party
    rm -rf third_party/ffmpeg-statigo
    cp -r ${ffmpeg-statigo} third_party/ffmpeg-statigo
    chmod -R u+w third_party/ffmpeg-statigo
  '';
  ldflags = [
    "-s"
    "-w"
    "-X main.version=${version}"
  ];
  meta = {
    description = "Raw microphone recordings into broadcast-ready audio in one command";
    homepage = "https://linuxmatters.sh/jive/vocals/";
    license = pkgs.lib.licenses.gpl3Only;
    mainProgram = "jive-vocals";
    platforms = builtins.attrNames libraries;
  };
}
