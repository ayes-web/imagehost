package main

import (
	"bytes"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
)

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
