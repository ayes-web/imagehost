package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"
)

// Checks if the user is an admin with token
func (app *Application) isAdmin(token string) (bool, error) {
	var result string
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := app.db.QueryRowContext(ctx, "SELECT token FROM accounts WHERE token=$1 AND account_type='ADMIN'::account_type; ", token).Scan(&result); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, err
		}

		return false, nil
	}

	return result == token, nil
}

// Admin api for creating new user
func (app *Application) adminCreateUser(w http.ResponseWriter, r *http.Request) {
	if !r.Form.Has("token") {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	token := r.FormValue("token")

	if admin, err := app.isAdmin(token); err != nil {
		app.logError.Println(err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	} else if !admin {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}

	var newUser User
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := app.db.QueryRowContext(ctx, "INSERT INTO public.accounts DEFAULT values RETURNING token, upload_token, id, account_type").Scan(&newUser.Token, &newUser.UploadToken, &newUser.ID, &newUser.AccountType); err != nil {
		app.logError.Println(err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	data, err := json.MarshalIndent(newUser, "", "\t")
	if err != nil {
		app.logError.Println(err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	if _, err = fmt.Fprintln(w, string(data)); err != nil {
		app.logError.Println(err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
}

// Admin api for deleting user
func (app *Application) adminDeleteUser(w http.ResponseWriter, r *http.Request) {
	if !r.Form.Has("token") || !r.Form.Has("id") {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	token := r.FormValue("token")
	id := r.FormValue("id")

	if admin, err := app.isAdmin(token); err != nil {
		app.logError.Println(err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	} else if !admin {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err := app.db.ExecContext(ctx, "DELETE FROM public.accounts WHERE id=$1", id); err != nil {
		app.logError.Println(err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	ctx, cancel = context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	rows, err := app.db.QueryContext(ctx, "SELECT file_name FROM public.images WHERE file_uploader=$1", id)
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
	if _, err = app.db.ExecContext(ctx, "DELETE FROM public.images WHERE file_uploader=$1", id); err != nil {
		app.logError.Println(err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	if _, err = fmt.Fprintf(w, "User %s deleted\n", id); err != nil {
		app.logError.Println(err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
}
