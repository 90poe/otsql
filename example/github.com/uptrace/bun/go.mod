module github.com/j2gg0s/otsql/example/github.com/uptrace/bun

go 1.16

require (
	github.com/go-sql-driver/mysql v1.6.0
	github.com/90poe/otsql v1.1.0
	github.com/uptrace/bun v1.0.8
	github.com/uptrace/bun/dialect/mysqldialect v1.0.8
)

replace github.com/90poe/otsql => ../../../../
