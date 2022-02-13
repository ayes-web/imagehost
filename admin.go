package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
)

// Checks if the user is an admin with token
func is_admin(db *sql.DB, token string) bool {
	var result string
	if db.QueryRow("SELECT token FROM accounts WHERE token=$1 AND account_type='ADMIN'; ", token).Scan(&result) != nil {
		return false
	}

	return result == token
}

// Admin api for creating new user
func admin_create_user(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	if !r.Form.Has("token") {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	token := r.FormValue("token")

	if !is_admin(db, token) {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}

	var new_user User
	if db.QueryRow("INSERT INTO public.accounts DEFAULT values RETURNING token, upload_token, id, account_type").Scan(&new_user.Token, &new_user.Upload_token, &new_user.Id, &new_user.Account_type) != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	json, err := json.MarshalIndent(new_user, "", "\t")
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	fmt.Fprintln(w, string(json))
}

// Admin api for deleting user
func admin_delete_user(w http.ResponseWriter, r *http.Request, db *sql.DB, config Config) {
	if !r.Form.Has("token") || !r.Form.Has("id") {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	token := r.FormValue("token")
	id := r.FormValue("id")

	if !is_admin(db, token) {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}

	db.Exec("DELETE FROM public.accounts WHERE id=$1", id)

	// Gets all images from account
	rows, err := db.Query("SELECT file_name FROM public.images WHERE file_owner=$1", id)
	if err != nil { // Im guessing this happens when it gets no results
		return
	}

	for rows.Next() {
		var file_name string
		rows.Scan(&file_name)

		delete_file(config, file_name)
	}

	db.Exec("DELETE FROM public.images WHERE file_owner=$1", id)
	fmt.Fprintf(w, "User %s deleted\n", id)
}
