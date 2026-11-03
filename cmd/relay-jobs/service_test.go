package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/alexgorbatchev/relay-jobs-cli/internal/job"
)

func TestServeCommands(t *testing.T) {
	requireNativeManager(t)
	w := &job.Worker{Store: job.Store{Dir: t.TempDir()}, Cache: t.TempDir(), Binary: integrationBinary, Slots: 1}
	workerSocket := socketServer(t, w.Handler())
	config := filepath.Join(t.TempDir(), "relay-jobs.toml")
	if err := writeTOML(config, job.Config{Workers: []job.WorkerConfig{{Name: "test", Socket: workerSocket}}}); err != nil {
		t.Fatal(err)
	}
	for _, role := range []string{"worker", "coordinator"} {
		t.Run(role, func(t *testing.T) {
			dir, err := os.MkdirTemp("", "rj-serve-")
			if err != nil {
				t.Fatal(err)
			}
			defer os.RemoveAll(dir)
			socket := filepath.Join(dir, "api")
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			type exit struct {
				stderr string
				code   int
			}
			done := make(chan exit, 1)
			go func() {
				_, stderr, code := execute(ctx, "--socket", socket, "--state", filepath.Join(dir, "state"), "--cache", filepath.Join(dir, "cache"), "--config", config, role, "serve")
				done <- exit{stderr, code}
			}()
			// Serving starts after the rsync check, which runs an external process.
			watchdog := time.After(30 * time.Second)
			for {
				if _, err = os.Stat(socket); err == nil {
					break
				}
				select {
				case result := <-done:
					t.Fatalf("serve exited before listening: %d %s", result.code, result.stderr)
				case <-watchdog:
					t.Fatal("serve did not listen")
				case <-time.After(time.Millisecond):
				}
			}
			rpc := job.NewRPC(socket)
			defer rpc.Close()
			path := "/health"
			if role == "coordinator" {
				path = "/workers"
			}
			var reply any
			if err = rpc.Call(context.Background(), "GET", path, nil, &reply); err != nil {
				t.Fatal(err)
			}
			cancel()
			if result := <-done; result.code != 0 {
				t.Fatal(result.code, result.stderr)
			}
		})
	}
	for _, args := range [][]string{{"worker", "serve", "--slots", "0"}, {"coordinator", "serve", "--config", "/missing/rj-config"}, {"execution", "run", "--id", "bad"}} {
		if _, _, code := execute(context.Background(), args...); code == 0 {
			t.Fatal(args)
		}
	}
}
