package main

import (
	"context"
	"log"
	"time"

	_ "github.com/lib/pq"

	"github.com/robfig/cron/v3"
)

func (app *Application) autoDeletion() {
	c := cron.New()

	if _, err := c.AddFunc("@hourly", func() {
		app.logInfo.Println("Starting hourly cron job")

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		rows, err := app.db.QueryContext(ctx, "SELECT file_name FROM public.images WHERE created_date < NOW() - INTERVAL '7 days'")
		if err != nil {
			app.logError.Println(err)
			return
		}

		for rows.Next() {
			var fileName string
			if err = rows.Scan(&fileName); err != nil {
				app.logError.Println(err)
				continue
			}

			if err = app.deleteFile(fileName); err != nil {
				app.logError.Println(err)
			}
		}

		ctx, cancel = context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if _, err = app.db.ExecContext(ctx, "DELETE FROM public.images WHERE created_date < NOW() - INTERVAL '7 days'"); err != nil {
			app.logError.Println(err)
			return
		}
	}); err != nil {
		log.Fatal(err)
	}

	app.logInfo.Println("Starting auto deletion server")
	c.Start()
}
