package http

import (
	"net"
	"net/http"

	"itam/internal/domain"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// ---- vlans ---------------------------------------------------------------

func (s *Server) handleListVlans(w http.ResponseWriter, r *http.Request) {
	vlans := []domain.Vlan{}
	if err := s.db.NewSelect().Model(&vlans).Relation("Location").
		Order("vl.vlan_id ASC").Scan(r.Context()); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, vlans)
}

func (s *Server) handleCreateVlan(w http.ResponseWriter, r *http.Request) {
	var in domain.Vlan
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if in.Name == "" || in.VlanID < 1 || in.VlanID > 4094 {
		writeErr(w, http.StatusBadRequest, "name and a vlan_id in 1..4094 are required")
		return
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

func (s *Server) handleUpdateVlan(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	var in domain.Vlan
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	in.ID = id
	if in.Attributes == nil {
		in.Attributes = map[string]any{}
	}
	if _, err := s.db.NewUpdate().Model(&in).
		Column("vlan_id", "name", "description", "location_id", "attributes").
		Set("updated_at = now()").Where("id = ?", id).Exec(r.Context()); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, in)
}

func (s *Server) handleDeleteVlan(w http.ResponseWriter, r *http.Request) {
	s.deleteByID(w, r, (*domain.Vlan)(nil))
}

// ---- subnets -------------------------------------------------------------

type subnetOut struct {
	domain.Subnet
	UsedCount int `json:"used_count"`
	Capacity  int `json:"capacity"` // usable hosts; -1 when too large to count (e.g. IPv6)
}

// capacity returns usable host count for a CIDR, or -1 if it's impractically large.
func capacity(cidr string) int {
	_, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return -1
	}
	ones, bits := ipnet.Mask.Size()
	hostBits := bits - ones
	if hostBits > 20 { // > ~1M hosts: don't bother
		return -1
	}
	n := 1 << uint(hostBits)
	if n <= 2 {
		return n // /31, /32
	}
	return n - 2 // minus network + broadcast
}

func (s *Server) handleListSubnets(w http.ResponseWriter, r *http.Request) {
	subnets := []domain.Subnet{}
	q := s.db.NewSelect().Model(&subnets).Relation("Vlan").Relation("Location").Order("sn.cidr ASC")
	if loc := r.URL.Query().Get("location_id"); loc != "" {
		q = q.Where("sn.location_id = ?", loc)
	}
	if err := q.Scan(r.Context()); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	// Used counts in one grouped query.
	counts := map[string]int{}
	var rows []struct {
		SubnetID uuid.UUID `bun:"subnet_id"`
		N        int       `bun:"n"`
	}
	_ = s.db.NewSelect().Model((*domain.IPAddress)(nil)).
		ColumnExpr("subnet_id").ColumnExpr("count(*) AS n").
		Where("subnet_id IS NOT NULL").GroupExpr("subnet_id").Scan(r.Context(), &rows)
	for _, row := range rows {
		counts[row.SubnetID.String()] = row.N
	}
	out := make([]subnetOut, len(subnets))
	for i, sn := range subnets {
		out[i] = subnetOut{Subnet: sn, UsedCount: counts[sn.ID.String()], Capacity: capacity(sn.CIDR)}
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleGetSubnet(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	sn := new(domain.Subnet)
	if err := s.db.NewSelect().Model(sn).Relation("Vlan").Relation("Location").
		Where("sn.id = ?", id).Scan(r.Context()); err != nil {
		writeErr(w, http.StatusNotFound, "subnet not found")
		return
	}
	ips := []domain.IPAddress{}
	_ = s.db.NewSelect().Model(&ips).Relation("Asset").Relation("Port").
		Where("ip.subnet_id = ?", id).OrderExpr("ip.address ASC").Scan(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{
		"subnet":    sn,
		"used":      len(ips),
		"capacity":  capacity(sn.CIDR),
		"addresses": ips,
	})
}

func (s *Server) handleCreateSubnet(w http.ResponseWriter, r *http.Request) {
	var in domain.Subnet
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if in.Name == "" || in.CIDR == "" {
		writeErr(w, http.StatusBadRequest, "name and cidr are required")
		return
	}
	if _, _, err := net.ParseCIDR(in.CIDR); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid cidr (use e.g. 10.0.0.0/24)")
		return
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

func (s *Server) handleUpdateSubnet(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	var in domain.Subnet
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	in.ID = id
	if in.Attributes == nil {
		in.Attributes = map[string]any{}
	}
	if _, err := s.db.NewUpdate().Model(&in).
		Column("name", "vlan_id", "location_id", "gateway", "description", "attributes").
		Set("updated_at = now()").Where("id = ?", id).Exec(r.Context()); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, in)
}

func (s *Server) handleDeleteSubnet(w http.ResponseWriter, r *http.Request) {
	s.deleteByID(w, r, (*domain.Subnet)(nil))
}

// handleNextFreeIP returns the first unallocated host in a subnet.
func (s *Server) handleNextFreeIP(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	var addr string
	err = s.db.NewRaw(`
		WITH sub AS (SELECT cidr, gateway FROM ipam.subnets WHERE id = ?)
		SELECT host(c.cand)
		FROM sub,
		     generate_series(1, LEAST((power(2, (32 - masklen(sub.cidr)))::bigint - 2)::int, 4094)) AS g,
		     LATERAL (SELECT (host(network(sub.cidr))::inet + g) AS cand) c
		WHERE NOT EXISTS (SELECT 1 FROM ipam.ip_addresses a WHERE a.address = c.cand)
		  AND (sub.gateway IS NULL OR c.cand <> sub.gateway)
		ORDER BY g
		LIMIT 1`, id).Scan(r.Context(), &addr)
	if err != nil || addr == "" {
		writeErr(w, http.StatusNotFound, "no free address available in this subnet")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"address": addr})
}

// ---- ip addresses --------------------------------------------------------

func (s *Server) handleListIPs(w http.ResponseWriter, r *http.Request) {
	ips := []domain.IPAddress{}
	q := s.db.NewSelect().Model(&ips).Relation("Subnet").Relation("Asset").Relation("Port").
		OrderExpr("ip.address ASC")
	if v := r.URL.Query().Get("subnet_id"); v != "" {
		q = q.Where("ip.subnet_id = ?", v)
	}
	if v := r.URL.Query().Get("asset_id"); v != "" {
		q = q.Where("ip.asset_id = ?", v)
	}
	if v := r.URL.Query().Get("port_id"); v != "" {
		q = q.Where("ip.port_id = ?", v)
	}
	if err := q.Scan(r.Context()); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, ips)
}

func (s *Server) handleCreateIP(w http.ResponseWriter, r *http.Request) {
	var in domain.IPAddress
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if in.Address == "" {
		writeErr(w, http.StatusBadRequest, "address is required")
		return
	}
	if net.ParseIP(hostPart(in.Address)) == nil {
		writeErr(w, http.StatusBadRequest, "invalid IP address")
		return
	}
	if in.Status == "" {
		in.Status = "allocated"
	}
	if in.Attributes == nil {
		in.Attributes = map[string]any{}
	}
	// Auto-detect the containing subnet when none was supplied.
	if in.SubnetID == nil {
		var sid uuid.UUID
		if err := s.db.NewRaw(
			"SELECT id FROM ipam.subnets WHERE ?::inet << cidr ORDER BY masklen(cidr) DESC LIMIT 1",
			in.Address,
		).Scan(r.Context(), &sid); err == nil {
			in.SubnetID = &sid
		}
	}
	if _, err := s.db.NewInsert().Model(&in).Returning("*").Exec(r.Context()); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, in)
}

func (s *Server) handleUpdateIP(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	var in domain.IPAddress
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	in.ID = id
	if in.Attributes == nil {
		in.Attributes = map[string]any{}
	}
	if _, err := s.db.NewUpdate().Model(&in).
		Column("subnet_id", "asset_id", "port_id", "status", "dns_name", "mac", "description", "attributes").
		Set("updated_at = now()").Where("id = ?", id).Exec(r.Context()); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, in)
}

func (s *Server) handleDeleteIP(w http.ResponseWriter, r *http.Request) {
	s.deleteByID(w, r, (*domain.IPAddress)(nil))
}

// hostPart strips an optional /prefix so net.ParseIP can validate the host.
func hostPart(addr string) string {
	for i := 0; i < len(addr); i++ {
		if addr[i] == '/' {
			return addr[:i]
		}
	}
	return addr
}
