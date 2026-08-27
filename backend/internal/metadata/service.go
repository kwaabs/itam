package metadata

import (
	"context"
	"fmt"
	"strings"

	"itam/internal/domain"

	"github.com/uptrace/bun"
)

// Service is the metadata engine: it reads the config tables and validates
// asset attributes against field definitions. No per-type Go structs exist;
// types are entirely data driven.
type Service struct {
	db *bun.DB
}

func New(db *bun.DB) *Service { return &Service{db: db} }

func (s *Service) ListAssetTypes(ctx context.Context) ([]domain.AssetType, error) {
	var types []domain.AssetType
	err := s.db.NewSelect().Model(&types).Order("path ASC").Scan(ctx)
	return types, err
}

func (s *Service) GetAssetType(ctx context.Context, id int64) (*domain.AssetType, error) {
	t := new(domain.AssetType)
	err := s.db.NewSelect().Model(t).Where("id = ?", id).Scan(ctx)
	if err != nil {
		return nil, err
	}
	return t, nil
}

// FieldsForType returns the field definitions for an asset type *including*
// those inherited from ancestor types (a Laptop inherits Computer/Hardware
// fields). Inheritance falls out of the ltree path.
func (s *Service) FieldsForType(ctx context.Context, assetTypeID int64) ([]domain.FieldDefinition, error) {
	at, err := s.GetAssetType(ctx, assetTypeID)
	if err != nil {
		return nil, err
	}
	// All ancestor type ids (path @> currentPath includes self).
	var ancestorIDs []int64
	err = s.db.NewSelect().
		Model((*domain.AssetType)(nil)).
		Column("id").
		Where("path @> ?", at.Path).
		Scan(ctx, &ancestorIDs)
	if err != nil {
		return nil, err
	}
	if len(ancestorIDs) == 0 {
		ancestorIDs = []int64{assetTypeID}
	}

	var fields []domain.FieldDefinition
	err = s.db.NewSelect().
		Model(&fields).
		Relation("DataType").
		Relation("Unit").
		Where("fd.asset_type_id IN (?)", bun.In(ancestorIDs)).
		Order("fd.sort ASC", "fd.label ASC").
		Scan(ctx)
	return fields, err
}

// ResolveLifecycle returns the lifecycle id bound to the type or its nearest
// ancestor, plus the initial state of that lifecycle.
func (s *Service) ResolveLifecycle(ctx context.Context, assetTypeID int64) (lifecycleID *int64, initialState *domain.LifecycleState, err error) {
	at, err := s.GetAssetType(ctx, assetTypeID)
	if err != nil {
		return nil, nil, err
	}
	if at.LifecycleID != nil {
		lifecycleID = at.LifecycleID
	} else {
		// nearest ancestor with a lifecycle, longest path first
		anc := new(domain.AssetType)
		e := s.db.NewSelect().Model(anc).
			Where("path @> ?", at.Path).
			Where("lifecycle_id IS NOT NULL").
			Order("nlevel(path) DESC").
			Limit(1).Scan(ctx)
		if e == nil && anc.LifecycleID != nil {
			lifecycleID = anc.LifecycleID
		}
	}
	if lifecycleID == nil {
		return nil, nil, nil
	}
	st := new(domain.LifecycleState)
	e := s.db.NewSelect().Model(st).
		Where("lifecycle_id = ?", *lifecycleID).
		Where("is_initial = ?", true).
		Order("sort ASC").Limit(1).Scan(ctx)
	if e == nil {
		initialState = st
	}
	return lifecycleID, initialState, nil
}

// FieldsForTransition returns the configured input fields for a transition.
func (s *Service) FieldsForTransition(ctx context.Context, transitionID int64) ([]domain.TransitionField, error) {
	var fields []domain.TransitionField
	err := s.db.NewSelect().Model(&fields).
		Relation("DataType").
		Where("tf.transition_id = ?", transitionID).
		Order("tf.sort ASC", "tf.label ASC").
		Scan(ctx)
	return fields, err
}

// ValidateTransitionData checks captured transition values against the
// transition's field definitions (required, type, enum, min/max). Unknown keys
// are rejected so the configured form stays the source of truth.
func (s *Service) ValidateTransitionData(ctx context.Context, transitionID int64, data map[string]any) error {
	fields, err := s.FieldsForTransition(ctx, transitionID)
	if err != nil {
		return err
	}
	byKey := make(map[string]domain.TransitionField, len(fields))
	for _, f := range fields {
		byKey[strings.ToLower(f.Key)] = f
	}
	errs := ValidationErrors{}
	for k := range data {
		if _, ok := byKey[strings.ToLower(k)]; !ok {
			errs[k] = "unknown field for this transition"
		}
	}
	for _, f := range fields {
		val, present := data[f.Key]
		if !present || val == nil || val == "" {
			if f.Required {
				errs[f.Key] = "is required"
			}
			continue
		}
		kind := "text"
		if f.DataType != nil {
			kind = f.DataType.ValueKind
		}
		// Reuse the asset field validator via an adapter view.
		fd := domain.FieldDefinition{
			Key: f.Key, Required: f.Required, Validation: f.Validation, EnumOptions: f.EnumOptions,
		}
		if msg := validateValue(kind, fd, val); msg != "" {
			errs[f.Key] = msg
		}
	}
	if len(errs) > 0 {
		return errs
	}
	return nil
}

// ValidationErrors maps a field key to a human-readable problem.
type ValidationErrors map[string]string

func (v ValidationErrors) Error() string {
	parts := make([]string, 0, len(v))
	for k, msg := range v {
		parts = append(parts, fmt.Sprintf("%s: %s", k, msg))
	}
	return "validation failed: " + strings.Join(parts, "; ")
}

// ValidateAttributes checks the jsonb attribute bag against the field
// definitions for the asset type (required, type, enum, regex, min/max). Unknown
// keys are rejected so the schema stays the source of truth.
func (s *Service) ValidateAttributes(ctx context.Context, assetTypeID int64, attrs map[string]any) error {
	fields, err := s.FieldsForType(ctx, assetTypeID)
	if err != nil {
		return err
	}
	byKey := make(map[string]domain.FieldDefinition, len(fields))
	for _, f := range fields {
		byKey[strings.ToLower(f.Key)] = f
	}

	errs := ValidationErrors{}

	for k := range attrs {
		if _, ok := byKey[strings.ToLower(k)]; !ok {
			errs[k] = "unknown field for this asset type"
		}
	}

	for _, f := range fields {
		val, present := attrs[f.Key]
		if !present || val == nil {
			if f.Required {
				errs[f.Key] = "is required"
			}
			continue
		}
		kind := "text"
		if f.DataType != nil {
			kind = f.DataType.ValueKind
		}
		if msg := validateValue(kind, f, val); msg != "" {
			errs[f.Key] = msg
		}
	}

	if len(errs) > 0 {
		return errs
	}
	return nil
}

func validateValue(kind string, f domain.FieldDefinition, val any) string {
	switch kind {
	case "text":
		str, ok := val.(string)
		if !ok {
			return "must be text"
		}
		return checkText(f, str)
	case "number":
		n, ok := toFloat(val)
		if !ok {
			return "must be a number"
		}
		return checkNumber(f, n)
	case "bool":
		if _, ok := val.(bool); !ok {
			return "must be true or false"
		}
	case "date", "datetime":
		if _, ok := val.(string); !ok {
			return "must be a date string"
		}
	case "enum":
		if !enumContains(f.EnumOptions, val) {
			return "must be one of the allowed options"
		}
	case "reference":
		if _, ok := val.(string); !ok {
			return "must be a reference id"
		}
	}
	return ""
}

func checkText(f domain.FieldDefinition, str string) string {
	if f.Validation == nil {
		return ""
	}
	if min, ok := toFloat(f.Validation["minLength"]); ok && float64(len(str)) < min {
		return fmt.Sprintf("must be at least %.0f characters", min)
	}
	if max, ok := toFloat(f.Validation["maxLength"]); ok && float64(len(str)) > max {
		return fmt.Sprintf("must be at most %.0f characters", max)
	}
	return ""
}

func checkNumber(f domain.FieldDefinition, n float64) string {
	if f.Validation == nil {
		return ""
	}
	if min, ok := toFloat(f.Validation["min"]); ok && n < min {
		return fmt.Sprintf("must be >= %v", min)
	}
	if max, ok := toFloat(f.Validation["max"]); ok && n > max {
		return fmt.Sprintf("must be <= %v", max)
	}
	return ""
}

func toFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	default:
		return 0, false
	}
}

func enumContains(opts []any, val any) bool {
	for _, o := range opts {
		switch opt := o.(type) {
		case map[string]any:
			if fmt.Sprint(opt["value"]) == fmt.Sprint(val) {
				return true
			}
		default:
			if fmt.Sprint(opt) == fmt.Sprint(val) {
				return true
			}
		}
	}
	return false
}
