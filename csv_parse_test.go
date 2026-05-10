// Copyright (c) 2026 Tim Palacios. All rights reserved.
// Licensed under the LibreShelf License (see LICENSE in the repo root).

package main

import (
	"strings"
	"testing"
)

func TestNormalizeCSVHeader(t *testing.T) {
	cases := map[string]string{
		"Name":            "name",
		"Email Address":   "emailaddress",
		"E-mail":          "email",
		"IDOC Number":     "idocnumber",
		"Library Card #":  "librarycard",
		"Cell Block":      "cellblock",
		"Patron / Member": "patronmember",
		"  Padded  ":      "padded",
		"":                "",
		"123abc":          "123abc",
	}
	for in, want := range cases {
		got := normalizeCSVHeader(in)
		if got != want {
			t.Errorf("normalizeCSVHeader(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParsePatronCSV_StandardColumns(t *testing.T) {
	in := strings.NewReader("Name,Email,Phone\nJohn Smith,john@example.com,555-0101\nJane Doe,,555-0102\n")
	parsed, err := parsePatronCSV(in)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(parsed.Rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(parsed.Rows))
	}
	if parsed.Rows[0].Name != "John Smith" || parsed.Rows[0].Email != "john@example.com" || parsed.Rows[0].Phone != "555-0101" {
		t.Errorf("row 0 unexpected: %+v", parsed.Rows[0])
	}
	if parsed.Rows[1].Email != "" {
		t.Errorf("row 1 email should be empty, got %q", parsed.Rows[1].Email)
	}
	if !parsed.Rows[0].Insert || !parsed.Rows[1].Insert {
		t.Errorf("both rows should be Insert=true, got %v / %v", parsed.Rows[0].Insert, parsed.Rows[1].Insert)
	}
	wantStd := map[string]bool{"name": true, "email": true, "phone": true}
	if len(parsed.StandardColumns) != 3 {
		t.Errorf("expected 3 standard columns, got %v", parsed.StandardColumns)
	}
	for _, col := range parsed.StandardColumns {
		if !wantStd[col] {
			t.Errorf("unexpected standard column: %q", col)
		}
	}
}

func TestParsePatronCSV_StripsBOM(t *testing.T) {
	bomCSV := "\xEF\xBB\xBFName,Email\nAlice,alice@example.com\n"
	parsed, err := parsePatronCSV(strings.NewReader(bomCSV))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(parsed.Rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(parsed.Rows))
	}
	if parsed.Rows[0].Name != "Alice" {
		t.Errorf("BOM not stripped: got Name=%q", parsed.Rows[0].Name)
	}
	for _, std := range parsed.StandardColumns {
		if std != "name" && std != "email" {
			t.Errorf("unexpected standard column %q (BOM may have leaked into header)", std)
		}
	}
}

func TestParsePatronCSV_AliasMapping_IDOCNumberToExternalID(t *testing.T) {
	in := strings.NewReader("Full Name,IDOC Number,Cell Block\nJohn Smith,IDOC123,B-12\n")
	parsed, err := parsePatronCSV(in)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	row := parsed.Rows[0]
	if row.Name != "John Smith" {
		t.Errorf("Full Name -> Name failed: got %q", row.Name)
	}
	if row.ExternalID != "IDOC123" {
		t.Errorf("IDOC Number -> external_id failed: got %q", row.ExternalID)
	}
	if row.Metadata["Cell Block"] != "B-12" {
		t.Errorf("Cell Block -> metadata failed: got %v", row.Metadata)
	}
}

func TestParsePatronCSV_AliasMapping_VariousIDColumns(t *testing.T) {
	cases := []string{
		"inmate id", "inmate number", "library card", "card number",
		"student_id", "patron_id", "external_id",
	}
	for _, header := range cases {
		csv := "Name," + header + "\nAlice,X123\n"
		parsed, err := parsePatronCSV(strings.NewReader(csv))
		if err != nil {
			t.Errorf("header %q parse: %v", header, err)
			continue
		}
		if parsed.Rows[0].ExternalID != "X123" {
			t.Errorf("header %q did not map to external_id; ExternalID=%q metadata=%v", header, parsed.Rows[0].ExternalID, parsed.Rows[0].Metadata)
		}
	}
}

func TestParsePatronCSV_EmptyNameSkipped(t *testing.T) {
	in := strings.NewReader("Name,Email\n,nobody@example.com\nValid,valid@example.com\n")
	parsed, err := parsePatronCSV(in)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(parsed.Rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(parsed.Rows))
	}
	if parsed.Rows[0].Insert {
		t.Errorf("empty-name row should have Insert=false")
	}
	if parsed.Rows[0].SkipReason == "" {
		t.Errorf("empty-name row should have SkipReason populated")
	}
	if !parsed.Rows[1].Insert {
		t.Errorf("valid row should have Insert=true")
	}
}

func TestParsePatronCSV_ExtrasGoToMetadata(t *testing.T) {
	in := strings.NewReader("Name,Cell Block,Security Level\nJohn,B-12,Medium\n")
	parsed, err := parsePatronCSV(in)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got := parsed.Rows[0].Metadata["Cell Block"]; got != "B-12" {
		t.Errorf("metadata[Cell Block] = %q, want %q", got, "B-12")
	}
	if got := parsed.Rows[0].Metadata["Security Level"]; got != "Medium" {
		t.Errorf("metadata[Security Level] = %q, want %q", got, "Medium")
	}
	if len(parsed.MetadataColumns) != 2 {
		t.Errorf("expected 2 metadata columns, got %v", parsed.MetadataColumns)
	}
}

func TestBuildMetadataJSON(t *testing.T) {
	t.Run("EmptyReturnsEmpty", func(t *testing.T) {
		out, err := buildMetadataJSON("", nil)
		if err != nil {
			t.Fatalf("buildMetadataJSON: %v", err)
		}
		if out != "" {
			t.Errorf("expected empty string, got %q", out)
		}
	})
	t.Run("ExternalIDOnly", func(t *testing.T) {
		out, err := buildMetadataJSON("IDOC123", nil)
		if err != nil {
			t.Fatalf("buildMetadataJSON: %v", err)
		}
		if !strings.Contains(out, `"external_id":"IDOC123"`) {
			t.Errorf("missing external_id in %q", out)
		}
	})
	t.Run("ExternalIDPlusExtras", func(t *testing.T) {
		out, err := buildMetadataJSON("IDOC123", map[string]string{"Cell Block": "B-12"})
		if err != nil {
			t.Fatalf("buildMetadataJSON: %v", err)
		}
		if !strings.Contains(out, `"external_id":"IDOC123"`) {
			t.Errorf("missing external_id in %q", out)
		}
		if !strings.Contains(out, `"Cell Block":"B-12"`) {
			t.Errorf("missing Cell Block in %q", out)
		}
	})
	t.Run("ExtrasOnly", func(t *testing.T) {
		out, err := buildMetadataJSON("", map[string]string{"Notes": "n/a"})
		if err != nil {
			t.Fatalf("buildMetadataJSON: %v", err)
		}
		if !strings.Contains(out, `"Notes":"n/a"`) {
			t.Errorf("missing Notes in %q", out)
		}
	})
}

func TestDedupePatronCSV(t *testing.T) {
	dm := setupTestDB(t)
	if _, err := dm.CreatePatronNoLogin("Existing", "existing@example.com", "", `{"external_id":"DUPE-EXT"}`); err != nil {
		t.Fatalf("seed: %v", err)
	}

	in := strings.NewReader("Name,Email,IDOC Number\nNew Patron,new@example.com,NEW-EXT\nDup Ext,extdup@example.com,DUPE-EXT\nDup Email,existing@example.com,OTHER-EXT\n")
	parsed, err := parsePatronCSV(in)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if err := dedupePatronCSV(parsed, dm); err != nil {
		t.Fatalf("dedupe: %v", err)
	}

	if !parsed.Rows[0].Insert {
		t.Errorf("row 0 (new) should remain Insert=true, reason=%q", parsed.Rows[0].SkipReason)
	}
	if parsed.Rows[1].Insert {
		t.Errorf("row 1 should be marked Insert=false (dup external_id)")
	}
	if !strings.Contains(parsed.Rows[1].SkipReason, "External ID") {
		t.Errorf("row 1 SkipReason should mention External ID, got %q", parsed.Rows[1].SkipReason)
	}
	if parsed.Rows[2].Insert {
		t.Errorf("row 2 should be marked Insert=false (dup email)")
	}
	if !strings.Contains(parsed.Rows[2].SkipReason, "Email") {
		t.Errorf("row 2 SkipReason should mention Email, got %q", parsed.Rows[2].SkipReason)
	}
}
