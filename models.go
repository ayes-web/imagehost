package main

import (
	"github.com/google/uuid"
	"time"
)

const postgresExtensionQuery = `
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
`

const accountRolesEnumCreation = `
		DO $$
		BEGIN
    		IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'account_type') THEN
        		CREATE TYPE account_type AS ENUM ('USER', 'ADMIN');
    		END IF;
		END
		$$;
`

type AccountRole string

const imagesTableCreation = `
		CREATE TABLE IF NOT EXISTS public.images (
			file_name varchar NOT NULL, 
			created_date timestamptz NOT NULL DEFAULT now(), 
			file_uploader integer NOT NULL,
			CONSTRAINT images_un UNIQUE (file_name)
		);
`

type imageModel struct {
	FileName     string    `db:"file_name"`
	CreatedDate  time.Time `db:"created_date"`
	FileUploader int       `db:"file_uploader"`
}

const accountsTableCreation = `
		CREATE TABLE IF NOT EXISTS public.accounts (
			token uuid NOT NULL DEFAULT uuid_generate_v4(), 
			upload_token uuid NOT NULL DEFAULT uuid_generate_v4(), 
			id serial4 NOT NULL, 
			account_type account_type NOT NULL DEFAULT 'USER'::account_type, 
			CONSTRAINT accounts_pk PRIMARY KEY (id), 
			CONSTRAINT accounts_un UNIQUE (upload_token)
		);
`

type accountModel struct {
	Token       uuid.UUID   `db:"token"`
	UploadToken uuid.UUID   `db:"upload_token"`
	ID          int         `db:"id"`
	AccountType AccountRole `db:"account_type"`
}
