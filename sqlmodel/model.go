package sqlmodel

import (
	"database/sql"
	"net"
	"time"
)

type Domain struct {
	ID             uint           `gorm:"primary_key"`
	Name           string         `gorm:"type:varchar(255);not null"`
	Master         sql.NullString `gorm:"type:varchar(128)"`
	LastCheck      sql.NullInt64
	Type           string `gorm:"type:varchar(6);not null"`
	NotifiedSerial sql.NullInt64
	Account        sql.NullString `gorm:"type:varchar(40)"`
}

type Record struct {
	ID        uint `gorm:"primary_key"`
	DomainId  sql.NullInt64
	Name      string `gorm:"type:varchar(255)"`
	Type      string `gorm:"type:varchar(10)"`
	Content   string `gorm:"type:text"`
	Ttl       uint32
	Prio      int
	ChangDate int
	Disabled  bool
}

type UnifiHost struct {
	ID        uint      `gorm:"primary_key"`
	Name      string    `gorm:"type:varchar(255);not null"`
	IP        net.IP    `gorm:"type:varchar(15);not null"`
	LastSeen  time.Time `gorm:"type:timestamp"`
	FirstSeen time.Time `gorm:"type:timestamp"`
}
