package main

import (
	"log"

	_ "github.com/lib/pq"

	"github.com/robfig/cron/v3"
)

func (app *Application) autoDeletion() {
	c := cron.New()

	if _, err := c.AddFunc("@hourly", func() {
		app.logger.Println("Starting hourly cron job")

		rows, err := app.db.Query("SELECT file_name FROM public.images WHERE created_date < NOW() - INTERVAL '7 days'")
		if err != nil {
			app.logger.Println(err)
			return
		}

		for rows.Next() {
			var fileName string
			if err = rows.Scan(&fileName); err != nil {
				app.logger.Println(err)
				continue
			}

			if err = app.deleteFile(fileName); err != nil {
				app.logger.Println(err)
			}
		}

		if _, err = app.db.Exec("DELETE FROM public.images WHERE created_date < NOW() - INTERVAL '7 days'"); err != nil {
			app.logger.Println(err)
			return
		}
	}); err != nil {
		log.Fatal(err)
	}

	app.logger.Println("Starting auto deletion server")
	c.Start()
}
