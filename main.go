package main

import (
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/didip/tollbooth/v6/limiter"
	"github.com/gin-gonic/gin"
	"html/template"
	"log"
	"net/http"
)

type Application struct {
	*Logger
	*Templates
	config      Config
	db          Database
	s3client    *s3.S3
	RateLimiter *limiter.Limiter

	Router *gin.Engine
}

type fileStorageMethod string

const (
	fileStorageLocal fileStorageMethod = "LOCAL"
	fileStorageS3    fileStorageMethod = "S3"
)

type Logger struct {
	logError   *log.Logger
	logWarning *log.Logger
	logInfo    *log.Logger
}

type Templates struct {
	apiListTemplate *template.Template
	indexTemplate   *template.Template
}

type Config struct {
	DataFolder               string `toml:"data_folder"`
	FileNameLength           int    `toml:"file_name_length"`
	MaxUploadSize            int64  `toml:"max_upload_size"`
	PostgresConnectionString string `toml:"postgres_connection_string"`
	WebPort                  string `toml:"web_port"`

	fileStorageMethod fileStorageMethod

	S3 s3Config `toml:"s3"`
}

type s3Config struct {
	AccessKeyID     string `toml:"access_key_id"`
	SecretAccessKey string `toml:"secret_access_key"`
	Bucket          string `toml:"bucket"`
	Region          string `toml:"region"`
	Endpoint        string `toml:"endpoint"`
	CdnDomain       string `toml:"cdn_domain"`
}

func (app *Application) run() {
	app.logInfo.Printf("Starting server at http://localhost:%s\n", app.config.WebPort)
	app.logError.Fatal(http.ListenAndServe(":"+app.config.WebPort, app.Router))
}

func main() {
	app := initializeApplication()

	go app.autoDeletion()

	app.run()
}
