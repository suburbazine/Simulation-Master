package main

import (
	"bytes"
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// StorageSubdirSpec describes one canonical folder in the local storage tree.
type StorageSubdirSpec struct {
	RelPath     string            `json:"rel_path"`
	Category    string            `json:"category"` // "db" or "bin"
	Module      string            `json:"module"`   // "ecu", "tcu", "awd"
	Description string            `json:"description"`
	Files       []StorageFileInfo `json:"files"`
}

// StorageFileInfo describes a single file inside the local storage tree.
type StorageFileInfo struct {
	Name      string `json:"name"`
	RelPath   string `json:"rel_path"`
	FullPath  string `json:"full_path"`
	SizeBytes int64  `json:"size_bytes"`
	UpdatedAt string `json:"updated_at"`
}

// StorageTreeStatus represents the complete local storage folder hierarchy.
type StorageTreeStatus struct {
	RootDir     string              `json:"root_dir"`
	TotalFiles  int                 `json:"total_files"`
	TotalBytes  int64               `json:"total_bytes"`
	Directories []StorageSubdirSpec `json:"directories"`
}

// ImportedFileResult summarizes one imported DB or BIN file.
type ImportedFileResult struct {
	Filename  string `json:"filename"`
	Category  string `json:"category"` // "db" or "bin"
	Module    string `json:"module"`   // "ecu", "tcu", "awd"
	SavedPath string `json:"saved_path"`
	RelPath   string `json:"rel_path"`
	SizeBytes int64  `json:"size_bytes"`
}

// ImportResponse is returned by POST /api/import.
type ImportResponse struct {
	Imported  []ImportedFileResult `json:"imported"`
	Storage   StorageTreeStatus    `json:"storage"`
	Inventory InventoryResponse    `json:"inventory"`
}

var canonicalStorageSubdirs = []struct {
	RelPath     string
	Category    string
	Module      string
	Description string
}{
	{"storage/db/ecu", "db", "ecu", "SIM2K-260 (TC1791) BinGo SQLite Databases (e.g. flash_analysis.db)"},
	{"storage/db/tcu", "db", "tcu", "Kefico CPTSH 8AT (SH72549R) BinGo SQLite Databases (e.g. tcu_cptsh_95441_4J571.db)"},
	{"storage/db/awd", "db", "awd", "Magna ATC AWD (95447-4J105) BinGo SQLite Databases (e.g. awd_95447_4J105.db)"},
	{"storage/bins/ecu", "bin", "ecu", "SIM2K-260 4MB TriCore ECU Flash Binaries (.bin)"},
	{"storage/bins/tcu", "bin", "tcu", "Kefico CPTSH 8AT 768KB TCU Calibration Binaries (.bin)"},
	{"storage/bins/awd", "bin", "awd", "Magna ATC AWD 48KB / 32KB Calibration Binaries (.bin)"},
}

// GetStorageRoot returns the absolute path to the local storage directory inside the application folder.
func GetStorageRoot() string {
	cwd, err := os.Getwd()
	if err != nil {
		return "storage"
	}
	return filepath.Join(cwd, "storage")
}

// EnsureStorageTree creates the organized local storage folder tree and .gitkeep placeholders if missing.
func EnsureStorageTree() error {
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}
	for _, d := range canonicalStorageSubdirs {
		fullDir := filepath.Join(cwd, filepath.FromSlash(d.RelPath))
		if err := os.MkdirAll(fullDir, 0755); err != nil {
			return fmt.Errorf("failed creating storage dir %s: %w", fullDir, err)
		}
		gitkeep := filepath.Join(fullDir, ".gitkeep")
		if _, err := os.Stat(gitkeep); os.IsNotExist(err) {
			_ = os.WriteFile(gitkeep, []byte("# Keep directory in git while ignoring .bin and .db files\n"), 0644)
		}
	}
	return nil
}

// GetStorageTreeStatus scans the local storage/ tree and returns its organized directory & file inventory.
func GetStorageTreeStatus() StorageTreeStatus {
	_ = EnsureStorageTree()
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}
	status := StorageTreeStatus{
		RootDir: filepath.Join(cwd, "storage"),
	}

	for _, spec := range canonicalStorageSubdirs {
		fullDir := filepath.Join(cwd, filepath.FromSlash(spec.RelPath))
		dirItem := StorageSubdirSpec{
			RelPath:     spec.RelPath,
			Category:    spec.Category,
			Module:      spec.Module,
			Description: spec.Description,
			Files:       []StorageFileInfo{},
		}
		entries, err := os.ReadDir(fullDir)
		if err == nil {
			for _, e := range entries {
				if e.IsDir() {
					continue
				}
				name := e.Name()
				if strings.HasPrefix(name, ".") || strings.EqualFold(name, "README.md") {
					continue
				}
				lower := strings.ToLower(name)
				if strings.HasSuffix(lower, "-wal") || strings.HasSuffix(lower, "-shm") || strings.HasSuffix(lower, "-journal") {
					continue
				}
				info, err := e.Info()
				if err != nil {
					continue
				}
				fPath := filepath.Clean(filepath.Join(fullDir, name))
				dirItem.Files = append(dirItem.Files, StorageFileInfo{
					Name:      name,
					RelPath:   spec.RelPath + "/" + name,
					FullPath:  fPath,
					SizeBytes: info.Size(),
					UpdatedAt: info.ModTime().Format("2006-01-02 15:04"),
				})
				status.TotalFiles++
				status.TotalBytes += info.Size()
			}
		}
		sort.Slice(dirItem.Files, func(i, j int) bool {
			return strings.ToLower(dirItem.Files[i].Name) < strings.ToLower(dirItem.Files[j].Name)
		})
		status.Directories = append(status.Directories, dirItem)
	}
	return status
}

// ClassifyAndStoreFile classifies an uploaded .db or .bin file into the local storage folder tree.
func ClassifyAndStoreFile(origFilename string, data []byte, targetHint string) (ImportedFileResult, error) {
	if err := EnsureStorageTree(); err != nil {
		return ImportedFileResult{}, err
	}
	safeName := filepath.Base(strings.TrimSpace(origFilename))
	if safeName == "" || safeName == "." || safeName == "/" || safeName == "\\" {
		return ImportedFileResult{}, fmt.Errorf("invalid filename")
	}

	lowerName := strings.ToLower(safeName)
	hint := strings.ToLower(strings.TrimSpace(targetHint))

	isSQLite := strings.HasSuffix(lowerName, ".db") ||
		strings.HasSuffix(lowerName, ".sqlite") ||
		strings.HasSuffix(lowerName, ".sqlite3") ||
		(len(data) >= 16 && bytes.Equal(data[:15], []byte("SQLite format 3")))

	category := "bin"
	if isSQLite {
		category = "db"
	} else if !strings.HasSuffix(lowerName, ".bin") && !strings.HasSuffix(lowerName, ".rom") && !strings.HasSuffix(lowerName, ".ori") {
		return ImportedFileResult{}, fmt.Errorf("unsupported file %q: only .bin, .db, .sqlite, or .sqlite3 files are accepted", safeName)
	}

	module := ""
	switch hint {
	case "ecu", "tcu", "awd":
		module = hint
	}

	if module == "" {
		if category == "db" {
			module = detectDBModule(lowerName, data)
		} else {
			module = detectBinModule(lowerName, int64(len(data)))
		}
	}

	relDir := fmt.Sprintf("storage/%s/%s", category, module)
	if category == "bin" {
		relDir = fmt.Sprintf("storage/bins/%s", module)
	}

	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}
	destDir := filepath.Join(cwd, filepath.FromSlash(relDir))
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return ImportedFileResult{}, err
	}

	destPath := filepath.Clean(filepath.Join(destDir, safeName))
	if err := os.WriteFile(destPath, data, 0644); err != nil {
		return ImportedFileResult{}, fmt.Errorf("failed saving %s: %w", destPath, err)
	}

	return ImportedFileResult{
		Filename:  safeName,
		Category:  category,
		Module:    module,
		SavedPath: destPath,
		RelPath:   relDir + "/" + safeName,
		SizeBytes: int64(len(data)),
	}, nil
}

func detectDBModule(lowerName string, data []byte) string {
	switch {
	case strings.Contains(lowerName, "awd") || strings.Contains(lowerName, "95447") || strings.Contains(lowerName, "atc"):
		return "awd"
	case strings.Contains(lowerName, "tcu") || strings.Contains(lowerName, "cptsh") || strings.Contains(lowerName, "95441"):
		return "tcu"
	case strings.Contains(lowerName, "ecu") || strings.Contains(lowerName, "flash_analysis") || strings.Contains(lowerName, "sim2k"):
		return "ecu"
	}

	// Inspect SQLite address range in a temporary file if filename is generic
	tmpFile, err := os.CreateTemp("", "simmaster_probe_*.db")
	if err == nil {
		tmpName := tmpFile.Name()
		_, _ = tmpFile.Write(data)
		_ = tmpFile.Close()
		defer os.Remove(tmpName)

		db, err := sql.Open("sqlite3", tmpName+"?mode=ro")
		if err == nil {
			defer db.Close()
			var maxEA int64
			if err := db.QueryRow("SELECT COALESCE(MAX(ea), 0) FROM cal_maps").Scan(&maxEA); err == nil {
				if maxEA >= 0x80000000 {
					return "ecu"
				}
				if maxEA >= 0xC00000 && maxEA <= 0xC10000 {
					return "awd"
				}
				if maxEA > 0 {
					return "tcu"
				}
			}
		}
	}
	return "ecu"
}

func detectBinModule(lowerName string, sz int64) string {
	switch sz {
	case 4194304, 2097152:
		return "ecu"
	case 785408, 786432, 1048576, 2621440:
		return "tcu"
	case 49152, 32768, 65536:
		return "awd"
	}
	switch {
	case strings.Contains(lowerName, "awd") || strings.Contains(lowerName, "95447") || strings.Contains(lowerName, "sportlaunch"):
		return "awd"
	case strings.Contains(lowerName, "tcu") || strings.Contains(lowerName, "cptsh") || strings.Contains(lowerName, "ekm") || strings.Contains(lowerName, "95441"):
		return "tcu"
	default:
		return "ecu"
	}
}

// handleStorageStatus handles GET /api/storage
func handleStorageStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, GetStorageTreeStatus())
}

// handleImportFiles handles POST /api/import (multipart/form-data upload of .db and .bin files)
func handleImportFiles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}

	// Allow up to 256 MB multipart upload batch
	if err := r.ParseMultipartForm(256 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "Failed to parse multipart upload: "+err.Error())
		return
	}

	targetHint := r.FormValue("target")
	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		writeError(w, http.StatusBadRequest, "No files provided in 'files' field")
		return
	}

	var imported []ImportedFileResult
	for _, fh := range files {
		f, err := fh.Open()
		if err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("Failed to open uploaded file %s: %v", fh.Filename, err))
			return
		}
		data, err := io.ReadAll(f)
		_ = f.Close()
		if err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("Failed reading uploaded file %s: %v", fh.Filename, err))
			return
		}

		res, err := ClassifyAndStoreFile(fh.Filename, data, targetHint)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		imported = append(imported, res)
	}

	// Re-scan inventory after import
	ecuBins, tcuBins, awdBins := DiscoverBinaries()
	dbs := DiscoverDatabases()
	surfaces := GetSurfacePresets()
	presets := GetScenarioPresets(ecuBins, tcuBins, awdBins)

	resp := ImportResponse{
		Imported: imported,
		Storage:  GetStorageTreeStatus(),
		Inventory: InventoryResponse{
			ECUBins:   ecuBins,
			TCUBins:   tcuBins,
			AWDBins:   awdBins,
			Databases: dbs,
			Surfaces:  surfaces,
			Presets:   presets,
		},
	}
	_ = time.Now()
	writeJSON(w, http.StatusOK, resp)
}
