// Code generated by Wire. DO NOT EDIT.

//go:generate go run github.com/google/wire/cmd/wire
//go:build !wireinject
// +build !wireinject

package main

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"flag"
	"github.com/BurntSushi/toml"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
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

// Injectors from container.go:

func initializeApplication() *Application {
	logger := setupLogging()
	templates := setupTemplates()
	config := initializeConfig(logger)
	database := prepareDB(logger, config)
	s3 := prepareStorage(logger, config)
	limiter := setupRatelimiting()
	mainUninitializedApplication := &uninitializedApplication{
		Logger:      logger,
		Templates:   templates,
		config:      config,
		db:          database,
		s3client:    s3,
		RateLimiter: limiter,
	}
	application := addRouter(mainUninitializedApplication)
	return application
}

// container.go:

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

func setupLogging() *Logger {
	flags := log.Ldate | log.Ltime | log.Lshortfile | log.Lmsgprefix

	return &Logger{
		logInfo:    log.New(os.Stdout, "INFO: ", flags),
		logError:   log.New(os.Stdout, "ERROR: ", flags),
		logWarning: log.New(os.Stdout, "WARN: ", flags),
	}
}

func prepareStorage(l *Logger, c Config) (s3client *s3.S3) {
	switch c.fileStorageMethod {
	case fileStorageS3:
		l.logInfo.Println("Storing files in s3 bucket")

		if s3session, err := session.NewSession(&aws.Config{
			Credentials:      credentials.NewStaticCredentials(c.S3.AccessKeyID, c.S3.SecretAccessKey, ""),
			Endpoint:         aws.String(c.S3.Endpoint),
			Region:           aws.String(c.S3.Region),
			S3ForcePathStyle: aws.Bool(true),
		}); err != nil {
			l.logInfo.Fatal(err)
		} else {
			s3client = s3.New(s3session)
		}
	case fileStorageLocal:
		l.logInfo.Println("Storing files in", c.DataFolder)

		if file, _ := os.Stat(c.DataFolder); file == nil {
			l.logInfo.Println("Creating data folder")

			if err := os.Mkdir(c.DataFolder, 0777); err != nil {
				l.logError.Fatal(err)
			}
		}
	default:
		l.logError.Fatal(ErrUnknownStorageMethod)
	}

	return
}

func initializeConfig(l *Logger) (c Config) {
	var configLocation string
	flag.StringVar(&configLocation, "c", "config.toml", "Location of config file")
	flag.Parse()

	rawConfig, err := os.ReadFile(configLocation)
	if err != nil {
		l.logError.Fatal(err)
	}

	if _, err = toml.Decode(string(rawConfig), &c); err != nil {
		l.logError.Fatal(err)
	}

	if c.S3 != (s3Config{}) {
		c.fileStorageMethod = fileStorageS3
	} else {
		c.fileStorageMethod = fileStorageLocal
	}

	return
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

func setupRatelimiting() *limiter.Limiter {
	rateLimiter := tollbooth.NewLimiter(2, &limiter.ExpirableOptions{DefaultExpirationTTL: time.Hour})
	rateLimiter.SetIPLookups([]string{"X-Forwarded-For", "RemoteAddr", "X-Real-IP"})

	return rateLimiter
}

func prepareDB(l *Logger, c Config) (db Database) {
	l.logInfo.Println("Setting up database")

	var err error
	db.db, err = pgxpool.Connect(context.Background(), c.PostgresConnectionString)
	if err != nil {
		l.logError.Fatal(err)
	}

	initQueries := &pgx.Batch{}
	initQueries.Queue(accountRolesEnumCreation)
	initQueries.Queue(postgresExtensionQuery)
	initQueries.Queue(imagesTableCreation)
	initQueries.Queue(accountsTableCreation)

	batchResult := db.db.SendBatch(context.Background(), initQueries)
	if _, err = batchResult.Exec(); err != nil {
		l.logError.Fatal(err)
	}

	if err = batchResult.Close(); err != nil {
		l.logError.Fatal(err)
	}

	if _, err = db.getUserByID(1); errors.Is(err, pgx.ErrNoRows) {
		var user accountModel
		user, err = db.createNewAdmin()
		if err != nil {
			l.logError.Fatal(err)
		}

		var jsonData []byte
		if jsonData, err = json.MarshalIndent(user, "", "\t"); err != nil {
			l.logError.Fatal(err)
		} else {
			l.logInfo.Println("Created first account: ", string(jsonData))
		}
	}

	return
}

func addRouter(uninitializedApp *uninitializedApplication) (app *Application) {
	app = (*Application)(uninitializedApp)
	app.logInfo.Println("Setting up router")
	gin.SetMode(gin.ReleaseMode)
	app.Router = gin.Default()

	app.Router.Use(
		app.bodySizeMiddleware(),
		app.databaseConnectionCheck(),
	)

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

type uninitializedApplication Application
