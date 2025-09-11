package scraper

import (
	"github.com/withmandala/go-log"
	"gorm.io/gorm"
)

// App holds application-wide dependencies and configuration
// This avoids passing the logger to every function while still
// avoiding global variables
type App struct {
	Logger *log.Logger
	DB     *gorm.DB
	Config *TomlConfig
}

// NewApp creates a new application context with common dependencies
func NewApp(logger *log.Logger, config *TomlConfig) *App {
	return &App{
		Logger: logger,
		Config: config,
	}
}

// SetDB sets the database connection for the app
func (a *App) SetDB(db *gorm.DB) {
	a.DB = db
}
