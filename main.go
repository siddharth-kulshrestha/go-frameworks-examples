package main

import (
	"fmt"

	"github.com/siddharth-kulshrestha/taskforge/domain/models"
)

func main() {
	fmt.Println("starting the execution of Task forge....")
	name := "SidOrg"
	org, err := models.NewOrganization(name, name+"-slug")
	if err != nil {
		fmt.Println("Error creating organization:", err)
		return
	}
	fmt.Println(org)
}
