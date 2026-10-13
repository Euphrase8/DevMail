package httpapi

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type listContextKey struct{}
type ListOptions struct {
	Page, PerPage, Total       int
	Query, Status, Sort, Order string
	Applied                    bool
}

func ListMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		options := &ListOptions{Page: 1, PerPage: 100, Query: c.Query("q"), Status: c.Query("status"), Sort: c.Query("sort"), Order: c.DefaultQuery("order", "asc")}
		for key, dest := range map[string]*int{"page": &options.Page, "perPage": &options.PerPage} {
			if raw := c.Query(key); raw != "" {
				value, err := strconv.Atoi(raw)
				if err != nil || value < 1 || value > 1000000 || (key == "perPage" && value > 200) {
					Fail(c, 422, ErrValidation, "Invalid pagination value.")
					return
				}
				*dest = value
			}
		}
		if len(options.Query) > 200 || (options.Order != "asc" && options.Order != "desc") {
			Fail(c, 422, ErrValidation, "Invalid list query.")
			return
		}
		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), listContextKey{}, options))
		c.Next()
	}
}

// PageRows counts and selects the same filtered dataset. All identifiers are trusted
// compile-time inputs; search/filter values always use bound SQL parameters.
func PageRows(ctx context.Context, pool *pgxpool.Pool, columns, table, where, defaultSort string, searchFields []string, sortFields map[string]string, statusField string, args ...any) (pgx.Rows, error) {
	options, _ := ctx.Value(listContextKey{}).(*ListOptions)
	if options == nil {
		return pool.Query(ctx, `SELECT `+columns+` FROM `+table+` WHERE `+where+` ORDER BY `+defaultSort, args...)
	}
	if options.Query != "" && len(searchFields) > 0 {
		args = append(args, "%"+options.Query+"%")
		clauses := []string{}
		for _, field := range searchFields {
			clauses = append(clauses, fmt.Sprintf("%s ILIKE $%d", field, len(args)))
		}
		where += " AND (" + strings.Join(clauses, " OR ") + ")"
	}
	if options.Status != "" && statusField != "" {
		args = append(args, options.Status)
		where += fmt.Sprintf(" AND %s::text=$%d", statusField, len(args))
	}
	sort := defaultSort
	if field, ok := sortFields[options.Sort]; ok {
		sort = field + " " + options.Order + ", " + defaultSort
	}
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM `+table+` WHERE `+where, args...).Scan(&options.Total); err != nil {
		return nil, err
	}
	options.Applied = true
	args = append(args, options.PerPage, (options.Page-1)*options.PerPage)
	return pool.Query(ctx, fmt.Sprintf("SELECT %s FROM %s WHERE %s ORDER BY %s LIMIT $%d OFFSET $%d", columns, table, where, sort, len(args)-1, len(args)), args...)
}
