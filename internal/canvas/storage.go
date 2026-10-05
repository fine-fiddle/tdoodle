package canvas

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const fileFormat = "tdoodle"

// Each cell takes under 128 bytes in the saved representation. Bound the input
// too, so malformed files cannot allocate arbitrary amounts of memory.
const maxFileBytes = int64(MaxCells)*128 + 4096

type fileDocument struct {
	Format string `json:"format"`
	Document
}

func RecoveryPath(path string) string { return path + ".recovery" }

func Load(path string) (*Document, error) {
	// Opening a named pipe can block indefinitely, so reject nonregular input
	// before opening it. Stat again below to check the file we actually opened.
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("drawing is not a regular file")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err = f.Stat()
	if err != nil {
		return nil, fmt.Errorf("inspect drawing: %w", err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("drawing is not a regular file")
	}
	if info.Size() > maxFileBytes {
		return nil, fmt.Errorf("drawing exceeds the maximum file size")
	}
	decoder := json.NewDecoder(io.LimitReader(f, maxFileBytes+1))
	decoder.DisallowUnknownFields()
	stored, err := decodeDocument(decoder)
	if err != nil {
		return nil, fmt.Errorf("decode drawing: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return nil, fmt.Errorf("drawing contains trailing JSON")
		}
		return nil, fmt.Errorf("drawing contains trailing data: %w", err)
	}
	if stored.Format != fileFormat {
		return nil, fmt.Errorf("unsupported drawing format %q", stored.Format)
	}
	if err := stored.Document.Validate(); err != nil {
		return nil, fmt.Errorf("invalid drawing: %w", err)
	}
	return &stored.Document, nil
}

// Decode the cells one at a time, validating them immediately and bounding
// their count before allocation. A single whole-document Decode would first
// allocate every cell in a malformed array and only then validate its size.
func decodeDocument(decoder *json.Decoder) (fileDocument, error) {
	var stored fileDocument
	token, err := decoder.Token()
	if err != nil {
		return stored, err
	}
	if token != json.Delim('{') {
		return stored, fmt.Errorf("expected a document object")
	}
	seen := make(map[string]bool)
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return stored, err
		}
		key, ok := token.(string)
		if !ok {
			return stored, fmt.Errorf("expected a document field")
		}
		if seen[key] {
			return stored, fmt.Errorf("duplicate document field %q", key)
		}
		seen[key] = true
		switch key {
		case "format":
			err = decoder.Decode(&stored.Format)
		case "version":
			err = decoder.Decode(&stored.Version)
		case "width":
			err = decoder.Decode(&stored.Width)
		case "height":
			err = decoder.Decode(&stored.Height)
		case "aspect":
			err = decoder.Decode(&stored.Aspect)
		case "cells":
			stored.Cells, err = decodeCells(decoder)
		default:
			return stored, fmt.Errorf("unknown document field %q", key)
		}
		if err != nil {
			return stored, fmt.Errorf("field %q: %w", key, err)
		}
	}
	if _, err := decoder.Token(); err != nil {
		return stored, err
	}
	for _, key := range []string{"format", "version", "width", "height", "aspect", "cells"} {
		if !seen[key] {
			return stored, fmt.Errorf("missing document field %q", key)
		}
	}
	return stored, nil
}

func decodeCells(decoder *json.Decoder) ([]Cell, error) {
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	if token != json.Delim('[') {
		return nil, fmt.Errorf("expected an array")
	}
	var cells []Cell
	for decoder.More() {
		if len(cells) >= MaxCells {
			return nil, fmt.Errorf("canvas exceeds %d cells", MaxCells)
		}
		var c Cell
		if err := decoder.Decode(&c); err != nil {
			return nil, err
		}
		if err := c.validate(); err != nil {
			return nil, fmt.Errorf("cell %d: %w", len(cells), err)
		}
		cells = append(cells, c)
	}
	if _, err := decoder.Token(); err != nil {
		return nil, err
	}
	return cells, nil
}

func targetMode(path string) (os.FileMode, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return 0600, nil
	}
	if err != nil {
		return 0, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return 0, fmt.Errorf("refusing to replace a symbolic link")
	}
	if !info.Mode().IsRegular() {
		return 0, fmt.Errorf("save target is not a regular file")
	}
	return info.Mode().Perm(), nil
}

// Save replaces the target atomically after a complete, synchronized write.
// Before a successful rename every error leaves the previous drawing intact.
func Save(path string, doc *Document) error {
	if path == "" {
		return fmt.Errorf("save filename is empty")
	}
	if err := doc.Validate(); err != nil {
		return fmt.Errorf("invalid drawing: %w", err)
	}
	mode, err := targetMode(path)
	if err != nil {
		return fmt.Errorf("inspect save target: %w", err)
	}
	dir := filepath.Dir(path)
	temp, err := os.CreateTemp(dir, "."+filepath.Base(path)+"-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary drawing: %w", err)
	}
	defer os.Remove(temp.Name())
	defer temp.Close()
	if err := temp.Chmod(mode); err != nil {
		return fmt.Errorf("set drawing permissions: %w", err)
	}
	encoder := json.NewEncoder(temp)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(fileDocument{Format: fileFormat, Document: *doc}); err != nil {
		return fmt.Errorf("write drawing: %w", err)
	}
	if err := temp.Sync(); err != nil {
		return fmt.Errorf("synchronize drawing: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close temporary drawing: %w", err)
	}
	// Recheck just before replacing the target, in case it became a symlink
	// while this drawing was being written. Rename itself never follows it.
	if _, err := targetMode(path); err != nil {
		return fmt.Errorf("recheck save target: %w", err)
	}
	if err := os.Rename(temp.Name(), path); err != nil {
		return fmt.Errorf("replace drawing: %w", err)
	}
	return nil
}
