package store

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestMigrationFilesUsesOnlyTopLevelSQLFiles(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "011_next_change.sql"), []byte("SELECT 1;"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "README.md"), []byte("documentation"), 0o600); err != nil {
		t.Fatal(err)
	}
	legacy := filepath.Join(directory, "legacy")
	if err := os.Mkdir(legacy, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "001_initial.sql"), []byte("SELECT 1;"), 0o600); err != nil {
		t.Fatal(err)
	}

	files, err := migrationFiles(directory)
	if err != nil {
		t.Fatalf("migrationFiles() error = %v", err)
	}
	if want := []string{"011_next_change.sql"}; !reflect.DeepEqual(files, want) {
		t.Fatalf("migration files = %#v, want %#v", files, want)
	}
}

func TestCanonicalSchemaIsCreateOnlyAndContainsFinalCommandState(t *testing.T) {
	contents, err := os.ReadFile(filepath.Join("..", "..", "db", "schema.sql"))
	if err != nil {
		t.Fatalf("read canonical schema: %v", err)
	}
	schema := string(contents)
	if strings.Contains(schema, "ALTER TABLE") {
		t.Fatal("canonical schema must contain final CREATE definitions, not migration alterations")
	}
	for _, definition := range []string{
		"CREATE TABLE terminals",
		"CREATE TABLE access_event_projections",
		"CREATE TABLE isapi_commands",
		"response_data_format_declared BOOLEAN",
		"isapi_commands_response_state_check",
		"status IN ('queued', 'sent', 'completed', 'expired')",
		"CREATE TABLE access_event_sync_runs",
		"CREATE TABLE retained_access_events",
	} {
		if !strings.Contains(schema, definition) {
			t.Fatalf("canonical schema lacks %q", definition)
		}
	}
}
