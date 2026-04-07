package api

import (
	"database/sql/driver"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"time"
)

type UserRole string

const (
	RoleAdmin   UserRole = "admin"
	RoleManager UserRole = "manager"
	RoleCashier UserRole = "cashier"
)

func (ur *UserRole) Scan(value interface{}) error {
	*ur = UserRole(value.([]byte))
	return nil
}

func (ur UserRole) Value() (driver.Value, error) {
	return string(ur), nil
}

type User struct {
	ID        uint           `json:"id" gorm:"primaryKey"`
	Name      string         `gorm:"size:127"`
	Username  string         `gorm:"size:64,index"`
	Password  string         `gorm:"size:255"`
	Role      UserRole       `json:"role" gorm:"type:user_role;not null;default:'cashier'"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"deleted_at" gorm:"index"`
}

type Product struct {
	ID          uint           `json:"id" gorm:"primaryKey"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Price       float64        `json:"price"`
	SKU         string         `json:"sku" gorm:"size:64;uniqueIndex"`
	Image       string         `json:"image"`
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
	ID        uint `json:"id" gorm:"primaryKey"`
	User      `json:"user"`
	OrderItem []OrderItem
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"deleted_at" gorm:"index"`
}

type OrderItem struct {
	ID        uint `json:"id" gorm:"primaryKey"`
	OrderID   uint `json:"order_id"`
	Product   Product
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
