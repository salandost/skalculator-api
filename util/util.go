package util

import (
	"fmt"
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
