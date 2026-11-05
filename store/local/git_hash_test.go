package local

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGitObjectHashMatchesGit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	dir := t.TempDir()
	file := filepath.Join(dir, "hello.txt")
	if err := os.WriteFile(file, []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := GitObjectHash(file, "sha1")
	if err != nil {
		t.Fatal(err)
	}
	want := gitHashObjectCLI(t, file)
	if got != want {
		t.Fatalf("file git hash go=%s git=%s", got, want)
	}

	sub := filepath.Join(dir, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "b"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a"), []byte("hello\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("a", filepath.Join(dir, "link")); err != nil {
		t.Fatal(err)
	}
	got, err = GitObjectHash(dir, "sha1")
	if err != nil {
		t.Fatal(err)
	}
	want = gitWriteTreeCLI(t, dir)
	if got != want {
		t.Fatalf("tree git hash go=%s git=%s", got, want)
	}
}

func gitHashObjectCLI(t *testing.T, path string) string {
	t.Helper()
	out, err := exec.Command("git", "hash-object", path).Output()
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(out))
}

func gitWriteTreeCLI(t *testing.T, dir string) string {
	t.Helper()
	repo := t.TempDir()
	cmd := exec.Command("git", "init")
	cmd.Dir = repo
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	// Copy the tree into the repo so git add sees the same modes.
	copyTreeForGit(t, dir, repo)
	add := exec.Command("git", "add", "-A")
	add.Dir = repo
	add.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	if out, err := add.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v %s", err, out)
	}
	wt := exec.Command("git", "write-tree")
	wt.Dir = repo
	out, err := wt.Output()
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(out))
}

func copyTreeForGit(t *testing.T, src, dst string) {
	t.Helper()
	entries, err := os.ReadDir(src)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		s := filepath.Join(src, e.Name())
		d := filepath.Join(dst, e.Name())
		fi, err := os.Lstat(s)
		if err != nil {
			t.Fatal(err)
		}
		if fi.Mode()&os.ModeSymlink != 0 {
			target, err := os.Readlink(s)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(target, d); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if fi.IsDir() {
			if err := os.Mkdir(d, 0o755); err != nil {
				t.Fatal(err)
			}
			copyTreeForGit(t, s, d)
			continue
		}
		data, err := os.ReadFile(s)
		if err != nil {
			t.Fatal(err)
		}
		mode := os.FileMode(0o644)
		if fi.Mode()&0o111 != 0 {
			mode = 0o755
		}
		if err := os.WriteFile(d, data, mode); err != nil {
			t.Fatal(err)
		}
	}
}
