package main

import (
	"database/sql"

	"github.com/tgulacsi/go/safesql"
)

func main() {
	var db *sql.DB
	db.Exec("SELECT * FROM cat")

	s := safesql.FromConstant("SELECT * FROM all_objects")
	db.Exec(s.String())
	db.Exec(safesql.FromConstant("SELECT * FROM all_objects").String())
}
