package main

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"flag"
	"github.com/BurntSushi/toml"
	"github.com/didip/tollbooth/v6"
	"github.com/didip/tollbooth/v6/limiter"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"os"
	"time"
)

var ErrUnknownStorageMethod = errors.New("unknown file storage method")

//go:embed templates/*
var templateFiles embed.FS

//go:embed public/*
var publicFiles embed.FS

func PublicFiles() http.FileSystem {
	sub, err := fs.Sub(publicFiles, "public")
	if err != nil {
		panic(err)
	}

	return http.FS(sub)
}

func (app *Application) prepareDb() {
	app.logInfo.Println("Setting up database")

	var err error
	app.db, err = pgxpool.Connect(context.Background(), app.config.PostgresConnectionString)
	if err != nil {
		app.logError.Fatal(err)
	}

	initQueries := &pgx.Batch{}
	initQueries.Queue(accountRolesEnumCreation)
	initQueries.Queue(postgresExtensionQuery)
	initQueries.Queue(imagesTableCreation)
	initQueries.Queue(accountsTableCreation)

	batchResult := app.db.SendBatch(context.Background(), initQueries)
	if _, err = batchResult.Exec(); err != nil {
		app.logError.Fatal(err)
	}

	if err = batchResult.Close(); err != nil {
		app.logError.Fatal(err)
	}

	if _, err = app.getUserByID(1); errors.Is(err, pgx.ErrNoRows) { // if doesnt find account id 1 creates it
		var user accountModel
		user, err = app.createNewAdmin()
		if err != nil {
			app.logError.Fatal(err)
		}

		var jsonData []byte
		if jsonData, err = json.MarshalIndent(user, "", "\t"); err != nil {
			app.logError.Fatal(err)
		} else {
			app.logInfo.Println("Created first account: ", string(jsonData))
		}
	} else if err != nil {
		app.logError.Fatal(err)
	}
}

func (app *Application) initializeConfig() {
	var configLocation string
	flag.StringVar(&configLocation, "c", "config.toml", "Location of config file")
	flag.Parse()

	rawConfig, err := os.ReadFile(configLocation)
	if err != nil {
		app.logError.Fatal(err)
	}

	if _, err = toml.Decode(string(rawConfig), &app.config); err != nil {
		app.logError.Fatal(err)
	}

	if app.config.S3 != (s3Config{}) {
		app.fileStorageMethod = fileStorageS3
	} else {
		app.fileStorageMethod = fileStorageLocal
	}

	switch app.fileStorageMethod {
	case fileStorageS3:
		app.logInfo.Println("Storing files in s3 bucket")
		app.prepareS3()
	case fileStorageLocal:
		app.logInfo.Println("Storing files in", app.config.DataFolder)

		if file, _ := os.Stat(app.config.DataFolder); file == nil {
			app.logInfo.Println("Creating data folder")

			if err = os.Mkdir(app.config.DataFolder, 0777); err != nil {
				app.logError.Fatal(err)
			}
		}
	default:
		app.logError.Fatal(ErrUnknownStorageMethod)
	}
}

func (app *Application) initializeRouter() {
	app.logInfo.Println("Setting up router")
	gin.SetMode(gin.ReleaseMode)
	app.Router = gin.Default()

	app.Router.Use(app.bodySizeMiddleware())

	api := app.Router.Group("/api")
	api.Use(app.apiMiddleware())

	miscAPI := api.Group("/")
	miscAPI.Use(
		hasUploadTokenMiddleware(),
		app.uploadTokenVerificationMiddleware(),
	)

	miscAPI.POST("/upload", app.uploadImageAPI).Use()
	miscAPI.POST("/delete", app.deleteImageAPI)

	accountAPI := api.Group("/account")
	accountAPI.Use(
		hasTokenMiddleware(),
		app.userTokenVerificationMiddleware(),
	)

	accountAPI.POST("/new_upload_token", app.newUploadTokenAPI)
	accountAPI.POST("/delete", app.accountDeleteAPI)

	adminAPI := api.Group("/admin")
	adminAPI.Use(
		hasTokenMiddleware(),
		app.adminTokenVerificationMiddleware(),
	)

	adminAPI.POST("/create_user", app.adminCreateUser)
	adminAPI.POST("/delete_user", app.adminDeleteUser)

	app.Router.GET("/api_list", app.apiList)

	app.Router.StaticFS("/public/", PublicFiles())

	app.Router.GET("/", app.indexPage)

	app.Router.Use(app.ratelimitMiddleware())
	app.Router.NoRoute(app.indexFiles)

	return
}

func setupLogging() *Logger {
	flags := log.Ldate | log.Ltime | log.Lshortfile | log.Lmsgprefix

	return &Logger{
		logInfo:    log.New(os.Stdout, "INFO: ", flags),
		logError:   log.New(os.Stdout, "ERROR: ", flags),
		logWarning: log.New(os.Stdout, "WARN: ", flags),
	}
}
func setupTemplates() *Templates {
	return &Templates{
		apiListTemplate: template.Must(template.New("api_list.gohtml").ParseFS(
			templateFiles,
			"templates/api_list.gohtml",
		)),
		indexTemplate: template.Must(template.New("index.gohtml").ParseFS(
			templateFiles,
			"templates/index.gohtml",
		)),
	}
}
func setupRatelimiting() (rateLimiter *limiter.Limiter) {
	rateLimiter = tollbooth.NewLimiter(2, &limiter.ExpirableOptions{DefaultExpirationTTL: time.Hour})
	rateLimiter.SetIPLookups([]string{"X-Forwarded-For", "RemoteAddr", "X-Real-IP"})

	return
}
