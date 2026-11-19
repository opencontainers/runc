package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func listTestEntries(t *testing.T, count int) (string, []os.DirEntry) {
	t.Helper()
	root := t.TempDir()
	for i := range count {
		if err := os.Mkdir(filepath.Join(root, fmt.Sprintf("item-%03d", i)), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	return root, entries
}

func TestCollectContainerStatesOrder(t *testing.T) {
	_, entries := listTestEntries(t, 200)
	releaseFirst := make(chan struct{})
	var warnings bytes.Buffer
	states, err := collectContainerStates(entries, func(item os.DirEntry) listResult {
		switch item.Name() {
		case "item-000":
			<-releaseFirst
		case "item-007":
			close(releaseFirst)
		}
		return listResult{state: &containerState{ID: item.Name()}}
	}, &warnings)
	if err != nil {
		t.Fatal(err)
	}
	if len(states) != 200 {
		t.Fatalf("got %d states, want 200", len(states))
	}
	for i, state := range states {
		want := fmt.Sprintf("item-%03d", i)
		if state.ID != want {
			t.Fatalf("state %d has ID %q, want %q", i, state.ID, want)
		}
	}
	if warnings.Len() != 0 {
		t.Fatalf("unexpected warnings: %s", warnings.String())
	}
}

func TestCollectContainerStatesWarningsAndError(t *testing.T) {
	_, entries := listTestEntries(t, 16)
	releaseFirst := make(chan struct{})
	var calls atomic.Int32
	var warnings bytes.Buffer
	wantErr := errors.New("fatal directory error")
	states, err := collectContainerStates(entries, func(item os.DirEntry) listResult {
		calls.Add(1)
		switch item.Name() {
		case "item-000":
			<-releaseFirst
			return listResult{warning: "first\n"}
		case "item-001":
			return listResult{warning: "second\n"}
		case "item-007":
			close(releaseFirst)
		case "item-010":
			return listResult{err: wantErr}
		case "item-011":
			return listResult{warning: "after error\n"}
		}
		return listResult{}
	}, &warnings)
	if !errors.Is(err, wantErr) {
		t.Fatalf("got error %v, want %v", err, wantErr)
	}
	if states != nil {
		t.Fatalf("got states %v, want nil", states)
	}
	if got := warnings.String(); got != "first\nsecond\n" {
		t.Fatalf("got warnings %q", got)
	}
	if got := calls.Load(); got != int32(len(entries)) {
		t.Fatalf("processed %d entries, want %d", got, len(entries))
	}
}

func TestCollectContainerStatesEmpty(t *testing.T) {
	var calls atomic.Int32
	var warnings bytes.Buffer
	states, err := collectContainerStates(nil, func(os.DirEntry) listResult {
		calls.Add(1)
		return listResult{}
	}, &warnings)
	if err != nil || states != nil || warnings.Len() != 0 || calls.Load() != 0 {
		t.Fatalf("got states %v, error %v, warnings %q, calls %d", states, err, warnings.String(), calls.Load())
	}
}

func TestOwnerLookupOncePerUID(t *testing.T) {
	var calls atomic.Int32
	lookup := newOwnerLookup(func(id string) (*user.User, error) {
		calls.Add(1)
		time.Sleep(time.Millisecond)
		if id == "42" {
			return &user.User{Username: "owner"}, nil
		}
		return nil, errors.New("unknown UID")
	})
	var workers sync.WaitGroup
	for range 100 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			if got := lookup(42); got != "owner" {
				t.Errorf("got owner %q", got)
			}
		}()
	}
	workers.Wait()
	if got := calls.Load(); got != 1 {
		t.Fatalf("looked up UID 42 %d times, want one", got)
	}
	for range 2 {
		if got := lookup(43); got != "#43" {
			t.Fatalf("got fallback owner %q", got)
		}
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("looked up two UIDs %d times, want two", got)
	}
}

func TestGetContainerStateInvalidAndDeleted(t *testing.T) {
	root, entries := listTestEntries(t, 1)
	lookup := func(uint32) string { return "owner" }
	result := getContainerState(root, entries[0], lookup)
	if !strings.Contains(result.warning, "load container item-000:") || result.err != nil || result.state != nil {
		t.Fatalf("unexpected invalid state result: %+v", result)
	}
	if err := os.Remove(filepath.Join(root, entries[0].Name())); err != nil {
		t.Fatal(err)
	}
	result = getContainerState(root, entries[0], lookup)
	if result.warning != "" || result.err != nil || result.state != nil {
		t.Fatalf("unexpected deleted state result: %+v", result)
	}
}

func TestCollectContainerStatesInvalidStateWarningOrder(t *testing.T) {
	root, entries := listTestEntries(t, 16)
	for _, entry := range entries {
		path := filepath.Join(root, entry.Name(), "state.json")
		if err := os.WriteFile(path, []byte("{invalid"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	var warnings bytes.Buffer
	states, err := collectContainerStates(entries, func(item os.DirEntry) listResult {
		return getContainerState(root, item, func(uint32) string { return "owner" })
	}, &warnings)
	if err != nil || states != nil {
		t.Fatalf("got states %v, error %v", states, err)
	}
	lines := strings.Split(strings.TrimSuffix(warnings.String(), "\n"), "\n")
	if len(lines) != len(entries) {
		t.Fatalf("got %d warnings, want %d", len(lines), len(entries))
	}
	for i, line := range lines {
		if want := "load container " + entries[i].Name() + ":"; !strings.HasPrefix(line, want) {
			t.Fatalf("warning %d is %q, want prefix %q", i, line, want)
		}
	}
}

func TestCollectContainerStatesConcurrentDeletion(t *testing.T) {
	root, entries := listTestEntries(t, 16)
	var warnings bytes.Buffer
	changeErr := make(chan error, 1)
	go func() {
		for i := range 100 {
			path := filepath.Join(root, fmt.Sprintf("item-%03d", i%len(entries)))
			if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
				changeErr <- err
				return
			}
			if err := os.Mkdir(path, 0o700); err != nil {
				changeErr <- err
				return
			}
		}
		changeErr <- nil
	}()
	var listErr error
	for range 10 {
		_, err := collectContainerStates(entries, func(item os.DirEntry) listResult {
			return getContainerState(root, item, func(uint32) string { return "owner" })
		}, &warnings)
		if err != nil {
			listErr = err
			break
		}
	}
	if err := <-changeErr; err != nil {
		t.Fatal(err)
	}
	if listErr != nil {
		t.Fatal(listErr)
	}
}
