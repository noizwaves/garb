package pkg

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strings"
)

const localBinPath = ".local/bin"

func unTgzFileNamed(binaryName string, data []byte) ([]byte, error) {
	decompressed, err := gzip.NewReader(bytes.NewBuffer(data))
	if err != nil {
		return nil, fmt.Errorf("Error decompressing Gzipped data: %w", err)
	}

	tarReader := tar.NewReader(decompressed)

	for {
		header, err := tarReader.Next()

		if err == io.EOF {
			break
		}

		if err != nil {
			return nil, fmt.Errorf("Error extracting from tar: %w", err)
		}

		switch header.Typeflag {
		case tar.TypeReg:
			_, archivedName := path.Split(header.Name)
			if archivedName != binaryName {
				continue
			}

			outData, err := io.ReadAll(tarReader)
			if err != nil {
				return nil, fmt.Errorf("Error extracting file from tar: %w", err)
			}
			return outData, nil
		}
	}

	return nil, fmt.Errorf("No file named %q found in archive", binaryName)
}

func unGzip(data []byte) ([]byte, error) {
	decompressed, err := gzip.NewReader(bytes.NewBuffer(data))
	if err != nil {
		return nil, fmt.Errorf("Error decompressing Gzipped data: %w", err)
	}

	return io.ReadAll(decompressed)
}

func fetchBinaryData(binaryName string, sourceUrl string) ([]byte, error) {
	resp, err := http.Get(sourceUrl)
	if err != nil {
		return nil, fmt.Errorf("Error requesting binary: %w", err)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("Error reading response body: %w", err)
	}

	if strings.HasSuffix(sourceUrl, ".tar.gz") || strings.HasSuffix(sourceUrl, ".tgz") {
		data, err = unTgzFileNamed(binaryName, data)

		if err != nil {
			return nil, fmt.Errorf("Error extracting binary from tgz archive: %w", err)
		}
	} else if strings.HasSuffix(sourceUrl, ".gz") {
		data, err = unGzip(data)

		if err != nil {
			return nil, fmt.Errorf("Error extracting binary from gzip archive: %w", err)
		}
	}

	return data, nil
}

func Install() error {
	config, err := loadConfig()
	if err != nil {
		return fmt.Errorf("Error loading config: %w", err)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("Error determining home directory: %w", err)
	}

	for _, binary := range config.Binaries {
		// if destination file exists
		destPath := path.Join(homeDir, localBinPath, binary.Name)
		if _, err := os.Stat(destPath); err == nil {
			fmt.Printf("%s already installed\n", binary.Name)
			continue
		}

		fmt.Printf("Installing %s...\n", binary.Name)

		// download and extract target URL
		sourceUrl, err := getSourceUrl(binary)
		if err != nil {
			return fmt.Errorf("Error getting source url for %s: %w", binary.Name, err)
		}

		data, err := fetchBinaryData(binary.Name, sourceUrl)
		if err != nil {
			return fmt.Errorf("Error downloading binary for %s: %w", binary.Name, err)
		}

		// write binary to file system
		err = os.WriteFile(destPath, data, 0744)
		if err != nil {
			return fmt.Errorf("Error writing binary to disk: %w", err)
		}

		fmt.Printf("%s has been installed\n", binary.Name)
	}

	return nil
}
