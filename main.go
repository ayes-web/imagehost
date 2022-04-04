package main

import (
	"flag"
	"github.com/h2non/filetype"
	"html/template"
	"log"
	"net/http"
	"os"
	"time"

	"database/sql"
	"github.com/BurntSushi/toml"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/dchest/uniuri"
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

	config   Config
	db       *sql.DB
	s3client *s3.S3
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

func (app *Application) setupLogging() {
	flags := log.Ldate | log.Ltime | log.Lshortfile | log.Lmsgprefix

	app.logInfo = log.New(os.Stdout, "INFO: ", flags)
	app.logError = log.New(os.Stdout, "ERROR: ", flags)

	app.logInfo.Println("Setup logging")
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

	rateLimiter := tollbooth.NewLimiter(2, &limiter.ExpirableOptions{DefaultExpirationTTL: time.Hour})
	rateLimiter.SetIPLookups([]string{"X-Forwarded-For", "RemoteAddr", "X-Real-IP"})

	http.Handle("/api/upload", tollbooth.LimitFuncHandler(rateLimiter, app.uploadImageAPI))
	http.Handle("/api/delete", tollbooth.LimitFuncHandler(rateLimiter, app.deleteImageAPI))
	http.Handle("/api/account/new_upload_token", tollbooth.LimitFuncHandler(rateLimiter, app.newUploadTokenAPI))
	http.Handle("/api/account/delete", tollbooth.LimitFuncHandler(rateLimiter, app.accountDeleteAPI))
	http.Handle("/api/admin/create_user", tollbooth.LimitFuncHandler(rateLimiter, app.adminCreateUser))
	http.Handle("/api/admin/delete_user", tollbooth.LimitFuncHandler(rateLimiter, app.adminDeleteUser))

	http.Handle("/api_list", tollbooth.LimitFuncHandler(rateLimiter, app.apiList))
	http.Handle("/public/", tollbooth.LimitFuncHandler(rateLimiter, app.publicFiles))
	http.Handle("/", tollbooth.LimitFuncHandler(rateLimiter, app.indexPage))

	app.logInfo.Printf("Starting server at http://localhost:%s\n", app.config.WebPort)
	app.logError.Fatal(http.ListenAndServe(":"+app.config.WebPort, nil))
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

// Deletes a file
func (app *Application) deleteFile(fileName string) (err error) {
	if app.s3client == nil { // Deletes from local storage
		err = os.Remove(app.config.DataFolder + fileName)
	} else { // Delete from s3
		err = app.deleteFileS3(fileName)
	}

	return
}

func randomString(fileNameLength int) string {
	return uniuri.NewLenChars(fileNameLength, []byte("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"))
}

func (app *Application) generateFullFileName(file []byte) (string, error) {
	extension, err := filetype.Get(file)
	if err != nil {
		return "", err
	}

	if extension.Extension == "unknown" { // Unknown file type defaults to txt
		return randomString(app.config.FileNameLength) + "." + "txt", nil
	}

	return randomString(app.config.FileNameLength) + "." + extension.Extension, nil
}
