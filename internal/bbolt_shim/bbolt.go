// Package bbolt_shim is a minimal, file-backed implementation of the go.etcd.io/bbolt API.
// Replace with the real bbolt when network access is available:
//
//	go get go.etcd.io/bbolt@v1.4.0
//	# then remove: replace go.etcd.io/bbolt => ./internal/bbolt_shim
package bbolt_shim

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// DB represents a database. It wraps a JSON file storing bucket data.
type DB struct {
	path    string
	mu      sync.RWMutex
	buckets map[string]map[string][]byte
	closed  bool
}

// Tx represents a read-only or read-write transaction.
type Tx struct {
	db       *DB
	writable bool
	buckets  map[string]map[string][]byte
}

// Bucket represents a collection of key-value pairs.
type Bucket struct {
	data map[string][]byte
}

var (
	ErrBucketNotFound = errors.New("bucket not found")
	ErrTxNotWritable  = errors.New("transaction not writable")
	ErrDatabaseClosed = errors.New("database closed")
)

// Open opens a database at the given path.
func Open(path string, mode os.FileMode, options *Options) (*DB, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("bbolt_shim: create dir: %w", err)
	}

	db := &DB{
		path:    path,
		buckets: make(map[string]map[string][]byte),
	}

	// Load existing data
	data, err := os.ReadFile(path)
	if err == nil {
		if err := json.Unmarshal(data, &db.buckets); err != nil {
			return nil, fmt.Errorf("bbolt_shim: unmarshal db: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("bbolt_shim: read db: %w", err)
	}

	// Ensure buckets map initialized
	if db.buckets == nil {
		db.buckets = make(map[string]map[string][]byte)
	}

	return db, nil
}

// Close closes the database, flushing pending writes.
func (db *DB) Close() error {
	db.mu.Lock()
	defer db.mu.Unlock()
	db.closed = true
	return db.flush()
}

func (db *DB) flush() error {
	data, err := json.MarshalIndent(db.buckets, "", "  ")
	if err != nil {
		return fmt.Errorf("bbolt_shim: marshal: %w", err)
	}
	// Write atomically via temp file + rename to avoid corruption
	tmpPath := db.path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("bbolt_shim: write tmp: %w", err)
	}
	if err := os.Rename(tmpPath, db.path); err != nil {
		return fmt.Errorf("bbolt_shim: rename: %w", err)
	}
	return nil
}

// Update executes a function within a read-write transaction.
func (db *DB) Update(fn func(*Tx) error) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if db.closed {
		return ErrDatabaseClosed
	}

	// Deep copy buckets for the transaction
	txBuckets := deepCopyBuckets(db.buckets)
	tx := &Tx{db: db, writable: true, buckets: txBuckets}

	if err := fn(tx); err != nil {
		return err
	}

	// Apply changes back
	db.buckets = txBuckets
	return db.flush()
}

// View executes a function within a read-only transaction.
func (db *DB) View(fn func(*Tx) error) error {
	db.mu.RLock()
	defer db.mu.RUnlock()

	if db.closed {
		return ErrDatabaseClosed
	}

	txBuckets := deepCopyBuckets(db.buckets)
	tx := &Tx{db: db, writable: false, buckets: txBuckets}
	return fn(tx)
}

// CreateBucketIfNotExists creates a new bucket if it doesn't already exist.
func (tx *Tx) CreateBucketIfNotExists(name []byte) (*Bucket, error) {
	if !tx.writable {
		return nil, ErrTxNotWritable
	}
	n := string(name)
	if _, ok := tx.buckets[n]; !ok {
		tx.buckets[n] = make(map[string][]byte)
	}
	return &Bucket{data: tx.buckets[n]}, nil
}

// Bucket returns a bucket by name, or nil if it doesn't exist.
func (tx *Tx) Bucket(name []byte) *Bucket {
	n := string(name)
	if data, ok := tx.buckets[n]; ok {
		return &Bucket{data: data}
	}
	return nil
}

// Put sets a key-value pair in the bucket.
func (b *Bucket) Put(key, value []byte) error {
	if b.data == nil {
		return ErrBucketNotFound
	}
	k := make([]byte, len(key))
	copy(k, key)
	v := make([]byte, len(value))
	copy(v, value)
	b.data[string(k)] = v
	return nil
}

// Get retrieves a value by key, or nil if not found.
func (b *Bucket) Get(key []byte) []byte {
	if b.data == nil {
		return nil
	}
	return b.data[string(key)]
}

// Delete removes a key from the bucket.
func (b *Bucket) Delete(key []byte) error {
	if b.data == nil {
		return ErrBucketNotFound
	}
	delete(b.data, string(key))
	return nil
}

// ForEach executes a function for each key-value pair in the bucket.
// If the function returns an error, iteration stops.
func (b *Bucket) ForEach(fn func(k, v []byte) error) error {
	if b.data == nil {
		return nil
	}
	for k, v := range b.data {
		if err := fn([]byte(k), v); err != nil {
			return err
		}
	}
	return nil
}

// Options for opening a database (minimal, matching bbolt signature).
type Options struct {
	Timeout int64 // ignored in shim
}

func deepCopyBuckets(src map[string]map[string][]byte) map[string]map[string][]byte {
	dst := make(map[string]map[string][]byte, len(src))
	for name, b := range src {
		bCopy := make(map[string][]byte, len(b))
		for k, v := range b {
			val := make([]byte, len(v))
			copy(val, v)
			bCopy[k] = val
		}
		dst[name] = bCopy
	}
	return dst
}
