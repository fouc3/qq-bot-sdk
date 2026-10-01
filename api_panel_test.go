package qqbotsdk

import (
	"net/http"
	"strings"
	"testing"
)

// TestListPanelsParsesDocumentedExample reproduces the documented response.
func TestListPanelsParsesDocumentedExample(t *testing.T) {
	client, captured := newMessageServer(t, http.StatusOK, `{
		"records": [{
			"panel_id": "p_102030405_x8k2",
			"scope": "c2c",
			"target_type": "all",
			"panel": {"items": [{"type":"command","name":"查询天气","desc":"查询当前天气"}]},
			"version": 1
		}],
		"next_cursor": "",
		"is_end": true
	}`)

	page, err := client.ListPanels(t.Context(), PanelScopeC2C, "", 10)
	if err != nil {
		t.Fatalf("ListPanels: %v", err)
	}

	req := last(t, captured)
	if req.Method != http.MethodGet {
		t.Errorf("method = %s, want GET", req.Method)
	}
	if req.Path != "/v2/panels" {
		t.Errorf("path = %s, want /v2/panels", req.Path)
	}
	if req.Query != "limit=10&scope=c2c" {
		t.Errorf("query = %q, want scope and limit", req.Query)
	}

	if len(page.Records) != 1 {
		t.Fatalf("Records = %d, want 1", len(page.Records))
	}
	record := page.Records[0]
	if record.PanelID != "p_102030405_x8k2" || record.Scope != PanelScopeC2C {
		t.Errorf("record = %+v", record)
	}
	if record.TargetType != PanelTargetAll {
		t.Errorf("TargetType = %q, want all", record.TargetType)
	}
	if record.Panel == nil || len(record.Panel.Items) != 1 {
		t.Fatalf("Panel = %+v", record.Panel)
	}
	if record.Panel.Items[0].Type != PanelItemCommand || record.Panel.Items[0].Name != "查询天气" {
		t.Errorf("item = %+v", record.Panel.Items[0])
	}
	if !page.IsEnd {
		t.Error("IsEnd = false, want true")
	}
}

// TestListPanelsPaginationQuery checks the cursor handling.
func TestListPanelsPaginationQuery(t *testing.T) {
	client, captured := newMessageServer(t, http.StatusOK, `{"records":[],"next_cursor":"NEXT","is_end":false}`)

	page, err := client.ListPanels(t.Context(), PanelScopeGroup, "CUR", 0)
	if err != nil {
		t.Fatalf("ListPanels: %v", err)
	}
	if page.NextCursor != "NEXT" {
		t.Errorf("NextCursor = %q, want NEXT", page.NextCursor)
	}

	req := last(t, captured)
	// No limit is sent when it is not requested.
	if req.Query != "cursor=CUR&scope=group" {
		t.Errorf("query = %q, want cursor and scope only", req.Query)
	}
}

// TestListPanelsCapsLimit checks the documented maximum of 50.
func TestListPanelsCapsLimit(t *testing.T) {
	client, captured := newMessageServer(t, http.StatusOK, `{"records":[]}`)

	if _, err := client.ListPanels(t.Context(), PanelScopeDM, "", 500); err != nil {
		t.Fatalf("ListPanels: %v", err)
	}
	if query := last(t, captured).Query; query != "limit=50&scope=dm" {
		t.Errorf("query = %q, want the limit capped at 50", query)
	}
}

// TestListPanelsRequiresValidScope checks the documented scope enum, since the
// endpoint demands a scope.
func TestListPanelsRequiresValidScope(t *testing.T) {
	client, captured := newMessageServer(t, http.StatusOK, `{"records":[]}`)

	for _, scope := range []string{"", "guild", "C2C"} {
		if _, err := client.ListPanels(t.Context(), scope, "", 0); err == nil {
			t.Errorf("scope %q must be rejected", scope)
		}
	}
	if len(*captured) != 0 {
		t.Error("an invalid scope must be rejected without a request")
	}
}

// TestCreatePanelMatchesDocumentedExample reproduces the documented c2c create.
func TestCreatePanelMatchesDocumentedExample(t *testing.T) {
	client, captured := newMessageServer(t, http.StatusOK, `{"panel_id":"p_x8k2x8k2x8k2"}`)

	panelID, err := client.CreatePanel(t.Context(), &PanelCreateRequest{
		Scope:      PanelScopeC2C,
		TargetType: PanelTargetAll,
		Panel: &Panel{
			Items: []PanelItem{
				{Type: PanelItemCommand, Name: "查询天气", Desc: "查询当前天气"},
				{Type: PanelItemLink, Name: "更多服务", Link: "https://example.com"},
			},
			Remark: "C2C面板",
		},
	})
	if err != nil {
		t.Fatalf("CreatePanel: %v", err)
	}

	req := last(t, captured)
	if req.Method != http.MethodPost {
		t.Errorf("method = %s, want POST", req.Method)
	}
	if req.Path != "/v2/panels" {
		t.Errorf("path = %s, want /v2/panels", req.Path)
	}
	if panelID != "p_x8k2x8k2x8k2" {
		t.Errorf("panelID = %q", panelID)
	}

	body := decodeBody(t, req)
	if body["scope"] != PanelScopeC2C || body["target_type"] != PanelTargetAll {
		t.Errorf("body = %v", body)
	}
	panel := body["panel"].(map[string]any)
	if panel["remark"] != "C2C面板" {
		t.Errorf("remark = %v", panel["remark"])
	}
	items := panel["items"].([]any)
	if len(items) != 2 {
		t.Fatalf("items = %v, want 2", items)
	}
	if items[1].(map[string]any)["link"] != "https://example.com" {
		t.Errorf("link item = %v", items[1])
	}
}

// TestCreatePanelSpecificGroup covers the documented specific target form.
func TestCreatePanelSpecificGroup(t *testing.T) {
	client, captured := newMessageServer(t, http.StatusOK, `{"panel_id":"p1"}`)

	if _, err := client.CreatePanel(t.Context(), &PanelCreateRequest{
		Scope:        PanelScopeGroup,
		TargetType:   PanelTargetSpecific,
		GroupOpenIDs: []string{"openid_group_001", "openid_group_002"},
		Panel: &Panel{Items: []PanelItem{
			{Type: PanelItemCommand, Name: "群签到", Desc: "每日签到"},
		}},
	}); err != nil {
		t.Fatalf("CreatePanel: %v", err)
	}

	body := decodeBody(t, last(t, captured))
	groups, ok := body["group_openids"].([]any)
	if !ok || len(groups) != 2 {
		t.Fatalf("group_openids = %v, want 2", body["group_openids"])
	}
}

// TestCreatePanelValidation covers the documented rules that are checked
// locally.
func TestCreatePanelValidation(t *testing.T) {
	validPanel := &Panel{Items: []PanelItem{{Type: PanelItemCommand, Name: "x"}}}

	invalid := map[string]*PanelCreateRequest{
		"no scope":        {Panel: validPanel},
		"bad scope":       {Scope: "guild", Panel: validPanel},
		"no panel":        {Scope: PanelScopeC2C},
		"bad target type": {Scope: PanelScopeC2C, TargetType: "everyone", Panel: validPanel},
		// channel and dm panels are always global.
		"channel specific": {Scope: PanelScopeChannel, TargetType: PanelTargetSpecific, Panel: validPanel},
		"dm specific":      {Scope: PanelScopeDM, TargetType: PanelTargetSpecific, Panel: validPanel},
		// a specific panel needs its list.
		"c2c specific without users": {Scope: PanelScopeC2C, TargetType: PanelTargetSpecific, Panel: validPanel},
		"group specific without groups": {
			Scope: PanelScopeGroup, TargetType: PanelTargetSpecific, Panel: validPanel,
		},
		// panel item rules.
		"item without type": {Scope: PanelScopeC2C, Panel: &Panel{Items: []PanelItem{{Name: "x"}}}},
		"item link not https": {
			Scope: PanelScopeC2C,
			Panel: &Panel{Items: []PanelItem{{Type: PanelItemLink, Name: "x", Link: "http://example.com"}}},
		},
	}

	for name, req := range invalid {
		t.Run(name, func(t *testing.T) {
			if err := req.Validate(); err == nil {
				t.Error("expected a validation error")
			}
		})
	}

	// The documented creation shapes must pass.
	valid := []*PanelCreateRequest{
		{Scope: PanelScopeC2C, TargetType: PanelTargetAll, Panel: validPanel},
		{Scope: PanelScopeC2C, Panel: validPanel}, // empty target_type means all
		{
			Scope: PanelScopeC2C, TargetType: PanelTargetSpecific,
			UserOpenIDs: []string{"u1"}, Panel: validPanel,
		},
		{
			Scope: PanelScopeGroup, TargetType: PanelTargetSpecific,
			GroupOpenIDs: []string{"g1"}, Panel: validPanel,
		},
		{Scope: PanelScopeChannel, TargetType: PanelTargetAll, Panel: validPanel},
		{Scope: PanelScopeDM, Panel: validPanel},
	}
	for i, req := range valid {
		if err := req.Validate(); err != nil {
			t.Errorf("valid request %d rejected: %v", i, err)
		}
	}
}

// TestPanelValidateLimits covers the documented counts.
func TestPanelValidateLimits(t *testing.T) {
	if err := (*Panel)(nil).Validate(); err == nil {
		t.Error("a nil panel must be rejected")
	}

	tooMany := &Panel{}
	for i := 0; i < maxPanelItems+1; i++ {
		tooMany.Items = append(tooMany.Items, PanelItem{Type: PanelItemCommand, Name: "x"})
	}
	if err := tooMany.Validate(); err == nil {
		t.Error("a panel over the item limit must be rejected")
	} else if !strings.Contains(err.Error(), "at most 20") {
		t.Errorf("err = %v, want it to name the limit", err)
	}

	atLimit := &Panel{}
	for i := 0; i < maxPanelItems; i++ {
		atLimit.Items = append(atLimit.Items, PanelItem{Type: PanelItemCommand, Name: "x"})
	}
	if err := atLimit.Validate(); err != nil {
		t.Errorf("a panel at the item limit must be accepted: %v", err)
	}

	longRemark := &Panel{Remark: strings.Repeat("字", maxPanelRemarkRunes+1)}
	if err := longRemark.Validate(); err == nil {
		t.Error("an over-long remark must be rejected")
	}
}

// TestGetPanelParsesDetail covers the detail response, including the associated
// target lists.
func TestGetPanelParsesDetail(t *testing.T) {
	client, captured := newMessageServer(t, http.StatusOK, `{
		"panel_id":"p_x8k2x8k2x8k2",
		"scope":"group",
		"target_type":"specific",
		"panel":{"items":[{"type":"command","name":"群签到","desc":"每日签到"}]},
		"version":1,
		"user_openids":[],
		"group_openids":["openid_group_001"]
	}`)

	detail, err := client.GetPanel(t.Context(), "p_x8k2x8k2x8k2")
	if err != nil {
		t.Fatalf("GetPanel: %v", err)
	}
	if got := last(t, captured).Path; got != "/v2/panels/p_x8k2x8k2x8k2" {
		t.Errorf("path = %s", got)
	}

	if detail.Scope != PanelScopeGroup || detail.TargetType != PanelTargetSpecific {
		t.Errorf("detail = %+v", detail)
	}
	if len(detail.GroupOpenIDs) != 1 || detail.GroupOpenIDs[0] != "openid_group_001" {
		t.Errorf("GroupOpenIDs = %v", detail.GroupOpenIDs)
	}
	if len(detail.UserOpenIDs) != 0 {
		t.Errorf("UserOpenIDs = %v, want none", detail.UserOpenIDs)
	}
	if detail.Panel == nil || len(detail.Panel.Items) != 1 {
		t.Fatalf("Panel = %+v", detail.Panel)
	}
}

// TestUpdatePanel covers the content update.
func TestUpdatePanel(t *testing.T) {
	client, captured := newMessageServer(t, http.StatusOK, `{"version":1}`)

	version, err := client.UpdatePanel(t.Context(), "p1", &Panel{
		Items:  []PanelItem{{Type: PanelItemCommand, Name: "新指令", Desc: "更新后的指令"}},
		Remark: "更新备注",
	})
	if err != nil {
		t.Fatalf("UpdatePanel: %v", err)
	}
	if version.Version != 1 {
		t.Errorf("Version = %d, want 1", version.Version)
	}

	req := last(t, captured)
	if req.Method != http.MethodPut {
		t.Errorf("method = %s, want PUT", req.Method)
	}
	if req.Path != "/v2/panels/p1" {
		t.Errorf("path = %s", req.Path)
	}
	body := decodeBody(t, req)
	if _, ok := body["panel"].(map[string]any); !ok {
		t.Errorf("body = %v, want a panel object", body)
	}
}

func TestDeletePanel(t *testing.T) {
	client, captured := newMessageServer(t, http.StatusOK, `{}`)

	if err := client.DeletePanel(t.Context(), "p1"); err != nil {
		t.Fatalf("DeletePanel: %v", err)
	}

	req := last(t, captured)
	if req.Method != http.MethodDelete {
		t.Errorf("method = %s, want DELETE", req.Method)
	}
	if req.Path != "/v2/panels/p1" {
		t.Errorf("path = %s", req.Path)
	}
}

// TestUpdatePanelTargetsMatchesDocumentedExample covers the association change.
func TestUpdatePanelTargetsMatchesDocumentedExample(t *testing.T) {
	client, captured := newMessageServer(t, http.StatusOK, `{}`)

	if err := client.UpdatePanelTargets(t.Context(), "p1", &PanelTargetRequest{
		Op:           PanelTargetOpAdd,
		GroupOpenIDs: []string{"openid_group_003"},
	}); err != nil {
		t.Fatalf("UpdatePanelTargets: %v", err)
	}

	req := last(t, captured)
	if req.Method != http.MethodPut {
		t.Errorf("method = %s, want PUT", req.Method)
	}
	if req.Path != "/v2/panels/p1/target" {
		t.Errorf("path = %s, want the target address", req.Path)
	}
	body := decodeBody(t, req)
	if body["op"] != PanelTargetOpAdd {
		t.Errorf("op = %v, want add", body["op"])
	}
	if groups, ok := body["group_openids"].([]any); !ok || len(groups) != 1 {
		t.Errorf("group_openids = %v", body["group_openids"])
	}

	if err := client.UpdatePanelTargets(t.Context(), "p1", &PanelTargetRequest{
		Op:          PanelTargetOpDel,
		UserOpenIDs: []string{"openid_user_001"},
	}); err != nil {
		t.Fatalf("UpdatePanelTargets: %v", err)
	}
	body = decodeBody(t, last(t, captured))
	if body["op"] != PanelTargetOpDel {
		t.Errorf("op = %v, want del", body["op"])
	}
}

func TestUpdatePanelTargetsValidation(t *testing.T) {
	client, captured := newMessageServer(t, http.StatusOK, `{}`)

	invalid := map[string]*PanelTargetRequest{
		"nil":            nil,
		"no op":          {GroupOpenIDs: []string{"g"}},
		"bad op":         {Op: "remove", GroupOpenIDs: []string{"g"}},
		"no openids":     {Op: PanelTargetOpAdd},
		"too many users": {Op: PanelTargetOpAdd, UserOpenIDs: make([]string, maxPanelTargets+1)},
	}
	for name, req := range invalid {
		t.Run(name, func(t *testing.T) {
			if err := client.UpdatePanelTargets(t.Context(), "p1", req); err == nil {
				t.Error("expected a validation error")
			}
		})
	}

	if len(*captured) != 0 {
		t.Error("an invalid target request must be rejected without a request")
	}
}

// TestPanelMethodsRequirePanelID checks the local guards.
func TestPanelMethodsRequirePanelID(t *testing.T) {
	client, captured := newMessageServer(t, http.StatusOK, `{}`)
	validPanel := &Panel{Items: []PanelItem{{Type: PanelItemCommand, Name: "x"}}}
	validTarget := &PanelTargetRequest{Op: PanelTargetOpAdd, GroupOpenIDs: []string{"g"}}

	calls := map[string]func() error{
		"GetPanel":           func() error { _, err := client.GetPanel(t.Context(), ""); return err },
		"UpdatePanel":        func() error { _, err := client.UpdatePanel(t.Context(), "", validPanel); return err },
		"DeletePanel":        func() error { return client.DeletePanel(t.Context(), "") },
		"UpdatePanelTargets": func() error { return client.UpdatePanelTargets(t.Context(), "", validTarget) },
	}
	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			if err := call(); err == nil {
				t.Error("a missing panel id must be rejected before any request")
			}
		})
	}

	if len(*captured) != 0 {
		t.Error("a rejected call must not send a request")
	}
}

// TestPanelErrorCodes checks the documented panel failures.
func TestPanelErrorCodes(t *testing.T) {
	client, _ := newMessageServer(t, http.StatusOK, `{"err_code":40030021,"message":"全局面板不支持添加指定关联对象"}`)

	err := client.UpdatePanelTargets(t.Context(), "p1", &PanelTargetRequest{
		Op:           PanelTargetOpAdd,
		GroupOpenIDs: []string{"g1"},
	})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if !IsOpenAPIError(err, ErrGlobalPanelNoTarget) {
		t.Errorf("err = %v, want the global panel error code", err)
	}
	if !strings.Contains(err.Error(), "全局面板不支持添加指定关联对象") {
		t.Errorf("err = %v, want the documented description", err)
	}
}

func TestPanelScopeConstants(t *testing.T) {
	for _, scope := range []string{PanelScopeC2C, PanelScopeGroup, PanelScopeChannel, PanelScopeDM} {
		if err := ValidateScope(scope); err != nil {
			t.Errorf("ValidateScope(%q) = %v, want nil", scope, err)
		}
	}
}

// TestMenuPanelErrorCodeNames pins the menu and panel descriptions.
func TestMenuPanelErrorCodeNames(t *testing.T) {
	cases := map[OpenAPIErrorCode]string{
		ErrMenuPanelParamInvalid:    "参数错误",
		ErrPanelNotFound:            "指令面板不存在",
		ErrURLFormatInvalid:         "URL 格式错误",
		ErrPanelOperationInProgress: "指令面板操作进行中，请稍后重试",
		ErrPanelScopeInvalid:        "生效场景不合法",
		ErrPanelTargetTypeInvalid:   "生效范围不合法",
		ErrCountLimitExceeded:       "超出数量限制",
		ErrMenuTypeInvalid:          "菜单类型不合法",
		ErrPanelItemTypeInvalid:     "面板元素类型不合法",
		ErrRequiredFieldMissing:     "必填字段缺失",
		ErrPanelTargetOpInvalid:     "操作类型不合法",
		ErrScopeNotSupported:        "当前场景不支持此操作",
		ErrContentSecurityRisk:      "内容存在安全风险，请修改后重试",
		ErrGlobalPanelNoTarget:      "全局面板不支持添加指定关联对象",
	}
	for code, want := range cases {
		if got := code.String(); got != want {
			t.Errorf("OpenAPIErrorCode(%d).String() = %q, want %q", code, got, want)
		}
	}
}
