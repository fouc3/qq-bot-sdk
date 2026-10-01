package qqbotsdk

import (
	"net/http"
	"strings"
	"testing"
)

// TestGetMenuParsesDocumentedExample reproduces the documented response.
func TestGetMenuParsesDocumentedExample(t *testing.T) {
	client, captured := newMessageServer(t, http.StatusOK, `{
		"menu": {"items": [{"type":"send_message","name":"帮助","send_message":"/help"}]},
		"version": 1
	}`)

	config, err := client.GetMenu(t.Context())
	if err != nil {
		t.Fatalf("GetMenu: %v", err)
	}

	req := last(t, captured)
	if req.Method != http.MethodGet {
		t.Errorf("method = %s, want GET", req.Method)
	}
	if req.Path != "/v2/menu" {
		t.Errorf("path = %s, want /v2/menu", req.Path)
	}

	if config.Version != 1 {
		t.Errorf("Version = %d, want 1", config.Version)
	}
	if config.Menu == nil || len(config.Menu.Items) != 1 {
		t.Fatalf("Menu = %+v, want one item", config.Menu)
	}
	item := config.Menu.Items[0]
	if item.Type != MenuTypeSendMessage || item.Name != "帮助" || item.SendMessage != "/help" {
		t.Errorf("item = %+v", item)
	}
}

// TestGetMenuWithoutMenu checks that a never configured menu is not an error.
func TestGetMenuWithoutMenu(t *testing.T) {
	client, _ := newMessageServer(t, http.StatusOK, `{"version":0}`)

	config, err := client.GetMenu(t.Context())
	if err != nil {
		t.Fatalf("GetMenu: %v", err)
	}
	if config.Menu != nil {
		t.Errorf("Menu = %+v, want nil when no menu was set", config.Menu)
	}
}

// TestSetMenuMatchesDocumentedExample reproduces the documented multi type menu.
func TestSetMenuMatchesDocumentedExample(t *testing.T) {
	client, captured := newMessageServer(t, http.StatusOK, `{"version":1}`)

	version, err := client.SetMenu(t.Context(), &Menu{Items: []MenuItem{
		{Type: MenuTypeSendMessage, Name: "帮助", SendMessage: "/help"},
		{Type: MenuTypeLink, Name: "官网", Link: "https://example.com"},
		{Type: MenuTypeMenu, Name: "更多", SubMenuItems: []SubMenuItem{
			{Type: SubMenuTypeSendMessage, Name: "设置", SendMessage: "/settings"},
		}},
	}})
	if err != nil {
		t.Fatalf("SetMenu: %v", err)
	}

	req := last(t, captured)
	if req.Method != http.MethodPut {
		t.Errorf("method = %s, want PUT", req.Method)
	}
	if req.Path != "/v2/menu" {
		t.Errorf("path = %s, want /v2/menu", req.Path)
	}
	if version.Version != 1 {
		t.Errorf("Version = %d, want 1", version.Version)
	}

	body := decodeBody(t, req)
	menu, ok := body["menu"].(map[string]any)
	if !ok {
		t.Fatalf("body = %v, want a menu object", body)
	}
	items, ok := menu["items"].([]any)
	if !ok || len(items) != 3 {
		t.Fatalf("items = %v, want 3", menu["items"])
	}

	link := items[1].(map[string]any)
	if link["type"] != MenuTypeLink || link["link"] != "https://example.com" {
		t.Errorf("link item = %v", link)
	}

	collapsible := items[2].(map[string]any)
	subs := collapsible["sub_menu_items"].([]any)
	if len(subs) != 1 {
		t.Fatalf("sub_menu_items = %v, want 1", subs)
	}
	if subs[0].(map[string]any)["send_message"] != "/settings" {
		t.Errorf("sub item = %v", subs[0])
	}
}

// TestSetMenuSwitchButton covers the toggle configuration.
func TestSetMenuSwitchButton(t *testing.T) {
	client, captured := newMessageServer(t, http.StatusOK, `{"version":2}`)

	if _, err := client.SetMenu(t.Context(), &Menu{Items: []MenuItem{{
		Type:   MenuTypeSwitch,
		Name:   "搜索",
		Switch: &MenuSwitch{SwitchID: "search", Default: true},
	}}}); err != nil {
		t.Fatalf("SetMenu: %v", err)
	}

	item := decodeBody(t, last(t, captured))["menu"].(map[string]any)["items"].([]any)[0].(map[string]any)
	toggle, ok := item["switch"].(map[string]any)
	if !ok {
		t.Fatalf("item = %v, want a switch object", item)
	}
	if toggle["switch_id"] != "search" || toggle["default"] != true {
		t.Errorf("switch = %v", toggle)
	}
}

// TestMenuValidate covers the documented limits, which are checked locally so
// the error names the offending entry.
func TestMenuValidate(t *testing.T) {
	validSend := MenuItem{Type: MenuTypeSendMessage, Name: "帮助", SendMessage: "/help"}

	tooMany := &Menu{}
	for i := 0; i < maxMenuItems+1; i++ {
		tooMany.Items = append(tooMany.Items, validSend)
	}

	tooManySubs := &Menu{Items: []MenuItem{{
		Type: MenuTypeMenu, Name: "更多",
		SubMenuItems: make([]SubMenuItem, maxSubMenuItems+1),
	}}}

	invalid := map[string]*Menu{
		"nil":                nil,
		"too many items":     tooMany,
		"too many sub items": tooManySubs,
		"unknown type":       {Items: []MenuItem{{Type: "nope"}}},
		"missing type":       {Items: []MenuItem{{Name: "x"}}},
		"link without link":  {Items: []MenuItem{{Type: MenuTypeLink, Name: "x"}}},
		"link not https":     {Items: []MenuItem{{Type: MenuTypeLink, Name: "x", Link: "http://example.com"}}},
		"switch without cfg": {Items: []MenuItem{{Type: MenuTypeSwitch, Name: "x"}}},
		"menu without subs":  {Items: []MenuItem{{Type: MenuTypeMenu, Name: "x"}}},
		"sub unknown type": {Items: []MenuItem{{Type: MenuTypeMenu, Name: "x", SubMenuItems: []SubMenuItem{
			{Type: "nope"},
		}}}},
		"nested sub menu": {Items: []MenuItem{{Type: MenuTypeMenu, Name: "x", SubMenuItems: []SubMenuItem{
			{Type: MenuTypeMenu, Name: "y"},
		}}}},
		"sub link not https": {Items: []MenuItem{{Type: MenuTypeMenu, Name: "x", SubMenuItems: []SubMenuItem{
			{Type: SubMenuTypeLink, Link: "http://example.com"},
		}}}},
	}

	for name, menu := range invalid {
		t.Run(name, func(t *testing.T) {
			if err := menu.Validate(); err == nil {
				t.Error("expected a validation error")
			}
		})
	}

	// A menu at the limit must pass.
	atLimit := &Menu{}
	for i := 0; i < maxMenuItems; i++ {
		atLimit.Items = append(atLimit.Items, validSend)
	}
	if err := atLimit.Validate(); err != nil {
		t.Errorf("a menu at the item limit must be accepted: %v", err)
	}
}

// TestSetMenuRejectsInvalidWithoutRequest checks that validation happens before
// any call is made.
func TestSetMenuRejectsInvalidWithoutRequest(t *testing.T) {
	client, captured := newMessageServer(t, http.StatusOK, `{"version":1}`)

	_, err := client.SetMenu(t.Context(), &Menu{Items: []MenuItem{
		{Type: MenuTypeLink, Name: "坏链接", Link: "http://example.com"},
	}})
	if err == nil {
		t.Fatal("expected a validation error")
	}
	if !strings.Contains(err.Error(), "https://") {
		t.Errorf("err = %v, want it to name the https requirement", err)
	}
	if len(*captured) != 0 {
		t.Error("an invalid menu must be rejected without a request")
	}
}

func TestMenuPropagatesError(t *testing.T) {
	client, _ := newMessageServer(t, http.StatusOK, `{"err_code":40030014,"message":"菜单类型不合法"}`)

	_, err := client.SetMenu(t.Context(), &Menu{Items: []MenuItem{
		{Type: MenuTypeSendMessage, Name: "帮助", SendMessage: "/help"},
	}})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if !IsOpenAPIError(err, ErrMenuTypeInvalid) {
		t.Errorf("err = %v, want the menu type error code", err)
	}
}
