package models

import (
	"archive/tar"
	"compress/bzip2"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

func extractTarBz2(archivePath, targetDir, expectedRoot string) error {
	archive, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("open archive: %w", err)
	}
	defer archive.Close()

	reader := tar.NewReader(bzip2.NewReader(archive))
	for {
		header, err := reader.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("read archive: %w", err)
		}

		relative, skip, err := archiveEntryPath(header.Name, expectedRoot)
		if err != nil {
			return err
		}
		if skip {
			if header.Typeflag != tar.TypeDir {
				return fmt.Errorf("archive root %q is not a directory", header.Name)
			}
			continue
		}

		target, err := pathInside(targetDir, relative)
		if err != nil {
			return fmt.Errorf("unsafe archive entry %q: %w", header.Name, err)
		}
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return fmt.Errorf("create directory %q: %w", relative, err)
			}
		case tar.TypeReg:
			if err := writeArchiveFile(target, reader); err != nil {
				return fmt.Errorf("extract %q: %w", relative, err)
			}
		case tar.TypeSymlink, tar.TypeLink:
			return fmt.Errorf("archive entry %q uses an unsupported link type", header.Name)
		default:
			return fmt.Errorf("archive entry %q has unsupported type %d", header.Name, header.Typeflag)
		}
	}
}

func archiveEntryPath(name, expectedRoot string) (relative string, skip bool, err error) {
	if name == "" || strings.ContainsRune(name, '\x00') {
		return "", false, fmt.Errorf("archive contains an invalid empty path")
	}
	name = strings.TrimPrefix(name, "./")
	native := filepath.FromSlash(name)
	if path.IsAbs(name) || filepath.IsAbs(native) || filepath.VolumeName(native) != "" {
		return "", false, fmt.Errorf("archive entry %q uses an absolute path", name)
	}
	cleaned := path.Clean(name)
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", false, fmt.Errorf("archive entry %q escapes its root", name)
	}
	if cleaned == expectedRoot {
		return "", true, nil
	}
	prefix := expectedRoot + "/"
	if !strings.HasPrefix(cleaned, prefix) {
		return "", false, fmt.Errorf("archive entry %q is outside expected root %q", name, expectedRoot)
	}
	relative = strings.TrimPrefix(cleaned, prefix)
	if relative == "" || relative == "." {
		return "", true, nil
	}
	return relative, false, nil
}

func pathInside(root, relative string) (string, error) {
	target := filepath.Join(root, filepath.FromSlash(relative))
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", fmt.Errorf("path escapes extraction directory")
	}
	return target, nil
}

func writeArchiveFile(target string, source io.Reader) (err error) {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, file.Close())
		if err != nil {
			os.Remove(target)
		}
	}()
	if _, err := io.Copy(file, source); err != nil {
		return err
	}
	return nil
}
