package local

import (
	"testing"
	"time"
)

// Two processes on one store: the second writer waits for the first instead
// of failing with "database is locked".
func TestSecondWriterWaitsForFirst(t *testing.T) {
	root := t.TempDir()
	s1, err := OpenStore(root)
	if err != nil {
		t.Fatal(err)
	}
	defer s1.DB.Close()
	s2, err := OpenStore(root)
	if err != nil {
		t.Fatal(err)
	}
	defer s2.DB.Close()

	tx, err := s1.DB.Begin()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(`INSERT INTO ValidPaths(path, narHash, narSize) VALUES ('/nix/store/aaaa-one', 'h', 1)`); err != nil {
		t.Fatal(err)
	}
	go func() {
		time.Sleep(300 * time.Millisecond)
		_ = tx.Commit()
	}()
	if err := s2.registerPath("/nix/store/bbbb-two", "h", 1, nil); err != nil {
		t.Fatalf("second writer: %v", err)
	}
}

// Garbage collection waits for a running build to release gc.lock.
func TestCollectGarbageWaitsForBuild(t *testing.T) {
	root := t.TempDir()
	st, err := OpenStore(root)
	if err != nil {
		t.Fatal(err)
	}
	defer st.DB.Close()
	unlock, err := st.LockForBuild()
	if err != nil {
		t.Fatal(err)
	}
	released := make(chan time.Time, 1)
	go func() {
		time.Sleep(300 * time.Millisecond)
		released <- time.Now()
		unlock()
	}()
	if _, err := st.CollectGarbage(); err != nil {
		t.Fatal(err)
	}
	done := time.Now()
	if rel := <-released; done.Before(rel) {
		t.Fatal("garbage collection ran while a build held gc.lock")
	}
}
