package main

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"path"
	"path/filepath"
)

func (app *Application) apiList(c *gin.Context) {
	if err := app.apiListTemplate.Execute(c.Writer, c.Request.Host); err != nil {
		app.logError.Println(err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
}

func (app *Application) indexPage(c *gin.Context) {
	if err := app.indexTemplate.Execute(c.Writer, c.Request.Host); err != nil {
		app.logError.Println(err)
		c.AbortWithStatus(http.StatusInternalServerError)
	}
}

func (app *Application) indexFiles(c *gin.Context) {
	c.Status(http.StatusOK)

	// Looks if file exists in public folder then redirects there
	filePath := filepath.Join("public", path.Clean(c.Request.URL.Path))
	if file, err := publicFiles.Open(filePath); err == nil {
		file.Close()
		c.Redirect(http.StatusPermanentRedirect, path.Join("public", path.Clean(c.Request.URL.Path)))
		return
	}

	// Looks in database for uploaded file
	if exists, err := app.db.fileExists(path.Base(path.Clean(c.Request.URL.Path))); err != nil {
		app.logError.Println(err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	} else if !exists {
		c.Redirect(http.StatusTemporaryRedirect, "https://www.youtube.com/watch?v=dQw4w9WgXcQ")
		return
	}

	switch app.config.fileStorageMethod {
	case fileStorageS3:
		c.Redirect(http.StatusTemporaryRedirect, "https://"+app.config.S3.CdnDomain+"/file/"+app.config.S3.Bucket+path.Clean(c.Request.URL.Path))
	case fileStorageLocal:
		c.File(filepath.Join(app.config.DataFolder, path.Clean(c.Request.URL.Path)))
	default:
		app.logError.Println(ErrUnknownStorageMethod)
		c.AbortWithStatus(http.StatusInternalServerError)
	}
}
