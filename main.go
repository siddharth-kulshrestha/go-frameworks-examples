package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	bobpgx "github.com/stephenafamo/bob/drivers/pgx"

	bobrepository "github.com/siddharth-kulshrestha/taskforge/bob/repository"
	"github.com/siddharth-kulshrestha/taskforge/service"
	sqlcrepository "github.com/siddharth-kulshrestha/taskforge/sqlc/repository"
)

const CONN_STR1 = "postgres://taskforge@localhost:5432/taskforge?sslmode=disable"
const CONN_STR2 = "postgres://taskforge:taskforge@localhost:5432/taskforge?sslmode=disable"

func main() {
	implementation := flag.String("repository", "sqlc", "repository example to run: sqlc or bob")
	flag.Parse()

	ctx := context.Background()
	connStr := CONN_STR1

	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		fmt.Println("Error creating connection pool:", err)
		return
	}
	defer pool.Close()

	var runExample func(context.Context, *pgxpool.Pool) error
	switch *implementation {
	case "sqlc":
		runExample = runSQLCExample
	case "bob":
		runExample = runBobExample
	default:
		fmt.Fprintf(os.Stderr, "unknown repository %q (choose sqlc or bob)\n", *implementation)
		os.Exit(2)
	}

	if err := runExample(ctx, pool); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runSQLCExample(ctx context.Context, pool *pgxpool.Pool) error {
	orgRepo := sqlcrepository.NewOrganizationRepositoryImpl(pool)
	orgService := service.NewOrganizationService(orgRepo)
	return runOrganizationExample(ctx, orgService, "sqlc")
}

func runBobExample(ctx context.Context, pool *pgxpool.Pool) error {
	bobDB := bobpgx.NewPool(pool)
	orgRepo := bobrepository.NewOrganizationRepository(bobDB)
	orgService := service.NewOrganizationService(orgRepo)
	return runOrganizationExample(ctx, orgService, "bob")
}

func runOrganizationExample(ctx context.Context, orgService *service.OrganizationService, implementation string) error {
	orgR, err := orgService.CreateOrganizationWithName(ctx, "SidOrg")
	if err != nil {
		return fmt.Errorf("create organization with %s: %w", implementation, err)
	}
	fmt.Printf("Write successful via %s....\n", implementation)
	fmt.Println(orgR)

	// Read
	orgId := orgR.ID
	fmt.Println("Reading the ID")
	fmt.Println(orgId.String())

	orgR2, err := orgService.GetOrganizationByID(ctx, orgId)
	if err != nil {
		return fmt.Errorf("get organization with %s: %w", implementation, err)
	}
	fmt.Printf("Read successful via %s....\n", implementation)
	fmt.Println(orgR2)
	return nil
}
