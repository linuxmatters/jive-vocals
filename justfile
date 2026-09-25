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

# Make a VHS tape recording
vhs: build
    @rm -f testdata/LMP-81s-*-processed.flac
    @vhs ./jive-vocals.tape

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
