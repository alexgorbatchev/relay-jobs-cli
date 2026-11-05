package main

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alexgorbatchev/relay-jobs-cli/internal/job"
)

func TestSocketServerWithDeepTMPDIR(t *testing.T) {
	deep := filepath.Join(t.TempDir(), strings.Repeat("nested-", 24))
	if err := os.Mkdir(deep, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TMPDIR", deep)
	var socket string
	t.Run("request", func(t *testing.T) {
		socket = socketServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}))
		rpc := job.NewRPC(socket)
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
