package http

import "github.com/google/uuid"

// parseOptionalUUID parses s as a UUID, returning nil (not an error) when s
// is empty — used for optional folder/parent IDs given as plain form or
// query string values (JSON-bound *uuid.UUID fields don't need this, that
// type already unmarshals "absent" as nil natively).
func parseOptionalUUID(s string) (*uuid.UUID, error) {
	if s == "" {
		return nil, nil
	}
	id, err := uuid.Parse(s)
	if err != nil {
		return nil, err
	}
	return &id, nil
}
