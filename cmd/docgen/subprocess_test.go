package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestMain enables the "test binary as subprocess" pattern used by the
// subprocess-style e2e tests below.  When GO_WANT_HELPER_PROCESS=1 the
// binary re-invokes itself acting as the docgen CLI; otherwise it runs
// the normal test suite.
func TestMain(m *testing.M) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") == "1" {
		// Strip everything up to and including "--" so that main() sees
		// only the docgen arguments.
		args := os.Args
		for i, a := range args {
			if a == "--" {
				os.Args = append([]string{"docgen"}, args[i+1:]...)
				break
			}
		}
		main()
		os.Exit(0) // reached only when main() returns without calling os.Exit(1)
	}
	os.Exit(m.Run())
}

// TestHelperProcess is the named target for -test.run= in subprocess calls.
// It is a no-op in normal test runs; the real work happens inside TestMain
// when GO_WANT_HELPER_PROCESS=1.
func TestHelperProcess(t *testing.T) {}

// docgenCmd returns an exec.Cmd that runs the test binary as the docgen CLI.
// The caller should set cmd.Dir before calling cmd.Run or cmd.CombinedOutput.
func docgenCmd(t *testing.T, args ...string) *exec.Cmd {
	t.Helper()
	cs := []string{"-test.run=TestHelperProcess", "--"}
	cs = append(cs, args...)
	cmd := exec.Command(os.Args[0], cs...)
	cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
	return cmd
}

// TestOutputPathDerivation (scenario 3) verifies that an input file at
// subdir/input.md produces output at site/subdir/input.html, mirroring
// the input's directory segment under site/.
func TestOutputPathDerivation(t *testing.T) {
	wd := t.TempDir()

	inputDir := filepath.Join(wd, "subdir")
	if err := os.MkdirAll(inputDir, 0o755); err != nil {
		t.Fatalf("mkdir subdir: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(inputDir, "input.md"),
		[]byte("# Input\n"),
		0o644,
	); err != nil {
		t.Fatalf("write input: %v", err)
	}

	cmd := docgenCmd(t, filepath.Join("subdir", "input.md"))
	cmd.Dir = wd
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("docgen failed: %v\noutput: %s", err, out)
	}

	want := filepath.Join(wd, "site", "subdir", "input.html")
	if _, err := os.Stat(want); os.IsNotExist(err) {
		t.Errorf("output file not found: %s", want)
	}
}

// TestMultipleFilesOneInvocation (scenario 4) verifies that passing multiple
// input files in a single run produces one output file per input in the
// correct location and exits 0.
func TestMultipleFilesOneInvocation(t *testing.T) {
	wd := t.TempDir()

	inputs := []struct {
		src  string
		want string
	}{
		{filepath.Join("dir-a", "first.md"), filepath.Join("site", "dir-a", "first.html")},
		{filepath.Join("dir-b", "second.md"), filepath.Join("site", "dir-b", "second.html")},
	}

	for _, f := range inputs {
		dir := filepath.Join(wd, filepath.Dir(f.src))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
		if err := os.WriteFile(filepath.Join(wd, f.src), []byte("# Doc\n"), 0o644); err != nil {
			t.Fatalf("write %s: %v", f.src, err)
		}
	}

	args := make([]string, len(inputs))
	for i, f := range inputs {
		args[i] = f.src
	}
	cmd := docgenCmd(t, args...)
	cmd.Dir = wd
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("docgen failed: %v\noutput: %s", err, out)
	}

	for _, f := range inputs {
		want := filepath.Join(wd, f.want)
		if _, err := os.Stat(want); os.IsNotExist(err) {
			t.Errorf("output file not found: %s", want)
		}
	}
}

// TestNoArguments (scenario 5) verifies that invoking docgen with no arguments
// exits non-zero and prints a usage message to stderr.
func TestNoArguments(t *testing.T) {
	var stdout, stderr strings.Builder
	cmd := docgenCmd(t) // no args
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err == nil {
		t.Fatal("expected non-zero exit, got 0")
	}

	if stdout.Len() != 0 {
		t.Errorf("expected empty stdout, got: %s", stdout.String())
	}
	if !strings.Contains(stderr.String(), "usage:") {
		t.Errorf("expected stderr to contain %q, got: %s", "usage:", stderr.String())
	}
}

// TestMissingFile (scenario 6 — basic) verifies that a non-existent input path
// causes docgen to exit non-zero and write an error message to stderr.
func TestMissingFile(t *testing.T) {
	var stderr strings.Builder
	cmd := docgenCmd(t, "nonexistent/file.md")
	cmd.Stderr = &stderr

	if err := cmd.Run(); err == nil {
		t.Fatal("expected non-zero exit, got 0")
	}
	if !strings.Contains(stderr.String(), "error:") {
		t.Errorf("expected stderr to contain %q, got: %s", "error:", stderr.String())
	}
}

// TestMissingFileMixedInput (scenario 6 — extended) verifies that when one
// valid and one missing file are passed together, docgen still processes the
// valid file, reports the error for the missing one, and exits non-zero.
func TestMissingFileMixedInput(t *testing.T) {
	wd := t.TempDir()

	if err := os.MkdirAll(filepath.Join(wd, "subdir"), 0o755); err != nil {
		t.Fatalf("mkdir subdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(wd, "subdir", "valid.md"), []byte("# Valid\n"), 0o644); err != nil {
		t.Fatalf("write valid.md: %v", err)
	}

	var stderr strings.Builder
	cmd := docgenCmd(t, filepath.Join("subdir", "valid.md"), filepath.Join("nonexistent", "missing.md"))
	cmd.Dir = wd
	cmd.Stderr = &stderr

	if err := cmd.Run(); err == nil {
		t.Fatal("expected non-zero exit, got 0")
	}
	if !strings.Contains(stderr.String(), "error:") {
		t.Errorf("expected stderr to contain %q, got: %s", "error:", stderr.String())
	}
	want := filepath.Join(wd, "site", "subdir", "valid.html")
	if _, err := os.Stat(want); os.IsNotExist(err) {
		t.Errorf("valid output file not found: %s", want)
	}
}
