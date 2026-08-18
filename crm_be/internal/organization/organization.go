// Package organization is the tenant root — every tenant-scoped table
// elsewhere in this database has an organization_id pointing back to a
// row here. See docs/architecture/freeze.md bagian 1.1.
package organization

import (
	"time"

	"github.com/google/uuid"
)

type Organization struct {
	ID        uuid.UUID
	Name      string
	Timezone  string
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
}
