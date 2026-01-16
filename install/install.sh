#!/usr/bin/env bash
set -euo pipefail

# Reset
Color_Off=''

# Regular Colors
Red=''
Green=''
Dim='' # White

# Bold
Bold_White=''
Bold_Green=''

if [[ -t 1 ]]; then
  # Reset
  Color_Off='\033[0m' # Text Reset

  # Regular Colors
  Red='\033[0;31m'   # Red
  Green='\033[0;32m' # Green
  Dim='\033[0;2m'    # White

  # Bold
  Bold_Green='\033[1;32m' # Bold Green
  Bold_White='\033[1m'    # Bold White
fi

error() {
  echo -e "${Red}error${Color_Off}:" "$@" >&2
  exit 1
}

info() {
  echo -e "${Dim}$@ ${Color_Off}"
}

info_bold() {
  echo -e "${Bold_White}$@ ${Color_Off}"
}

success() {
  echo -e "${Green}$@ ${Color_Off}"
}

if [[ $# -gt 1 ]]; then
    error 'Usage: install.sh [version]'
fi

platform=$(uname -ms)

case $platform in
  *'MINGW'* | *'CYGWIN'* | *'MSYS'* | 'Windows_NT'*)
    error "Please run \`powershell -c \"irm wpm.so/install.ps1|iex\"\` to install wpm on Windows systems."
    ;;
esac

case $platform in
  # --- macOS ---
  'Darwin x86_64')
    target="darwin-amd64"
    ;;
  'Darwin arm64')
    target="darwin-arm64"
    ;;

  # --- Linux ---
  'Linux x86_64')
    target="linux-amd64"
    ;;
  'Linux aarch64' | 'Linux arm64')
    target="linux-arm64"
    ;;
  'Linux armv7'*)
    target="linux-arm-v7"
    ;;
  'Linux armv6'*)
    target="linux-arm-v6"
    ;;
  'Linux ppc64le')
    target="linux-ppc64le"
    ;;
  'Linux riscv64')
    target="linux-riscv64"
    ;;
  'Linux s390x')
    target="linux-s390x"
    ;;

  # --- Unsupported ---
  *)
    error "Unsupported platform: $platform"
    exit 1
    ;;
esac

if [[ $target = darwin-amd64 ]]; then
  # Is this process running in Rosetta?
  # redirect stderr to devnull to avoid error message when not running in Rosetta
  if [[ $(sysctl -n sysctl.proc_translated 2>/dev/null) = 1 ]]; then
    target=darwin-arm64
    info "Your shell is running in Rosetta 2. Downloading wpm for $target instead"
  fi
fi

GITHUB=${GITHUB-"https://github.com"}

github_repo="$GITHUB/trywpm/cli"

exe_name=wpm
bin_dir="/usr/local/bin"
exe="$bin_dir/$exe_name"

if [[ $# = 0 ]]; then
  wpm_uri=$github_repo/releases/latest/download/wpm-$target
else
  wpm_uri=$github_repo/releases/download/$1/wpm-$target
fi

curl --fail --location --progress-bar --output "/tmp/$exe_name" "$wpm_uri" ||
  error "Failed to download wpm from \"$wpm_uri\""

chmod +x "/tmp/$exe_name" ||
  error 'Failed to make wpm executable'

sudo mv "/tmp/$exe_name" "$exe" ||
  error 'Failed to move extracted wpm to destination'

echo
success "wpm installed to ${Bold_Green}$exe${Color_Off}"

echo
info "To get started, run:"
echo
info_bold "  wpm --help"
