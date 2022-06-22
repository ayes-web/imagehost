package main

import (
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/jackc/pgx/v4"
	"net/http"
)

// Checks if the user is an admin with token
func (app *Application) isAdmin(token string) (bool, error) {
	if err := app.db.findAdminByToken(token); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}

		return false, err
	}

	return true, nil
}

// Admin api for creating new user
func (app *Application) adminCreateUser(c *gin.Context) {
	user, err := app.db.createNewUser()
	if err != nil {
		app.logError.Println(err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	c.JSON(http.StatusOK, user)
}

// Admin api for deleting user
type adminDeleteUserInput struct {
	ID int `form:"id"`
}

func (app *Application) adminDeleteUser(c *gin.Context) {
	var input adminDeleteUserInput
	var err error

	if err = c.MustBindWith(&input, binding.FormPost); err != nil {
		c.String(http.StatusBadRequest, err.Error())
		c.Abort()
		return
	}

	if err = app.deleteAccountWithImages(input.ID); err != nil {
		app.logError.Println(err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	c.String(http.StatusOK, fmt.Sprintf("User %d deleted", input.ID))
}
