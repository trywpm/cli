#!/usr/bin/env bash
set -euo pipefail

# Reset
Color_Off=''
# Regular Colors
Red=''
Green=''
Dim=''
# Bold
Bold_White=''
Bold_Green=''

if [[ -t 1 ]]; then
  Color_Off='\033[0m'
  Red='\033[0;31m'
  Green='\033[0;32m'
  Dim='\033[0;2m'
  Bold_Green='\033[1;32m'
  Bold_White='\033[1m'
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

tildify() {
  if [[ $1 == $HOME* ]]; then
    echo "~${1#$HOME}"
  else
    echo "$1"
  fi
}

if [[ -f "/usr/local/bin/wpm" ]]; then
  info "A previous installation of wpm was found in /usr/local/bin."
  info "Removing it to avoid conflicts..."
  sudo rm /usr/local/bin/wpm || error "Failed to remove /usr/local/bin/wpm"
  success "Removed /usr/local/bin/wpm"
fi

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
bin_dir="$HOME/.local/bin"
exe="$bin_dir/$exe_name"

if [[ $# = 0 ]]; then
  wpm_uri=$github_repo/releases/latest/download/wpm-$target
else
  wpm_uri=$github_repo/releases/download/$1/wpm-$target
fi

mkdir -p "$bin_dir"

info "Downloading wpm..."
curl --fail --location --progress-bar --output "/tmp/$exe_name" "$wpm_uri" ||
  error "Failed to download wpm from \"$wpm_uri\""


checksum_cmd=""
if [[ $platform == 'Darwin'* ]] && command -v shasum >/dev/null; then
  checksum_cmd="shasum -a 256 -c"
elif command -v sha256sum >/dev/null; then
  checksum_cmd="sha256sum -c"
elif command -v shasum >/dev/null; then
  checksum_cmd="shasum -a 256 -c"
fi

if [[ -n "$checksum_cmd" ]]; then
  checksum_uri="$wpm_uri.sha256"
  curl --fail --silent --location --output "/tmp/$exe_name.sha256" "$checksum_uri" || :

  if [[ -f "/tmp/$exe_name.sha256" ]]; then
    info "Verifying checksum..."
    echo "$(awk '{print $1}' "/tmp/$exe_name.sha256")  /tmp/$exe_name" | $checksum_cmd >/dev/null ||
      error "Checksum verification failed!"
  fi
fi

chmod +x "/tmp/$exe_name" || error 'Failed to make wpm executable'
mv "/tmp/$exe_name" "$exe" || error 'Failed to move wpm to destination'

success "wpm installed to ${Bold_Green}$(tildify "$exe")${Color_Off}"

if [[ ":$PATH:" == *":$bin_dir:"* ]]; then
  info "To get started, run:"
  echo
  info_bold "  wpm --help"
  exit 0
fi

case $(basename "$SHELL") in
fish)
    commands=(
        "set --export PATH $bin_dir \$PATH"
    )

    fish_config=$HOME/.config/fish/config.fish
    tilde_fish_config=$(tildify "$fish_config")

    if [[ -w $fish_config ]]; then
        {
            echo -e '\n# wpm'
            for command in "${commands[@]}"; do echo "$command"; done
        } >>"$fish_config"

        info "Added \"$(tildify "$bin_dir")\" to \$PATH in \"$tilde_fish_config\""
        info "To get started, run:"
        info_bold "  source $tilde_fish_config"
        info_bold "  wpm --help"
    else
        echo "Manually add the directory to $tilde_fish_config (or similar):"
        for command in "${commands[@]}"; do info_bold "  $command"; done
    fi
    ;;
zsh)
    commands=(
        "export PATH=\"$bin_dir:\$PATH\""
    )

    zsh_config=$HOME/.zshrc
    tilde_zsh_config=$(tildify "$zsh_config")

    if [[ -w $zsh_config ]]; then
        {
            echo -e '\n# wpm'
            for command in "${commands[@]}"; do echo "$command"; done
        } >>"$zsh_config"

        info "Added \"$(tildify "$bin_dir")\" to \$PATH in \"$tilde_zsh_config\""
        info "To get started, run:"
        info_bold "  exec \$SHELL"
        info_bold "  wpm --help"
    else
        echo "Manually add the directory to $tilde_zsh_config (or similar):"
        for command in "${commands[@]}"; do info_bold "  $command"; done
    fi
    ;;
bash)
    commands=(
        "export PATH=\"$bin_dir:\$PATH\""
    )

    bash_configs=("$HOME/.bashrc" "$HOME/.bash_profile")
    if [[ ${XDG_CONFIG_HOME:-} ]]; then
        bash_configs+=(
            "$XDG_CONFIG_HOME/.bash_profile"
            "$XDG_CONFIG_HOME/.bashrc"
            "$XDG_CONFIG_HOME/bash_profile"
            "$XDG_CONFIG_HOME/bashrc"
        )
    fi

    set_manually=true
    for bash_config in "${bash_configs[@]}"; do
        tilde_bash_config=$(tildify "$bash_config")

        if [[ -w $bash_config ]]; then
            {
                echo -e '\n# wpm'
                for command in "${commands[@]}"; do echo "$command"; done
            } >>"$bash_config"

            info "Added \"$(tildify "$bin_dir")\" to \$PATH in \"$tilde_bash_config\""
            info "To get started, run:"
            info_bold "  source $tilde_bash_config"
            info_bold "  wpm --help"

            set_manually=false
            break
        fi
    done

    if [[ $set_manually = true ]]; then
        echo "Manually add the directory to $tilde_bash_config (or similar):"
        for command in "${commands[@]}"; do info_bold "  $command"; done
    fi
    ;;
*)
    echo "Manually add the directory to your shell configuration:"
    info_bold "  export PATH=\"$bin_dir:\$PATH\""
    ;;
esac
