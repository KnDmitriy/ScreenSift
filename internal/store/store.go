package store

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
	"screensift/internal/model"
)

type Store struct {
	db *sql.DB
	mu sync.RWMutex
}

func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, err
	}
	s := &Store{db: db}
	if err := s.init(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) init() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS documents (
 id INTEGER PRIMARY KEY,
 path TEXT NOT NULL UNIQUE,
 size INTEGER NOT NULL,
 modified_at TEXT NOT NULL,
 status TEXT NOT NULL,
 recognized_text TEXT NOT NULL DEFAULT '',
 indexed_at TEXT,
 error TEXT NOT NULL DEFAULT ''
);
CREATE VIRTUAL TABLE IF NOT EXISTS documents_fts USING fts5(
 path UNINDEXED,
 recognized_text,
 content='documents',
 content_rowid='id'
);
`)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`INSERT INTO documents_fts(rowid,path,recognized_text)
SELECT id,path,recognized_text FROM documents
WHERE id NOT IN (SELECT rowid FROM documents_fts)`)
	return err
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) UpsertProcessing(path string, size int64, mod time.Time) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	res, err := s.db.Exec(`INSERT INTO documents(path,size,modified_at,status)
VALUES(?,?,?,'processing')
ON CONFLICT(path) DO UPDATE SET size=excluded.size, modified_at=excluded.modified_at,
status='processing', error=''`, path, size, mod.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil || id != 0 {
		return id, err
	}
	var existing int64
	err = s.db.QueryRow(`SELECT id FROM documents WHERE path=?`, path).Scan(&existing)
	return existing, err
}

func (s *Store) MarkIndexed(id int64, text string, indexed time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var old string
	if err = tx.QueryRow(`SELECT recognized_text FROM documents WHERE id=?`, id).Scan(&old); err != nil {
		return err
	}
	_, _ = tx.Exec(`DELETE FROM documents_fts WHERE rowid=?`, id)
	_, err = tx.Exec(`UPDATE documents SET status='indexed',recognized_text=?,indexed_at=?,error='' WHERE id=?`,
		text, indexed.UTC().Format(time.RFC3339Nano), id)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`INSERT INTO documents_fts(rowid,path,recognized_text)
SELECT id,path,recognized_text FROM documents WHERE id=?`, id)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) MarkFailed(id int64, errText string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`UPDATE documents SET status='failed',error=? WHERE id=?`, errText, id)
	return err
}

func (s *Store) DeletePath(path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var id int64
	err := s.db.QueryRow(`SELECT id FROM documents WHERE path=?`, path).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.Exec(`DELETE FROM documents_fts WHERE rowid=?`, id); err != nil {
		return err
	}
	if _, err = tx.Exec(`DELETE FROM documents WHERE id=?`, id); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) GetByID(id int64) (model.Document, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var d model.Document
	var mod, indexed sql.NullString
	err := s.db.QueryRow(`SELECT id,path,size,modified_at,status,recognized_text,indexed_at,error FROM documents WHERE id=?`, id).
		Scan(&d.ID, &d.Path, &d.Size, &mod, &d.Status, &d.RecognizedText, &indexed, &d.Error)
	if err != nil {
		return d, err
	}
	d.ModifiedAt, _ = time.Parse(time.RFC3339Nano, mod.String)
	if indexed.Valid {
		t, _ := time.Parse(time.RFC3339Nano, indexed.String)
		d.IndexedAt = &t
	}
	return d, nil
}

type SearchResult struct {
	ID       int64   `json:"id"`
	Path     string  `json:"path"`
	Fragment string  `json:"fragment"`
	Text     string  `json:"text"`
	Score    float64 `json:"score"`
}

func (s *Store) Search(query string, limit int) ([]SearchResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	query = strings.TrimSpace(query)
	if query == "" {
		return []SearchResult{}, nil
	}
	tokens := strings.Fields(strings.ToLower(query))
	match := make([]string, 0, len(tokens))
	for _, tok := range tokens {
		tok = strings.Trim(tok, `"`)
		if tok != "" {
			match = append(match, `"`+strings.ReplaceAll(tok, `"`, `""`)+`"`)
		}
	}
	q := strings.Join(match, " AND ")
	rows, err := s.db.Query(`SELECT d.id,d.path,d.recognized_text,bm25(documents_fts)
FROM documents_fts f JOIN documents d ON d.id=f.rowid
WHERE f MATCH ? AND d.status='indexed' ORDER BY bm25(f) LIMIT ?`, q, limit)
	if err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}
	defer rows.Close()
	var out []SearchResult
	for rows.Next() {
		var r SearchResult
		if err := rows.Scan(&r.ID, &r.Path, &r.Text, &r.Score); err != nil {
			return nil, err
		}
		r.Fragment = r.Text
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) List(limit, offset int) ([]model.Document, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rows, err := s.db.Query(`SELECT id,path,size,modified_at,status,recognized_text,indexed_at,error
FROM documents ORDER BY path LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Document
	for rows.Next() {
		var d model.Document
		var mod, indexed sql.NullString
		if err := rows.Scan(&d.ID, &d.Path, &d.Size, &mod, &d.Status, &d.RecognizedText, &indexed, &d.Error); err != nil {
			return nil, err
		}
		d.ModifiedAt, _ = time.Parse(time.RFC3339Nano, mod.String)
		if indexed.Valid {
			t, _ := time.Parse(time.RFC3339Nano, indexed.String)
			d.IndexedAt = &t
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

type Stats struct {
	Images        int        `json:"images"`
	Indexed       int        `json:"indexed"`
	Failed        int        `json:"failed"`
	Processing    int        `json:"processing"`
	Pending       int        `json:"pending"`
	LastIndexedAt *time.Time `json:"lastIndexedAt,omitempty"`
}

func (s *Store) Stats() (Stats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var st Stats
	rows, err := s.db.Query(`SELECT status,COUNT(*) FROM documents GROUP BY status`)
	if err != nil {
		return st, err
	}
	defer rows.Close()
	for rows.Next() {
		var status string
		var n int
		if err := rows.Scan(&status, &n); err != nil {
			return st, err
		}
		st.Images += n
		switch model.Status(status) {
		case model.StatusIndexed:
			st.Indexed = n
		case model.StatusFailed:
			st.Failed = n
		case model.StatusProcessing:
			st.Processing = n
		case model.StatusPending:
			st.Pending = n
		}
	}
	var t sql.NullString
	if err = s.db.QueryRow(`SELECT MAX(indexed_at) FROM documents`).Scan(&t); err != nil {
		return st, err
	}
	if t.Valid {
		v, _ := time.Parse(time.RFC3339Nano, t.String)
		st.LastIndexedAt = &v
	}
	return st, nil
}

func (s *Store) AllPaths() (map[string]model.Document, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rows, err := s.db.Query(`SELECT id,path,size,modified_at,status,recognized_text,indexed_at,error FROM documents`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]model.Document{}
	for rows.Next() {
		var d model.Document
		var mod, indexed sql.NullString
		if err := rows.Scan(&d.ID, &d.Path, &d.Size, &mod, &d.Status, &d.RecognizedText, &indexed, &d.Error); err != nil {
			return nil, err
		}
		d.ModifiedAt, _ = time.Parse(time.RFC3339Nano, mod.String)
		if indexed.Valid {
			t, _ := time.Parse(time.RFC3339Nano, indexed.String)
			d.IndexedAt = &t
		}
		out[d.Path] = d
	}
	return out, rows.Err()
}
