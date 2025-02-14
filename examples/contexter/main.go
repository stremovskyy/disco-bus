package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	dir := flag.String("dir", ".", "Directory to scan")
	flag.Parse()

	fmt.Println("Scanning directory:", *dir)

	var buffer bytes.Buffer
	var totalSize, fileCount int
	err := filepath.Walk(
		*dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				fmt.Println("Error accessing", path, err)
				return err
			}
			if info.IsDir() {
				if info.Name() == "vendor" || info.Name() == ".git" {
					return filepath.SkipDir
				}
				fmt.Println("Entering directory:", path)
				return nil
			}
			if strings.HasSuffix(info.Name(), ".go") {
				fmt.Println("Processing file:", path)
				content, err := readFileContent(path)
				if err != nil {
					fmt.Println("Error reading file", path, err)
					return err
				}
				fileData := "file " + path + "\n" + content + "\n"
				buffer.WriteString(fileData)
				totalSize += len(fileData)
				fileCount++
			}
			return nil
		},
	)

	if err != nil {
		fmt.Println("Error walking directory:", err)
		return
	}

	outputFile := "context_data.txt"
	err = os.WriteFile(outputFile, buffer.Bytes(), 0644)
	if err != nil {
		fmt.Println("Error writing file:", err)
		return
	}

	fmt.Println("Context file created:", outputFile, "Files processed:", fileCount, "Total Size:", totalSize, "bytes")
}

func readFileContent(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, strings.TrimSpace(scanner.Text()))
	}

	if err := scanner.Err(); err != nil {
		return "", err
	}

	return strings.Join(lines, "\n"), nil
}
