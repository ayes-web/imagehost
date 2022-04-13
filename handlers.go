package main

import (
	"net/http"
	"os"
	"path"
	"path/filepath"
)

func (app *Application) apiList(w http.ResponseWriter, r *http.Request) {
	if err := app.apiListTemplate.Execute(w, r.Host); err != nil {
		app.logError.Println(err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
}

func (app *Application) indexPage(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/" {
		if err := app.indexTemplate.Execute(w, r.Host); err != nil {
			app.logError.Println(err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}

		return
	}

	// Looks if file exists in public folder then redirects there
	filePath := app.config.StaticFolder + path.Clean(r.URL.Path)
	if _, err := os.Stat(filePath); err == nil {
		http.Redirect(w, r, path.Join("/public/", path.Clean(r.URL.Path)), http.StatusPermanentRedirect)
		return
	}

	// Looks in database for uploaded file
	if exists, err := app.fileExists(path.Base(path.Clean(r.URL.Path))); err != nil {
		app.logError.Println(err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	} else if !exists {
		http.Redirect(w, r, "https://www.youtube.com/watch?v=dQw4w9WgXcQ", http.StatusFound)
		return
	}

	if app.isUsingS3() {
		http.Redirect(w, r, "https://"+app.config.S3.CdnDomain+"/file/"+app.config.S3.Bucket+path.Clean(r.URL.Path), http.StatusFound)
	} else {
		http.ServeFile(w, r, filepath.Join(app.config.DataFolder, path.Clean(r.URL.Path)))
	}
}

func (app *Application) publicFiles(w http.ResponseWriter, r *http.Request) {
	filePath := filepath.Join(app.config.StaticFolder, path.Base(path.Clean(r.URL.Path)))

	if _, err := os.Stat(filePath); err != nil {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}

	w.Header().Set("Cache-Control", "public, max-age=2592000")
	http.ServeFile(w, r, filePath)
}
