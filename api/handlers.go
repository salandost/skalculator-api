package api

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"skalculator/util"
)

var DB *gorm.DB

func InitDB() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}
	dsn := os.Getenv("DB_URL")
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	// Create enum if not exists
	err = util.CreateEnumIfNotExists(DB, "user_role", []string{"admin", "manager", "cashier"})
	if err != nil {
		log.Fatalf("failed to create enum: %v", err)
	}

	// migrate the schema
	if err := DB.AutoMigrate(&Product{}); err != nil {
		log.Fatal("Failed to migrate Product table:", err)
	}

	if err := DB.AutoMigrate(&User{}); err != nil {
		log.Fatal("Failed to migrate User table:", err)
	}
}

// CreateProduct godoc
// @Summary Create a new product
// @Description Create a new product with the provided details
// @Tags Products
// @Accept json
// @Produce json
// @Param product body Product true "Product data"
// @Success 201 {object} Product
// @Failure 400 {object} map[string]interface{}
// @Router /products [post]
func CreateProduct(c *gin.Context) {
	var product Product
	//bind the request body
	if err := c.ShouldBindJSON(&product); err != nil {
		ResponseJSON(c, http.StatusBadRequest, "Invalid input", nil)
		return
	}
	DB.Create(&product)
	ResponseJSON(c, http.StatusCreated, "Product created successfully", product)
}

// GetProducts godoc
// @Summary Get products
// @Description Retrieve a list of products with filtering, sorting, and pagination
// @Tags products
// @Accept json
// @Produce json
//
// @Param search query string false "Search in product name, description, or SKU"
// @Param sku query string false "Filter by exact SKU"
// @Param min_price query number false "Minimum price"
// @Param max_price query number false "Maximum price"
// @Param created_from query string false "Created from date (YYYY-MM-DD)"
// @Param created_to query string false "Created to date (YYYY-MM-DD)"
// @Param with_deleted query boolean false "Include soft-deleted products"
//
// @Param sort query string false "Sort field (id, name, price, created_at)" default(id)
// @Param order query string false "Sort order (asc, desc)" default(asc)
//
// @Param page query int false "Page number" default(1)
// @Param limit query int false "Items per page" default(10)
//
// @Success 200 {object} map[string]interface{} "Products retrieved successfully"
// @Failure 500 {object} map[string]interface{} "Failed to retrieve products"
//
// @Router /products [get]
func GetProducts(c *gin.Context) {
	var products []Product

	query := DB.Model(&Product{})

	// Filtering

	// name search
	if name := c.Query("name"); name != "" {
		query = query.Where("name ILIKE ?", "%"+name+"%")
	}

	// sku exact match
	if sku := c.Query("sku"); sku != "" {
		query = query.Where("sku = ?", sku)
	}

	// price range
	if minPrice := c.Query("min_price"); minPrice != "" {
		query = query.Where("price >= ?", minPrice)
	}

	if maxPrice := c.Query("max_price"); maxPrice != "" {
		query = query.Where("price <= ?", maxPrice)
	}

	// created date range
	if from := c.Query("created_from"); from != "" {
		query = query.Where("created_at >= ?", from)
	}

	if to := c.Query("created_to"); to != "" {
		query = query.Where("created_at <= ?", to)
	}

	// include soft-deleted (optional)
	if withDeleted := c.Query("with_deleted"); withDeleted == "true" {
		query = query.Unscoped()
	}

	// Sorting

	sort := c.DefaultQuery("sort", "id")
	order := c.DefaultQuery("order", "asc")

	allowedSort := map[string]bool{
		"id":         true,
		"name":       true,
		"price":      true,
		"created_at": true,
	}

	if !allowedSort[sort] {
		sort = "id"
	}

	if order != "asc" && order != "desc" {
		order = "asc"
	}

	query = query.Order(sort + " " + order)

	// Pagination

	limit := 10
	page := 1

	if l := c.Query("limit"); l != "" {
		fmt.Sscanf(l, "%d", &limit)
	}

	if p := c.Query("page"); p != "" {
		fmt.Sscanf(p, "%d", &page)
	}

	if limit > 100 {
		limit = 100
	}

	offset := (page - 1) * limit

	query = query.Limit(limit).Offset(offset)

	if err := query.Find(&products).Error; err != nil {
		ResponseJSON(c, http.StatusInternalServerError, "Failed to retrieve products", nil)
		return
	}

	ResponseJSON(c, http.StatusOK, "Products retrieved successfully", products)
}

// GetProduct godoc
// @Summary Get a product by ID
// @Description Retrieve a single product by its ID
// @Tags Products
// @Produce json
// @Param id path int true "Product ID"
// @Success 200 {object} Product
// @Failure 404 {object} map[string]interface{}
// @Router /products/{id} [get]
func GetProduct(c *gin.Context) {
	var product Product
	if err := DB.First(&product, c.Param("id")).Error; err != nil {
		ResponseJSON(c, http.StatusNotFound, "Product not found", nil)
		return
	}
	ResponseJSON(c, http.StatusOK, "Product retrieved successfully", product)
}

// UpdateProduct godoc
// @Summary Update a product
// @Description Update an existing product by ID
// @Tags Products
// @Accept json
// @Produce json
// @Param id path int true "Product ID"
// @Param product body Product true "Updated product data"
// @Success 200 {object} Product
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /products/{id} [put]
func UpdateProduct(c *gin.Context) {
	var product Product
	if err := DB.First(&product, c.Param("id")).Error; err != nil {
		ResponseJSON(c, http.StatusNotFound, "Product not found", nil)
		return
	}
	// bind the request body
	if err := c.ShouldBindJSON(&product); err != nil {
		ResponseJSON(c, http.StatusBadRequest, "Invalid input", nil)
		return
	}
	DB.Save(&product)
	ResponseJSON(c, http.StatusOK, "Product updated successfully", product)
}

// DeleteProduct godoc
// @Summary Delete a product
// @Description Delete a product by ID
// @Tags Products
// @Produce json
// @Param id path int true "Product ID"
// @Success 200 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /products/{id} [delete]
func DeleteProduct(c *gin.Context) {
	var product Product
	if err := DB.Delete(&product, c.Param("id")).Error; err != nil {
		ResponseJSON(c, http.StatusNotFound, "Product not found", nil)
		return
	}
	ResponseJSON(c, http.StatusOK, "Product deleted successfully", nil)
}

func GenerateJWT(c *gin.Context) {
	var loginRequest LoginRequest
	if err := c.ShouldBindJSON(&loginRequest); err != nil {
		ResponseJSON(c, http.StatusBadRequest, "Invalid request payload", nil)
		return
	}
	if loginRequest.Username != "admin" || loginRequest.Password != "password" {
		ResponseJSON(c, http.StatusUnauthorized, "Invalid credentials", nil)
		return
	}
	expirationTime := time.Now().Add(15 * time.Minute)
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"exp": expirationTime.Unix(),
	})
	// Sign the token
	tokenString, err := token.SignedString(jwtSecret)
	if err != nil {
		ResponseJSON(c, http.StatusInternalServerError, "Could not generate token", nil)
		return
	}
	ResponseJSON(c, http.StatusOK, "Token generated successfully", gin.H{"token": tokenString})
}

// GetCategories godoc
// @Summary Get categories
// @Description Retrieve all categories
// @Tags categories
// @Produce json
// @Success 200 {array} Category
// @Failure 500 {object} map[string]string
// @Router /categories [get]
func GetCategories(c *gin.Context) {
	var categories []Category

	if err := DB.Find(&categories).Error; err != nil {
		ResponseJSON(c, http.StatusInternalServerError, "Failed to retrieve categories", nil)
		return
	}

	ResponseJSON(c, http.StatusOK, "Categories retrieved successfully", categories)
}
