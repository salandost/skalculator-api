package api

import (
	"database/sql/driver"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type UserRole string

const (
	RoleAdmin   UserRole = "admin"
	RoleManager UserRole = "manager"
	RoleCashier UserRole = "cashier"
)

func (ur *UserRole) Scan(value interface{}) error {
	switch v := value.(type) {
	case string:
		*ur = UserRole(v)
	case []byte:
		*ur = UserRole(v)
	case nil:
		*ur = RoleCashier
	default:
		*ur = RoleCashier
	}
	return nil
}

func (ur UserRole) Value() (driver.Value, error) {
	return string(ur), nil
}

type User struct {
	ID           uint           `json:"id" gorm:"primaryKey"`
	Name         string         `json:"name" gorm:"size:127"`
	Username     string         `json:"username" gorm:"size:64,index"`
	Password     string         `json:"-" gorm:"size:255"`
	Role         UserRole       `json:"role" gorm:"type:user_role;not null;default:'cashier'"`
	StoreID      *uint          `json:"store_id,omitempty"`
	Store        *Store         `json:"store,omitempty"`
	StoreChainID *uint          `json:"store_chain_id,omitempty"`
	StoreChain   *StoreChain    `json:"store_chain,omitempty"`
	Orders       []Order        `json:"orders" gorm:"foreignKey:UserID"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `json:"deleted_at" gorm:"index"`
}

type Product struct {
	ID          uint           `json:"id" gorm:"primaryKey"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Price       float64        `json:"price"`
	BasePrice   float64        `json:"base_price" gorm:"default:0"`
	Amount      uint           `json:"amount"`
	SKU         string         `json:"sku" gorm:"size:64;uniqueIndex"`
	Image       string         `json:"image"`
	StoreID     uint           `json:"store_id"`
	UserID      uint           `json:"user_id"`
	Categories  []Category     `json:"categories" gorm:"many2many:product_categories;"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `json:"deleted_at" gorm:"index"`
}

type Category struct {
	ID        uint           `json:"id" gorm:"primaryKey"`
	Name      string         `json:"name" gorm:"unique;not null"`
	Products  []Product      `json:"products" gorm:"many2many:product_categories;"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"deleted_at" gorm:"index"`
}

type Order struct {
	ID        uint           `json:"id" gorm:"primaryKey"`
	UserID    uint           `json:"user_id"`
	StoreID   uint           `json:"store_id"`
	OrderItem []OrderItem    `json:"order_items"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"deleted_at" gorm:"index"`
}

type OrderItem struct {
	ID               uint           `json:"id" gorm:"primaryKey"`
	OrderID          uint           `json:"order_id " gorm:"foreignKey:OrderID"`
	ProductID        uint           `json:"product_id" gorm:"foreignKey:ProductID"`
	Product          Product        `json:"product"`
	Amount           uint           `json:"amount"`
	CurrentPrice     float64        `json:"current_price"`
	CurrentBasePrice float64        `json:"current_base_price"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
	DeletedAt        gorm.DeletedAt `json:"deleted_at" gorm:"index"`
}

type Store struct {
	ID           uint           `json:"id" gorm:"primaryKey"`
	Name         string         `json:"name"`
	Orders	   []Order        `json:"orders" gorm:"foreignKey:StoreID"`
	StoreChainID uint           `json:"store_chain_id"`
	Products     []Product      `json:"products" gorm:"foreignKey:StoreID"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `json:"deleted_at" gorm:"index"`
}

type StoreChain struct {
	ID        uint           `json:"id" gorm:"primaryKey"`
	Name      string         `json:"name"`
	Stores    []Store        `json:"stores" gorm:"foreignKey:StoreChainID"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"deleted_at" gorm:"index"`
}

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required,min=6"`
}

type JsonResponse struct {
	Status  int    `json:"status"`
	Message string `json:"message"`
	Data    any    `json:"data"`
}

func ResponseJSON(c *gin.Context, status int, message string, data any) {
	response := JsonResponse{
		Status:  status,
		Message: message,
		Data:    data,
	}
	c.JSON(status, response)
}

type ProductInput struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Price       float64 `json:"price"`
	BasePrice   float64 `json:"base_price"`
	Amount      *uint   `json:"amount,omitempty" binding:"omitempty,min=0"`
	StoreID     *uint   `json:"store_id,omitempty"`
	SKU         string  `json:"sku"`
	Image       string  `json:"image"`
	CategoryIDs []uint  `json:"category_ids"`
}

type CategoryInput struct {
	Name string `json:"name" binding:"required"`
}

type StoreInput struct {
	Name         string `json:"name" binding:"required"`
	StoreChainID uint   `json:"store_chain_id,omitempty"`
}

type StoreChainInput struct {
	Name string `json:"name" binding:"required"`
}

type UserInput struct {
	Name     string   `json:"name"`
	Username string   `json:"username" binding:"required"`
	Password string   `json:"password" binding:"required,min=6"`
	Role     UserRole `json:"role"`
	StoreID  *uint    `json:"store_id,omitempty"`
}

type UserUpdateInput struct {
	Name     string   `json:"name"`
	Password string   `json:"password"`
	Role     UserRole `json:"role"`
	StoreID  *uint    `json:"store_id,omitempty"`
}

type OrderItemInput struct {
	ProductID uint `json:"product_id" binding:"required"`
	Amount    uint `json:"amount" binding:"required,min=1"`
}

type OrderInput struct {
	Items []OrderItemInput `json:"order_items" binding:"required,dive"`
}

type OrderUpdateInput struct {
	UserID  uint             `json:"user_id"`
	StoreID uint             `json:"store_id"`
	Items   []OrderItemInput `json:"order_items"`
}

type OrderItemUpdateInput struct {
	Amount uint `json:"amount" binding:"required,min=1"`
}
