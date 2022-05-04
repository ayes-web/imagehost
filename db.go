package main

import (
	"context"
	"errors"
	"github.com/georgysavva/scany/pgxscan"
	"github.com/jackc/pgx/v4"
	"time"
)

const getUserByIDQuery = `
SELECT * FROM public.accounts WHERE id=$1;
`

func (app *Application) getUserByID(userID int) (account accountModel, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	err = pgxscan.Get(
		ctx,
		app.db,
		&account,
		getUserByIDQuery,
		userID,
	)

	return
}

const deleteImageUploadTokenQuery = `
DELETE FROM public.images WHERE file_name=$1 AND file_uploader=(SELECT id FROM accounts WHERE upload_token=$2);
`

func (app *Application) deleteImageUploadToken(fileName string, uploadToken string) (err error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, err = app.db.Exec(
		ctx,
		deleteImageUploadTokenQuery,
		&fileName,
		&uploadToken,
	)

	return
}

const deleteImageQuery = `
DELETE FROM public.images WHERE file_name=$1 AND file_uploader=$2;
`

func (app *Application) deleteImage(fileName string, userID int) (err error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, err = app.db.Exec(
		ctx,
		deleteImageQuery,
		&fileName,
		&userID,
	)

	return
}

const insertNewImageUploadTokenQuery = `
INSERT INTO public.images (file_name, file_uploader) 
VALUES ($1, (SELECT id FROM accounts WHERE upload_token=$2));
`

func (app *Application) insertNewImageUploadToken(fileName string, uploadToken string) (err error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, err = app.db.Query(
		ctx,
		insertNewImageUploadTokenQuery,
		&fileName,
		&uploadToken,
	)

	return
}

const insertNewImageQuery = `
INSERT INTO public.images (file_name, file_uploader) VALUES ($1, $2);
`

func (app *Application) insertNewImage(fileName string, userID int) (err error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, err = app.db.Query(
		ctx,
		insertNewImageQuery,
		&fileName,
		&userID,
	)

	return
}

const deleteImagesFromAccountQuery = `
DELETE FROM public.images WHERE file_uploader=$1;
`

func (app *Application) deleteImagesFromAccount(userID int) (err error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, err = app.db.Exec(
		ctx,
		deleteImagesFromAccountQuery,
		userID,
	)

	return
}

const deleteAccountQuery = `
DELETE FROM public.accounts WHERE id=$1;
`

func (app *Application) deleteAccount(userID int) (err error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, err = app.db.Exec(
		ctx,
		deleteAccountQuery,
		userID,
	)

	return
}

const getAllImagesFromAccountQuery = `
SELECT file_name FROM public.images WHERE file_uploader=$1;
`

func (app *Application) getAllImagesFromAccount(userID int) (images []imageModel, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	err = pgxscan.Select(
		ctx,
		app.db,
		images,
		getAllImagesFromAccountQuery,
		userID,
	)

	return
}

const fileExistsQuery = `
SELECT FROM public.images WHERE file_name=$1;
`

// Looks if file exists in database
func (app *Application) fileExists(fileName string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := app.db.QueryRow(ctx, fileExistsQuery, fileName).Scan(); err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return false, err
		}

		return false, nil
	}

	return true, nil
}

const findIDByTokenQuery = `
SELECT id FROM accounts WHERE token=$1;
`

// gets user id by token
func (app *Application) idByToken(token string) (id int, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	err = app.db.QueryRow(
		ctx,
		findIDByTokenQuery,
		token,
	).Scan(&id)

	return
}

const findIDByUploadTokenQuery = `
SELECT id FROM accounts WHERE upload_token=$1;
`

// gets user id by upload token
func (app *Application) idByUploadToken(uploadToken string) (id int, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	err = app.db.QueryRow(
		ctx,
		findIDByUploadTokenQuery,
		uploadToken,
	).Scan(&id)

	return
}

const replaceUploadTokenQuery = `
UPDATE accounts SET upload_token=uuid_generate_v4() WHERE token=$1 RETURNING upload_token;
`

func (app *Application) replaceUploadToken(token string) (uploadToken string, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	err = app.db.QueryRow(
		ctx,
		replaceUploadTokenQuery,
		token,
	).Scan(&uploadToken)

	return
}

const createNewUserQuery = `
INSERT INTO public.accounts DEFAULT values RETURNING *;
`

func (app *Application) createNewUser() (account accountModel, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	err = pgxscan.Get(
		ctx,
		app.db,
		&account,
		createNewUserQuery,
	)

	return
}

const createNewAdminQuery = `
INSERT INTO public.accounts (account_type) VALUES ('ADMIN'::account_type) RETURNING *;
`

func (app *Application) createNewAdmin() (account accountModel, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	err = pgxscan.Get(
		ctx,
		app.db,
		&account,
		createNewAdminQuery,
	)

	return
}

const findAdminByTokenQuery = `
SELECT FROM accounts WHERE token=$1 AND account_type='ADMIN'::account_type; 
`

func (app *Application) findAdminByToken(token string) (err error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	err = app.db.QueryRow(
		ctx,
		findAdminByTokenQuery,
		token,
	).Scan()

	return
}

const findAllExpiredImagesQuery = `
SELECT * FROM public.images WHERE created_date < NOW() - INTERVAL '7 days';
`

func (app *Application) findAllExpiredImages() (images []imageModel, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	err = pgxscan.Select(
		ctx,
		app.db,
		&images,
		findAllExpiredImagesQuery,
	)

	return
}

const deleteAllExpiredImagesQuery = `
DELETE FROM public.images WHERE created_date < NOW() - INTERVAL '7 days';
`

func (app *Application) deleteAllExpiredImages() (err error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, err = app.db.Exec(
		ctx,
		deleteAllExpiredImagesQuery,
	)

	return
}
