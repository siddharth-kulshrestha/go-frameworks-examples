module github.com/siddharth-kulshrestha/taskforge

go 1.25.6

require github.com/google/uuid v1.6.0

require (
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/pgx/v5 v5.11.0
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/siddharth-kulshrestha/taskforge/sqlc v0.0.0-unpublished
	golang.org/x/sync v0.17.0 // indirect
	golang.org/x/text v0.29.0 // indirect
)

replace github.com/siddharth-kulshrestha/taskforge/sqlc => ./sqlc
