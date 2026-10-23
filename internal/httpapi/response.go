package httpapi

import "github.com/gin-gonic/gin"

type errorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

func fail(c *gin.Context, status int, code, message string) {
	c.JSON(status, errorResponse{Error: code, Message: message})
}
