package http

import (
	"context"
	"net"
	"strconv"
	"strings"

	"itam/internal/domain"
	"itam/internal/importer"

	"github.com/google/uuid"
)

// ---- people --------------------------------------------------------------

func (s *Server) importPeople(ctx context.Context, headers []string, rows []importer.Row, commit bool) importResult {
	res := importResult{Target: "people", Mode: modeStr(commit), Headers: headers}
	seenEmail := map[string]bool{}
	created := 0

	for _, row := range rows {
		rr := importRowResult{Line: row.Line, Values: row.Values}
		v := row.Values
		first := strings.TrimSpace(v["first_name"])
		last := strings.TrimSpace(v["last_name"])
		if first == "" {
			rr.Errors = append(rr.Errors, "first_name is required")
		}
		if last == "" {
			rr.Errors = append(rr.Errors, "last_name is required")
		}

		orgID, err := s.resolveOrgKey(ctx, v["org_unit"])
		if err != nil {
			rr.Errors = append(rr.Errors, err.Error())
		}

		var managerID *uuid.UUID
		if me := strings.TrimSpace(v["manager_email"]); me != "" {
			if id, ok := s.personByEmail(ctx, me); ok {
				managerID = id
			} else if seenEmail[strings.ToLower(me)] {
				// defined earlier in this file; will resolve on commit
			} else {
				rr.Errors = append(rr.Errors, "unknown manager_email: "+me)
			}
		}

		active := true
		if raw := v["is_active"]; raw != "" {
			if b, e := coerceValue("bool", raw); e != nil {
				rr.Errors = append(rr.Errors, "is_active must be true/false")
			} else {
				active = b.(bool)
			}
		}

		if email := strings.TrimSpace(v["email"]); email != "" {
			seenEmail[strings.ToLower(email)] = true
		}

		if len(rr.Errors) > 0 {
			res.Rows = append(res.Rows, rr)
			continue
		}
		rr.OK = true
		if commit {
			// Resolve manager again in case it was defined earlier in this file.
			if managerID == nil {
				if me := strings.TrimSpace(v["manager_email"]); me != "" {
					managerID, _ = s.personByEmail(ctx, me)
				}
			}
			p := &domain.Person{
				EmployeeNo: v["employee_no"], FirstName: first, LastName: last,
				Email: v["email"], Title: v["title"], OrgUnitID: orgID, ManagerID: managerID,
				IsActive: active,
			}
			if _, e := s.db.NewInsert().Model(p).Returning("*").Exec(ctx); e != nil {
				rr.OK = false
				rr.Errors = append(rr.Errors, e.Error())
			} else {
				created++
				rr.Result = p.ID.String()
			}
		}
		res.Rows = append(res.Rows, rr)
	}
	res.Summary = summarize(res.Rows, commit, created)
	return res
}

func (s *Server) personByEmail(ctx context.Context, email string) (*uuid.UUID, bool) {
	p := new(domain.Person)
	if err := s.db.NewSelect().Model(p).Column("id").
		Where("lower(email) = lower(?)", email).Limit(1).Scan(ctx); err != nil {
		return nil, false
	}
	return &p.ID, true
}

// ---- locations -----------------------------------------------------------

func (s *Server) importLocations(ctx context.Context, headers []string, rows []importer.Row, commit bool) importResult {
	res := importResult{Target: "locations", Mode: modeStr(commit), Headers: headers}
	seenKey := map[string]bool{}
	created := 0

	for _, row := range rows {
		rr := importRowResult{Line: row.Line, Values: row.Values}
		v := row.Values

		key, err := ltreeKey(v["key"])
		if err != nil {
			rr.Errors = append(rr.Errors, "key: "+err.Error())
		}
		if strings.TrimSpace(v["name"]) == "" {
			rr.Errors = append(rr.Errors, "name is required")
		}

		var parentID *uuid.UUID
		if pk := strings.TrimSpace(v["parent"]); pk != "" {
			if id, e := s.resolveLocationKey(ctx, pk); e == nil {
				parentID = id
			} else if !seenKey[strings.ToLower(pk)] {
				rr.Errors = append(rr.Errors, "unknown parent: "+pk)
			}
		}

		lat, latErr := parseFloatPtr(v["latitude"])
		lng, lngErr := parseFloatPtr(v["longitude"])
		if latErr != nil || lngErr != nil {
			rr.Errors = append(rr.Errors, "latitude/longitude must be numbers")
		}

		if key != "" {
			seenKey[strings.ToLower(key)] = true
		}

		if len(rr.Errors) > 0 {
			res.Rows = append(res.Rows, rr)
			continue
		}
		rr.OK = true
		if commit {
			if parentID == nil {
				if pk := strings.TrimSpace(v["parent"]); pk != "" {
					parentID, _ = s.resolveLocationKey(ctx, pk)
				}
			}
			path, e := s.childPath(ctx, "core.locations", parentID, key)
			if e != nil {
				rr.OK = false
				rr.Errors = append(rr.Errors, e.Error())
				res.Rows = append(res.Rows, rr)
				continue
			}
			kind := firstNonEmpty(v["kind"], "site")
			loc := &domain.Location{
				Key: key, Name: v["name"], Kind: kind, ParentID: parentID, Path: path,
				DRRole: v["dr_role"], Tier: v["tier"], Timezone: v["timezone"], Address: v["address"],
				Attributes: map[string]any{},
			}
			if _, e := s.db.NewInsert().Model(loc).Returning("*").Exec(ctx); e != nil {
				rr.OK = false
				rr.Errors = append(rr.Errors, e.Error())
			} else {
				_ = s.setGeog(ctx, loc.ID, lat, lng)
				created++
				rr.Result = loc.ID.String()
			}
		}
		res.Rows = append(res.Rows, rr)
	}
	res.Summary = summarize(res.Rows, commit, created)
	return res
}

// ---- org units -----------------------------------------------------------

func (s *Server) importOrgUnits(ctx context.Context, headers []string, rows []importer.Row, commit bool) importResult {
	res := importResult{Target: "org-units", Mode: modeStr(commit), Headers: headers}
	seenKey := map[string]bool{}
	created := 0

	for _, row := range rows {
		rr := importRowResult{Line: row.Line, Values: row.Values}
		v := row.Values
		key, err := ltreeKey(v["key"])
		if err != nil {
			rr.Errors = append(rr.Errors, "key: "+err.Error())
		}
		if strings.TrimSpace(v["name"]) == "" {
			rr.Errors = append(rr.Errors, "name is required")
		}
		var parentID *uuid.UUID
		if pk := strings.TrimSpace(v["parent"]); pk != "" {
			if id, e := s.resolveOrgKey(ctx, pk); e == nil {
				parentID = id
			} else if !seenKey[strings.ToLower(pk)] {
				rr.Errors = append(rr.Errors, "unknown parent: "+pk)
			}
		}
		if key != "" {
			seenKey[strings.ToLower(key)] = true
		}
		if len(rr.Errors) > 0 {
			res.Rows = append(res.Rows, rr)
			continue
		}
		rr.OK = true
		if commit {
			if parentID == nil {
				if pk := strings.TrimSpace(v["parent"]); pk != "" {
					parentID, _ = s.resolveOrgKey(ctx, pk)
				}
			}
			path, e := s.childPath(ctx, "core.org_units", parentID, key)
			if e != nil {
				rr.OK = false
				rr.Errors = append(rr.Errors, e.Error())
				res.Rows = append(res.Rows, rr)
				continue
			}
			ou := &domain.OrgUnit{Key: key, Name: v["name"], ParentID: parentID, Path: path}
			if _, e := s.db.NewInsert().Model(ou).Returning("*").Exec(ctx); e != nil {
				rr.OK = false
				rr.Errors = append(rr.Errors, e.Error())
			} else {
				created++
				rr.Result = ou.ID.String()
			}
		}
		res.Rows = append(res.Rows, rr)
	}
	res.Summary = summarize(res.Rows, commit, created)
	return res
}

// ---- vlans ---------------------------------------------------------------

func (s *Server) importVlans(ctx context.Context, headers []string, rows []importer.Row, commit bool) importResult {
	res := importResult{Target: "vlans", Mode: modeStr(commit), Headers: headers}
	created := 0
	for _, row := range rows {
		rr := importRowResult{Line: row.Line, Values: row.Values}
		v := row.Values
		tag, e := strconv.Atoi(strings.TrimSpace(v["vlan_id"]))
		if e != nil || tag < 1 || tag > 4094 {
			rr.Errors = append(rr.Errors, "vlan_id must be a number 1..4094")
		}
		if strings.TrimSpace(v["name"]) == "" {
			rr.Errors = append(rr.Errors, "name is required")
		}
		locID, le := s.resolveLocationKey(ctx, v["location"])
		if le != nil {
			rr.Errors = append(rr.Errors, le.Error())
		}
		if len(rr.Errors) > 0 {
			res.Rows = append(res.Rows, rr)
			continue
		}
		rr.OK = true
		if commit {
			vl := &domain.Vlan{VlanID: tag, Name: v["name"], Description: v["description"], LocationID: locID, Attributes: map[string]any{}}
			if _, e := s.db.NewInsert().Model(vl).Returning("*").Exec(ctx); e != nil {
				rr.OK = false
				rr.Errors = append(rr.Errors, e.Error())
			} else {
				created++
				rr.Result = vl.ID.String()
			}
		}
		res.Rows = append(res.Rows, rr)
	}
	res.Summary = summarize(res.Rows, commit, created)
	return res
}

// ---- subnets -------------------------------------------------------------

func (s *Server) importSubnets(ctx context.Context, headers []string, rows []importer.Row, commit bool) importResult {
	res := importResult{Target: "subnets", Mode: modeStr(commit), Headers: headers}
	created := 0
	for _, row := range rows {
		rr := importRowResult{Line: row.Line, Values: row.Values}
		v := row.Values
		cidr := strings.TrimSpace(v["cidr"])
		if cidr == "" {
			rr.Errors = append(rr.Errors, "cidr is required")
		} else if _, _, e := net.ParseCIDR(cidr); e != nil {
			rr.Errors = append(rr.Errors, "invalid cidr (use e.g. 10.0.0.0/24)")
		}
		if strings.TrimSpace(v["name"]) == "" {
			rr.Errors = append(rr.Errors, "name is required")
		}
		locID, le := s.resolveLocationKey(ctx, v["location"])
		if le != nil {
			rr.Errors = append(rr.Errors, le.Error())
		}
		var vlanTag int
		if raw := strings.TrimSpace(v["vlan_id"]); raw != "" {
			if t, e := strconv.Atoi(raw); e != nil || t < 1 || t > 4094 {
				rr.Errors = append(rr.Errors, "vlan_id must be a number 1..4094")
			} else {
				vlanTag = t
			}
		}
		if len(rr.Errors) > 0 {
			res.Rows = append(res.Rows, rr)
			continue
		}
		rr.OK = true
		if commit {
			var vlanID *uuid.UUID
			if vlanTag > 0 {
				vlanID = s.resolveVlanTag(ctx, vlanTag, locID, v["name"])
			}
			sn := &domain.Subnet{
				CIDR: cidr, Name: v["name"], VlanID: vlanID, LocationID: locID,
				Gateway: v["gateway"], Description: v["description"], Attributes: map[string]any{},
			}
			if _, e := s.db.NewInsert().Model(sn).Returning("*").Exec(ctx); e != nil {
				rr.OK = false
				rr.Errors = append(rr.Errors, e.Error())
			} else {
				created++
				rr.Result = sn.ID.String()
			}
		}
		res.Rows = append(res.Rows, rr)
	}
	res.Summary = summarize(res.Rows, commit, created)
	return res
}

// resolveVlanTag finds a VLAN by numeric tag or creates one.
func (s *Server) resolveVlanTag(ctx context.Context, tag int, locID *uuid.UUID, fallbackName string) *uuid.UUID {
	vl := new(domain.Vlan)
	if err := s.db.NewSelect().Model(vl).Column("id").Where("vlan_id = ?", tag).Limit(1).Scan(ctx); err == nil {
		return &vl.ID
	}
	nv := &domain.Vlan{VlanID: tag, Name: "VLAN " + strconv.Itoa(tag), LocationID: locID, Attributes: map[string]any{}}
	if _, err := s.db.NewInsert().Model(nv).Returning("*").Exec(ctx); err != nil {
		return nil
	}
	return &nv.ID
}

// ---- ip addresses --------------------------------------------------------

func (s *Server) importIPs(ctx context.Context, headers []string, rows []importer.Row, commit bool) importResult {
	res := importResult{Target: "ips", Mode: modeStr(commit), Headers: headers}
	created := 0
	for _, row := range rows {
		rr := importRowResult{Line: row.Line, Values: row.Values}
		v := row.Values
		addr := strings.TrimSpace(v["address"])
		if addr == "" {
			rr.Errors = append(rr.Errors, "address is required")
		} else if net.ParseIP(hostPart(addr)) == nil {
			rr.Errors = append(rr.Errors, "invalid IP address")
		}
		status := firstNonEmpty(strings.TrimSpace(v["status"]), "allocated")
		switch status {
		case "allocated", "reserved", "deprecated":
		default:
			rr.Errors = append(rr.Errors, "status must be allocated, reserved or deprecated")
		}
		var assetID *uuid.UUID
		if tag := strings.TrimSpace(v["asset_tag"]); tag != "" {
			a := new(domain.Asset)
			if err := s.db.NewSelect().Model(a).Column("id").
				Where("asset_tag = ?", tag).Limit(1).Scan(ctx); err == nil {
				assetID = &a.ID
			} else {
				rr.Errors = append(rr.Errors, "unknown asset_tag: "+tag)
			}
		}
		if len(rr.Errors) > 0 {
			res.Rows = append(res.Rows, rr)
			continue
		}
		rr.OK = true
		if commit {
			ip := &domain.IPAddress{
				Address: hostPart(addr), AssetID: assetID, Status: status,
				DNSName: v["dns_name"], MAC: v["mac"], Description: v["description"],
				Attributes: map[string]any{},
			}
			var sid uuid.UUID
			if err := s.db.NewRaw(
				"SELECT id FROM ipam.subnets WHERE ?::inet << cidr ORDER BY masklen(cidr) DESC LIMIT 1", ip.Address,
			).Scan(ctx, &sid); err == nil {
				ip.SubnetID = &sid
			}
			if _, e := s.db.NewInsert().Model(ip).Returning("*").Exec(ctx); e != nil {
				rr.OK = false
				rr.Errors = append(rr.Errors, e.Error())
			} else {
				created++
				rr.Result = ip.ID.String()
			}
		}
		res.Rows = append(res.Rows, rr)
	}
	res.Summary = summarize(res.Rows, commit, created)
	return res
}

// ---- software ------------------------------------------------------------

func (s *Server) importSoftware(ctx context.Context, headers []string, rows []importer.Row, commit bool) importResult {
	res := importResult{Target: "software", Mode: modeStr(commit), Headers: headers}
	created := 0
	for _, row := range rows {
		rr := importRowResult{Line: row.Line, Values: row.Values}
		v := row.Values
		if strings.TrimSpace(v["name"]) == "" {
			rr.Errors = append(rr.Errors, "name is required")
		}
		if len(rr.Errors) > 0 {
			res.Rows = append(res.Rows, rr)
			continue
		}
		rr.OK = true
		if commit {
			sw := &domain.Software{
				Name: v["name"], Publisher: v["publisher"],
				Category: firstNonEmpty(v["category"], "application"), Description: v["description"],
				Attributes: map[string]any{},
			}
			if _, e := s.db.NewInsert().Model(sw).Returning("*").Exec(ctx); e != nil {
				rr.OK = false
				rr.Errors = append(rr.Errors, e.Error())
			} else {
				created++
				rr.Result = sw.ID.String()
			}
		}
		res.Rows = append(res.Rows, rr)
	}
	res.Summary = summarize(res.Rows, commit, created)
	return res
}

// ---- licenses ------------------------------------------------------------

func (s *Server) importLicenses(ctx context.Context, headers []string, rows []importer.Row, commit bool) importResult {
	res := importResult{Target: "licenses", Mode: modeStr(commit), Headers: headers}
	created := 0
	for _, row := range rows {
		rr := importRowResult{Line: row.Line, Values: row.Values}
		v := row.Values
		if strings.TrimSpace(v["name"]) == "" {
			rr.Errors = append(rr.Errors, "name is required")
		}
		var seats *int
		if raw := strings.TrimSpace(v["seats"]); raw != "" {
			if n, e := strconv.Atoi(raw); e != nil || n < 0 {
				rr.Errors = append(rr.Errors, "seats must be a non-negative whole number")
			} else {
				seats = &n
			}
		}
		var cost *float64
		if raw := strings.TrimSpace(v["purchase_cost"]); raw != "" {
			if f, e := strconv.ParseFloat(raw, 64); e != nil {
				rr.Errors = append(rr.Errors, "purchase_cost must be a number")
			} else {
				cost = &f
			}
		}
		start, e1 := parseImportDate(v["start_date"])
		expiry, e2 := parseImportDate(v["expiry_date"])
		if e1 != nil || e2 != nil {
			rr.Errors = append(rr.Errors, "dates must be YYYY-MM-DD")
		}
		if len(rr.Errors) > 0 {
			res.Rows = append(res.Rows, rr)
			continue
		}
		rr.OK = true
		if commit {
			var swID *uuid.UUID
			if name := strings.TrimSpace(v["software"]); name != "" {
				sw := new(domain.Software)
				if err := s.db.NewSelect().Model(sw).Column("id").Where("lower(name) = lower(?)", name).Limit(1).Scan(ctx); err == nil {
					swID = &sw.ID
				}
			}
			vendorID, _ := s.resolveVendor(ctx, v["vendor"])
			lic := &domain.License{
				Name: v["name"], SoftwareID: swID, VendorID: vendorID,
				LicenseType: firstNonEmpty(v["license_type"], "subscription"),
				Seats:       seats, PurchaseCost: cost, Currency: firstNonEmpty(v["currency"], "USD"),
				StartDate: start, ExpiryDate: expiry, Notes: v["notes"], Attributes: map[string]any{},
			}
			if _, e := s.db.NewInsert().Model(lic).Returning("*").Exec(ctx); e != nil {
				rr.OK = false
				rr.Errors = append(rr.Errors, e.Error())
			} else {
				created++
				rr.Result = lic.ID.String()
			}
		}
		res.Rows = append(res.Rows, rr)
	}
	res.Summary = summarize(res.Rows, commit, created)
	return res
}

// parseFloatPtr parses an optional float; "" yields nil with no error.
func parseFloatPtr(raw string) (*float64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	f, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return nil, err
	}
	return &f, nil
}
