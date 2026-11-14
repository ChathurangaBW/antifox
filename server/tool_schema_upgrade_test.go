package server

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"antifox/agent"
	"antifox/db"
)

func listAssetsSeed(t *testing.T) agent.ToolSeed {
	t.Helper()
	for _, s := range agent.BuiltinToolSeeds() {
		if s.Key == "list_assets" {
			return s
		}
	}
	t.Fatal("list_assets seed missing")
	return agent.ToolSeed{}
}

func stockVersion(t *testing.T, version string) stockToolDef {
	t.Helper()
	defs, err := loadStockToolDefs("list_assets")
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range defs {
		if d.Version == version {
			return d
		}
	}
	t.Fatalf("stock version %q missing", version)
	return stockToolDef{}
}

func cloneSchema(m map[string]any) map[string]any {
	return jsonNormalize(m).(map[string]any)
}

func props(m map[string]any) map[string]any {
	p, _ := m["properties"].(map[string]any)
	return p
}

// TestStockDefsMatchCurrentCode: the newest recorded shipped definition must be
// exactly what the code ships, or a stock row would be misread as customized.
func TestStockDefsMatchCurrentCode(t *testing.T) {
	seed := listAssetsSeed(t)
	cur := stockVersion(t, "fork v2.1.2")
	if cur.Description != seed.Desc || !reflect.DeepEqual(cloneSchema(cur.Schema), cloneSchema(seed.Schema)) {
		t.Fatal("stockdefs/list_assets.json 'fork v2.1.2' no longer matches the code; record the new shipped version")
	}
}

func TestMergeToolDefinition(t *testing.T) {
	seed := listAssetsSeed(t)
	stock, _ := loadStockToolDefs("list_assets")
	seedSchema := cloneSchema(seed.Schema)
	for _, version := range []string{"upstream v0.1.0-v2.0.3 (zh)", "fork v2.0.3-v2.1.1"} {
		old := stockVersion(t, version)

		t.Run("stock "+version+" upgrades fully", func(t *testing.T) {
			up := mergeToolDefinition(old.Description, cloneSchema(old.Schema), seed.Desc, seed.Schema, stock)
			if up.Description != seed.Desc || !reflect.DeepEqual(up.Schema, seedSchema) || len(up.Kept) != 0 || !up.Changed {
				t.Fatalf("got desc=%q kept=%v changed=%v schema-equal=%v", up.Description, up.Kept, up.Changed, reflect.DeepEqual(up.Schema, seedSchema))
			}
		})

		t.Run("prefixed custom description "+version+" is kept", func(t *testing.T) {
			custom := old.Description[:20] + " (operator addition)"
			up := mergeToolDefinition(custom, cloneSchema(old.Schema), seed.Desc, seed.Schema, stock)
			if up.Description != custom {
				t.Fatalf("custom description replaced with %q", up.Description)
			}
			if _, ok := props(up.Schema)["q"]; !ok {
				t.Fatal("new properties must still be added")
			}
		})

		t.Run("schema-only edits "+version+" are kept", func(t *testing.T) {
			s := cloneSchema(old.Schema)
			props(s)["dsl"].(map[string]any)["description"] = "operator wording"
			props(s)["limit"].(map[string]any)["maximum"] = float64(50) // a constraint
			props(s)["note"] = map[string]any{"type": "string"}         // operator-added property
			s["required"] = []any{"dsl"}                                // operator-added keyword
			up := mergeToolDefinition(old.Description, s, seed.Desc, seed.Schema, stock)
			p := props(up.Schema)
			if p["dsl"].(map[string]any)["description"] != "operator wording" {
				t.Fatal("edited property description lost")
			}
			if p["limit"].(map[string]any)["maximum"] != float64(50) {
				t.Fatal("added constraint lost")
			}
			if _, ok := p["note"]; !ok {
				t.Fatal("operator-added property lost")
			}
			if !reflect.DeepEqual(up.Schema["required"], []any{"dsl"}) {
				t.Fatal("operator-added keyword lost")
			}
			if up.Description != seed.Desc {
				t.Fatal("stock description should still upgrade")
			}
			for _, name := range []string{"q", "company_id", "task_id"} {
				if _, ok := p[name]; !ok {
					t.Fatalf("new property %s not added", name)
				}
			}
			// Untouched stock properties take the current wording.
			if !reflect.DeepEqual(p["offset"], props(seedSchema)["offset"]) {
				t.Fatal("untouched stock property not upgraded")
			}
		})
	}

	t.Run("current description with stale schema still upgrades", func(t *testing.T) {
		old := stockVersion(t, "fork v2.0.3-v2.1.1")
		up := mergeToolDefinition(seed.Desc, cloneSchema(old.Schema), seed.Desc, seed.Schema, stock)
		if !up.Changed || !reflect.DeepEqual(up.Schema, seedSchema) {
			t.Fatal("stale schema behind a current description was not upgraded")
		}
	})

	t.Run("already current is a no-op", func(t *testing.T) {
		up := mergeToolDefinition(seed.Desc, cloneSchema(seed.Schema), seed.Desc, seed.Schema, stock)
		if up.Changed {
			t.Fatal("current definition reported as changed")
		}
	})

	t.Run("defaults on stock properties are kept", func(t *testing.T) {
		old := stockVersion(t, "fork v2.0.3-v2.1.1")
		s := cloneSchema(old.Schema)
		props(s)["limit"].(map[string]any)["default"] = float64(25)
		up := mergeToolDefinition(old.Description, s, seed.Desc, seed.Schema, stock)
		if props(up.Schema)["limit"].(map[string]any)["default"] != float64(25) {
			t.Fatal("operator default lost")
		}
	})
}

// TestUpgradeBuiltinToolDefinitionDB covers the database paths: stale schema
// behind a current description, a missing row, a concurrent edit, repeat runs,
// and that agent bindings and the enabled flag are never touched.
func TestUpgradeBuiltinToolDefinitionDB(t *testing.T) {
	m, err := NewManager(t.TempDir(), "")
	if err != nil || m.pg == nil {
		t.Skip("postgres unavailable — skipping")
	}
	pg := m.pg
	wireTools(pg, buildDomainReg(pg.Assets()))
	t.Cleanup(func() { agent.ToolResolve = nil })
	s := &Server{m: m}
	orig, err := pg.GetTool("list_assets")
	if err != nil || orig == nil {
		t.Fatalf("list_assets not seeded: %v", err)
	}
	t.Cleanup(func() {
		_ = pg.UpsertToolForce(orig.Key, orig.Description, orig.Schema, mustJSON(orig.Agents))
		_ = pg.UpdateTool(orig.Key, orig.Description, orig.Schema, mustJSON(orig.Agents), orig.Enabled)
		_ = pg.SetSetting("tool_schema_refresh_list_assets_v2", "true")
	})
	seed := listAssetsSeed(t)
	old := stockVersion(t, "fork v2.0.3-v2.1.1")
	oldSchema, _ := json.Marshal(old.Schema)
	const flag = "tool_schema_refresh_list_assets_v2"

	// D2: current description, stale schema, disabled, custom bindings.
	if err := pg.UpdateTool("list_assets", seed.Desc, oldSchema, mustJSON([]string{"worker"}), false); err != nil {
		t.Fatal(err)
	}
	_, _ = pg.Exec(`DELETE FROM settings WHERE key=$1`, flag)
	s.refreshToolListAssets()
	got, _ := pg.GetTool("list_assets")
	var sch map[string]any
	_ = json.Unmarshal(got.Schema, &sch)
	if missing := missingSeedProperties(sch, seed.Schema); len(missing) > 0 {
		t.Fatalf("stale schema not upgraded, missing %v", missing)
	}
	if got.Enabled || !reflect.DeepEqual(got.Agents, []string{"worker"}) {
		t.Fatalf("migration touched enabled/bindings: enabled=%v agents=%v", got.Enabled, got.Agents)
	}
	if !s.migrationDone(flag) {
		t.Fatal("flag not recorded after a verified upgrade")
	}

	// Repeat run is a no-op.
	before := got.Schema
	_, _ = pg.Exec(`DELETE FROM settings WHERE key=$1`, flag)
	s.refreshToolListAssets()
	again, _ := pg.GetTool("list_assets")
	var a, b any
	_ = json.Unmarshal(before, &a)
	_ = json.Unmarshal(again.Schema, &b)
	if !reflect.DeepEqual(a, b) {
		t.Fatal("second run changed an already-current row")
	}

	// Concurrent edit: the conditional write must refuse when the row changed.
	ok, err := pg.UpdateToolDefinitionIfUnchanged("list_assets", "not what is stored", again.Schema, "x", again.Schema)
	if err != nil || ok {
		t.Fatalf("conditional write applied over a changed row: ok=%v err=%v", ok, err)
	}

	// Missing row: no flag, so it retries once the tool is seeded.
	_, _ = pg.Exec(`DELETE FROM tools WHERE key='list_assets'`)
	_, _ = pg.Exec(`DELETE FROM settings WHERE key=$1`, flag)
	s.refreshToolListAssets()
	if s.migrationDone(flag) {
		t.Fatal("flag recorded although the tool row does not exist")
	}
	if err := s.upgradeBuiltinToolDefinition("list_assets"); err == nil || !strings.Contains(err.Error(), "not seeded") {
		t.Fatalf("missing row: got %v", err)
	}
}

// TestUpgradeWithFailingDatabase: when reads and writes fail (closed pool), the
// upgrade reports an error, records no flag and does not panic.
func TestUpgradeWithFailingDatabase(t *testing.T) {
	dsn, _, err := db.DSN()
	if err != nil {
		t.Skip("no database config — skipping")
	}
	pg, err := db.Open(dsn)
	if err != nil {
		t.Skip("postgres unavailable — skipping")
	}
	_ = pg.Close()
	s := &Server{m: &Manager{pg: pg}}
	if err := s.upgradeBuiltinToolDefinition("list_assets"); err == nil {
		t.Fatal("upgrade on a failing database reported success")
	}
	s.refreshToolListAssets() // must log and return, not panic
	s.markMigrationDone("x")  // must log the failed write, not panic
}
