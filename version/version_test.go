package version_test

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestVersionMain(t *testing.T) {
	dir, err := os.MkdirTemp("", "version.test-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Log("dir:", dir)
	defer os.RemoveAll(dir)
	const pkg = "github.com/tgulacsi/go/version-test"
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(`package main
import "`+pkg+`/version"
import "os"
func main() {
	os.Stdout.Write([]byte(version.Main()))
}
`), 0644); err != nil {
		t.Fatal(err)
	}
	os.Mkdir(filepath.Join(dir, "version"), 0755)
	if b, err := exec.Command("cp", "-a", "version.go", filepath.Join(dir, "version")).CombinedOutput(); err != nil {
		t.Fatalf("%s: %+v", string(b), err)
	}
	exe := filepath.Join(dir, "exe")
	for _, ss := range [][]string{
		{"go", "mod", "init", pkg},
		{"git", "init"}, {"git", "add", "."}, {"git", "commit", "-am", "initial commit"},
		{"go", "build", "-o", exe},
	} {
		if _, err := runAt(dir, exec.Command(ss[0], ss[1:]...)); err != nil {
			t.Fatal(err)
		}
	}
	check := func(name string) {
		t.Run(name, func(t *testing.T) {
			b, err := exec.Command(exe).CombinedOutput()
			if err != nil {
				t.Fatal(err)
			}
			t.Log(string(b))
			if bytes.HasSuffix(bytes.TrimSpace(b), []byte("@-")) {
				t.Errorf("got %s, wanted 0.0.0-xxxx", string(b))
			}
		})
	}
	check("raw")

	if b, err := exec.Command("upx", "-2", exe).CombinedOutput(); err != nil {
		t.Fatalf("%s: %+v", string(b), err)
	} else {
		t.Logf("UPX: %s", string(b))
	}
	check("upx")
}

func runAt(dir string, cmd *exec.Cmd) (string, error) {
	cmd.Dir = dir
	b, err := cmd.CombinedOutput()
	s := string(b)
	if err != nil {
		err = fmt.Errorf("%q: %s: %+v", cmd.Args, s, err)
	}
	return s, err
}
