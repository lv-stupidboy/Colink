package main

import (
	"database/sql"
	"fmt"
	"os"

	_ "modernc.org/sqlite"
)

func main() {
	db, err := sql.Open("sqlite", "data/sqlite/colink.db")
	if err != nil {
		fmt.Println("open db error:", err)
		os.Exit(1)
	}
	defer db.Close()

	rows, err := db.Query(`SELECT id, thread_id, status, role_name, created_at, updated_at FROM agent_invocations ORDER BY updated_at DESC LIMIT 10`)
	if err != nil {
		fmt.Println("query error:", err)
		os.Exit(1)
	}
	defer rows.Close()

	for rows.Next() {
		var id, threadID, status, roleName, createdAt, updatedAt string
		if err := rows.Scan(&id, &threadID, &status, &roleName, &createdAt, &updatedAt); err != nil {
			fmt.Println("scan error:", err)
			continue
		}
		fmt.Printf("ID=%s Thread=%s Status=%s Role=%s Created=%s Updated=%s\n", id[:8], threadID[:8], status, roleName, createdAt, updatedAt)
	}
}
