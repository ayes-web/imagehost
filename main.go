package main

import (
	"flag"
	"html/template"
	"log"
	"net/http"
	"time"

	"database/sql"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/didip/tollbooth/v6"
	"github.com/didip/tollbooth/v6/limiter"
	_ "github.com/lib/pq"
)

type Application struct {
	logError   *log.Logger
	logWarning *log.Logger
	logInfo    *log.Logger

	apiListTemplate *template.Template
	indexTemplate   *template.Template

	config      Config
	db          *sql.DB
	s3client    *s3.S3
	rateLimiter *limiter.Limiter
}

type Config struct {
	TemplateFolder           string `toml:"template_folder"`
	StaticFolder             string `toml:"static_folder"`
	DataFolder               string `toml:"data_folder"`
	FileNameLength           int    `toml:"file_name_length"`
	MaxUploadSize            int64  `toml:"max_upload_size"`
	PostgresConnectionString string `toml:"postgres_connection_string"`
	WebPort                  string `toml:"web_port"`

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

type User struct {
	UploadToken string
	Token       string
	ID          int
	AccountType string
}

func main() {
	app := Application{}

	app.setupLogging()

	var configLocation string
	flag.StringVar(&configLocation, "c", "config.toml", "Location of config file")
	flag.Parse()

	app.initializeConfig(configLocation)
	app.setupTemplates()
	app.prepareDb()

	go app.autoDeletion()

	app.rateLimiter = tollbooth.NewLimiter(2, &limiter.ExpirableOptions{DefaultExpirationTTL: time.Hour})
	app.rateLimiter.SetIPLookups([]string{"X-Forwarded-For", "RemoteAddr", "X-Real-IP"})

	router := app.initializeRouter()

	app.logInfo.Printf("Starting server at http://localhost:%s\n", app.config.WebPort)
	app.logError.Fatal(http.ListenAndServe(":"+app.config.WebPort, router))
}
