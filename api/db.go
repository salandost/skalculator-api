package api

import (
	"log"
	"os"

	"skalculator/util"

	"github.com/joho/godotenv"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB

func InitDB() {
	if err := godotenv.Load(); err != nil {
		log.Println(".env not found or could not be loaded, proceeding with environment variables")
	}

	JwtSecret = []byte(os.Getenv("SECRET_TOKEN"))

	dsn := os.Getenv("DB_URL")
	if dsn == "" {
		log.Fatal("DB_URL is required")
	}

	var err error
	DB, err = gorm.Open(postgres.New(postgres.Config{DSN: dsn, PreferSimpleProtocol: true}), &gorm.Config{})
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	// Ensure enum exists
	if err := util.CreateEnumIfNotExists(DB, "user_role", []string{"admin", "manager", "cashier"}); err != nil {
		log.Fatalf("failed to create enum: %v", err)
	}

	// Auto-migrate models
	if err := DB.AutoMigrate(&Product{}); err != nil {
		log.Fatal("Failed to migrate Product table:", err)
	}
	if err := DB.AutoMigrate(&Category{}); err != nil {
		log.Fatal("Failed to migrate Category table:", err)
	}
	if err := DB.AutoMigrate(&Order{}); err != nil {
		log.Fatal("Failed to migrate Order table:", err)
	}
	if err := DB.AutoMigrate(&OrderItem{}); err != nil {
		log.Fatal("Failed to migrate OrderItem table:", err)
	}
	if err := DB.AutoMigrate(&User{}); err != nil {
		log.Fatal("Failed to migrate User table:", err)
	}
	if err := DB.AutoMigrate(&Store{}); err != nil {
		log.Fatal("Failed to migrate Store table:", err)
	}
	if err := DB.AutoMigrate(&StoreChain{}); err != nil {
		log.Fatal("Failed to migrate StoreChain table:", err)
	}
	if err := DB.AutoMigrate(&Product{}); err != nil {
		log.Fatal("Failed to migrate Product table:", err)
	}
	if err := DB.AutoMigrate(&StoreChain{}); err != nil {
		log.Fatal("Failed to migrate StoreChain table:", err)
	}

	var userCount int64
	var chainCount int64
	if err := DB.Model(&User{}).Count(&userCount).Error; err != nil {
		log.Fatal("Failed to count users:", err)
	}
	if err := DB.Model(&StoreChain{}).Count(&chainCount).Error; err != nil {
		log.Fatal("Failed to count store chains:", err)
	}

	if userCount == 0 && chainCount == 0 {
		log.Println("Seeding initial admin and store chain")
		if err := DB.Transaction(func(tx *gorm.DB) error {
			chain := StoreChain{Name: "Store Chain"}
			if err := tx.Create(&chain).Error; err != nil {
				return err
			}

			hash, err := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.DefaultCost)
			if err != nil {
				return err
			}

			user := User{
				Name:         "Admin",
				Username:     "admin",
				Password:     string(hash),
				Role:         RoleAdmin,
				StoreChainID: &chain.ID,
			}
			if err := tx.Create(&user).Error; err != nil {
				return err
			}

			return nil
		}); err != nil {
			log.Fatal("Failed to seed initial admin user and store chain:", err)
		}
	}
}
