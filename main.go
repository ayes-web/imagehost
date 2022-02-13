package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path"
	"strings"
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

type User struct {
	Upload_token string
	Token        string
	Id           int
	Account_type string
}

type s3_config struct {
	Access_key_id     string `toml:"access_key_id"`
	Secret_access_key string `toml:"secret_access_key"`
	Bucket            string `toml:"bucket"`
	Region            string `toml:"region"`
	Endpoint          string `toml:"endpoint"`
	CDN_Domain        string `toml:"cdn_domain"`
}
type Config struct {
	Template_folder            string `toml:"template_folder"`
	Static_folder              string `toml:"static_folder"`
	Data_folder                string `toml:"data_folder"`
	File_name_length           int    `toml:"file_name_length"`
	Max_upload_size            int    `toml:"max_upload_size"`
	Postgres_connection_string string `toml:"postgres_connection_string"`
	Web_port                   string `toml:"web_port"`

	S3       s3_config `toml:"s3"`
	s3client *s3.S3
}

func main() {
	logger := log.Default()

	var config_location string
	flag.StringVar(&config_location, "c", "config.toml", "Location of config file")
	flag.Parse()

	raw_config, err := os.ReadFile(config_location)
	if err != nil {
		logger.Fatal(err)
	}

	var config Config
	if _, err := toml.Decode(string(raw_config), &config); err != nil {
		logger.Fatal(err)
	}

	if config.S3 == (s3_config{}) {
		logger.Println("Storing files in", config.Data_folder)

		if file, _ := os.Stat(config.Data_folder); file == nil {
			logger.Println("Creating data folder")

			if err := os.Mkdir(config.Data_folder, 0777); err != nil {
				logger.Fatal(err)
			}
		}
	} else {
		logger.Println("Storing files in s3 bucket")
		config.s3client = prepeare_s3(config)
	}

	var db *sql.DB = prepeare_db(config, logger)

	go auto_deletion(db, config, logger)

	apiListTemplate, err := template.New("api_list.html").ParseFiles(config.Template_folder + "api_list.html")
	if err != nil {
		logger.Fatal(err)
	}

	rateLimiter := tollbooth.NewLimiter(2, &limiter.ExpirableOptions{DefaultExpirationTTL: time.Hour})
	rateLimiter.SetIPLookups([]string{"X-Forwarded-For", "RemoteAddr", "X-Real-IP"})

	http.Handle("/api_list", tollbooth.LimitFuncHandler(rateLimiter, func(w http.ResponseWriter, r *http.Request) {
		if apiListTemplate.Execute(w, r.Host) != nil {
			fmt.Fprintf(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
	}))

	http.Handle("/api/", tollbooth.LimitFuncHandler(rateLimiter, func(w http.ResponseWriter, r *http.Request) {
		logger.Println(r.URL.Path)

		if r.Method == "POST" {
			r.Body = http.MaxBytesReader(w, r.Body, int64(config.Max_upload_size))

			if r.ParseMultipartForm(int64(config.Max_upload_size)) != nil {
				fmt.Fprintf(w, http.StatusText(http.StatusRequestEntityTooLarge), http.StatusRequestEntityTooLarge)
				return
			}

			if err := db.Ping(); err != nil { // If database is down
				fmt.Fprintf(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}

			switch r.URL.Path {
			case "/api/upload":
				upload_image_api(w, r, db, config, logger)
			case "/api/delete":
				delete_image_api(w, r, db, config)
			case "/api/account/new_upload_token":
				new_upload_token_api(w, r, db)
			case "/api/account/delete":
				account_delete_api(w, r, db, config)
			case "/api/admin/create_user":
				admin_create_user(w, r, db)
			case "/api/admin/delete_user":
				admin_delete_user(w, r, db, config)
			default:
				http.NotFound(w, r)
			}
		} else {
			http.NotFound(w, r)
		}
	}))

	indexTemplate, err := template.New("index.html").ParseFiles(config.Template_folder + "index.html")
	if err != nil {
		logger.Fatal(err)
	}

	http.Handle("/", tollbooth.LimitFuncHandler(rateLimiter, func(w http.ResponseWriter, r *http.Request) {
		logger.Println(r.URL.Path, r.Header.Get("X-Forwarded-For"))

		if strings.HasPrefix(r.URL.Path, "/public/") {
			file_path := config.Static_folder + path.Base(path.Clean(r.URL.Path))
			if _, err := os.Stat(file_path); err != nil {
				fmt.Fprintf(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
				return
			}

			http.ServeFile(w, r, file_path)
			return
		} else if r.URL.Path == "/" {
			if indexTemplate.Execute(w, r.Host) != nil {
				fmt.Fprintf(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			}

			return
		}

		// Looks in database for uploaded file
		if db.QueryRow("SELECT FROM public.images WHERE file_name=$1", path.Base(path.Clean(r.URL.Path))).Scan() != nil {
			http.Redirect(w, r, "https://www.youtube.com/watch?v=dQw4w9WgXcQ", http.StatusFound)
			return
		}

		if config.s3client == nil {
			http.ServeFile(w, r, config.Data_folder+path.Clean(r.URL.Path))
		} else {
			http.Redirect(w, r, "https://"+config.S3.CDN_Domain+"/file/"+config.S3.Bucket+path.Clean(r.URL.Path), http.StatusFound)
		}
	}))

	logger.Printf("Starting server at http://localhost:%s\n", config.Web_port)
	logger.Fatal(http.ListenAndServe(":"+config.Web_port, nil))
}

// Deletes a file
func delete_file(config Config, file_name string) {
	if config.s3client == nil { // Deletes from local storage
		os.Remove(config.Data_folder + file_name)
	} else { // Delete from s3
		config.s3client.DeleteObject(&s3.DeleteObjectInput{
			Bucket: aws.String(config.S3.Bucket),
			Key:    aws.String(file_name),
		})
	}
}

// Gets s3 session from config
func prepeare_s3(config Config) *s3.S3 {
	return s3.New(session.New(&aws.Config{
		Credentials:      credentials.NewStaticCredentials(config.S3.Access_key_id, config.S3.Secret_access_key, ""),
		Endpoint:         aws.String(config.S3.Endpoint),
		Region:           aws.String(config.S3.Region),
		S3ForcePathStyle: aws.Bool(true),
	}))
}

// Makes sure db is correctly setup and connects to it
func prepeare_db(config Config, logger *log.Logger) *sql.DB {
	db, err := sql.Open("postgres", config.Postgres_connection_string)
	if err != nil {
		logger.Fatal(err)
	}

	if _, err := db.Exec(`CREATE EXTENSION IF NOT EXISTS "uuid-ossp";`); err != nil {
		logger.Fatal(err)
	}

	if _, err := db.Exec("CREATE TABLE IF NOT EXISTS public.images (file_name varchar NOT NULL, created_date timestamptz NOT NULL DEFAULT now(), file_owner int4 NOT NULL, CONSTRAINT images_un UNIQUE (file_name));"); err != nil {
		logger.Fatal(err)
	}
	if _, err := db.Exec("CREATE TABLE IF NOT EXISTS public.accounts (token uuid NOT NULL DEFAULT uuid_generate_v4(), upload_token uuid NOT NULL DEFAULT uuid_generate_v4(), id serial4 NOT NULL, account_type text NOT NULL DEFAULT 'USER', CONSTRAINT accounts_pk PRIMARY KEY (id), CONSTRAINT accounts_un UNIQUE (upload_token));"); err != nil {
		logger.Fatal(err)
	}

	var amount int
	if err := db.QueryRow("SELECT count(id) from public.accounts;").Scan(&amount); err != nil {
		logger.Fatal(err)
	}

	if amount == 0 {
		var user User
		if err := db.QueryRow("INSERT INTO public.accounts (id, account_type) values (1,'ADMIN') RETURNING *;").Scan(&user.Token, &user.Upload_token, &user.Id, &user.Account_type); err != nil {
			logger.Fatal(err)
		}

		json, err := json.MarshalIndent(user, "", "\t")
		if err != nil {
			logger.Fatal(err)
		}
		fmt.Println("Created first account: ", string(json))
	}

	return db
}

func generate_file_name(file_name_length int) string {
	return uniuri.NewLenChars(file_name_length, []byte("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"))
}
