package state

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type BackupManager struct {
	dataDir    string
	backupDir  string
	maxBackups int
}

func NewBackupManager(dataDir, backupDir string, maxBackups int) *BackupManager {
	return &BackupManager{
		dataDir:    dataDir,
		backupDir:  backupDir,
		maxBackups: maxBackups,
	}
}

func (bm *BackupManager) CreateBackup() (string, error) {
	if err := os.MkdirAll(bm.backupDir, 0750); err != nil {
		return "", fmt.Errorf("failed to create backup directory: %w", err)
	}

	timestamp := time.Now().UTC().Format("20060102-150405")
	backupName := fmt.Sprintf("viri-backup-%s.tar.gz", timestamp)
	backupPath := filepath.Join(bm.backupDir, backupName)

	outFile, err := os.Create(backupPath)
	if err != nil {
		return "", fmt.Errorf("failed to create backup file: %w", err)
	}
	defer outFile.Close()

	gzWriter := gzip.NewWriter(outFile)
	defer gzWriter.Close()

	tarWriter := tar.NewWriter(gzWriter)
	defer tarWriter.Close()

	err = filepath.Walk(bm.dataDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(bm.dataDir, path)
		if err != nil {
			return err
		}

		if strings.Contains(relPath, "lock") || (info.IsDir() && strings.Contains(relPath, "compaction")) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return fmt.Errorf("failed to create tar header for %s: %w", path, err)
		}

		header.Name = filepath.ToSlash(relPath)

		if info.IsDir() {
			header.Name += "/"
			header.Mode = 0750
			return tarWriter.WriteHeader(header)
		}

		header.Mode = 0640
		if err := tarWriter.WriteHeader(header); err != nil {
			return err
		}

		file, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("failed to open %s: %w", path, err)
		}
		defer file.Close()

		_, err = io.Copy(tarWriter, file)
		return err
	})

	if err != nil {
		os.Remove(backupPath)
		return "", fmt.Errorf("failed to create backup: %w", err)
	}

	bm.pruneOldBackups()

	return backupPath, nil
}

func (bm *BackupManager) RestoreBackup(backupPath string, targetDir string) error {
	if err := os.MkdirAll(targetDir, 0700); err != nil {
		return fmt.Errorf("failed to create target directory: %w", err)
	}

	file, err := os.Open(backupPath)
	if err != nil {
		return fmt.Errorf("failed to open backup file: %w", err)
	}
	defer file.Close()

	gzReader, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar entry: %w", err)
		}

		targetPath := filepath.Join(targetDir, header.Name)

		if header.Typeflag == tar.TypeDir {
			if err := os.MkdirAll(targetPath, os.FileMode(header.Mode)); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", targetPath, err)
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(targetPath), 0700); err != nil {
			return fmt.Errorf("failed to create parent directory for %s: %w", targetPath, err)
		}

		outFile, err := os.Create(targetPath)
		if err != nil {
			return fmt.Errorf("failed to create file %s: %w", targetPath, err)
		}

		if _, err := io.Copy(outFile, tarReader); err != nil {
			outFile.Close()
			return fmt.Errorf("failed to write file %s: %w", targetPath, err)
		}

		if err := outFile.Chmod(os.FileMode(header.Mode)); err != nil {
			outFile.Close()
			return fmt.Errorf("failed to set permissions on %s: %w", targetPath, err)
		}

		outFile.Close()
	}

	return nil
}

func (bm *BackupManager) ListBackups() ([]BackupInfo, error) {
	entries, err := os.ReadDir(bm.backupDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read backup directory: %w", err)
	}

	var backups []BackupInfo
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".tar.gz") {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		backups = append(backups, BackupInfo{
			Name:      info.Name(),
			Path:      filepath.Join(bm.backupDir, info.Name()),
			Size:      info.Size(),
			CreatedAt: info.ModTime(),
		})
	}

	return backups, nil
}

func (bm *BackupManager) DeleteBackup(backupName string) error {
	path := filepath.Join(bm.backupDir, backupName)
	if !strings.HasPrefix(path, bm.backupDir) {
		return fmt.Errorf("invalid backup path")
	}
	return os.Remove(path)
}

func (bm *BackupManager) pruneOldBackups() {
	backups, err := bm.ListBackups()
	if err != nil || len(backups) <= bm.maxBackups {
		return
	}

	for i := 0; i < len(backups)-bm.maxBackups; i++ {
		os.Remove(backups[i].Path)
	}
}

type BackupInfo struct {
	Name      string
	Path      string
	Size      int64
	CreatedAt time.Time
}
