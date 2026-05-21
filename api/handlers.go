package api

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"skalculator/util"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// var DB *gorm.DB

// func InitDB() {
// 	err := godotenv.Load()
// 	if err != nil {
// 		log.Fatal("Failed to connect to database:", err)
// 	}
// 	jwtSecret = []byte(os.Getenv("SECRET_TOKEN"))
// 	dsn := os.Getenv("DB_URL")
// 	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
// 	if err != nil {
// 		log.Fatal("Failed to connect to database:", err)
// 	}

// 	// Create enum if not exists
// 	err = util.CreateEnumIfNotExists(DB, "user_role", []string{"admin", "manager", "cashier"})
// 	if err != nil {
// 		log.Fatalf("failed to create enum: %v", err)
// 	}

// 	// migrate the schema
// 	if err := DB.AutoMigrate(&Product{}); err != nil {
// 		log.Fatal("Failed to migrate Product table:", err)
// 	}

// 	if err := DB.AutoMigrate(&Category{}); err != nil {
// 		log.Fatal("Failed to migrate Category table:", err)
// 	}

// 	if err := DB.AutoMigrate(&Order{}); err != nil {
// 		log.Fatal("Failed to migrate Order table:", err)
// 	}

// 	if err := DB.AutoMigrate(&OrderItem{}); err != nil {
// 		log.Fatal("Failed to migrate OrderItem table:", err)
// 	}

// 	if err := DB.AutoMigrate(&User{}); err != nil {
// 		log.Fatal("Failed to migrate User table:", err)
// 	}

// 		if err := DB.AutoMigrate(&Store{}); err != nil {
// 			log.Fatal("Failed to migrate Store table:", err)
// 		}
// }

func parsePagination(c *gin.Context) (limit, page, offset int) {
	limit = 12
	page = 1

	if l := c.Query("limit"); l != "" {
		fmt.Sscanf(l, "%d", &limit)
	}

	if p := c.Query("page"); p != "" {
		fmt.Sscanf(p, "%d", &page)
	}

	if limit <= 0 {
		limit = 12
	}
	if limit > 100 {
		limit = 100
	}
	if page <= 0 {
		page = 1
	}
	offset = (page - 1) * limit
	return
}

func paginatedResponse(c *gin.Context, message string, items any, total int64, page, limit int) {
	ResponseJSON(c, http.StatusOK, message, gin.H{
		"items": items,
		"total": total,
		"page":  page,
		"limit": limit,
	})
}

// CreateProduct godoc
// @Summary Create a new product
// @Description Create a new product and associate it with categories via category_ids (many-to-many)
// @Tags Products
// @Accept json
// @Produce json
// @Param product body ProductInput true "Product data with category IDs"
// @Success 201 {object} Product "Product with associated categories"
// @Failure 400 {object} map[string]interface{} "Invalid input"
// @Failure 500 {object} map[string]interface{} "Failed to create product"
// @Router /products [post]
func CreateProduct(c *gin.Context) {
	var input ProductInput

	if err := c.ShouldBindJSON(&input); err != nil {
		ResponseJSON(c, http.StatusBadRequest, "Invalid input", nil)
		return
	}

	creator, err := getCurrentUser(c)
	if err != nil {
		ResponseJSON(c, http.StatusUnauthorized, "Unable to identify current user", nil)
		return
	}

	var selectedStoreID uint
	if creator.StoreID != nil {
		selectedStoreID = *creator.StoreID
	} else {
		// creator has no store
		if creator.Role == RoleAdmin {
			if input.StoreID == nil {
				ResponseJSON(c, http.StatusBadRequest, "store_id is required when admin has no store", nil)
				return
			}
			var s Store
			if err := DB.First(&s, *input.StoreID).Error; err != nil {
				ResponseJSON(c, http.StatusBadRequest, "Invalid store", nil)
				return
			}
			if creator.StoreChainID == nil || s.StoreChainID != *creator.StoreChainID {
				ResponseJSON(c, http.StatusBadRequest, "store must belong to the same store chain as admin", nil)
				return
			}
			selectedStoreID = *input.StoreID
		} else {
			ResponseJSON(c, http.StatusBadRequest, "Creator must belong to a store", nil)
			return
		}
	}

	// fetch categories
	var categories []Category
	if len(input.CategoryIDs) > 0 {
		if err := DB.Where("id IN ?", input.CategoryIDs).Find(&categories).Error; err != nil {
			ResponseJSON(c, http.StatusBadRequest, "Invalid categories", nil)
			return
		}
	}

	product := Product{
		Name:        input.Name,
		Description: input.Description,
		Price:       input.Price,
		BasePrice:   input.BasePrice,
		Amount:      0,
		SKU:         input.SKU,
		Image:       input.Image,
		StoreID:     selectedStoreID,
		UserID:      creator.ID,
	}
	if input.Amount != nil {
		product.Amount = *input.Amount
	}

	// create product FIRST
	if err := DB.Create(&product).Error; err != nil {
		ResponseJSON(c, http.StatusInternalServerError, "Failed to create product", nil)
		return
	}

	if len(categories) > 0 {
		if err := DB.Model(&product).Association("Categories").Append(categories); err != nil {
			ResponseJSON(c, http.StatusInternalServerError, "Failed to attach categories", nil)
			return
		}
	}

	DB.Preload("Categories").First(&product, product.ID)

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
// @Param store_id query int false "Filter by store ID"
// @Param store_chain_id query int false "Filter by store chain ID"
// @Param with_deleted query boolean false "Include soft-deleted products"
// @Param with_categories query boolean false "Load categories for products"
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

	applyFilters := func(q *gorm.DB) *gorm.DB {
		if name := c.Query("name"); name != "" {
			q = q.Where("name ILIKE ?", "%"+name+"%")
		}

		if sku := c.Query("sku"); sku != "" {
			q = q.Where("sku = ?", sku)
		}

		if minPrice := c.Query("min_price"); minPrice != "" {
			q = q.Where("price >= ?", minPrice)
		}

		if maxPrice := c.Query("max_price"); maxPrice != "" {
			q = q.Where("price <= ?", maxPrice)
		}

		if from := c.Query("created_from"); from != "" {
			q = q.Where("created_at >= ?", from)
		}

		if to := c.Query("created_to"); to != "" {
			q = q.Where("created_at <= ?", to)
		}

		if storeID := c.Query("store_id"); storeID != "" {
			q = q.Where("store_id = ?", storeID)
		}

		if storeChainID := c.Query("store_chain_id"); storeChainID != "" {
			q = q.Joins("JOIN stores ON stores.id = products.store_id").Where("stores.store_chain_id = ?", storeChainID)
		}

		if withDeleted := c.Query("with_deleted"); withDeleted == "true" {
			q = q.Unscoped()
		}

		return q
	}

	withCategories := strings.EqualFold(c.Query("with_categories"), "true") || c.Query("with_categories") == "1"

	var catIDs []int64
	if q := c.Query("category_ids"); q != "" {
		for _, part := range strings.Split(q, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			if id, err := strconv.ParseInt(part, 10, 64); err == nil {
				catIDs = append(catIDs, id)
			}
		}
	}
	if arr := c.QueryArray("category_ids"); len(arr) > 0 {
		for _, s := range arr {
			if s == "" {
				continue
			}
			if id, err := strconv.ParseInt(s, 10, 64); err == nil {
				catIDs = append(catIDs, id)
			}
		}
	}

	countQuery := applyFilters(DB.Session(&gorm.Session{}).Model(&Product{}))
	fetchQuery := applyFilters(DB.Session(&gorm.Session{}).Model(&Product{}))

	if len(catIDs) > 0 {
		countQuery = countQuery.Joins("JOIN product_categories pc ON pc.product_id = products.id").Where("pc.category_id IN ?", catIDs).Group("products.id")
		fetchQuery = fetchQuery.Joins("JOIN product_categories pc ON pc.product_id = products.id").Where("pc.category_id IN ?", catIDs).Group("products.id")
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

	fetchQuery = fetchQuery.Order(sort + " " + order)

	var total int64
	if err := countQuery.Count(&total).Error; err != nil {
		ResponseJSON(c, http.StatusInternalServerError, "Failed to count products", nil)
		return
	}

	limit, page, offset := util.ParsePagination(c)
	if withCategories {
		fetchQuery = fetchQuery.Preload("Categories")
	}
	fetchQuery = fetchQuery.Limit(limit).Offset(offset)

	if err := fetchQuery.Find(&products).Error; err != nil {
		ResponseJSON(c, http.StatusInternalServerError, "Failed to retrieve products", nil)
		return
	}

	util.PaginatedResponse(c, "Products retrieved successfully", products, total, page, limit)
}

// GetProduct godoc
// @Summary Get a product by ID
// @Description Retrieve a single product by its ID
// @Tags Products
// @Produce json
// @Param id path int true "Product ID"
// @Param with_categories query bool false "Load product categories"
// @Success 200 {object} Product
// @Failure 404 {object} map[string]interface{}
// @Router /products/{id} [get]
func GetProduct(c *gin.Context) {
	var product Product
	withCategories := strings.EqualFold(c.Query("with_categories"), "true") || c.Query("with_categories") == "1"
	query := DB
	if withCategories {
		query = query.Preload("Categories")
	}
	if err := query.First(&product, c.Param("id")).Error; err != nil {
		ResponseJSON(c, http.StatusNotFound, "Product not found", nil)
		return
	}
	ResponseJSON(c, http.StatusOK, "Product retrieved successfully", product)
}

// UpdateProduct godoc
// @Summary Update a product
// @Description Update an existing product and synchronize its categories (replaces existing category associations)
// @Tags Products
// @Accept json
// @Produce json
// @Param id path int true "Product ID"
// @Param product body ProductInput true "Updated product data with category IDs"
// @Success 200 {object} Product "Updated product with categories"
// @Failure 400 {object} map[string]interface{} "Invalid input"
// @Failure 404 {object} map[string]interface{} "Product not found"
// @Failure 500 {object} map[string]interface{} "Failed to update product"
// @Router /products/{id} [put]
func UpdateProduct(c *gin.Context) {
	var product Product

	if err := DB.First(&product, c.Param("id")).Error; err != nil {
		ResponseJSON(c, http.StatusNotFound, "Product not found", nil)
		return
	}

	var input ProductInput
	if err := c.ShouldBindJSON(&input); err != nil {
		ResponseJSON(c, http.StatusBadRequest, "Invalid input", nil)
		return
	}

	// update scalar fields
	product.Name = input.Name
	product.Description = input.Description
	product.Price = input.Price
	product.BasePrice = input.BasePrice
	if input.Amount != nil {
		product.Amount = *input.Amount
	}
	product.SKU = input.SKU
	product.Image = input.Image

	if err := DB.Save(&product).Error; err != nil {
		ResponseJSON(c, http.StatusInternalServerError, "Failed to update product", nil)
		return
	}

	// sync join table
	if input.CategoryIDs != nil {
		var categories []Category

		if len(input.CategoryIDs) > 0 {
			if err := DB.Where("id IN ?", input.CategoryIDs).Find(&categories).Error; err != nil {
				ResponseJSON(c, http.StatusBadRequest, "Invalid categories", nil)
				return
			}
		}

		// Replace = DELETE old + INSERT new in product_categories
		if err := DB.Model(&product).Association("Categories").Replace(categories); err != nil {
			ResponseJSON(c, http.StatusInternalServerError, "Failed to update categories", nil)
			return
		}
	}

	DB.Preload("Categories").First(&product, product.ID)

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

// CreateOrder godoc
// @Summary Create a new order
// @Description Create an order with items
// @Tags Orders
// @Accept json
// @Produce json
// @Param order body object true "Order data"
// @Success 201 {object} Order
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /orders [post]
func CreateOrder(c *gin.Context) {
	var input OrderInput
	if err := c.ShouldBindJSON(&input); err != nil {
		ResponseJSON(c, http.StatusBadRequest, "Invalid input", nil)
		return
	}

	if len(input.Items) == 0 {
		ResponseJSON(c, http.StatusBadRequest, "Order must contain at least one item", nil)
		return
	}

	creator, err := getCurrentUser(c)
	if err != nil {
		ResponseJSON(c, http.StatusUnauthorized, "Unable to identify current user", nil)
		return
	}

	if creator.StoreID == nil {
		ResponseJSON(c, http.StatusBadRequest, "Creator must belong to a store", nil)
		return
	}

	var store Store
	if err := DB.First(&store, *creator.StoreID).Error; err != nil {
		ResponseJSON(c, http.StatusBadRequest, "Creator store not found", nil)
		return
	}

	tx := DB.Begin()
	if tx.Error != nil {
		ResponseJSON(c, http.StatusInternalServerError, "Failed to start transaction", nil)
		return
	}

	order := Order{
		UserID:  creator.ID,
		StoreID: *creator.StoreID,
	}

	if err := tx.Create(&order).Error; err != nil {
		tx.Rollback()
		ResponseJSON(c, http.StatusInternalServerError, "Failed to create order", nil)
		return
	}

	for _, it := range input.Items {
		// validate product exists
		var product Product
		if err := tx.First(&product, it.ProductID).Error; err != nil {
			tx.Rollback()
			ResponseJSON(c, http.StatusBadRequest, "Invalid product in items", nil)
			return
		}

		if it.Amount > product.Amount {
			tx.Rollback()
			ResponseJSON(c, http.StatusBadRequest, fmt.Sprintf("Order item amount %d exceeds product available amount %d", it.Amount, product.Amount), nil)
			return
		}

		orderItem := OrderItem{
			OrderID:          order.ID,
			ProductID:        it.ProductID,
			Amount:           it.Amount,
			CurrentPrice:     product.Price,
			CurrentBasePrice: product.BasePrice,
		}

		product.Amount -= it.Amount
		if err := tx.Save(&product).Error; err != nil {
			tx.Rollback()
			ResponseJSON(c, http.StatusInternalServerError, "Failed to update product inventory", nil)
			return
		}

		if err := tx.Create(&orderItem).Error; err != nil {
			tx.Rollback()
			ResponseJSON(c, http.StatusInternalServerError, "Failed to create order item", nil)
			return
		}
	}

	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		ResponseJSON(c, http.StatusInternalServerError, "Failed to commit transaction", nil)
		return
	}

	// preload items and products
	if err := DB.Preload("OrderItem.Product").First(&order, order.ID).Error; err != nil {
		ResponseJSON(c, http.StatusInternalServerError, "Failed to load order", nil)
		return
	}

	ResponseJSON(c, http.StatusCreated, "Order created successfully", order)
}

// GetOrders godoc
// @Summary Get orders
// @Description Retrieve a list of orders with filtering and pagination
// @Tags Orders
// @Accept json
// @Produce json
// @Param user_id query int false "Filter by user ID"
// @Param store_id query int false "Filter by store ID"
// @Param store_chain_id query int false "Filter by store chain ID"
// @Param created_from query string false "Created from date (YYYY-MM-DD)"
// @Param created_to query string false "Created to date (YYYY-MM-DD)"
// @Param with_deleted query boolean false "Include soft-deleted orders"
// @Param sort query string false "Sort field (id, created_at)" default(id)
// @Param order query string false "Sort order (asc, desc)" default(asc)
// @Param page query int false "Page number" default(1)
// @Param limit query int false "Items per page" default(10)
// @Success 200 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /orders [get]
func GetOrders(c *gin.Context) {
	var orders []Order

	applyFilters := func(q *gorm.DB) *gorm.DB {
		if userID := c.Query("user_id"); userID != "" {
			q = q.Where("user_id = ?", userID)
		}

		if storeID := c.Query("store_id"); storeID != "" {
			q = q.Where("store_id = ?", storeID)
		}

		if storeChainID := c.Query("store_chain_id"); storeChainID != "" {
			q = q.Joins("JOIN stores ON stores.id = orders.store_id").Where("stores.store_chain_id = ?", storeChainID)
		}

		if from := c.Query("created_from"); from != "" {
			q = q.Where("created_at >= ?", from)
		}

		if to := c.Query("created_to"); to != "" {
			q = q.Where("created_at <= ?", to)
		}

		if withDeleted := c.Query("with_deleted"); withDeleted == "true" {
			q = q.Unscoped()
		}

		return q
	}

	countQuery := applyFilters(DB.Session(&gorm.Session{}).Model(&Order{}))

	var total int64
	if err := countQuery.Count(&total).Error; err != nil {
		ResponseJSON(c, http.StatusInternalServerError, "Failed to count orders", nil)
		return
	}

	query := applyFilters(DB.Session(&gorm.Session{}).Model(&Order{})).Preload("OrderItem.Product")

	sort := c.DefaultQuery("sort", "id")
	order := c.DefaultQuery("order", "asc")

	allowedSort := map[string]bool{"id": true, "created_at": true}
	if !allowedSort[sort] {
		sort = "id"
	}
	if order != "asc" && order != "desc" {
		order = "asc"
	}
	query = query.Order(sort + " " + order)

	limit, page, offset := util.ParsePagination(c)
	query = query.Limit(limit).Offset(offset)

	if err := query.Find(&orders).Error; err != nil {
		ResponseJSON(c, http.StatusInternalServerError, "Failed to retrieve orders", nil)
		return
	}

	util.PaginatedResponse(c, "Orders retrieved successfully", orders, total, page, limit)
}

// GetStats godoc
// @Summary Get orders statistics
// @Description Retrieve statistics for orders filtered by product_id, user_id, or store_id. Returns totals and interval arrays (max 25 points).
// @Tags Statistics
// @Accept json
// @Produce json
// @Param product_id query int false "Filter by product ID"
// @Param user_id query int false "Filter by user ID"
// @Param store_id query int false "Filter by store ID"
// @Param from query string false "Start date (RFC3339 or YYYY-MM-DD)"
// @Param to query string false "End date (RFC3339 or YYYY-MM-DD)"
// @Param points query int false "Number of intervals (max 25)"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 403 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /stats [get]
func GetStats(c *gin.Context) {
	creator, err := getCurrentUser(c)
	if err != nil {
		ResponseJSON(c, http.StatusInternalServerError, "Failed to identify creator", nil)
		return
	}

	productID := c.Query("product_id")
	userID := c.Query("user_id")
	storeID := c.Query("store_id")

	// If no filters provided, admin sees chain-wide stats, manager sees store-only stats
	if productID == "" && userID == "" && storeID == "" {
		if creator.Role == RoleManager {
			if creator.StoreID == nil {
				ResponseJSON(c, http.StatusForbidden, "Manager must have a store assigned", nil)
				return
			}
			storeID = fmt.Sprintf("%d", *creator.StoreID)
		} else if creator.Role == RoleAdmin {
			if creator.StoreChainID == nil {
				ResponseJSON(c, http.StatusForbidden, "Admin must have a store chain assigned", nil)
				return
			}
			// Use empty string to signal "all chain" mode
		} else {
			ResponseJSON(c, http.StatusForbidden, "Insufficient permissions", nil)
			return
		}
	}

	fromStr := c.Query("from")
	toStr := c.Query("to")
	pointsQ := c.DefaultQuery("points", "0")

	var fromTime, toTime time.Time
	var parseErr error
	if fromStr != "" {
		fromTime, parseErr = time.Parse(time.RFC3339, fromStr)
		if parseErr != nil {
			fromTime, parseErr = time.Parse("2006-01-02", fromStr)
		}
		if parseErr != nil {
			ResponseJSON(c, http.StatusBadRequest, "Invalid 'from' format", nil)
			return
		}
	}
	if toStr != "" {
		toTime, parseErr = time.Parse(time.RFC3339, toStr)
		if parseErr != nil {
			toTime, parseErr = time.Parse("2006-01-02", toStr)
		}
		if parseErr != nil {
			ResponseJSON(c, http.StatusBadRequest, "Invalid 'to' format", nil)
			return
		}
	}
	if fromStr == "" || toStr == "" {
		toTime = time.Now()
		fromTime = toTime.AddDate(0, 0, -30)
	}
	if toTime.Before(fromTime) {
		ResponseJSON(c, http.StatusBadRequest, "'to' must be after 'from'", nil)
		return
	}

	// determine points (intervals), cap to 25
	totalDuration := toTime.Sub(fromTime)
	points := 0
	if p, err := strconv.Atoi(pointsQ); err == nil && p > 0 {
		if p > 25 {
			points = 25
		} else {
			points = p
		}
	} else {
		// default: one point per day if >=1 day else one per hour
		if totalDuration.Hours() >= 24 {
			points = int(totalDuration.Hours()/24.0) + 1
		} else {
			points = int(totalDuration.Hours()) + 1
		}
		if points <= 0 {
			points = 1
		}
		if points > 25 {
			points = 25
		}
	}

	interval := time.Duration(int64(totalDuration) / int64(points))
	if interval <= 0 {
		interval = time.Second
	}

	// prepare result arrays
	labels := make([]string, points)
	ordersArr := make([]int64, points)
	earningsArr := make([]float64, points)
	unitsArr := make([]int64, points) // product-only
	soldCurrentArr := make([]float64, points)

	// helper: apply creator scoping to a query targeting orders table
	applyScope := func(q *gorm.DB) *gorm.DB {
		if creator.Role == RoleManager && creator.StoreID != nil {
			q = q.Where("orders.store_id = ?", *creator.StoreID)
		} else if creator.Role == RoleAdmin && creator.StoreChainID != nil {
			q = q.Joins("JOIN stores ON stores.id = orders.store_id").Where("stores.store_chain_id = ?", *creator.StoreChainID)
		}
		return q
	}

	// fetch per-interval aggregates
	for i := 0; i < points; i++ {
		start := fromTime.Add(time.Duration(i) * interval)
		end := start.Add(interval)
		if i == points-1 {
			end = toTime
		}
		labels[i] = start.Format(time.RFC3339)

		// base query depending on type
		if productID != "" {
			var row struct {
				TotalOrders int64
				Earnings    float64
				Units       int64
				SoldCurrent float64
			}
			q := DB.Table("order_items").Select("COUNT(DISTINCT orders.id) AS total_orders, COALESCE(SUM(order_items.amount * (products.price - products.base_price)),0) AS earnings, COALESCE(SUM(order_items.amount),0) AS units, COALESCE(SUM(order_items.amount * order_items.current_price),0) AS sold_current").Joins("JOIN orders ON orders.id = order_items.order_id").Joins("JOIN products ON products.id = order_items.product_id").Where("order_items.product_id = ?", productID)
			q = applyScope(q)
			q = q.Where("orders.created_at >= ? AND orders.created_at < ?", start, end)
			if err := q.Scan(&row).Error; err == nil {
				ordersArr[i] = row.TotalOrders
				earningsArr[i] = row.Earnings
				unitsArr[i] = row.Units
				soldCurrentArr[i] = row.SoldCurrent
			}
		} else if userID != "" {
			var row struct {
				TotalOrders int64
				Earnings    float64
				SoldCurrent float64
			}
			q := DB.Table("orders").Select("COUNT(DISTINCT orders.id) AS total_orders, COALESCE(SUM(order_items.amount * (products.price - products.base_price)),0) AS earnings, COALESCE(SUM(order_items.amount * order_items.current_price),0) AS sold_current").Joins("JOIN order_items ON order_items.order_id = orders.id").Joins("JOIN products ON products.id = order_items.product_id").Where("orders.user_id = ?", userID)
			q = applyScope(q)
			q = q.Where("orders.created_at >= ? AND orders.created_at < ?", start, end)
			if err := q.Scan(&row).Error; err == nil {
				ordersArr[i] = row.TotalOrders
				earningsArr[i] = row.Earnings
				soldCurrentArr[i] = row.SoldCurrent
			}
		} else if storeID != "" {
			var row struct {
				TotalOrders int64
				Earnings    float64
				SoldCurrent float64
			}
			q := DB.Table("orders").Select("COUNT(DISTINCT orders.id) AS total_orders, COALESCE(SUM(order_items.amount * (products.price - products.base_price)),0) AS earnings, COALESCE(SUM(order_items.amount * order_items.current_price),0) AS sold_current").Joins("JOIN order_items ON order_items.order_id = orders.id").Joins("JOIN products ON products.id = order_items.product_id").Where("orders.store_id = ?", storeID)
			q = applyScope(q)
			q = q.Where("orders.created_at >= ? AND orders.created_at < ?", start, end)
			if err := q.Scan(&row).Error; err == nil {
				ordersArr[i] = row.TotalOrders
				earningsArr[i] = row.Earnings
				soldCurrentArr[i] = row.SoldCurrent
			}
		} else {
			// No filters: all creator's scope
			var row struct {
				TotalOrders int64
				Earnings    float64
				SoldCurrent float64
			}
			q := DB.Table("orders").Select("COUNT(DISTINCT orders.id) AS total_orders, COALESCE(SUM(order_items.amount * (products.price - products.base_price)),0) AS earnings, COALESCE(SUM(order_items.amount * order_items.current_price),0) AS sold_current").Joins("JOIN order_items ON order_items.order_id = orders.id").Joins("JOIN products ON products.id = order_items.product_id")
			q = applyScope(q)
			q = q.Where("orders.created_at >= ? AND orders.created_at < ?", start, end)
			if err := q.Scan(&row).Error; err == nil {
				ordersArr[i] = row.TotalOrders
				earningsArr[i] = row.Earnings
				soldCurrentArr[i] = row.SoldCurrent
			}
		}
	}

	// totals for full range
	if productID != "" {
		var totals struct {
			TotalOrders int64
			Earnings    float64
			Units       int64
			SoldCurrent float64
		}
		q := DB.Table("order_items").Select("COUNT(DISTINCT orders.id) AS total_orders, COALESCE(SUM(order_items.amount * (products.price - products.base_price)),0) AS earnings, COALESCE(SUM(order_items.amount),0) AS units, COALESCE(SUM(order_items.amount * order_items.current_price),0) AS sold_current").Joins("JOIN orders ON orders.id = order_items.order_id").Joins("JOIN products ON products.id = order_items.product_id").Where("order_items.product_id = ?", productID)
		q = applyScope(q)
		q = q.Where("orders.created_at >= ? AND orders.created_at <= ?", fromTime, toTime)
		if err := q.Scan(&totals).Error; err != nil {
			ResponseJSON(c, http.StatusInternalServerError, "Failed to calculate totals", nil)
			return
		}
		payload := gin.H{
			"total_orders":                      totals.TotalOrders,
			"total_earning":                     totals.Earnings,
			"total_units_sold":                  totals.Units,
			"total_products":                    0,
			"total_sold_by_current_price":       totals.SoldCurrent,
			"points":                            points,
			"labels":                            labels,
			"orders_by_interval":                ordersArr,
			"earnings_by_interval":              earningsArr,
			"units_sold_by_interval":            unitsArr,
			"sold_by_current_price_by_interval": soldCurrentArr,
		}

		// compute total_products: if store_id provided use that store, otherwise for admins use their chain, managers use their store
		var totalProducts int64
		if storeID != "" {
			DB.Table("products").Select("COALESCE(SUM(amount),0)").Where("store_id = ?", storeID).Scan(&totalProducts)
		} else if creator.Role == RoleAdmin && creator.StoreChainID != nil {
			DB.Table("products").Joins("JOIN stores ON stores.id = products.store_id").Select("COALESCE(SUM(products.amount),0)").Where("stores.store_chain_id = ?", *creator.StoreChainID).Scan(&totalProducts)
		} else if creator.Role == RoleManager && creator.StoreID != nil {
			DB.Table("products").Select("COALESCE(SUM(amount),0)").Where("store_id = ?", *creator.StoreID).Scan(&totalProducts)
		}
		payload["total_products"] = totalProducts
		ResponseJSON(c, http.StatusOK, "Stats retrieved", payload)
		return
	}

	// user, store, or global totals
	var totals struct {
		TotalOrders int64
		Earnings    float64
		SoldCurrent float64
	}
	if userID != "" {
		q := DB.Table("orders").Select("COUNT(DISTINCT orders.id) AS total_orders, COALESCE(SUM(order_items.amount * (products.price - products.base_price)),0) AS earnings, COALESCE(SUM(order_items.amount * order_items.current_price),0) AS sold_current").Joins("JOIN order_items ON order_items.order_id = orders.id").Joins("JOIN products ON products.id = order_items.product_id").Where("orders.user_id = ?", userID)
		q = applyScope(q)
		q = q.Where("orders.created_at >= ? AND orders.created_at <= ?", fromTime, toTime)
		if err := q.Scan(&totals).Error; err != nil {
			ResponseJSON(c, http.StatusInternalServerError, "Failed to calculate totals", nil)
			return
		}
	} else if storeID != "" {
		q := DB.Table("orders").Select("COUNT(DISTINCT orders.id) AS total_orders, COALESCE(SUM(order_items.amount * (products.price - products.base_price)),0) AS earnings, COALESCE(SUM(order_items.amount * order_items.current_price),0) AS sold_current").Joins("JOIN order_items ON order_items.order_id = orders.id").Joins("JOIN products ON products.id = order_items.product_id").Where("orders.store_id = ?", storeID)
		q = applyScope(q)
		q = q.Where("orders.created_at >= ? AND orders.created_at <= ?", fromTime, toTime)
		if err := q.Scan(&totals).Error; err != nil {
			ResponseJSON(c, http.StatusInternalServerError, "Failed to calculate totals", nil)
			return
		}
	} else {
		// Global stats: no filter, use creator scope
		q := DB.Table("orders").Select("COUNT(DISTINCT orders.id) AS total_orders, COALESCE(SUM(order_items.amount * (products.price - products.base_price)),0) AS earnings, COALESCE(SUM(order_items.amount * order_items.current_price),0) AS sold_current").Joins("JOIN order_items ON order_items.order_id = orders.id").Joins("JOIN products ON products.id = order_items.product_id")
		q = applyScope(q)
		q = q.Where("orders.created_at >= ? AND orders.created_at <= ?", fromTime, toTime)
		if err := q.Scan(&totals).Error; err != nil {
			ResponseJSON(c, http.StatusInternalServerError, "Failed to calculate totals", nil)
			return
		}
	}

	// compute total_products for final totals: prefer explicit storeID, else admin chain, else manager store
	var totalProducts int64
	if storeID != "" {
		DB.Table("products").Select("COALESCE(SUM(amount),0)").Where("store_id = ?", storeID).Scan(&totalProducts)
	} else if creator.Role == RoleAdmin && creator.StoreChainID != nil {
		DB.Table("products").Joins("JOIN stores ON stores.id = products.store_id").Select("COALESCE(SUM(products.amount),0)").Where("stores.store_chain_id = ?", *creator.StoreChainID).Scan(&totalProducts)
	} else if creator.Role == RoleManager && creator.StoreID != nil {
		DB.Table("products").Select("COALESCE(SUM(amount),0)").Where("store_id = ?", *creator.StoreID).Scan(&totalProducts)
	}

	payload := gin.H{
		"total_orders":                      totals.TotalOrders,
		"total_earning":                     totals.Earnings,
		"total_products":                    totalProducts,
		"total_sold_by_current_price":       totals.SoldCurrent,
		"points":                            points,
		"labels":                            labels,
		"orders_by_interval":                ordersArr,
		"earnings_by_interval":              earningsArr,
		"sold_by_current_price_by_interval": soldCurrentArr,
	}
	ResponseJSON(c, http.StatusOK, "Stats retrieved", payload)
}

// GetOrder godoc
// @Summary Get an order by ID
// @Description Retrieve a single order by ID
// @Tags Orders
// @Produce json
// @Param id path int true "Order ID"
// @Success 200 {object} Order
// @Failure 404 {object} map[string]interface{}
// @Router /orders/{id} [get]
func GetOrder(c *gin.Context) {
	var order Order
	if err := DB.Preload("OrderItem.Product").First(&order, c.Param("id")).Error; err != nil {
		ResponseJSON(c, http.StatusNotFound, "Order not found", nil)
		return
	}
	ResponseJSON(c, http.StatusOK, "Order retrieved successfully", order)
}

// UpdateOrder godoc
// @Summary Update an order
// @Description Update an existing order and its items
// @Tags Orders
// @Accept json
// @Produce json
// @Param id path int true "Order ID"
// @Param order body OrderUpdateInput true "Order update data"
// @Success 200 {object} Order
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /orders/{id} [put]
func UpdateOrder(c *gin.Context) {
	var order Order
	if err := DB.Preload("OrderItem").First(&order, c.Param("id")).Error; err != nil {
		ResponseJSON(c, http.StatusNotFound, "Order not found", nil)
		return
	}

	var input OrderUpdateInput
	if err := c.ShouldBindJSON(&input); err != nil {
		ResponseJSON(c, http.StatusBadRequest, "Invalid input", nil)
		return
	}

	if input.UserID != 0 {
		var user User
		if err := DB.First(&user, input.UserID).Error; err != nil {
			ResponseJSON(c, http.StatusBadRequest, "Invalid user", nil)
			return
		}
		order.UserID = user.ID
	}
	if input.StoreID != 0 {
		var store Store
		if err := DB.First(&store, input.StoreID).Error; err != nil {
			ResponseJSON(c, http.StatusBadRequest, "Invalid store", nil)
			return
		}
		order.StoreID = store.ID
	}

	if input.Items != nil {
		var items []OrderItem
		for _, it := range input.Items {
			var product Product
			if err := DB.First(&product, it.ProductID).Error; err != nil {
				ResponseJSON(c, http.StatusBadRequest, "Invalid product in items", nil)
				return
			}
			items = append(items, OrderItem{OrderID: order.ID, ProductID: it.ProductID, Amount: it.Amount})
		}
		if err := DB.Model(&order).Association("OrderItem").Replace(items); err != nil {
			ResponseJSON(c, http.StatusInternalServerError, "Failed to update order items", nil)
			return
		}
	}

	if err := DB.Save(&order).Error; err != nil {
		ResponseJSON(c, http.StatusInternalServerError, "Failed to update order", nil)
		return
	}

	if err := DB.Preload("OrderItem.Product").First(&order, order.ID).Error; err != nil {
		ResponseJSON(c, http.StatusInternalServerError, "Failed to load order", nil)
		return
	}

	ResponseJSON(c, http.StatusOK, "Order updated successfully", order)
}

// DeleteOrder godoc
// @Summary Delete an order
// @Description Delete an order by ID
// @Tags Orders
// @Produce json
// @Param id path int true "Order ID"
// @Success 200 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /orders/{id} [delete]
func DeleteOrder(c *gin.Context) {
	var order Order
	if err := DB.Delete(&order, c.Param("id")).Error; err != nil {
		ResponseJSON(c, http.StatusNotFound, "Order not found", nil)
		return
	}
	ResponseJSON(c, http.StatusOK, "Order deleted successfully", nil)
}

// CreateOrderItem godoc
// @Summary Create a new order item
// @Description Create an order item for an order
// @Tags OrderItems
// @Accept json
// @Produce json
// @Param order_item body OrderItemInput true "Order item data"
// @Success 201 {object} OrderItem
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /order-items [post]
func CreateOrderItem(c *gin.Context) {
	var input OrderItemInput
	if err := c.ShouldBindJSON(&input); err != nil {
		ResponseJSON(c, http.StatusBadRequest, "Invalid input", nil)
		return
	}

	var product Product
	if err := DB.First(&product, input.ProductID).Error; err != nil {
		ResponseJSON(c, http.StatusBadRequest, "Invalid product", nil)
		return
	}

	orderItem := OrderItem{ProductID: input.ProductID, Amount: input.Amount, CurrentPrice: product.Price, CurrentBasePrice: product.BasePrice}
	if err := DB.Create(&orderItem).Error; err != nil {
		ResponseJSON(c, http.StatusInternalServerError, "Failed to create order item", nil)
		return
	}

	ResponseJSON(c, http.StatusCreated, "Order item created successfully", orderItem)
}

// GetOrderItems godoc
// @Summary Get order items
// @Description Retrieve a list of order items with filtering and pagination
// @Tags OrderItems
// @Accept json
// @Produce json
// @Param order_id query int false "Filter by order ID"
// @Param product_id query int false "Filter by product ID"
// @Param store_id query int false "Filter by store ID"
// @Param with_deleted query boolean false "Include soft-deleted order items"
// @Param sort query string false "Sort field (id, created_at)" default(id)
// @Param order query string false "Sort order (asc, desc)" default(asc)
// @Param page query int false "Page number" default(1)
// @Param limit query int false "Items per page" default(10)
// @Success 200 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /order-items [get]
func GetOrderItems(c *gin.Context) {
	var items []OrderItem

	query := DB.Model(&OrderItem{}).Preload("Product")

	if orderID := c.Query("order_id"); orderID != "" {
		query = query.Where("order_id = ?", orderID)
	}

	if productID := c.Query("product_id"); productID != "" {
		query = query.Where("product_id = ?", productID)
	}

	if storeID := c.Query("store_id"); storeID != "" {
		query = query.Joins("JOIN orders ON orders.id = order_items.order_id").Where("orders.store_id = ?", storeID)
	}

	if withDeleted := c.Query("with_deleted"); withDeleted == "true" {
		query = query.Unscoped()
	}

	sort := c.DefaultQuery("sort", "id")
	order := c.DefaultQuery("order", "asc")

	allowedSort := map[string]bool{"id": true, "created_at": true}
	if !allowedSort[sort] {
		sort = "id"
	}
	if order != "asc" && order != "desc" {
		order = "asc"
	}
	query = query.Order(sort + " " + order)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		ResponseJSON(c, http.StatusInternalServerError, "Failed to count order items", nil)
		return
	}

	limit, page, offset := util.ParsePagination(c)
	query = query.Limit(limit).Offset(offset)

	if err := query.Find(&items).Error; err != nil {
		ResponseJSON(c, http.StatusInternalServerError, "Failed to retrieve order items", nil)
		return
	}

	util.PaginatedResponse(c, "Order items retrieved successfully", items, total, page, limit)
}

// GetOrderItem godoc
// @Summary Get an order item by ID
// @Description Retrieve a single order item by its ID
// @Tags OrderItems
// @Produce json
// @Param id path int true "Order Item ID"
// @Success 200 {object} OrderItem
// @Failure 404 {object} map[string]interface{}
// @Router /order-items/{id} [get]
func GetOrderItem(c *gin.Context) {
	var item OrderItem
	if err := DB.Preload("Product").First(&item, c.Param("id")).Error; err != nil {
		ResponseJSON(c, http.StatusNotFound, "Order item not found", nil)
		return
	}
	ResponseJSON(c, http.StatusOK, "Order item retrieved successfully", item)
}

// UpdateOrderItem godoc
// @Summary Update an order item
// @Description Update the amount for an order item
// @Tags OrderItems
// @Accept json
// @Produce json
// @Param id path int true "Order Item ID"
// @Param order_item body OrderItemUpdateInput true "Order item update data"
// @Success 200 {object} OrderItem
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /order-items/{id} [put]
func UpdateOrderItem(c *gin.Context) {
	var item OrderItem
	if err := DB.First(&item, c.Param("id")).Error; err != nil {
		ResponseJSON(c, http.StatusNotFound, "Order item not found", nil)
		return
	}

	var input OrderItemUpdateInput
	if err := c.ShouldBindJSON(&input); err != nil {
		ResponseJSON(c, http.StatusBadRequest, "Invalid input", nil)
		return
	}

	item.Amount = input.Amount
	if err := DB.Save(&item).Error; err != nil {
		ResponseJSON(c, http.StatusInternalServerError, "Failed to update order item", nil)
		return
	}

	ResponseJSON(c, http.StatusOK, "Order item updated successfully", item)
}

// DeleteOrderItem godoc
// @Summary Delete an order item
// @Description Delete an order item by ID
// @Tags OrderItems
// @Produce json
// @Param id path int true "Order Item ID"
// @Success 200 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /order-items/{id} [delete]
func DeleteOrderItem(c *gin.Context) {
	var item OrderItem
	if err := DB.Delete(&item, c.Param("id")).Error; err != nil {
		ResponseJSON(c, http.StatusNotFound, "Order item not found", nil)
		return
	}
	ResponseJSON(c, http.StatusOK, "Order item deleted successfully", nil)
}

func GenerateJWT(c *gin.Context) {
	var loginRequest LoginRequest
	if err := c.ShouldBindJSON(&loginRequest); err != nil {
		ResponseJSON(c, http.StatusBadRequest, "Invalid request payload", nil)
		return
	}

	// find user
	var user User
	if err := DB.Where("username = ?", loginRequest.Username).First(&user).Error; err != nil {
		ResponseJSON(c, http.StatusUnauthorized, "Invalid credentials", nil)
		return
	}

	// compare password
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(loginRequest.Password)); err != nil {
		ResponseJSON(c, http.StatusUnauthorized, "Invalid credentials", nil)
		return
	}

	expirationTime := time.Now().Add(24 * time.Hour)
	claims := JWTClaims{
		UserID:   user.ID,
		Username: user.Username,
		Role:     user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   fmt.Sprint(user.ID),
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	if len(JwtSecret) == 0 {
		JwtSecret = []byte(os.Getenv("SECRET_TOKEN"))
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(JwtSecret)
	if err != nil {
		ResponseJSON(c, http.StatusInternalServerError, "Could not generate token", nil)
		return
	}
	user.Password = ""
	ResponseJSON(c, http.StatusOK, "Token generated successfully", gin.H{"token": tokenString, "user": user})
}

// RegisterUser godoc
// @Summary Register a new user
// @Description Create a new user account
// @Tags Auth
// @Accept json
// @Produce json
// @Param user body User true "User registration"
// @Success 201 {object} User
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /register [post]
func RegisterUser(c *gin.Context) {
	var input UserInput
	if err := c.ShouldBindJSON(&input); err != nil {
		ResponseJSON(c, http.StatusBadRequest, "Invalid input", nil)
		return
	}

	if input.Role == "" {
		input.Role = RoleCashier
	}

	if input.Role == RoleAdmin {
		ResponseJSON(c, http.StatusBadRequest, "admin registration is not allowed", nil)
		return
	}

	// check existing username
	var exists User
	if err := DB.Where("username = ?", input.Username).First(&exists).Error; err == nil {
		ResponseJSON(c, http.StatusBadRequest, "username already taken", nil)
		return
	}

	// hash password
	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		ResponseJSON(c, http.StatusInternalServerError, "Failed to hash password", nil)
		return
	}

	if input.StoreID == nil {
		ResponseJSON(c, http.StatusBadRequest, "store_id is required for manager or cashier registration", nil)
		return
	}

	var store Store
	if err := DB.First(&store, *input.StoreID).Error; err != nil {
		ResponseJSON(c, http.StatusBadRequest, "Invalid store", nil)
		return
	}

	user := User{
		Name:         input.Name,
		Username:     input.Username,
		Password:     string(hash),
		Role:         input.Role,
		StoreID:      input.StoreID,
		StoreChainID: &store.StoreChainID,
	}

	if err := DB.Create(&user).Error; err != nil {
		ResponseJSON(c, http.StatusInternalServerError, "Failed to create user", nil)
		return
	}

	user.Password = ""
	ResponseJSON(c, http.StatusCreated, "User registered successfully", user)
}

func getCurrentUser(c *gin.Context) (*User, error) {
	userIDValue, exists := c.Get("userID")
	if !exists {
		return nil, fmt.Errorf("current user not found in context")
	}

	var userID uint
	switch v := userIDValue.(type) {
	case uint:
		userID = v
	case int:
		userID = uint(v)
	case int64:
		userID = uint(v)
	case float64:
		userID = uint(v)
	default:
		return nil, fmt.Errorf("invalid userID type")
	}

	var user User
	if err := DB.Preload("Store").Preload("StoreChain").First(&user, userID).Error; err != nil {
		return nil, err
	}

	return &user, nil
}

// CreateUser godoc
// @Summary Create a new user
// @Description Create a new user account
// @Tags Users
// @Accept json
// @Produce json
// @Param user body UserInput true "User data"
// @Success 201 {object} User
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /users [post]
func CreateUser(c *gin.Context) {
	creator, err := getCurrentUser(c)
	if err != nil {
		ResponseJSON(c, http.StatusUnauthorized, "Unable to identify current user", nil)
		return
	}

	var input UserInput
	if err := c.ShouldBindJSON(&input); err != nil {
		ResponseJSON(c, http.StatusBadRequest, "Invalid input", nil)
		return
	}

	if input.Role == "" {
		input.Role = RoleCashier
	}

	if input.Role == RoleAdmin {
		if creator.Role != RoleAdmin {
			ResponseJSON(c, http.StatusForbidden, "Only admins can create admin users", nil)
			return
		}
		if creator.StoreChainID == nil {
			ResponseJSON(c, http.StatusBadRequest, "Creator admin must belong to a store chain", nil)
			return
		}
	}

	var store *Store
	if creator.Role == RoleManager || creator.Role == RoleCashier {
		if creator.StoreID == nil {
			ResponseJSON(c, http.StatusBadRequest, "Creator does not belong to a store", nil)
			return
		}
		store = &Store{}
		if err := DB.First(store, *creator.StoreID).Error; err != nil {
			ResponseJSON(c, http.StatusInternalServerError, "Creator store not found", nil)
			return
		}
		input.StoreID = creator.StoreID
	}

	if input.Role != RoleAdmin && input.StoreID == nil {
		ResponseJSON(c, http.StatusBadRequest, "Store ID is required for non-admin users", nil)
		return
	}

	if input.StoreID != nil {
		if store == nil {
			store = &Store{}
			if err := DB.First(store, *input.StoreID).Error; err != nil {
				ResponseJSON(c, http.StatusBadRequest, "Invalid store", nil)
				return
			}
		}
	}

	if !validateUserRole(input.Role) {
		ResponseJSON(c, http.StatusBadRequest, "Invalid role", nil)
		return
	}

	// check existing username
	var exists User
	if err := DB.Where("username = ?", input.Username).First(&exists).Error; err == nil {
		ResponseJSON(c, http.StatusBadRequest, "username already taken", nil)
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		ResponseJSON(c, http.StatusInternalServerError, "Failed to hash password", nil)
		return
	}

	newUser := User{
		Name:     input.Name,
		Username: input.Username,
		Password: string(hash),
		Role:     input.Role,
	}

	if input.Role == RoleAdmin {
		newUser.StoreID = nil
		newUser.StoreChainID = creator.StoreChainID
	} else {
		newUser.StoreID = input.StoreID
		if store != nil {
			newUser.StoreChainID = &store.StoreChainID
		}
	}

	if err := DB.Create(&newUser).Error; err != nil {
		ResponseJSON(c, http.StatusInternalServerError, "Failed to create user", nil)
		return
	}

	newUser.Password = ""
	ResponseJSON(c, http.StatusCreated, "User created successfully", newUser)
}

func validateUserRole(role UserRole) bool {
	switch role {
	case RoleAdmin, RoleManager, RoleCashier:
		return true
	default:
		return false
	}
}

// GetUsers godoc
// @Summary Get users
// @Description Retrieve a list of registered users with filtering and pagination
// @Tags Users
// @Produce json
// @Param username query string false "Filter by username"
// @Param name query string false "Filter by name"
// @Param role query string false "Filter by role"
// @Param store_id query int false "Filter users by store ID"
// @Param store_chain_id query int false "Filter users by store chain ID"
// @Param with_deleted query boolean false "Include soft-deleted users"
// @Param sort query string false "Sort field (id, username, created_at)" default(id)
// @Param order query string false "Sort order (asc, desc)" default(asc)
// @Param page query int false "Page number" default(1)
// @Param limit query int false "Items per page" default(10)
// @Success 200 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /users [get]
func GetUsers(c *gin.Context) {
	var users []User

	query := DB.Model(&User{})

	if username := c.Query("username"); username != "" {
		query = query.Where("username ILIKE ?", "%"+username+"%")
	}

	if name := c.Query("name"); name != "" {
		query = query.Where("name ILIKE ?", "%"+name+"%")
	}

	if role := c.Query("role"); role != "" {
		query = query.Where("role = ?", role)
	}

	if storeID := c.Query("store_id"); storeID != "" {
		query = query.Where("users.store_id = ?", storeID)
	}

	if storeChainID := c.Query("store_chain_id"); storeChainID != "" {
		query = query.Joins("LEFT JOIN stores ON stores.id = users.store_id").Joins("LEFT JOIN store_chains ON store_chains.id = users.store_chain_id OR store_chains.id = stores.store_chain_id").Where("store_chains.id = ?", storeChainID).Group("users.id")
	}

	if withDeleted := c.Query("with_deleted"); withDeleted == "true" {
		query = query.Unscoped()
	}

	sort := c.DefaultQuery("sort", "id")
	order := c.DefaultQuery("order", "asc")

	allowedSort := map[string]bool{"id": true, "username": true, "created_at": true}
	if !allowedSort[sort] {
		sort = "id"
	}
	if order != "asc" && order != "desc" {
		order = "asc"
	}
	query = query.Order(sort + " " + order)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		ResponseJSON(c, http.StatusInternalServerError, "Failed to count users", nil)
		return
	}

	limit, page, offset := util.ParsePagination(c)
	query = query.Limit(limit).Offset(offset)

	if err := query.Find(&users).Error; err != nil {
		ResponseJSON(c, http.StatusInternalServerError, "Failed to retrieve users", nil)
		return
	}

	for i := range users {
		users[i].Password = ""
	}

	util.PaginatedResponse(c, "Users retrieved successfully", users, total, page, limit)
}

// GetUser godoc
// @Summary Get a user by ID
// @Description Retrieve a single user by its ID
// @Tags Users
// @Produce json
// @Param id path int true "User ID"
// @Success 200 {object} User
// @Failure 404 {object} map[string]interface{}
// @Router /users/{id} [get]
func GetUser(c *gin.Context) {
	var user User
	if err := DB.First(&user, c.Param("id")).Error; err != nil {
		ResponseJSON(c, http.StatusNotFound, "User not found", nil)
		return
	}
	user.Password = ""
	ResponseJSON(c, http.StatusOK, "User retrieved successfully", user)
}

// UpdateUser godoc
// @Summary Update a user
// @Description Update an existing user account
// @Tags Users
// @Accept json
// @Produce json
// @Param id path int true "User ID"
// @Param user body UserUpdateInput true "User update data"
// @Success 200 {object} User
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /users/{id} [put]
func UpdateUser(c *gin.Context) {
	var user User
	if err := DB.First(&user, c.Param("id")).Error; err != nil {
		ResponseJSON(c, http.StatusNotFound, "User not found", nil)
		return
	}

	var input UserUpdateInput
	if err := c.ShouldBindJSON(&input); err != nil {
		ResponseJSON(c, http.StatusBadRequest, "Invalid input", nil)
		return
	}

	if input.Name != "" {
		user.Name = input.Name
	}
	if input.Role != "" {
		user.Role = input.Role
	}
	if input.Password != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
		if err != nil {
			ResponseJSON(c, http.StatusInternalServerError, "Failed to hash password", nil)
			return
		}
		user.Password = string(hash)
	}

	if input.StoreID != nil {
		var targetStore Store
		if err := DB.First(&targetStore, *input.StoreID).Error; err != nil {
			ResponseJSON(c, http.StatusBadRequest, "Invalid store", nil)
			return
		}

		if user.Role == RoleAdmin {
			ResponseJSON(c, http.StatusBadRequest, "admin users cannot be assigned a store", nil)
			return
		}

		if user.StoreChainID == nil && user.StoreID != nil {
			var currentStore Store
			if err := DB.First(&currentStore, *user.StoreID).Error; err == nil {
				user.StoreChainID = &currentStore.StoreChainID
			}
		}

		if user.StoreChainID != nil && targetStore.StoreChainID != *user.StoreChainID {
			ResponseJSON(c, http.StatusBadRequest, "store must belong to the same store chain", nil)
			return
		}

		user.StoreID = input.StoreID
		user.StoreChainID = &targetStore.StoreChainID
	}

	if user.Role == RoleAdmin {
		if input.StoreID != nil {
			ResponseJSON(c, http.StatusBadRequest, "admin users cannot be assigned a store", nil)
			return
		}
		user.StoreID = nil
		if user.StoreChainID == nil {
			ResponseJSON(c, http.StatusBadRequest, "admin users must belong to a store chain", nil)
			return
		}
	} else {
		if user.StoreID == nil {
			ResponseJSON(c, http.StatusBadRequest, "manager or cashier users must belong to a store", nil)
			return
		}
		if user.StoreChainID == nil {
			var store Store
			if err := DB.First(&store, *user.StoreID).Error; err != nil {
				ResponseJSON(c, http.StatusInternalServerError, "Failed to determine store chain", nil)
				return
			}
			user.StoreChainID = &store.StoreChainID
		}
	}

	if err := DB.Save(&user).Error; err != nil {
		ResponseJSON(c, http.StatusInternalServerError, "Failed to update user", nil)
		return
	}

	user.Password = ""
	ResponseJSON(c, http.StatusOK, "User updated successfully", user)
}

// DeleteUser godoc
// @Summary Delete a user
// @Description Delete a user by ID
// @Tags Users
// @Produce json
// @Param id path int true "User ID"
// @Success 200 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /users/{id} [delete]
func DeleteUser(c *gin.Context) {
	var user User
	if err := DB.Delete(&user, c.Param("id")).Error; err != nil {
		ResponseJSON(c, http.StatusNotFound, "User not found", nil)
		return
	}
	ResponseJSON(c, http.StatusOK, "User deleted successfully", nil)
}

// CreateStore godoc
// @Summary Create a new store
// @Description Create a new store
// @Tags Stores
// @Accept json
// @Produce json
// @Param store body StoreInput true "Store data"
// @Success 201 {object} Store
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /stores [post]
func CreateStore(c *gin.Context) {
	var input StoreInput
	if err := c.ShouldBindJSON(&input); err != nil {
		ResponseJSON(c, http.StatusBadRequest, "Invalid input", nil)
		return
	}

	creator, err := getCurrentUser(c)
	if err != nil {
		ResponseJSON(c, http.StatusInternalServerError, "Failed to identify creator", nil)
		return
	}

	if creator.StoreChainID == nil {
		ResponseJSON(c, http.StatusBadRequest, "Creator admin must belong to a store chain", nil)
		return
	}

	storeChainID := *creator.StoreChainID
	if input.StoreChainID != 0 && input.StoreChainID != storeChainID {
		ResponseJSON(c, http.StatusBadRequest, "store_chain_id must match creator's store chain", nil)
		return
	}

	store := Store{Name: input.Name, StoreChainID: storeChainID}
	if err := DB.Create(&store).Error; err != nil {
		ResponseJSON(c, http.StatusInternalServerError, "Failed to create store", nil)
		return
	}

	ResponseJSON(c, http.StatusCreated, "Store created successfully", store)
}

// GetStores godoc
// @Summary Get stores
// @Description Retrieve a list of stores with filtering and pagination
// @Tags Stores
// @Accept json
// @Produce json
// @Param name query string false "Filter by store name"
// @Param store_chain_id query int false "Filter by store chain ID"
// @Param with_deleted query boolean false "Include soft-deleted stores"
// @Param sort query string false "Sort field (id, name, created_at)" default(id)
// @Param order query string false "Sort order (asc, desc)" default(asc)
// @Param page query int false "Page number" default(1)
// @Param limit query int false "Items per page" default(10)
// @Success 200 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /stores [get]
func GetStores(c *gin.Context) {
	var stores []Store

	query := DB.Model(&Store{})

	if name := c.Query("name"); name != "" {
		query = query.Where("name ILIKE ?", "%"+name+"%")
	}

	if storeChainID := c.Query("store_chain_id"); storeChainID != "" {
		query = query.Where("store_chain_id = ?", storeChainID)
	}

	if withDeleted := c.Query("with_deleted"); withDeleted == "true" {
		query = query.Unscoped()
	}

	sort := c.DefaultQuery("sort", "id")
	order := c.DefaultQuery("order", "asc")

	allowedSort := map[string]bool{"id": true, "name": true, "created_at": true}
	if !allowedSort[sort] {
		sort = "id"
	}
	if order != "asc" && order != "desc" {
		order = "asc"
	}
	query = query.Order(sort + " " + order)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		ResponseJSON(c, http.StatusInternalServerError, "Failed to count stores", nil)
		return
	}

	limit, page, offset := util.ParsePagination(c)
	query = query.Limit(limit).Offset(offset)

	if err := query.Find(&stores).Error; err != nil {
		ResponseJSON(c, http.StatusInternalServerError, "Failed to retrieve stores", nil)
		return
	}

	util.PaginatedResponse(c, "Stores retrieved successfully", stores, total, page, limit)
}

// GetStore godoc
// @Summary Get a store by ID
// @Description Retrieve a single store by its ID
// @Tags Stores
// @Produce json
// @Param id path int true "Store ID"
// @Success 200 {object} Store
// @Failure 404 {object} map[string]interface{}
// @Router /stores/{id} [get]
func GetStore(c *gin.Context) {
	var store Store
	if err := DB.First(&store, c.Param("id")).Error; err != nil {
		ResponseJSON(c, http.StatusNotFound, "Store not found", nil)
		return
	}
	ResponseJSON(c, http.StatusOK, "Store retrieved successfully", store)
}

// UpdateStore godoc
// @Summary Update a store
// @Description Update an existing store
// @Tags Stores
// @Accept json
// @Produce json
// @Param id path int true "Store ID"
// @Param store body StoreInput true "Store data"
// @Success 200 {object} Store
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /stores/{id} [put]
func UpdateStore(c *gin.Context) {
	var store Store
	if err := DB.First(&store, c.Param("id")).Error; err != nil {
		ResponseJSON(c, http.StatusNotFound, "Store not found", nil)
		return
	}

	var input StoreInput
	if err := c.ShouldBindJSON(&input); err != nil {
		ResponseJSON(c, http.StatusBadRequest, "Invalid input", nil)
		return
	}

	if input.Name != "" {
		store.Name = input.Name
	}
	if input.StoreChainID != 0 {
		var storeChain StoreChain
		if err := DB.First(&storeChain, input.StoreChainID).Error; err != nil {
			ResponseJSON(c, http.StatusBadRequest, "Invalid store chain", nil)
			return
		}
		store.StoreChainID = input.StoreChainID
	}

	if err := DB.Save(&store).Error; err != nil {
		ResponseJSON(c, http.StatusInternalServerError, "Failed to update store", nil)
		return
	}

	ResponseJSON(c, http.StatusOK, "Store updated successfully", store)
}

// DeleteStore godoc
// @Summary Delete a store
// @Description Delete a store by ID
// @Tags Stores
// @Produce json
// @Param id path int true "Store ID"
// @Success 200 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /stores/{id} [delete]
func DeleteStore(c *gin.Context) {
	var store Store
	if err := DB.Delete(&store, c.Param("id")).Error; err != nil {
		ResponseJSON(c, http.StatusNotFound, "Store not found", nil)
		return
	}
	ResponseJSON(c, http.StatusOK, "Store deleted successfully", nil)
}

// CreateStoreChain godoc
// @Summary Create a new store chain
// @Description Create a new store chain
// @Tags StoreChains
// @Accept json
// @Produce json
// @Param storeChain body StoreChainInput true "Store chain data"
// @Success 201 {object} StoreChain
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /store-chains [post]
func CreateStoreChain(c *gin.Context) {
	var input StoreChainInput
	if err := c.ShouldBindJSON(&input); err != nil {
		ResponseJSON(c, http.StatusBadRequest, "Invalid input", nil)
		return
	}

	storeChain := StoreChain{Name: input.Name}
	if err := DB.Create(&storeChain).Error; err != nil {
		ResponseJSON(c, http.StatusInternalServerError, "Failed to create store chain", nil)
		return
	}

	ResponseJSON(c, http.StatusCreated, "Store chain created successfully", storeChain)
}

// GetStoreChains godoc
// @Summary Get store chains
// @Description Retrieve a list of store chains with filtering and pagination
// @Tags StoreChains
// @Accept json
// @Produce json
// @Param name query string false "Filter by store chain name"
// @Param with_deleted query boolean false "Include soft-deleted store chains"
// @Param sort query string false "Sort field (id, name, created_at)" default(id)
// @Param order query string false "Sort order (asc, desc)" default(asc)
// @Param page query int false "Page number" default(1)
// @Param limit query int false "Items per page" default(10)
// @Success 200 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /store-chains [get]
func GetStoreChains(c *gin.Context) {
	var storeChains []StoreChain

	query := DB.Model(&StoreChain{})

	if name := c.Query("name"); name != "" {
		query = query.Where("name ILIKE ?", "%"+name+"%")
	}

	if withDeleted := c.Query("with_deleted"); withDeleted == "true" {
		query = query.Unscoped()
	}

	sort := c.DefaultQuery("sort", "id")
	order := c.DefaultQuery("order", "asc")

	allowedSort := map[string]bool{"id": true, "name": true, "created_at": true}
	if !allowedSort[sort] {
		sort = "id"
	}
	if order != "asc" && order != "desc" {
		order = "asc"
	}
	query = query.Order(sort + " " + order)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		ResponseJSON(c, http.StatusInternalServerError, "Failed to count store chains", nil)
		return
	}

	limit, page, offset := util.ParsePagination(c)
	query = query.Limit(limit).Offset(offset).Preload("Stores")

	if err := query.Find(&storeChains).Error; err != nil {
		ResponseJSON(c, http.StatusInternalServerError, "Failed to retrieve store chains", nil)
		return
	}

	util.PaginatedResponse(c, "Store chains retrieved successfully", storeChains, total, page, limit)
}

// GetStoreChain godoc
// @Summary Get a store chain by ID
// @Description Retrieve a single store chain by its ID with all associated stores
// @Tags StoreChains
// @Produce json
// @Param id path int true "Store chain ID"
// @Success 200 {object} StoreChain
// @Failure 404 {object} map[string]interface{}
// @Router /store-chains/{id} [get]
func GetStoreChain(c *gin.Context) {
	var storeChain StoreChain
	if err := DB.Preload("Stores").First(&storeChain, c.Param("id")).Error; err != nil {
		ResponseJSON(c, http.StatusNotFound, "Store chain not found", nil)
		return
	}
	ResponseJSON(c, http.StatusOK, "Store chain retrieved successfully", storeChain)
}

// UpdateStoreChain godoc
// @Summary Update a store chain
// @Description Update an existing store chain
// @Tags StoreChains
// @Accept json
// @Produce json
// @Param id path int true "Store chain ID"
// @Param storeChain body StoreChainInput true "Store chain data"
// @Success 200 {object} StoreChain
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /store-chains/{id} [put]
func UpdateStoreChain(c *gin.Context) {
	var storeChain StoreChain
	if err := DB.First(&storeChain, c.Param("id")).Error; err != nil {
		ResponseJSON(c, http.StatusNotFound, "Store chain not found", nil)
		return
	}

	var input StoreChainInput
	if err := c.ShouldBindJSON(&input); err != nil {
		ResponseJSON(c, http.StatusBadRequest, "Invalid input", nil)
		return
	}

	if input.Name != "" {
		storeChain.Name = input.Name
	}

	if err := DB.Save(&storeChain).Error; err != nil {
		ResponseJSON(c, http.StatusInternalServerError, "Failed to update store chain", nil)
		return
	}

	ResponseJSON(c, http.StatusOK, "Store chain updated successfully", storeChain)
}

// DeleteStoreChain godoc
// @Summary Delete a store chain
// @Description Delete a store chain by ID
// @Tags StoreChains
// @Produce json
// @Param id path int true "Store chain ID"
// @Success 200 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /store-chains/{id} [delete]
func DeleteStoreChain(c *gin.Context) {
	var storeChain StoreChain
	if err := DB.Delete(&storeChain, c.Param("id")).Error; err != nil {
		ResponseJSON(c, http.StatusNotFound, "Store chain not found", nil)
		return
	}
	ResponseJSON(c, http.StatusOK, "Store chain deleted successfully", nil)
}

// CreateCategory godoc
// @Summary Create a new category
// @Description Create a new category
// @Tags Categories
// @Accept json
// @Produce json
// @Param category body CategoryInput true "Category data"
// @Success 201 {object} Category
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /categories [post]
func CreateCategory(c *gin.Context) {
	var input CategoryInput
	if err := c.ShouldBindJSON(&input); err != nil {
		ResponseJSON(c, http.StatusBadRequest, "Invalid input", nil)
		return
	}

	category := Category{Name: input.Name}
	if err := DB.Create(&category).Error; err != nil {
		ResponseJSON(c, http.StatusInternalServerError, "Failed to create category", nil)
		return
	}

	ResponseJSON(c, http.StatusCreated, "Category created successfully", category)
}

// GetCategories godoc
// @Summary Get categories
// @Description Retrieve a list of categories with filtering and pagination
// @Tags Categories
// @Accept json
// @Produce json
// @Param name query string false "Filter by category name"
// @Param with_deleted query boolean false "Include soft-deleted categories"
// @Param sort query string false "Sort field (id, name, created_at)" default(id)
// @Param order query string false "Sort order (asc, desc)" default(asc)
// @Param page query int false "Page number" default(1)
// @Param limit query int false "Items per page" default(10)
// @Success 200 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /categories [get]
func GetCategories(c *gin.Context) {
	var categories []Category

	query := DB.Model(&Category{})

	if name := c.Query("name"); name != "" {
		query = query.Where("name ILIKE ?", "%"+name+"%")
	}

	if withDeleted := c.Query("with_deleted"); withDeleted == "true" {
		query = query.Unscoped()
	}

	sort := c.DefaultQuery("sort", "id")
	order := c.DefaultQuery("order", "asc")

	allowedSort := map[string]bool{"id": true, "name": true, "created_at": true}
	if !allowedSort[sort] {
		sort = "id"
	}
	if order != "asc" && order != "desc" {
		order = "asc"
	}
	query = query.Order(sort + " " + order)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		ResponseJSON(c, http.StatusInternalServerError, "Failed to count categories", nil)
		return
	}

	limit, page, offset := parsePagination(c)
	query = query.Limit(limit).Offset(offset)

	if err := query.Find(&categories).Error; err != nil {
		ResponseJSON(c, http.StatusInternalServerError, "Failed to retrieve categories", nil)
		return
	}

	paginatedResponse(c, "Categories retrieved successfully", categories, total, page, limit)
}

// GetCategory godoc
// @Summary Get a category by ID
// @Description Retrieve a single category by its ID
// @Tags Categories
// @Produce json
// @Param id path int true "Category ID"
// @Success 200 {object} Category
// @Failure 404 {object} map[string]interface{}
// @Router /categories/{id} [get]
func GetCategory(c *gin.Context) {
	var category Category
	if err := DB.First(&category, c.Param("id")).Error; err != nil {
		ResponseJSON(c, http.StatusNotFound, "Category not found", nil)
		return
	}
	ResponseJSON(c, http.StatusOK, "Category retrieved successfully", category)
}

// UpdateCategory godoc
// @Summary Update a category
// @Description Update an existing category
// @Tags Categories
// @Accept json
// @Produce json
// @Param id path int true "Category ID"
// @Param category body CategoryInput true "Category data"
// @Success 200 {object} Category
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /categories/{id} [put]
func UpdateCategory(c *gin.Context) {
	var category Category
	if err := DB.First(&category, c.Param("id")).Error; err != nil {
		ResponseJSON(c, http.StatusNotFound, "Category not found", nil)
		return
	}

	var input CategoryInput
	if err := c.ShouldBindJSON(&input); err != nil {
		ResponseJSON(c, http.StatusBadRequest, "Invalid input", nil)
		return
	}

	category.Name = input.Name
	if err := DB.Save(&category).Error; err != nil {
		ResponseJSON(c, http.StatusInternalServerError, "Failed to update category", nil)
		return
	}

	ResponseJSON(c, http.StatusOK, "Category updated successfully", category)
}

// DeleteCategory godoc
// @Summary Delete a category
// @Description Delete a category by ID
// @Tags Categories
// @Produce json
// @Param id path int true "Category ID"
// @Success 200 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /categories/{id} [delete]
func DeleteCategory(c *gin.Context) {
	var category Category
	if err := DB.Delete(&category, c.Param("id")).Error; err != nil {
		ResponseJSON(c, http.StatusNotFound, "Category not found", nil)
		return
	}
	ResponseJSON(c, http.StatusOK, "Category deleted successfully", nil)
}
