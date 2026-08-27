package http

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"itam/internal/domain"
	"itam/internal/events"
	"itam/internal/importer"
	"itam/internal/metadata"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"
)

// importPerm maps an import target to the permission that gates it.
func importPerm(target string) (string, bool) {
	switch target {
	case "assets":
		return "asset.write", true
	case "purchase-orders":
		return "procurement.manage", true
	case "people":
		return "hierarchy.manage", true
	case "locations":
		return "hierarchy.manage", true
	case "org-units":
		return "hierarchy.manage", true
	case "subnets":
		return "ipam.manage", true
	case "vlans":
		return "ipam.manage", true
	case "ips":
		return "ipam.manage", true
	case "software":
		return "software.manage", true
	case "licenses":
		return "software.manage", true
	}
	return "", false
}

// ---- template download ---------------------------------------------------

// handleImportTemplate returns a CSV (default) or XLSX (?format=xlsx) starter
// file for a target. For assets, ?type=<key> appends that type's attribute
// columns so the template is tailored to what the engine will validate.
func (s *Server) handleImportTemplate(w http.ResponseWriter, r *http.Request) {
	target := chi.URLParam(r, "target")
	perm, ok := importPerm(target)
	if !ok {
		writeErr(w, http.StatusNotFound, "unknown import target")
		return
	}
	if !s.rbac.Can(r.Context(), s.principal(r), perm) {
		writeErr(w, http.StatusForbidden, "missing permission: "+perm)
		return
	}

	headers, examples, notes := s.templateFor(r.Context(), target, r.URL.Query().Get("type"))

	name := strings.ReplaceAll(target, "-", "_")
	if r.URL.Query().Get("format") == "xlsx" {
		s.writeTemplateXLSX(w, name, headers, examples, notes)
		return
	}
	s.writeTemplateCSV(w, name, headers, examples, notes)
}

// templateFor returns the header row, example data rows and a help/comment line
// for a target. The comment line is prefixed with '#' and ignored on import.
func (s *Server) templateFor(ctx context.Context, target, typeKey string) (headers []string, examples [][]string, notes string) {
	switch target {
	case "assets":
		headers = []string{"asset_tag", "name", "asset_type", "serial", "location", "owner_org_unit", "purchase_cost", "purchase_date", "vendor", "notes"}
		ex := []string{"LT-0001", "Latitude 5440", firstNonEmpty(typeKey, "laptop"), "SN123", "hq", "", "1200", "2026-01-15", "Dell", ""}
		if typeKey != "" {
			if at, err := s.assetTypeByKey(ctx, typeKey); err == nil {
				if defs, err := s.meta.FieldsForType(ctx, at.ID); err == nil {
					for _, d := range defs {
						headers = append(headers, "attr:"+d.Key)
						ex = append(ex, "")
					}
				}
			}
		}
		examples = [][]string{ex}
		notes = "# asset_type/location/owner_org_unit use the KEY (slug). Dates are YYYY-MM-DD. attr:* columns are type-specific attributes. Delete these example/comment rows before importing."
	case "purchase-orders":
		headers = []string{"po_number", "vendor", "currency", "ordered_at", "expected_at", "location", "asset_type", "description", "quantity", "unit_cost"}
		examples = [][]string{
			{"PO-2026-001", "Dell", "USD", "2026-01-10", "2026-02-01", "hq", "laptop", "Latitude 5440", "10", "1200"},
			{"PO-2026-001", "Dell", "USD", "2026-01-10", "2026-02-01", "hq", "ssd", "1TB NVMe SSD", "10", "90"},
		}
		notes = "# One row per PO line; rows sharing a po_number become one PO with multiple lines. vendor is matched by name (created if new). asset_type/location use the KEY. Delete these example/comment rows before importing."
	case "people":
		headers = []string{"employee_no", "first_name", "last_name", "email", "title", "org_unit", "manager_email", "is_active"}
		examples = [][]string{{"E1001", "Ada", "Lovelace", "ada@example.com", "Engineer", "it", "", "true"}}
		notes = "# org_unit uses the org-unit KEY. manager_email links to an existing person by email. is_active = true/false. Delete these example/comment rows before importing."
	case "locations":
		headers = []string{"key", "name", "kind", "parent", "tier", "dr_role", "timezone", "address", "latitude", "longitude"}
		examples = [][]string{
			{"dc_west", "West Data Center", "datacenter", "", "tier3", "primary", "America/Los_Angeles", "123 Cloud Way", "37.77", "-122.42"},
			{"dc_west_fl1", "Floor 1", "floor", "dc_west", "", "", "", "", "", ""},
		}
		notes = "# parent uses a location KEY (blank = root). kind e.g. site/datacenter/building/floor/room. lat/long optional decimals. Rows are inserted top-down, so define parents before children. Delete these example/comment rows before importing."
	case "org-units":
		headers = []string{"key", "name", "parent"}
		examples = [][]string{
			{"it", "IT Department", ""},
			{"it_ops", "IT Operations", "it"},
		}
		notes = "# parent uses an org-unit KEY (blank = root). Define parents before children. Delete these example/comment rows before importing."
	case "subnets":
		headers = []string{"cidr", "name", "vlan_id", "gateway", "location", "description"}
		examples = [][]string{{"10.30.0.0/24", "Branch LAN", "20", "10.30.0.1", "hq", "Main branch network"}}
		notes = "# cidr like 10.0.0.0/24. vlan_id is the numeric VLAN tag (matched/created). location uses a location KEY. Delete these example/comment rows before importing."
	case "vlans":
		headers = []string{"vlan_id", "name", "location", "description"}
		examples = [][]string{{"20", "Servers", "hq", "Server VLAN"}}
		notes = "# vlan_id is 1..4094. location uses a location KEY. Delete these example/comment rows before importing."
	case "ips":
		headers = []string{"address", "status", "dns_name", "mac", "asset_tag", "description"}
		examples = [][]string{{"10.30.0.10", "allocated", "host01.example.com", "00:11:22:33:44:55", "SRV-0001", "Web server"}}
		notes = "# address is the host IP (subnet auto-detected). status = allocated/reserved/deprecated. asset_tag links to an existing asset (blank allowed). Delete these example/comment rows before importing."
	case "software":
		headers = []string{"name", "publisher", "category", "description"}
		examples = [][]string{{"Slack", "Salesforce", "application", "Team chat"}}
		notes = "# category e.g. application/os/driver/utility/firmware. name+publisher must be unique. Delete these example/comment rows before importing."
	case "licenses":
		headers = []string{"name", "software", "license_type", "seats", "vendor", "purchase_cost", "currency", "start_date", "expiry_date", "notes"}
		examples = [][]string{{"Slack Business+", "Slack", "subscription", "50", "Salesforce", "9000", "USD", "2026-01-01", "2026-12-31", ""}}
		notes = "# software matched by name (blank allowed). license_type e.g. subscription/perpetual/volume/oem. seats blank = unlimited. vendor matched by name (created if new). Dates YYYY-MM-DD. Delete these example/comment rows before importing."
	}
	return headers, examples, notes
}

func (s *Server) writeTemplateCSV(w http.ResponseWriter, name string, headers []string, examples [][]string, notes string) {
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=itam_%s_template.csv", name))
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, strings.Join(headers, ",")+"\n")
	for _, ex := range examples {
		_, _ = io.WriteString(w, csvJoin(ex)+"\n")
	}
	if notes != "" {
		_, _ = io.WriteString(w, notes+"\n")
	}
}

func (s *Server) writeTemplateXLSX(w http.ResponseWriter, name string, headers []string, examples [][]string, notes string) {
	f := excelize.NewFile()
	defer f.Close()
	sheet := f.GetSheetName(0)
	_ = f.SetSheetRow(sheet, "A1", &headers)
	row := 2
	for _, ex := range examples {
		cell := fmt.Sprintf("A%d", row)
		exCopy := ex
		_ = f.SetSheetRow(sheet, cell, &exCopy)
		row++
	}
	if notes != "" {
		_ = f.SetCellValue(sheet, fmt.Sprintf("A%d", row), notes)
	}
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=itam_%s_template.xlsx", name))
	w.WriteHeader(http.StatusOK)
	_, _ = f.WriteTo(w)
}

// ---- run (preview / commit) ----------------------------------------------

type importRowResult struct {
	Line   int               `json:"line"`
	Values map[string]string `json:"values"`
	OK     bool              `json:"ok"`
	Errors []string          `json:"errors,omitempty"`
	Result string            `json:"result,omitempty"`
}

type importResult struct {
	Target  string            `json:"target"`
	Mode    string            `json:"mode"`
	Headers []string          `json:"headers"`
	Rows    []importRowResult `json:"rows"`
	Summary map[string]any    `json:"summary"`
}

// handleImportRun validates an uploaded file and (when ?mode=commit) creates the
// records. Default mode is preview: nothing is written, but every row is checked
// against the same metadata rules used by single-record creates.
func (s *Server) handleImportRun(w http.ResponseWriter, r *http.Request) {
	target := chi.URLParam(r, "target")
	perm, ok := importPerm(target)
	if !ok {
		writeErr(w, http.StatusNotFound, "unknown import target")
		return
	}
	if !s.rbac.Can(r.Context(), s.principal(r), perm) {
		writeErr(w, http.StatusForbidden, "missing permission: "+perm)
		return
	}
	commit := r.URL.Query().Get("mode") == "commit"

	filename, data, err := readUpload(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	headers, rows, err := importer.Parse(filename, data)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if len(rows) == 0 {
		writeErr(w, http.StatusBadRequest, "no data rows found")
		return
	}

	actor := s.principal(r).UserID
	var res importResult
	switch target {
	case "assets":
		res = s.importAssets(r.Context(), headers, rows, actor, commit)
	case "purchase-orders":
		res = s.importPurchaseOrders(r.Context(), headers, rows, actor, commit)
	case "people":
		res = s.importPeople(r.Context(), headers, rows, commit)
	case "locations":
		res = s.importLocations(r.Context(), headers, rows, commit)
	case "org-units":
		res = s.importOrgUnits(r.Context(), headers, rows, commit)
	case "subnets":
		res = s.importSubnets(r.Context(), headers, rows, commit)
	case "vlans":
		res = s.importVlans(r.Context(), headers, rows, commit)
	case "ips":
		res = s.importIPs(r.Context(), headers, rows, commit)
	case "software":
		res = s.importSoftware(r.Context(), headers, rows, commit)
	case "licenses":
		res = s.importLicenses(r.Context(), headers, rows, commit)
	}
	writeJSON(w, http.StatusOK, res)
}

// readUpload accepts either a multipart "file" field or a raw request body
// (with ?filename= used to detect the format).
func readUpload(r *http.Request) (string, []byte, error) {
	ct := r.Header.Get("Content-Type")
	if strings.HasPrefix(ct, "multipart/form-data") {
		if err := r.ParseMultipartForm(25 << 20); err != nil {
			return "", nil, fmt.Errorf("invalid upload: %w", err)
		}
		f, hdr, err := r.FormFile("file")
		if err != nil {
			return "", nil, fmt.Errorf("missing 'file' field")
		}
		defer f.Close()
		data, err := io.ReadAll(io.LimitReader(f, 25<<20))
		if err != nil {
			return "", nil, err
		}
		return hdr.Filename, data, nil
	}
	data, err := io.ReadAll(io.LimitReader(r.Body, 25<<20))
	if err != nil {
		return "", nil, err
	}
	name := r.URL.Query().Get("filename")
	if name == "" {
		name = "upload.csv"
	}
	return name, data, nil
}

// ---- assets import -------------------------------------------------------

func (s *Server) importAssets(ctx context.Context, headers []string, rows []importer.Row, actor uuid.UUID, commit bool) importResult {
	res := importResult{Target: "assets", Mode: modeStr(commit), Headers: headers}
	typeCache := map[string]*domain.AssetType{}
	fieldCache := map[int64]map[string]string{} // typeID -> attrKey -> value_kind
	created := 0

	for _, row := range rows {
		rr := importRowResult{Line: row.Line, Values: row.Values}
		v := row.Values

		tag := v["asset_tag"]
		name := v["name"]
		typeKey := v["asset_type"]
		if tag == "" {
			rr.Errors = append(rr.Errors, "asset_tag is required")
		}
		if name == "" {
			rr.Errors = append(rr.Errors, "name is required")
		}

		var at *domain.AssetType
		if typeKey == "" {
			rr.Errors = append(rr.Errors, "asset_type is required")
		} else if cached, ok := typeCache[strings.ToLower(typeKey)]; ok {
			at = cached
		} else if found, err := s.assetTypeByKey(ctx, typeKey); err != nil {
			rr.Errors = append(rr.Errors, "unknown asset_type: "+typeKey)
			typeCache[strings.ToLower(typeKey)] = nil
		} else {
			at = found
			typeCache[strings.ToLower(typeKey)] = found
		}
		if at != nil && at.IsAbstract {
			rr.Errors = append(rr.Errors, "asset_type is abstract and cannot hold assets: "+typeKey)
		}

		locID, err := s.resolveLocationKey(ctx, v["location"])
		if err != nil {
			rr.Errors = append(rr.Errors, err.Error())
		}
		orgID, err := s.resolveOrgKey(ctx, v["owner_org_unit"])
		if err != nil {
			rr.Errors = append(rr.Errors, err.Error())
		}

		var cost *float64
		if raw := v["purchase_cost"]; raw != "" {
			if f, e := strconv.ParseFloat(raw, 64); e != nil {
				rr.Errors = append(rr.Errors, "purchase_cost must be a number")
			} else {
				cost = &f
			}
		}
		pdate, err := parseImportDate(v["purchase_date"])
		if err != nil {
			rr.Errors = append(rr.Errors, "purchase_date must be YYYY-MM-DD")
		}

		attrs := map[string]any{}
		if at != nil {
			kinds, ok := fieldCache[at.ID]
			if !ok {
				kinds = map[string]string{}
				if defs, e := s.meta.FieldsForType(ctx, at.ID); e == nil {
					for _, d := range defs {
						kind := "text"
						if d.DataType != nil {
							kind = d.DataType.ValueKind
						}
						kinds[strings.ToLower(d.Key)] = kind
					}
				}
				fieldCache[at.ID] = kinds
			}
			for h, val := range v {
				if !strings.HasPrefix(h, "attr:") || val == "" {
					continue
				}
				key := strings.TrimPrefix(h, "attr:")
				typed, e := coerceValue(kinds[strings.ToLower(key)], val)
				if e != nil {
					rr.Errors = append(rr.Errors, "attr "+key+": "+e.Error())
					continue
				}
				attrs[key] = typed
			}
			if len(rr.Errors) == 0 {
				if e := s.meta.ValidateAttributes(ctx, at.ID, attrs); e != nil {
					rr.Errors = append(rr.Errors, validationStrings(e)...)
				}
			}
		}

		if len(rr.Errors) > 0 {
			res.Rows = append(res.Rows, rr)
			continue
		}
		rr.OK = true
		if commit {
			id, e := s.createImportedAsset(ctx, actor, at, tag, name, v["serial"], locID, orgID, cost, pdate, v["vendor"], v["notes"], attrs)
			if e != nil {
				rr.OK = false
				rr.Errors = append(rr.Errors, e.Error())
			} else {
				created++
				rr.Result = id.String()
			}
		}
		res.Rows = append(res.Rows, rr)
	}

	res.Summary = summarize(res.Rows, commit, created)
	return res
}

// createImportedAsset mirrors handleCreateAsset: it resolves the lifecycle's
// initial state, inserts the asset, and records the created event + purchase cost.
func (s *Server) createImportedAsset(ctx context.Context, actor uuid.UUID, at *domain.AssetType, tag, name, serial string,
	locID, orgID *uuid.UUID, cost *float64, pdate *time.Time, vendor, notes string, attrs map[string]any) (uuid.UUID, error) {

	lifecycleID, initial, err := s.meta.ResolveLifecycle(ctx, at.ID)
	if err != nil {
		return uuid.Nil, err
	}
	a := &domain.Asset{
		AssetTag: tag, Serial: serial, Name: name, AssetTypeID: at.ID,
		LifecycleID: lifecycleID, LocationID: locID, OwnerOrgUnitID: orgID,
		Attributes: attrs, PurchaseCost: cost, PurchaseDate: pdate, Vendor: vendor, Notes: notes,
	}
	if initial != nil {
		a.CurrentStateID = &initial.ID
	}
	if _, err := s.db.NewInsert().Model(a).Returning("*").Exec(ctx); err != nil {
		return uuid.Nil, err
	}
	if initial != nil {
		_, _ = s.db.NewInsert().Model(&domain.LifecycleHistory{
			AssetID: a.ID, ToStateID: initial.ID, Actor: &actor, Note: "imported",
		}).Exec(ctx)
	}
	s.recordAssetEvent(ctx, a.ID, &actor, assetEvent{
		Kind: "created", Subject: events.SubjectAssetCreated, Summary: "Asset imported",
		Data: map[string]any{"asset_tag": a.AssetTag, "name": a.Name, "source": "import"},
	})
	if cost != nil && *cost != 0 {
		s.recordCost(ctx, &domain.AssetCost{
			AssetID: a.ID, Kind: "purchase", Amount: *cost, Vendor: vendor, CreatedBy: &actor,
		})
	}
	s.emit(ctx, events.SubjectAssetCreated, "asset", a.ID.String(), actor.String(),
		map[string]any{"asset_tag": a.AssetTag, "asset_type_id": a.AssetTypeID, "source": "import"})
	return a.ID, nil
}

// ---- purchase orders import ----------------------------------------------

type poGroup struct {
	number   string
	vendor   string
	currency string
	ordered  string
	expected string
	location string
	rowIdx   []int // indexes into res.Rows for this PO
	lines    []poLineInput
}

func (s *Server) importPurchaseOrders(ctx context.Context, headers []string, rows []importer.Row, actor uuid.UUID, commit bool) importResult {
	res := importResult{Target: "purchase-orders", Mode: modeStr(commit), Headers: headers}
	typeCache := map[string]*domain.AssetType{}

	order := []string{}
	groups := map[string]*poGroup{}

	for _, row := range rows {
		rr := importRowResult{Line: row.Line, Values: row.Values}
		v := row.Values
		num := v["po_number"]
		desc := v["description"]
		if num == "" {
			rr.Errors = append(rr.Errors, "po_number is required")
		}
		if desc == "" {
			rr.Errors = append(rr.Errors, "description is required")
		}

		qty := 1
		if raw := v["quantity"]; raw != "" {
			if n, e := strconv.Atoi(raw); e != nil || n < 1 {
				rr.Errors = append(rr.Errors, "quantity must be a positive whole number")
			} else {
				qty = n
			}
		}
		var unit float64
		if raw := v["unit_cost"]; raw != "" {
			if f, e := strconv.ParseFloat(raw, 64); e != nil {
				rr.Errors = append(rr.Errors, "unit_cost must be a number")
			} else {
				unit = f
			}
		}
		var typeID *int64
		if tk := v["asset_type"]; tk != "" {
			at, ok := typeCache[strings.ToLower(tk)]
			if !ok {
				if found, e := s.assetTypeByKey(ctx, tk); e != nil {
					rr.Errors = append(rr.Errors, "unknown asset_type: "+tk)
					typeCache[strings.ToLower(tk)] = nil
				} else {
					at = found
					typeCache[strings.ToLower(tk)] = found
				}
			}
			if at != nil {
				if at.IsAbstract {
					rr.Errors = append(rr.Errors, "asset_type is abstract: "+tk)
				}
				id := at.ID
				typeID = &id
			}
		}
		if _, e := s.resolveLocationKey(ctx, v["location"]); e != nil {
			rr.Errors = append(rr.Errors, e.Error())
		}

		if num != "" {
			g, ok := groups[strings.ToLower(num)]
			if !ok {
				g = &poGroup{number: num, vendor: v["vendor"], currency: v["currency"],
					ordered: v["ordered_at"], expected: v["expected_at"], location: v["location"]}
				groups[strings.ToLower(num)] = g
				order = append(order, strings.ToLower(num))
			}
			g.rowIdx = append(g.rowIdx, len(res.Rows))
			if len(rr.Errors) == 0 {
				attrs := map[string]any{}
				g.lines = append(g.lines, poLineInput{
					AssetTypeID: typeID, Description: desc, Quantity: qty, UnitCost: unit, Attributes: attrs,
				})
			}
		}
		rr.OK = len(rr.Errors) == 0
		res.Rows = append(res.Rows, rr)
	}

	created := 0
	if commit {
		for _, key := range order {
			g := groups[key]
			// skip groups that had any invalid row to avoid partial POs
			groupOK := true
			for _, idx := range g.rowIdx {
				if !res.Rows[idx].OK {
					groupOK = false
					break
				}
			}
			if !groupOK || len(g.lines) == 0 {
				continue
			}
			id, e := s.createImportedPO(ctx, actor, g)
			if e != nil {
				for _, idx := range g.rowIdx {
					res.Rows[idx].OK = false
					res.Rows[idx].Errors = append(res.Rows[idx].Errors, "PO create failed: "+e.Error())
				}
				continue
			}
			created++
			for _, idx := range g.rowIdx {
				res.Rows[idx].Result = id.String()
			}
		}
	}

	res.Summary = summarize(res.Rows, commit, created)
	res.Summary["purchase_orders"] = len(order)
	return res
}

func (s *Server) createImportedPO(ctx context.Context, actor uuid.UUID, g *poGroup) (uuid.UUID, error) {
	vendorID, err := s.resolveVendor(ctx, g.vendor)
	if err != nil {
		return uuid.Nil, err
	}
	locID, _ := s.resolveLocationKey(ctx, g.location)
	currency := g.currency
	if currency == "" {
		currency = "USD"
	}
	po := &domain.PurchaseOrder{
		PONumber: g.number, VendorID: vendorID, Status: "draft", LocationID: locID,
		Currency: currency, OrderedAt: parseDate(g.ordered), ExpectedAt: parseDate(g.expected),
		CreatedBy: &actor,
	}
	if _, err := s.db.NewInsert().Model(po).Returning("*").Exec(ctx); err != nil {
		return uuid.Nil, err
	}
	if err := s.insertPOLines(ctx, po.ID, g.lines); err != nil {
		return uuid.Nil, err
	}
	return po.ID, nil
}

// ---- shared lookups & helpers --------------------------------------------

func (s *Server) assetTypeByKey(ctx context.Context, key string) (*domain.AssetType, error) {
	at := new(domain.AssetType)
	if err := s.db.NewSelect().Model(at).Where("key = ?", key).Scan(ctx); err != nil {
		return nil, err
	}
	return at, nil
}

func (s *Server) resolveLocationKey(ctx context.Context, key string) (*uuid.UUID, error) {
	if key == "" {
		return nil, nil
	}
	loc := new(domain.Location)
	if err := s.db.NewSelect().Model(loc).Column("id").Where("key = ?", key).Scan(ctx); err == nil {
		return &loc.ID, nil
	}
	if id, ok := s.resolveLocationAlias(ctx, key); ok {
		return id, nil
	}
	loc = new(domain.Location)
	if err := s.db.NewSelect().Model(loc).Column("id").Where("lower(name) = lower(?)", key).Limit(1).Scan(ctx); err == nil {
		return &loc.ID, nil
	}
	return nil, fmt.Errorf("unknown location: %s", key)
}

// resolveLocationBestEffort resolves a location key, alias, or name; returns nil
// when nothing matches (ingest must not fail on unknown site labels).
func (s *Server) resolveLocationBestEffort(ctx context.Context, key string) *uuid.UUID {
	id, err := s.resolveLocationKey(ctx, key)
	if err != nil {
		return nil
	}
	return id
}

func (s *Server) resolveLocationAlias(ctx context.Context, alias string) (*uuid.UUID, bool) {
	alias = strings.TrimSpace(alias)
	if alias == "" {
		return nil, false
	}
	row := new(domain.LocationAlias)
	if err := s.db.NewSelect().Model(row).Where("alias = lower(?)", alias).Scan(ctx); err != nil {
		return nil, false
	}
	return &row.LocationID, true
}

func (s *Server) resolveOrgKey(ctx context.Context, key string) (*uuid.UUID, error) {
	if key == "" {
		return nil, nil
	}
	ou := new(domain.OrgUnit)
	if err := s.db.NewSelect().Model(ou).Column("id").Where("key = ?", key).Scan(ctx); err != nil {
		return nil, fmt.Errorf("unknown owner_org_unit: %s", key)
	}
	return &ou.ID, nil
}

// resolveVendor matches a vendor by name (case-insensitive) or creates one.
func (s *Server) resolveVendor(ctx context.Context, name string) (*uuid.UUID, error) {
	if strings.TrimSpace(name) == "" {
		return nil, nil
	}
	v := new(domain.Vendor)
	if err := s.db.NewSelect().Model(v).Column("id").Where("lower(name) = lower(?)", name).Limit(1).Scan(ctx); err == nil {
		return &v.ID, nil
	}
	nv := &domain.Vendor{Name: name, Key: slugify(name), IsActive: true}
	if _, err := s.db.NewInsert().Model(nv).Returning("*").Exec(ctx); err != nil {
		return nil, fmt.Errorf("create vendor %q: %w", name, err)
	}
	return &nv.ID, nil
}

func coerceValue(kind, raw string) (any, error) {
	switch kind {
	case "number":
		f, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return nil, fmt.Errorf("must be a number")
		}
		return f, nil
	case "bool":
		switch strings.ToLower(raw) {
		case "true", "yes", "y", "1":
			return true, nil
		case "false", "no", "n", "0":
			return false, nil
		}
		return nil, fmt.Errorf("must be true/false")
	default:
		return raw, nil
	}
}

func parseImportDate(s string) (*time.Time, error) {
	if s == "" {
		return nil, nil
	}
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return &t, nil
	}
	return nil, fmt.Errorf("bad date")
}

func validationStrings(err error) []string {
	if ve, ok := err.(metadata.ValidationErrors); ok {
		out := make([]string, 0, len(ve))
		for k, msg := range ve {
			out = append(out, k+": "+msg)
		}
		return out
	}
	return []string{err.Error()}
}

func summarize(rows []importRowResult, commit bool, created int) map[string]any {
	valid, invalid := 0, 0
	for _, r := range rows {
		if r.OK {
			valid++
		} else {
			invalid++
		}
	}
	m := map[string]any{"total": len(rows), "valid": valid, "invalid": invalid}
	if commit {
		m["created"] = created
	}
	return m
}

func modeStr(commit bool) string {
	if commit {
		return "commit"
	}
	return "preview"
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func csvJoin(fields []string) string {
	out := make([]string, len(fields))
	for i, f := range fields {
		if strings.ContainsAny(f, ",\"\n") {
			f = "\"" + strings.ReplaceAll(f, "\"", "\"\"") + "\""
		}
		out[i] = f
	}
	return strings.Join(out, ",")
}
