package main

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func main() {
	_ = godotenv.Load("configs/.env")
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		os.Getenv("DB_USER"), os.Getenv("DB_PASSWORD"), os.Getenv("DB_HOST"), os.Getenv("DB_PORT"), os.Getenv("DB_NAME"))
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		fmt.Println("connect fail:", err)
		os.Exit(1)
	}
	var rows []map[string]any
	db.Raw("SELECT id, platform, status, external_url, LEFT(error_msg,120) AS err FROM geo_publish_jobs ORDER BY created_at DESC LIMIT 2").Scan(&rows)
	f, _ := os.Create("data/job_check.txt")
	defer f.Close()
	for _, r := range rows {
		fmt.Fprintf(f, "%v | %v | %v | url=%v | err=%v\n", r["id"], r["platform"], r["status"], r["external_url"], r["err"])
	}
}
