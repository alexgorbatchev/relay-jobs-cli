package job

import (
	"context"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestEndpointWithDeepTMPDIR(t *testing.T) {
	deep := filepath.Join(t.TempDir(), strings.Repeat("nested-", 24))
	if err := os.Mkdir(deep, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TMPDIR", deep)
	var socket string
	t.Run("request", func(t *testing.T) {
		socket = endpoint(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}))
		rpc := NewRPC(socket)
		defer rpc.Close()
		if err := rpc.Call(context.Background(), http.MethodGet, "/", nil, nil); err != nil {
			t.Fatal(err)
		}
	})
	if _, err := os.Stat(filepath.Dir(socket)); !os.IsNotExist(err) {
		t.Fatalf("socket directory survived test cleanup: %v", err)
	}
	if entries, err := os.ReadDir(deep); err != nil || len(entries) != 0 {
		t.Fatalf("TMPDIR contains leftovers: %v, %v", entries, err)
	}
}

func TestSocketFixtureFailureCleanup(t *testing.T) {
	manifest := filepath.Join(t.TempDir(), "socket-directory")
	cmd := exec.Command(os.Args[0], "-test.run=^TestSocketFixtureFailureProcess$")
	cmd.Env = append(os.Environ(), "RELAY_JOBS_TEST_BINARY="+testBinary, "RELAY_JOBS_FIXTURE_MANIFEST="+manifest)
	out, err := cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(out), "intentional fixture failure") {
		t.Fatalf("expected failing fixture process: %v\n%s", err, out)
	}
	dir, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(string(dir)); !os.IsNotExist(err) {
		t.Fatalf("failed fixture left its directory: %v", err)
	}
}

func TestSocketFixtureFailureProcess(t *testing.T) {
	manifest := os.Getenv("RELAY_JOBS_FIXTURE_MANIFEST")
	if manifest == "" {
		return
	}
	dir := socketTestDir(t)
	if err := os.WriteFile(manifest, []byte(dir), 0600); err != nil {
		t.Fatal(err)
	}
	// Reproduce failure after lock creation but before the endpoint can listen.
	lock, err := Lock(filepath.Join(dir, "api.lock"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { lock.Close() })
	t.Fatal("intentional fixture failure")
}
