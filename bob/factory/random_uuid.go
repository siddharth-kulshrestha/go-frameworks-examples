package factory

import (
	"github.com/google/uuid"
	"github.com/jaswdr/faker/v2"
)

func random_uuid_UUID(_ *faker.Faker, _ ...string) uuid.UUID {
	return uuid.New()
}
