package main

import (
	"net/http"
	"os"
	"path"
)

func (app *Application) apiList(w http.ResponseWriter, r *http.Request) {
	app.logger.Println(r.URL.Path, r.Header.Get("X-Forwarded-For"))

	if err := app.apiListTemplate.Execute(w, r.Host); err != nil {
		app.logger.Println(err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
}

func (app *Application) indexPage(w http.ResponseWriter, r *http.Request) {
	app.logger.Println(r.URL.Path, r.Header.Get("X-Forwarded-For"))

	if r.URL.Path == "/" {
		if err := app.indexTemplate.Execute(w, r.Host); err != nil {
			app.logger.Println(err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}

		return
	}

	// Looks if file exists in public folder then redirects there
	filePath := app.config.StaticFolder + path.Clean(r.URL.Path)
	if _, err := os.Stat(filePath); err == nil {
		http.Redirect(w, r, "/public/"+path.Clean(r.URL.Path), http.StatusPermanentRedirect)
		return
	}

	// Looks in database for uploaded file
	if exists, err := app.fileExists(path.Base(path.Clean(r.URL.Path))); err != nil {
		app.logger.Println(err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	} else if !exists {
		http.Redirect(w, r, "https://www.youtube.com/watch?v=dQw4w9WgXcQ", http.StatusFound)
		return
	}

	if app.s3client == nil {
		http.ServeFile(w, r, app.config.DataFolder+path.Clean(r.URL.Path))
	} else {
		http.Redirect(w, r, "https://"+app.config.S3.CdnDomain+"/file/"+app.config.S3.Bucket+path.Clean(r.URL.Path), http.StatusFound)
	}
}

func (app *Application) publicFiles(w http.ResponseWriter, r *http.Request) {
	app.logger.Println(r.URL.Path, r.Header.Get("X-Forwarded-For"))

	filePath := app.config.StaticFolder + path.Base(path.Clean(r.URL.Path))
	if _, err := os.Stat(filePath); err != nil {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}

	w.Header().Set("Cache-Control", "public, max-age=2592000")
	http.ServeFile(w, r, filePath)
}
