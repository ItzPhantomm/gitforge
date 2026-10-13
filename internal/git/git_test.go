package git

import (
	"os"
	"path/filepath"
	"testing"
)

func setupTestRepo(t *testing.T) (*Repo, string) {
	t.Helper()
	dir, err := os.MkdirTemp("", "gitforge-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	repo, err := InitRepo(dir, false)
	if err != nil {
		os.RemoveAll(dir)
		t.Fatalf("failed to init repo: %v", err)
	}

	// Configure test user for commits
	_ = repo.SetConfig("user.name", "Test User")
	_ = repo.SetConfig("user.email", "test@example.com")

	return repo, dir
}

func TestInitAndOpenRepo(t *testing.T) {
	repo, dir := setupTestRepo(t)
	defer os.RemoveAll(dir)

	if repo.Workdir() == "" {
		t.Errorf("expected non-empty workdir")
	}

	opened, err := OpenRepo(dir)
	if err != nil {
		t.Fatalf("failed to open repo: %v", err)
	}
	if opened.Workdir() != repo.Workdir() {
		t.Errorf("expected %s, got %s", repo.Workdir(), opened.Workdir())
	}
}

func TestStatusAndCommit(t *testing.T) {
	repo, dir := setupTestRepo(t)
	defer os.RemoveAll(dir)

	status, err := repo.Status()
	if err != nil {
		t.Fatalf("status error: %v", err)
	}
	if !status.IsClean() {
		t.Errorf("expected clean status initially")
	}

	// Create an untracked file
	testFile := filepath.Join(dir, "hello.txt")
	if err := os.WriteFile(testFile, []byte("hello world"), 0644); err != nil {
		t.Fatalf("write file error: %v", err)
	}

	status, err = repo.Status()
	if err != nil {
		t.Fatalf("status error: %v", err)
	}
	if len(status.Untracked) != 1 || status.Untracked[0] != "hello.txt" {
		t.Errorf("expected untracked hello.txt, got %v", status.Untracked)
	}

	// Stage the file
	if err := repo.Add("hello.txt"); err != nil {
		t.Fatalf("add error: %v", err)
	}

	status, err = repo.Status()
	if err != nil {
		t.Fatalf("status error: %v", err)
	}
	if len(status.Staged) != 1 || status.Staged[0] != "hello.txt" {
		t.Errorf("expected staged hello.txt, got %v", status.Staged)
	}

	// Commit
	hash, err := repo.Commit("initial commit", nil)
	if err != nil {
		t.Fatalf("commit error: %v", err)
	}
	if hash == "" {
		t.Errorf("expected non-empty commit hash")
	}

	// Log
	commits, err := repo.Log(&LogOptions{Limit: 5})
	if err != nil {
		t.Fatalf("log error: %v", err)
	}
	if len(commits) != 1 {
		t.Fatalf("expected 1 commit, got %d", len(commits))
	}
	if commits[0].Subject != "initial commit" {
		t.Errorf("expected subject 'initial commit', got %s", commits[0].Subject)
	}
}

func TestBranches(t *testing.T) {
	repo, dir := setupTestRepo(t)
	defer os.RemoveAll(dir)

	// Make an initial commit
	testFile := filepath.Join(dir, "file.txt")
	_ = os.WriteFile(testFile, []byte("data"), 0644)
	_ = repo.Add("file.txt")
	_, err := repo.Commit("first", nil)
	if err != nil {
		t.Fatalf("commit failed: %v", err)
	}

	branches, err := repo.Branches()
	if err != nil {
		t.Fatalf("branches failed: %v", err)
	}
	if len(branches) == 0 {
		t.Fatalf("expected at least 1 branch")
	}

	// Create branch
	if err := repo.CreateBranch("feature-test", ""); err != nil {
		t.Fatalf("create branch failed: %v", err)
	}

	branches, err = repo.Branches()
	if err != nil {
		t.Fatalf("branches failed: %v", err)
	}
	found := false
	for _, b := range branches {
		if b.Name == "feature-test" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected feature-test branch")
	}

	// Delete branch
	if err := repo.DeleteBranch("feature-test", false); err != nil {
		t.Fatalf("delete branch failed: %v", err)
	}
}

func TestTags(t *testing.T) {
	repo, dir := setupTestRepo(t)
	defer os.RemoveAll(dir)

	testFile := filepath.Join(dir, "file.txt")
	_ = os.WriteFile(testFile, []byte("data"), 0644)
	_ = repo.Add("file.txt")
	_, _ = repo.Commit("first", nil)

	if err := repo.CreateTag("v1.0.0", "HEAD", "release 1.0.0"); err != nil {
		t.Fatalf("create tag failed: %v", err)
	}

	tags, err := repo.Tags()
	if err != nil {
		t.Fatalf("tags failed: %v", err)
	}
	if len(tags) != 1 || tags[0].Name != "v1.0.0" {
		t.Errorf("expected tag v1.0.0, got %v", tags)
	}

	if err := repo.DeleteTag("v1.0.0"); err != nil {
		t.Fatalf("delete tag failed: %v", err)
	}
}
