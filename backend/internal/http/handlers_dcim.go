package http

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"itam/internal/domain"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

var validFaces = map[string]bool{"front": true, "rear": true, "full": true}

// ---- racks ---------------------------------------------------------------

func (s *Server) handleListRacks(w http.ResponseWriter, r *http.Request) {
	var racks []domain.Rack
	q := s.db.NewSelect().Model(&racks).Relation("Location").Relation("Mounts").Order("rk.name ASC")
	if loc := r.URL.Query().Get("location_id"); loc != "" {
		q = q.Where("rk.location_id = ?", loc)
	}
	if err := q.Scan(r.Context()); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, racks)
}

// handleGetRack returns a rack with its mounts and the mounted assets, for the
// elevation view.
func (s *Server) handleGetRack(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	rack := new(domain.Rack)
	err = s.db.NewSelect().Model(rack).
		Relation("Location").
		Relation("Mounts", func(q *bunSelect) *bunSelect { return q.Order("position ASC") }).
		Relation("Mounts.Asset").
		Relation("Mounts.Asset.AssetType").
		Relation("Mounts.Asset.CurrentState").
		Where("rk.id = ?", id).Scan(r.Context())
	if err != nil {
		writeErr(w, http.StatusNotFound, "rack not found")
		return
	}
	writeJSON(w, http.StatusOK, rack)
}

type rackInput struct {
	Key          string         `json:"key"`
	Name         string         `json:"name"`
	LocationID   *uuid.UUID     `json:"location_id"`
	AssetID      *uuid.UUID     `json:"asset_id"`
	UHeight      int            `json:"u_height"`
	StartingUnit int            `json:"starting_unit"`
	DescUnits    bool           `json:"desc_units"`
	WidthMM      *int           `json:"width_mm"`
	DepthMM      *int           `json:"depth_mm"`
	PowerCapacityW *int         `json:"power_capacity_w"`
	MaxWeightKg  *float64       `json:"max_weight_kg"`
	Attributes   map[string]any `json:"attributes"`
	Notes        string         `json:"notes"`
}

func (s *Server) handleCreateRack(w http.ResponseWriter, r *http.Request) {
	var in rackInput
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if in.Name == "" {
		writeErr(w, http.StatusBadRequest, "name is required")
		return
	}
	rack := &domain.Rack{
		Key: firstNonEmpty(strings.ToLower(strings.TrimSpace(in.Key)), slugify(in.Name)),
		Name: in.Name, LocationID: in.LocationID, AssetID: in.AssetID,
		UHeight: in.UHeight, StartingUnit: in.StartingUnit, DescUnits: in.DescUnits,
		WidthMM: in.WidthMM, DepthMM: in.DepthMM, PowerCapacityW: in.PowerCapacityW,
		MaxWeightKg: in.MaxWeightKg, Attributes: in.Attributes, Notes: in.Notes,
	}
	if rack.UHeight <= 0 {
		rack.UHeight = 42
	}
	if rack.StartingUnit <= 0 {
		rack.StartingUnit = 1
	}
	if rack.Attributes == nil {
		rack.Attributes = map[string]any{}
	}
	if _, err := s.db.NewInsert().Model(rack).Returning("*").Exec(r.Context()); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, rack)
}

func (s *Server) handleUpdateRack(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	var in rackInput
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if in.Attributes == nil {
		in.Attributes = map[string]any{}
	}
	upd := &domain.Rack{
		ID: id, Name: in.Name, LocationID: in.LocationID, AssetID: in.AssetID,
		UHeight: in.UHeight, StartingUnit: in.StartingUnit, DescUnits: in.DescUnits,
		WidthMM: in.WidthMM, DepthMM: in.DepthMM, PowerCapacityW: in.PowerCapacityW,
		MaxWeightKg: in.MaxWeightKg, Attributes: in.Attributes, Notes: in.Notes,
	}
	if upd.UHeight <= 0 {
		upd.UHeight = 42
	}
	if upd.StartingUnit <= 0 {
		upd.StartingUnit = 1
	}
	if _, err := s.db.NewUpdate().Model(upd).
		Column("name", "location_id", "asset_id", "u_height", "starting_unit", "desc_units",
			"width_mm", "depth_mm", "power_capacity_w", "max_weight_kg", "attributes", "notes").
		Set("updated_at = now()").Where("id = ?", id).Exec(r.Context()); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, upd)
}

func (s *Server) handleDeleteRack(w http.ResponseWriter, r *http.Request) {
	s.deleteByID(w, r, (*domain.Rack)(nil))
}

// ---- topology ------------------------------------------------------------

type topoNode struct {
	ID       uuid.UUID `json:"id"`
	Name     string    `json:"name"`
	AssetTag string    `json:"asset_tag"`
	TypeKey  string    `json:"type_key"`
	Location string    `json:"location"`
	Vlans    []int     `json:"vlans"`
	Subnets  []string  `json:"subnets"`
}

type topoLink struct {
	ID        uuid.UUID `json:"id"`
	AAssetID  uuid.UUID `json:"a_asset_id"`
	APort     string    `json:"a_port"`
	BAssetID  uuid.UUID `json:"b_asset_id"`
	BPort     string    `json:"b_port"`
	CableType string    `json:"cable_type"`
}

// handleTopology returns the cable graph (nodes = devices, links = cables),
// optionally scoped to a location subtree.
func (s *Server) handleTopology(w http.ResponseWriter, r *http.Request) {
	var path string
	if loc := r.URL.Query().Get("location_id"); loc != "" {
		_ = s.db.NewRaw("SELECT path::text FROM core.locations WHERE id = ?", loc).Scan(r.Context(), &path)
	}
	var rows []struct {
		ID        uuid.UUID `bun:"id"`
		CableType string    `bun:"cable_type"`
		AID       uuid.UUID `bun:"a_id"`
		APort     string    `bun:"a_port"`
		AName     string    `bun:"a_name"`
		ATag      string    `bun:"a_tag"`
		BID       uuid.UUID `bun:"b_id"`
		BPort     string    `bun:"b_port"`
		BName     string    `bun:"b_name"`
		BTag      string    `bun:"b_tag"`
	}
	q := s.db.NewSelect().
		TableExpr("dcim.connections AS cn").
		ColumnExpr("cn.id, cn.cable_type").
		ColumnExpr("pa.asset_id AS a_id, pa.name AS a_port, aa.name AS a_name, aa.asset_tag AS a_tag").
		ColumnExpr("pb.asset_id AS b_id, pb.name AS b_port, ab.name AS b_name, ab.asset_tag AS b_tag").
		Join("JOIN dcim.ports pa ON pa.id = cn.a_port_id").
		Join("JOIN dcim.ports pb ON pb.id = cn.b_port_id").
		Join("JOIN core.assets aa ON aa.id = pa.asset_id").
		Join("JOIN core.assets ab ON ab.id = pb.asset_id")
	if path != "" {
		q = q.Join("LEFT JOIN core.locations la ON la.id = aa.location_id").
			Join("LEFT JOIN core.locations lb ON lb.id = ab.location_id").
			Where("la.path <@ ?::ltree OR lb.path <@ ?::ltree", path, path)
	}
	if err := q.Scan(r.Context(), &rows); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	nodes := map[uuid.UUID]topoNode{}
	links := make([]topoLink, 0, len(rows))
	for _, row := range rows {
		nodes[row.AID] = topoNode{ID: row.AID, Name: row.AName, AssetTag: row.ATag}
		nodes[row.BID] = topoNode{ID: row.BID, Name: row.BName, AssetTag: row.BTag}
		links = append(links, topoLink{
			ID: row.ID, AAssetID: row.AID, APort: row.APort,
			BAssetID: row.BID, BPort: row.BPort, CableType: row.CableType,
		})
	}
	// Enrich nodes with type, location and VLAN/subnet membership for overlays.
	if len(nodes) > 0 {
		ids := make([]uuid.UUID, 0, len(nodes))
		for id := range nodes {
			ids = append(ids, id)
		}
		var meta []struct {
			ID       uuid.UUID `bun:"id"`
			TypeKey  string    `bun:"type_key"`
			Location string    `bun:"location"`
		}
		_ = s.db.NewSelect().
			TableExpr("core.assets AS a").
			ColumnExpr("a.id").
			ColumnExpr("t.key AS type_key").
			ColumnExpr("COALESCE(l.name, '') AS location").
			Join("LEFT JOIN meta.asset_types t ON t.id = a.asset_type_id").
			Join("LEFT JOIN core.locations l ON l.id = a.location_id").
			Where("a.id IN (?)", bun.In(ids)).Scan(r.Context(), &meta)
		for _, m := range meta {
			if n, ok := nodes[m.ID]; ok {
				n.TypeKey = m.TypeKey
				n.Location = m.Location
				nodes[m.ID] = n
			}
		}
		// VLAN + subnet membership via IPAM addresses bound to the asset.
		var nets []struct {
			AssetID uuid.UUID `bun:"asset_id"`
			VlanID  *int      `bun:"vlan_id"`
			Subnet  string    `bun:"subnet"`
		}
		_ = s.db.NewSelect().
			TableExpr("ipam.ip_addresses AS ip").
			ColumnExpr("ip.asset_id").
			ColumnExpr("vl.vlan_id").
			ColumnExpr("COALESCE(sn.name, host(sn.cidr)) AS subnet").
			Join("LEFT JOIN ipam.subnets sn ON sn.id = ip.subnet_id").
			Join("LEFT JOIN ipam.vlans vl ON vl.id = sn.vlan_id").
			Where("ip.asset_id IN (?)", bun.In(ids)).Scan(r.Context(), &nets)
		for _, nx := range nets {
			n, ok := nodes[nx.AssetID]
			if !ok {
				continue
			}
			if nx.VlanID != nil && !containsInt(n.Vlans, *nx.VlanID) {
				n.Vlans = append(n.Vlans, *nx.VlanID)
			}
			if nx.Subnet != "" && !containsStr(n.Subnets, nx.Subnet) {
				n.Subnets = append(n.Subnets, nx.Subnet)
			}
			nodes[nx.AssetID] = n
		}
	}
	nodeList := make([]topoNode, 0, len(nodes))
	for _, n := range nodes {
		nodeList = append(nodeList, n)
	}
	writeJSON(w, http.StatusOK, map[string]any{"nodes": nodeList, "links": links})
}

type rackMoveInput struct {
	LocationID *uuid.UUID `json:"location_id"`
}

// handleMoveRack relocates a rack to another location. Floor coordinates are
// cleared so it lands on the new floor's auto-grid until repositioned.
func (s *Server) handleMoveRack(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	var in rackMoveInput
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if _, err := s.db.NewUpdate().Model((*domain.Rack)(nil)).
		Set("location_id = ?", in.LocationID).
		Set("pos_x = NULL").Set("pos_y = NULL").
		Set("updated_at = now()").
		Where("id = ?", id).Exec(r.Context()); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "moved"})
}

// ---- mounting ------------------------------------------------------------

type mountInput struct {
	AssetID  uuid.UUID `json:"asset_id"`
	Position int       `json:"position"`
	UHeight  int       `json:"u_height"`
	Face     string    `json:"face"`
}

func (s *Server) handleMountAsset(w http.ResponseWriter, r *http.Request) {
	rackID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	var in mountInput
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if in.AssetID == uuid.Nil {
		writeErr(w, http.StatusBadRequest, "asset_id is required")
		return
	}
	rack := new(domain.Rack)
	if err := s.db.NewSelect().Model(rack).Where("rk.id = ?", rackID).Scan(r.Context()); err != nil {
		writeErr(w, http.StatusNotFound, "rack not found")
		return
	}
	if in.UHeight <= 0 {
		in.UHeight = 1
	}
	if in.Face == "" {
		in.Face = "front"
	}
	if !validFaces[in.Face] {
		writeErr(w, http.StatusBadRequest, "face must be front, rear or full")
		return
	}
	lowest := rack.StartingUnit
	highest := rack.StartingUnit + rack.UHeight - 1
	if in.Position < lowest || in.Position+in.UHeight-1 > highest {
		writeErr(w, http.StatusUnprocessableEntity,
			fmt.Sprintf("position %d (height %d) does not fit in a %dU rack", in.Position, in.UHeight, rack.UHeight))
		return
	}

	p := s.principal(r)
	mount := &domain.RackMount{
		RackID: rackID, AssetID: in.AssetID, Position: in.Position, UHeight: in.UHeight,
		Face: in.Face, MountedBy: &p.UserID,
	}
	if _, err := s.db.NewInsert().Model(mount).Returning("*").Exec(r.Context()); err != nil {
		msg := err.Error()
		switch {
		case strings.Contains(msg, "rack_mounts_asset_id_key"):
			writeErr(w, http.StatusConflict, "this asset is already mounted in a rack; unmount it first")
		case strings.Contains(msg, "exclusion") || strings.Contains(msg, "overlap") || strings.Contains(msg, "conflicting key"):
			writeErr(w, http.StatusConflict, "those rack units are already occupied on this face")
		default:
			writeErr(w, http.StatusBadRequest, msg)
		}
		return
	}

	// Keep the asset's location in sync with the rack's room, and log to timeline.
	if rack.LocationID != nil {
		_, _ = s.db.NewUpdate().Model((*domain.Asset)(nil)).
			Set("location_id = ?", *rack.LocationID).Set("updated_at = now()").
			Where("id = ?", in.AssetID).Exec(r.Context())
	}
	s.recordAssetEvent(r.Context(), in.AssetID, &p.UserID, assetEvent{
		Kind:    "mounted",
		Summary: fmt.Sprintf("Mounted in %s at U%d–U%d (%s)", rack.Name, in.Position, in.Position+in.UHeight-1, in.Face),
		Data: map[string]any{
			"rack": rack.Name, "rack_id": rackID.String(),
			"position": in.Position, "u_height": in.UHeight, "face": in.Face,
		},
		RefTable: "dcim.rack_mounts", RefID: &mount.ID,
	})
	writeJSON(w, http.StatusCreated, mount)
}

type assetPlacement struct {
	Mount      *domain.RackMount `json:"mount,omitempty"`
	RackRecord *domain.Rack      `json:"rack_record,omitempty"`
	PortCount  int               `json:"port_count"`
	PowerWatts *int              `json:"power_watts,omitempty"`
	RackUnits  *int              `json:"rack_units,omitempty"`
	WeightKg   *float64          `json:"weight_kg,omitempty"`
}

func attrInt(attrs map[string]any, key string) *int {
	if attrs == nil {
		return nil
	}
	v, ok := attrs[key]
	if !ok || v == nil {
		return nil
	}
	switch n := v.(type) {
	case float64:
		i := int(n)
		return &i
	case int:
		return &n
	case int64:
		i := int(n)
		return &i
	}
	return nil
}

func attrFloat(attrs map[string]any, key string) *float64 {
	if attrs == nil {
		return nil
	}
	v, ok := attrs[key]
	if !ok || v == nil {
		return nil
	}
	switch n := v.(type) {
	case float64:
		return &n
	case int:
		f := float64(n)
		return &f
	case int64:
		f := float64(n)
		return &f
	}
	return nil
}

// handleAssetPlacement returns rack mount info, linked rack record, and DCIM
// specs for an asset. Devices stay in core.assets; this is the DCIM overlay.
func (s *Server) handleAssetPlacement(w http.ResponseWriter, r *http.Request) {
	assetID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	asset := new(domain.Asset)
	if err := s.db.NewSelect().Model(asset).Where("a.id = ?", assetID).Scan(r.Context()); err != nil {
		writeErr(w, http.StatusNotFound, "asset not found")
		return
	}

	out := assetPlacement{}

	mount := new(domain.RackMount)
	if err := s.db.NewSelect().Model(mount).
		Relation("Rack").
		Relation("Rack.Location").
		Where("rm.asset_id = ?", assetID).Scan(r.Context()); err == nil {
		out.Mount = mount
	}

	rack := new(domain.Rack)
	if err := s.db.NewSelect().Model(rack).
		Relation("Location").
		Where("rk.asset_id = ?", assetID).Scan(r.Context()); err == nil {
		out.RackRecord = rack
	}

	out.PortCount, _ = s.db.NewSelect().Model((*domain.Port)(nil)).Where("asset_id = ?", assetID).Count(r.Context())

	out.PowerWatts = attrInt(asset.Attributes, "power_watts")
	if out.PowerWatts == nil {
		out.PowerWatts = attrInt(asset.Attributes, "output_watts")
	}
	out.RackUnits = attrInt(asset.Attributes, "rack_units")
	out.WeightKg = attrFloat(asset.Attributes, "weight_kg")

	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleUnmount(w http.ResponseWriter, r *http.Request) {
	mountID, err := uuid.Parse(chi.URLParam(r, "mountID"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	mount := new(domain.RackMount)
	if err := s.db.NewSelect().Model(mount).Where("rm.id = ?", mountID).Scan(r.Context()); err != nil {
		writeErr(w, http.StatusNotFound, "mount not found")
		return
	}
	rack := new(domain.Rack)
	_ = s.db.NewSelect().Model(rack).Column("name").Where("rk.id = ?", mount.RackID).Scan(r.Context())

	if _, err := s.db.NewDelete().Model((*domain.RackMount)(nil)).Where("id = ?", mountID).Exec(r.Context()); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	p := s.principal(r)
	s.recordAssetEvent(r.Context(), mount.AssetID, &p.UserID, assetEvent{
		Kind:    "unmounted",
		Summary: fmt.Sprintf("Unmounted from %s", firstNonEmpty(rack.Name, "rack")),
		Data:    map[string]any{"rack": rack.Name, "rack_id": mount.RackID.String()},
	})
	writeJSON(w, http.StatusNoContent, nil)
}

// ---- rack floor position (layout canvas) ---------------------------------

type rackPosInput struct {
	PosX     *float64 `json:"pos_x"`
	PosY     *float64 `json:"pos_y"`
	Rotation *int     `json:"rotation"`
}

func (s *Server) handleSetRackPosition(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	var in rackPosInput
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	q := s.db.NewUpdate().Model((*domain.Rack)(nil)).Set("updated_at = now()").Where("id = ?", id)
	q = q.Set("pos_x = ?", in.PosX).Set("pos_y = ?", in.PosY)
	if in.Rotation != nil {
		q = q.Set("rotation = ?", *in.Rotation)
	}
	if _, err := q.Exec(r.Context()); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusNoContent, nil)
}

// ---- ports ---------------------------------------------------------------

// portPeer describes the far end of a cable from a given port's perspective.
type portPeer struct {
	ConnectionID uuid.UUID `json:"connection_id"`
	CableType    string    `json:"cable_type"`
	Label        string    `json:"label,omitempty"`
	PortID       uuid.UUID `json:"port_id"`
	PortName     string    `json:"port_name"`
	AssetID      uuid.UUID `json:"asset_id"`
	AssetTag     string    `json:"asset_tag"`
	AssetName    string    `json:"asset_name"`
}

type portOut struct {
	domain.Port
	Connection *portPeer `json:"connection"`
}

func (s *Server) handleListPorts(w http.ResponseWriter, r *http.Request) {
	assetID := r.URL.Query().Get("asset_id")
	if assetID == "" {
		writeErr(w, http.StatusBadRequest, "asset_id is required")
		return
	}
	var ports []domain.Port
	if err := s.db.NewSelect().Model(&ports).
		Where("asset_id = ?", assetID).Order("sort ASC", "name ASC").Scan(r.Context()); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	out, err := s.annotatePorts(r.Context(), ports)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// annotatePorts attaches each port's cable peer (if any).
func (s *Server) annotatePorts(ctx context.Context, ports []domain.Port) ([]portOut, error) {
	out := make([]portOut, 0, len(ports))
	if len(ports) == 0 {
		return out, nil
	}
	ids := make([]uuid.UUID, len(ports))
	ourSet := map[uuid.UUID]bool{}
	for i, p := range ports {
		ids[i] = p.ID
		ourSet[p.ID] = true
	}
	var conns []domain.Connection
	if err := s.db.NewSelect().Model(&conns).
		Where("a_port_id IN (?) OR b_port_id IN (?)", bun.In(ids), bun.In(ids)).Scan(ctx); err != nil {
		return nil, err
	}
	// our port id -> (connection, peer port id)
	type link struct {
		conn   domain.Connection
		peerID uuid.UUID
	}
	linkByOur := map[uuid.UUID]link{}
	peerIDset := map[uuid.UUID]bool{}
	for _, c := range conns {
		if ourSet[c.APortID] {
			linkByOur[c.APortID] = link{c, c.BPortID}
			peerIDset[c.BPortID] = true
		}
		if ourSet[c.BPortID] {
			linkByOur[c.BPortID] = link{c, c.APortID}
			peerIDset[c.APortID] = true
		}
	}
	peers := map[uuid.UUID]domain.Port{}
	if len(peerIDset) > 0 {
		peerIDs := make([]uuid.UUID, 0, len(peerIDset))
		for id := range peerIDset {
			peerIDs = append(peerIDs, id)
		}
		var pp []domain.Port
		if err := s.db.NewSelect().Model(&pp).Relation("Asset").
			Where("pt.id IN (?)", bun.In(peerIDs)).Scan(ctx); err != nil {
			return nil, err
		}
		for _, p := range pp {
			peers[p.ID] = p
		}
	}
	for _, p := range ports {
		row := portOut{Port: p}
		if lk, ok := linkByOur[p.ID]; ok {
			peer := peers[lk.peerID]
			pp := &portPeer{
				ConnectionID: lk.conn.ID, CableType: lk.conn.CableType, Label: lk.conn.Label,
				PortID: peer.ID, PortName: peer.Name, AssetID: peer.AssetID,
			}
			if peer.Asset != nil {
				pp.AssetTag = peer.Asset.AssetTag
				pp.AssetName = peer.Asset.Name
			}
			row.Connection = pp
		}
		out = append(out, row)
	}
	return out, nil
}

type portInput struct {
	AssetID  uuid.UUID `json:"asset_id"`
	Name     string    `json:"name"`
	PortType string    `json:"port_type"`
	Speed    string    `json:"speed"`
	Sort     int       `json:"sort"`
}

func (s *Server) handleCreatePort(w http.ResponseWriter, r *http.Request) {
	var in portInput
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if in.AssetID == uuid.Nil || in.Name == "" {
		writeErr(w, http.StatusBadRequest, "asset_id and name are required")
		return
	}
	if in.PortType == "" {
		in.PortType = "ethernet"
	}
	port := &domain.Port{
		AssetID: in.AssetID, Name: in.Name, PortType: in.PortType, Speed: in.Speed,
		Sort: in.Sort, Attributes: map[string]any{},
	}
	if _, err := s.db.NewInsert().Model(port).Returning("*").Exec(r.Context()); err != nil {
		if strings.Contains(err.Error(), "ports_asset_id_name_key") {
			writeErr(w, http.StatusConflict, "a port with that name already exists on this asset")
			return
		}
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, port)
}

func (s *Server) handleUpdatePort(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	var in portInput
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	upd := &domain.Port{ID: id, Name: in.Name, PortType: in.PortType, Speed: in.Speed, Sort: in.Sort}
	if _, err := s.db.NewUpdate().Model(upd).
		Column("name", "port_type", "speed", "sort").Where("id = ?", id).Exec(r.Context()); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, upd)
}

func (s *Server) handleDeletePort(w http.ResponseWriter, r *http.Request) {
	s.deleteByID(w, r, (*domain.Port)(nil))
}

// handleListFreePorts returns ports not currently part of any cable, for the
// connection picker. ?exclude_asset_id filters out the asset being cabled from.
func (s *Server) handleListFreePorts(w http.ResponseWriter, r *http.Request) {
	var ports []domain.Port
	q := s.db.NewSelect().Model(&ports).Relation("Asset").
		Where("pt.id NOT IN (SELECT a_port_id FROM dcim.connections UNION SELECT b_port_id FROM dcim.connections)").
		Order("pt.asset_id ASC", "pt.sort ASC")
	if ex := r.URL.Query().Get("exclude_asset_id"); ex != "" {
		q = q.Where("pt.asset_id <> ?", ex)
	}
	if err := q.Scan(r.Context()); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, ports)
}

// ---- connections (cables) ------------------------------------------------

type connInput struct {
	APortID   uuid.UUID `json:"a_port_id"`
	BPortID   uuid.UUID `json:"b_port_id"`
	CableType string    `json:"cable_type"`
	Label     string    `json:"label"`
	LengthM   *float64  `json:"length_m"`
}

func (s *Server) loadPort(ctx context.Context, id uuid.UUID) (*domain.Port, error) {
	p := new(domain.Port)
	err := s.db.NewSelect().Model(p).Relation("Asset").Where("pt.id = ?", id).Scan(ctx)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (s *Server) handleCreateConnection(w http.ResponseWriter, r *http.Request) {
	var in connInput
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if in.APortID == uuid.Nil || in.BPortID == uuid.Nil {
		writeErr(w, http.StatusBadRequest, "a_port_id and b_port_id are required")
		return
	}
	if in.APortID == in.BPortID {
		writeErr(w, http.StatusBadRequest, "a port cannot connect to itself")
		return
	}
	a, err := s.loadPort(r.Context(), in.APortID)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "a_port not found")
		return
	}
	b, err := s.loadPort(r.Context(), in.BPortID)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "b_port not found")
		return
	}
	count, _ := s.db.NewSelect().Model((*domain.Connection)(nil)).
		Where("a_port_id IN (?, ?) OR b_port_id IN (?, ?)", a.ID, b.ID, a.ID, b.ID).Count(r.Context())
	if count > 0 {
		writeErr(w, http.StatusConflict, "one of those ports is already connected")
		return
	}
	if in.CableType == "" {
		in.CableType = "cat6"
	}
	p := s.principal(r)
	conn := &domain.Connection{
		APortID: a.ID, BPortID: b.ID, CableType: in.CableType, Label: in.Label,
		LengthM: in.LengthM, CreatedBy: &p.UserID,
	}
	if _, err := s.db.NewInsert().Model(conn).Returning("*").Exec(r.Context()); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	s.cableEvent(r.Context(), &p.UserID, "cabled", a, b, in.CableType)
	s.cableEvent(r.Context(), &p.UserID, "cabled", b, a, in.CableType)
	writeJSON(w, http.StatusCreated, conn)
}

func (s *Server) handleDeleteConnection(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	conn := new(domain.Connection)
	if err := s.db.NewSelect().Model(conn).Where("cn.id = ?", id).Scan(r.Context()); err != nil {
		writeErr(w, http.StatusNotFound, "connection not found")
		return
	}
	a, _ := s.loadPort(r.Context(), conn.APortID)
	b, _ := s.loadPort(r.Context(), conn.BPortID)
	if _, err := s.db.NewDelete().Model((*domain.Connection)(nil)).Where("id = ?", id).Exec(r.Context()); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	p := s.principal(r)
	if a != nil && b != nil {
		s.cableEvent(r.Context(), &p.UserID, "uncabled", a, b, conn.CableType)
		s.cableEvent(r.Context(), &p.UserID, "uncabled", b, a, conn.CableType)
	}
	writeJSON(w, http.StatusNoContent, nil)
}

// cableEvent records a (un)cabling on `near`'s asset timeline, naming the far end.
func (s *Server) cableEvent(ctx context.Context, actor *uuid.UUID, kind string, near, far *domain.Port, cable string) {
	farAsset := ""
	if far.Asset != nil {
		farAsset = far.Asset.AssetTag
	}
	verb := "Cabled"
	if kind == "uncabled" {
		verb = "Disconnected"
	}
	s.recordAssetEvent(ctx, near.AssetID, actor, assetEvent{
		Kind:    kind,
		Summary: fmt.Sprintf("%s %s ↔ %s/%s", verb, near.Name, farAsset, far.Name),
		Data: map[string]any{
			"port": near.Name, "cable_type": cable,
			"peer_asset": farAsset, "peer_port": far.Name,
		},
	})
}
