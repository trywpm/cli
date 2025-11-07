//go:build windows && arm

//go:generate goversioninfo -arm=true -o=./winresources/resource.syso -icon=internal/assets/wpm.ico -manifest=internal/assets/wpm.exe.manifest ./winresources/versioninfo.json

package main

import _ "wpm/cmd/wpm/winresources"
