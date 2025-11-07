ARG GO_VERSION=1.25.4

ARG ALPINE_VERSION=3.22

# XX_VERSION specifies the version of the xx utility to use.
# It must be a valid tag in the docker.io/tonistiigi/xx image repository.
ARG XX_VERSION=1.7.0

# GOVERSIONINFO_VERSION is the version of GoVersionInfo to install.
# It must be a valid tag from https://github.com/josephspurrier/goversioninfo
ARG GOVERSIONINFO_VERSION=v1.5.0

FROM --platform=$BUILDPLATFORM tonistiigi/xx:${XX_VERSION} AS xx

FROM --platform=$BUILDPLATFORM golang:${GO_VERSION}-alpine${ALPINE_VERSION} AS build-base-alpine
ENV GOTOOLCHAIN=local
COPY --link --from=xx / /
RUN apk add --no-cache bash clang lld llvm file git git-daemon
WORKDIR /go/src/wpm

FROM build-base-alpine AS build-alpine
ARG TARGETPLATFORM
# gcc is installed for libgcc only
RUN xx-apk add --no-cache musl-dev gcc

FROM build-base-debian AS build-debian
ARG TARGETPLATFORM
RUN xx-apt-get install --no-install-recommends -y libc6-dev libgcc-12-dev pkgconf

FROM build-base-alpine AS goversioninfo
ARG GOVERSIONINFO_VERSION
RUN --mount=type=cache,target=/root/.cache/go-build \
	--mount=type=cache,target=/go/pkg/mod \
	GOBIN=/out GO111MODULE=on CGO_ENABLED=0 go install "github.com/josephspurrier/goversioninfo/cmd/goversioninfo@${GOVERSIONINFO_VERSION}"

FROM build-alpine AS build
# GO_LINKMODE defines if static or dynamic binary should be produced
ARG GO_LINKMODE=static
# GO_BUILDTAGS defines additional build tags
ARG GO_BUILDTAGS
# GO_STRIP strips debugging symbols if set
ARG GO_STRIP
# CGO_ENABLED manually sets if cgo is used
ARG CGO_ENABLED
# VERSION sets the version for the produced binary
ARG VERSION
# PACKAGER_NAME sets the company that produced the windows binary
ARG PACKAGER_NAME
COPY --link --from=goversioninfo /out/goversioninfo /usr/bin/goversioninfo
RUN --mount=type=bind,target=.,ro \
	--mount=type=cache,target=/root/.cache \
	--mount=type=tmpfs,target=cmd/wpm/winresources \
	# override the default behavior of go with xx-go
	xx-go --wrap && \
	TARGET=/out ./scripts/build/binary && \
	xx-verify $([ "$GO_LINKMODE" = "static" ] && echo "--static") /out/wpm

FROM scratch AS binary
COPY --from=build /out .
