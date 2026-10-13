package backups

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUnconfiguredAdapterFails(t *testing.T) {
	if _, _, err := (PGDump{}).Run(context.Background(), "test"); err == nil {
		t.Fatal("unconfigured backup must not succeed")
	}
}
func TestPGDumpLive(t *testing.T) {
	url, binary := os.Getenv("TEST_DATABASE_URL"), os.Getenv("TEST_PG_DUMP_PATH")
	if url == "" || binary == "" {
		t.Skip("set TEST_DATABASE_URL and TEST_PG_DUMP_PATH")
	}
	dir := t.TempDir()
	size, detail, err := (PGDump{Binary: binary, Directory: dir, DatabaseURL: url}).Run(context.Background(), "test")
	if err != nil {
		t.Fatal(err)
	}
	if size == 0 || !strings.Contains(detail, "sha256=") {
		t.Fatal("missing snapshot/checksum")
	}
	files, _ := filepath.Glob(filepath.Join(dir, "*.dump"))
	if len(files) != 1 {
		t.Fatal("snapshot missing")
	}
	file, _ := os.ReadFile(files[0])
	if string(file[:5]) != "PGDMP" {
		t.Fatal("invalid PostgreSQL archive")
	}
}
