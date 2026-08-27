package http

import (
	"context"
	"net"
	"net/http"
	"strings"

	"itam/internal/domain"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// ===========================================================================
// Port profiles (metadata-driven port generation)
// ===========================================================================

func (s *Server) handleListPortProfiles(w http.ResponseWriter, r *http.Request) {
	var profiles []domain.PortProfile
	q := s.db.NewSelect().Model(&profiles).Order("asset_type_id ASC", "sort ASC")
	if t := r.URL.Query().Get("asset_type_id"); t != "" {
		q = q.Where("asset_type_id = ?", t)
	}
	if err := q.Scan(r.Context()); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, profiles)
}

func (s *Server) handleUpsertPortProfile(w http.ResponseWriter, r *http.Request) {
	var in domain.PortProfile
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if in.AssetTypeID == 0 || in.Label == "" {
		writeErr(w, http.StatusBadRequest, "asset_type_id and label are required")
		return
	}
	if in.PortType == "" {
		in.PortType = "ethernet"
	}
	if in.Count < 1 {
		in.Count = 1
	}
	if in.StartIndex == 0 && in.Count > 0 {
		in.StartIndex = 1
	}
	if in.ID == uuid.Nil {
		if _, err := s.db.NewInsert().Model(&in).Returning("*").Exec(r.Context()); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, in)
		return
	}
	if _, err := s.db.NewUpdate().Model(&in).
		Column("label", "port_type", "speed", "name_prefix", "start_index", "count", "sort").
		Where("id = ?", in.ID).Exec(r.Context()); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, in)
}

func (s *Server) handleDeletePortProfile(w http.ResponseWriter, r *http.Request) {
	s.deleteByID(w, r, (*domain.PortProfile)(nil))
}

// handleGeneratePorts creates dcim.ports on an asset from its type's port
// profiles. Existing port names are skipped, so it is safe to re-run.
func (s *Server) handleGeneratePorts(w http.ResponseWriter, r *http.Request) {
	assetID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	asset := new(domain.Asset)
	if err := s.db.NewSelect().Model(asset).Column("id", "asset_type_id").
		Where("a.id = ?", assetID).Scan(r.Context()); err != nil {
		writeErr(w, http.StatusNotFound, "asset not found")
		return
	}
	var profiles []domain.PortProfile
	if err := s.db.NewSelect().Model(&profiles).
		Where("asset_type_id = ?", asset.AssetTypeID).Order("sort ASC").Scan(r.Context()); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if len(profiles) == 0 {
		writeErr(w, http.StatusUnprocessableEntity, "this asset type has no port profile; define one in Admin → Port Profiles")
		return
	}
	// Existing port names + current max sort.
	var existing []domain.Port
	_ = s.db.NewSelect().Model(&existing).Where("asset_id = ?", assetID).Scan(r.Context())
	have := map[string]bool{}
	maxSort := 0
	for _, p := range existing {
		have[p.Name] = true
		if p.Sort > maxSort {
			maxSort = p.Sort
		}
	}
	toCreate := []domain.Port{}
	sortN := maxSort
	for _, pf := range profiles {
		for i := 0; i < pf.Count; i++ {
			name := pf.NamePrefix + itoa(pf.StartIndex+i)
			if have[name] {
				continue
			}
			have[name] = true
			sortN++
			toCreate = append(toCreate, domain.Port{
				AssetID: assetID, Name: name, PortType: pf.PortType, Speed: pf.Speed,
				Sort: sortN, Attributes: map[string]any{},
			})
		}
	}
	if len(toCreate) > 0 {
		if _, err := s.db.NewInsert().Model(&toCreate).Exec(r.Context()); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]int{"created": len(toCreate)})
}

// ===========================================================================
// Cable trace (follows the cable graph, hopping through patch panels)
// ===========================================================================

type traceHop struct {
	ConnectionID uuid.UUID `json:"connection_id"`
	CableType    string    `json:"cable_type"`
	Label        string    `json:"label,omitempty"`
	LengthM      *float64  `json:"length_m,omitempty"`
	FromAssetID  uuid.UUID `json:"from_asset_id"`
	FromAssetTag string    `json:"from_asset_tag"`
	FromAsset    string    `json:"from_asset"`
	FromPort     string    `json:"from_port"`
	ToAssetID    uuid.UUID `json:"to_asset_id"`
	ToAssetTag   string    `json:"to_asset_tag"`
	ToAsset      string    `json:"to_asset"`
	ToPort       string    `json:"to_port"`
	PatchPanel   bool      `json:"patch_panel"`
}

func (s *Server) handleCableTrace(w http.ResponseWriter, r *http.Request) {
	portID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	current, err := s.loadPort(r.Context(), portID)
	if err != nil {
		writeErr(w, http.StatusNotFound, "port not found")
		return
	}
	hops := []traceHop{}
	visited := map[uuid.UUID]bool{}
	for {
		visited[current.ID] = true
		var conn domain.Connection
		err := s.db.NewSelect().Model(&conn).
			Where("a_port_id = ? OR b_port_id = ?", current.ID, current.ID).
			Limit(1).Scan(r.Context())
		if err != nil {
			break // no further cable
		}
		peerID := conn.BPortID
		if conn.APortID != current.ID {
			peerID = conn.APortID
		}
		if visited[peerID] {
			break
		}
		peer, err := s.loadPort(r.Context(), peerID)
		if err != nil {
			break
		}
		hop := traceHop{
			ConnectionID: conn.ID, CableType: conn.CableType, Label: conn.Label, LengthM: conn.LengthM,
			FromAssetID: current.AssetID, FromPort: current.Name,
			ToAssetID: peer.AssetID, ToPort: peer.Name,
		}
		if current.Asset != nil {
			hop.FromAsset, hop.FromAssetTag = current.Asset.Name, current.Asset.AssetTag
		}
		if peer.Asset != nil {
			hop.ToAsset, hop.ToAssetTag = peer.Asset.Name, peer.Asset.AssetTag
		}
		peerType := s.assetTypeKey(r.Context(), peer.AssetID)
		hop.PatchPanel = peerType == "patch_panel"
		hops = append(hops, hop)
		visited[peer.ID] = true
		if !hop.PatchPanel {
			break // reached a real device
		}
		paired := s.pairedPatchPort(r.Context(), peer, visited)
		if paired == nil {
			break
		}
		current = paired
	}
	writeJSON(w, http.StatusOK, map[string]any{"hops": hops})
}

func (s *Server) assetTypeKey(ctx context.Context, assetID uuid.UUID) string {
	var key string
	_ = s.db.NewRaw(
		"SELECT t.key FROM core.assets a JOIN meta.asset_types t ON t.id = a.asset_type_id WHERE a.id = ?",
		assetID).Scan(ctx, &key)
	return key
}

// pairedPatchPort finds the opposite-face port on a patch panel (same trailing
// number, e.g. F12 <-> R12).
func (s *Server) pairedPatchPort(ctx context.Context, p *domain.Port, visited map[uuid.UUID]bool) *domain.Port {
	num, ok := trailingNum(p.Name)
	if !ok {
		return nil
	}
	var ports []domain.Port
	_ = s.db.NewSelect().Model(&ports).Relation("Asset").Where("pt.asset_id = ?", p.AssetID).Scan(ctx)
	for i := range ports {
		cand := &ports[i]
		if cand.ID == p.ID || visited[cand.ID] {
			continue
		}
		if n, ok := trailingNum(cand.Name); ok && n == num {
			return cand
		}
	}
	return nil
}

// ===========================================================================
// DHCP scopes
// ===========================================================================

func (s *Server) handleListDHCPScopes(w http.ResponseWriter, r *http.Request) {
	subnetID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	type scopeUsage struct {
		domain.DHCPScope
		PoolSize int64 `bun:"pool_size" json:"pool_size"`
		Used     int   `bun:"used" json:"used"`
	}
	scopes := []scopeUsage{}
	if err := s.db.NewRaw(
		`SELECT d.*,
		        (d.range_end - d.range_start + 1) AS pool_size,
		        (SELECT count(*) FROM ipam.ip_addresses ip
		           WHERE ip.subnet_id = d.subnet_id
		             AND ip.address >= d.range_start AND ip.address <= d.range_end) AS used
		   FROM ipam.dhcp_scopes d
		  WHERE d.subnet_id = ?
		  ORDER BY d.name ASC`, subnetID,
	).Scan(r.Context(), &scopes); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, scopes)
}

func (s *Server) handleCreateDHCPScope(w http.ResponseWriter, r *http.Request) {
	var in domain.DHCPScope
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if in.SubnetID == uuid.Nil || in.Name == "" || in.RangeStart == "" || in.RangeEnd == "" {
		writeErr(w, http.StatusBadRequest, "subnet_id, name, range_start and range_end are required")
		return
	}
	if net.ParseIP(in.RangeStart) == nil || net.ParseIP(in.RangeEnd) == nil {
		writeErr(w, http.StatusBadRequest, "range_start and range_end must be valid IP addresses")
		return
	}
	if in.LeaseHours <= 0 {
		in.LeaseHours = 24
	}
	if in.Attributes == nil {
		in.Attributes = map[string]any{}
	}
	if _, err := s.db.NewInsert().Model(&in).Returning("*").Exec(r.Context()); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, in)
}

func (s *Server) handleUpdateDHCPScope(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	var in domain.DHCPScope
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	in.ID = id
	if in.Attributes == nil {
		in.Attributes = map[string]any{}
	}
	if in.LeaseHours <= 0 {
		in.LeaseHours = 24
	}
	if _, err := s.db.NewUpdate().Model(&in).
		Column("name", "range_start", "range_end", "gateway", "dns", "domain", "lease_hours", "enabled", "attributes").
		Set("updated_at = now()").Where("id = ?", id).Exec(r.Context()); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, in)
}

func (s *Server) handleDeleteDHCPScope(w http.ResponseWriter, r *http.Request) {
	s.deleteByID(w, r, (*domain.DHCPScope)(nil))
}

// ===========================================================================
// netcfg: address objects, routes, NAT rules
// ===========================================================================

func (s *Server) handleListAddressObjects(w http.ResponseWriter, r *http.Request) {
	assetID := chi.URLParam(r, "id")
	objs := []domain.AddressObject{}
	if err := s.db.NewSelect().Model(&objs).Where("asset_id = ?", assetID).
		Order("name ASC").Scan(r.Context()); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, objs)
}

func (s *Server) handleCreateAddressObject(w http.ResponseWriter, r *http.Request) {
	assetID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	var in domain.AddressObject
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	in.AssetID = assetID
	if in.Name == "" || in.Value == "" {
		writeErr(w, http.StatusBadRequest, "name and value are required")
		return
	}
	if in.Kind == "" {
		in.Kind = "host"
	}
	if in.Attributes == nil {
		in.Attributes = map[string]any{}
	}
	if _, err := s.db.NewInsert().Model(&in).Returning("*").Exec(r.Context()); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, in)
}

func (s *Server) handleDeleteAddressObject(w http.ResponseWriter, r *http.Request) {
	s.deleteByParam(w, r, "subID", (*domain.AddressObject)(nil))
}

func (s *Server) handleListRoutes(w http.ResponseWriter, r *http.Request) {
	assetID := chi.URLParam(r, "id")
	routes := []domain.Route{}
	if err := s.db.NewSelect().Model(&routes).Where("asset_id = ?", assetID).
		Order("destination ASC").Scan(r.Context()); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, routes)
}

func (s *Server) handleCreateRoute(w http.ResponseWriter, r *http.Request) {
	assetID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	var in domain.Route
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	in.AssetID = assetID
	if in.Destination == "" {
		writeErr(w, http.StatusBadRequest, "destination is required (CIDR or 'default')")
		return
	}
	if _, err := s.db.NewInsert().Model(&in).Returning("*").Exec(r.Context()); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, in)
}

func (s *Server) handleDeleteRoute(w http.ResponseWriter, r *http.Request) {
	s.deleteByParam(w, r, "subID", (*domain.Route)(nil))
}

func (s *Server) handleListNATRules(w http.ResponseWriter, r *http.Request) {
	assetID := chi.URLParam(r, "id")
	rules := []domain.NATRule{}
	if err := s.db.NewSelect().Model(&rules).Where("asset_id = ?", assetID).
		Order("seq ASC", "created_at ASC").Scan(r.Context()); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, rules)
}

func (s *Server) handleCreateNATRule(w http.ResponseWriter, r *http.Request) {
	assetID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	var in domain.NATRule
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	in.AssetID = assetID
	if in.NATType == "" {
		in.NATType = "source"
	}
	if _, err := s.db.NewInsert().Model(&in).Returning("*").Exec(r.Context()); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, in)
}

func (s *Server) handleDeleteNATRule(w http.ResponseWriter, r *http.Request) {
	s.deleteByParam(w, r, "subID", (*domain.NATRule)(nil))
}

// ===========================================================================
// netcfg interface -> IPAM registration
// ===========================================================================

// handleRegisterInterfaceIP creates (or links) an IPAM address from a netcfg
// interface's ip_cidr and records it back on the interface.
func (s *Server) handleRegisterInterfaceIP(w http.ResponseWriter, r *http.Request) {
	ifaceID, err := uuid.Parse(chi.URLParam(r, "subID"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	iface := new(domain.NetInterface)
	if err := s.db.NewSelect().Model(iface).Where("ni.id = ?", ifaceID).Scan(r.Context()); err != nil {
		writeErr(w, http.StatusNotFound, "interface not found")
		return
	}
	if iface.IPCidr == "" {
		writeErr(w, http.StatusUnprocessableEntity, "interface has no IP/CIDR to register")
		return
	}
	host := hostPart(iface.IPCidr)
	if net.ParseIP(host) == nil {
		writeErr(w, http.StatusBadRequest, "interface ip_cidr is not a valid address")
		return
	}
	// Reuse an existing IPAM entry for this address if present.
	existing := new(domain.IPAddress)
	if err := s.db.NewSelect().Model(existing).Where("address = ?", host).Limit(1).Scan(r.Context()); err == nil {
		_, _ = s.db.NewUpdate().Model((*domain.NetInterface)(nil)).
			Set("ip_id = ?", existing.ID).Where("id = ?", ifaceID).Exec(r.Context())
		writeJSON(w, http.StatusOK, existing)
		return
	}
	ip := &domain.IPAddress{
		Address: host, AssetID: &iface.AssetID, Status: "allocated",
		Description: "iface " + iface.Name, Attributes: map[string]any{},
	}
	var sid uuid.UUID
	if err := s.db.NewRaw(
		"SELECT id FROM ipam.subnets WHERE ?::inet << cidr ORDER BY masklen(cidr) DESC LIMIT 1", host,
	).Scan(r.Context(), &sid); err == nil {
		ip.SubnetID = &sid
	}
	if _, err := s.db.NewInsert().Model(ip).Returning("*").Exec(r.Context()); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	_, _ = s.db.NewUpdate().Model((*domain.NetInterface)(nil)).
		Set("ip_id = ?", ip.ID).Where("id = ?", ifaceID).Exec(r.Context())
	writeJSON(w, http.StatusCreated, ip)
}

// ===========================================================================
// Config backup diff
// ===========================================================================

type diffLine struct {
	Op   string `json:"op"` // "=", "-", "+"
	Text string `json:"text"`
}

func (s *Server) handleBackupDiff(w http.ResponseWriter, r *http.Request) {
	aID := r.URL.Query().Get("a")
	bID := r.URL.Query().Get("b")
	if aID == "" || bID == "" {
		writeErr(w, http.StatusBadRequest, "a and b backup ids are required")
		return
	}
	var a, b domain.ConfigBackup
	if err := s.db.NewSelect().Model(&a).Where("cb.id = ?", aID).Scan(r.Context()); err != nil {
		writeErr(w, http.StatusNotFound, "backup a not found")
		return
	}
	if err := s.db.NewSelect().Model(&b).Where("cb.id = ?", bID).Scan(r.Context()); err != nil {
		writeErr(w, http.StatusNotFound, "backup b not found")
		return
	}
	lines := lineDiff(strings.Split(a.Content, "\n"), strings.Split(b.Content, "\n"))
	added, removed := 0, 0
	for _, l := range lines {
		switch l.Op {
		case "+":
			added++
		case "-":
			removed++
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"a": map[string]any{"id": a.ID, "taken_at": a.TakenAt, "version": a.Version},
		"b": map[string]any{"id": b.ID, "taken_at": b.TakenAt, "version": b.Version},
		"added": added, "removed": removed, "lines": lines,
	})
}

// lineDiff computes a minimal LCS-based line diff between two slices.
func lineDiff(a, b []string) []diffLine {
	n, m := len(a), len(b)
	// LCS length table.
	lcs := make([][]int, n+1)
	for i := range lcs {
		lcs[i] = make([]int, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if a[i] == b[j] {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else if lcs[i+1][j] >= lcs[i][j+1] {
				lcs[i][j] = lcs[i+1][j]
			} else {
				lcs[i][j] = lcs[i][j+1]
			}
		}
	}
	out := []diffLine{}
	i, j := 0, 0
	for i < n && j < m {
		if a[i] == b[j] {
			out = append(out, diffLine{Op: "=", Text: a[i]})
			i++
			j++
		} else if lcs[i+1][j] >= lcs[i][j+1] {
			out = append(out, diffLine{Op: "-", Text: a[i]})
			i++
		} else {
			out = append(out, diffLine{Op: "+", Text: b[j]})
			j++
		}
	}
	for ; i < n; i++ {
		out = append(out, diffLine{Op: "-", Text: a[i]})
	}
	for ; j < m; j++ {
		out = append(out, diffLine{Op: "+", Text: b[j]})
	}
	return out
}

// ---- small helpers --------------------------------------------------------

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

func containsInt(s []int, v int) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

func containsStr(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

// trailingNum returns the trailing run of digits in s (e.g. "F12" -> "12").
func trailingNum(s string) (string, bool) {
	i := len(s)
	for i > 0 && s[i-1] >= '0' && s[i-1] <= '9' {
		i--
	}
	if i == len(s) {
		return "", false
	}
	return s[i:], true
}
