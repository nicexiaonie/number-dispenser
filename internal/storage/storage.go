package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/nicexiaonie/number-dispenser/internal/dispenser"
)

// Storage provides persistence for dispensers
type Storage interface {
	Save(name string, cfg dispenser.Config, current int64) error
	Load(name string) (dispenser.Config, int64, error)
	Delete(name string) error
	ListAll() (map[string]DispenserData, error)
}

// DispenserData represents the persisted data of a dispenser
type DispenserData struct {
	Config  dispenser.Config `json:"config"`
	Current int64            `json:"current"`
	Updated time.Time        `json:"updated"`
}

// FileStorage implements Storage using local file system
type FileStorage struct {
	mu       sync.RWMutex
	dataDir  string
	data     map[string]DispenserData
	autoSave bool
	dirty    bool
}

// NewFileStorage creates a new file storage
func NewFileStorage(dataDir string, autoSave bool) (*FileStorage, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, err
	}

	fs := &FileStorage{
		dataDir:  dataDir,
		data:     make(map[string]DispenserData),
		autoSave: autoSave,
	}

	// Load existing data
	if err := fs.loadFromDisk(); err != nil {
		return nil, err
	}

	// Start auto-save goroutine if enabled
	if autoSave {
		go fs.autoSaveLoop()
	}

	return fs, nil
}

// Save saves dispenser data
func (fs *FileStorage) Save(name string, cfg dispenser.Config, current int64) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	fs.data[name] = DispenserData{
		Config:  cfg,
		Current: current,
		Updated: time.Now(),
	}
	fs.dirty = true

	if !fs.autoSave {
		return fs.saveToDisk()
	}

	return nil
}

// Load loads dispenser data
func (fs *FileStorage) Load(name string) (dispenser.Config, int64, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	data, exists := fs.data[name]
	if !exists {
		return dispenser.Config{}, 0, os.ErrNotExist
	}

	return data.Config, data.Current, nil
}

// Delete deletes dispenser data
func (fs *FileStorage) Delete(name string) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	delete(fs.data, name)
	fs.dirty = true

	if !fs.autoSave {
		return fs.saveToDisk()
	}

	return nil
}

// ListAll returns all dispenser data
func (fs *FileStorage) ListAll() (map[string]DispenserData, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	result := make(map[string]DispenserData, len(fs.data))
	for k, v := range fs.data {
		result[k] = v
	}

	return result, nil
}

// Flush forces a save to disk
func (fs *FileStorage) Flush() error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	if !fs.dirty {
		return nil
	}

	return fs.saveToDisk()
}

// saveToDisk saves data to disk (must be called with lock held)
func (fs *FileStorage) saveToDisk() error {
	tmpFile := filepath.Join(fs.dataDir, "dispensers.json.tmp")
	finalFile := filepath.Join(fs.dataDir, "dispensers.json")

	// Marshal data
	data, err := json.MarshalIndent(fs.data, "", "  ")
	if err != nil {
		return err
	}

	// Write to temporary file
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		return err
	}

	// Atomic rename
	if err := os.Rename(tmpFile, finalFile); err != nil {
		return err
	}

	fs.dirty = false
	return nil
}

// loadFromDisk loads data from disk
func (fs *FileStorage) loadFromDisk() error {
	filePath := filepath.Join(fs.dataDir, "dispensers.json")

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No data file yet, that's ok
		}
		return err
	}

	return json.Unmarshal(data, &fs.data)
}

// autoSaveLoop periodically saves dirty data to disk
func (fs *FileStorage) autoSaveLoop() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		fs.mu.Lock()
		if fs.dirty {
			_ = fs.saveToDisk() // Ignore error in background save
		}
		fs.mu.Unlock()
	}
}
