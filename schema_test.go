package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Skema input tidak boleh memuat union "null" (hemat token di tools/list),
// parameter opsional tetap opsional, dan argumen yang tidak dikirim tetap
// memakai nilai bawaan.
func TestSkemaTanpaUnionNull(t *testing.T) {
	m := newTestManager(t)
	ctx := context.Background()
	srv := newServer(m.cfg)
	ct, st := mcp.NewInMemoryTransports()
	ss, err := srv.Connect(ctx, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ss.Close()
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "uji", Version: "0"}, nil).Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()

	tools, err := cs.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(tools.Tools) != 11+execToolCount {
		t.Fatalf("jumlah tool = %d, harapan %d", len(tools.Tools), 11+execToolCount)
	}
	schemas := map[string]map[string]any{}
	for _, tool := range tools.Tools {
		b, err := json.Marshal(tool.InputSchema)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(b), `"null"`) {
			t.Errorf("%s: skema memuat union null: %s", tool.Name, b)
		}
		var s map[string]any
		if err := json.Unmarshal(b, &s); err != nil {
			t.Fatal(err)
		}
		schemas[tool.Name] = s
	}

	// Tipe sederhana dan hanya parameter wajib yang masuk required.
	props := func(tool string) map[string]any {
		t.Helper()
		p, _ := schemas[tool]["properties"].(map[string]any)
		return p
	}
	wantType := map[string]map[string]string{
		"list_directory": {"path": "string", "depth": "integer"},
		"start_search": {"pattern": "string", "path": "string", "searchType": "string",
			"filePattern": "string", "ignoreCase": "boolean", "maxResults": "integer"},
		"read_file":  {"path": "string", "offset": "integer", "length": "integer"},
		"edit_block": {"expected_replacements": "integer"},
	}
	for tool, fields := range wantType {
		for name, typ := range fields {
			prop, ok := props(tool)[name].(map[string]any)
			if !ok {
				t.Errorf("%s: parameter %s tidak ada di skema", tool, name)
				continue
			}
			if prop["type"] != typ {
				t.Errorf("%s.%s: type = %v, harapan %q", tool, name, prop["type"], typ)
			}
		}
	}
	if req, _ := schemas["start_search"]["required"].([]any); len(req) != 1 || req[0] != "pattern" {
		t.Errorf("start_search: required = %v, harapan [pattern]", req)
	}
	if req, _ := schemas["list_directory"]["required"].([]any); len(req) != 0 {
		t.Errorf("list_directory: required = %v, harapan kosong", req)
	}

	// Argumen opsional yang tidak dikirim memakai nilai bawaan; yang dikirim berlaku.
	m.raw(t, "d/e/f.txt", "x\n")
	call := func(args map[string]any) []any {
		t.Helper()
		res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "list_directory", Arguments: args})
		if err != nil {
			t.Fatalf("error protokol: %v", err)
		}
		tc, _ := res.Content[0].(*mcp.TextContent)
		var out map[string]any
		if err := json.Unmarshal([]byte(tc.Text), &out); err != nil {
			t.Fatalf("bukan JSON: %v: %s", err, tc.Text)
		}
		entries, _ := out["entries"].([]any)
		return entries
	}
	if got := call(map[string]any{}); len(got) != 2 { // bawaan depth=2: d dan d/e
		t.Errorf("tanpa argumen: %v", got)
	}
	if got := call(map[string]any{"depth": 1}); len(got) != 1 {
		t.Errorf("depth=1: %v", got)
	}
	if got := call(map[string]any{"depth": 3}); len(got) != 3 {
		t.Errorf("depth=3: %v", got)
	}
	// Sebagian client mengirim null untuk parameter yang dikosongkan: harus
	// diperlakukan sama dengan tidak dikirim (perilaku sebelum skema dirapikan).
	if got := call(map[string]any{"depth": nil, "path": nil}); len(got) != 2 {
		t.Errorf("argumen null: %v", got)
	}
}
