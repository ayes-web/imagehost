package main

import (
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/jackc/pgx/v4"
	"io"
	"net/http"
	"os"
)

// Api for deleting your own account
func (app *Application) accountDeleteAPI(c *gin.Context) {
	userID, err := app.idByToken(c.GetString("token"))
	if errors.Is(err, pgx.ErrNoRows) {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	} else if err != nil {
		app.logError.Println(err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	if err = app.deleteAccountWithImages(userID); err != nil {
		app.logError.Println(err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
}

func (app *Application) deleteAccountWithImages(userID int) (err error) {
	images, err := app.getAllImagesFromAccount(userID)
	if err != nil {
		return
	}

	for _, image := range images {
		if err = app.deleteFile(image.FileName); err != nil {
			app.logError.Println(err)
		}
	}

	if err = app.deleteImagesFromAccount(userID); err != nil {
		return
	}

	if err = app.deleteAccount(userID); err != nil {
		return
	}

	return
}

// Api for deleting an image from your account
type deleteImageAPIInput struct {
	FileName string `form:"file_name"`
}

func (app *Application) deleteImageAPI(c *gin.Context) {
	var input deleteImageAPIInput
	var err error

	if err = c.MustBindWith(&input, binding.FormPost); err != nil {
		c.String(http.StatusBadRequest, err.Error())
		c.Abort()
		return
	}

	// Makes sure the image exists
	var exists bool
	if exists, err = app.fileExists(input.FileName); err != nil {
		app.logError.Println(err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	} else if !exists {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	if err = app.deleteFile(input.FileName); err != nil { // Deletes file
		app.logError.Println(err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	if err = app.deleteImageUploadToken(input.FileName, c.GetString("uploadToken")); err != nil { // Deletes file entry from database
		app.logError.Println(err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	c.String(http.StatusOK, "Successfully deleted the image")
}

// Api for uploading image
func (app *Application) uploadImageAPI(c *gin.Context) {
	fileRaw, _, err := c.Request.FormFile("file")
	if err != nil {
		c.String(http.StatusBadRequest, "No file provided")
		c.Abort()
		return
	}

	file, err := io.ReadAll(fileRaw)
	if err != nil {
		app.logError.Println(err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	//if filetype.IsApplication(file) {
	//	c.AbortWithStatus(http.StatusUnsupportedMediaType)
	//	return
	//}

	fullFileName, err := app.generateFullFileName(file)
	if err != nil {
		app.logError.Println(err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	switch app.fileStorageMethod {
	case fileStorageS3:
		err = app.uploadFileS3(file, fullFileName)
	case fileStorageLocal:
		err = os.WriteFile(app.config.DataFolder+fullFileName, file, 0600)
	default:
		err = ErrUnknownStorageMethod
	}

	if err != nil {
		app.logError.Println(err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	if err = app.insertNewImageUploadToken(fullFileName, c.GetString("uploadToken")); err != nil {
		app.logError.Println(err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	c.Redirect(http.StatusTemporaryRedirect, "https://"+c.Request.Host+"/"+fullFileName)
}

// Api for changing your upload token
func (app *Application) newUploadTokenAPI(c *gin.Context) {
	uploadToken, err := app.replaceUploadToken(c.GetString("token"))
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			app.logError.Println(err)
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	c.String(http.StatusOK, uploadToken)
}
