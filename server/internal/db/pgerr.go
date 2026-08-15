package db

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

func pgErrCode(err error) string {
	if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok {
		return pgErr.Code
	}
	return ""
}
