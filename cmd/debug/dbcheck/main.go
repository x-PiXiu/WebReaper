// dbcheck：快速验证 OAuth 迁移是否落库（一次性调试工具）。
package main

import (
	"fmt"
	"os"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load("configs/.env")
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		os.Getenv("DB_USER"), os.Getenv("DB_PASSWORD"), os.Getenv("DB_HOST"), os.Getenv("DB_PORT"), os.Getenv("DB_NAME"))
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		fmt.Println("连接失败:", err)
		os.Exit(1)
	}
	var cols []string
	db.Raw("SELECT COLUMN_NAME FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = ? AND TABLE_NAME = 'geo_accounts'", os.Getenv("DB_NAME")).Scan(&cols)
	fmt.Println("geo_accounts 列数:", len(cols))
	for _, want := range []string{"auth_type", "access_token_enc", "refresh_token_enc", "open_id"} {
		found := false
		for _, c := range cols {
			if c == want {
				found = true
				break
			}
		}
		fmt.Printf("  %-18s %v\n", want, found)
	}
	var versions []string
	db.Raw("SELECT version FROM webreaper_schema_migrations ORDER BY version DESC LIMIT 3").Scan(&versions)
	fmt.Println("最近迁移版本:", versions)
}
