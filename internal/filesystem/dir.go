package filesystem

import (
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/comagnaw/regattaClock/internal/common"
)


func CreateDirs(dirPath string) error {
	if dirPath == common.EmptyString {
		return fmt.Errorf("filepath provided to create directory is empty and cannot be created")
	}
	if !DirExists(dirPath) {
		if err := os.MkdirAll(dirPath, 0755); err != nil {
			return err
		}
	}
	return nil
}

func DirExists(dirPath string) bool {
	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		return false
	}
	return true
}

func ReadDir(dirPath string) ([]fs.DirEntry, error) {
	if !DirExists(dirPath) {
		return nil, fmt.Errorf("directory %s does not exist and cannot be read", dirPath)
	}
	return os.ReadDir(dirPath)
}

func GetFilteredFilesInDir(dirPath, subStr string) ([]fs.DirEntry, error) {
	
	result := []fs.DirEntry{}
	listing, err := ReadDir(dirPath)
	if err != nil {
		return result, err
	}

	for _, f := range listing {
		if strings.Contains(f.Name(), subStr) {
			result = append(result, f)
		}
	}
	
	return result, nil

}