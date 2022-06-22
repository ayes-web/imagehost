package main

import (
	"context"
	"errors"
	"github.com/georgysavva/scany/pgxscan"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"time"
)

type Database struct {
	db *pgxpool.Pool
}

const getUserByIDQuery = `
SELECT * FROM public.accounts WHERE id=$1;
`

func (db *Database) getUserByID(userID int) (account accountModel, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	err = pgxscan.Get(
		ctx,
		db.db,
		&account,
		getUserByIDQuery,
		userID,
	)

	return
}

const deleteImageUploadTokenQuery = `
DELETE FROM public.images WHERE file_name=$1 AND file_uploader=(SELECT id FROM accounts WHERE upload_token=$2);
`

func (db *Database) deleteImageUploadToken(fileName string, uploadToken string) (err error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, err = db.db.Exec(
		ctx,
		deleteImageUploadTokenQuery,
		&fileName,
		&uploadToken,
	)

	return
}

//const deleteImageQuery = `
//DELETE FROM public.images WHERE file_name=$1 AND file_uploader=$2;
//`
//
//func (db *Database) deleteImage(fileName string, userID int) (err error) {
//	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
//	defer cancel()
//
//	_, err = db.db.Exec(
//		ctx,
//		deleteImageQuery,
//		&fileName,
//		&userID,
//	)
//
//	return
//}

const insertNewImageUploadTokenQuery = `
INSERT INTO public.images (file_name, file_uploader) 
VALUES ($1, (SELECT id FROM accounts WHERE upload_token=$2));
`

func (db *Database) insertNewImageUploadToken(fileName string, uploadToken string) (err error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, err = db.db.Query(
		ctx,
		insertNewImageUploadTokenQuery,
		&fileName,
		&uploadToken,
	)

	return
}

//const insertNewImageQuery = `
//INSERT INTO public.images (file_name, file_uploader) VALUES ($1, $2);
//`
//
//func (db *Database) insertNewImage(fileName string, userID int) (err error) {
//	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
//	defer cancel()
//
//	_, err = db.db.Query(
//		ctx,
//		insertNewImageQuery,
//		&fileName,
//		&userID,
//	)
//
//	return
//}

const deleteImagesFromAccountQuery = `
DELETE FROM public.images WHERE file_uploader=$1;
`

func (db *Database) deleteImagesFromAccount(userID int) (err error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, err = db.db.Exec(
		ctx,
		deleteImagesFromAccountQuery,
		userID,
	)

	return
}

const deleteAccountQuery = `
DELETE FROM public.accounts WHERE id=$1;
`

func (db *Database) deleteAccount(userID int) (err error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, err = db.db.Exec(
		ctx,
		deleteAccountQuery,
		userID,
	)

	return
}

const getAllImagesFromAccountQuery = `
SELECT file_name FROM public.images WHERE file_uploader=$1;
`

func (db *Database) getAllImagesFromAccount(userID int) (images []imageModel, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	err = pgxscan.Select(
		ctx,
		db.db,
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
func (db *Database) fileExists(fileName string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := db.db.QueryRow(ctx, fileExistsQuery, fileName).Scan(); err != nil {
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
func (db *Database) idByToken(token string) (id int, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	err = db.db.QueryRow(
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
func (db *Database) idByUploadToken(uploadToken string) (id int, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	err = db.db.QueryRow(
		ctx,
		findIDByUploadTokenQuery,
		uploadToken,
	).Scan(&id)

	return
}

const replaceUploadTokenQuery = `
UPDATE accounts SET upload_token=uuid_generate_v4() WHERE token=$1 RETURNING upload_token;
`

func (db *Database) replaceUploadToken(token string) (uploadToken string, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	err = db.db.QueryRow(
		ctx,
		replaceUploadTokenQuery,
		token,
	).Scan(&uploadToken)

	return
}

const createNewUserQuery = `
INSERT INTO public.accounts DEFAULT values RETURNING *;
`

func (db *Database) createNewUser() (account accountModel, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	err = pgxscan.Get(
		ctx,
		db.db,
		&account,
		createNewUserQuery,
	)

	return
}

const createNewAdminQuery = `
INSERT INTO public.accounts (account_type) VALUES ('ADMIN'::account_type) RETURNING *;
`

func (db *Database) createNewAdmin() (account accountModel, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	err = pgxscan.Get(
		ctx,
		db.db,
		&account,
		createNewAdminQuery,
	)

	return
}

const findAdminByTokenQuery = `
SELECT FROM accounts WHERE token=$1 AND account_type='ADMIN'::account_type; 
`

func (db *Database) findAdminByToken(token string) (err error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	err = db.db.QueryRow(
		ctx,
		findAdminByTokenQuery,
		token,
	).Scan()

	return
}

const findAllExpiredImagesQuery = `
SELECT * FROM public.images WHERE created_date < NOW() - INTERVAL '7 days';
`

func (db *Database) findAllExpiredImages() (images []imageModel, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	err = pgxscan.Select(
		ctx,
		db.db,
		&images,
		findAllExpiredImagesQuery,
	)

	return
}

const deleteAllExpiredImagesQuery = `
DELETE FROM public.images WHERE created_date < NOW() - INTERVAL '7 days';
`

func (db *Database) deleteAllExpiredImages() (err error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, err = db.db.Exec(
		ctx,
		deleteAllExpiredImagesQuery,
	)

	return
}
