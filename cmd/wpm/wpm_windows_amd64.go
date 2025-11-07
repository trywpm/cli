//go:build windows && amd64

//go:generate goversioninfo -64=true -o=./winresources/resource.syso -icon=internal/assets/wpm.ico -manifest=internal/assets/wpm.exe.manifest ./winresources/versioninfo.json

package main

import _ "wpm/cmd/wpm/winresources"
