#!/bin/sh

set -eu

repository="UFFeScience/akoflow"
install_dir="${AKOFLOW_INSTALL_DIR:-${HOME}/.local/bin}"
version="${AKOFLOW_VERSION:-}"

command -v curl >/dev/null 2>&1 || {
    echo "curl is required" >&2
    exit 1
}

if [ -z "$version" ]; then
    version=$(curl -fsSL "https://api.github.com/repos/${repository}/releases/latest" |
        sed -n 's/.*"tag_name": *"v\([^"]*\)".*/\1/p' |
        head -n 1)
fi

[ -n "$version" ] || {
    echo "could not resolve the latest Akoflow version" >&2
    exit 1
}

case "$(uname -s)" in
    Linux) platform=linux ;;
    Darwin) platform=darwin ;;
    *) echo "unsupported operating system: $(uname -s)" >&2; exit 1 ;;
esac

case "$(uname -m)" in
    x86_64|amd64) architecture=amd64 ;;
    arm64|aarch64) architecture=arm64 ;;
    *) echo "unsupported architecture: $(uname -m)" >&2; exit 1 ;;
esac

mkdir -p "$install_dir"

download() {
    binary=$1
    asset=$2
    temporary="${install_dir}/.${binary}.download"
    curl -fsSL \
        "https://github.com/${repository}/releases/download/v${version}/${asset}" \
        -o "$temporary"
    chmod 0755 "$temporary"
    mv "$temporary" "${install_dir}/${binary}"
}

download akoflow "akoflow_${version}_${platform}_${architecture}"
if [ "$platform" = "linux" ]; then
    download akoflow-server "akoflow-server_${version}_${platform}_${architecture}"
fi

echo "Akoflow ${version} installed in ${install_dir}"
if [ "$platform" = "linux" ]; then
    echo "Run the server with: akoflow-server"
else
    echo "The Akoflow server is distributed for Linux; the CLI is ready to use."
fi
