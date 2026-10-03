package models

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Mistake -- Too much over engineering

// type BaseModel struct {
// }

// type Organization struct {
// 	id        uuid.UUID `json:"id"`
// 	name      string    `json:"name"`
// 	slug      string    `json:"slug"`
// 	createdAt time.Time `json:"created_at"`
// }

// func (o *Organization) ID() uuid.UUID {
// 	return o.id
// }

// func (o *Organization) Name() string {
// 	return o.name
// }

// func (o *Organization) Slug() string {
// 	return o.slug
// }

// func (o *Organization) CreatedAt() time.Time {
// 	return o.createdAt
// }

// type organizationBuilder interface {
// 	Name(name string) organizationBuilder
// 	Slug(slug string) organizationBuilder
// 	CreatedAt(createdAt time.Time) organizationBuilder
// 	ID(id uuid.UUID) organizationBuilder
// 	Build() *Organization
// 	Save() *Organization
// }

// type organizationBuilderImpl struct {
// 	org *Organization
// }

// func NewOrganizationBuilder() organizationBuilder {
// 	return &organizationBuilderImpl{org: &Organization{}}
// }

// func (orgb *organizationBuilderImpl) Name(name string) organizationBuilder {
// 	orgb.org.name = name
// 	return orgb
// }

// func (orgb *organizationBuilderImpl) Slug(slug string) organizationBuilder {
// 	orgb.org.slug = slug
// 	return orgb
// }

// func (orgb *organizationBuilderImpl) CreatedAt(createdAt time.Time) organizationBuilder {
// 	orgb.org.createdAt = createdAt
// 	return orgb
// }

// func (orgb *organizationBuilderImpl) ID(id uuid.UUID) organizationBuilder {
// 	orgb.org.id = id
// 	return orgb
// }

// func (orgb *organizationBuilderImpl) Build() *Organization {
// 	return orgb.org
// }

// func (orgb *organizationBuilderImpl) Save() *Organization {
// 	// save the organization
// 	// if id is not there, generate it
// 	if orgb.org.id == uuid.Nil {
// 		orgb.org.id = uuid.New()
// 	}

// 	// if created at is not there, set it to current time
// 	if orgb.org.createdAt.IsZero() {
// 		orgb.org.createdAt = time.Now()
// 	}

// 	// save the organization

// 	return orgb.org
// }

var (
	ErrInvalidOrgName = errors.New("organization name cannot be empty")
	ErrInvalidOrgSlug = errors.New("organization slug cannot be empty")
)

// Organization is the multi-tenant root account entity.
type Organization struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Slug      string    `json:"slug"`
	CreatedAt time.Time `json:"created_at"`
}

// NewOrganization creates a new valid Organization (Factory / Invariant Protector).
func NewOrganization(name, slug string) (*Organization, error) {
	name = strings.TrimSpace(name)
	slug = strings.TrimSpace(slug)
	if name == "" {
		return nil, ErrInvalidOrgName
	}
	if slug == "" {
		return nil, ErrInvalidOrgSlug
	}
	return &Organization{
		ID:        uuid.New(),
		Name:      name,
		Slug:      slug,
		CreatedAt: time.Now().UTC(),
	}, nil
}
