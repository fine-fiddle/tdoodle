package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/gdamore/tcell/v3"
	"tdoodle/internal/canvas"
	"tdoodle/internal/editor"
	"tdoodle/internal/ui"
)

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "tdoodle:", err)
		os.Exit(1)
	}
}

type options struct {
	path     string
	aspect   float64
	autosave time.Duration
}

func parseOptions(args []string, out io.Writer) (options, error) {
	o := options{aspect: 2, autosave: 15 * time.Second}
	f := flag.NewFlagSet("tdoodle", flag.ContinueOnError)
	f.SetOutput(out)
	f.Float64Var(&o.aspect, "aspect", 2, "terminal cell height / width (new drawings)")
	f.DurationVar(&o.autosave, "autosave", 15*time.Second, "recovery save interval (0 disables)")
	f.Usage = func() {
		fmt.Fprintln(out, "tDoodle is a terminal drawing tool for text, lines, rectangles, ovals, and freehand marks.")
		fmt.Fprintln(out, "\nUsage: tdoodle [options] [filename]")
		fmt.Fprintln(out, "\nFilename (optional):")
		fmt.Fprintln(out, "  Existing file: open the drawing for editing.")
		fmt.Fprintln(out, "  Missing file: start a new drawing and save to that filename.")
		fmt.Fprintln(out, "  Omitted: start a new drawing and save to a timestamped .tdoodle file in the current directory.")
		fmt.Fprintln(out, "\nF7 opens drawing help; Ctrl+C twice saves and quits.")
		fmt.Fprintln(out, "\nOptions:")
		f.PrintDefaults()
		fmt.Fprintln(out, "  -h, --help\n        show this command-line help (-help also works)")
	}
	if err := f.Parse(args); err != nil {
		return o, err
	}
	if f.NArg() > 1 {
		return o, errors.New("expected at most one filename")
	}
	if math.IsNaN(o.aspect) || math.IsInf(o.aspect, 0) || o.aspect < 0.1 || o.aspect > 10 {
		return o, errors.New("aspect must be between 0.1 and 10")
	}
	if o.autosave < 0 {
		return o, errors.New("autosave must be nonnegative")
	}
	if f.NArg() == 1 {
		o.path = f.Arg(0)
		if o.path == "" {
			return o, errors.New("filename must not be empty")
		}
	}
	return o, nil
}

// generatedPath includes nanoseconds to distinguish ordinary launches.
func generatedPath(now time.Time) string {
	return "tdoodle-" + now.Format("20060102T150405.000000000") + ".tdoodle"
}

func openDrawing(path string) (*canvas.Document, string, error) {
	doc, err := canvas.Load(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, "", fmt.Errorf("open %s: %w", path, err)
	}
	primary, statErr := os.Stat(path)
	if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
		return nil, "", statErr
	}
	recoveryPath := canvas.RecoveryPath(path)
	recovery, err := os.Stat(recoveryPath)
	if errors.Is(err, os.ErrNotExist) {
		return doc, "", nil
	}
	if err != nil {
		return doc, "Recovery unavailable: " + err.Error(), nil
	}
	if primary != nil && !recovery.ModTime().After(primary.ModTime()) {
		return doc, "", nil
	}
	recovered, err := canvas.Load(recoveryPath)
	if err != nil {
		return doc, "Recovery unreadable; kept original", nil
	}
	return recovered, "Recovered unsaved drawing", nil
}

func run(args []string, out, errOut io.Writer) error {
	o, err := parseOptions(args, errOut)
	if errors.Is(err, flag.ErrHelp) {
		return nil
	}
	if err != nil {
		return err
	}
	if o.path == "" {
		o.path = generatedPath(time.Now())
	}
	doc, message, err := openDrawing(o.path)
	if err != nil {
		return err
	}
	screen, err := tcell.NewScreen()
	if err != nil {
		return fmt.Errorf("open terminal: %w", err)
	}
	if err = screen.Init(); err != nil {
		return fmt.Errorf("initialize terminal: %w", err)
	}
	defer screen.Fini()
	w, h := screen.Size()
	if doc == nil {
		doc, err = canvas.New(max(1, w), max(1, h-1), o.aspect)
		if err != nil {
			return err
		}
	}
	screen.DisableMouse()
	screen.EnablePaste()
	e := editor.New(doc)
	e.Message = message
	e.Resize(w, h)
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	defer signal.Stop(signals)
	var recoveryTick <-chan time.Time
	if o.autosave > 0 {
		timer := time.NewTicker(o.autosave)
		defer timer.Stop()
		recoveryTick = timer.C
	}
	guardTimer := time.NewTicker(250 * time.Millisecond)
	defer guardTimer.Stop()
	var recoveredRevision uint64
	pasting := false
	ui.Render(screen, e)
	for {
		action := editor.ActionNone
		redraw := true
		select {
		case event, ok := <-screen.EventQ():
			if !ok {
				return errors.New("terminal closed unexpectedly; check the recovery file")
			}
			switch ev := event.(type) {
			case *tcell.EventResize:
				w, h = screen.Size()
				e.Resize(w, h)
				screen.Sync()
			case *tcell.EventKey:
				if pasting {
					// Paste is text only: escape sequences and control characters
					// cannot accidentally commit a shape or quit the application.
					if e.Tool == editor.ToolText && !e.Help && e.Palette == 0 && !e.Toolbar && ev.Key() == tcell.KeyRune && ev.Modifiers() == tcell.ModNone {
						for _, r := range ev.Str() {
							if r >= 32 && r <= 126 {
								e.Type(r)
							}
						}
					}
				} else {
					action = e.HandleKey(ev, time.Now())
				}
			case *tcell.EventPaste:
				pasting = ev.Start()
			default:
				redraw = false
			}
		case <-signals:
			action = editor.ActionQuit
		case <-recoveryTick:
			redraw = false
			if e.Revision != recoveredRevision {
				if err := canvas.Save(canvas.RecoveryPath(o.path), doc); err != nil {
					e.Message = "RECOVERY FAILED: " + err.Error()
					redraw = true
				} else {
					recoveredRevision = e.Revision
				}
			}
		case now := <-guardTimer.C:
			redraw = e.Tick(now)
		}
		if action == editor.ActionSave || action == editor.ActionQuit {
			if err := canvas.Save(o.path, doc); err != nil {
				e.Message = "SAVE FAILED: " + err.Error()
			} else {
				// A recovery copy older than the successful save is no longer needed.
				_ = os.Remove(canvas.RecoveryPath(o.path))
				recoveredRevision = e.Revision
				if action == editor.ActionQuit {
					screen.Fini()
					fmt.Fprintln(out, "Saved", filepath.Clean(o.path))
					return nil
				}
				e.Message = "Saved " + filepath.Base(o.path)
			}
		}
		if action == editor.ActionRefresh {
			screen.Sync()
		}
		if redraw {
			if e.Help {
				e.HelpScroll = min(e.HelpScroll, max(0, ui.HelpRows(w)-max(1, h-2)))
			}
			ui.Render(screen, e)
		}
	}
}
