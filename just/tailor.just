# Managed by Tailor: just/tailor.just
export CGO_ENABLED := "1"

# Run explicitly requested setup for enabled components
setup:
    #!/usr/bin/env bash
    set -euo pipefail
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
        RELEASES=$(curl -fsS https://api.github.com/repos/linuxmatters/ffmpeg-statigo/releases) || exit 1
        if command -v jq &> /dev/null; then
            printf '%s\n' "$RELEASES" | \
                jq -r '[.[] | select(.prerelease == false and .draft == false and (.tag_name | startswith("v")))][0].tag_name'
        else
            printf '%s\n' "$RELEASES" | \
                grep -B5 '"prerelease": false' | grep '"tag_name": "v' | grep -v 'lib-' | head -1 | cut -d'"' -f4
        fi
    ) || TAG=""
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
    output_count=0

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
    if [[ -z "${main_packages[*]-}" ]]; then
        go build ./...
        echo "Go packages built successfully. No executable packages were found."
        exit 0
    fi

    for package in "${main_packages[@]}"; do
        output="${package##*/}"
        if [[ "${output}" =~ ^v([2-9]|[1-9][0-9]+)$ ]]; then
            parent="${package%/*}"
            output="${parent##*/}"
        fi
        output="${output}${goexe}"
        for ((index = 0; index < output_count; index++)); do
            if [[ "${outputs[${index}]}" == "${output}" ]]; then
                echo "Error: Go executable packages ${owners[${index}]} and ${package} both produce bin/${output}" >&2
                exit 1
            fi
        done
        check_output_destination "${output}"
        owners+=("${package}")
        outputs+=("${output}")
        output_count=$((output_count + 1))
    done

    version="$(git describe --tags --match 'v*.*.*' --always --dirty 2>/dev/null || echo dev)"
    version="${version#v}"
    work="$(mktemp -d "${TMPDIR:-/tmp}/tailor-go-build.XXXXXX")"
    trap 'rm -rf "${work}"' EXIT
    mkdir "${work}/validate" "${work}/executables"
    go build -o "${work}/validate/" ./...
    go build -trimpath -ldflags "-s -w -X main.version=${version}" -o "${work}/executables/" "${main_packages[@]}"

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
    check_vendor_hash() {
        local file='nix/package.nix' line value='' count=0 computed updated=''
        local assignment='^[[:space:]]*vendorHash[[:space:]]*=[[:space:]]*([^;]*);'
        local literal='^"(sha256-[A-Za-z0-9+/]+=*)"$'
        local unsupported='^[[:space:]]*(proxyVendor[[:space:]]*=[[:space:]]*true|modRoot|overrideModAttrs)'
        if [[ ! -f "${file}" ]]; then
            echo "No ${file} found. Skipping the vendorHash check."
            return 0
        fi
        while IFS= read -r line || [[ -n "${line}" ]]; do
            if [[ "${line}" =~ ${unsupported} ]]; then
                echo "${file} sets ${BASH_REMATCH[1]}. Skipping the vendorHash check."
                return 0
            fi
            if [[ "${line}" =~ ${assignment} ]]; then
                value="${BASH_REMATCH[1]}"
                count=$((count + 1))
            fi
        done < "${file}"
        value="${value%"${value##*[![:space:]]}"}"
        if [[ "${count}" -eq 0 ]]; then
            echo "${file} has no vendorHash. Skipping the vendorHash check."
            return 0
        fi
        if [[ "${count}" -eq 1 && "${value}" == null ]]; then
            echo "${file} sets vendorHash = null. Skipping the vendorHash check."
            return 0
        fi
        if [[ "${count}" -ne 1 || ! "${value}" =~ ${literal} ]]; then
            echo "${file} has no single literal vendorHash. Skipping the vendorHash check."
            return 0
        fi
        value="${BASH_REMATCH[1]}"
        if ! command -v nix > /dev/null; then
            echo "nix is unavailable. Skipping the vendorHash check."
            return 0
        fi
        vendor_hash_work="$(mktemp -d "${TMPDIR:-/tmp}/tailor-go-vendor.XXXXXX")"
        trap 'rm -rf "${vendor_hash_work}"' EXIT
        GOWORK=off go mod vendor -o "${vendor_hash_work}/vendor"
        if [[ ! -f "${vendor_hash_work}/vendor/modules.txt" ]]; then
            echo "Error: go mod vendor found no dependencies. Set vendorHash = null in ${file}" >&2
            return 1
        fi
        computed="$(nix --extra-experimental-features nix-command hash path --sri "${vendor_hash_work}/vendor")"
        if [[ ! "${computed}" =~ ^sha256-[A-Za-z0-9+/]+=*$ ]]; then
            echo "Error: nix hash path returned an unexpected value: ${computed}" >&2
            return 1
        fi
        if [[ "${computed}" == "${value}" ]]; then
            echo "${file} vendorHash matches go.mod and go.sum."
            return 0
        fi
        if [[ "${mode}" != correct ]]; then
            echo "Error: ${file} declares vendorHash ${value}, but go.mod and go.sum give ${computed}" >&2
            echo "Run 'just lint correct', or set vendorHash = \"${computed}\"; in ${file}" >&2
            return 1
        fi
        if [[ -L "${file}" ]]; then
            echo "Error: refusing to update symlink ${file}" >&2
            return 1
        fi
        while IFS= read -r line || [[ -n "${line}" ]]; do
            if [[ "${line}" =~ ${assignment} ]]; then
                line="${line%%"${value}"*}${computed}${line#*"${value}"}"
            fi
            updated+="${line}"$'\n'
        done < "${file}"
        printf '%s' "${updated}" > "${file}"
        echo "Updated vendorHash in ${file} from ${value} to ${computed}."
    }
    check_vendor_hash
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
