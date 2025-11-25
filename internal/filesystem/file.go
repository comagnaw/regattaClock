package filesystem

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/comagnaw/regattaClock/internal/common"
)

func SaveFile(contents, filename string) error {
	newFile, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("filename %s could not be created: %s", filename, err)
	}
	err = newFile.Chmod(os.FileMode(0644))
	if err != nil {
		return fmt.Errorf("filename %s could not be chmod: %s", filename, err)
	}

	defer newFile.Close()
	_, err = newFile.Write([]byte(contents))
	if err != nil {
		return fmt.Errorf("filename %s could not be written: %s", filename, err)
	}
	return nil
}

func SaveJSONFile(data interface{}, filename string) error {
	fileBytes, err := json.MarshalIndent(data, common.EmptyString, "  ")
	if err != nil {
		return fmt.Errorf("data could not be marshaled into filename %s:%s", filename, err)
	}

	err = os.WriteFile(filename, fileBytes, 0644)
	if err != nil {
		return fmt.Errorf("filename %s could not be written: %s", filename, err)
	}
	return nil
}

func ReadJSONFile(data interface{}, filename string) error {
	fileBytes, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("filename %s could not be read: %s", filename, err)
	}
	err = json.Unmarshal(fileBytes, &data)
	if err != nil {
		return fmt.Errorf("filename %s could not be unmarshalled: %s", filename, err)
	}
	return nil
}