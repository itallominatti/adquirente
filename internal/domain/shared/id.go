package shared

import (
	"strings"

	"github.com/google/uuid"
)

func NewID(prefix string) string {
	return prefix + "_" + strings.ReplaceAll(uuid.NewString(), "-", "")
}
