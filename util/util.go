package util

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func CreateEnumIfNotExists(db *gorm.DB, enumName string, values []string) error {
	// Check if enum exists
	var exists bool
	checkSQL := `
		SELECT EXISTS (
			SELECT 1
			FROM pg_type t
			JOIN pg_enum e ON t.oid = e.enumtypid
			WHERE t.typname = ?
		)
	`
	if err := db.Raw(checkSQL, enumName).Scan(&exists).Error; err != nil {
		return err
	}

	if exists {
		return nil // enum already exists
	}

	// Build CREATE TYPE statement
	valStr := ""
	for i, v := range values {
		valStr += fmt.Sprintf("'%s'", v)
		if i < len(values)-1 {
			valStr += ", "
		}
	}

	createSQL := fmt.Sprintf("CREATE TYPE %s AS ENUM (%s);", enumName, valStr)
	return db.Exec(createSQL).Error
}

func ParsePagination(c *gin.Context) (limit, page, offset int) {
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

func PaginatedResponse(c *gin.Context, message string, items any, total int64, page, limit int) {
	c.JSON(http.StatusOK, gin.H{
		"status":  http.StatusOK,
		"message": message,
		"data": gin.H{
			"items": items,
			"total": total,
			"page":  page,
			"limit": limit,
		},
	})
}
