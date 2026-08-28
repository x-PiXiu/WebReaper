// crawleracccheck：crawler_accounts 表检查/清理工具（一次性）。
//
// 背景：旧版前端扫码成功后会补创建一条 cookie 为占位符"从accounts表同步"的
// 记录（明文非密文，解密必失败、永远无法被账号池使用）——本工具列出并清理。
//
// 用法：
//
//	go run ./cmd/crawleracccheck            # 只列出
//	go run ./cmd/crawleracccheck -cleanup   # 删除占位符 cookie 的记录
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/joho/godotenv"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type crawlerAccountRow struct {
	ID                int64  `gorm:"column:id"`
	Platform          string `gorm:"column:platform"`
	AccountName       string `gorm:"column:account_name"`
	CookieEncrypted   string `gorm:"column:cookie_encrypted"`
	Status            string `gorm:"column:status"`
	HealthCheckResult string `gorm:"column:health_check_result"`
	DailyUsageCount   int    `gorm:"column:daily_usage_count"`
	DailyUsageLimit   int    `gorm:"column:daily_usage_limit"`
}

const placeholder = "从accounts表同步"

func main() {
	cleanup := flag.Bool("cleanup", false, "删除占位符 cookie 的记录")
	flag.Parse()

	_ = godotenv.Load("configs/.env")
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		os.Getenv("DB_USER"), os.Getenv("DB_PASSWORD"), os.Getenv("DB_HOST"),
		os.Getenv("DB_PORT"), os.Getenv("DB_NAME"))
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		fmt.Println("连接失败:", err)
		os.Exit(1)
	}

	var rows []crawlerAccountRow
	if err := db.Table("crawler_accounts").Order("id").Find(&rows).Error; err != nil {
		fmt.Println("查询失败:", err)
		os.Exit(1)
	}
	fmt.Printf("crawler_accounts 共 %d 条：\n", len(rows))
	for _, r := range rows {
		cookieDesc := "密文(" + fmt.Sprint(len(r.CookieEncrypted)) + "字符)"
		if r.CookieEncrypted == placeholder {
			cookieDesc = "★占位符垃圾★"
		} else if len(r.CookieEncrypted) < 200 {
			cookieDesc = "可疑短值(" + fmt.Sprint(len(r.CookieEncrypted)) + "字符): " + r.CookieEncrypted
		}
		fmt.Printf("  #%d %s %s status=%s health=%s 用量%d/%d cookie=%s\n",
			r.ID, r.Platform, r.AccountName, r.Status, r.HealthCheckResult,
			r.DailyUsageCount, r.DailyUsageLimit, cookieDesc)
	}

	if !*cleanup {
		fmt.Println("\n（预览模式——加 -cleanup 删除占位符记录）")
		return
	}
	res := db.Table("crawler_accounts").Where("cookie_encrypted = ?", placeholder).Delete(nil)
	if res.Error != nil {
		fmt.Println("删除失败:", res.Error)
		os.Exit(1)
	}
	fmt.Printf("\n已删除 %d 条占位符记录\n", res.RowsAffected)
}
