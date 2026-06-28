package transfer

import (
	"encoding/json"
	"errors"
	"time"
)

var (
	ErrInvalidExport  = errors.New("invalid export request")
	ErrNotFound       = errors.New("export resource not found")
	ErrImportConflict = errors.New("backup project already exists")
)

const BackupVersion = 1

type Backup struct {
	Version    int                        `json:"version"`
	ExportedAt time.Time                  `json:"exported_at"`
	ProjectID  string                     `json:"project_id"`
	Tables     map[string]json.RawMessage `json:"tables"`
}

type MarkdownDocument struct {
	Filename string
	Content  string
}
