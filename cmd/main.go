package main

import (
	"skalculator/api"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func main() {
	api.InitDB()
	r := gin.Default()
	r.Use(cors.Default()) // All origins allowed by default

	//routes
	r.POST("/product", api.CreateProduct)
	r.GET("/product", api.GetProducts)
	r.GET("/product/:id", api.GetProduct)
	r.PUT("/product/:id", api.UpdateProduct)
	r.DELETE("/product/:id", api.DeleteProduct)

	r.GET("/category", api.GetCategories)

	// docs
	// r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	r.Run(":8080")
}
