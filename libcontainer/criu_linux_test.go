//go:build !runc_nocriu

package libcontainer

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestReadRegularFileAt(t *testing.T) {
	tmpDir := t.TempDir()
	dir, err := os.Open(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	defer dir.Close()

	regularName := "regular.json"
	regularContent := []byte("{}")
	if err := os.WriteFile(filepath.Join(tmpDir, regularName), regularContent, 0o600); err != nil {
		t.Fatal(err)
	}

	fifoName := "fifo.json"
	fifoPath := filepath.Join(tmpDir, fifoName)
	if err := unix.Mkfifo(fifoPath, 0o600); err != nil {
		t.Fatal(err)
	}

	symlinkName := "symlink.json"
	if err := os.Symlink(regularName, filepath.Join(tmpDir, symlinkName)); err != nil {
		t.Fatal(err)
	}

	if err := os.Mkdir(filepath.Join(tmpDir, "directory.json"), 0o700); err != nil {
		t.Fatal(err)
	}

	oversizedName := "oversized.json"
	oversized, err := os.Create(filepath.Join(tmpDir, oversizedName))
	if err != nil {
		t.Fatal(err)
	}
	if err := oversized.Truncate(maxRegularFileSize + 1); err != nil {
		t.Fatal(err)
	}
	if err := oversized.Close(); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		file    string
		want    []byte
		wantErr bool
	}{
		{name: "regular file", file: regularName, want: regularContent},
		{name: "FIFO", file: fifoName, wantErr: true},
		{name: "symbolic link", file: symlinkName, wantErr: true},
		{name: "directory", file: "directory.json", wantErr: true},
		{name: "oversized file", file: oversizedName, wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			type result struct {
				data []byte
				err  error
			}
			resultCh := make(chan result, 1)
			go func() {
				data, err := readRegularFileAt(dir, test.file)
				resultCh <- result{data: data, err: err}
			}()

			select {
			case result := <-resultCh:
				if test.wantErr {
					if result.err == nil {
						t.Fatal("expected an error, got nil")
					}
					return
				}
				if result.err != nil {
					t.Fatal(result.err)
				}
				if !bytes.Equal(result.data, test.want) {
					t.Fatalf("expected %q, got %q", test.want, result.data)
				}
			case <-time.After(time.Second):
				t.Fatal("readRegularFileAt blocked")
			}
		})
	}
}
