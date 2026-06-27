package main

import (
	"fmt"
	"os"

	"dev.helix.code/internal/database"
)

func main() {
	cfg := database.Config{
		Host:     "127.0.0.1",
		Port:     55432,
		User:     "helixcode",
		Password: "helixcode_test_password",
		DBName:   "helixcode_test",
		SSLMode:  "disable",
	}
	db, err := database.New(cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, "connect failed:", err)
		os.Exit(1)
	}
	if err := db.InitializeSchema(); err != nil {
		fmt.Fprintln(os.Stderr, "schema init failed:", err)
		os.Exit(2)
	}
	fmt.Println("SCHEMA_OK")
}
