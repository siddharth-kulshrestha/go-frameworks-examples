# Domain Model Design: Retrospective, Anti-Patterns & Best Practices

This document captures the architectural review, anti-patterns identified in early iterations of `organization.go`, the rationale behind Go-idiomatic patterns, and how to handle struct serialization in Go.

---

## Table of Contents

- [1. Retrospective: The Initial Approach & Mistakes](#1-retrospective-the-initial-approach--mistakes)
  - [Mistake 1: Unexported Fields with Default JSON Tags](#mistake-1-unexported-fields-with-default-json-tags)
  - [Mistake 2: The Active Record Anti-Pattern (`.Save()` in Domain)](#mistake-2-the-active-record-anti-pattern-save-in-domain)
  - [Mistake 3: Java-Style Builder Over-Engineering](#mistake-3-java-style-builder-over-engineering)
  - [Mistake 4: Missing Invariant Validation](#mistake-4-missing-invariant-validation)
  - [Mistake 5: Redundant `BaseModel` Struct](#mistake-5-redundant-basemodel-struct)
- [2. Deep Dive: Can You `json.Marshal()` Structs with Private Members?](#2-deep-dive-can-you-jsonmarshal-structs-with-private-members)
  - [Why Standard Reflection Ignores Unexported Fields](#why-standard-reflection-ignores-unexported-fields)
  - [The Solution: Implementing `json.Marshaler`](#the-solution-implementing-jsonmarshaler)
  - [Deserializing with `json.Unmarshaler`](#deserializing-with-jsonunmarshaler)
  - [When to Use Private Fields vs. Exported Fields](#when-to-use-private-fields-vs-exported-fields)
- [3. The Final Recommended Design](#3-the-final-recommended-design)
  - [Domain Model Code](#domain-model-code)
  - [Why This Design Fits TaskForge](#why-this-design-fits-taskforge)
- [4. Architectural Flow in Practice](#4-architectural-flow-in-practice)
- [5. Quick Rule-of-Thumb Cheatsheet for Go Domain Models](#5-quick-rule-of-thumb-cheatsheet-for-go-domain-models)

---

## 1. Retrospective: The Initial Approach & Mistakes

The initial draft of [`organization.go`](./organization.go) applied classic Object-Oriented Programming (OOP) patterns from Java / C# / Python (Active Record, Builder interfaces, private fields with getters). While the intent was encapsulation and fluent creation, it conflicted with Go's language design and clean architecture boundaries.

```go
// --- Initial Draft (Anti-Patterns) ---

type BaseModel struct {}

type Organization struct {
    id        uuid.UUID `json:"id"`
    name      string    `json:"name"`
    slug      string    `json:"slug"`
    createdAt time.Time `json:"created_at"`
}

func (o *Organization) ID() uuid.UUID       { return o.id }
func (o *Organization) Name() string        { return o.name }
func (o *Organization) Slug() string        { return o.slug }
func (o *Organization) CreatedAt() time.Time { return o.createdAt }

type organizationBuilder interface {
    Name(name string) organizationBuilder
    Slug(slug string) organizationBuilder
    CreatedAt(createdAt time.Time) organizationBuilder
    ID(id uuid.UUID) organizationBuilder
    Build() *Organization
    Save() *Organization
}
```

### Mistake 1: Unexported Fields with Default JSON Tags

```go
type Organization struct {
    id   uuid.UUID `json:"id"`   // Unexported!
    name string    `json:"name"` // Unexported!
}
```

- **The Problem**: In Go, the `encoding/json` package uses runtime reflection (`reflect`). By design, Go's reflection package **ignores all unexported (lowercase) struct fields**.
- **The Symptom**: Calling `json.Marshal(org)` on this struct does not produce an error; it silently returns an empty JSON object: `{}`.
- **Rule**: Struct tags like `json:"..."`, `db:"..."`, or `yaml:"..."` on unexported fields are completely ignored by standard libraries.

---

### Mistake 2: The Active Record Anti-Pattern (`.Save()` in Domain)

```go
func (orgb *organizationBuilderImpl) Save() *Organization {
    // save the organization...
    return orgb.org
}
```

- **Violation of Separation of Concerns (SoC)**: Domain models represent business entities and business rules. They should have **zero knowledge of databases, SQL, or persistence infrastructure**.
- **Missing Context & Error Propagation**:
  - Real database I/O is fallible and cancellable. It requires `ctx context.Context` (for timeouts and tracing) and returns an `error` (e.g. database down, unique slug collision).
  - The signature `Save() *Organization` has no `ctx` and cannot report errors to the caller.
- **Breaks Multi-Framework Portability**: TaskForge's core goal is comparing **sqlc**, **Bun**, and **Ent**. If the domain builder has `.Save()`, which database library is it calling? Hardcoding persistence inside the domain couples the entire application to one specific ORM/driver.
- **Clean Architecture Rule**: Persistence belongs strictly to the **Repository Layer** (`OrganizationRepository.Create(ctx, org)`), never on the domain entity or domain builder.

---

### Mistake 3: Java-Style Builder Over-Engineering

The initial code introduced:
1. `type organizationBuilder interface`
2. `type organizationBuilderImpl struct`
3. `func NewOrganizationBuilder() organizationBuilder`
4. 4 setter methods + 4 getter methods

For a struct with just 4 fields (`ID`, `Name`, `Slug`, `CreatedAt`), this generated over **75 lines of boilerplate**.

- **Interfaces in Go**: Go interfaces belong where behavior is **consumed** (to enable mocking or polymorphism), not where types are defined. Defining an interface solely to hide a builder implementation is unidiomatic and provides no benefit.
- **Builders in Go**: Builders are only justified when an object has 8–10+ optional parameters. For standard entities, simple constructors or functional options are far more idiomatic.

---

### Mistake 4: Missing Invariant Validation

The core purpose of Domain-Driven Design (DDD) is to **guarantee valid state** at all times:
- With the builder pattern as implemented, calling `NewOrganizationBuilder().Build()` produced an empty `Organization` with empty strings for `name` and `slug`.
- A domain model should guarantee that an entity cannot exist in an invalid state.

---

### Mistake 5: Redundant `BaseModel` Struct

```go
type BaseModel struct {}
```

- In class-based languages (e.g., Python Django, Ruby on Rails, Java Hibernate), models inherit from a common base class.
- Go uses **composition**, not inheritance. An empty struct embedding adds cognitive noise and conveys no architectural value.

---

## 2. Deep Dive: Can You `json.Marshal()` Structs with Private Members?

### Why Standard Reflection Ignores Unexported Fields

By default, **no**: `json.Marshal` will skip unexported fields because reflection cannot access unexported fields across package boundaries for serialization without violating Go's safety invariants.

### The Solution: Implementing `json.Marshaler`

Yes, you **can** marshal private fields by implementing the standard library's [`json.Marshaler`](https://pkg.go.dev/encoding/json#Marshaler) interface on your struct:

```go
type Marshaler interface {
    MarshalJSON() ([]byte, error)
}
```

#### Technique: Using an Internal Shadow Struct

You keep your struct fields private, but define a local, exported shadow struct inside `MarshalJSON()`:

```go
package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type Organization struct {
	id        uuid.UUID
	name      string
	slug      string
	createdAt time.Time
}

// MarshalJSON implements json.Marshaler for Organization.
func (o Organization) MarshalJSON() ([]byte, error) {
	// Shadow struct with exported fields and JSON tags
	type Alias Organization // Prevents infinite recursion if embedding
	return json.Marshal(&struct {
		ID        uuid.UUID `json:"id"`
		Name      string    `json:"name"`
		Slug      string    `json:"slug"`
		CreatedAt time.Time `json:"created_at"`
	}{
		ID:        o.id,
		Name:      o.name,
		Slug:      o.slug,
		CreatedAt: o.createdAt,
	})
}
```

Now, calling `json.Marshal(org)` will output:
```json
{"id":"524111af-d6c5-455f-862d-bdc52e7f2ca9","name":"SidOrg","slug":"sidorg-slug","created_at":"2026-10-03T17:28:40.46909Z"}
```

---

### Deserializing with `json.Unmarshaler`

Similarly, if you need to parse JSON back into private fields while enforcing business rules:

```go
// UnmarshalJSON implements json.Unmarshaler for Organization.
func (o *Organization) UnmarshalJSON(data []byte) error {
	var aux struct {
		ID        uuid.UUID `json:"id"`
		Name      string    `json:"name"`
		Slug      string    `json:"slug"`
		CreatedAt time.Time `json:"created_at"`
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	// Validate business invariants during deserialization
	if aux.Name == "" {
		return ErrInvalidOrgName
	}
	if aux.Slug == "" {
		return ErrInvalidOrgSlug
	}

	o.id = aux.ID
	o.name = aux.Name
	o.slug = aux.Slug
	o.createdAt = aux.CreatedAt
	return nil
}
```

---

### When to Use Private Fields vs. Exported Fields

| Consideration | Exported Fields (Recommended for Entities) | Private Fields + `MarshalJSON` |
|---|---|---|
| **Boilerplate** | Minimal (Zero extra code) | High (Requires custom `MarshalJSON`, `UnmarshalJSON`, and getters) |
| **ORM / Database Mapping** | Trivial (SQLC, Bun, Ent map directly) | Requires manual mapping code or custom scanners |
| **JSON Serialization** | Automatic via `json:"..."` tags | Requires custom shadow struct mapping |
| **Encapsulation** | Fields are readable/writable within package | Strict field read-only protection outside package |
| **Best Used For** | Domain Entities, DTOs, Aggregate Roots | Value Objects (e.g. `Money`, `EmailAddress`), Secure Tokens |
> [!TIP]
> In Go, **Domain Entities** almost always use **exported fields** combined with a **constructor function (`NewOrganization`)**. This gives you invariant protection upon creation, while allowing ORMs, serializers, and database scanners to operate without reflection hacks.

---

## 3. The Final Recommended Design

Here is the clean, idiomatic Go domain model implemented in [`organization.go`](./organization.go):

```go
package models

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	// ErrInvalidOrgName indicates an empty or invalid organization name.
	ErrInvalidOrgName = errors.New("organization name cannot be empty")

	// ErrInvalidOrgSlug indicates an empty or invalid organization slug.
	ErrInvalidOrgSlug = errors.New("organization slug cannot be empty")
)

// Organization represents the root multi-tenant account entity.
type Organization struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Slug      string    `json:"slug"`
	CreatedAt time.Time `json:"created_at"`
}

// NewOrganization is the factory function enforcing domain invariants.
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
```

### Why This Design Fits TaskForge

1. **Guaranteed Invariants**: An `Organization` cannot be created in an invalid state using `NewOrganization`.
2. **Persistence Agnostic**: Zero database code or ORM dependencies in the domain.
3. **Database Reconstitution Friendly**: When `sqlc`, `Bun`, or `Ent` loads a row from PostgreSQL, the repository adapter can easily instantiate `&models.Organization{ID: row.ID, Name: row.Name, ...}` directly.
4. **Native Serialization**: Fully compatible with `json.Marshal`, HTTP handlers, logging, and gRPC.
5. **No Clutter**: Reduced from 90+ lines of builder ceremony to ~40 clean lines.

---

## 4. Architectural Flow in Practice

In TaskForge, the domain entity flows seamlessly from API to Persistence without violating boundaries:

```text
1. Transport Layer (HTTP / API DTO)
   └── Validates JSON wire format (e.g. CreateOrganizationRequest)
         │
         ▼
2. Domain Factory
   └── models.NewOrganization(req.Name, req.Slug)
       ├── Validates business invariants (non-empty, slug rules)
       └── Generates identity (uuid.New()) & timestamp
         │
         ▼
3. Service Layer (Use Case Workflow)
   └── OrganizationService.CreateOrganization(ctx, name, slug)
       ├── Checks cross-cutting business rules (e.g. slug uniqueness)
       └── Calls repository interface
         │
         ▼
4. Persistence Boundary (Repository Adapter)
   └── orgRepo.Create(ctx, org)
       ├── sqlc: queries.CreateOrganization(ctx, params)
       ├── Bun:  db.NewInsert().Model(org).Exec(ctx)
       └── Ent:  client.Organization.Create().Save(ctx)
```

In [`main.go`](../../main.go):

```go
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
```

---

## 5. Quick Rule-of-Thumb Cheatsheet for Go Domain Models

1. **Entities vs. Active Record**: Never put `.Save()`, `.Update()`, or `.Delete()` on domain structs. Use Repository interfaces for I/O.
2. **Constructors Protect Invariants**: Use `New<Entity>(...) (*Entity, error)` as the single gatekeeper for new entity creation.
3. **Export Fields for Entities**: Entities that need to be saved to DBs or serialized to JSON should have exported fields with proper struct tags.
4. **Value Objects Use Private Fields**: If an object is an immutable value object (e.g., `Email`, `Currency`), make fields private and implement `json.Marshaler` if needed.
5. **Keep Interfaces on the Consumer Side**: Don't define interfaces for builders or domain models unless you have multiple consumer-driven implementations.
