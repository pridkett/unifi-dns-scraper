package scraper

import (
	"os"
	"testing"
)

func TestUpdateConfigFromEnv(t *testing.T) {
	// Define test cases
	tests := []struct {
		name           string
		initialConfig  TomlConfig
		envVars        map[string]string
		expectedConfig TomlConfig
	}{
		{
			name: "no environment variables",
			initialConfig: TomlConfig{
				Unifi: struct {
					Host     string
					User     string
					Password string
				}{
					Host:     "https://unifi.example.com",
					User:     "admin",
					Password: "password",
				},
			},
			envVars: map[string]string{},
			expectedConfig: TomlConfig{
				Unifi: struct {
					Host     string
					User     string
					Password string
				}{
					Host:     "https://unifi.example.com",
					User:     "admin",
					Password: "password",
				},
			},
		},
		{
			name: "override user only",
			initialConfig: TomlConfig{
				Unifi: struct {
					Host     string
					User     string
					Password string
				}{
					Host:     "https://unifi.example.com",
					User:     "admin",
					Password: "password",
				},
			},
			envVars: map[string]string{
				"SCRAPER_UNIFI_USER": "env_admin",
			},
			expectedConfig: TomlConfig{
				Unifi: struct {
					Host     string
					User     string
					Password string
				}{
					Host:     "https://unifi.example.com",
					User:     "env_admin",
					Password: "password",
				},
			},
		},
		{
			name: "override host only",
			initialConfig: TomlConfig{
				Unifi: struct {
					Host     string
					User     string
					Password string
				}{
					Host:     "https://unifi.example.com",
					User:     "admin",
					Password: "password",
				},
			},
			envVars: map[string]string{
				"SCRAPER_UNIFI_HOST": "https://env.unifi.example.com",
			},
			expectedConfig: TomlConfig{
				Unifi: struct {
					Host     string
					User     string
					Password string
				}{
					Host:     "https://env.unifi.example.com",
					User:     "admin",
					Password: "password",
				},
			},
		},
		{
			name: "override password only",
			initialConfig: TomlConfig{
				Unifi: struct {
					Host     string
					User     string
					Password string
				}{
					Host:     "https://unifi.example.com",
					User:     "admin",
					Password: "password",
				},
			},
			envVars: map[string]string{
				"SCRAPER_UNIFI_PASSWORD": "env_password",
			},
			expectedConfig: TomlConfig{
				Unifi: struct {
					Host     string
					User     string
					Password string
				}{
					Host:     "https://unifi.example.com",
					User:     "admin",
					Password: "env_password",
				},
			},
		},
		{
			name: "override all values",
			initialConfig: TomlConfig{
				Unifi: struct {
					Host     string
					User     string
					Password string
				}{
					Host:     "https://unifi.example.com",
					User:     "admin",
					Password: "password",
				},
			},
			envVars: map[string]string{
				"SCRAPER_UNIFI_USER":     "env_admin",
				"SCRAPER_UNIFI_PASSWORD": "env_password",
				"SCRAPER_UNIFI_HOST":     "https://env.unifi.example.com",
			},
			expectedConfig: TomlConfig{
				Unifi: struct {
					Host     string
					User     string
					Password string
				}{
					Host:     "https://env.unifi.example.com",
					User:     "env_admin",
					Password: "env_password",
				},
			},
		},
		{
			name: "set values only in env, not in config",
			initialConfig: TomlConfig{
				Unifi: struct {
					Host     string
					User     string
					Password string
				}{
					Host:     "",
					User:     "",
					Password: "",
				},
			},
			envVars: map[string]string{
				"SCRAPER_UNIFI_USER":     "env_admin",
				"SCRAPER_UNIFI_PASSWORD": "env_password",
				"SCRAPER_UNIFI_HOST":     "https://env.unifi.example.com",
			},
			expectedConfig: TomlConfig{
				Unifi: struct {
					Host     string
					User     string
					Password string
				}{
					Host:     "https://env.unifi.example.com",
					User:     "env_admin",
					Password: "env_password",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear existing environment variables
			os.Unsetenv("SCRAPER_UNIFI_USER")
			os.Unsetenv("SCRAPER_UNIFI_PASSWORD")
			os.Unsetenv("SCRAPER_UNIFI_HOST")

			// Set up environment variables for the test
			for key, value := range tt.envVars {
				os.Setenv(key, value)
			}

			// Run the function
			config := tt.initialConfig
			UpdateConfigFromEnv(&config)

			// Verify the config was updated correctly
			if config.Unifi.User != tt.expectedConfig.Unifi.User {
				t.Errorf("UpdateConfigFromEnv() User = %v, want %v", config.Unifi.User, tt.expectedConfig.Unifi.User)
			}
			if config.Unifi.Password != tt.expectedConfig.Unifi.Password {
				t.Errorf("UpdateConfigFromEnv() Password = %v, want %v", config.Unifi.Password, tt.expectedConfig.Unifi.Password)
			}
			if config.Unifi.Host != tt.expectedConfig.Unifi.Host {
				t.Errorf("UpdateConfigFromEnv() Host = %v, want %v", config.Unifi.Host, tt.expectedConfig.Unifi.Host)
			}

			// Clean up
			for key := range tt.envVars {
				os.Unsetenv(key)
			}
		})
	}
}
