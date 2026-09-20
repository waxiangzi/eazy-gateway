// Package version carries the build identity of the running binary. Releases
// inject it at link time from the git tag, which is the single source of truth;
// see the Version variable.
package version

// Version is the release identity of this build:
//
//	go build -ldflags "-X github.com/eazy-gateway/eazy-gateway/internal/version.Version=v1.0.3"
//
// A build without the flag reports "dev". It is a variable rather than a
// constant so the linker can set it.
var Version = "dev"
