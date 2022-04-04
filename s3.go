package main

import (
	"bytes"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

// Gets s3 session from config
func (app *Application) prepareS3() {
	if s3session, err := session.NewSession(&aws.Config{
		Credentials:      credentials.NewStaticCredentials(app.config.S3.AccessKeyID, app.config.S3.SecretAccessKey, ""),
		Endpoint:         aws.String(app.config.S3.Endpoint),
		Region:           aws.String(app.config.S3.Region),
		S3ForcePathStyle: aws.Bool(true),
	}); err != nil {
		app.logInfo.Fatal(err)
	} else {
		app.s3client = s3.New(s3session)
	}
}

func (app *Application) uploadFileS3(file []byte, fileName string) (err error) {
	_, err = app.s3client.PutObject(&s3.PutObjectInput{
		Body:   bytes.NewReader(file),
		Bucket: aws.String(app.config.S3.Bucket),
		Key:    aws.String(fileName),
	})

	return
}

func (app *Application) deleteFileS3(fileName string) (err error) {
	_, err = app.s3client.DeleteObject(&s3.DeleteObjectInput{
		Bucket: aws.String(app.config.S3.Bucket),
		Key:    aws.String(fileName),
	})

	return
}

func (app *Application) isUsingS3() bool {
	return app.s3client == nil
}
