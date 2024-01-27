package pack

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
)

func DownloadFile(url, fileName string) error {
	// Perform the HTTP request
	response, err := http.Get(url)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	// Check if the response status is successful (200 OK)
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP request failed with status: %s", response.Status)
	}

	// Get the current directory
	currentDir, err := os.Getwd()
	if err != nil {
		return err
	}

	// Save the content to a file in the current directory
	filePath := filepath.Join(currentDir, fileName)
	content, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(filePath, content, 0644)
	if err != nil {
		return err
	}

	return nil
}
