package main

import (
	"github.com/gin-gonic/gin"
)

func main() {
	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()
	router.GET("/", chatIndex)
	router.Run()
}

func chatIndex(c *gin.Context) {
	c.JSON(200, gin.H{
		"message": "pong",
	})
}
