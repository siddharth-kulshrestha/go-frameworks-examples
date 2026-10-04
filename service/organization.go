package service

import (
	"context"

	"github.com/google/uuid"
	"github.com/siddharth-kulshrestha/taskforge/domain/models"
)

// OrganizationRepository interface to be implemented by different frameworks
type OrganizationRepository interface {
	Create(ctx context.Context, org *models.Organization) (*models.Organization, error)
	GetByID(ctx context.Context, id uuid.UUID) (*models.Organization, error)
}

type OrganizationService struct {
	repo OrganizationRepository
}

func NewOrganizationService(repo OrganizationRepository) *OrganizationService {
	return &OrganizationService{repo: repo}
}

func (s *OrganizationService) CreateOrganizationWithName(ctx context.Context, name string) (*models.Organization, error) {
	org, err := models.NewOrganization(name, name+"-slug")
	if err != nil {
		return nil, err
	}
	return s.repo.Create(ctx, org)
}

func (s *OrganizationService) GetOrganizationByID(ctx context.Context, id uuid.UUID) (*models.Organization, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *OrganizationService) CreateOrganization(ctx context.Context, org *models.Organization) (*models.Organization, error) {
	return s.repo.Create(ctx, org)
}
