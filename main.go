package main

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/siddharth-kulshrestha/taskforge/domain/models"
	"github.com/siddharth-kulshrestha/taskforge/service"
	"github.com/siddharth-kulshrestha/taskforge/sqlc/repository"
)

const CONN_STR1 = "postgres://taskforge@localhost:5432/taskforge?sslmode=disable"
const CONN_STR2 = "postgres://taskforge:taskforge@localhost:5432/taskforge?sslmode=disable"

func main() {
	fmt.Println("starting the execution of Task forge....")

	// SQL Connection
	ctx := context.Background()
	connStr := CONN_STR1

	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		fmt.Println("Error creating connection pool:", err)
		return
	}
	defer pool.Close()

	orgRepo := repository.NewOrganizationRepositoryImpl(pool)
	orgService := service.NewOrganizationService(orgRepo)

	// Create domain model and Save
	name := "SidOrg"
	org, err := models.NewOrganization(name, name+"-slug")
	if err != nil {
		fmt.Println("Error creating organization:", err)
		return
	}
	fmt.Println(org)

	orgR, err := orgService.CreateOrganization(ctx, org)
	if err != nil {
		fmt.Println("Error creating organization:", err)
		return
	}
	fmt.Println("Write successful via sqlc....")
	fmt.Println(orgR)

	// Read
	orgId := orgR.ID
	fmt.Println("Reading the ID")
	fmt.Println(orgId.String())

	orgR2, err := orgService.GetOrganizationByID(ctx, orgId)
	if err != nil {
		fmt.Println("Error getting organization:", err)
		return
	}
	fmt.Println("Read successful via sqlc....")
	fmt.Println(orgR2)

}
