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
	ActUserApplied       = "user.applied"
	ActUserApproved      = "user.approved"
	ActUserRejected      = "user.rejected"
	ActUserCreated       = "user.created"
	ActUserUpdated       = "user.updated"
	ActUserDisabled      = "user.disabled"
	ActUserEnabled       = "user.enabled"
	ActUserAdminGranted  = "user.admin_granted"
	ActUserAdminRevoked  = "user.admin_revoked"
	ActUserMCPGranted    = "user.mcp_granted"
	ActUserMCPRevoked    = "user.mcp_revoked"
	ActUserResidentSet   = "user.resident_set"
	ActUserResidentUnset = "user.resident_unset"
	ActChannelLinked     = "channel.linked"
	ActChannelUnlinked   = "channel.unlinked"
	ActAuthCodeSent      = "auth.code_sent"
	ActAuthLogin         = "auth.login"
	ActAuthLoginFailed   = "auth.login_failed"
	ActAuthLogout        = "auth.logout"
	ActHouseCreated      = "house.created"
	ActHouseUpdated      = "house.updated"
	ActStatusCreated     = "status.created"
	ActStatusUpdated     = "status.updated"
	ActSiteUpdated       = "site.updated"
	// The site picture was removed; these two stay for older audit entries.
	ActSitePictureSet     = "site.picture_set"
	ActSitePictureRemoved = "site.picture_removed"
	ActBootstrapAdmin     = "bootstrap.admin_created"
	ActChatAdded          = "chat.added"
	ActChatAutoLeft       = "chat.auto_left"
	ActChatAccepted       = "chat.accepted"
	ActChatLeft           = "chat.left"
	ActChatRemoved        = "chat.removed"
	ActChatMoved          = "chat.moved"
	ActTopicRenamed       = "topic.renamed"
	ActLabelCreated       = "label.created"
	ActLabelUpdated       = "label.updated"
	ActLabelDeleted       = "label.deleted"
	ActLabelBound         = "label.bound"
	ActLabelUnbound       = "label.unbound"
	ActNotificationsSaved = "notifications.updated"
	ActUserActivated      = "user.activated"
)

// UserTarget is the audit target of a user.
func UserTarget(id int64) string { return "user:" + strconv.FormatInt(id, 10) }

// HouseTarget is the audit target of a house.
func HouseTarget(id int64) string { return "house:" + strconv.FormatInt(id, 10) }

// StatusTarget is the audit target of a residency status.
func StatusTarget(id int64) string { return "status:" + strconv.FormatInt(id, 10) }

// SiteTarget is the audit target of site settings.
const SiteTarget = "site"

// Data space audit actions.
const (
	ActDataCreated       = "data.created"
	ActDataUpdated       = "data.updated"
	ActDataDeleted       = "data.deleted"
	ActDataVisibilitySet = "data.visibility_set"
	ActDataRowsChanged   = "data.rows_changed"
	// The public flag became a visibility; this stays for older audit entries.
	ActDataPublicSet = "data.public_set"
)

// DataTarget is the audit target of a data item.
func DataTarget(key string) string { return "data:" + key }

// Page audit actions.
const (
	ActPageDraftCreated = "page.draft_created"
	ActPageDraftUpdated = "page.draft_updated"
	ActPagePublished    = "page.published"
	ActPageUnpublished  = "page.unpublished"
	ActPageDeleted      = "page.deleted"
)

// PageTarget is the audit target of a page version.
func PageTarget(id int64) string { return "page:" + strconv.FormatInt(id, 10) }

// Module audit actions.
const (
	ActModuleEnabled       = "module.enabled"
	ActModuleDisabled      = "module.disabled"
	ActModuleSettingsSaved = "module.settings_saved"
)

// Feed audit actions.
const (
	ActFeedPostCreated   = "feed.post_created"
	ActFeedPostPublished = "feed.post_published"
	ActFeedPostUpdated   = "feed.post_updated"
	ActFeedPostDeleted   = "feed.post_deleted"
)

// FeedPostTarget is the audit target of a feed post.
func FeedPostTarget(id int64) string { return "feed_post:" + strconv.FormatInt(id, 10) }
