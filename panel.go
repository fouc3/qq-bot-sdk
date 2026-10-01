package qqbotsdk

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"unicode/utf8"
)

// panelPath is the instruction panel collection endpoint.
const panelPath = "/v2/panels"

// Documented panel limits.
const (
	// maxPanelItems is how many entries one panel may hold.
	maxPanelItems = 20
	// maxPanelTargets is how many openids one request may add or remove.
	maxPanelTargets = 20
	// maxPanelRemarkRunes is the length of the developer-only remark.
	maxPanelRemarkRunes = 255
	// DefaultPanelPageSize is the page size used when none is requested.
	DefaultPanelPageSize = 20
	// MaxPanelPageSize is the largest page the platform accepts.
	MaxPanelPageSize = 50
)

// Panel scopes: where a panel is shown.
const (
	// PanelScopeC2C shows the panel in single chats.
	PanelScopeC2C = "c2c"
	// PanelScopeGroup shows the panel in groups.
	PanelScopeGroup = "group"
	// PanelScopeChannel shows the panel in text channels.
	PanelScopeChannel = "channel"
	// PanelScopeDM shows the panel in channel direct messages.
	PanelScopeDM = "dm"
)

// Panel target types: who a panel applies to.
const (
	// PanelTargetAll applies to every user or group of the scope.
	PanelTargetAll = "all"
	// PanelTargetSpecific applies only to the listed users or groups, which
	// only the c2c and group scopes support.
	PanelTargetSpecific = "specific"
)

// Panel entry types.
const (
	// PanelItemCommand inserts its name into the composer when clicked.
	PanelItemCommand = "command"
	// PanelItemLink opens a URL.
	PanelItemLink = "link"
)

// Panel target operations.
const (
	// PanelTargetOpAdd associates users or groups with a panel.
	PanelTargetOpAdd = "add"
	// PanelTargetOpDel dissociates them.
	PanelTargetOpDel = "del"
)

// Panel is the content of an instruction panel.
type Panel struct {
	// Items are the panel entries, at most 20.
	Items []PanelItem `json:"items,omitempty"`
	// Remark is a developer note, at most 255 characters, never shown to
	// users.
	Remark string `json:"remark,omitempty"`
	// Version is the panel version.
	Version int `json:"version,omitempty"`
}

// PanelItem is one panel entry.
type PanelItem struct {
	// Name is the entry label, at most 14 characters where one Chinese
	// character counts as two.
	Name string `json:"name,omitempty"`
	// Desc explains the entry to the user, at most 30 characters where one
	// Chinese character counts as two.
	Desc string `json:"desc,omitempty"`
	// Type is PanelItemCommand or PanelItemLink.
	Type string `json:"type,omitempty"`
	// OnlyAdmin restricts the entry to channel or group administrators.
	OnlyAdmin bool `json:"only_admin,omitempty"`
	// Link is the target URL, only when Type is PanelItemLink.
	Link string `json:"link,omitempty"`
}

// PanelRecord is one panel in a list response.
type PanelRecord struct {
	// PanelID identifies the panel for later calls.
	PanelID string `json:"panel_id"`
	// Scope is where the panel is shown.
	Scope string `json:"scope"`
	// TargetType is PanelTargetAll or PanelTargetSpecific.
	TargetType string `json:"target_type"`
	// Panel is the panel content.
	Panel *Panel `json:"panel"`
	// CreatedAt is the creation time in RFC3339.
	CreatedAt string `json:"created_at,omitempty"`
	// UpdatedAt is the last update time in RFC3339.
	UpdatedAt string `json:"updated_at,omitempty"`
	// Version is the panel version.
	Version int `json:"version,omitempty"`
}

// PanelList is one page of panels.
type PanelList struct {
	// Records are the panels, newest first.
	Records []PanelRecord `json:"records"`
	// NextCursor fetches the next page. Empty means there is none.
	NextCursor string `json:"next_cursor,omitempty"`
	// IsEnd reports whether the last page was reached.
	IsEnd bool `json:"is_end,omitempty"`
}

// PanelDetail is one panel together with its associated targets.
type PanelDetail struct {
	PanelRecord
	// UserOpenIDs are the associated users, present for the c2c scope with
	// PanelTargetSpecific.
	UserOpenIDs []string `json:"user_openids,omitempty"`
	// GroupOpenIDs are the associated groups, present for the group scope with
	// PanelTargetSpecific.
	GroupOpenIDs []string `json:"group_openids,omitempty"`
}

// PanelCreateRequest is the body of the panel creation call.
type PanelCreateRequest struct {
	// Scope is required, one of the PanelScope constants.
	Scope string `json:"scope"`
	// TargetType is PanelTargetAll or PanelTargetSpecific. Empty is treated
	// as PanelTargetAll.
	TargetType string `json:"target_type,omitempty"`
	// UserOpenIDs associates the panel with users, only for the c2c scope
	// with PanelTargetSpecific, at most 20 per request.
	UserOpenIDs []string `json:"user_openids,omitempty"`
	// GroupOpenIDs associates the panel with groups, only for the group scope
	// with PanelTargetSpecific, at most 20 per request.
	GroupOpenIDs []string `json:"group_openids,omitempty"`
	// Panel is the panel content, which is required.
	Panel *Panel `json:"panel"`
}

// PanelTargetRequest changes which users or groups a panel applies to.
type PanelTargetRequest struct {
	// Op is required: PanelTargetOpAdd or PanelTargetOpDel.
	Op string `json:"op"`
	// UserOpenIDs are the users to add or remove, for the c2c scope.
	UserOpenIDs []string `json:"user_openids,omitempty"`
	// GroupOpenIDs are the groups to add or remove, for the group scope.
	GroupOpenIDs []string `json:"group_openids,omitempty"`
}

// VersionResponse is the version reported after an update.
type VersionResponse struct {
	// Version is the version after the change, which a caller can compare
	// later to detect changes.
	Version int `json:"version"`
}

// ValidateScope reports whether a value is one of the documented scopes.
func ValidateScope(scope string) error {
	switch scope {
	case PanelScopeC2C, PanelScopeGroup, PanelScopeChannel, PanelScopeDM:
		return nil
	case "":
		return errors.New("scope is required, one of c2c, group, channel or dm")
	default:
		return fmt.Errorf("scope %q is not one of c2c, group, channel or dm", scope)
	}
}

// Validate checks a panel against the documented limits.
func (p *Panel) Validate() error {
	if p == nil {
		return errors.New("qqbotsdk: panel is nil")
	}
	if len(p.Items) > maxPanelItems {
		return fmt.Errorf("qqbotsdk: panel has %d items, at most %d are allowed", len(p.Items), maxPanelItems)
	}
	if utf8.RuneCountInString(p.Remark) > maxPanelRemarkRunes {
		return fmt.Errorf("qqbotsdk: panel remark is longer than %d characters", maxPanelRemarkRunes)
	}
	for i, item := range p.Items {
		if err := item.validate(); err != nil {
			return fmt.Errorf("qqbotsdk: panel item %d: %w", i, err)
		}
	}
	return nil
}

// validate checks one panel entry.
func (p *PanelItem) validate() error {
	switch p.Type {
	case PanelItemCommand:
		if p.Name == "" {
			return errors.New("a command item needs a name")
		}
	case PanelItemLink:
		if p.Name == "" {
			return errors.New("a link item needs a name")
		}
		if err := validateHTTPSLink(p.Link); err != nil {
			return err
		}
	case "":
		return errors.New("type is required, one of command or link")
	default:
		return fmt.Errorf("type %q is not one of command or link", p.Type)
	}
	return nil
}

// Validate checks a creation request against the documented rules.
func (r *PanelCreateRequest) Validate() error {
	if r == nil {
		return errors.New("qqbotsdk: panel request is nil")
	}
	if err := ValidateScope(r.Scope); err != nil {
		return fmt.Errorf("qqbotsdk: %w", err)
	}
	if err := r.Panel.Validate(); err != nil {
		return err
	}

	targetType := r.TargetType
	if targetType == "" {
		targetType = PanelTargetAll
	}

	switch targetType {
	case PanelTargetAll:
		return nil
	case PanelTargetSpecific:
		// Only c2c and group panels may be limited to specific targets.
		if r.Scope != PanelScopeC2C && r.Scope != PanelScopeGroup {
			return fmt.Errorf("qqbotsdk: scope %s only supports target_type %s", r.Scope, PanelTargetAll)
		}
		switch r.Scope {
		case PanelScopeC2C:
			if len(r.UserOpenIDs) == 0 {
				return errors.New("qqbotsdk: a specific c2c panel needs user_openids")
			}
		case PanelScopeGroup:
			if len(r.GroupOpenIDs) == 0 {
				return errors.New("qqbotsdk: a specific group panel needs group_openids")
			}
		}
		if len(r.UserOpenIDs) > maxPanelTargets || len(r.GroupOpenIDs) > maxPanelTargets {
			return fmt.Errorf("qqbotsdk: at most %d openids may be associated per request", maxPanelTargets)
		}
		return nil
	default:
		return fmt.Errorf("qqbotsdk: target_type %q is not one of all or specific", r.TargetType)
	}
}

// Validate checks a target change request.
func (r *PanelTargetRequest) Validate() error {
	if r == nil {
		return errors.New("qqbotsdk: panel target request is nil")
	}
	switch r.Op {
	case PanelTargetOpAdd, PanelTargetOpDel:
	case "":
		return errors.New("qqbotsdk: op is required, one of add or del")
	default:
		return fmt.Errorf("qqbotsdk: op %q is not one of add or del", r.Op)
	}
	if len(r.UserOpenIDs) == 0 && len(r.GroupOpenIDs) == 0 {
		return errors.New("qqbotsdk: a target change needs user_openids or group_openids")
	}
	if len(r.UserOpenIDs) > maxPanelTargets || len(r.GroupOpenIDs) > maxPanelTargets {
		return fmt.Errorf("qqbotsdk: at most %d openids may be changed per request", maxPanelTargets)
	}
	return nil
}

// panelIDPath builds the address of one panel.
func panelIDPath(panelID string) string {
	return panelPath + "/" + url.PathEscape(panelID)
}

// ListPanels lists the panels of one scope, newest first.
//
// scope is required. cursor is the next_cursor of the previous page, or empty
// for the first one. limit defaults to DefaultPanelPageSize and is capped at
// MaxPanelPageSize.
func (c *Client) ListPanels(ctx context.Context, scope, cursor string, limit int) (*PanelList, error) {
	if err := ValidateScope(scope); err != nil {
		return nil, fmt.Errorf("qqbotsdk: ListPanels: %w", err)
	}

	query := url.Values{}
	query.Set("scope", scope)
	if cursor != "" {
		query.Set("cursor", cursor)
	}
	if limit > 0 {
		if limit > MaxPanelPageSize {
			limit = MaxPanelPageSize
		}
		query.Set("limit", strconv.Itoa(limit))
	}

	var out PanelList
	if err := c.doJSON(ctx, http.MethodGet, panelPath+"?"+query.Encode(), nil, &out, openAPICall); err != nil {
		return nil, err
	}
	return &out, nil
}

// CreatePanel creates an instruction panel and returns its id.
func (c *Client) CreatePanel(ctx context.Context, req *PanelCreateRequest) (string, error) {
	if err := req.Validate(); err != nil {
		return "", err
	}

	var out struct {
		PanelID string `json:"panel_id"`
	}
	if err := c.doJSON(ctx, http.MethodPost, panelPath, req, &out, openAPICall); err != nil {
		return "", err
	}
	if out.PanelID == "" {
		return "", errors.New("qqbotsdk: " + panelPath + ": empty panel_id in response")
	}
	return out.PanelID, nil
}

// GetPanel returns one panel with its associated users or groups.
func (c *Client) GetPanel(ctx context.Context, panelID string) (*PanelDetail, error) {
	if panelID == "" {
		return nil, errors.New("qqbotsdk: GetPanel needs a panel id")
	}

	var out PanelDetail
	if err := c.doJSON(ctx, http.MethodGet, panelIDPath(panelID), nil, &out, openAPICall); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdatePanel replaces the content of a panel.
//
// The associated users and groups are unaffected; use UpdatePanelTargets for
// those. It returns the new version.
func (c *Client) UpdatePanel(ctx context.Context, panelID string, panel *Panel) (*VersionResponse, error) {
	if panelID == "" {
		return nil, errors.New("qqbotsdk: UpdatePanel needs a panel id")
	}
	if err := panel.Validate(); err != nil {
		return nil, err
	}

	payload := struct {
		Panel *Panel `json:"panel"`
	}{Panel: panel}

	var out VersionResponse
	if err := c.doJSON(ctx, http.MethodPut, panelIDPath(panelID), payload, &out, openAPICall); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeletePanel removes a panel, so it stops applying to every user and group.
func (c *Client) DeletePanel(ctx context.Context, panelID string) error {
	if panelID == "" {
		return errors.New("qqbotsdk: DeletePanel needs a panel id")
	}
	return c.doJSON(ctx, http.MethodDelete, panelIDPath(panelID), nil, nil, openAPICall)
}

// UpdatePanelTargets adds or removes the users or groups a panel applies to.
//
// It only works for a specific panel: a global panel rejects the call, since it
// already applies to everyone in its scope.
func (c *Client) UpdatePanelTargets(ctx context.Context, panelID string, req *PanelTargetRequest) error {
	if panelID == "" {
		return errors.New("qqbotsdk: UpdatePanelTargets needs a panel id")
	}
	if err := req.Validate(); err != nil {
		return err
	}
	return c.doJSON(ctx, http.MethodPut, panelIDPath(panelID)+"/target", req, nil, openAPICall)
}
