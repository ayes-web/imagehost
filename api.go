package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/h2non/filetype"
)

func (app *Application) isValidToken(token string) (bool, *int, error) {
	var id *int
	if err := app.db.QueryRow("SELECT id FROM accounts WHERE token=$1", token).Scan(&id); err != nil {
		if err.Error() != "sql: no rows in result set" {
			return false, nil, err
		}

		return false, nil, nil
	}

	return true, id, nil
}

func (app *Application) isValidUploadToken(token string) (bool, *int, error) {
	var id *int
	if err := app.db.QueryRow("SELECT id FROM accounts WHERE upload_token=$1", token).Scan(&id); err != nil {
		if err.Error() != "sql: no rows in result set" {
			return false, nil, err
		}

		return false, nil, nil
	}

	return true, id, nil
}

func (app *Application) fileExists(fileName string) (bool, error) {
	if err := app.db.QueryRow("SELECT FROM public.images WHERE file_name=$1", fileName).Scan(); err != nil {
		if err.Error() != "sql: no rows in result set" {
			return false, err
		}

		return false, nil
	}

	return true, nil
}

func (app *Application) apiCommons(w http.ResponseWriter, r *http.Request) bool {
	app.logger.Println(r.URL.Path)

	if r.Method == "POST" {
		r.Body = http.MaxBytesReader(w, r.Body, app.config.MaxUploadSize)

		if r.ParseMultipartForm(app.config.MaxUploadSize) != nil {
			http.Error(w, http.StatusText(http.StatusRequestEntityTooLarge), http.StatusRequestEntityTooLarge)
			return false
		}

		if err := app.db.Ping(); err != nil { // Makes sure database is alive
			app.logger.Println(err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return false
		}

		return true
	}

	http.NotFound(w, r)
	return false
}

// Api for deleting your own account
func (app *Application) accountDeleteApi(w http.ResponseWriter, r *http.Request) {
	if !app.apiCommons(w, r) {
		return
	}

	if !r.Form.Has("token") {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	token := r.FormValue("token")

	result, userId, err := app.isValidToken(token)
	if err != nil {
		app.logger.Println(err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	} else if !result {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}

	// Gets all images from account
	rows, err := app.db.Query("SELECT file_name FROM public.images WHERE file_owner=$1", userId)
	if err != nil {
		app.logger.Println(err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	for rows.Next() {
		var fileName string
		if err = rows.Scan(&fileName); err != nil {
			app.logger.Println(err)
			continue
		}

		if err = app.deleteFile(fileName); err != nil {
			app.logger.Println(err)
		}
	}

	if _, err = app.db.Exec("DELETE FROM public.images WHERE file_owner=$1", userId); err != nil {
		app.logger.Println(err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
}

// Api for deleting 1 image from your account
func (app *Application) deleteImageApi(w http.ResponseWriter, r *http.Request) {
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
	result, userId, err := app.isValidUploadToken(uploadToken)
	if err != nil {
		app.logger.Println(err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	} else if !result {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}

	// Makes sure the image exists
	if exists, err := app.fileExists(fileName); err != nil {
		app.logger.Println(err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	} else if !exists {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	if err = app.deleteFile(fileName); err != nil {
		app.logger.Println(err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	if _, err = app.db.Exec("DELETE FROM public.images WHERE file_name=$1 AND file_owner=$2", fileName, userId); err != nil {
		app.logger.Println(err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	if _, err = fmt.Fprintln(w, "Successfully deleted image"); err != nil {
		app.logger.Println(err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
}

// Api for uploading image
func (app *Application) uploadImageApi(w http.ResponseWriter, r *http.Request) {
	if !app.apiCommons(w, r) {
		return
	}

	if !r.Form.Has("upload_token") {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	uploadToken := r.FormValue("upload_token")

	// Makes sure the token is valid
	result, userId, err := app.isValidUploadToken(uploadToken)
	if err != nil {
		app.logger.Println(err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	} else if !result {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}

	fileRaw, _, err := r.FormFile("file")
	if err != nil { // This error occurs when user doesn't send anything on file
		http.Error(w, "No file provided", http.StatusBadRequest)
		return
	}

	file, err := io.ReadAll(fileRaw) // Reads the file into file variable
	if err != nil {
		app.logger.Println(err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	if filetype.IsApplication(file) {
		http.Error(w, "Unsupported file type", http.StatusUnsupportedMediaType)
		return
	}

	fullFileName, err := app.generateFullFileName(file)
	if err != nil {
		app.logger.Println(err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	if app.s3client == nil { // Uploads to local storage
		if err = os.WriteFile(app.config.DataFolder+fullFileName, file, 0644); err != nil {
			app.logger.Println(err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
	} else { // Uploads to bucket
		if _, err = app.s3client.PutObject(&s3.PutObjectInput{
			Body:   bytes.NewReader(file),
			Bucket: aws.String(app.config.S3.Bucket),
			Key:    aws.String(fullFileName),
		}); err != nil {
			app.logger.Println(err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
	}

	if _, err = app.db.Query(`INSERT INTO public.images (file_name, file_owner) VALUES ($1, $2)`, fullFileName, userId); err != nil {
		app.logger.Println(err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	if err = fileRaw.Close(); err != nil {
		app.logger.Println(err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "https://"+r.Host+"/"+fullFileName, http.StatusFound)

	if _, err = fmt.Fprintln(w, "https://"+r.Host+"/"+fullFileName); err != nil {
		app.logger.Println(err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
}

// Api for changing your upload token
func (app *Application) newUploadTokenApi(w http.ResponseWriter, r *http.Request) {
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
		app.logger.Println(err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	} else if !valid {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}

	var newToken string
	if err = app.db.QueryRow("UPDATE accounts SET upload_token=uuid_generate_v4() WHERE token=$1 RETURNING upload_token", token).Scan(&newToken); err != nil {
		if err.Error() != "sql: no rows in result set" {
			app.logger.Println(err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}

	if _, err = fmt.Fprintln(w, newToken); err != nil {
		app.logger.Println(err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
}
