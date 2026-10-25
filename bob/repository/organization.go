package repository

import (
	"context"

	"github.com/aarondl/opt/omit"
	"github.com/google/uuid"
	"github.com/siddharth-kulshrestha/taskforge/bob/models"
	dm "github.com/siddharth-kulshrestha/taskforge/domain/models"
	"github.com/stephenafamo/bob"
)

type Organization struct {
	db bob.Executor
}

func NewOrganizationRepository(db bob.Executor) *Organization {
	return &Organization{db: db}
}

func (o *Organization) Create(ctx context.Context, org *dm.Organization) (*dm.Organization, error) {
	setter := &models.OrganizationSetter{
		ID:        omit.From(org.ID),
		Name:      omit.From(org.Name),
		Slug:      omit.From(org.Slug),
		CreatedAt: omit.From(org.CreatedAt),
	}

	row, err := models.Organizations.Insert(setter).One(ctx, o.db)
	if err != nil {
		return nil, err
	}

	// Update the domain model with DB Generated values
	org.ID = row.ID
	org.CreatedAt = row.CreatedAt

	return org, nil

}

func (o *Organization) GetByID(ctx context.Context, id uuid.UUID) (*dm.Organization, error) {
	row, err := models.FindOrganization(ctx, o.db, id)
	if err != nil {
		return nil, err
	}
	return &dm.Organization{
		ID:        row.ID,
		Name:      row.Name,
		Slug:      row.Slug,
		CreatedAt: row.CreatedAt,
	}, nil
}
