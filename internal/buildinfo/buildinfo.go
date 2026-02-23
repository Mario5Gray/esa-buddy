package buildinfo

// Commit is the git commit hash used to build the binary.
// It can be overridden at build time via ldflags.
var Commit = "dev"
