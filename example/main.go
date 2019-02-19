package main

import (
	"database/sql"
	"fmt"

	account "github.com/YiCodes/gosql/example/account/gen"
)

func main() {
	db, err := sql.Open("driver", "dataSourceName")

	if err != nil {
		panic(err)
	}
	defer db.Close()

	user, err := account.GetUser(db, "123")

	if err != nil {
		panic(err)
	}

	fmt.Println(user.UserName)

	user = new(account.User)
	user.UserID = "222"
	user.UserName = "peter"

	_, err = account.InsertUser(db, user)
}
