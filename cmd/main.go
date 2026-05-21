package main

import (
	"skalculator/api"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func main() {
	gin.SetMode(gin.DebugMode)

	api.InitDB()
	r := gin.Default()

	corsConfig := cors.Config{
		AllowAllOrigins:  true,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization", "X-Requested-With"},
		ExposeHeaders:    []string{"Content-Length", "Authorization"},
		AllowCredentials: true,
	}
	r.Use(cors.New(corsConfig))

	// Public routes (no authentication required)
	public := r.Group("/")
	{
		// Auth endpoints
		public.POST("/token/", api.GenerateJWT)
		public.POST("/token", api.GenerateJWT)
		public.POST("/register/", api.RegisterUser)
		public.POST("/register", api.RegisterUser)
		public.POST("/login/", api.GenerateJWT)
		public.POST("/login", api.GenerateJWT)

	}

	// Protected routes (require JWT authentication)
	protected := r.Group("/")
	protected.Use(api.JWTAuthMiddleware())
	{

		// GET endpoints are public
		protected.GET("/products/", api.GetProducts)
		protected.GET("/products", api.GetProducts)
		protected.GET("/products/:id/", api.GetProduct)
		protected.GET("/products/:id", api.GetProduct)

		protected.GET("/categories/", api.GetCategories)
		protected.GET("/categories", api.GetCategories)
		protected.GET("/categories/:id/", api.GetCategory)
		protected.GET("/categories/:id", api.GetCategory)

		protected.GET("/store-chains/", api.GetStoreChains)
		protected.GET("/store-chains", api.GetStoreChains)
		protected.GET("/store-chains/:id/", api.GetStoreChain)
		protected.GET("/store-chains/:id", api.GetStoreChain)

		// Product management
		protected.POST("/products/", api.CreateProduct)
		protected.POST("/products", api.CreateProduct)
		protected.PUT("/products/:id/", api.UpdateProduct)
		protected.PUT("/products/:id", api.UpdateProduct)
		protected.DELETE("/products/:id/", api.DeleteProduct)
		protected.DELETE("/products/:id", api.DeleteProduct)

		// User management
		protected.POST("/users/", api.CreateUser)
		protected.POST("/users", api.CreateUser)
		protected.GET("/users/", api.GetUsers)
		protected.GET("/users", api.GetUsers)
		protected.GET("/users/:id/", api.GetUser)
		protected.GET("/users/:id", api.GetUser)
		protected.PUT("/users/:id/", api.UpdateUser)
		protected.PUT("/users/:id", api.UpdateUser)
		protected.DELETE("/users/:id/", api.DeleteUser)
		protected.DELETE("/users/:id", api.DeleteUser)

		// Order management
		protected.POST("/orders/", api.CreateOrder)
		protected.POST("/orders", api.CreateOrder)
		protected.GET("/orders/", api.GetOrders)
		protected.GET("/orders", api.GetOrders)
		protected.GET("/orders/:id/", api.GetOrder)
		protected.GET("/orders/:id", api.GetOrder)
		protected.PUT("/orders/:id/", api.UpdateOrder)
		protected.PUT("/orders/:id", api.UpdateOrder)
		protected.DELETE("/orders/:id/", api.DeleteOrder)
		protected.DELETE("/orders/:id", api.DeleteOrder)

		// Statistics for admin and manager
		protected.GET("/stats/", api.RoleAuthMiddleware(api.RoleAdmin, api.RoleManager), api.GetStats)
		protected.GET("/stats", api.RoleAuthMiddleware(api.RoleAdmin, api.RoleManager), api.GetStats)

		// Order items
		protected.POST("/order-items/", api.CreateOrderItem)
		protected.POST("/order-items", api.CreateOrderItem)
		protected.GET("/order-items/", api.GetOrderItems)
		protected.GET("/order-items", api.GetOrderItems)
		protected.GET("/order-items/:id/", api.GetOrderItem)
		protected.GET("/order-items/:id", api.GetOrderItem)
		protected.PUT("/order-items/:id/", api.UpdateOrderItem)
		protected.PUT("/order-items/:id", api.UpdateOrderItem)
		protected.DELETE("/order-items/:id/", api.DeleteOrderItem)
		protected.DELETE("/order-items/:id", api.DeleteOrderItem)
	}

	// Admin-only routes (require JWT + Admin role)
	admin := r.Group("/")
	admin.Use(api.JWTAuthMiddleware(), api.RoleAuthMiddleware(api.RoleAdmin))
	{
		// Store management (admin only)
		admin.POST("/stores/", api.CreateStore)
		admin.POST("/stores", api.CreateStore)
		admin.PUT("/stores/:id/", api.UpdateStore)
		admin.PUT("/stores/:id", api.UpdateStore)
		admin.DELETE("/stores/:id/", api.DeleteStore)
		admin.DELETE("/stores/:id", api.DeleteStore)

		// Store chain management (admin only)
		admin.POST("/store-chains/", api.CreateStoreChain)
		admin.POST("/store-chains", api.CreateStoreChain)
		admin.PUT("/store-chains/:id/", api.UpdateStoreChain)
		admin.PUT("/store-chains/:id", api.UpdateStoreChain)
		admin.DELETE("/store-chains/:id/", api.DeleteStoreChain)
		admin.DELETE("/store-chains/:id", api.DeleteStoreChain)

		// Category management (admin only)
		admin.POST("/categories/", api.CreateCategory)
		admin.POST("/categories", api.CreateCategory)
		admin.PUT("/categories/:id/", api.UpdateCategory)
		admin.PUT("/categories/:id", api.UpdateCategory)
		admin.DELETE("/categories/:id/", api.DeleteCategory)
		admin.DELETE("/categories/:id", api.DeleteCategory)

		admin.GET("/stores/", api.GetStores)
		admin.GET("/stores", api.GetStores)
		admin.GET("/stores/:id/", api.GetStore)
		admin.GET("/stores/:id", api.GetStore)
	}

	r.Run(":8080")
}
