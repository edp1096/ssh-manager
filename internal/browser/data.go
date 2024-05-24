package browser

import (
	"database/sql"
	"log"

	_ "modernc.org/sqlite"
)

func EditBrowserDataLogins(url string) {
	dbPath := "./browser_data/Default/Login Data"

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	_, err = db.Exec("UPDATE logins SET origin_url = ?, signon_realm = ? WHERE id = (SELECT id FROM logins LIMIT 1)", url, url, 3)
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec("UPDATE stats SET origin_domain = ? WHERE update_time = (SELECT update_time FROM stats LIMIT 1)", url)
	if err != nil {
		log.Fatal(err)
	}
}
