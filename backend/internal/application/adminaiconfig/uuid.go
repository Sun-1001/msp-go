package adminaiconfig

import "mathstudy/backend/internal/platform/identifier"

func newUUID() (string, error) {
	return identifier.NewUUID()
}
