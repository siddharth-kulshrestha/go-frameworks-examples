package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/siddharth-kulshrestha/taskforge/domain/models"
	"github.com/siddharth-kulshrestha/taskforge/sqlc/internal/store/sqlc"
)

type OrganizationRepositoryImpl struct {
	queries *sqlc.Queries
}

func NewOrganizationRepositoryImpl(db sqlc.DBTX) *OrganizationRepositoryImpl {
	queries := sqlc.New(db)
	return &OrganizationRepositoryImpl{
		queries: queries,
	}
}

func (r *OrganizationRepositoryImpl) Create(ctx context.Context, org *models.Organization) (*models.Organization, error) {
	cop := sqlc.CreateOrganizationParams{
		ID:   pgtype.UUID{Bytes: org.ID, Valid: true},
		Name: org.Name,
		Slug: org.Slug,
	}

	orgModel, err := r.queries.CreateOrganization(ctx, cop)
	if err != nil {
		return nil, err
	}

	return &models.Organization{
		ID:        orgModel.ID.Bytes,
		Name:      orgModel.Name,
		Slug:      orgModel.Slug,
		CreatedAt: orgModel.CreatedAt.Time,
	}, nil
}

func (r *OrganizationRepositoryImpl) GetByID(ctx context.Context, id uuid.UUID) (*models.Organization, error) {
	orgModel, err := r.queries.GetOrganization(ctx, pgtype.UUID{Bytes: id, Valid: true})
	if err != nil {
		return nil, err
	}

	return &models.Organization{
		ID:        orgModel.ID.Bytes,
		Name:      orgModel.Name,
		Slug:      orgModel.Slug,
		CreatedAt: orgModel.CreatedAt.Time,
	}, nil
}
