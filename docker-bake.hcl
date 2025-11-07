variable "GO_VERSION" {
	default = "1.25.4"
}
variable "VERSION" {
	default = ""
}
variable "IMAGE_NAME" {
	default = "wpm-cli"
}

# Sets the name of the company that produced the windows binary.
variable "PACKAGER_NAME" {
	default = "wpm team"
}

target "_common" {
	args = {
		GO_VERSION = GO_VERSION
	}
}

target "_platforms" {
	platforms = [
		"darwin/amd64",
		"darwin/arm64",
		"linux/amd64",
		"linux/arm/v6",
		"linux/arm/v7",
		"linux/arm64",
		"linux/ppc64le",
		"linux/riscv64",
		"linux/s390x",
		"windows/amd64",
		"windows/arm64"
	]
}

group "default" {
	targets = ["binary"]
}

target "binary" {
	inherits = ["_common"]
	target = "binary"
	platforms = ["local"]
	output = ["build"]
	args = {
		VERSION = VERSION
		PACKAGER_NAME = PACKAGER_NAME
	}
}

target "cross" {
	inherits = ["binary", "_platforms"]
}
