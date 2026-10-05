package canvas

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func encodedDocument(t *testing.T, d *Document) string {
	t.Helper()
	data, err := json.Marshal(fileDocument{Format: fileFormat, Document: *d})
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "drawing.tdoodle")
	d := mustDocument(t, 10, 2)
	for i, color := range Colors {
		d.Set(Point{i, 0}, Cell{Rune: rune('A' + i), FG: color, BG: Colors[9-i]})
		d.Set(Point{i, 1}, Cell{Rune: ' ', FG: color, BG: color})
	}
	if err := Save(path, d); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(loaded, d) {
		t.Fatalf("round trip changed document: %#v", loaded)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"format": "tdoodle"`) {
		t.Fatal("missing format identifier")
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("temporary file left behind: %v, %v", entries, err)
	}
}

func TestLoadRejectsMalformedFiles(t *testing.T) {
	d := mustDocument(t, 1, 1)
	valid := encodedDocument(t, d)
	for _, tc := range []struct {
		name string
		data string
	}{
		{"empty", ""},
		{"truncated", valid[:len(valid)-1]},
		{"non object", "[]"},
		{"null", "null"},
		{"unknown format", strings.Replace(valid, `"tdoodle"`, `"other"`, 1)},
		{"missing format", strings.Replace(valid, `"format":"tdoodle",`, "", 1)},
		{"future version", strings.Replace(valid, `"version":1`, `"version":2`, 1)},
		{"zero dimension", strings.Replace(valid, `"width":1`, `"width":0`, 1)},
		{"overflow dimension", strings.Replace(valid, `"width":1`, `"width":999999999999999999999999`, 1)},
		{"missing cells", strings.Replace(valid, `[{"rune":32,"fg":"white","bg":"black"}]`, `[]`, 1)},
		{"null cells", strings.Replace(valid, `[{"rune":32,"fg":"white","bg":"black"}]`, `null`, 1)},
		{"unicode", strings.Replace(valid, `"rune":32`, `"rune":233`, 1)},
		{"control", strings.Replace(valid, `"rune":32`, `"rune":10`, 1)},
		{"unknown color", strings.Replace(valid, `"white"`, `"purple"`, 1)},
		{"unknown field", strings.Replace(valid, `"version":1`, `"version":1,"extra":true`, 1)},
		{"unknown cell field", strings.Replace(valid, `"rune":32`, `"rune":32,"extra":true`, 1)},
		{"duplicate field", strings.Replace(valid, `"width":1`, `"width":1,"width":1`, 1)},
		{"second value", valid + "{}"},
		{"trailing garbage", valid + "garbage"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "bad.tdoodle")
			if err := os.WriteFile(path, []byte(tc.data), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := Load(path); err == nil {
				t.Fatalf("malformed drawing accepted: %s", tc.data)
			}
		})
	}
}

func TestLoadReportsNotExist(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "missing"))
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing file error lost: %v", err)
	}
}

func TestLoadRejectsDirectory(t *testing.T) {
	if _, err := Load(t.TempDir()); err == nil {
		t.Fatal("directory accepted as a drawing")
	}
}

func TestSaveFailurePreservesOriginal(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "existing")
	original := []byte("untouched contents")
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	d := mustDocument(t, 1, 1)
	d.Cells[0].Rune = '\n'
	if err := Save(path, d); err == nil {
		t.Fatal("invalid drawing saved")
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != string(original) {
		t.Fatalf("failed save changed original: %q, %v", data, err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("failed save left a temporary file: %v, %v", entries, err)
	}
}

func TestSaveRejectsSymlink(t *testing.T) {
	dir := t.TempDir()
	original := filepath.Join(dir, "original")
	link := filepath.Join(dir, "link")
	if err := os.WriteFile(original, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(original, link); err != nil {
		t.Fatal(err)
	}
	if err := Save(link, mustDocument(t, 1, 1)); err == nil {
		t.Fatal("save replaced symlink")
	}
	data, err := os.ReadFile(original)
	if err != nil || string(data) != "keep" {
		t.Fatalf("save damaged symlink target: %q, %v", data, err)
	}
	info, err := os.Lstat(link)
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("save damaged symlink: %v, %v", info, err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 2 {
		t.Fatalf("failed save left a temporary file: %v, %v", entries, err)
	}
}

func TestSavePreservesPermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "drawing")
	if err := os.WriteFile(path, []byte("old"), 0640); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0640); err != nil {
		t.Fatal(err)
	}
	if err := Save(path, mustDocument(t, 1, 1)); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0640 {
		t.Fatalf("permissions changed: %v, %v", info, err)
	}
}

func TestSaveRejectsInvalidTargets(t *testing.T) {
	dir := t.TempDir()
	d := mustDocument(t, 1, 1)
	for _, path := range []string{"", dir, filepath.Join(dir, "missing", "drawing")} {
		if err := Save(path, d); err == nil {
			t.Errorf("save accepted invalid target %q", path)
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("failed save left files: %v, %v", entries, err)
	}
}

func TestRecoveryPath(t *testing.T) {
	if got := RecoveryPath("art.tdoodle"); got != "art.tdoodle.recovery" {
		t.Fatalf("recovery path = %q", got)
	}
}
