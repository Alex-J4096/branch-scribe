package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type Envelope struct {
	Data  any       `json:"data"`
	Error *APIError `json:"error"`
}

type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func RespondOK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, Envelope{
		Data:  data,
		Error: nil,
	})
}

func RespondError(c *gin.Context, status int, code string, message string) {
	c.JSON(status, Envelope{
		Data: nil,
		Error: &APIError{
			Code:    code,
			Message: message,
		},
	})
}
