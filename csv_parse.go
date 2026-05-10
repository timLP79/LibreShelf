// Copyright (c) 2026 Tim Palacios. All rights reserved.
// Licensed under the LibreShelf License (see LICENSE in the repo root).

package main

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
)

var standardColumnAliases = map[string]string{
	"name":              "name",
	"fullname":          "name",
	"patron":            "name",
	"patronname":        "name",
	"email":             "email",
	"emailaddress":      "email",
	"mail":              "email",
	"phone":             "phone",
	"phonenumber":       "phone",
	"telephone":         "phone",
	"cell":              "phone",
	"mobile":            "phone",
	"idoc":              "external_id",
	"idocnumber":        "external_id",
	"inmate":            "external_id",
	"inmateid":          "external_id",
	"inmatenumber":      "external_id",
	"librarycard":       "external_id",
	"librarycardnumber": "external_id",
	"cardnumber":        "external_id",
	"studentid":         "external_id",
	"patronid":          "external_id",
	"externalid":        "external_id",
}

type ParsedPatronCSV struct {
	Rows            []ParsedPatronRow
	StandardColumns []string
	MetadataColumns []string
}

type ParsedPatronRow struct {
	LineNumber int
	Name       string
	Email      string
	Phone      string
	ExternalID string
	Metadata   map[string]string
	Insert     bool
	SkipReason string
}

func parsePatronCSV(r io.Reader) (*ParsedPatronCSV, error) {
	br := bufio.NewReader(r)
	// Strip Excel UTF-8 BOM so the first header doesn't carry it.
	if peek, err := br.Peek(3); err == nil && bytes.Equal(peek, []byte{0xEF, 0xBB, 0xBF}) {
		_, _ = br.Discard(3)
	}

	reader := csv.NewReader(br)
	reader.FieldsPerRecord = -1

	headers, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("read header row: %w", err)
	}

	canonicalByCol := make(map[int]string)
	metaHeaderByCol := make(map[int]string)
	standardSet := make(map[string]bool)
	var metaList []string

	for i, h := range headers {
		clean := strings.TrimSpace(h)
		if clean == "" {
			continue
		}
		normalized := normalizeCSVHeader(clean)
		if canonical, ok := standardColumnAliases[normalized]; ok {
			canonicalByCol[i] = canonical
			standardSet[canonical] = true
		} else {
			metaHeaderByCol[i] = clean
			metaList = append(metaList, clean)
		}
	}

	var rows []ParsedPatronRow
	lineNumber := 1
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		lineNumber++
		if err != nil {
			rows = append(rows, ParsedPatronRow{
				LineNumber: lineNumber,
				Insert:     false,
				SkipReason: fmt.Sprintf("Malformed CSV row: %v", err),
			})
			continue
		}

		row := ParsedPatronRow{
			LineNumber: lineNumber,
			Metadata:   make(map[string]string),
		}
		for i, value := range record {
			value = strings.TrimSpace(value)
			if value == "" {
				continue
			}
			if canonical, ok := canonicalByCol[i]; ok {
				switch canonical {
				case "name":
					row.Name = value
				case "email":
					row.Email = value
				case "phone":
					row.Phone = value
				case "external_id":
					row.ExternalID = value
				}
			} else if origHeader, ok := metaHeaderByCol[i]; ok {
				row.Metadata[origHeader] = value
			}
		}

		if row.Name == "" {
			row.Insert = false
			row.SkipReason = "Name is required."
		} else {
			row.Insert = true
		}

		rows = append(rows, row)
	}

	standardList := make([]string, 0, len(standardSet))
	for s := range standardSet {
		standardList = append(standardList, s)
	}
	sort.Strings(standardList)

	return &ParsedPatronCSV{
		Rows:            rows,
		StandardColumns: standardList,
		MetadataColumns: metaList,
	}, nil
}

func normalizeCSVHeader(h string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(h) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func dedupePatronCSV(parsed *ParsedPatronCSV, dm *DatabaseManager) error {
	for i := range parsed.Rows {
		row := &parsed.Rows[i]
		if !row.Insert {
			continue
		}
		if row.ExternalID != "" {
			existing, err := dm.FindPatronByExternalID(row.ExternalID)
			if err != nil {
				return fmt.Errorf("dedup external_id row %d: %w", row.LineNumber, err)
			}
			if existing != nil {
				row.Insert = false
				row.SkipReason = fmt.Sprintf("External ID %q matches existing patron #%d.", row.ExternalID, existing.ID)
				continue
			}
		}
		if row.Email != "" {
			existing, err := dm.FindPatronByEmail(row.Email)
			if err != nil {
				return fmt.Errorf("dedup email row %d: %w", row.LineNumber, err)
			}
			if existing != nil {
				row.Insert = false
				row.SkipReason = fmt.Sprintf("Email %q matches existing patron #%d.", row.Email, existing.ID)
				continue
			}
		}
	}
	return nil
}

func buildMetadataJSON(externalID string, extras map[string]string) (string, error) {
	if externalID == "" && len(extras) == 0 {
		return "", nil
	}
	combined := make(map[string]string, len(extras)+1)
	for k, v := range extras {
		combined[k] = v
	}
	if externalID != "" {
		combined["external_id"] = externalID
	}
	out, err := json.Marshal(combined)
	if err != nil {
		return "", err
	}
	return string(out), nil
}
