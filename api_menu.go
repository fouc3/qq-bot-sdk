package qqbotsdk

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// menuPath is the global custom menu endpoint.
const menuPath = "/v2/menu"

// Documented menu limits.
const (
	// maxMenuItems is how many top level buttons a menu may hold.
	maxMenuItems = 10
	// maxSubMenuItems is how many entries a collapsible sub menu may hold.
	maxSubMenuItems = 5
)

// Menu button types.
const (
	// MenuTypeSwitch is a toggle.
	MenuTypeSwitch = "switch"
	// MenuTypeSendMessage fills the composer with a text.
	MenuTypeSendMessage = "send_message"
	// MenuTypeLink opens a URL.
	MenuTypeLink = "link"
	// MenuTypeMenu is a collapsible entry holding a sub menu.
	MenuTypeMenu = "menu"
)

// Sub menu button types. A sub menu cannot hold another sub menu.
const (
	// SubMenuTypeSendMessage fills the composer with a text.
	SubMenuTypeSendMessage = "send_message"
	// SubMenuTypeLink opens a URL.
	SubMenuTypeLink = "link"
)

// Menu is the custom menu shown at the bottom of a single chat window.
//
// The menu applies to every user; the platform does not support per-user
// menus.
type Menu struct {
	// Items are the top level buttons, at most 10, laid out left to right.
	Items []MenuItem `json:"items,omitempty"`
}

// MenuItem is one top level menu button.
//
// Which field is read depends on Type.
type MenuItem struct {
	// Name is the button label, at most 10 characters where one Chinese
	// character counts as two.
	Name string `json:"name,omitempty"`
	// Type is one of the MenuType constants.
	Type string `json:"type,omitempty"`
	// SubMenuItems are the entries of a collapsible menu, at most 5, and only
	// read when Type is MenuTypeMenu.
	SubMenuItems []SubMenuItem `json:"sub_menu_items,omitempty"`
	// SendMessage is inserted into the composer, only when Type is
	// MenuTypeSendMessage.
	SendMessage string `json:"send_message,omitempty"`
	// Link is the target URL, only when Type is MenuTypeLink. It must start
	// with https://.
	Link string `json:"link,omitempty"`
	// Switch is the toggle configuration, only when Type is MenuTypeSwitch.
	Switch *MenuSwitch `json:"switch,omitempty"`
}

// SubMenuItem is one entry of a collapsible menu.
type SubMenuItem struct {
	// Name is the entry label, at most 14 characters where one Chinese
	// character counts as two.
	Name string `json:"name,omitempty"`
	// Type is SubMenuTypeSendMessage or SubMenuTypeLink.
	Type string `json:"type,omitempty"`
	// SendMessage is inserted into the composer, only when Type is
	// SubMenuTypeSendMessage.
	SendMessage string `json:"send_message,omitempty"`
	// Link is the target URL, only when Type is SubMenuTypeLink. It must
	// start with https://.
	Link string `json:"link,omitempty"`
}

// MenuSwitch describes a toggle button.
type MenuSwitch struct {
	// SwitchID identifies the toggle. When a user changes it, the platform
	// sends a message whose ext data carries this id, for example "search=1"
	// when the toggle is on.
	SwitchID string `json:"switch_id,omitempty"`
	// Default is the initial state: true is on, false is off.
	//
	// False is the zero value, so it is omitted and the platform applies its
	// own default.
	Default bool `json:"default,omitempty"`
}

// MenuConfig is the current menu together with its version.
type MenuConfig struct {
	// Version is the current menu version.
	Version int `json:"version"`
	// Menu is the active configuration. It is nil when no menu was ever set.
	Menu *Menu `json:"menu,omitempty"`
}

// Validate checks the menu against the documented limits.
//
// The platform enforces the same rules; checking locally reports which entry is
// wrong instead of returning only an error code.
func (m *Menu) Validate() error {
	if m == nil {
		return errors.New("qqbotsdk: menu is nil")
	}
	if len(m.Items) > maxMenuItems {
		return fmt.Errorf("qqbotsdk: menu has %d items, at most %d are allowed", len(m.Items), maxMenuItems)
	}

	for i, item := range m.Items {
		if err := item.validate(); err != nil {
			return fmt.Errorf("qqbotsdk: menu item %d: %w", i, err)
		}
	}
	return nil
}

// validate checks one top level menu item.
func (m *MenuItem) validate() error {
	switch m.Type {
	case MenuTypeSwitch:
		if m.Switch == nil {
			return errors.New("a switch item needs a switch configuration")
		}
	case MenuTypeSendMessage:
		if m.SendMessage == "" {
			return errors.New("a send_message item needs send_message")
		}
	case MenuTypeLink:
		if err := validateHTTPSLink(m.Link); err != nil {
			return err
		}
	case MenuTypeMenu:
		if len(m.SubMenuItems) == 0 {
			return errors.New("a menu item needs sub_menu_items")
		}
		if len(m.SubMenuItems) > maxSubMenuItems {
			return fmt.Errorf("a sub menu has %d items, at most %d are allowed",
				len(m.SubMenuItems), maxSubMenuItems)
		}
		for j, sub := range m.SubMenuItems {
			if err := sub.validate(); err != nil {
				return fmt.Errorf("sub menu item %d: %w", j, err)
			}
		}
	case "":
		return errors.New("type is required, one of switch, send_message, link or menu")
	default:
		return fmt.Errorf("type %q is not one of switch, send_message, link or menu", m.Type)
	}
	return nil
}

// validate checks one sub menu entry.
func (s *SubMenuItem) validate() error {
	switch s.Type {
	case SubMenuTypeSendMessage:
		if s.SendMessage == "" {
			return errors.New("a send_message entry needs send_message")
		}
	case SubMenuTypeLink:
		if err := validateHTTPSLink(s.Link); err != nil {
			return err
		}
	case MenuTypeMenu:
		return errors.New("a sub menu cannot hold another sub menu")
	case "":
		return errors.New("type is required, one of send_message or link")
	default:
		return fmt.Errorf("type %q is not one of send_message or link", s.Type)
	}
	return nil
}

// validateHTTPSLink enforces the documented requirement that a link button
// target starts with https://.
func validateHTTPSLink(link string) error {
	if link == "" {
		return errors.New("a link item needs a link")
	}
	if !strings.HasPrefix(link, "https://") {
		return fmt.Errorf("link %q must start with https://", link)
	}
	return nil
}

// GetMenu returns the current global custom menu.
//
// It is the documented GET /v2/menu endpoint. A never configured menu comes
// back with a nil Menu field rather than an error.
func (c *Client) GetMenu(ctx context.Context) (*MenuConfig, error) {
	var out MenuConfig
	if err := c.doJSON(ctx, http.MethodGet, menuPath, nil, &out, openAPICall); err != nil {
		return nil, err
	}
	return &out, nil
}

// SetMenu replaces the whole global custom menu.
//
// It is the documented PUT /v2/menu endpoint. The previous configuration is
// overwritten entirely, so the menu passed here must be complete rather than a
// patch. It returns the new version.
func (c *Client) SetMenu(ctx context.Context, menu *Menu) (*VersionResponse, error) {
	if err := menu.Validate(); err != nil {
		return nil, err
	}

	payload := struct {
		Menu *Menu `json:"menu"`
	}{Menu: menu}

	var out VersionResponse
	if err := c.doJSON(ctx, http.MethodPut, menuPath, payload, &out, openAPICall); err != nil {
		return nil, err
	}
	return &out, nil
}
