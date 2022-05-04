package main

import (
	"github.com/robfig/cron/v3"
)

func (app *Application) autoDeletion() {
	c := cron.New()

	if _, err := c.AddFunc("@hourly", func() {
		app.logInfo.Println("Starting hourly cron job")

		images, err := app.findAllExpiredImages()
		if err != nil {
			app.logError.Println(err)
			return
		}

		for _, image := range images {
			if err = app.deleteFile(image.FileName); err != nil {
				app.logError.Println(err)
			}
		}

		if err = app.deleteAllExpiredImages(); err != nil {
			app.logError.Println(err)
			return
		}
	}); err != nil {
		app.logError.Fatal(err)
	}

	app.logInfo.Println("Starting auto deletion server")
	c.Start()
}
