param(
    [string]$OpenWrtVersion = "24.10.7",
    [string]$OpenWrtTarget = "x86/64"
)

$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent $MyInvocation.MyCommand.Path

docker run --rm `
    -e "OPENWRT_VERSION=$OpenWrtVersion" `
    -e "OPENWRT_TARGET=$OpenWrtTarget" `
    -v "${Root}:/work" `
    -w /work `
    ubuntu:22.04 `
    bash scripts/build-openwrt-ipk.sh
