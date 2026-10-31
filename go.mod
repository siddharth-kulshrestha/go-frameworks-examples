module github.com/siddharth-kulshrestha/taskforge

go 1.27.0

require (
	github.com/go-chi/chi/v5 v5.3.2
	github.com/google/uuid v1.6.0
	github.com/jackc/pgx/v5 v5.11.0
	github.com/siddharth-kulshrestha/taskforge/bob v0.0.0-unpublished
	github.com/siddharth-kulshrestha/taskforge/sqlc v0.0.0-unpublished
	github.com/stephenafamo/bob v0.50.0
)

require (
	github.com/aarondl/opt v0.0.0-20250607033636-982744e1bd65 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/qdm12/reprint v0.0.0-20200326205758-722754a53494 // indirect
	github.com/stephenafamo/scan v0.9.0 // indirect
	golang.org/x/sync v0.17.0 // indirect
	golang.org/x/text v0.29.0 // indirect
)

replace github.com/siddharth-kulshrestha/taskforge/sqlc => ./sqlc

replace github.com/siddharth-kulshrestha/taskforge/bob => ./bob
