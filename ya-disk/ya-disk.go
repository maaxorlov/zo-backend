package ya_disk

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	yd "github.com/nikitaksv/yandex-disk-sdk-go"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type YaDisk struct{ yd.YaDisk }

func InitYaDisk(accessToken string) (*YaDisk, error) {
	errExplanation := "can't init Yandex Disk client"

	disk, err := yd.NewYaDisk(context.Background(), http.DefaultClient, &yd.Token{AccessToken: accessToken})
	if err != nil {
		return nil, errWithExplanation(errExplanation, err)
	}

	return &YaDisk{disk}, nil
}

func (d *YaDisk) GetYaDiskFolder(folderName string, checkForEmpty bool) error {
	errExplanation := "getting Yandex Disk folder error"

	folder, err := d.GetResource(folderName, nil, 1e9, 0, false, "", "")
	if err != nil {
		return errWithExplanation(errExplanation, err)
	}

	if checkForEmpty && len(folder.Embedded.Items) != 0 {
		err = fmt.Errorf("the folder for this event is not empty (try to clear or delete and restart)")
		return errWithExplanation(errExplanation, err)
	}

	return nil
}

func (d *YaDisk) CreateYaDiskFolder(folderName string) error {
	errExplanation := "creating Yandex Disk folder error"

	err := error(nil)
	_, err = d.CreateResource(folderName, nil)
	if err != nil {
		return errWithExplanation(errExplanation, err)
	}

	return nil
}

func (d *YaDisk) LoadToYaDisk(fileName, localDir, remoteDir string, overwrite bool) (string, error) {
	errExplanation := "loading to Yandex Disk error"

	remoteFilePath := strings.Replace(filepath.Join(remoteDir, fileName), "\\", "/", -1)
	uploadLink, err := d.GetResourceUploadLink(remoteFilePath, nil, overwrite)
	if err != nil {
		return "", errWithExplanation(errExplanation, err)
	}

	wd, err := os.Getwd()
	if err != nil {
		return "", errWithExplanation(errExplanation, err)
	}

	localFilePath := filepath.Join(wd, localDir, fileName)
	data, err := readLocalFile(localFilePath)
	if err != nil {
		return "", errWithExplanation(errExplanation, err)
	}

	pu, err := d.PerformUpload(uploadLink, data)
	if err != nil {
		return "", errWithExplanation(errExplanation, err)
	}

	if pu == nil {
		err = fmt.Errorf("nil pu (perform upload) caused by error %v", err)
		return "", errWithExplanation(errExplanation, err)
	}

	_, err = d.PublishResource(remoteFilePath, nil)
	if err != nil {
		return "", errWithExplanation(errExplanation, err)
	}

	fileInfo, err := d.GetResource(remoteFilePath, nil, 1, 0, false, "", "")
	if err != nil {
		return "", errWithExplanation(errExplanation, err)
	}

	if fileInfo.PublicURL == "" {
		err = fmt.Errorf("empty public url")
		return "", errWithExplanation(errExplanation, err)
	}

	return fileInfo.PublicURL, nil
}

func errWithExplanation(errExplanation string, err error) error {
	return fmt.Errorf("%s: %+v", errExplanation, err)
}

func readLocalFile(path string) (*bytes.Buffer, error) {
	data, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	defer func() {
		if err := data.Close(); err != nil {
			panic(err)
		}
	}()

	reader, buffer, part := bufio.NewReader(data), bytes.NewBuffer(make([]byte, 0)), make([]byte, 1024)
	for {
		var count int
		if count, err = reader.Read(part); err != nil {
			break
		}

		buffer.Write(part[:count])
	}

	if err != io.EOF {
		return nil, fmt.Errorf("error reading %s: %v", filepath.Base(path), err)
	}

	s := cap(buffer.Bytes())
	if s > 100 {
		s = 100
	}

	return buffer, nil
}
