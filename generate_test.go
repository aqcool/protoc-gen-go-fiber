package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateTestdata(t *testing.T) {
	root := findModuleRoot(t)
	protoInclude := findProtoInclude(t)

	outPath := filepath.Join(root, "testdata", "api", "path", "v1", "path_fiber.pb.go")

	cmd := exec.Command("protoc",
		"-I", filepath.Join(root, "testdata", "api"),
		"-I", filepath.Join(root, "third_party"),
		"-I", protoInclude,
		"--go-fiber_out="+filepath.Join(root, "testdata", "api"),
		"--go-fiber_opt=paths=source_relative",
		"path/v1/path.proto",
	)
	cmd.Dir = root
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("protoc failed: %v\n%s", err, output)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	for _, want := range []string{
		`RegisterBooksHTTPServer(r v3.Router`,
		`/v1/books/:book_id`,
		`binding.SetURIParam(&in, "book.id"`,
		`/v1/members/:name`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("generated file missing %q:\n%s", want, got)
		}
	}
}

func findModuleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}

func findProtoInclude(t *testing.T) string {
	t.Helper()
	candidates := []string{
		"/opt/homebrew/include",
		"/usr/local/include",
	}
	for _, dir := range candidates {
		if _, err := os.Stat(filepath.Join(dir, "google", "protobuf", "any.proto")); err == nil {
			return dir
		}
	}
	t.Fatal("google/protobuf/any.proto include path not found")
	return ""
}
