package main

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDotenvPreservesDatabaseCredentials(t *testing.T) {
	for _, tc := range []struct{ name, dotenv, password string }{
		{"hash", "DEV_ENV_PASSWORD=demo#secret\n", "demo#secret"},
		{"quoted dollar", "DEV_ENV_PASSWORD='demo$secret'\n", "demo$secret"},
		{"quote", "DEV_ENV_PASSWORD=\"demo\\\"secret\"\n", "demo\"secret"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			clearEnv(t, "DEV_ENV_PASSWORD")
			clearEnv(t, "DEV_ENV_DATABASE_URL")
			u := &url.URL{Scheme: "postgres", User: url.UserPassword("test", tc.password), Host: "127.0.0.1:5432", Path: "test"}
			path := filepath.Join(t.TempDir(), ".env")
			if err := os.WriteFile(path, []byte(tc.dotenv+"DEV_ENV_DATABASE_URL="+u.String()+"\n"), 0600); err != nil {
				t.Fatal(err)
			}
			if err := loadEnv(path); err != nil {
				t.Fatal(err)
			}
			actualURL, err := url.Parse(os.Getenv("DEV_ENV_DATABASE_URL"))
			if err != nil {
				t.Fatal("invalid database URL")
			}
			password, _ := actualURL.User.Password()
			if os.Getenv("DEV_ENV_PASSWORD") != tc.password || password != tc.password {
				t.Fatal("dotenv changed a password or diverged from the database URL")
			}
		})
	}
}

func TestDotenvPreservesEnvironmentOverride(t *testing.T) {
	t.Setenv("DEV_ENV_PASSWORD", "override")
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte("DEV_ENV_PASSWORD=from-file\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := loadEnv(path); err != nil {
		t.Fatal(err)
	}
	if os.Getenv("DEV_ENV_PASSWORD") != "override" {
		t.Fatal("dotenv replaced an explicit environment value")
	}
}

func TestDotenvErrorsDoNotExposeSource(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte("DEV_ENV_PASSWORD='private-value\n"), 0600); err != nil {
		t.Fatal(err)
	}
	err := loadEnv(path)
	if err == nil || strings.Contains(err.Error(), "private-value") {
		t.Fatal("invalid dotenv was accepted or exposed a secret")
	}
}

func clearEnv(t *testing.T, key string) {
	t.Helper()
	t.Setenv(key, "")
	if err := os.Unsetenv(key); err != nil {
		t.Fatal(err)
	}
}
