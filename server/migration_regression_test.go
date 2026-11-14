package server

import (
	"encoding/json"
	"testing"

	"antifox/agent"
	"antifox/db"
	actool "antifox/engine/tool"
)

func TestPromptAndToolUpgradePreservesCustomizations(t *testing.T) {
	dsn, _, err := db.DSN()
	if err != nil {
		t.Skipf("no database config (%v)", err)
	}
	pg, err := db.Open(dsn)
	if err != nil {
		t.Skipf("postgres unavailable (%v)", err)
	}
	t.Cleanup(func() { _ = pg.Close() })

	seedPrompts(pg)
	wireTools(pg, buildDomainReg(pg.Assets()))
	t.Cleanup(func() { agent.ToolResolve = nil })

	s := &Server{m: &Manager{pg: pg}}
	s.seedOrchestrationTools()

	// --- 0. Resolved tools include planner list_assets and mainagent report_finding ---
	probeTS := agent.NewToolSet(nil, "planner")
	plannerTools := agent.ToolResolve(t.Context(), "planner", probeTS.PlannerTools())
	var resolvedLA actool.CoreTool
	for _, pt := range plannerTools {
		if pt.Name() == "list_assets" {
			resolvedLA = pt
			break
		}
	}
	if resolvedLA == nil {
		t.Fatal("ToolResolve dropped list_assets for planner")
	}
	if props, _ := resolvedLA.InputSchema()["properties"].(map[string]any); props["q"] == nil {
		t.Fatalf("resolved planner list_assets schema missing q param: %v", props)
	}
	mainTools := agent.ToolResolve(t.Context(), "mainagent", agent.NewToolSet(nil, "human").MainAgentTools())
	hasMainRF := false
	for _, mt := range mainTools {
		if mt.Name() == "report_finding" {
			hasMainRF = true
			break
		}
	}
	if !hasMainRF {
		t.Fatal("ToolResolve dropped report_finding for mainagent")
	}

	// --- 1. Prompt upgrade: user-customized prompt stays intact ---
	plannerAg, err := pg.GetAgentByKey("planner")
	if err != nil || plannerAg == nil {
		t.Fatalf("planner agent missing: %v", err)
	}
	origPlannerPrompt, origPlannerBy, _ := pg.CurrentPromptMeta(plannerAg.ID)
	t.Cleanup(func() {
		if origPlannerPrompt != "" {
			by := origPlannerBy
			if by == "" {
				by = "system"
			}
			_, _ = pg.SavePrompt(plannerAg.ID, origPlannerPrompt, "test restore", by)
		}
		_, _ = pg.Exec(`DELETE FROM settings WHERE key LIKE 'test_prompt_upgrade_%'`)
	})

	if _, err := pg.SavePrompt(plannerAg.ID, "CUSTOM UI PLANNER PROMPT {{.Goal}}", "ui edit", "ui"); err != nil {
		t.Fatal(err)
	}
	s.reseedAgentPrompt("planner", "test_prompt_upgrade_custom_v1", "should not overwrite custom")
	gotCustom, gotBy, _ := pg.CurrentPromptMeta(plannerAg.ID)
	if gotCustom != "CUSTOM UI PLANNER PROMPT {{.Goal}}" || gotBy != "ui" {
		t.Fatalf("custom prompt was overwritten: text=%q by=%q", gotCustom, gotBy)
	}

	// --- 2. Prompt upgrade: system-owned stale prompt upgrades cleanly ---
	if _, err := pg.SavePrompt(plannerAg.ID, "STALE SYSTEM PLANNER PROMPT {{.Goal}}", "old system", "system"); err != nil {
		t.Fatal(err)
	}
	s.reseedAgentPrompt("planner", "test_prompt_upgrade_sys_v1", "upgrade stale system prompt")
	gotUpgraded, gotUpBy, _ := pg.CurrentPromptMeta(plannerAg.ID)
	wantPlanner := agent.BuiltinPromptSeeds()["planner"]
	if gotUpgraded != wantPlanner || gotUpBy != "system" {
		t.Fatalf("system prompt did not upgrade cleanly: len=%d want=%d by=%q", len(gotUpgraded), len(wantPlanner), gotUpBy)
	}

	// --- 3. Tool migration scope: updating list_assets must NOT reset customized record_fact ---
	origRF, err := pg.GetTool("record_fact")
	if err != nil || origRF == nil {
		t.Fatalf("record_fact missing: %v", err)
	}
	origLA, err := pg.GetTool("list_assets")
	if err != nil || origLA == nil {
		t.Fatalf("list_assets missing: %v", err)
	}
	t.Cleanup(func() {
		_ = pg.UpsertToolForce(origRF.Key, origRF.Description, origRF.Schema, mustJSON(origRF.Agents))
		_ = pg.UpdateTool(origRF.Key, origRF.Description, origRF.Schema, mustJSON(origRF.Agents), origRF.Enabled)
		_ = pg.UpsertToolForce(origLA.Key, origLA.Description, origLA.Schema, mustJSON(origLA.Agents))
		_ = pg.UpdateTool(origLA.Key, origLA.Description, origLA.Schema, mustJSON(origLA.Agents), origLA.Enabled)
		_ = pg.SetSetting("tool_schema_refresh_list_assets_v2", "true")
		_ = pg.SetSetting("planner_list_assets_v1", "true")
	})
	// The real v2.1.1 stock definition, from the recorded shipped versions.
	stock, err := loadStockToolDefs("list_assets")
	if err != nil {
		t.Fatal(err)
	}
	var v211 stockToolDef
	for _, d := range stock {
		if d.Version == "fork v2.0.3-v2.1.1" {
			v211 = d
		}
	}
	if v211.Description == "" {
		t.Fatal("v2.1.1 stock definition missing from stockdefs")
	}
	// The operator had changed only the limit default; every other part is stock.
	v211Schema := jsonNormalize(v211.Schema).(map[string]any)
	v211Schema["properties"].(map[string]any)["limit"].(map[string]any)["default"] = float64(25)
	oldLASchema, _ := json.Marshal(v211Schema)

	if err := pg.UpdateTool("record_fact", "CUSTOM RECORD FACT DESC", json.RawMessage(`{"type":"object","properties":{"summary":{"type":"string","default":"x"}}}`), mustJSON([]string{"worker"}), false); err != nil {
		t.Fatal(err)
	}
	// Simulate an un-migrated v2.1.1 list_assets row without q or planner binding.
	if err := pg.UpdateTool("list_assets", v211.Description, oldLASchema, mustJSON([]string{"mainagent", "worker"}), true); err != nil {
		t.Fatal(err)
	}
	_, _ = pg.Exec(`DELETE FROM settings WHERE key IN ('tool_schema_refresh_list_assets_v2','planner_list_assets_v1')`)

	s.seedOrchestrationTools()

	rfAfter, _ := pg.GetTool("record_fact")
	if rfAfter == nil || rfAfter.Description != "CUSTOM RECORD FACT DESC" || rfAfter.Enabled {
		t.Fatalf("unrelated customized tool record_fact was altered by migration: %+v", rfAfter)
	}
	laAfter, _ := pg.GetTool("list_assets")
	if laAfter == nil || laAfter.Description == v211.Description {
		t.Fatalf("list_assets stock description did not upgrade: %+v", laAfter)
	}
	var laProps map[string]any
	var laMap map[string]any
	_ = json.Unmarshal(laAfter.Schema, &laMap)
	laProps, _ = laMap["properties"].(map[string]any)
	if _, hasQ := laProps["q"]; !hasQ {
		t.Fatalf("upgraded list_assets schema missing q param: %s", string(laAfter.Schema))
	}
	if lim, ok := laProps["limit"].(map[string]any); !ok || lim["default"] != float64(25) {
		t.Fatalf("upgraded list_assets dropped user limit default=25: %v", laProps["limit"])
	}
	hasPlanner := false
	for _, a := range laAfter.Agents {
		if a == "planner" {
			hasPlanner = true
			break
		}
	}
	if !hasPlanner {
		t.Fatalf("upgraded list_assets agents missing planner: %v", laAfter.Agents)
	}

	// --- 4. User-customized list_assets descriptions are ALSO preserved, including
	// an edit that keeps the stock opening words (the old prefix check overwrote it).
	for _, custom := range []string{"CUSTOM USER LIST ASSETS", "Search for asset bank. Operator note: prefer dsl."} {
		if err := pg.UpdateTool("list_assets", custom, oldLASchema, mustJSON([]string{"worker"}), true); err != nil {
			t.Fatal(err)
		}
		_, _ = pg.Exec(`DELETE FROM settings WHERE key='tool_schema_refresh_list_assets_v2'`)
		s.refreshToolListAssets()
		laCustom, _ := pg.GetTool("list_assets")
		if laCustom == nil || laCustom.Description != custom {
			t.Fatalf("customized list_assets description %q was overwritten: %+v", custom, laCustom)
		}
		var m map[string]any
		_ = json.Unmarshal(laCustom.Schema, &m)
		if _, ok := m["properties"].(map[string]any)["q"]; !ok {
			t.Fatalf("customized description must not block the schema upgrade: %s", laCustom.Schema)
		}
	}
}
