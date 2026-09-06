package filesystem

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"

	"github.com/comagnaw/regattaClock/internal/common"
)

func SaveJSONFile(data interface{}, filename string) error {
	fileBytes, err := json.MarshalIndent(data, common.EmptyString, "  ")
	if err != nil {
		return fmt.Errorf("data could not be marshaled into filename %s:%s", filename, err)
	}

	err = os.WriteFile(filename, fileBytes, 0644)
	if err != nil {
		return fmt.Errorf("filename %s could not be written: %w", filename, err)
	}
	return nil
}

func ReadJSONFile(data interface{}, filename string) error {
	fileBytes, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("filename %s could not be read: %w", filename, err)
	}
	err = json.Unmarshal(fileBytes, data)
	if err != nil {
		return fmt.Errorf("filename %s could not be unmarshalled: %s", filename, err)
	}
	return nil
}

func FileHash(filename string) (string, error) {
	fileBytes, err := os.ReadFile(filename)
	if err != nil {
		return "", fmt.Errorf("filename %s could not be read: %s", filename, err)
	}

	h := sha256.New()

	_, err = h.Write(fileBytes)
	if err != nil {
		return "", fmt.Errorf("filename %s could not be hashed: %s", filename, err)
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}
