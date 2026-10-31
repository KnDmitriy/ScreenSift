package indexer

import (
	"context"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"screensift/internal/config"
	"screensift/internal/ocr"
	"screensift/internal/store"
)

type Indexer struct {
	cfg        config.Config
	store      *store.Store
	recognizer ocr.Recognizer
	queue      chan string
	wg         sync.WaitGroup
	ctx        context.Context
	cancel     context.CancelFunc
	mu         sync.Mutex
	pending    map[string]time.Time
}

func New(cfg config.Config, st *store.Store, r ocr.Recognizer) *Indexer {
	return &Indexer{cfg: cfg, store: st, recognizer: r, queue: make(chan string, cfg.QueueSize), pending: map[string]time.Time{}}
}

func (i *Indexer) Start(parent context.Context) error {
	i.ctx, i.cancel = context.WithCancel(parent)
	if err := os.MkdirAll(i.cfg.RootDir, 0755); err != nil {
		return err
	}
	for n := 0; n < i.cfg.Workers; n++ {
		i.wg.Add(1)
		go i.worker()
	}
	if err := i.reconcile(); err != nil {
		return err
	}
	i.wg.Add(1)
	go i.watch()
	return nil
}

func (i *Indexer) Stop() {
	if i.cancel != nil {
		i.cancel()
	}
	i.wg.Wait()
}

func (i *Indexer) worker() {
	defer i.wg.Done()
	for {
		select {
		case <-i.ctx.Done():
			return
		case path := <-i.queue:
			i.process(path)
		}
	}
}

func (i *Indexer) process(path string) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			_ = i.store.DeletePath(path)
		}
		return
	}
	if !i.supported(path) {
		return
	}
	if err = i.validate(path, info); err != nil {
		id, e := i.store.UpsertProcessing(path, info.Size(), info.ModTime())
		if e == nil {
			_ = i.store.MarkFailed(id, err.Error())
		}
		return
	}
	id, err := i.store.UpsertProcessing(path, info.Size(), info.ModTime())
	if err != nil {
		slog.Error("store processing", "path", path, "error", err)
		return
	}
	text, err := i.recognizer.Recognize(i.ctx, path)
	if err != nil {
		_ = i.store.MarkFailed(id, err.Error())
		return
	}
	info2, err := os.Stat(path)
	if err != nil {
		return
	}
	if info2.Size() != info.Size() || !info2.ModTime().Equal(info.ModTime()) {
		i.enqueue(path)
		return
	}
	if err = i.store.MarkIndexed(id, normalize(text), time.Now()); err != nil {
		slog.Error("store indexed", "path", path, "error", err)
	}
}

func (i *Indexer) validate(path string, info os.FileInfo) error {
	if info.Size() > i.cfg.MaxImageSize {
		return fmt.Errorf("image exceeds max size")
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	cfg, format, err := image.DecodeConfig(f)
	if err != nil {
		return fmt.Errorf("invalid image: %w", err)
	}
	if !isAllowedFormat(format) {
		return fmt.Errorf("unsupported image format: %s", format)
	}
	if cfg.Width > i.cfg.MaxWidth || cfg.Height > i.cfg.MaxHeight {
		return fmt.Errorf("image dimensions exceed limit")
	}
	if int64(cfg.Width)*int64(cfg.Height) > i.cfg.MaxPixels {
		return fmt.Errorf("image pixel count exceeds limit")
	}
	return nil
}

func (i *Indexer) reconcile() error {
	saved, err := i.store.AllPaths()
	if err != nil {
		return err
	}
	seen := map[string]bool{}
	err = filepath.WalkDir(i.cfg.RootDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != i.cfg.RootDir && i.ignored(path) {
				return filepath.SkipDir
			}
			return nil
		}
		if !i.supported(path) {
			return nil
		}
		seen[path] = true
		info, e := d.Info()
		if e != nil {
			return e
		}
		old, ok := saved[path]
		if !ok || old.Size != info.Size() || !old.ModifiedAt.Equal(info.ModTime()) || old.Status != "indexed" {
			i.enqueue(path)
		}
		return nil
	})
	if err != nil {
		return err
	}
	for path := range saved {
		if !seen[path] {
			_ = i.store.DeletePath(path)
		}
	}
	return nil
}

func (i *Indexer) watch() {
	defer i.wg.Done()
	w, err := fsnotify.NewWatcher()
	if err != nil {
		slog.Error("watcher", "error", err)
		return
	}
	defer w.Close()
	if err := i.addDirs(w, i.cfg.RootDir); err != nil {
		slog.Error("watch dirs", "error", err)
		return
	}
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-i.ctx.Done():
			return
		case ev, ok := <-w.Events:
			if !ok {
				return
			}
			path := filepath.Clean(ev.Name)
			if ev.Op&fsnotify.Create != 0 {
				if info, e := os.Stat(path); e == nil && info.IsDir() {
					_ = i.addDirs(w, path)
					continue
				}
			}
			if !i.supported(path) {
				continue
			}
			if ev.Op&(fsnotify.Remove|fsnotify.Rename) != 0 {
				_ = i.store.DeletePath(path)
			}
			if ev.Op&(fsnotify.Create|fsnotify.Write) != 0 {
				i.schedule(path)
			}
		case err := <-w.Errors:
			if err != nil {
				slog.Error("watcher error", "error", err)
			}
		case now := <-ticker.C:
			i.mu.Lock()
			for p, t := range i.pending {
				if now.Sub(t) >= i.cfg.Debounce {
					delete(i.pending, p)
					i.enqueue(p)
				}
			}
			i.mu.Unlock()
		}
	}
}

func (i *Indexer) schedule(path string) { i.mu.Lock(); i.pending[path] = time.Now(); i.mu.Unlock() }

func (i *Indexer) enqueue(path string) {
	select {
	case i.queue <- path:
	default:
		slog.Warn("index queue full", "path", path)
	}
}

func (i *Indexer) addDirs(w *fsnotify.Watcher, root string) error {
	return filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		if path != root && i.ignored(path) {
			return filepath.SkipDir
		}
		return w.Add(path)
	})
}

func (i *Indexer) ignored(path string) bool {
	base := filepath.Base(path)
	for _, v := range i.cfg.IgnoreDirs {
		if base == v {
			return true
		}
	}
	return strings.HasPrefix(base, ".")
}

func (i *Indexer) supported(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png", ".jpg", ".jpeg":
		return true
	}
	return false
}
func isAllowedFormat(format string) bool { return format == "png" || format == "jpeg" }
func normalize(s string) string          { return strings.Join(strings.Fields(strings.ToLower(s)), " ") }

func (i *Indexer) Rebuild() error { return i.reconcile() }
func (i *Indexer) Reindex(id int64) error {
	d, err := i.store.GetByID(id)
	if err != nil {
		return err
	}
	i.enqueue(d.Path)
	return nil
}
func (i *Indexer) QueueSize() int { return len(i.queue) }
func (i *Indexer) Workers() int   { return i.cfg.Workers }
