package domain

import (
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

// ---------------------------------------------------------------------------
// meta schema (configuration)
// ---------------------------------------------------------------------------

type DataType struct {
	bun.BaseModel `bun:"table:meta.data_types,alias:dt"`
	ID            int64     `bun:"id,pk,autoincrement" json:"id"`
	Key           string    `bun:"key" json:"key"`
	Label         string    `bun:"label" json:"label"`
	ValueKind     string    `bun:"value_kind" json:"value_kind"`
	Description   string    `bun:"description" json:"description"`
	CreatedAt     time.Time `bun:"created_at,nullzero" json:"created_at"`
	UpdatedAt     time.Time `bun:"updated_at,nullzero" json:"updated_at"`
}

type Unit struct {
	bun.BaseModel `bun:"table:meta.units,alias:u"`
	ID            int64  `bun:"id,pk,autoincrement" json:"id"`
	Key           string `bun:"key" json:"key"`
	Label         string `bun:"label" json:"label"`
	Symbol        string `bun:"symbol" json:"symbol"`
}

type Currency struct {
	bun.BaseModel `bun:"table:meta.currencies,alias:cur"`
	Code          string `bun:"code,pk" json:"code"`
	Name          string `bun:"name" json:"name"`
	Symbol        string `bun:"symbol" json:"symbol"`
	IsDefault     bool   `bun:"is_default" json:"is_default"`
	Enabled       bool   `bun:"enabled" json:"enabled"`
	Sort          int    `bun:"sort" json:"sort"`
}

type Lifecycle struct {
	bun.BaseModel `bun:"table:meta.lifecycles,alias:lc"`
	ID            int64              `bun:"id,pk,autoincrement" json:"id"`
	Key           string             `bun:"key" json:"key"`
	Name          string             `bun:"name" json:"name"`
	Description   string             `bun:"description" json:"description"`
	CreatedAt     time.Time          `bun:"created_at,nullzero" json:"created_at"`
	States        []*LifecycleState  `bun:"rel:has-many,join:id=lifecycle_id" json:"states,omitempty"`
	Transitions   []*LifecycleTrans  `bun:"rel:has-many,join:id=lifecycle_id" json:"transitions,omitempty"`
}

type LifecycleState struct {
	bun.BaseModel `bun:"table:meta.lifecycle_states,alias:ls"`
	ID            int64  `bun:"id,pk,autoincrement" json:"id"`
	LifecycleID   int64  `bun:"lifecycle_id" json:"lifecycle_id"`
	Key           string `bun:"key" json:"key"`
	Label         string `bun:"label" json:"label"`
	IsInitial     bool   `bun:"is_initial" json:"is_initial"`
	IsTerminal    bool   `bun:"is_terminal" json:"is_terminal"`
	Color         string `bun:"color" json:"color"`
	Sort          int    `bun:"sort" json:"sort"`
}

type LifecycleTrans struct {
	bun.BaseModel      `bun:"table:meta.lifecycle_transitions,alias:lt"`
	ID                 int64    `bun:"id,pk,autoincrement" json:"id"`
	LifecycleID        int64    `bun:"lifecycle_id" json:"lifecycle_id"`
	Key                string   `bun:"key" json:"key"`
	Label              string   `bun:"label" json:"label"`
	FromStateID        *int64   `bun:"from_state_id" json:"from_state_id"`
	ToStateID          int64    `bun:"to_state_id" json:"to_state_id"`
	RequiredPermission string   `bun:"required_permission" json:"required_permission"`
	RequiredFields     []string `bun:"required_fields,type:jsonb" json:"required_fields"`
	Guard              map[string]any `bun:"guard,type:jsonb" json:"guard"`
	EmitSubject        string   `bun:"emit_subject" json:"emit_subject"`
	Sort               int      `bun:"sort" json:"sort"`
}

// TransitionField is a configurable input collected when a transition runs.
type TransitionField struct {
	bun.BaseModel `bun:"table:meta.transition_fields,alias:tf"`
	ID            int64          `bun:"id,pk,autoincrement" json:"id"`
	TransitionID  int64          `bun:"transition_id" json:"transition_id"`
	Key           string         `bun:"key" json:"key"`
	Label         string         `bun:"label" json:"label"`
	DataTypeID    int64          `bun:"data_type_id" json:"data_type_id"`
	Required      bool           `bun:"required" json:"required"`
	DefaultValue  any            `bun:"default_value,type:jsonb" json:"default_value"`
	Validation    map[string]any `bun:"validation,type:jsonb" json:"validation"`
	EnumOptions   []any          `bun:"enum_options,type:jsonb" json:"enum_options"`
	HelpText      string         `bun:"help_text" json:"help_text"`
	Sort          int            `bun:"sort" json:"sort"`

	DataType *DataType `bun:"rel:belongs-to,join:data_type_id=id" json:"data_type,omitempty"`
}

type AssetType struct {
	bun.BaseModel `bun:"table:meta.asset_types,alias:at"`
	ID            int64     `bun:"id,pk,autoincrement" json:"id"`
	Key           string    `bun:"key" json:"key"`
	Name          string    `bun:"name" json:"name"`
	ParentID      *int64    `bun:"parent_id" json:"parent_id"`
	Path          string    `bun:"path" json:"path"`
	LifecycleID   *int64    `bun:"lifecycle_id" json:"lifecycle_id"`
	Icon          string    `bun:"icon" json:"icon"`
	IsAbstract    bool      `bun:"is_abstract" json:"is_abstract"`
	Sort          int       `bun:"sort" json:"sort"`
	UsefulLifeMonths int    `bun:"useful_life_months" json:"useful_life_months"`
	CreatedAt     time.Time `bun:"created_at,nullzero" json:"created_at"`
	UpdatedAt     time.Time `bun:"updated_at,nullzero" json:"updated_at"`
}

type Budget struct {
	bun.BaseModel `bun:"table:fin.budgets,alias:b"`
	ID            uuid.UUID  `bun:"id,pk,type:uuid,default:gen_random_uuid()" json:"id"`
	OrgUnitID     uuid.UUID  `bun:"org_unit_id" json:"org_unit_id"`
	PeriodYear    int        `bun:"period_year" json:"period_year"`
	Amount        float64    `bun:"amount" json:"amount"`
	Currency      string     `bun:"currency" json:"currency"`
	Notes         string     `bun:"notes,nullzero" json:"notes"`
	CreatedBy     *uuid.UUID `bun:"created_by" json:"created_by"`
	CreatedAt     time.Time  `bun:"created_at,nullzero" json:"created_at"`
	UpdatedAt     time.Time  `bun:"updated_at,nullzero" json:"updated_at"`

	OrgUnit *OrgUnit `bun:"rel:belongs-to,join:org_unit_id=id" json:"org_unit,omitempty"`
}

type FieldDefinition struct {
	bun.BaseModel   `bun:"table:meta.field_definitions,alias:fd"`
	ID              int64          `bun:"id,pk,autoincrement" json:"id"`
	AssetTypeID     int64          `bun:"asset_type_id" json:"asset_type_id"`
	Key             string         `bun:"key" json:"key"`
	Label           string         `bun:"label" json:"label"`
	DataTypeID      int64          `bun:"data_type_id" json:"data_type_id"`
	Required        bool           `bun:"required" json:"required"`
	IsUnique        bool           `bun:"is_unique" json:"is_unique"`
	DefaultValue    any            `bun:"default_value,type:jsonb" json:"default_value"`
	Validation      map[string]any `bun:"validation,type:jsonb" json:"validation"`
	UnitID          *int64         `bun:"unit_id" json:"unit_id"`
	EnumOptions     []any          `bun:"enum_options,type:jsonb" json:"enum_options"`
	ReferenceTarget string         `bun:"reference_target" json:"reference_target"`
	Indexed         bool           `bun:"indexed" json:"indexed"`
	HelpText        string         `bun:"help_text" json:"help_text"`
	Sort            int            `bun:"sort" json:"sort"`

	// Joined for convenience (not a column).
	DataType *DataType `bun:"rel:belongs-to,join:data_type_id=id" json:"data_type,omitempty"`
	Unit     *Unit     `bun:"rel:belongs-to,join:unit_id=id" json:"unit,omitempty"`
}

type RelationshipType struct {
	bun.BaseModel `bun:"table:meta.relationship_types,alias:rt"`
	ID            int64  `bun:"id,pk,autoincrement" json:"id"`
	Key           string `bun:"key" json:"key"`
	Label         string `bun:"label" json:"label"`
	InverseLabel  string `bun:"inverse_label" json:"inverse_label"`
	FromTypeKey   string `bun:"from_type_key" json:"from_type_key"`
	ToTypeKey     string `bun:"to_type_key" json:"to_type_key"`
	Cardinality   string `bun:"cardinality" json:"cardinality"`
	Sort          int    `bun:"sort" json:"sort"`
}

type AutomationRule struct {
	bun.BaseModel `bun:"table:meta.automation_rules,alias:ar"`
	ID            int64          `bun:"id,pk,autoincrement" json:"id"`
	Key           string         `bun:"key" json:"key"`
	Name          string         `bun:"name" json:"name"`
	OnSubject     string         `bun:"on_subject" json:"on_subject"`
	Condition     map[string]any `bun:"condition,type:jsonb" json:"condition"`
	Action        map[string]any `bun:"action,type:jsonb" json:"action"`
	Enabled       bool           `bun:"enabled" json:"enabled"`
	Sort          int            `bun:"sort" json:"sort"`
	CreatedAt     time.Time      `bun:"created_at,nullzero" json:"created_at"`
	UpdatedAt     time.Time      `bun:"updated_at,nullzero" json:"updated_at"`
}

// ---------------------------------------------------------------------------
// core schema (the spine)
// ---------------------------------------------------------------------------

type Location struct {
	bun.BaseModel `bun:"table:core.locations,alias:loc"`
	ID            uuid.UUID      `bun:"id,pk,type:uuid,default:gen_random_uuid()" json:"id"`
	Key           string         `bun:"key" json:"key"`
	Name          string         `bun:"name" json:"name"`
	Kind          string         `bun:"kind" json:"kind"`
	ParentID      *uuid.UUID     `bun:"parent_id" json:"parent_id"`
	Path          string         `bun:"path" json:"path"`
	DRRole        string         `bun:"dr_role,nullzero" json:"dr_role"`
	Tier          string         `bun:"tier,nullzero" json:"tier"`
	Timezone      string         `bun:"timezone,nullzero" json:"timezone"`
	Address       string         `bun:"address,nullzero" json:"address"`
	Attributes    map[string]any `bun:"attributes,type:jsonb" json:"attributes"`
	CreatedAt     time.Time      `bun:"created_at,nullzero" json:"created_at"`
	UpdatedAt     time.Time      `bun:"updated_at,nullzero" json:"updated_at"`

	// geog is stored as PostGIS geography; we never insert/update it through the
	// model (handlers build it from lat/long), but loc.* selects it back here.
	Geog      string   `bun:"geog,scanonly" json:"-"`
	Latitude  *float64 `bun:"latitude,scanonly" json:"latitude"`
	Longitude *float64 `bun:"longitude,scanonly" json:"longitude"`
}

type LocationKind struct {
	bun.BaseModel `bun:"table:meta.location_kinds,alias:lk"`
	Key           string `bun:"key,pk" json:"key"`
	Label         string `bun:"label" json:"label"`
	Sort          int    `bun:"sort" json:"sort"`
	Geo           bool   `bun:"geo" json:"geo"`
	Container     bool   `bun:"container" json:"container"`
}

type OrgUnitKind struct {
	bun.BaseModel `bun:"table:meta.org_unit_kinds,alias:ouk"`
	Key           string `bun:"key,pk" json:"key"`
	Label         string `bun:"label" json:"label"`
	Sort          int    `bun:"sort" json:"sort"`
}

type OrgUnit struct {
	bun.BaseModel       `bun:"table:core.org_units,alias:ou"`
	ID                  uuid.UUID  `bun:"id,pk,type:uuid,default:gen_random_uuid()" json:"id"`
	Key                 string     `bun:"key" json:"key"`
	Name                string     `bun:"name" json:"name"`
	Kind                string     `bun:"kind,nullzero" json:"kind"`
	ParentID            *uuid.UUID `bun:"parent_id" json:"parent_id"`
	Path                string     `bun:"path" json:"path"`
	DefaultLocationID   *uuid.UUID `bun:"default_location_id" json:"default_location_id"`
	CreatedAt           time.Time  `bun:"created_at,nullzero" json:"created_at"`
	UpdatedAt           time.Time  `bun:"updated_at,nullzero" json:"updated_at"`

	DefaultLocation *Location `bun:"rel:belongs-to,join:default_location_id=id" json:"default_location,omitempty"`
}

type LocationAlias struct {
	bun.BaseModel `bun:"table:meta.location_aliases,alias:la"`
	Alias         string    `bun:"alias,pk" json:"alias"`
	LocationID    uuid.UUID `bun:"location_id" json:"location_id"`
	Note          string    `bun:"note" json:"note"`
	CreatedAt     time.Time `bun:"created_at,nullzero" json:"created_at"`

	Location *Location `bun:"rel:belongs-to,join:location_id=id" json:"location,omitempty"`
}

type Person struct {
	bun.BaseModel `bun:"table:core.people,alias:p"`
	ID            uuid.UUID  `bun:"id,pk,type:uuid,default:gen_random_uuid()" json:"id"`
	EmployeeNo    string     `bun:"employee_no,nullzero" json:"employee_no"`
	FirstName     string     `bun:"first_name" json:"first_name"`
	LastName      string     `bun:"last_name" json:"last_name"`
	Email         string     `bun:"email,nullzero" json:"email"`
	Title         string     `bun:"title" json:"title"`
	OrgUnitID     *uuid.UUID `bun:"org_unit_id" json:"org_unit_id"`
	ManagerID     *uuid.UUID `bun:"manager_id" json:"manager_id"`
	IsActive      bool       `bun:"is_active" json:"is_active"`
	CreatedAt     time.Time  `bun:"created_at,nullzero" json:"created_at"`
	UpdatedAt     time.Time  `bun:"updated_at,nullzero" json:"updated_at"`
}

type Asset struct {
	bun.BaseModel    `bun:"table:core.assets,alias:a"`
	ID               uuid.UUID      `bun:"id,pk,type:uuid,default:gen_random_uuid()" json:"id"`
	AssetTag         string         `bun:"asset_tag" json:"asset_tag"`
	Serial           string         `bun:"serial,nullzero" json:"serial"`
	Name             string         `bun:"name" json:"name"`
	AssetTypeID      int64          `bun:"asset_type_id" json:"asset_type_id"`
	LifecycleID      *int64         `bun:"lifecycle_id" json:"lifecycle_id"`
	CurrentStateID   *int64         `bun:"current_state_id" json:"current_state_id"`
	LocationID       *uuid.UUID     `bun:"location_id" json:"location_id"`
	OwnerOrgUnitID   *uuid.UUID     `bun:"owner_org_unit_id" json:"owner_org_unit_id"`
	AssignedPersonID *uuid.UUID     `bun:"assigned_person_id" json:"assigned_person_id"`
	Attributes       map[string]any `bun:"attributes,type:jsonb" json:"attributes"`
	PurchaseCost     *float64       `bun:"purchase_cost" json:"purchase_cost"`
	PurchaseDate     *time.Time     `bun:"purchase_date" json:"purchase_date"`
	WarrantyExpiry   *time.Time     `bun:"warranty_expiry" json:"warranty_expiry"`
	Vendor           string         `bun:"vendor" json:"vendor"`
	Notes            string         `bun:"notes" json:"notes"`
	LastSeenAt       *time.Time     `bun:"last_seen_at" json:"last_seen_at"`
	LastSeenLocationID *uuid.UUID   `bun:"last_seen_location_id" json:"last_seen_location_id"`
	LastSeenSource   string         `bun:"last_seen_source,nullzero" json:"last_seen_source"`
	CreatedAt        time.Time      `bun:"created_at,nullzero" json:"created_at"`
	UpdatedAt        time.Time      `bun:"updated_at,nullzero" json:"updated_at"`
	DeletedAt        *time.Time     `bun:"deleted_at,soft_delete,nullzero" json:"deleted_at"`

	Geog      string   `bun:"geog,scanonly" json:"-"`
	Latitude  *float64 `bun:"latitude,scanonly" json:"latitude"`
	Longitude *float64 `bun:"longitude,scanonly" json:"longitude"`

	AssetType    *AssetType      `bun:"rel:belongs-to,join:asset_type_id=id" json:"asset_type,omitempty"`
	LastSeenLocation *Location    `bun:"rel:belongs-to,join:last_seen_location_id=id" json:"last_seen_location,omitempty"`
	CurrentState *LifecycleState `bun:"rel:belongs-to,join:current_state_id=id" json:"current_state,omitempty"`
	Location     *Location       `bun:"rel:belongs-to,join:location_id=id" json:"location,omitempty"`
	OwnerOrgUnit *OrgUnit        `bun:"rel:belongs-to,join:owner_org_unit_id=id" json:"owner_org_unit,omitempty"`
	AssignedTo   *Person         `bun:"rel:belongs-to,join:assigned_person_id=id" json:"assigned_to,omitempty"`
}

type Assignment struct {
	bun.BaseModel    `bun:"table:core.assignments,alias:asg"`
	ID               uuid.UUID  `bun:"id,pk,type:uuid,default:gen_random_uuid()" json:"id"`
	AssetID          uuid.UUID  `bun:"asset_id" json:"asset_id"`
	Kind             string     `bun:"kind" json:"kind"`
	HolderPersonID   *uuid.UUID `bun:"holder_person_id" json:"holder_person_id"`
	HolderOrgUnitID  *uuid.UUID `bun:"holder_org_unit_id" json:"holder_org_unit_id"`
	FromLocationID   *uuid.UUID `bun:"from_location_id" json:"from_location_id"`
	ToLocationID     *uuid.UUID `bun:"to_location_id" json:"to_location_id"`
	AssignedAt       time.Time  `bun:"assigned_at,nullzero" json:"assigned_at"`
	ReturnedAt       *time.Time `bun:"returned_at" json:"returned_at"`
	AssignedBy       *uuid.UUID `bun:"assigned_by" json:"assigned_by"`
	AcknowledgedAt   *time.Time `bun:"acknowledged_at" json:"acknowledged_at"`
	AcknowledgedBy   *uuid.UUID `bun:"acknowledged_by" json:"acknowledged_by"`
	Reason           string     `bun:"reason" json:"reason"`
	CreatedAt        time.Time  `bun:"created_at,nullzero" json:"created_at"`

	HolderPerson  *Person   `bun:"rel:belongs-to,join:holder_person_id=id" json:"holder_person,omitempty"`
	HolderOrgUnit *OrgUnit  `bun:"rel:belongs-to,join:holder_org_unit_id=id" json:"holder_org_unit,omitempty"`
	FromLocation  *Location `bun:"rel:belongs-to,join:from_location_id=id" json:"from_location,omitempty"`
	ToLocation    *Location `bun:"rel:belongs-to,join:to_location_id=id" json:"to_location,omitempty"`
}

type AssetRelationship struct {
	bun.BaseModel      `bun:"table:core.asset_relationships,alias:ar"`
	ID                 uuid.UUID  `bun:"id,pk,type:uuid,default:gen_random_uuid()" json:"id"`
	FromAssetID        uuid.UUID  `bun:"from_asset_id" json:"from_asset_id"`
	ToAssetID          uuid.UUID  `bun:"to_asset_id" json:"to_asset_id"`
	RelationshipTypeID int64      `bun:"relationship_type_id" json:"relationship_type_id"`
	ValidFrom          time.Time  `bun:"valid_from,nullzero" json:"valid_from"`
	ValidTo            *time.Time `bun:"valid_to" json:"valid_to"`
	CreatedAt          time.Time  `bun:"created_at,nullzero" json:"created_at"`
}

type LifecycleHistory struct {
	bun.BaseModel `bun:"table:core.lifecycle_history,alias:lh"`
	ID            uuid.UUID  `bun:"id,pk,type:uuid,default:gen_random_uuid()" json:"id"`
	AssetID       uuid.UUID  `bun:"asset_id" json:"asset_id"`
	FromStateID   *int64     `bun:"from_state_id" json:"from_state_id"`
	ToStateID     int64      `bun:"to_state_id" json:"to_state_id"`
	TransitionID  *int64         `bun:"transition_id" json:"transition_id"`
	Actor         *uuid.UUID     `bun:"actor" json:"actor"`
	Note          string         `bun:"note" json:"note"`
	Data          map[string]any `bun:"data,type:jsonb" json:"data"`
	CreatedAt     time.Time      `bun:"created_at,nullzero" json:"created_at"`
}

// AssetEvent is one row in the append-only asset timeline ledger.
type AssetEvent struct {
	bun.BaseModel `bun:"table:core.asset_events,alias:ae"`
	ID            uuid.UUID      `bun:"id,pk,type:uuid,default:gen_random_uuid()" json:"id"`
	AssetID       uuid.UUID      `bun:"asset_id" json:"asset_id"`
	Kind          string         `bun:"kind" json:"kind"`
	Subject       string         `bun:"subject,nullzero" json:"subject"`
	Actor         *uuid.UUID     `bun:"actor" json:"actor"`
	Summary       string         `bun:"summary,nullzero" json:"summary"`
	Data          map[string]any `bun:"data,type:jsonb" json:"data"`
	RefTable      string         `bun:"ref_table,nullzero" json:"ref_table"`
	RefID         *uuid.UUID     `bun:"ref_id" json:"ref_id"`
	OccurredAt    time.Time      `bun:"occurred_at,nullzero" json:"occurred_at"`
}

// AssetCost is one entry in an asset's lifetime cost ledger.
type AssetCost struct {
	bun.BaseModel `bun:"table:core.asset_costs,alias:ac"`
	ID            uuid.UUID  `bun:"id,pk,type:uuid,default:gen_random_uuid()" json:"id"`
	AssetID       uuid.UUID  `bun:"asset_id" json:"asset_id"`
	Kind          string     `bun:"kind" json:"kind"`
	Amount        float64    `bun:"amount" json:"amount"`
	Currency      string     `bun:"currency" json:"currency"`
	IncurredAt    time.Time  `bun:"incurred_at,nullzero" json:"incurred_at"`
	Vendor        string     `bun:"vendor,nullzero" json:"vendor"`
	Reference     string     `bun:"reference,nullzero" json:"reference"`
	Note          string     `bun:"note,nullzero" json:"note"`
	SourceTable   string     `bun:"source_table,nullzero" json:"source_table"`
	SourceID      *uuid.UUID `bun:"source_id" json:"source_id"`
	CreatedBy     *uuid.UUID `bun:"created_by" json:"created_by"`
	CreatedAt     time.Time  `bun:"created_at,nullzero" json:"created_at"`
}

type Attachment struct {
	bun.BaseModel `bun:"table:core.attachments,alias:att"`
	ID            uuid.UUID  `bun:"id,pk,type:uuid,default:gen_random_uuid()" json:"id"`
	EntityType    string     `bun:"entity_type" json:"entity_type"`
	EntityID      uuid.UUID  `bun:"entity_id" json:"entity_id"`
	Bucket        string     `bun:"bucket" json:"bucket"`
	ObjectKey     string     `bun:"object_key" json:"object_key"`
	Filename      string     `bun:"filename" json:"filename"`
	ContentType   string     `bun:"content_type" json:"content_type"`
	SizeBytes     int64      `bun:"size_bytes" json:"size_bytes"`
	UploadedBy    *uuid.UUID `bun:"uploaded_by" json:"uploaded_by"`
	CreatedAt     time.Time  `bun:"created_at,nullzero" json:"created_at"`
}

// ---------------------------------------------------------------------------
// proc schema (procurement)
// ---------------------------------------------------------------------------

type Vendor struct {
	bun.BaseModel `bun:"table:proc.vendors,alias:v"`
	ID            uuid.UUID `bun:"id,pk,type:uuid,default:gen_random_uuid()" json:"id"`
	Key           string    `bun:"key" json:"key"`
	Name          string    `bun:"name" json:"name"`
	ContactEmail  string    `bun:"contact_email,nullzero" json:"contact_email"`
	ContactPhone  string    `bun:"contact_phone,nullzero" json:"contact_phone"`
	Website       string    `bun:"website,nullzero" json:"website"`
	Notes         string    `bun:"notes,nullzero" json:"notes"`
	IsActive      bool      `bun:"is_active" json:"is_active"`
	Status        string    `bun:"status" json:"status"`
	CreatedAt     time.Time `bun:"created_at,nullzero" json:"created_at"`
	UpdatedAt     time.Time `bun:"updated_at,nullzero" json:"updated_at"`
}

type VendorStatus struct {
	bun.BaseModel `bun:"table:meta.vendor_statuses,alias:vs"`
	Key           string `bun:"key,pk" json:"key"`
	Label         string `bun:"label" json:"label"`
	Color         string `bun:"color" json:"color"`
	BlocksOrders  bool   `bun:"blocks_orders" json:"blocks_orders"`
	Enabled       bool   `bun:"enabled" json:"enabled"`
	Sort          int    `bun:"sort" json:"sort"`
}

type PurchaseOrder struct {
	bun.BaseModel `bun:"table:proc.purchase_orders,alias:po"`
	ID            uuid.UUID  `bun:"id,pk,type:uuid,default:gen_random_uuid()" json:"id"`
	PONumber      string     `bun:"po_number" json:"po_number"`
	VendorID      *uuid.UUID `bun:"vendor_id" json:"vendor_id"`
	Status        string     `bun:"status" json:"status"`
	RequesterID   *uuid.UUID `bun:"requester_id" json:"requester_id"`
	ApproverID    *uuid.UUID `bun:"approver_id" json:"approver_id"`
	LocationID    *uuid.UUID `bun:"location_id" json:"location_id"`
	Currency      string     `bun:"currency" json:"currency"`
	OrderedAt     *time.Time `bun:"ordered_at" json:"ordered_at"`
	ExpectedAt    *time.Time `bun:"expected_at" json:"expected_at"`
	Notes         string     `bun:"notes,nullzero" json:"notes"`
	CreatedBy     *uuid.UUID `bun:"created_by" json:"created_by"`
	CreatedAt     time.Time  `bun:"created_at,nullzero" json:"created_at"`
	UpdatedAt     time.Time  `bun:"updated_at,nullzero" json:"updated_at"`

	Vendor   *Vendor   `bun:"rel:belongs-to,join:vendor_id=id" json:"vendor,omitempty"`
	Location *Location `bun:"rel:belongs-to,join:location_id=id" json:"location,omitempty"`
	Lines    []*POLine `bun:"rel:has-many,join:id=po_id" json:"lines,omitempty"`
}

type POLine struct {
	bun.BaseModel `bun:"table:proc.po_lines,alias:pol"`
	ID            uuid.UUID      `bun:"id,pk,type:uuid,default:gen_random_uuid()" json:"id"`
	POID          uuid.UUID      `bun:"po_id" json:"po_id"`
	AssetTypeID   *int64         `bun:"asset_type_id" json:"asset_type_id"`
	ModelID       *int64         `bun:"model_id" json:"model_id"`
	Description   string         `bun:"description" json:"description"`
	Quantity      int            `bun:"quantity" json:"quantity"`
	UnitCost      float64        `bun:"unit_cost" json:"unit_cost"`
	ReceivedQty   int            `bun:"received_qty" json:"received_qty"`
	WarrantyMonths int           `bun:"warranty_months" json:"warranty_months"`
	Attributes    map[string]any `bun:"attributes,type:jsonb" json:"attributes"`
	Sort          int            `bun:"sort" json:"sort"`

	AssetType *AssetType `bun:"rel:belongs-to,join:asset_type_id=id" json:"asset_type,omitempty"`
}

type Receipt struct {
	bun.BaseModel `bun:"table:proc.receipts,alias:rcp"`
	ID            uuid.UUID  `bun:"id,pk,type:uuid,default:gen_random_uuid()" json:"id"`
	POID          *uuid.UUID `bun:"po_id" json:"po_id"`
	ReceivedBy    *uuid.UUID `bun:"received_by" json:"received_by"`
	ReceivedAt    time.Time  `bun:"received_at,nullzero" json:"received_at"`
	LocationID    *uuid.UUID `bun:"location_id" json:"location_id"`
	Notes         string     `bun:"notes,nullzero" json:"notes"`
	CreatedAt     time.Time  `bun:"created_at,nullzero" json:"created_at"`
}

type ReceiptLine struct {
	bun.BaseModel `bun:"table:proc.receipt_lines,alias:rcpl"`
	ID            uuid.UUID  `bun:"id,pk,type:uuid,default:gen_random_uuid()" json:"id"`
	ReceiptID     uuid.UUID  `bun:"receipt_id" json:"receipt_id"`
	POLineID      *uuid.UUID `bun:"po_line_id" json:"po_line_id"`
	Quantity      int        `bun:"quantity" json:"quantity"`
	CreatedAt     time.Time  `bun:"created_at,nullzero" json:"created_at"`
}

// ---------------------------------------------------------------------------
// dcim schema (data-center physical layer)
// ---------------------------------------------------------------------------

type Rack struct {
	bun.BaseModel `bun:"table:dcim.racks,alias:rk"`
	ID            uuid.UUID      `bun:"id,pk,type:uuid,default:gen_random_uuid()" json:"id"`
	Key           string         `bun:"key" json:"key"`
	Name          string         `bun:"name" json:"name"`
	LocationID    *uuid.UUID     `bun:"location_id" json:"location_id"`
	AssetID       *uuid.UUID     `bun:"asset_id" json:"asset_id"`
	UHeight       int            `bun:"u_height" json:"u_height"`
	StartingUnit  int            `bun:"starting_unit" json:"starting_unit"`
	DescUnits     bool           `bun:"desc_units" json:"desc_units"`
	WidthMM       *int           `bun:"width_mm" json:"width_mm"`
	DepthMM       *int           `bun:"depth_mm" json:"depth_mm"`
	PosX          *float64       `bun:"pos_x" json:"pos_x"`
	PosY          *float64       `bun:"pos_y" json:"pos_y"`
	Rotation      int            `bun:"rotation" json:"rotation"`
	PowerCapacityW *int          `bun:"power_capacity_w" json:"power_capacity_w"`
	MaxWeightKg   *float64       `bun:"max_weight_kg" json:"max_weight_kg"`
	Attributes    map[string]any `bun:"attributes,type:jsonb" json:"attributes"`
	Notes         string         `bun:"notes,nullzero" json:"notes"`
	CreatedAt     time.Time      `bun:"created_at,nullzero" json:"created_at"`
	UpdatedAt     time.Time      `bun:"updated_at,nullzero" json:"updated_at"`

	Location *Location    `bun:"rel:belongs-to,join:location_id=id" json:"location,omitempty"`
	Mounts   []*RackMount `bun:"rel:has-many,join:id=rack_id" json:"mounts,omitempty"`
}

type PowerFeed struct {
	bun.BaseModel `bun:"table:dcim.power_feeds,alias:pf"`
	ID            uuid.UUID      `bun:"id,pk,type:uuid,default:gen_random_uuid()" json:"id"`
	Key           string         `bun:"key" json:"key"`
	Name          string         `bun:"name" json:"name"`
	LocationID    *uuid.UUID     `bun:"location_id" json:"location_id"`
	Source        string         `bun:"source" json:"source"`
	CapacityW     *int           `bun:"capacity_w" json:"capacity_w"`
	Voltage       *int           `bun:"voltage" json:"voltage"`
	Phase         string         `bun:"phase,nullzero" json:"phase"`
	Redundancy    string         `bun:"redundancy,nullzero" json:"redundancy"`
	Attributes    map[string]any `bun:"attributes,type:jsonb" json:"attributes"`
	Notes         string         `bun:"notes,nullzero" json:"notes"`
	CreatedAt     time.Time      `bun:"created_at,nullzero" json:"created_at"`
	UpdatedAt     time.Time      `bun:"updated_at,nullzero" json:"updated_at"`

	Location *Location `bun:"rel:belongs-to,join:location_id=id" json:"location,omitempty"`
}

type PDU struct {
	bun.BaseModel `bun:"table:dcim.pdus,alias:pd"`
	ID            uuid.UUID      `bun:"id,pk,type:uuid,default:gen_random_uuid()" json:"id"`
	Key           string         `bun:"key" json:"key"`
	Name          string         `bun:"name" json:"name"`
	RackID        uuid.UUID      `bun:"rack_id" json:"rack_id"`
	AssetID       *uuid.UUID     `bun:"asset_id" json:"asset_id"`
	FeedID        *uuid.UUID     `bun:"feed_id" json:"feed_id"`
	CapacityW     *int           `bun:"capacity_w" json:"capacity_w"`
	Attributes    map[string]any `bun:"attributes,type:jsonb" json:"attributes"`
	CreatedAt     time.Time      `bun:"created_at,nullzero" json:"created_at"`
	UpdatedAt     time.Time      `bun:"updated_at,nullzero" json:"updated_at"`

	Feed  *PowerFeed `bun:"rel:belongs-to,join:feed_id=id" json:"feed,omitempty"`
	Asset *Asset     `bun:"rel:belongs-to,join:asset_id=id" json:"asset,omitempty"`
}

type Sensor struct {
	bun.BaseModel `bun:"table:dcim.sensors,alias:sen"`
	ID            uuid.UUID      `bun:"id,pk,type:uuid,default:gen_random_uuid()" json:"id"`
	Key           string         `bun:"key" json:"key"`
	Name          string         `bun:"name" json:"name"`
	Metric        string         `bun:"metric" json:"metric"`
	Unit          string         `bun:"unit,nullzero" json:"unit"`
	LocationID    *uuid.UUID     `bun:"location_id" json:"location_id"`
	RackID        *uuid.UUID     `bun:"rack_id" json:"rack_id"`
	AssetID       *uuid.UUID     `bun:"asset_id" json:"asset_id"`
	MinThreshold  *float64       `bun:"min_threshold" json:"min_threshold"`
	MaxThreshold  *float64       `bun:"max_threshold" json:"max_threshold"`
	Enabled       bool           `bun:"enabled" json:"enabled"`
	Attributes    map[string]any `bun:"attributes,type:jsonb" json:"attributes"`
	CreatedAt     time.Time      `bun:"created_at,nullzero" json:"created_at"`
	UpdatedAt     time.Time      `bun:"updated_at,nullzero" json:"updated_at"`

	Location *Location `bun:"rel:belongs-to,join:location_id=id" json:"location,omitempty"`
}

type SensorReading struct {
	bun.BaseModel `bun:"table:dcim.sensor_readings,alias:srd"`
	ID            int64     `bun:"id,pk,autoincrement" json:"id"`
	SensorID      uuid.UUID `bun:"sensor_id" json:"sensor_id"`
	Value         float64   `bun:"value" json:"value"`
	Status        string    `bun:"status,nullzero" json:"status"`
	Source        string    `bun:"source" json:"source"`
	TS            time.Time `bun:"ts,nullzero" json:"ts"`
}

// ---------------------------------------------------------------------------
// virt schema (virtualization)
// ---------------------------------------------------------------------------

type Cluster struct {
	bun.BaseModel `bun:"table:virt.clusters,alias:cl"`
	ID            uuid.UUID      `bun:"id,pk,type:uuid,default:gen_random_uuid()" json:"id"`
	Key           string         `bun:"key" json:"key"`
	Name          string         `bun:"name" json:"name"`
	LocationID    *uuid.UUID     `bun:"location_id" json:"location_id"`
	Hypervisor    string         `bun:"hypervisor,nullzero" json:"hypervisor"`
	HA            bool           `bun:"ha" json:"ha"`
	DRS           bool           `bun:"drs" json:"drs"`
	Attributes    map[string]any `bun:"attributes,type:jsonb" json:"attributes"`
	Notes         string         `bun:"notes,nullzero" json:"notes"`
	CreatedAt     time.Time      `bun:"created_at,nullzero" json:"created_at"`
	UpdatedAt     time.Time      `bun:"updated_at,nullzero" json:"updated_at"`

	Location *Location `bun:"rel:belongs-to,join:location_id=id" json:"location,omitempty"`
}

type Host struct {
	bun.BaseModel `bun:"table:virt.hosts,alias:ht"`
	ID            uuid.UUID      `bun:"id,pk,type:uuid,default:gen_random_uuid()" json:"id"`
	AssetID       uuid.UUID      `bun:"asset_id" json:"asset_id"`
	ClusterID     *uuid.UUID     `bun:"cluster_id" json:"cluster_id"`
	Hypervisor    string         `bun:"hypervisor,nullzero" json:"hypervisor"`
	CPUCores      *int           `bun:"cpu_cores" json:"cpu_cores"`
	CPUThreads    *int           `bun:"cpu_threads" json:"cpu_threads"`
	RAMGb         *float64       `bun:"ram_gb" json:"ram_gb"`
	Attributes    map[string]any `bun:"attributes,type:jsonb" json:"attributes"`
	CreatedAt     time.Time      `bun:"created_at,nullzero" json:"created_at"`
	UpdatedAt     time.Time      `bun:"updated_at,nullzero" json:"updated_at"`

	Asset   *Asset   `bun:"rel:belongs-to,join:asset_id=id" json:"asset,omitempty"`
	Cluster *Cluster `bun:"rel:belongs-to,join:cluster_id=id" json:"cluster,omitempty"`
	VMs     []*VM    `bun:"rel:has-many,join:id=host_id" json:"vms,omitempty"`
}

type VM struct {
	bun.BaseModel `bun:"table:virt.vms,alias:vm"`
	ID            uuid.UUID      `bun:"id,pk,type:uuid,default:gen_random_uuid()" json:"id"`
	AssetID       *uuid.UUID     `bun:"asset_id" json:"asset_id"`
	HostID        *uuid.UUID     `bun:"host_id" json:"host_id"`
	ClusterID     *uuid.UUID     `bun:"cluster_id" json:"cluster_id"`
	Name          string         `bun:"name" json:"name"`
	VCPUs         *int           `bun:"vcpus" json:"vcpus"`
	RAMGb         *float64       `bun:"ram_gb" json:"ram_gb"`
	DiskGb        *float64       `bun:"disk_gb" json:"disk_gb"`
	PowerState    string         `bun:"power_state" json:"power_state"`
	GuestOS       string         `bun:"guest_os,nullzero" json:"guest_os"`
	IP            string         `bun:"ip,nullzero" json:"ip"`
	Attributes    map[string]any `bun:"attributes,type:jsonb" json:"attributes"`
	CreatedAt     time.Time      `bun:"created_at,nullzero" json:"created_at"`
	UpdatedAt     time.Time      `bun:"updated_at,nullzero" json:"updated_at"`

	Asset *Asset `bun:"rel:belongs-to,join:asset_id=id" json:"asset,omitempty"`
}

// ---------------------------------------------------------------------------
// stor schema (storage)
// ---------------------------------------------------------------------------

type StoragePool struct {
	bun.BaseModel `bun:"table:stor.pools,alias:sp"`
	ID            uuid.UUID      `bun:"id,pk,type:uuid,default:gen_random_uuid()" json:"id"`
	Key           string         `bun:"key" json:"key"`
	Name          string         `bun:"name" json:"name"`
	ArrayAssetID  uuid.UUID      `bun:"array_asset_id" json:"array_asset_id"`
	RAID          string         `bun:"raid,nullzero" json:"raid"`
	RawGb         *float64       `bun:"raw_gb" json:"raw_gb"`
	UsableGb      *float64       `bun:"usable_gb" json:"usable_gb"`
	Attributes    map[string]any `bun:"attributes,type:jsonb" json:"attributes"`
	CreatedAt     time.Time      `bun:"created_at,nullzero" json:"created_at"`
	UpdatedAt     time.Time      `bun:"updated_at,nullzero" json:"updated_at"`

	Array *Asset `bun:"rel:belongs-to,join:array_asset_id=id" json:"array,omitempty"`
}

type StorageVolume struct {
	bun.BaseModel   `bun:"table:stor.volumes,alias:sv"`
	ID              uuid.UUID      `bun:"id,pk,type:uuid,default:gen_random_uuid()" json:"id"`
	Key             string         `bun:"key" json:"key"`
	Name            string         `bun:"name" json:"name"`
	ArrayAssetID    uuid.UUID      `bun:"array_asset_id" json:"array_asset_id"`
	PoolID          *uuid.UUID     `bun:"pool_id" json:"pool_id"`
	CapacityGb      *float64       `bun:"capacity_gb" json:"capacity_gb"`
	UsedGb          *float64       `bun:"used_gb" json:"used_gb"`
	Protocol        string         `bun:"protocol,nullzero" json:"protocol"`
	AttachedAssetID *uuid.UUID     `bun:"attached_asset_id" json:"attached_asset_id"`
	Attributes      map[string]any `bun:"attributes,type:jsonb" json:"attributes"`
	CreatedAt       time.Time      `bun:"created_at,nullzero" json:"created_at"`
	UpdatedAt       time.Time      `bun:"updated_at,nullzero" json:"updated_at"`

	Pool     *StoragePool `bun:"rel:belongs-to,join:pool_id=id" json:"pool,omitempty"`
	Attached *Asset       `bun:"rel:belongs-to,join:attached_asset_id=id" json:"attached,omitempty"`
}

// ---------------------------------------------------------------------------
// netcfg schema (firewall / network configuration)
// ---------------------------------------------------------------------------

type NetZone struct {
	bun.BaseModel `bun:"table:netcfg.zones,alias:nz"`
	ID            uuid.UUID      `bun:"id,pk,type:uuid,default:gen_random_uuid()" json:"id"`
	AssetID       uuid.UUID      `bun:"asset_id" json:"asset_id"`
	Name          string         `bun:"name" json:"name"`
	Description   string         `bun:"description,nullzero" json:"description"`
	Attributes    map[string]any `bun:"attributes,type:jsonb" json:"attributes"`
	CreatedAt     time.Time      `bun:"created_at,nullzero" json:"created_at"`
}

type NetInterface struct {
	bun.BaseModel `bun:"table:netcfg.interfaces,alias:ni"`
	ID            uuid.UUID      `bun:"id,pk,type:uuid,default:gen_random_uuid()" json:"id"`
	AssetID       uuid.UUID      `bun:"asset_id" json:"asset_id"`
	Name          string         `bun:"name" json:"name"`
	IPCidr        string         `bun:"ip_cidr,nullzero" json:"ip_cidr"`
	ZoneID        *uuid.UUID     `bun:"zone_id" json:"zone_id"`
	IPID          *uuid.UUID     `bun:"ip_id" json:"ip_id"`
	Vlan          *int           `bun:"vlan" json:"vlan"`
	Enabled       bool           `bun:"enabled" json:"enabled"`
	Attributes    map[string]any `bun:"attributes,type:jsonb" json:"attributes"`
	CreatedAt     time.Time      `bun:"created_at,nullzero" json:"created_at"`

	Zone *NetZone   `bun:"rel:belongs-to,join:zone_id=id" json:"zone,omitempty"`
	IP   *IPAddress `bun:"rel:belongs-to,join:ip_id=id" json:"ip,omitempty"`
}

type NetRule struct {
	bun.BaseModel `bun:"table:netcfg.rules,alias:nr"`
	ID            uuid.UUID      `bun:"id,pk,type:uuid,default:gen_random_uuid()" json:"id"`
	AssetID       uuid.UUID      `bun:"asset_id" json:"asset_id"`
	Seq           int            `bun:"seq" json:"seq"`
	Name          string         `bun:"name,nullzero" json:"name"`
	Action        string         `bun:"action" json:"action"`
	SrcZone       string         `bun:"src_zone,nullzero" json:"src_zone"`
	DstZone       string         `bun:"dst_zone,nullzero" json:"dst_zone"`
	Source        string         `bun:"source,nullzero" json:"source"`
	Destination   string         `bun:"destination,nullzero" json:"destination"`
	Service       string         `bun:"service,nullzero" json:"service"`
	Protocol      string         `bun:"protocol,nullzero" json:"protocol"`
	Ports         string         `bun:"ports,nullzero" json:"ports"`
	Enabled       bool           `bun:"enabled" json:"enabled"`
	Attributes    map[string]any `bun:"attributes,type:jsonb" json:"attributes"`
	CreatedAt     time.Time      `bun:"created_at,nullzero" json:"created_at"`
	UpdatedAt     time.Time      `bun:"updated_at,nullzero" json:"updated_at"`
}

type HAGroup struct {
	bun.BaseModel `bun:"table:netcfg.ha_groups,alias:hg"`
	ID            uuid.UUID      `bun:"id,pk,type:uuid,default:gen_random_uuid()" json:"id"`
	Key           string         `bun:"key" json:"key"`
	Name          string         `bun:"name" json:"name"`
	Mode          string         `bun:"mode,nullzero" json:"mode"`
	VIP           string         `bun:"vip,nullzero" json:"vip"`
	Attributes    map[string]any `bun:"attributes,type:jsonb" json:"attributes"`
	CreatedAt     time.Time      `bun:"created_at,nullzero" json:"created_at"`

	Members []*HAMember `bun:"rel:has-many,join:id=group_id" json:"members,omitempty"`
}

type HAMember struct {
	bun.BaseModel `bun:"table:netcfg.ha_members,alias:hm"`
	ID            uuid.UUID  `bun:"id,pk,type:uuid,default:gen_random_uuid()" json:"id"`
	GroupID       uuid.UUID  `bun:"group_id" json:"group_id"`
	AssetID       uuid.UUID  `bun:"asset_id" json:"asset_id"`
	Role          string     `bun:"role,nullzero" json:"role"`
	Priority      *int       `bun:"priority" json:"priority"`

	Asset *Asset `bun:"rel:belongs-to,join:asset_id=id" json:"asset,omitempty"`
}

type ConfigBackup struct {
	bun.BaseModel `bun:"table:netcfg.config_backups,alias:cb"`
	ID            int64     `bun:"id,pk,autoincrement" json:"id"`
	AssetID       uuid.UUID `bun:"asset_id" json:"asset_id"`
	TakenAt       time.Time `bun:"taken_at,nullzero" json:"taken_at"`
	Source        string    `bun:"source" json:"source"`
	Version       string    `bun:"version,nullzero" json:"version"`
	Hash          string    `bun:"hash,nullzero" json:"hash"`
	SizeBytes     *int      `bun:"size_bytes" json:"size_bytes"`
	Content       string    `bun:"content,nullzero" json:"content"`
	Note          string    `bun:"note,nullzero" json:"note"`
}

type RackMount struct {
	bun.BaseModel `bun:"table:dcim.rack_mounts,alias:rm"`
	ID            uuid.UUID  `bun:"id,pk,type:uuid,default:gen_random_uuid()" json:"id"`
	RackID        uuid.UUID  `bun:"rack_id" json:"rack_id"`
	AssetID       uuid.UUID  `bun:"asset_id" json:"asset_id"`
	Position      int        `bun:"position" json:"position"`
	UHeight       int        `bun:"u_height" json:"u_height"`
	Face          string     `bun:"face" json:"face"`
	MountedBy     *uuid.UUID `bun:"mounted_by" json:"mounted_by"`
	MountedAt     time.Time  `bun:"mounted_at,nullzero" json:"mounted_at"`

	Asset *Asset `bun:"rel:belongs-to,join:asset_id=id" json:"asset,omitempty"`
	Rack  *Rack  `bun:"rel:belongs-to,join:rack_id=id" json:"rack,omitempty"`
}

type Port struct {
	bun.BaseModel `bun:"table:dcim.ports,alias:pt"`
	ID            uuid.UUID      `bun:"id,pk,type:uuid,default:gen_random_uuid()" json:"id"`
	AssetID       uuid.UUID      `bun:"asset_id" json:"asset_id"`
	Name          string         `bun:"name" json:"name"`
	PortType      string         `bun:"port_type" json:"port_type"`
	Speed         string         `bun:"speed,nullzero" json:"speed"`
	Attributes    map[string]any `bun:"attributes,type:jsonb" json:"attributes"`
	Sort          int            `bun:"sort" json:"sort"`
	CreatedAt     time.Time      `bun:"created_at,nullzero" json:"created_at"`

	Asset *Asset `bun:"rel:belongs-to,join:asset_id=id" json:"asset,omitempty"`
}

type Connection struct {
	bun.BaseModel `bun:"table:dcim.connections,alias:cn"`
	ID            uuid.UUID  `bun:"id,pk,type:uuid,default:gen_random_uuid()" json:"id"`
	APortID       uuid.UUID  `bun:"a_port_id" json:"a_port_id"`
	BPortID       uuid.UUID  `bun:"b_port_id" json:"b_port_id"`
	CableType     string     `bun:"cable_type" json:"cable_type"`
	Label         string     `bun:"label,nullzero" json:"label"`
	LengthM       *float64   `bun:"length_m" json:"length_m"`
	CreatedBy     *uuid.UUID `bun:"created_by" json:"created_by"`
	CreatedAt     time.Time  `bun:"created_at,nullzero" json:"created_at"`
}

// PortProfile is a per-asset-type template used to auto-generate dcim.ports.
type PortProfile struct {
	bun.BaseModel `bun:"table:meta.port_profiles,alias:pp"`
	ID            uuid.UUID `bun:"id,pk,type:uuid,default:gen_random_uuid()" json:"id"`
	AssetTypeID   int64     `bun:"asset_type_id" json:"asset_type_id"`
	Label         string    `bun:"label" json:"label"`
	PortType      string    `bun:"port_type" json:"port_type"`
	Speed         string    `bun:"speed,nullzero" json:"speed"`
	NamePrefix    string    `bun:"name_prefix" json:"name_prefix"`
	StartIndex    int       `bun:"start_index" json:"start_index"`
	Count         int       `bun:"count" json:"count"`
	Sort          int       `bun:"sort" json:"sort"`
	CreatedAt     time.Time `bun:"created_at,nullzero" json:"created_at"`
}

// ---------------------------------------------------------------------------
// ipam schema (IP address management)
// ---------------------------------------------------------------------------

type DHCPScope struct {
	bun.BaseModel `bun:"table:ipam.dhcp_scopes,alias:ds"`
	ID            uuid.UUID      `bun:"id,pk,type:uuid,default:gen_random_uuid()" json:"id"`
	SubnetID      uuid.UUID      `bun:"subnet_id" json:"subnet_id"`
	Name          string         `bun:"name" json:"name"`
	RangeStart    string         `bun:"range_start" json:"range_start"`
	RangeEnd      string         `bun:"range_end" json:"range_end"`
	Gateway       string         `bun:"gateway,nullzero" json:"gateway"`
	DNS           string         `bun:"dns,nullzero" json:"dns"`
	Domain        string         `bun:"domain,nullzero" json:"domain"`
	LeaseHours    int            `bun:"lease_hours" json:"lease_hours"`
	Enabled       bool           `bun:"enabled" json:"enabled"`
	Attributes    map[string]any `bun:"attributes,type:jsonb" json:"attributes"`
	CreatedAt     time.Time      `bun:"created_at,nullzero" json:"created_at"`
	UpdatedAt     time.Time      `bun:"updated_at,nullzero" json:"updated_at"`
}

// ---------------------------------------------------------------------------
// netcfg additions: address objects, routes, NAT
// ---------------------------------------------------------------------------

type AddressObject struct {
	bun.BaseModel `bun:"table:netcfg.address_objects,alias:ao"`
	ID            uuid.UUID      `bun:"id,pk,type:uuid,default:gen_random_uuid()" json:"id"`
	AssetID       uuid.UUID      `bun:"asset_id" json:"asset_id"`
	Name          string         `bun:"name" json:"name"`
	Kind          string         `bun:"kind" json:"kind"`
	Value         string         `bun:"value" json:"value"`
	Description   string         `bun:"description,nullzero" json:"description"`
	Attributes    map[string]any `bun:"attributes,type:jsonb" json:"attributes"`
	CreatedAt     time.Time      `bun:"created_at,nullzero" json:"created_at"`
}

type Route struct {
	bun.BaseModel `bun:"table:netcfg.routes,alias:rt"`
	ID            uuid.UUID `bun:"id,pk,type:uuid,default:gen_random_uuid()" json:"id"`
	AssetID       uuid.UUID `bun:"asset_id" json:"asset_id"`
	Destination   string    `bun:"destination" json:"destination"`
	NextHop       string    `bun:"next_hop,nullzero" json:"next_hop"`
	Interface     string    `bun:"interface,nullzero" json:"interface"`
	Metric        *int      `bun:"metric" json:"metric"`
	Enabled       bool      `bun:"enabled" json:"enabled"`
	Description   string    `bun:"description,nullzero" json:"description"`
	CreatedAt     time.Time `bun:"created_at,nullzero" json:"created_at"`
}

type NATRule struct {
	bun.BaseModel `bun:"table:netcfg.nat_rules,alias:nat"`
	ID            uuid.UUID `bun:"id,pk,type:uuid,default:gen_random_uuid()" json:"id"`
	AssetID       uuid.UUID `bun:"asset_id" json:"asset_id"`
	Seq           int       `bun:"seq" json:"seq"`
	Name          string    `bun:"name,nullzero" json:"name"`
	NATType       string    `bun:"nat_type" json:"nat_type"`
	OrigSrc       string    `bun:"orig_src,nullzero" json:"orig_src"`
	OrigDst       string    `bun:"orig_dst,nullzero" json:"orig_dst"`
	OrigService   string    `bun:"orig_service,nullzero" json:"orig_service"`
	TransSrc      string    `bun:"trans_src,nullzero" json:"trans_src"`
	TransDst      string    `bun:"trans_dst,nullzero" json:"trans_dst"`
	TransService  string    `bun:"trans_service,nullzero" json:"trans_service"`
	Enabled       bool      `bun:"enabled" json:"enabled"`
	Description   string    `bun:"description,nullzero" json:"description"`
	CreatedAt     time.Time `bun:"created_at,nullzero" json:"created_at"`
}

type Vlan struct {
	bun.BaseModel `bun:"table:ipam.vlans,alias:vl"`
	ID            uuid.UUID      `bun:"id,pk,type:uuid,default:gen_random_uuid()" json:"id"`
	VlanID        int            `bun:"vlan_id" json:"vlan_id"`
	Name          string         `bun:"name" json:"name"`
	Description   string         `bun:"description,nullzero" json:"description"`
	LocationID    *uuid.UUID     `bun:"location_id" json:"location_id"`
	Attributes    map[string]any `bun:"attributes,type:jsonb" json:"attributes"`
	CreatedAt     time.Time      `bun:"created_at,nullzero" json:"created_at"`
	UpdatedAt     time.Time      `bun:"updated_at,nullzero" json:"updated_at"`

	Location *Location `bun:"rel:belongs-to,join:location_id=id" json:"location,omitempty"`
}

type Subnet struct {
	bun.BaseModel `bun:"table:ipam.subnets,alias:sn"`
	ID            uuid.UUID      `bun:"id,pk,type:uuid,default:gen_random_uuid()" json:"id"`
	CIDR          string         `bun:"cidr" json:"cidr"`
	Name          string         `bun:"name" json:"name"`
	VlanID        *uuid.UUID     `bun:"vlan_id" json:"vlan_id"`
	LocationID    *uuid.UUID     `bun:"location_id" json:"location_id"`
	Gateway       string         `bun:"gateway,nullzero" json:"gateway"`
	Description   string         `bun:"description,nullzero" json:"description"`
	Attributes    map[string]any `bun:"attributes,type:jsonb" json:"attributes"`
	CreatedAt     time.Time      `bun:"created_at,nullzero" json:"created_at"`
	UpdatedAt     time.Time      `bun:"updated_at,nullzero" json:"updated_at"`

	Vlan     *Vlan     `bun:"rel:belongs-to,join:vlan_id=id" json:"vlan,omitempty"`
	Location *Location `bun:"rel:belongs-to,join:location_id=id" json:"location,omitempty"`
}

type IPAddress struct {
	bun.BaseModel `bun:"table:ipam.ip_addresses,alias:ip"`
	ID            uuid.UUID      `bun:"id,pk,type:uuid,default:gen_random_uuid()" json:"id"`
	Address       string         `bun:"address" json:"address"`
	SubnetID      *uuid.UUID     `bun:"subnet_id" json:"subnet_id"`
	AssetID       *uuid.UUID     `bun:"asset_id" json:"asset_id"`
	PortID        *uuid.UUID     `bun:"port_id" json:"port_id"`
	Status        string         `bun:"status" json:"status"`
	DNSName       string         `bun:"dns_name,nullzero" json:"dns_name"`
	MAC           string         `bun:"mac,nullzero" json:"mac"`
	Description   string         `bun:"description,nullzero" json:"description"`
	Attributes    map[string]any `bun:"attributes,type:jsonb" json:"attributes"`
	CreatedAt     time.Time      `bun:"created_at,nullzero" json:"created_at"`
	UpdatedAt     time.Time      `bun:"updated_at,nullzero" json:"updated_at"`

	Subnet *Subnet `bun:"rel:belongs-to,join:subnet_id=id" json:"subnet,omitempty"`
	Asset  *Asset  `bun:"rel:belongs-to,join:asset_id=id" json:"asset,omitempty"`
	Port   *Port   `bun:"rel:belongs-to,join:port_id=id" json:"port,omitempty"`
}

// ---------------------------------------------------------------------------
// swm schema (software & licensing)
// ---------------------------------------------------------------------------

type Software struct {
	bun.BaseModel `bun:"table:swm.software,alias:sw"`
	ID            uuid.UUID      `bun:"id,pk,type:uuid,default:gen_random_uuid()" json:"id"`
	Name          string         `bun:"name" json:"name"`
	Publisher     string         `bun:"publisher,nullzero" json:"publisher"`
	Category      string         `bun:"category" json:"category"`
	Description   string         `bun:"description,nullzero" json:"description"`
	Attributes    map[string]any `bun:"attributes,type:jsonb" json:"attributes"`
	CreatedAt     time.Time      `bun:"created_at,nullzero" json:"created_at"`
	UpdatedAt     time.Time      `bun:"updated_at,nullzero" json:"updated_at"`

	Versions []*SoftwareVersion `bun:"rel:has-many,join:id=software_id" json:"versions,omitempty"`
}

type SoftwareVersion struct {
	bun.BaseModel `bun:"table:swm.software_versions,alias:sv"`
	ID            uuid.UUID      `bun:"id,pk,type:uuid,default:gen_random_uuid()" json:"id"`
	SoftwareID    uuid.UUID      `bun:"software_id" json:"software_id"`
	Version       string         `bun:"version" json:"version"`
	ReleaseDate   *time.Time     `bun:"release_date" json:"release_date"`
	EOLDate       *time.Time     `bun:"eol_date" json:"eol_date"`
	Attributes    map[string]any `bun:"attributes,type:jsonb" json:"attributes"`
	CreatedAt     time.Time      `bun:"created_at,nullzero" json:"created_at"`
}

type Installation struct {
	bun.BaseModel `bun:"table:swm.installations,alias:inst"`
	ID            uuid.UUID      `bun:"id,pk,type:uuid,default:gen_random_uuid()" json:"id"`
	SoftwareID    uuid.UUID      `bun:"software_id" json:"software_id"`
	VersionID     *uuid.UUID     `bun:"version_id" json:"version_id"`
	AssetID       uuid.UUID      `bun:"asset_id" json:"asset_id"`
	PersonID      *uuid.UUID     `bun:"person_id" json:"person_id"`
	Source        string         `bun:"source" json:"source"`
	InstalledAt   *time.Time     `bun:"installed_at" json:"installed_at"`
	Attributes    map[string]any `bun:"attributes,type:jsonb" json:"attributes"`
	CreatedAt     time.Time      `bun:"created_at,nullzero" json:"created_at"`

	Software *Software        `bun:"rel:belongs-to,join:software_id=id" json:"software,omitempty"`
	Version  *SoftwareVersion `bun:"rel:belongs-to,join:version_id=id" json:"version,omitempty"`
	Asset    *Asset           `bun:"rel:belongs-to,join:asset_id=id" json:"asset,omitempty"`
}

type License struct {
	bun.BaseModel `bun:"table:swm.licenses,alias:lc"`
	ID            uuid.UUID      `bun:"id,pk,type:uuid,default:gen_random_uuid()" json:"id"`
	SoftwareID    *uuid.UUID     `bun:"software_id" json:"software_id"`
	Name          string         `bun:"name" json:"name"`
	LicenseKey    string         `bun:"license_key,nullzero" json:"license_key"`
	LicenseType   string         `bun:"license_type" json:"license_type"`
	Seats         *int           `bun:"seats" json:"seats"`
	VendorID      *uuid.UUID     `bun:"vendor_id" json:"vendor_id"`
	PurchaseCost  *float64       `bun:"purchase_cost" json:"purchase_cost"`
	Currency      string         `bun:"currency" json:"currency"`
	StartDate     *time.Time     `bun:"start_date" json:"start_date"`
	ExpiryDate    *time.Time     `bun:"expiry_date" json:"expiry_date"`
	Notes         string         `bun:"notes,nullzero" json:"notes"`
	Attributes    map[string]any `bun:"attributes,type:jsonb" json:"attributes"`
	CreatedAt     time.Time      `bun:"created_at,nullzero" json:"created_at"`
	UpdatedAt     time.Time      `bun:"updated_at,nullzero" json:"updated_at"`

	Software *Software `bun:"rel:belongs-to,join:software_id=id" json:"software,omitempty"`
	Vendor   *Vendor   `bun:"rel:belongs-to,join:vendor_id=id" json:"vendor,omitempty"`
}

type LicenseAssignment struct {
	bun.BaseModel  `bun:"table:swm.license_assignments,alias:la"`
	ID             uuid.UUID  `bun:"id,pk,type:uuid,default:gen_random_uuid()" json:"id"`
	LicenseID      uuid.UUID  `bun:"license_id" json:"license_id"`
	AssetID        *uuid.UUID `bun:"asset_id" json:"asset_id"`
	PersonID       *uuid.UUID `bun:"person_id" json:"person_id"`
	InstallationID *uuid.UUID `bun:"installation_id" json:"installation_id"`
	Notes          string     `bun:"notes,nullzero" json:"notes"`
	AssignedAt     time.Time  `bun:"assigned_at,nullzero" json:"assigned_at"`

	Asset  *Asset  `bun:"rel:belongs-to,join:asset_id=id" json:"asset,omitempty"`
	Person *Person `bun:"rel:belongs-to,join:person_id=id" json:"person,omitempty"`
}

// ---------------------------------------------------------------------------
// integration schema (ingestion connectors + sync history)
// ---------------------------------------------------------------------------

type Connector struct {
	bun.BaseModel `bun:"table:integration.connectors,alias:cn"`
	ID            uuid.UUID      `bun:"id,pk,type:uuid,default:gen_random_uuid()" json:"id"`
	Key           string         `bun:"key" json:"key"`
	Name          string         `bun:"name" json:"name"`
	Kind          string         `bun:"kind" json:"kind"`
	Enabled       bool           `bun:"enabled" json:"enabled"`
	Direction     string         `bun:"direction" json:"direction"`
	Schedule      string         `bun:"schedule,nullzero" json:"schedule"`
	Secret        string         `bun:"secret,nullzero" json:"-"` // never serialised
	Config        map[string]any `bun:"config,type:jsonb" json:"config"`
	PullSecret    string         `bun:"pull_secret,nullzero" json:"-"` // outbound OAuth client secret
	LastRunAt     *time.Time     `bun:"last_run_at" json:"last_run_at"`
	LastStatus    string         `bun:"last_status,nullzero" json:"last_status"`
	CreatedAt     time.Time      `bun:"created_at,nullzero" json:"created_at"`
	UpdatedAt     time.Time      `bun:"updated_at,nullzero" json:"updated_at"`
}

// FieldMap is one rule turning a source dot-path into a target field, optionally
// through a named transform (with an argument). Target is a core column name
// (name, serial, asset_tag, last_seen, location, asset_type, email, ...) or an
// "attr.<key>" path that lands in the entity's attributes jsonb.
type FieldMap struct {
	Source    string `json:"source"`
	Target    string `json:"target"`
	Transform string `json:"transform,omitempty"`
	Arg       string `json:"arg,omitempty"`
}

// FilterCond drops records that do not match. Op: eq|ne|contains|exists|not_exists|in.
type FilterCond struct {
	Source string `json:"source"`
	Op     string `json:"op"`
	Value  any    `json:"value,omitempty"`
}

type Mapping struct {
	bun.BaseModel  `bun:"table:integration.mappings,alias:mp"`
	ID             uuid.UUID      `bun:"id,pk,type:uuid,default:gen_random_uuid()" json:"id"`
	ConnectorID    uuid.UUID      `bun:"connector_id" json:"connector_id"`
	SourceObject   string         `bun:"source_object" json:"source_object"`
	TargetEntity   string         `bun:"target_entity" json:"target_entity"`
	Enabled        bool           `bun:"enabled" json:"enabled"`
	Sort           int            `bun:"sort" json:"sort"`
	Identity       string         `bun:"identity" json:"identity"`
	MatchFallbacks []string       `bun:"match_fallbacks,type:jsonb" json:"match_fallbacks"`
	TypeResolution map[string]any `bun:"type_resolution,type:jsonb" json:"type_resolution"`
	Fields         []FieldMap     `bun:"fields,type:jsonb" json:"fields"`
	Filters        []FilterCond   `bun:"filters,type:jsonb" json:"filters"`
	Version        int            `bun:"version" json:"version"`
	CreatedAt      time.Time      `bun:"created_at,nullzero" json:"created_at"`
	UpdatedAt      time.Time      `bun:"updated_at,nullzero" json:"updated_at"`
}

type RawRecord struct {
	bun.BaseModel `bun:"table:integration.raw_records,alias:rr"`
	ID            uuid.UUID      `bun:"id,pk,type:uuid,default:gen_random_uuid()" json:"id"`
	ConnectorID   uuid.UUID      `bun:"connector_id" json:"connector_id"`
	SourceObject  string         `bun:"source_object" json:"source_object"`
	ExternalID    string         `bun:"external_id" json:"external_id"`
	Payload       map[string]any `bun:"payload,type:jsonb" json:"payload"`
	SyncRunID     *uuid.UUID     `bun:"sync_run_id" json:"sync_run_id"`
	Processed     bool           `bun:"processed" json:"processed"`
	Status        string         `bun:"status,nullzero" json:"status"`
	Error         string         `bun:"error,nullzero" json:"error"`
	AssetID       *uuid.UUID     `bun:"asset_id" json:"asset_id"`
	ReceivedAt    time.Time      `bun:"received_at,nullzero" json:"received_at"`
}

type SyncRun struct {
	bun.BaseModel `bun:"table:integration.sync_runs,alias:sr"`
	ID            uuid.UUID      `bun:"id,pk,type:uuid,default:gen_random_uuid()" json:"id"`
	ConnectorID   uuid.UUID      `bun:"connector_id" json:"connector_id"`
	Mode          string         `bun:"mode" json:"mode"`
	Status        string         `bun:"status" json:"status"`
	Seen          int            `bun:"seen" json:"seen"`
	Created       int            `bun:"created" json:"created"`
	Updated       int            `bun:"updated" json:"updated"`
	Skipped       int            `bun:"skipped" json:"skipped"`
	Errors        int            `bun:"errors" json:"errors"`
	Message       string         `bun:"message,nullzero" json:"message"`
	Detail        map[string]any `bun:"detail,type:jsonb" json:"detail"`
	StartedAt     time.Time      `bun:"started_at,nullzero" json:"started_at"`
	FinishedAt    *time.Time     `bun:"finished_at" json:"finished_at"`
}

type AssetIdentity struct {
	bun.BaseModel `bun:"table:core.asset_identities,alias:aid"`
	ID            uuid.UUID      `bun:"id,pk,type:uuid,default:gen_random_uuid()" json:"id"`
	AssetID       uuid.UUID      `bun:"asset_id" json:"asset_id"`
	Source        string         `bun:"source" json:"source"`
	ExternalID    string         `bun:"external_id" json:"external_id"`
	Attributes    map[string]any `bun:"attributes,type:jsonb" json:"attributes"`
	LastSeenAt    *time.Time     `bun:"last_seen_at" json:"last_seen_at"`
	CreatedAt     time.Time      `bun:"created_at,nullzero" json:"created_at"`
}

// ---------------------------------------------------------------------------
// notifications + scheduling (metadata-driven, Phase 2)
// ---------------------------------------------------------------------------

type NotificationChannel struct {
	bun.BaseModel `bun:"table:meta.notification_channels,alias:nch"`
	ID            uuid.UUID      `bun:"id,pk,type:uuid,default:gen_random_uuid()" json:"id"`
	Key           string         `bun:"key" json:"key"`
	Name          string         `bun:"name" json:"name"`
	Type          string         `bun:"type" json:"type"`
	Config        map[string]any `bun:"config,type:jsonb" json:"config"`
	Enabled       bool           `bun:"enabled" json:"enabled"`
	CreatedAt     time.Time      `bun:"created_at,nullzero" json:"created_at"`
	UpdatedAt     time.Time      `bun:"updated_at,nullzero" json:"updated_at"`
}

type ScheduledCheck struct {
	bun.BaseModel   `bun:"table:meta.scheduled_checks,alias:sc"`
	ID              uuid.UUID      `bun:"id,pk,type:uuid,default:gen_random_uuid()" json:"id"`
	Key             string         `bun:"key" json:"key"`
	Name            string         `bun:"name" json:"name"`
	Kind            string         `bun:"kind" json:"kind"`
	IntervalSeconds int            `bun:"interval_seconds" json:"interval_seconds"`
	Params          map[string]any `bun:"params,type:jsonb" json:"params"`
	EventSubject    string         `bun:"event_subject" json:"event_subject"`
	Enabled         bool           `bun:"enabled" json:"enabled"`
	LastRunAt       *time.Time     `bun:"last_run_at" json:"last_run_at"`
	NextRunAt       *time.Time     `bun:"next_run_at" json:"next_run_at"`
	LastStatus      string         `bun:"last_status,nullzero" json:"last_status"`
	LastCount       int            `bun:"last_count" json:"last_count"`
	CreatedAt       time.Time      `bun:"created_at,nullzero" json:"created_at"`
	UpdatedAt       time.Time      `bun:"updated_at,nullzero" json:"updated_at"`
}

type NotificationLog struct {
	bun.BaseModel `bun:"table:meta.notification_log,alias:nlg"`
	ID            int64          `bun:"id,pk,autoincrement" json:"id"`
	ChannelID     *uuid.UUID     `bun:"channel_id" json:"channel_id"`
	ChannelKey    string         `bun:"channel_key,nullzero" json:"channel_key"`
	Subject       string         `bun:"subject,nullzero" json:"subject"`
	Status        string         `bun:"status" json:"status"`
	Error         string         `bun:"error,nullzero" json:"error"`
	Payload       map[string]any `bun:"payload,type:jsonb" json:"payload"`
	CreatedAt     time.Time      `bun:"created_at,nullzero" json:"created_at"`
}

// ---------------------------------------------------------------------------
// iam schema (RBAC)
// ---------------------------------------------------------------------------

type UserProfile struct {
	bun.BaseModel `bun:"table:iam.user_profiles,alias:up"`
	UserID        uuid.UUID  `bun:"user_id,pk,type:uuid" json:"user_id"`
	Email         string     `bun:"email" json:"email"`
	DisplayName   string     `bun:"display_name" json:"display_name"`
	PersonID      *uuid.UUID `bun:"person_id" json:"person_id"`
	IsSuperuser   bool       `bun:"is_superuser" json:"is_superuser"`
	IsActive      bool       `bun:"is_active" json:"is_active"`
	CreatedAt     time.Time  `bun:"created_at,nullzero" json:"created_at"`
	UpdatedAt     time.Time  `bun:"updated_at,nullzero" json:"updated_at"`
}

type Permission struct {
	bun.BaseModel `bun:"table:iam.permissions,alias:perm"`
	ID            int64  `bun:"id,pk,autoincrement" json:"id"`
	Key           string `bun:"key" json:"key"`
	Description   string `bun:"description" json:"description"`
}

type Role struct {
	bun.BaseModel `bun:"table:iam.roles,alias:role"`
	ID            int64     `bun:"id,pk,autoincrement" json:"id"`
	Key           string    `bun:"key" json:"key"`
	Name          string    `bun:"name" json:"name"`
	Description   string    `bun:"description" json:"description"`
	CreatedAt     time.Time `bun:"created_at,nullzero" json:"created_at"`
}

type RolePermission struct {
	bun.BaseModel `bun:"table:iam.role_permissions,alias:rp"`
	RoleID        int64 `bun:"role_id,pk" json:"role_id"`
	PermissionID  int64 `bun:"permission_id,pk" json:"permission_id"`
}

type RoleGrant struct {
	bun.BaseModel `bun:"table:iam.role_grants,alias:rg"`
	ID            uuid.UUID `bun:"id,pk,type:uuid,default:gen_random_uuid()" json:"id"`
	UserID        uuid.UUID `bun:"user_id" json:"user_id"`
	RoleID        int64     `bun:"role_id" json:"role_id"`
	ScopeType     string    `bun:"scope_type" json:"scope_type"`
	ScopePath     string    `bun:"scope_path,nullzero" json:"scope_path"`
	CreatedAt     time.Time `bun:"created_at,nullzero" json:"created_at"`

	Role *Role `bun:"rel:belongs-to,join:role_id=id" json:"role,omitempty"`
}

// ---------------------------------------------------------------------------
// audit schema
// ---------------------------------------------------------------------------

type AuditLog struct {
	bun.BaseModel `bun:"table:audit.audit_log,alias:al"`
	ID            int64          `bun:"id,pk,autoincrement" json:"id"`
	EventID       uuid.UUID      `bun:"event_id,type:uuid" json:"event_id"`
	Subject       string         `bun:"subject" json:"subject"`
	Actor         string         `bun:"actor" json:"actor"`
	EntityType    string         `bun:"entity_type" json:"entity_type"`
	EntityID      string         `bun:"entity_id" json:"entity_id"`
	OccurredAt    time.Time      `bun:"occurred_at" json:"occurred_at"`
	Payload       map[string]any `bun:"payload,type:jsonb" json:"payload"`
	RecordedAt    time.Time      `bun:"recorded_at,nullzero" json:"recorded_at"`
}
