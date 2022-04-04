package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/h2non/filetype"
)

func (app *Application) isValidToken(token string) (bool, *int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	var id *int
	if err := app.db.QueryRowContext(ctx, "SELECT id FROM accounts WHERE token=$1", token).Scan(&id); err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return false, nil, err
		}

		return false, nil, nil
	}

	return true, id, nil
}

func (app *Application) isValidUploadToken(token string) (bool, *int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	var id *int
	if err := app.db.QueryRowContext(ctx, "SELECT id FROM accounts WHERE upload_token=$1", token).Scan(&id); err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return false, nil, err
		}

		return false, nil, nil
	}

	return true, id, nil
}

func (app *Application) fileExists(fileName string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := app.db.QueryRowContext(ctx, "SELECT FROM public.images WHERE file_name=$1", fileName).Scan(); err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return false, err
		}

		return false, nil
	}

	return true, nil
}

func (app *Application) apiCommons(w http.ResponseWriter, r *http.Request) bool {
	app.logInfo.Println(r.URL.Path)

	if r.Method != "POST" {
		http.NotFound(w, r)
		return false
	}

	r.Body = http.MaxBytesReader(w, r.Body, app.config.MaxUploadSize)
	if r.ParseMultipartForm(app.config.MaxUploadSize) != nil {
		http.Error(w, http.StatusText(http.StatusRequestEntityTooLarge), http.StatusRequestEntityTooLarge)
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := app.db.PingContext(ctx); err != nil { // Makes sure database is alive
		app.logError.Println(err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return false
	}

	return true
}

// Api for deleting your own account
func (app *Application) accountDeleteAPI(w http.ResponseWriter, r *http.Request) {
	if !app.apiCommons(w, r) {
		return
	}

	if !r.Form.Has("token") {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	token := r.FormValue("token")

	result, userID, err := app.isValidToken(token)
	if err != nil {
		app.logError.Println(err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	} else if !result {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	rows, err := app.db.QueryContext(ctx, "SELECT file_name FROM public.images WHERE file_uploader=$1", userID)
	if err != nil {
		app.logError.Println(err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	for rows.Next() {
		var fileName string
		if err = rows.Scan(&fileName); err != nil {
			app.logError.Println(err)
			continue
		}

		if err = app.deleteFile(fileName); err != nil {
			app.logError.Println(err)
		}
	}

	ctx, cancel = context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err = app.db.ExecContext(ctx, "DELETE FROM public.images WHERE file_uploader=$1", userID); err != nil {
		app.logError.Println(err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
}

// Api for deleting 1 image from your account
func (app *Application) deleteImageAPI(w http.ResponseWriter, r *http.Request) {
	if !app.apiCommons(w, r) {
		return
	}

	if !r.Form.Has("upload_token") || !r.Form.Has("file_name") {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	uploadToken := r.FormValue("upload_token")
	fileName := r.FormValue("file_name")

	// Makes sure the upload token is valid
	result, userID, err := app.isValidUploadToken(uploadToken)
	if err != nil {
		app.logError.Println(err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	} else if !result {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}

	// Makes sure the image exists
	if exists, err := app.fileExists(fileName); err != nil {
		app.logError.Println(err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	} else if !exists {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	if err = app.deleteFile(fileName); err != nil {
		app.logError.Println(err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err = app.db.ExecContext(ctx, "DELETE FROM public.images WHERE file_name=$1 AND file_uploader=$2", fileName, userID); err != nil {
		app.logError.Println(err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	if _, err = fmt.Fprintln(w, "Successfully deleted image"); err != nil {
		app.logError.Println(err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
}

// Api for uploading image
func (app *Application) uploadImageAPI(w http.ResponseWriter, r *http.Request) {
	if !app.apiCommons(w, r) {
		return
	}

	if !r.Form.Has("upload_token") {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	uploadToken := r.FormValue("upload_token")

	// Makes sure the token is valid
	result, userID, err := app.isValidUploadToken(uploadToken)
	if err != nil {
		app.logError.Println(err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	} else if !result {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}

	fileRaw, _, err := r.FormFile("file")
	if err != nil { // Occurs when user doesn't provide a file
		http.Error(w, "No file provided", http.StatusBadRequest)
		return
	}

	file, err := io.ReadAll(fileRaw) // Reads the file into file variable
	if err != nil {
		app.logError.Println(err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	if filetype.IsApplication(file) {
		http.Error(w, "Unsupported file type", http.StatusUnsupportedMediaType)
		return
	}

	fullFileName, err := app.generateFullFileName(file)
	if err != nil {
		app.logError.Println(err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	if app.isUsingS3() { // Uploads to bucket
		err = app.uploadFileS3(file, fullFileName)
	} else { // Uploads to local storage
		err = os.WriteFile(app.config.DataFolder+fullFileName, file, 0600)
	}

	if err != nil {
		app.logError.Println(err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err = app.db.QueryContext(ctx, `INSERT INTO public.images (file_name, file_uploader) VALUES ($1, $2)`, fullFileName, userID); err != nil {
		app.logError.Println(err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	if err = fileRaw.Close(); err != nil {
		app.logError.Println(err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "https://"+r.Host+"/"+fullFileName, http.StatusFound)
}

// Api for changing your upload token
func (app *Application) newUploadTokenAPI(w http.ResponseWriter, r *http.Request) {
	if !app.apiCommons(w, r) {
		return
	}

	if !r.Form.Has("token") {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	token := r.FormValue("token")

	valid, _, err := app.isValidToken(token)
	if err != nil {
		app.logError.Println(err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	} else if !valid {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}

	var newToken string
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err = app.db.QueryRowContext(ctx, "UPDATE accounts SET upload_token=uuid_generate_v4() WHERE token=$1 RETURNING upload_token", token).Scan(&newToken); err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			app.logError.Println(err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}

	if _, err = fmt.Fprintln(w, newToken); err != nil {
		app.logError.Println(err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
}
