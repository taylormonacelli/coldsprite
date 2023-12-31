package main

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ulikunitz/xz"

	"github.com/taylormonacelli/tidehead"
)

// Define a struct that matches the JSON data structure
type Manifest struct {
	TimeEpoch   int64  `json:"TimeEpoch"`
	TimeRFC3339 string `json:"TimeRFC3339"`
	SHA         string `json:"SHA"`
	ShortSHA    string `json:"ShortSHA"`
	FileName    string `json:"FileName"`
	RepoHost    string `json:"RepoHost"`
	RepoPath    string `json:"RepoPath"`
}

func findMatchingFiles(directory string) ([]string, error) {
	var matchingFiles []string

	// Walk through the directory and its subdirectories
	err := filepath.Walk(directory, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Check if the file matches the desired format
		if info.IsDir() {
			return nil // Skip directories
		}
		if match, _ := filepath.Match("manifest_*.json", info.Name()); match {
			matchingFiles = append(matchingFiles, path)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return matchingFiles, nil
}

func expandXZFile(logger *slog.Logger, inputFile string, outputDir string) error {
	logger.Debug("Expanding XZ file %s to %s", inputFile, outputDir)

	// Create the output directory if it doesn't exist
	if err := os.MkdirAll(outputDir, os.ModePerm); err != nil {
		return err
	}

	// Open the XZ-compressed file for reading
	file, err := os.Open(inputFile)
	if err != nil {
		return err
	}
	defer file.Close()

	// Create an XZ reader
	r, err := xz.NewReader(file)
	if err != nil {
		return err
	}

	// Create the output file
	outputFile := filepath.Join(outputDir, filepath.Base(inputFile))

	// Determine the output filename without extension
	outputFile = strings.TrimSuffix(filepath.Base(outputFile), filepath.Ext(outputFile))

	// Create the output filename with the ".tar" extension
	outputFile = filepath.Join(outputDir, outputFile)

	output, err := os.Create(outputFile)
	if err != nil {
		return err
	}
	defer output.Close()

	// Copy the decompressed data to the output file
	_, err = io.Copy(output, r)
	if err != nil {
		return err
	}

	// Check if the expanded file has a .tar extension
	if strings.HasSuffix(outputFile, ".tar") {
		// The expanded file is a TAR file, so we should expand it
		tarOutputDir := outputDir // Use the same output directory
		if err := expandTarFile(logger, outputFile, tarOutputDir); err != nil {
			logger.Debug(fmt.Sprintf("Error expanding TAR file %s: %v", outputFile, err))
		} else {
			logger.Debug(fmt.Sprintf("TAR File expanded successfully: %s -> %s", outputFile, tarOutputDir))
		}
	}

	return nil
}

func expandTarGzFile(logger *slog.Logger, inputFile string, outputDir string) error {
	logger.Debug(fmt.Sprintf("Expanding TAR GZ file %s to %s", inputFile, outputDir))

	// Create the output directory if it doesn't exist
	if err := os.MkdirAll(outputDir, os.ModePerm); err != nil {
		return err
	}

	// Open the TAR GZ file for reading
	file, err := os.Open(inputFile)
	if err != nil {
		return err
	}
	defer file.Close()

	// Create a GZIP reader
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzipReader.Close()

	// Create a TAR reader
	tarReader := tar.NewReader(gzipReader)

	// Extract the contents of the TAR file
	for {
		header, err := tarReader.Next()

		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		// Construct the output path
		outputPath := filepath.Join(outputDir, header.Name)

		// Check if the file is a directory or a regular file
		if header.FileInfo().IsDir() {
			// Create directories as needed
			os.MkdirAll(outputPath, os.ModePerm)
		} else {
			// Create the output file
			outputFile, err := os.Create(outputPath)
			if err != nil {
				return err
			}
			defer outputFile.Close()

			// Copy the file content from the TAR archive to the output file
			_, err = io.Copy(outputFile, tarReader)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func expandTarFile(logger *slog.Logger, inputFile string, outputDir string) error {
	logger.Debug(fmt.Sprintf("Expanding TAR file %s to %s", inputFile, outputDir))

	// Create the output directory if it doesn't exist
	if err := os.MkdirAll(outputDir, os.ModePerm); err != nil {
		return err
	}

	// Open the TAR file for reading
	file, err := os.Open(inputFile)
	if err != nil {
		return err
	}
	defer file.Close()

	// Create a TAR reader
	tarReader := tar.NewReader(file)

	// Extract the contents of the TAR file
	for {
		header, err := tarReader.Next()

		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		// Construct the output path
		outputPath := filepath.Join(outputDir, header.Name)

		// Check if the file is a directory or a regular file
		if header.FileInfo().IsDir() {
			// Create directories as needed
			os.MkdirAll(outputPath, os.ModePerm)
		} else {
			// Create the output file
			outputFile, err := os.Create(outputPath)
			if err != nil {
				return err
			}
			defer outputFile.Close()

			// Copy the file content from the TAR archive to the output file
			_, err = io.Copy(outputFile, tarReader)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func epochToDuration(i int64) string {
	e := time.Unix(i, 0)
	now := time.Now()
	duration := now.Sub(e)

	friendly := tidehead.FormatDuration(duration)
	return friendly
}

func main() {
	handlerIngoreDebug := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn})
	loggerIgnoreDebug := slog.New(handlerIngoreDebug)
	slog.SetDefault(loggerIgnoreDebug)
	slog.Info("Info message")

	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		AddSource: true,
		Level:     slog.LevelDebug,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.SourceKey {
				source, _ := a.Value.Any().(*slog.Source)
				if source != nil {
					source.File = filepath.Base(source.File)
				}
			}
			// Remove time.
			if a.Key == slog.TimeKey && len(groups) == 0 {
				return slog.Attr{}
			}
			return a
		},
	})

	logger := slog.New(handler)

	// Specify the directory to search
	searchDirectory := "data/logs"

	// Call the function to find matching files
	matchingFiles, err := findMatchingFiles(searchDirectory)
	if err != nil {
		logger.Debug(fmt.Sprintf("Error finding matching files: %v", err))
		return
	}

	// Initialize a slice to hold the unmarshaled data
	var manifests []Manifest

	// Loop through matching files and unmarshal them into structs
	for _, filePath := range matchingFiles {
		// Open the file for reading
		file, err := os.Open(filePath)
		if err != nil {
			logger.Debug(fmt.Sprintf("Error opening file %s: %v", filePath, err))
			continue
		}
		defer file.Close()

		// Initialize a variable to hold the unmarshaled data
		var manifest Manifest

		// Read the file contents
		data, err := io.ReadAll(file)
		if err != nil {
			logger.Debug(fmt.Sprintf("Error reading file %s: %v", filePath, err))
			continue
		}

		// Unmarshal the JSON data into the struct
		if err := json.Unmarshal(data, &manifest); err != nil {
			logger.Debug(fmt.Sprintf("Error unmarshaling JSON from file %s: %v", filePath, err))
			continue
		}

		// Append the unmarshaled struct to the list
		manifests = append(manifests, manifest)
	}

	// Process the list of unmarshaled data as needed
	for _, manifest := range manifests {
		// Check if the directory exists
		outputDir := filepath.Join("data/logs/expanded", strconv.FormatInt(manifest.TimeEpoch, 10))
		outputDir, _ = filepath.Abs(outputDir)
		tarPath := filepath.Join(outputDir, manifest.FileName)

		d := epochToDuration(manifest.TimeEpoch)
		logger.Debug(fmt.Sprintf("%s: checking existance of directory: %s", d, outputDir))
		if _, err := os.Stat(outputDir); os.IsNotExist(err) {
			// Directory does not exist, expand the file based on file extension

			d := epochToDuration(manifest.TimeEpoch)
			logger.Debug(fmt.Sprintf("%s age %s ago", tarPath, d))

			inputFile := fmt.Sprintf("data/logs/%s", manifest.FileName)

			// Determine the file extension and expand accordingly
			fileExtension := strings.ToLower(filepath.Ext(inputFile))
			switch fileExtension {
			case ".xz":
				if err := expandXZFile(logger, inputFile, outputDir); err != nil {
					logger.Debug(fmt.Sprintf("Error expanding XZ file %s: %v", inputFile, err))
				} else {
					logger.Debug(fmt.Sprintf("XZ File expanded successfully: %s -> %s", inputFile, outputDir))
				}
			case ".gz":
				if err := expandTarGzFile(logger, inputFile, outputDir); err != nil {
					logger.Debug(fmt.Sprintf("Error expanding TAR GZ file %s: %v", inputFile, err))
				} else {
					logger.Debug(fmt.Sprintf("TAR GZ File expanded successfully: %s -> %s", inputFile, outputDir))
				}
			case ".tar":
				if err := expandTarFile(logger, inputFile, outputDir); err != nil {
					logger.Debug(fmt.Sprintf("Error expanding TAR file %s: %v", inputFile, err))
				} else {
					logger.Debug(fmt.Sprintf("TAR File expanded successfully: %s -> %s", inputFile, outputDir))
				}
			default:
				logger.Debug(fmt.Sprintf("Unsupported file format for %s", inputFile))
			}
		}
	}
}
