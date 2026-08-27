package rbac

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"itam/internal/auth"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/uptrace/bun"
)

// Grant is a resolved (permission, scope) pair for a user.
type Grant struct {
	Permission string `json:"permission"`
	ScopeType  string `json:"scope_type"` // global|location|org_unit
	ScopePath  string `json:"scope_path"`
}

// Target describes where an action takes place, for scope evaluation.
type Target struct {
	LocationPath string
	OrgPath      string
}

// Service resolves and caches a user's effective grants.
type Service struct {
	db    *bun.DB
	rdb   *redis.Client
	cache time.Duration
}

func New(db *bun.DB, rdb *redis.Client) *Service {
	return &Service{db: db, rdb: rdb, cache: 60 * time.Second}
}

func cacheKey(uid uuid.UUID) string { return "rbac:grants:" + uid.String() }

// Grants returns all resolved grants for a user (cached in Valkey).
func (s *Service) Grants(ctx context.Context, uid uuid.UUID) ([]Grant, error) {
	if s.rdb != nil {
		if raw, err := s.rdb.Get(ctx, cacheKey(uid)).Result(); err == nil {
			var g []Grant
			if json.Unmarshal([]byte(raw), &g) == nil {
				return g, nil
			}
		}
	}

	var grants []Grant
	err := s.db.NewRaw(`
		SELECT p.key AS permission, rg.scope_type, COALESCE(rg.scope_path::text, '') AS scope_path
		FROM iam.role_grants rg
		JOIN iam.role_permissions rp ON rp.role_id = rg.role_id
		JOIN iam.permissions p ON p.id = rp.permission_id
		WHERE rg.user_id = ?`, uid).Scan(ctx, &grants)
	if err != nil {
		return nil, err
	}

	if s.rdb != nil {
		if b, err := json.Marshal(grants); err == nil {
			_ = s.rdb.Set(ctx, cacheKey(uid), b, s.cache).Err()
		}
	}
	return grants, nil
}

// Invalidate drops the cached grants for a user (call after changing grants).
func (s *Service) Invalidate(ctx context.Context, uid uuid.UUID) {
	if s.rdb != nil {
		_ = s.rdb.Del(ctx, cacheKey(uid)).Err()
	}
}

// Permissions returns the distinct permission keys a user holds (any scope).
func (s *Service) Permissions(ctx context.Context, p *auth.Principal) ([]string, error) {
	if p.IsSuperuser {
		var keys []string
		err := s.db.NewRaw(`SELECT key FROM iam.permissions ORDER BY key`).Scan(ctx, &keys)
		return keys, err
	}
	grants, err := s.Grants(ctx, p.UserID)
	if err != nil {
		return nil, err
	}
	set := map[string]struct{}{}
	for _, g := range grants {
		set[g.Permission] = struct{}{}
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	return out, nil
}

// Can reports whether the principal holds a permission anywhere (scope-agnostic).
// Use for list endpoints and global capabilities.
func (s *Service) Can(ctx context.Context, p *auth.Principal, perm string) bool {
	if p == nil {
		return false
	}
	if p.IsSuperuser {
		return true
	}
	grants, err := s.Grants(ctx, p.UserID)
	if err != nil {
		return false
	}
	for _, g := range grants {
		if strings.EqualFold(g.Permission, perm) {
			return true
		}
	}
	return false
}

// CanOn reports whether the principal holds a permission for a specific target,
// honouring hierarchical scope (a grant applies to its subtree).
func (s *Service) CanOn(ctx context.Context, p *auth.Principal, perm string, t Target) bool {
	if p == nil {
		return false
	}
	if p.IsSuperuser {
		return true
	}
	grants, err := s.Grants(ctx, p.UserID)
	if err != nil {
		return false
	}
	for _, g := range grants {
		if !strings.EqualFold(g.Permission, perm) {
			continue
		}
		switch g.ScopeType {
		case "global":
			return true
		case "location":
			if t.LocationPath != "" && ancestorOrSelf(g.ScopePath, t.LocationPath) {
				return true
			}
		case "org_unit":
			if t.OrgPath != "" && ancestorOrSelf(g.ScopePath, t.OrgPath) {
				return true
			}
		}
	}
	return false
}

// ancestorOrSelf reports whether ltree path `anc` is an ancestor of (or equal
// to) `target`, mirroring Postgres `anc @> target`.
func ancestorOrSelf(anc, target string) bool {
	if anc == "" || target == "" {
		return false
	}
	return anc == target || strings.HasPrefix(target, anc+".")
}
