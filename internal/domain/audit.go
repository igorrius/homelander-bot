package domain

import (
	"context"
	"strconv"
	"time"
)

// AuditEntry records one security-relevant action.
type AuditEntry struct {
	ID          int64
	At          time.Time
	ActorUserID *int64
	Action      string
	Target      string
	Details     map[string]any
	IP          string
}

// AuditFilter narrows an audit listing; TargetUserID selects "user:<id>".
type AuditFilter struct {
	TargetUserID *int64
	Limit        int
	Offset       int
}

// AuditRepo stores audit entries; they are never changed or deleted.
type AuditRepo interface {
	Add(ctx context.Context, e AuditEntry) error
	List(ctx context.Context, f AuditFilter) ([]AuditEntry, int, error)
}

// Audit action names.
const (
	ActUserApplied        = "user.applied"
	ActUserApproved       = "user.approved"
	ActUserRejected       = "user.rejected"
	ActUserCreated        = "user.created"
	ActUserUpdated        = "user.updated"
	ActUserDisabled       = "user.disabled"
	ActUserEnabled        = "user.enabled"
	ActUserAdminGranted   = "user.admin_granted"
	ActUserAdminRevoked   = "user.admin_revoked"
	ActUserResidentSet    = "user.resident_set"
	ActUserResidentUnset  = "user.resident_unset"
	ActChannelLinked      = "channel.linked"
	ActChannelUnlinked    = "channel.unlinked"
	ActAuthCodeSent       = "auth.code_sent"
	ActAuthLogin          = "auth.login"
	ActAuthLoginFailed    = "auth.login_failed"
	ActAuthLogout         = "auth.logout"
	ActHouseCreated       = "house.created"
	ActHouseUpdated       = "house.updated"
	ActStatusCreated      = "status.created"
	ActStatusUpdated      = "status.updated"
	ActSiteUpdated        = "site.updated"
	ActSitePictureSet     = "site.picture_set"
	ActSitePictureRemoved = "site.picture_removed"
	ActBootstrapAdmin     = "bootstrap.admin_created"
)

// UserTarget, HouseTarget and StatusTarget format audit targets.
func UserTarget(id int64) string   { return "user:" + strconv.FormatInt(id, 10) }
func HouseTarget(id int64) string  { return "house:" + strconv.FormatInt(id, 10) }
func StatusTarget(id int64) string { return "status:" + strconv.FormatInt(id, 10) }

// SiteTarget is the audit target of site settings.
const SiteTarget = "site"
