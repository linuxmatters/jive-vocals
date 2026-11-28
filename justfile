# Jive Vocals - Just Commands

set allow-duplicate-recipes
import 'just/loader.just'

# Import presenter integration tests if testdata/justfile exists (not in git)
import? 'testdata/justfile'

# List commands
[default]
default:
    @just --list

# Check ffmpeg-statigo submodule is present
_check-submodule:
    #!/usr/bin/env bash
    if [ ! -f "third_party/ffmpeg-statigo/go.mod" ]; then
        echo "Error: ffmpeg-statigo submodule not initialised. Run 'just setup' first."
        exit 1
    fi
    if [ ! -f "third_party/ffmpeg-statigo/lib/$(go env GOOS)_$(go env GOARCH)/libffmpeg.a" ]; then
        echo "Error: ffmpeg-statigo library not downloaded. Run 'just setup' first."
        exit 1
    fi

# Clean build artifacts and generated reports/data (.log, .json run-records, .jsonl sidecars)
clean:
    @rm -fv jive-vocals 2>/dev/null || true
    @rm -fv testdata/*-processed.* 2>/dev/null || true
    @rm -fv testdata/*-analysis.* 2>/dev/null || true
    @rm -fv testdata/LMP-*-stashed.* 2>/dev/null || true
    @rm -fv testdata/*.png 2>/dev/null || true
    @rm -fv testdata/*.txt 2>/dev/null || true

# Install jive-vocals to ~/.local/bin
install: build
    @mkdir -p ~/.local/bin 2>/dev/null || true
    @mv ./jive-vocals ~/.local/bin/jive-vocals
    @echo "Installed jive-vocals to ~/.local/bin/jive-vocals"
    @echo "Make sure ~/.local/bin is in your PATH"

# Record pages/demo.webp and pages/demo-still.png from jive-vocals.tape using ntrecord
record: build
    #!/usr/bin/env bash
    set -euo pipefail

    for tool in ntrecord ffmpeg oxipng; do
        command -v "$tool" >/dev/null || {
            printf "just record: %s not found; enter the Nix shell with 'nix develop'\n" "$tool" >&2
            exit 1
        }
    done

    repo_root="$PWD"
    tmp_dir="$(mktemp -d)"

    cleanup() {
        rm -f -- "$repo_root/.demo.mp4"
        if [[ -n "${tmp_dir:-}" && -d "$tmp_dir" ]]; then
            rm -rf -- "$tmp_dir"
        fi
    }
    trap cleanup EXIT

    mkdir -p pages
    cp -- testdata/LMP-81s-mark.flac testdata/LMP-81s-popey.flac "$tmp_dir/"

    font_env=()
    if command -v fc-list >/dev/null 2>&1; then
        cjk_font="$(fc-list :charset=33c8 -f '%{file}\n' 2>/dev/null | head -n 1 || true)"
        if [[ -n "$cjk_font" ]]; then
            base_font_path="$(sed -n "s/^export NTRECORD_FONT_PATH=\${NTRECORD_FONT_PATH-'\([^']*\)'}$/\1/p" "$(command -v ntrecord)" 2>/dev/null || true)"
            if [[ -n "$base_font_path" ]]; then
                font_env=(NTRECORD_FONT_PATH="${base_font_path}:$(dirname "$cjk_font")")
            fi
        fi
    fi

    env -u DIRENV_DIR -u DIRENV_FILE -u DIRENV_DIFF -u DIRENV_WATCHES \
        -u PROMPT_COMMAND -u BASH_ENV \
        PATH="$repo_root:$PATH" \
        JIVE_VOCALS_DEMO_DIR="$tmp_dir" \
        "${font_env[@]}" \
        ntrecord jive-vocals.tape

    ffmpeg -y -v error -i "$repo_root/.demo.mp4" \
        -vf "setpts=0.6*PTS,fps=8" \
        -c:v libwebp_anim -lossless 0 -quality 70 -compression_level 6 -loop 0 \
        pages/demo.webp
    oxipng -q -o 4 --strip safe pages/demo-still.png

# Show current version (from git tags or "dev" if no tags)
version:
    @git describe --tags --always --dirty 2>/dev/null || echo "dev"

# List recent releases
releases:
    @git tag --sort=-creatordate | head -10

# Show what would be in the next release changelog
changelog:
    #!/usr/bin/env bash
    LATEST_TAG=$(git describe --tags --abbrev=0 2>/dev/null || echo "")
    if [ -n "$LATEST_TAG" ]; then
        echo "Changes since $LATEST_TAG:"
        git log $LATEST_TAG..HEAD --pretty=format:"* %s (%h)"
    else
        echo "No previous tags found. All commits:"
        git log --pretty=format:"* %s (%h)"
    fi

# Create a new release tag (requires VERSION=x.y.z)
release VERSION:
    #!/usr/bin/env bash
    set -e

    # Validate version format
    if ! echo "{{VERSION}}" | grep -qE '^[0-9]+\.[0-9]+\.[0-9]+$'; then
        echo "Error: VERSION must be in format x.y.z (e.g., 0.1.0)"
        exit 1
    fi

    # Check for uncommitted changes
    if ! git diff-index --quiet HEAD --; then
        echo "Error: Working directory has uncommitted changes"
        exit 1
    fi

    # Check if tag already exists
    if git rev-parse "{{VERSION}}" >/dev/null 2>&1; then
        echo "Error: Tag {{VERSION}} already exists"
        exit 1
    fi

    echo "Creating release {{VERSION}}..."
    git tag -a "{{VERSION}}" -m "Release {{VERSION}}"
    echo "✓ Tag {{VERSION}} created"
    echo ""
    echo "To publish the release:"
    echo "  git push origin {{VERSION}}"
    echo ""
    echo "This will trigger the GitHub Actions release workflow which will:"
    echo "  - Build binaries for all platforms"
    echo "  - Generate changelog from commits"
    echo "  - Create GitHub release with downloadable assets"
