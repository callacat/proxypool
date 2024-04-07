package database

import (
	"os"

	"github.com/timerzz/proxypool/log"

	"github.com/timerzz/proxypool/config"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

func connect() (err error) {
	// localhost url
	dsn := "proxypool.db?_pragma=busy_timeout(5000)"
	if config.Config.DatabaseType == "pgsql" {
		dsn = "user=proxypool password=proxypool dbname=proxypool port=5432 sslmode=disable TimeZone=Asia/Shanghai"
	}
	if url := config.Config.DatabaseUrl; url != "" {
		dsn = url
	}
	if url := os.Getenv("DATABASE_URL"); url != "" {
		dsn = url
	}
	var dialector gorm.Dialector
	if config.Config.DatabaseType == "pgsql" {
		dialector = postgres.Open(dsn)
	} else {
		dialector = sqlite.Open(dsn)
	}
	DB, err = gorm.Open(dialector, &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err == nil {
		log.Infoln("database: successfully connected to: %s", DB.Name())
	} else {
		DB = nil
		log.Warnln("database connection info: %s \n\t\tUse cache to store proxies", err.Error())
	}
	return
}
