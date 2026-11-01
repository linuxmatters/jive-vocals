# Managed by Tailor: just/tailor.just
export CGO_ENABLED := "1"

# Run explicitly requested setup for enabled components
setup:
    #!/usr/bin/env bash
    set -e
    (
    set -e
    if grep -Eq '^module[[:space:]]+"?github\.com/linuxmatters/ffmpeg-statigo"?([[:space:]]|$)' go.mod; then
        echo "Error: setup supports consumer repositories only, not the ffmpeg-statigo source root."
        exit 1
    fi
    echo "Configuring git for submodule-friendly pulls..."
    git config pull.ff only
    git config submodule.recurse true

    # Get latest stable release tag
    TAG=$(
        if command -v jq &> /dev/null; then
            curl -s https://api.github.com/repos/linuxmatters/ffmpeg-statigo/releases | \
                jq -r '[.[] | select(.prerelease == false and .draft == false and (.tag_name | startswith("v")))][0].tag_name'
        else
            curl -s https://api.github.com/repos/linuxmatters/ffmpeg-statigo/releases | \
                grep -B5 '"prerelease": false' | grep '"tag_name"' | grep -v 'lib-' | head -1 | cut -d'"' -f4
        fi
    )
    if [ -z "$TAG" ] || [ "$TAG" = "null" ]; then
        echo "Error: Could not fetch latest release tag"
        exit 1
    fi

    # Initialise submodule if not already present
    if [ ! -f "third_party/ffmpeg-statigo/go.mod" ]; then
        echo "Initialising ffmpeg-statigo submodule..."
        git submodule update --init --recursive
    fi

    # Check current version
    cd third_party/ffmpeg-statigo
    git fetch --no-tags origin "refs/tags/$TAG:refs/tags/$TAG"
    CURRENT=$(git describe --tags --exact-match 2>/dev/null || echo "")

    if [ "$CURRENT" = "$TAG" ]; then
        echo "ffmpeg-statigo already at latest version ($TAG)"
        cd ../..
    else
        if [ -n "$CURRENT" ]; then
            echo "Updating ffmpeg-statigo from $CURRENT to $TAG..."
        else
            echo "Setting up ffmpeg-statigo $TAG..."
        fi
        git checkout "$TAG"
        cd ../..

        # Remove old library to force re-download
        rm -f third_party/ffmpeg-statigo/lib/*/libffmpeg.a

        # Stage the submodule change
        git add third_party/ffmpeg-statigo
    fi

    # Download libraries (will skip if correct version already exists)
    echo "Checking ffmpeg-statigo libraries..."
    cd third_party/ffmpeg-statigo && env CGO_ENABLED=1 go run ./cmd/download-lib
    cd ../..

    # Check if there are staged changes to commit
    if git diff --cached --quiet third_party/ffmpeg-statigo; then
        echo "Setup complete!"
    else
        echo ""
        echo "Setup complete! Submodule updated to $TAG"
        echo "Don't forget to commit: git commit -m 'chore: update ffmpeg-statigo to $TAG'"
    fi
    )

# Build all enabled ecosystems
build:
    #!/usr/bin/env bash
    set -euo pipefail
    (
    list_output="$(go list -f '{{ "{{" }}if eq .Name "main"{{ "}}" }}{{ "{{" }}.ImportPath{{ "}}" }}{{ "{{" }}end{{ "}}" }}' ./...)"
    main_packages=()
    while IFS= read -r package; do
        if [[ -n "${package}" ]]; then
            main_packages+=("${package}")
        fi
    done <<< "${list_output}"

    goexe="$(go env GOEXE)"
    outputs=()
    owners=()

    check_bin_directory() {
        if [[ -L bin ]]; then
            echo "Error: refusing to use symlink bin" >&2
            exit 1
        fi
        if [[ -e bin && ! -d bin ]]; then
            echo "Error: bin exists and is not a directory" >&2
            exit 1
        fi
    }

    check_output_destination() {
        destination="bin/$1"
        if [[ -L "${destination}" ]]; then
            echo "Error: refusing to replace symlink ${destination}" >&2
            exit 1
        fi
        if [[ -e "${destination}" && ! -f "${destination}" ]]; then
            echo "Error: refusing to replace non-regular ${destination}" >&2
            exit 1
        fi
    }

    check_bin_directory
    for package in "${main_packages[@]}"; do
        output="${package##*/}"
        if [[ "${output}" =~ ^v([2-9]|[1-9][0-9]+)$ ]]; then
            parent="${package%/*}"
            output="${parent##*/}"
        fi
        output="${output}${goexe}"
        for index in "${!outputs[@]}"; do
            if [[ "${outputs[${index}]}" == "${output}" ]]; then
                echo "Error: Go executable packages ${owners[${index}]} and ${package} both produce bin/${output}" >&2
                exit 1
            fi
        done
        check_output_destination "${output}"
        owners+=("${package}")
        outputs+=("${output}")
    done

    if (( ${#main_packages[@]} == 0 )); then
        go build ./...
        echo "Go packages built successfully. No executable packages were found."
        exit 0
    fi

    work="$(mktemp -d "${TMPDIR:-/tmp}/tailor-go-build.XXXXXX")"
    trap 'rm -rf "${work}"' EXIT
    mkdir "${work}/validate" "${work}/executables"
    go build -o "${work}/validate/" ./...
    go build -o "${work}/executables/" "${main_packages[@]}"

    check_bin_directory
    mkdir -p bin
    check_bin_directory
    for output in "${outputs[@]}"; do
        check_output_destination "${output}"
    done
    for output in "${outputs[@]}"; do
        check_bin_directory
        check_output_destination "${output}"
        mv "${work}/executables/${output}" "bin/${output}"
    done
    )
    project_script='just/project/build.sh'
    for parent in just just/project; do
        if [[ -L "${parent}" || ( -e "${parent}" && ! -d "${parent}" ) ]]; then
            echo "Error: project script parent ${parent} must be a directory, not a symlink" >&2
            exit 1
        fi
    done
    if [[ -L "${project_script}" || ( -e "${project_script}" && ! -f "${project_script}" ) ]]; then
        echo "Error: project script ${project_script} must be a regular file, not a symlink" >&2
        exit 1
    fi
    if [[ -f "${project_script}" ]]; then
        bash "${project_script}"
    fi

# Run full vet and tests with console coverage
test:
    #!/usr/bin/env bash
    set -euo pipefail
    (
    go vet ./...
    go test -cover -covermode=atomic ./...
    )
    project_script='just/project/test.sh'
    for parent in just just/project; do
        if [[ -L "${parent}" || ( -e "${parent}" && ! -d "${parent}" ) ]]; then
            echo "Error: project script parent ${parent} must be a directory, not a symlink" >&2
            exit 1
        fi
    done
    if [[ -L "${project_script}" || ( -e "${project_script}" && ! -f "${project_script}" ) ]]; then
        echo "Error: project script ${project_script} must be a regular file, not a symlink" >&2
        exit 1
    fi
    if [[ -f "${project_script}" ]]; then
        bash "${project_script}"
    fi

# Check source without changes, or tidy and format with correct
lint $mode="check":
    #!/usr/bin/env bash
    set -euo pipefail
    case "${mode}" in
        check|correct) ;;
        *) echo "Error: lint mode must be check or correct" >&2; exit 1 ;;
    esac
    check_workflows() {
        for workflow in .github/workflows/* .github/workflows/.*; do
            case "${workflow}" in *.yml|*.yaml)
                if [[ -f "${workflow}" ]]; then
                    actionlint
                    return
                fi ;;
            esac
        done
        echo "No GitHub Actions workflows found. Skipping actionlint."
    }
    check_workflows
    (
    if [[ "${mode}" == correct ]]; then
        go mod tidy
        golangci-lint fmt --config .golangci.yml
    fi
    go mod tidy -diff
    golangci-lint fmt --config .golangci.yml --diff
    golangci-lint run --config .golangci.yml
    govulncheck -show verbose ./...
    gocyclo -top 20 -avg -ignore '_test\.go$' .
    )
    project_script='just/project/lint.sh'
    for parent in just just/project; do
        if [[ -L "${parent}" || ( -e "${parent}" && ! -d "${parent}" ) ]]; then
            echo "Error: project script parent ${parent} must be a directory, not a symlink" >&2
            exit 1
        fi
    done
    if [[ -L "${project_script}" || ( -e "${project_script}" && ! -f "${project_script}" ) ]]; then
        echo "Error: project script ${project_script} must be a regular file, not a symlink" >&2
        exit 1
    fi
    if [[ -f "${project_script}" ]]; then
        bash "${project_script}" "${mode}"
    fi

# Create a local release tag with just release x.y.z
release $VERSION:
    #!/usr/bin/env bash
    set -e

    if ! [[ "${VERSION}" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
        echo "Error: VERSION must be in format x.y.z (e.g., 0.1.0)"
        exit 1
    fi

    # Require a clean checkout so that the tag matches the local source files.
    if [ -n "$(git status --porcelain)" ]; then
        echo "Error: Working directory is not clean"
        exit 1
    fi

    if git show-ref --tags --verify --quiet "refs/tags/v${VERSION}"; then
        echo "Error: Tag v${VERSION} already exists"
        exit 1
    fi

    echo "Creating release v${VERSION}..."
    git tag -a "v${VERSION}" -m "v${VERSION}"
    echo "Tag v${VERSION} created"
    echo ""
    echo "To publish the release:"
    echo "  git push origin v${VERSION}"
    echo ""
