package main

import (
	"bytes"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"tdoodle/internal/canvas"
)

func TestParseOptions(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want options
	}{
		{"defaults", nil, options{aspect: 2, autosave: 15 * time.Second}},
		{"filename", []string{"drawing.tdoodle"}, options{path: "drawing.tdoodle", aspect: 2, autosave: 15 * time.Second}},
		{"flags", []string{"-aspect", "1.5", "-autosave", "30s", "drawing.tdoodle"}, options{path: "drawing.tdoodle", aspect: 1.5, autosave: 30 * time.Second}},
		{"disable recovery", []string{"-autosave=0"}, options{aspect: 2}},
		{"lower aspect boundary", []string{"-aspect=0.1"}, options{aspect: 0.1, autosave: 15 * time.Second}},
		{"upper aspect boundary", []string{"-aspect=10"}, options{aspect: 10, autosave: 15 * time.Second}},
		{"literal flag filename", []string{"--", "-art.tdoodle"}, options{path: "-art.tdoodle", aspect: 2, autosave: 15 * time.Second}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseOptions(tc.args, &bytes.Buffer{})
			if err != nil || got != tc.want {
				t.Fatalf("parseOptions = %#v, %v; want %#v", got, err, tc.want)
			}
		})
	}
}

func TestParseOptionsRejectsInvalidInput(t *testing.T) {
	for _, args := range [][]string{
		{"one", "two"},
		{""},
		{"-unknown"},
		{"-aspect=NaN"},
		{"-aspect=+Inf"},
		{"-aspect=-Inf"},
		{"-aspect=0"},
		{"-aspect=10.1"},
		{"-aspect=not-a-number"},
		{"-autosave=-1s"},
		{"-autosave=tomorrow"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			if _, err := parseOptions(args, &bytes.Buffer{}); err == nil {
				t.Fatalf("accepted invalid options %q", args)
			}
		})
	}
}

func TestParseOptionsHelp(t *testing.T) {
	var expected string
	for _, alias := range []string{"-h", "--help", "-help"} {
		t.Run(alias, func(t *testing.T) {
			var output bytes.Buffer
			_, err := parseOptions([]string{alias}, &output)
			if !errors.Is(err, flag.ErrHelp) {
				t.Fatalf("help returned %v, want flag.ErrHelp", err)
			}
			help := output.String()
			if expected == "" {
				expected = help
			} else if help != expected {
				t.Fatalf("help aliases produced different output: %q", help)
			}
			for _, want := range []string{
				"terminal drawing tool", "text, lines, rectangles, ovals, and freehand marks",
				"[filename]", "Filename (optional)", "Existing file", "Missing file",
				"timestamped .tdoodle file", "current directory", "F7", "Ctrl+C twice",
				"-aspect", "default 2", "-autosave", "default 15s", "-h, --help",
			} {
				if !strings.Contains(help, want) {
					t.Errorf("help is missing %q: %s", want, help)
				}
			}
		})
	}
}

func TestRunHelpReturnsWithoutOpeningTerminal(t *testing.T) {
	t.Setenv("TERM", "tdoodle-test-unavailable-terminal")
	for _, alias := range []string{"-h", "--help", "-help"} {
		t.Run(alias, func(t *testing.T) {
			var out, errOut bytes.Buffer
			if err := run([]string{alias}, &out, &errOut); err != nil {
				t.Fatalf("help should succeed without a terminal: %v", err)
			}
			if out.Len() != 0 || !strings.Contains(errOut.String(), "Filename (optional)") {
				t.Fatalf("unexpected help streams: stdout %q, stderr %q", out.String(), errOut.String())
			}
		})
	}
}

func TestGeneratedPath(t *testing.T) {
	now := time.Date(2026, time.October, 4, 12, 34, 56, 123456789, time.FixedZone("test", -5*60*60))
	path := generatedPath(now)
	if path != "tdoodle-20261004T123456.123456789.tdoodle" {
		t.Fatalf("unexpected generated filename %q", path)
	}
	if !regexp.MustCompile(`^tdoodle-[0-9]{8}T[0-9]{6}\.[0-9]{9}\.tdoodle$`).MatchString(path) {
		t.Fatalf("generated filename contains unsafe or unexpected characters: %q", path)
	}
	if filepath.Base(path) != path {
		t.Fatalf("generated filename contains a directory: %q", path)
	}
	if generatedPath(now.Add(time.Nanosecond)) == path {
		t.Fatal("distinct nanosecond timestamps collided")
	}
	if generatedPath(now) != path {
		t.Fatal("same timestamp was not deterministic")
	}
}

func testDrawing(t *testing.T, glyph rune) *canvas.Document {
	t.Helper()
	doc, err := canvas.New(2, 2, 2)
	if err != nil {
		t.Fatal(err)
	}
	doc.Set(canvas.Point{X: 0, Y: 0}, canvas.Cell{Rune: glyph, FG: canvas.Orange, BG: canvas.Blue})
	return doc
}

func saveDrawingAt(t *testing.T, path string, doc *canvas.Document, modified time.Time) {
	t.Helper()
	if err := canvas.Save(path, doc); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, modified, modified); err != nil {
		t.Fatal(err)
	}
}

func TestOpenDrawingExistingAndMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "art.tdoodle")
	got, message, err := openDrawing(path)
	if got != nil || message != "" || err != nil {
		t.Fatalf("missing drawing = %#v, %q, %v", got, message, err)
	}
	want := testDrawing(t, 'P')
	saveDrawingAt(t, path, want, time.Now())
	got, message, err = openDrawing(path)
	if err != nil || message != "" || !reflect.DeepEqual(got, want) {
		t.Fatalf("existing drawing = %#v, %q, %v", got, message, err)
	}
}

func TestOpenDrawingCorruptPrimaryRefusesToOverwrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "art.tdoodle")
	contents := []byte("corrupt primary remains available")
	if err := os.WriteFile(path, contents, 0600); err != nil {
		t.Fatal(err)
	}
	// A valid recovery stays available for explicit opening; startup must not
	// silently replace or hide the error in a corrupt primary drawing.
	saveDrawingAt(t, canvas.RecoveryPath(path), testDrawing(t, 'R'), time.Now().Add(time.Second))
	got, _, err := openDrawing(path)
	if err == nil || got != nil {
		t.Fatalf("corrupt primary accepted: %#v, %v", got, err)
	}
	unchanged, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(unchanged, contents) {
		t.Fatalf("corrupt primary changed: %q, %v", unchanged, err)
	}
}

func TestOpenDrawingNewerRecovery(t *testing.T) {
	path := filepath.Join(t.TempDir(), "art.tdoodle")
	old := time.Date(2026, time.October, 4, 12, 0, 0, 0, time.UTC)
	primary := testDrawing(t, 'P')
	recovery := testDrawing(t, 'R')
	saveDrawingAt(t, path, primary, old)
	saveDrawingAt(t, canvas.RecoveryPath(path), recovery, old.Add(time.Second))
	got, message, err := openDrawing(path)
	if err != nil || message != "Recovered unsaved drawing" || !reflect.DeepEqual(got, recovery) {
		t.Fatalf("newer recovery = %#v, %q, %v", got, message, err)
	}
	unchanged, err := canvas.Load(path)
	if err != nil || !reflect.DeepEqual(unchanged, primary) {
		t.Fatalf("loading recovery changed primary: %#v, %v", unchanged, err)
	}
}

func TestOpenDrawingRecoveryWithoutPrimary(t *testing.T) {
	path := filepath.Join(t.TempDir(), "art.tdoodle")
	want := testDrawing(t, 'R')
	saveDrawingAt(t, canvas.RecoveryPath(path), want, time.Now())
	got, message, err := openDrawing(path)
	if err != nil || message != "Recovered unsaved drawing" || !reflect.DeepEqual(got, want) {
		t.Fatalf("orphan recovery = %#v, %q, %v", got, message, err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("opening recovery created primary: %v", err)
	}
}

func TestOpenDrawingCorruptRecoveryKeepsOriginal(t *testing.T) {
	path := filepath.Join(t.TempDir(), "art.tdoodle")
	old := time.Date(2026, time.October, 4, 12, 0, 0, 0, time.UTC)
	primary := testDrawing(t, 'P')
	saveDrawingAt(t, path, primary, old)
	recoveryPath := canvas.RecoveryPath(path)
	corrupt := []byte("invalid recovery")
	if err := os.WriteFile(recoveryPath, corrupt, 0600); err != nil {
		t.Fatal(err)
	}
	newer := old.Add(time.Second)
	if err := os.Chtimes(recoveryPath, newer, newer); err != nil {
		t.Fatal(err)
	}
	got, message, err := openDrawing(path)
	if err != nil || message != "Recovery unreadable; kept original" || !reflect.DeepEqual(got, primary) {
		t.Fatalf("corrupt recovery = %#v, %q, %v", got, message, err)
	}
	data, err := os.ReadFile(recoveryPath)
	if err != nil || !bytes.Equal(data, corrupt) {
		t.Fatalf("opening drawing changed corrupt recovery: %q, %v", data, err)
	}
	unchanged, err := canvas.Load(path)
	if err != nil || !reflect.DeepEqual(unchanged, primary) {
		t.Fatalf("opening drawing changed primary: %#v, %v", unchanged, err)
	}
}

func TestOpenDrawingIgnoresOldRecovery(t *testing.T) {
	for _, offset := range []time.Duration{-time.Second, 0} {
		t.Run(offset.String(), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "art.tdoodle")
			now := time.Date(2026, time.October, 4, 12, 0, 0, 0, time.UTC)
			primary := testDrawing(t, 'P')
			saveDrawingAt(t, path, primary, now)
			saveDrawingAt(t, canvas.RecoveryPath(path), testDrawing(t, 'R'), now.Add(offset))
			got, message, err := openDrawing(path)
			if err != nil || message != "" || !reflect.DeepEqual(got, primary) {
				t.Fatalf("old recovery = %#v, %q, %v", got, message, err)
			}
		})
	}
}
