package proxy

import (
	"regexp"

	"github.com/pkg/errors"
)

var (
	roleArnRegex = regexp.MustCompile(`^arn:aws:iam::(\d+):role/([^:]+/)?([^:]+?)$`)
)

// RoleARN holds parsed ARN sections.
type RoleARN struct {
	value     string
	path      string
	name      string
	accountID string
}

// NewRoleARN creates a new instance by parsing a full ARN string.
func NewRoleARN(value string) (RoleARN, error) {
	result := roleArnRegex.FindStringSubmatch(value)

	if result == nil {
		return RoleARN{}, errors.Errorf("invalid role ARN [%s]", value)
	}

	return RoleARN{value, "/" + result[2], result[3], result[1]}, nil
}

// RoleName returns the "friendly" name, the ARN suffix.
func (r RoleARN) RoleName() string {
	return r.name
}

// Path returns the resource path including the trailing RoleName.
func (r RoleARN) Path() string {
	return r.path
}

// AccountID returns the numerical ID.
func (r RoleARN) AccountID() string {
	return r.accountID
}

// String returns the original, unparsed ARN.
func (r RoleARN) String() string {
	return r.value
}

// Empty returns true if the struct is uninitialized.
func (r RoleARN) Empty() bool {
	return len(r.value) == 0
}

// Equals returns true if the other struct represents the same ARN.
func (r RoleARN) Equals(other RoleARN) bool {
	return r.value == other.value
}
