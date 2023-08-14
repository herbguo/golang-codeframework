/**
 * @Author Herb
 * @Date 2023/8/14 10:08
 **/

package db

import (
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"github.com/jinzhu/gorm"
)

var DB *gorm.DB

type DBConfig struct {
	DriverName string `json:"driver_name"`
	Host       string `json:"host"`
	Port       int    `json:"port"`
	DataBase   string `json:"database"`
	UserName   string `json:"user"`
	Password   string `json:"passwd"`
	Charset    string `json:"charset"`
}

func (dbConfig *DBConfig) InitDB() {

	args := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=true",
		dbConfig.UserName,
		dbConfig.Password,
		dbConfig.Host,
		dbConfig.Port,
		dbConfig.DataBase,
		dbConfig.Charset,
	)

	db, err := gorm.Open(dbConfig.DriverName, args)
	if err != nil {
		panic("failed to connect database,err:" + err.Error())
	}
	DB = db
}
