//go:build windows && 386

//go:generate goversioninfo -o=./winresources/resource.syso -icon=internal/assets/wpm.ico -manifest=internal/assets/wpm.exe.manifest ./winresources/versioninfo.json

package main

import _ "wpm/cmd/wpm/winresources"
