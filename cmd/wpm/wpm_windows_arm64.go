//go:build windows && arm64

//go:generate goversioninfo -arm=true -64=true -o=./winresources/resource.syso -icon=internal/assets/wpm.ico -manifest=internal/assets/wpm.exe.manifest ./winresources/versioninfo.json

package main

import _ "wpm/cmd/wpm/winresources"
