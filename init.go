package main

import (
	"github.com/BurntSushi/toml"
	"github.com/gorilla/mux"
	"html/template"
	"log"
	"net/http"
	"os"
)

func (app *Application) setupLogging() {
	flags := log.Ldate | log.Ltime | log.Lshortfile | log.Lmsgprefix

	app.logInfo = log.New(os.Stdout, "INFO: ", flags)
	app.logError = log.New(os.Stdout, "ERROR: ", flags)

	app.logInfo.Println("Setup logging")
}

func (app *Application) initializeConfig(configLocation string) {
	rawConfig, err := os.ReadFile(configLocation)
	if err != nil {
		app.logError.Fatal(err)
	}

	if _, err = toml.Decode(string(rawConfig), &app.config); err != nil {
		app.logError.Fatal(err)
	}

	if app.config.S3 == (s3Config{}) {
		app.logInfo.Println("Storing files in", app.config.DataFolder)

		if file, _ := os.Stat(app.config.DataFolder); file == nil {
			app.logInfo.Println("Creating data folder")

			if err = os.Mkdir(app.config.DataFolder, 0777); err != nil {
				app.logError.Fatal(err)
			}
		}
	} else {
		app.logInfo.Println("Storing files in s3 bucket")
		app.prepareS3()
	}
}

func (app *Application) setupTemplates() {
	app.logInfo.Println("Setting up templates")

	var err error
	app.apiListTemplate, err = template.New("api_list.html").ParseFiles(app.config.TemplateFolder + "api_list.html")
	if err != nil {
		app.logInfo.Fatal(err)
	}

	app.indexTemplate, err = template.New("index.html").ParseFiles(app.config.TemplateFolder + "index.html")
	if err != nil {
		app.logInfo.Fatal(err)
	}
}

func (app *Application) initializeRouter() (r *mux.Router) {
	app.logInfo.Println("Setting up router")
	r = mux.NewRouter()

	r.Use(app.ratelimitMiddleware)
	r.Use(app.bodySizeMiddleware)
	r.Use(app.loggingMiddleware)

	api := r.PathPrefix("/api").Subrouter()
	api.Use(app.apiMiddleware)

	miscAPI := api.PathPrefix("/").Subrouter()
	miscAPI.Use(app.uploadTokenVerificationMiddleware)
	miscAPI.HandleFunc("/upload", app.uploadImageAPI).Methods(http.MethodPost)
	miscAPI.HandleFunc("/delete", app.deleteImageAPI).Methods(http.MethodPost)

	accountAPI := api.PathPrefix("/account").Subrouter()
	accountAPI.Use(app.userVerificationMiddleware)
	accountAPI.HandleFunc("/new_upload_token", app.newUploadTokenAPI).Methods(http.MethodPost)
	accountAPI.HandleFunc("/delete", app.accountDeleteAPI).Methods(http.MethodPost)

	adminAPI := api.PathPrefix("/admin").Subrouter()
	adminAPI.Use(app.adminVerificationMiddleware)
	adminAPI.HandleFunc("/create_user", app.adminCreateUser).Methods(http.MethodPost)
	adminAPI.HandleFunc("/delete_user", app.adminCreateUser).Methods(http.MethodPost)

	r.Path("/api_list").HandlerFunc(app.apiList).Methods(http.MethodGet)

	r.PathPrefix("/public/").HandlerFunc(app.publicFiles).Methods(http.MethodGet)
	r.PathPrefix("/").HandlerFunc(app.indexPage).Methods(http.MethodGet)
	return
}
