package fileutils

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

// TestReadFileLines tests the ReadFileLines function
func TestReadFileLines(t *testing.T) {
	// Create a temporary file
	tempFile, err := ioutil.TempFile("", "readlines-test")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())

	// Write content to the file
	content := "Line 1\nLine 2\nLine 3\n"
	if _, err := tempFile.Write([]byte(content)); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tempFile.Close()

	// Read lines
	lines, err := ReadFileLines(tempFile.Name())
	if err != nil {
		t.Fatalf("ReadFileLines failed: %v", err)
	}

	// Check results - note: readlines may or may not preserve the final empty line
	// so we're flexible in what we expect
	if len(lines) < 3 {
		t.Errorf("Expected at least 3 lines, got %d", len(lines))
	}
	
	// Check the content of the first 3 lines
	expectedLines := []string{"Line 1", "Line 2", "Line 3"}
	for i, expected := range expectedLines {
		if i >= len(lines) {
			t.Errorf("Missing line %d", i+1)
			continue
		}
		if lines[i] != expected {
			t.Errorf("Line %d: expected '%s', got '%s'", i+1, expected, lines[i])
		}
	}

	// Test with non-existent file
	_, err = ReadFileLines("/non/existent/file.txt")
	if err == nil {
		t.Errorf("Expected error for non-existent file, got none")
	}
}

// TestDirIsEmpty tests the DirIsEmpty function
func TestDirIsEmpty(t *testing.T) {
	// Create a temporary directory
	tempDir, err := ioutil.TempDir("", "dirisempty-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Test empty directory
	isEmpty, err := DirIsEmpty(tempDir)
	if err != nil {
		t.Fatalf("DirIsEmpty failed: %v", err)
	}
	if !isEmpty {
		t.Errorf("Expected directory to be empty")
	}

	// Create a file in the directory
	filePath := filepath.Join(tempDir, "testfile.txt")
	if err := ioutil.WriteFile(filePath, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Test non-empty directory
	isEmpty, err = DirIsEmpty(tempDir)
	if err != nil {
		t.Fatalf("DirIsEmpty failed: %v", err)
	}
	if isEmpty {
		t.Errorf("Expected directory to be non-empty")
	}

	// Test with non-existent directory
	_, err = DirIsEmpty("/non/existent/dir")
	if err == nil {
		t.Errorf("Expected error for non-existent directory, got none")
	}

	// Test with a file instead of a directory
	_, err = DirIsEmpty(filePath)
	if err == nil {
		t.Errorf("Expected error when passing a file instead of directory, got none")
	}
}

// TestCompareFiles tests the CompareFiles function
func TestCompareFiles(t *testing.T) {
	// Create temporary files
	tempFile1, err := ioutil.TempFile("", "compare-test1")
	if err != nil {
		t.Fatalf("Failed to create temp file 1: %v", err)
	}
	defer os.Remove(tempFile1.Name())

	tempFile2, err := ioutil.TempFile("", "compare-test2")
	if err != nil {
		t.Fatalf("Failed to create temp file 2: %v", err)
	}
	defer os.Remove(tempFile2.Name())

	// Write the same content to both files
	content := "This is a test file for comparison."
	if _, err := tempFile1.Write([]byte(content)); err != nil {
		t.Fatalf("Failed to write to temp file 1: %v", err)
	}
	tempFile1.Close()

	if _, err := tempFile2.Write([]byte(content)); err != nil {
		t.Fatalf("Failed to write to temp file 2: %v", err)
	}
	tempFile2.Close()

	// Test identical files
	identical, err := CompareFiles(tempFile1.Name(), tempFile2.Name())
	if err != nil {
		t.Fatalf("CompareFiles failed: %v", err)
	}
	if !identical {
		t.Errorf("Expected files to be identical")
	}

	// Create a third file with different content
	tempFile3, err := ioutil.TempFile("", "compare-test3")
	if err != nil {
		t.Fatalf("Failed to create temp file 3: %v", err)
	}
	defer os.Remove(tempFile3.Name())

	if _, err := tempFile3.Write([]byte(content + "Different")); err != nil {
		t.Fatalf("Failed to write to temp file 3: %v", err)
	}
	tempFile3.Close()

	// Test different files
	identical, err = CompareFiles(tempFile1.Name(), tempFile3.Name())
	if err != nil {
		t.Fatalf("CompareFiles failed: %v", err)
	}
	if identical {
		t.Errorf("Expected files to be different")
	}

	// Test with non-existent file
	_, err = CompareFiles(tempFile1.Name(), "/non/existent/file.txt")
	if err == nil {
		t.Errorf("Expected error for non-existent file, got none")
	}

	// Test with the same file
	identical, err = CompareFiles(tempFile1.Name(), tempFile1.Name())
	if err != nil {
		t.Fatalf("CompareFiles failed with same file: %v", err)
	}
	if !identical {
		t.Errorf("Expected a file to be identical to itself")
	}
}

// TestCreateTempFile tests the CreateTempFile function
func TestCreateTempFile(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := ioutil.TempDir("", "createtemp-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Test creating a temporary file with content
	content := "This is test content."
	prefix := "test-prefix"
	suffix := ".txt"
	tempFile := CreateTempFile(t, tempDir, prefix, suffix, []byte(content))
	defer os.Remove(tempFile)

	// Verify the file exists
	if _, err := os.Stat(tempFile); os.IsNotExist(err) {
		t.Errorf("Temp file was not created or was removed")
	}

	// Read the content back
	fileContent, err := ioutil.ReadFile(tempFile)
	if err != nil {
		t.Fatalf("Failed to read temp file: %v", err)
	}

	// Check content matches
	if string(fileContent) != content {
		t.Errorf("Expected content '%s', got '%s'", content, string(fileContent))
	}

	// Verify file name has prefix and suffix
	fileName := filepath.Base(tempFile)
	if filepath.Ext(fileName) != suffix {
		t.Errorf("Expected file with %s extension, got '%s'", suffix, filepath.Ext(fileName))
	}
	if len(fileName) < len(prefix) || fileName[:len(prefix)] != prefix {
		t.Errorf("Expected file name with '%s' prefix, got '%s'", prefix, fileName)
	}
}