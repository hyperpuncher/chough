package models

import (
	"archive/tar"
	"compress/bzip2"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const (
	DefaultModelName = "sherpa-onnx-nemo-parakeet-tdt-0.6b-v3-int8"
	ModelURL         = "https://github.com/k2-fsa/sherpa-onnx/releases/download/asr-models/sherpa-onnx-nemo-parakeet-tdt-0.6b-v3-int8.tar.bz2"
)

// GetModelPath returns the path to the model directory, downloading if necessary
func GetModelPath() (string, error) {
	// 1. Check CHOUGH_MODEL env var
	if envPath := os.Getenv("CHOUGH_MODEL"); envPath != "" {
		if isValidModel(envPath) {
			return envPath, nil
		}
		fmt.Fprintf(os.Stderr, "Warning: CHOUGH_MODEL=%s not found or invalid\n", envPath)
	}

	// 2. Check cache directory
	cacheDir := getCacheDir()
	modelDir := filepath.Join(cacheDir, "chough", "models", DefaultModelName)

	if isValidModel(modelDir) {
		return modelDir, nil
	}

	// 3. Download model
	fmt.Fprintf(os.Stderr, "Downloading model to %s...\n", modelDir)
	if err := downloadAndExtract(modelDir); err != nil {
		return "", fmt.Errorf("failed to download model: %w", err)
	}

	return modelDir, nil
}

func getCacheDir() string {
	if xdg := os.Getenv("XDG_CACHE_HOME"); xdg != "" {
		return xdg
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".cache"
	}
	return filepath.Join(home, ".cache")
}

func isValidModel(path string) bool {
	required := []string{"encoder.int8.onnx", "decoder.int8.onnx", "joiner.int8.onnx", "tokens.txt"}
	for _, file := range required {
		if _, err := os.Stat(filepath.Join(path, file)); err != nil {
			return false
		}
	}
	return true
}

func downloadAndExtract(targetDir string) error {
	tmpFile, err := os.CreateTemp("", "chough-model-*.tar.bz2")
	if err != nil {
		return err
	}
	defer os.Remove(tmpFile.Name())

	resp, err := http.Get(ModelURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: %s", resp.Status)
	}

	// Download with single-line progress bar
	size := resp.ContentLength
	written := int64(0)
	buf := make([]byte, 64*1024)
	lastPercent := -1

	for {
		nr, rerr := resp.Body.Read(buf)
		if nr > 0 {
			nw, werr := tmpFile.Write(buf[:nr])
			if nw > 0 {
				written += int64(nw)
			}
			if werr != nil {
				return werr
			}
			// Update progress every 5%
			if size > 0 {
				percent := int(float64(written) * 100 / float64(size))
				if percent != lastPercent && percent%5 == 0 {
					mb := float64(written) / (1024 * 1024)
					totalMb := float64(size) / (1024 * 1024)
					fmt.Fprintf(os.Stderr, "\r  Downloading: %.1f / %.1f MB (%d%%)", mb, totalMb, percent)
					lastPercent = percent
				}
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			return rerr
		}
	}
	tmpFile.Close()
	fmt.Fprintln(os.Stderr) // New line after progress

	fmt.Fprintf(os.Stderr, "Extracting...\n")

	if err := extractTarBz2(tmpFile.Name(), targetDir); err != nil {
		return fmt.Errorf("extraction failed: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Model ready\n")
	return nil
}

func extractTarBz2(archivePath, targetDir string) error {
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return err
	}

	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer file.Close()

	bz2Reader := bzip2.NewReader(file)
	tarReader := tar.NewReader(bz2Reader)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		// Strip root directory from path
		cleanName := filepath.Clean(header.Name)
		parts := strings.SplitN(cleanName, string(filepath.Separator), 2)
		if len(parts) == 1 {
			// Root directory entry - skip
			continue
		}
		cleanName = parts[1]
		if cleanName == "" || cleanName == "." {
			continue
		}

		target := filepath.Join(targetDir, cleanName)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			outFile, err := os.Create(target)
			if err != nil {
				return err
			}
			if _, err := io.Copy(outFile, tarReader); err != nil {
				outFile.Close()
				return err
			}
			outFile.Close()
		}
	}

	return nil
}
