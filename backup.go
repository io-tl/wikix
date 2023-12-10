package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

func addFileToTarWriter(filePath string, tarWriter *tar.Writer) (int64, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return 0, fmt.Errorf("could not open file '%s', error '%v'", filePath, err)
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return 0, fmt.Errorf("could not get stat for file '%s', error '%s'", filePath, err)
	}

	header, err := tar.FileInfoHeader(stat, stat.Name())
	if err != nil {
		return 0, err
	}
	header.Name = filePath

	err = tarWriter.WriteHeader(header)
	if err != nil {
		return 0, fmt.Errorf("could not write header for file '%s', got error '%v'", filePath, err)
	}

	bytesWritten, err := io.Copy(tarWriter, file)
	if err != nil {
		return 0, fmt.Errorf("could not copy the file '%s' data to the tarball, got error '%v'", filePath, err)
	}

	return bytesWritten, nil
}

func tarFiles() ([]string, error) {
	var files []string

	addFile := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			files = append(files, path)
		}
		return nil
	}

	if err := filepath.Walk(config["files"], addFile); err != nil {
		return nil, err
	}

	if err := filepath.Walk(config["pages"], addFile); err != nil {
		return nil, err
	}

	if err := filepath.Walk(config["gorm"], addFile); err != nil {
		log.Printf("ERROR no gorm database")
	}

	return files, nil
}

func CreateTarball(filePaths []string) (bytes.Buffer, error) {
	var buf bytes.Buffer
	var bufZ bytes.Buffer

	tarWriter := tar.NewWriter(&buf)

	for _, filePath := range filePaths {
		_, err := addFileToTarWriter(filePath, tarWriter)
		if err != nil {
			return buf, fmt.Errorf("could not add file '%s', to tarball, error '%v'", filePath, err)
		}
	}

	tarWriter.Close()

	gw := gzip.NewWriter(&bufZ)

	gw.Write(buf.Bytes())
	if err := gw.Close(); err != nil {
		return buf, fmt.Errorf("could not close gzip writer, error '%v'", err)
	}

	return bufZ, nil
}

func BackupHandler(w http.ResponseWriter, r *http.Request) {
	files, err := tarFiles()
	if err != nil {
		http.Error(w, fmt.Sprintf("Error enumerating files for backup : %v", err), http.StatusInternalServerError)
		return
	}
	tarball, err := CreateTarball(files)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error creating tarball : %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/gzip")
	ts := time.Now().Unix()
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=wiki.%d.tar.gz", ts))
	w.Write(tarball.Bytes())
}
