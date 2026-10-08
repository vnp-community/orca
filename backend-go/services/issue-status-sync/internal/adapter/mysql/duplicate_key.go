package mysql

import (
	"errors"

	driver "github.com/go-sql-driver/mysql"
)

const mysqlErrDuplicateEntry = 1062

func isDuplicateKey(err error) bool {
	var me *driver.MySQLError
	return errors.As(err, &me) && me.Number == mysqlErrDuplicateEntry
}
