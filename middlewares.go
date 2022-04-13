package main

import (
	"context"
	"github.com/didip/tollbooth/v6"
	"net/http"
	"time"
)

func (app *Application) ratelimitMiddleware(next http.Handler) http.Handler {
	return tollbooth.LimitHandler(app.rateLimiter, next)
}

func (app *Application) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		app.logInfo.Println(r.RequestURI)
		next.ServeHTTP(w, r)
	})
}

// sets limit to body size
func (app *Application) bodySizeMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, app.config.MaxUploadSize)
		next.ServeHTTP(w, r)
	})
}

// parses form and pings database
func (app *Application) apiMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.ParseMultipartForm(app.config.MaxUploadSize) != nil {
			http.Error(w, "Too big file", http.StatusRequestEntityTooLarge)
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := app.db.PingContext(ctx); err != nil { // Makes sure database is alive
			app.logError.Println(err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// Makes sure its a valid admin token
func (app *Application) adminVerificationMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

		next.ServeHTTP(w, r)
	})
}

// Makes sure its a valid user token
func (app *Application) userVerificationMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !r.Form.Has("token") {
			http.Error(w, "Missing token field", http.StatusBadRequest)
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

		next.ServeHTTP(w, r)
	})
}

// Makes sure it's a valid upload token
func (app *Application) uploadTokenVerificationMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !r.Form.Has("upload_token") {
			http.Error(w, "Missing upload token", http.StatusBadRequest)
			return
		}

		uploadToken := r.FormValue("upload_token")

		// Makes sure the token is valid
		result, _, err := app.isValidUploadToken(uploadToken)
		if err != nil {
			app.logError.Println(err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		} else if !result {
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}
