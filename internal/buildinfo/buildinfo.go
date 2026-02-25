package buildinfo

// Commit is the git commit hash, injected at build time via ldflags.
var Commit = "dev"

// Date is the UTC build timestamp (RFC3339), injected at build time via ldflags.
var Date = "unknown"
