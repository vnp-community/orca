package mysql

import (
	"errors"
	"strings"

	driver "github.com/go-sql-driver/mysql"
)

// isDuplicateKey reports a 1062 on the named key (MySQL 8 prints it as table.key); an empty key matches any.
func isDuplicateKey(err error, key string) bool {
	var myErr *driver.MySQLError
	return errors.As(err, &myErr) && myErr.Number == 1062 && (key == "" || strings.Contains(myErr.Message, key))
}
