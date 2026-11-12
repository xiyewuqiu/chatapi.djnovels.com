package dbs

import (
	"fmt"
	"im-server/commons/dbcommons"
	"time"
)

type AppStatus int
type AppType int

var (
	AppStatus_Normal AppStatus = 0
	AppStatus_Block  AppStatus = 1
	AppStatus_Expire AppStatus = 2

	AppType_Private AppType = 0
	AppType_Alone   AppType = 1
	AppType_Public  AppType = 2
)

type AppInfoDao struct {
	ID           int64     `gorm:"primary_key"`
	AppName      string    `gorm:"app_name"`
	AppKey       string    `gorm:"app_key"`
	AppSecret    string    `gorm:"app_secret"`
	AppSecureKey string    `gorm:"app_secure_key"`
	AppStatus    int       `gorm:"app_status"`
	AppType      int       `gorm:"app_type"`
	CreatedTime  time.Time `gorm:"created_time"`
	UpdatedTime  time.Time `gorm:"updated_time"`
}

func (app AppInfoDao) TableName() string {
	return "apps"
}

func (app AppInfoDao) Create(item AppInfoDao) error {
	db := dbcommons.GetDb()
	if db == nil {
		return fmt.Errorf("database not initialized")
	}
	err := db.Create(&item).Error
	return err
}

func (app AppInfoDao) Upsert(item AppInfoDao) error {
	db := dbcommons.GetDb()
	if db == nil {
		return fmt.Errorf("database not initialized")
	}
	sql := fmt.Sprintf("INSERT INTO %s (app_name,app_key,app_secret,app_secure_key,app_type,created_time)VALUES(?,?,?,?,?,?) ON DUPLICATE KEY UPDATE app_secret=?", app.TableName())
	return db.Exec(sql, item.AppName, item.AppKey, item.AppSecret, item.AppSecureKey, item.AppType, item.CreatedTime, item.AppSecret).Error
}

func (app AppInfoDao) FindByAppkey(appkey string) *AppInfoDao {
	db := dbcommons.GetDb()
	if db == nil {
		return nil
	}
	var appItem AppInfoDao
	err := db.Where("app_key=?", appkey).Take(&appItem).Error
	if err != nil {
		return nil
	}
	return &appItem
}

func (app AppInfoDao) FindById(id int64) *AppInfoDao {
	db := dbcommons.GetDb()
	if db == nil {
		return nil
	}
	var appItem AppInfoDao
	err := db.Where("id=?", id).Take(&appItem).Error
	if err != nil {
		return nil
	}
	return &appItem
}

func (app AppInfoDao) QryApps(limit int64, offset int64) ([]*AppInfoDao, error) {
	db := dbcommons.GetDb()
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	var list []*AppInfoDao
	err := db.Where("id < ?", offset).Order("id desc").Limit(int(limit)).Find(&list).Error
	return list, err
}
