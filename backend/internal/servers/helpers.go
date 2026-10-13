package servers

import (
	"errors"
	"net/netip"

	"github.com/jackc/pgconn"
)

func parseInet(v string) (netip.Addr, error) {
	return netip.ParseAddr(v)
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}

var errNotFound = errors.New("not found")