package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/h2non/filetype"
	"html/template"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/dchest/uniuri"
	"github.com/didip/tollbooth/v6"
	"github.com/didip/tollbooth/v6/limiter"
	_ "github.com/lib/pq"
)

type Application struct {
	logger          *log.Logger
	apiListTemplate *template.Template
	indexTemplate   *template.Template
	config          Config
	db              *sql.DB
	s3client        *s3.S3
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
	AccessKeyId     string `toml:"access_key_id"`
	SecretAccessKey string `toml:"secret_access_key"`
	Bucket          string `toml:"bucket"`
	Region          string `toml:"region"`
	Endpoint        string `toml:"endpoint"`
	CdnDomain       string `toml:"cdn_domain"`
}

type User struct {
	UploadToken string
	Token       string
	Id          int
	AccountType string
}

func main() {
	app := Application{
		logger: log.Default(),
	}

	var configLocation string
	flag.StringVar(&configLocation, "c", "config.toml", "Location of config file")
	flag.Parse()

	app.initializeConfig(configLocation)
	app.setupTemplates()
	app.prepareDb()

	go app.autoDeletion()

	rateLimiter := tollbooth.NewLimiter(2, &limiter.ExpirableOptions{DefaultExpirationTTL: time.Hour})
	rateLimiter.SetIPLookups([]string{"X-Forwarded-For", "RemoteAddr", "X-Real-IP"})

	http.Handle("/api/upload", tollbooth.LimitFuncHandler(rateLimiter, app.uploadImageApi))
	http.Handle("/api/delete", tollbooth.LimitFuncHandler(rateLimiter, app.deleteImageApi))
	http.Handle("/api/account/new_upload_token", tollbooth.LimitFuncHandler(rateLimiter, app.newUploadTokenApi))
	http.Handle("/api/account/delete", tollbooth.LimitFuncHandler(rateLimiter, app.accountDeleteApi))
	http.Handle("/api/admin/create_user", tollbooth.LimitFuncHandler(rateLimiter, app.adminCreateUser))
	http.Handle("/api/admin/delete_user", tollbooth.LimitFuncHandler(rateLimiter, app.adminDeleteUser))

	http.Handle("/api_list", tollbooth.LimitFuncHandler(rateLimiter, app.apiList))
	http.Handle("/public/", tollbooth.LimitFuncHandler(rateLimiter, app.publicFiles))
	http.Handle("/", tollbooth.LimitFuncHandler(rateLimiter, app.indexPage))

	app.logger.Printf("Starting server at http://localhost:%s\n", app.config.WebPort)
	app.logger.Fatal(http.ListenAndServe(":"+app.config.WebPort, nil))
}

func (app *Application) initializeConfig(configLocation string) {
	rawConfig, err := os.ReadFile(configLocation)
	if err != nil {
		app.logger.Fatal(err)
	}

	if _, err = toml.Decode(string(rawConfig), &app.config); err != nil {
		app.logger.Fatal(err)
	}

	if app.config.S3 == (s3Config{}) {
		app.logger.Println("Storing files in", app.config.DataFolder)

		if file, _ := os.Stat(app.config.DataFolder); file == nil {
			app.logger.Println("Creating data folder")

			if err = os.Mkdir(app.config.DataFolder, 0777); err != nil {
				app.logger.Fatal(err)
			}
		}
	} else {
		app.logger.Println("Storing files in s3 bucket")
		app.prepareS3()
	}
}

func (app *Application) setupTemplates() {
	var err error
	app.apiListTemplate, err = template.New("api_list.html").ParseFiles(app.config.TemplateFolder + "api_list.html")
	if err != nil {
		app.logger.Fatal(err)
	}

	app.indexTemplate, err = template.New("index.html").ParseFiles(app.config.TemplateFolder + "index.html")
	if err != nil {
		app.logger.Fatal(err)
	}
}

// Gets s3 session from config
func (app *Application) prepareS3() {
	if s3session, err := session.NewSession(&aws.Config{
		Credentials:      credentials.NewStaticCredentials(app.config.S3.AccessKeyId, app.config.S3.SecretAccessKey, ""),
		Endpoint:         aws.String(app.config.S3.Endpoint),
		Region:           aws.String(app.config.S3.Region),
		S3ForcePathStyle: aws.Bool(true),
	}); err != nil {
		app.logger.Fatal(err)
	} else {
		app.s3client = s3.New(s3session)
	}
}

// Makes sure db is correctly setup and connects to it
func (app *Application) prepareDb() {
	var err error
	app.db, err = sql.Open("postgres", app.config.PostgresConnectionString)
	if err != nil {
		app.logger.Fatal(err)
	}

	if _, err = app.db.Exec(`CREATE EXTENSION IF NOT EXISTS "uuid-ossp";`); err != nil {
		app.logger.Fatal(err)
	}
	if _, err = app.db.Exec("CREATE TABLE IF NOT EXISTS public.images (file_name varchar NOT NULL, created_date timestamptz NOT NULL DEFAULT now(), file_owner int4 NOT NULL, CONSTRAINT images_un UNIQUE (file_name));"); err != nil {
		app.logger.Fatal(err)
	}
	if _, err = app.db.Exec("CREATE TABLE IF NOT EXISTS public.accounts (token uuid NOT NULL DEFAULT uuid_generate_v4(), upload_token uuid NOT NULL DEFAULT uuid_generate_v4(), id serial4 NOT NULL, account_type text NOT NULL DEFAULT 'USER', CONSTRAINT accounts_pk PRIMARY KEY (id), CONSTRAINT accounts_un UNIQUE (upload_token));"); err != nil {
		app.logger.Fatal(err)
	}

	var amount int
	if err = app.db.QueryRow("SELECT count(id) from public.accounts;").Scan(&amount); err != nil {
		app.logger.Fatal(err)
	}

	if amount == 0 {
		var user User
		if err = app.db.QueryRow("INSERT INTO public.accounts (id, account_type) values (1,'ADMIN') RETURNING *;").Scan(&user.Token, &user.UploadToken, &user.Id, &user.AccountType); err != nil {
			app.logger.Fatal(err)
		}

		if data, err := json.MarshalIndent(user, "", "\t"); err != nil {
			app.logger.Fatal(err)
		} else {
			fmt.Println("Created first account: ", string(data))
		}
	}
}

// Deletes a file
func (app *Application) deleteFile(fileName string) (err error) {
	if app.s3client == nil { // Deletes from local storage
		err = os.Remove(app.config.DataFolder + fileName)
	} else { // Delete from s3
		_, err = app.s3client.DeleteObject(&s3.DeleteObjectInput{
			Bucket: aws.String(app.config.S3.Bucket),
			Key:    aws.String(fileName),
		})
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
	} else {
		return randomString(app.config.FileNameLength) + "." + extension.Extension, nil
	}
}
