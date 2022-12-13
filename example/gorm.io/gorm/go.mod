module github.com/j2gg0s/otsql/example/gorm.io/gorm

go 1.14

replace github.com/j2gg0s/otsql => ../../../

require (
	github.com/90poe/otsql v1.0.2
	github.com/go-sql-driver/mysql v1.6.0
	github.com/lib/pq v1.10.3
	gorm.io/driver/mysql v1.1.2
	gorm.io/driver/postgres v1.1.1
	gorm.io/gorm v1.21.15
)
