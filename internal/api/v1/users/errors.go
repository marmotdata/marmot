package users

import (
	"errors"
	"net/http"

	"github.com/marmotdata/marmot/internal/api/v1/common"
	"github.com/marmotdata/marmot/internal/core/user"
)

// respondUserError maps a core/user error to the status it deserves and reports
// whether it wrote a response. False means the caller could not have avoided the
// error, so it belongs in the log behind a 500.
//
// Matching is by errors.Is, not by value: core/user returns its sentinels
// wrapped in context ("creating user: %w"), so the == comparisons these
// handlers used never matched and every rejected request came back as 500.
func respondUserError(w http.ResponseWriter, err error) bool {
	// First, because it is the only case carrying field detail — and it also
	// satisfies errors.Is(err, ErrInvalidInput).
	var invalid *user.InvalidInputError
	if errors.As(err, &invalid) {
		fields := make([]common.ValidationError, 0, len(invalid.Fields))
		for _, f := range invalid.Fields {
			fields = append(fields, common.ValidationError{Field: f.Field, Message: f.Message})
		}
		common.RespondValidationError(w, "Invalid input", fields)
		return true
	}

	switch {
	case errors.Is(err, user.ErrUserNotFound):
		common.RespondError(w, http.StatusNotFound, "User not found")
	case errors.Is(err, user.ErrInvalidInput):
		// Validation failed with no field detail; the text is internal.
		common.RespondError(w, http.StatusBadRequest, "Invalid input")
	case errors.Is(err, user.ErrPasswordRequired):
		common.RespondError(w, http.StatusBadRequest, "password is required for non-OAuth users")
	case errors.Is(err, user.ErrReservedUsername):
		common.RespondError(w, http.StatusBadRequest, "That username is reserved")
	case errors.Is(err, user.ErrAlreadyExists):
		common.RespondError(w, http.StatusConflict, "User already exists")
	// 409 rather than 400: rules about the state of the system, and nothing the
	// caller can rewrite makes the request succeed.
	case errors.Is(err, user.ErrCannotDeleteSelf):
		common.RespondError(w, http.StatusConflict, "A user cannot delete their own account")
	case errors.Is(err, user.ErrCannotDeleteAdmin):
		common.RespondError(w, http.StatusConflict, "The admin user cannot be deleted")
	case errors.Is(err, user.ErrRoleNotFound):
		common.RespondError(w, http.StatusBadRequest, "Unknown role")
	default:
		return false
	}
	return true
}
