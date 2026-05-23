variable "GO_VERSION" {
	default = "1.26.3"
}
variable "VERSION" {
	default = ""
}
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

# Special target: https://github.com/docker/metadata-action#bake-definition
target "meta-helper" {}

group "default" {
	targets = ["binary"]
}

target "binary" {
	target = "binary"
	output = ["build"]
	inherits = ["_common"]
	platforms = ["local"]
	args = {
		VERSION = VERSION
		PACKAGER_NAME = PACKAGER_NAME
	}
}

target "binary-cross" {
	inherits = ["binary", "_platforms"]
}

target "image-cross" {
	target = "image"
	output = ["type=image"]
	inherits = ["meta-helper", "binary-cross"]
}
