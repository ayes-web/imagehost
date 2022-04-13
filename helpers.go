package main

import (
	"github.com/dchest/uniuri"
	"github.com/h2non/filetype"
	"os"
)

func (app *Application) deleteFile(fileName string) (err error) {
	if app.s3client == nil { // Deletes from local storage
		err = os.Remove(app.config.DataFolder + fileName)
	} else { // Delete from s3
		err = app.deleteFileS3(fileName)
	}

	return
}

func randomString(fileNameLength int) string {
	return uniuri.NewLenChars(fileNameLength, []byte("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"))
}

func (app *Application) generateFullFileName(file []byte) (string, error) {
	extension, err := filetype.Get(file)
	if err != nil {
		return "", err
	}

	if extension.Extension == "unknown" { // Unknown file type defaults to txt
		return randomString(app.config.FileNameLength) + "." + "txt", nil
	}

	return randomString(app.config.FileNameLength) + "." + extension.Extension, nil
}
