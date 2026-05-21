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
  sudo rm -f /usr/local/bin/wpm || error "Failed to remove /usr/local/bin/wpm"
  success "Removed /usr/local/bin/wpm"
fi

if [[ -f "$HOME/.local/bin/wpm" ]]; then
  info "A previous installation of wpm was found in $HOME/.local/bin."
  info "Removing it to avoid conflicts..."
  rm -f "$HOME/.local/bin/wpm" || error "Failed to remove $HOME/.local/bin/wpm"
  success "Removed $HOME/.local/bin/wpm"
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
  'Linux s390x') target="linux-s390x" ;;
  'Linux x86_64') target="linux-amd64" ;;
  'Darwin arm64') target="darwin-arm64" ;;
  'Linux armv6'*) target="linux-arm-v6" ;;
  'Linux armv7'*) target="linux-arm-v7" ;;
  'Darwin x86_64') target="darwin-amd64" ;;
  'Linux ppc64le') target="linux-ppc64le" ;;
  'Linux riscv64') target="linux-riscv64" ;;
  'Linux aarch64' | 'Linux arm64') target="linux-arm64" ;;
  *) error "Unsupported platform: $platform" ;;
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
install_env=WPM_INSTALL
bin_env=\$$install_env/bin
completions_env=\$$install_env/completions

wpm_install_dir="${!install_env:-$HOME/.wpm}"
bin_dir="$wpm_install_dir/bin"
completions_dir="$wpm_install_dir/completions"
exe="$bin_dir/$exe_name"

if [[ $# = 0 ]]; then
  wpm_uri=$github_repo/releases/latest/download/wpm-$target
else
  wpm_uri=$github_repo/releases/download/$1/wpm-$target
fi

mkdir -p "$bin_dir" "$completions_dir"

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

"$exe" completion zsh > "$completions_dir/_wpm" 2>/dev/null || :
"$exe" completion bash > "$completions_dir/wpm.bash" 2>/dev/null || :
"$exe" completion fish > "$completions_dir/wpm.fish" 2>/dev/null || :

success "wpm installed to ${Bold_Green}$(tildify "$exe")${Color_Off}"

if command -v wpm >/dev/null; then
  echo "Run 'wpm --help' to get started"
  exit
fi

refresh_command=''

tilde_bin_dir=$(tildify "$bin_dir")
quoted_install_dir=\"${wpm_install_dir//\"/\\\"}\"

if [[ $quoted_install_dir = \"$HOME/* ]]; then
    quoted_install_dir=${quoted_install_dir/$HOME\//\$HOME/}
fi

echo

case $(basename "$SHELL") in
fish)
    commands=(
      "set --export $install_env $quoted_install_dir"
      "set --export PATH $bin_env \$PATH"
      "set -gx fish_complete_path \"$completions_env\" \$fish_complete_path"
    )

    fish_config=$HOME/.config/fish/config.fish
    tilde_fish_config=$(tildify "$fish_config")

    if [[ -w $fish_config ]]; then
      {
        echo -e '\n# wpm'

        for command in "${commands[@]}"; do
          echo "$command"
        done
      } >>"$fish_config"

      info "Added \"$tilde_bin_dir\" to \$PATH in \"$tilde_fish_config\""

      refresh_command="source $tilde_fish_config"
    else
      echo "Manually add the directory to $tilde_fish_config (or similar):"

      for command in "${commands[@]}"; do
        info_bold "  $command"
      done
    fi
    ;;
zsh)
    commands=(
      "export $install_env=$quoted_install_dir"
      "export PATH=\"$bin_env:\$PATH\""
      "fpath=(\"$completions_env\" \$fpath)"
    )

    zsh_config=$HOME/.zshrc
    tilde_zsh_config=$(tildify "$zsh_config")

    if [[ -w $zsh_config ]]; then
      {
          echo -e '\n# wpm'

          for command in "${commands[@]}"; do
              echo "$command"
          done
      } >>"$zsh_config"

      info "Added \"$tilde_bin_dir\" to \$PATH in \"$tilde_zsh_config\""

      refresh_command="exec $SHELL"
    else
      echo "Manually add the directory to $tilde_zsh_config (or similar):"

      for command in "${commands[@]}"; do
          info_bold "  $command"
      done
    fi
    ;;
bash)
    commands=(
      "export $install_env=$quoted_install_dir"
      "export PATH=\"$bin_env:\$PATH\""
      "[ -s \"$completions_env/wpm.bash\" ] && source \"$completions_env/wpm.bash\""
    )

    bash_configs=(
      "$HOME/.bash_profile"
      "$HOME/.bashrc"
    )

    if [[ ${XDG_CONFIG_HOME:-} ]]; then
        bash_configs=(
          "${bash_configs[@]}"
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

            for command in "${commands[@]}"; do
              echo "$command"
            done
          } >>"$bash_config"

          info "Added \"$tilde_bin_dir\" to \$PATH in \"$tilde_bash_config\""

          refresh_command="source $bash_config"
          set_manually=false
          break
        fi
    done

    if [[ $set_manually = true ]]; then
        echo "Manually add the directory to $tilde_bash_config (or similar):"

        for command in "${commands[@]}"; do
          info_bold "  $command"
        done
    fi
    ;;
*)
    echo 'Manually add the directory to ~/.bashrc (or similar):'
    info_bold "  export $install_env=$quoted_install_dir"
    info_bold "  export PATH=\"$bin_env:\$PATH\""
    ;;
esac

echo
info "To get started, run:"
echo

if [[ $refresh_command ]]; then
  info_bold "  $refresh_command"
fi

info_bold "  wpm --help"
