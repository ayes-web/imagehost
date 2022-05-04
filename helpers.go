package main

import (
	"errors"
	"github.com/dchest/uniuri"
	"github.com/h2non/filetype"
	"github.com/h2non/filetype/types"
	"github.com/jackc/pgx/v4"
	"os"
)

func (app *Application) deleteFile(fileName string) (err error) {
	switch app.fileStorageMethod {
	case fileStorageLocal:
		err = os.Remove(app.config.DataFolder + fileName)
	case fileStorageS3:
		err = app.deleteFileS3(fileName)
	default:
		err = ErrUnknownStorageMethod
	}

	return
}

func randomString(fileNameLength int) string {
	return uniuri.NewLenChars(fileNameLength, []byte("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"))
}

func (app *Application) generateFullFileName(file []byte) (name string, err error) {
	var extension types.Type
	extension, err = filetype.Get(file)
	if err != nil {
		return
	}

	if extension.Extension == "unknown" { // Unknown file type defaults to txt
		name = randomString(app.config.FileNameLength) + "." + "txt"
		return
	}

	name = randomString(app.config.FileNameLength) + "." + extension.Extension
	return
}

func (app *Application) isValidToken(token string) (bool, error) {
	if _, err := app.idByToken(token); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}

		return false, err
	}

	return true, nil
}
func (app *Application) isValidUploadToken(uploadToken string) (bool, error) {
	if _, err := app.idByUploadToken(uploadToken); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}

		return false, err
	}

	return true, nil
}
