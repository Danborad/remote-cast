#!/usr/bin/env bash
set -euo pipefail

OPENWRT_VERSION="${OPENWRT_VERSION:-24.10.7}"
OPENWRT_TARGET="${OPENWRT_TARGET:-x86/64}"
SDK_ROOT="/tmp/openwrt-sdk"
OUT_DIR="/work/bin"
BASE_URL="https://downloads.openwrt.org/releases/${OPENWRT_VERSION}/targets/${OPENWRT_TARGET}"

export DEBIAN_FRONTEND=noninteractive

apt-get update
apt-get install -y --no-install-recommends \
	build-essential ca-certificates curl file gawk gettext-base git \
	golang-go libncurses5-dev libssl-dev perl python3 python3-distutils \
	python3-setuptools rsync tar unzip wget xz-utils zstd

rm -rf "$SDK_ROOT"
mkdir -p "$SDK_ROOT" "$OUT_DIR"

curl -fsSL "${BASE_URL}/sha256sums" -o /tmp/openwrt-sha256sums
SDK_FILE="$(grep -E 'openwrt-sdk-.*x86-64.*Linux-x86_64\.tar\.(xz|zst)$' /tmp/openwrt-sha256sums | head -n1 | sed 's/^.* \*//')"

if [ -z "$SDK_FILE" ]; then
	echo "Cannot find SDK file in ${BASE_URL}/sha256sums" >&2
	exit 1
fi

echo "Downloading ${SDK_FILE}"
curl -fSL "${BASE_URL}/${SDK_FILE}" -o "/tmp/${SDK_FILE}"
(cd /tmp && grep " \*${SDK_FILE}$" /tmp/openwrt-sha256sums | sha256sum -c -)

tar --strip-components=1 -xf "/tmp/${SDK_FILE}" -C "$SDK_ROOT"

rm -rf "$SDK_ROOT/package/remote-cast" "$SDK_ROOT/package/luci-app-remote-cast"
cp -a /work/package/remote-cast "$SDK_ROOT/package/remote-cast"
cp -a /work/package/luci-app-remote-cast "$SDK_ROOT/package/luci-app-remote-cast"

cd "$SDK_ROOT"
make defconfig
make package/remote-cast/compile V=s
make package/luci-app-remote-cast/compile V=s

find "$SDK_ROOT/bin/packages" -type f \( -name 'remote-cast_*.ipk' -o -name 'luci-app-remote-cast_*.ipk' \) -exec cp -v {} "$OUT_DIR/" \;
echo "Done. IPK files are in ${OUT_DIR}"
