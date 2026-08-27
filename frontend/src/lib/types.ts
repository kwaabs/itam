export interface AssetType {
	id: number;
	key: string;
	name: string;
	parent_id: number | null;
	path: string;
	lifecycle_id: number | null;
	icon: string;
	is_abstract: boolean;
	sort: number;
}

export interface DataType {
	id: number;
	key: string;
	label: string;
	value_kind: string;
}

export interface Unit {
	id: number;
	key: string;
	label: string;
	symbol: string;
}

export interface FieldDefinition {
	id: number;
	asset_type_id: number;
	key: string;
	label: string;
	data_type_id: number;
	required: boolean;
	is_unique: boolean;
	default_value: unknown;
	validation: Record<string, unknown>;
	unit_id: number | null;
	enum_options: unknown[];
	reference_target: string;
	indexed: boolean;
	help_text: string;
	sort: number;
	data_type?: DataType;
	unit?: Unit;
}

export interface LifecycleState {
	id: number;
	lifecycle_id: number;
	key: string;
	label: string;
	is_initial: boolean;
	is_terminal: boolean;
	color: string;
	sort: number;
}

export interface LifecycleTransition {
	id: number;
	lifecycle_id: number;
	key: string;
	label: string;
	from_state_id: number | null;
	to_state_id: number;
	required_permission: string;
	required_fields: string[];
	emit_subject: string;
	sort: number;
	allowed?: boolean;
}

export interface Location {
	id: string;
	key: string;
	name: string;
	kind: string;
	parent_id: string | null;
	path: string;
	dr_role?: string;
	tier?: string;
	timezone?: string;
	address?: string;
	latitude?: number | null;
	longitude?: number | null;
}

export interface AssetMapPoint {
	id: string;
	asset_tag: string;
	name: string;
	type_name: string;
	lat: number;
	lng: number;
	location_name: string;
	source: 'asset' | 'location';
}

export interface LocationKind {
	key: string;
	label: string;
	sort: number;
	geo: boolean;
	container: boolean;
}

export interface OrgUnitKind {
	key: string;
	label: string;
	sort: number;
}

export interface OrgUnit {
	id: string;
	key: string;
	name: string;
	kind: string;
	parent_id: string | null;
	path: string;
	default_location_id: string | null;
	default_location?: Location;
}

export interface LocationAlias {
	alias: string;
	location_id: string;
	note: string;
	location?: Location;
}

export interface Person {
	id: string;
	employee_no: string;
	first_name: string;
	last_name: string;
	email: string;
	title: string;
	org_unit_id: string | null;
	manager_id: string | null;
	is_active: boolean;
}

export interface Asset {
	id: string;
	asset_tag: string;
	serial: string;
	name: string;
	asset_type_id: number;
	lifecycle_id: number | null;
	current_state_id: number | null;
	location_id: string | null;
	owner_org_unit_id: string | null;
	assigned_person_id: string | null;
	attributes: Record<string, unknown>;
	purchase_cost: number | null;
	purchase_date?: string | null;
	warranty_expiry?: string | null;
	vendor: string;
	notes: string;
	created_at: string;
	last_seen_at?: string | null;
	last_seen_location_id?: string | null;
	last_seen_source?: string;
	latitude?: number | null;
	longitude?: number | null;
	asset_type?: AssetType;
	current_state?: LifecycleState;
	location?: Location;
	owner_org_unit?: OrgUnit;
	assigned_to?: Person;
	last_seen_location?: Location;
}

export interface Currency {
	code: string;
	name: string;
	symbol: string;
	is_default: boolean;
	enabled: boolean;
	sort: number;
}

export interface Paginated<T> {
	items: T[];
	total: number;
	page: number;
	page_size: number;
}

export interface Me {
	user_id: string;
	email: string;
	is_superuser: boolean;
	permissions: string[];
}

export interface Lifecycle {
	id: number;
	key: string;
	name: string;
	description: string;
	states?: LifecycleState[];
	transitions?: LifecycleTransition[];
}

export interface RelationshipType {
	id: number;
	key: string;
	label: string;
	inverse_label: string;
	from_type_key: string;
	to_type_key: string;
	cardinality: string;
	sort: number;
}

export interface AutomationRule {
	id: number;
	key: string;
	name: string;
	on_subject: string;
	condition: Record<string, unknown>;
	action: Record<string, unknown>;
	enabled: boolean;
	sort: number;
}

export interface Role {
	id: number;
	key: string;
	name: string;
	description: string;
}

export interface Permission {
	id: number;
	key: string;
	description: string;
}

export interface UserProfile {
	user_id: string;
	email: string;
	display_name: string;
	person_id: string | null;
	is_superuser: boolean;
	is_active: boolean;
}

export interface RoleGrant {
	id: string;
	user_id: string;
	role_id: number;
	scope_type: string;
	scope_path: string;
	created_at: string;
	role?: Role;
}

export interface Vendor {
	id: string;
	key: string;
	name: string;
	contact_email: string;
	contact_phone: string;
	website: string;
	notes: string;
	is_active: boolean;
	status: string;
	po_count?: number;
	total_spent?: number;
	asset_count?: number;
}

export interface VendorStatus {
	key: string;
	label: string;
	color: string;
	blocks_orders: boolean;
	enabled: boolean;
	sort: number;
}

export interface Store extends Location {
	department_count: number;
	in_stock_count: number;
	in_stock_value: number;
}

export interface ReportBucket {
	label: string;
	count: number;
}

export interface ReportOverview {
	total_assets: number;
	by_state: ReportBucket[];
	by_type: ReportBucket[];
	by_site: ReportBucket[];
	warranty: { tracked: number; expired: number; next_30: number; next_90: number };
	financial?: { currency: string; purchase_value: number; ledger_net: number };
	procurement: { open_pos: number; open_value: number; by_status: ReportBucket[] };
}

export interface OrgInventoryReport {
	org_unit: OrgUnit;
	breadcrumb: { key: string; name: string; kind: string; path: string }[];
	total: number;
	by_type: { key: string; label: string; count: number }[];
}

export interface Assignment {
	id: string;
	asset_id: string;
	kind: string;
	holder_person_id: string | null;
	holder_org_unit_id: string | null;
	from_location_id: string | null;
	to_location_id: string | null;
	assigned_at: string;
	returned_at: string | null;
	acknowledged_at?: string | null;
	acknowledged_by?: string | null;
	reason: string;
	holder_person?: Person;
	holder_org_unit?: OrgUnit;
	from_location?: Location;
	to_location?: Location;
}

export interface AssetEvent {
	id: string;
	asset_id: string;
	kind: string;
	summary: string;
	occurred_at: string;
	data: Record<string, unknown>;
}

export interface CustodyReport {
	assigned: number;
	available: number;
	by_holder: { person_id: string; name: string; count: number }[];
	not_in_stock_after_return: { id: string; asset_tag: string; name: string; state_name: string }[];
	unacknowledged?: { id: string; asset_tag: string; name: string; state_name: string }[];
	assigned_in_stock?: { id: string; asset_tag: string; name: string; state_name: string }[];
}

export interface ConnectorHealthRow {
	id: string;
	key: string;
	name: string;
	kind: string;
	enabled: boolean;
	last_run_at: string | null;
	last_status: string;
	runs_24h: number;
	errors_24h: number;
	last_seen: number;
	last_created: number;
	last_updated: number;
}

export interface DataQualityReport {
	missing_location: number;
	missing_org_unit: number;
	missing_serial: number;
	duplicate_serials: { serial: string; count: number }[];
	duplicate_groups?: { serial: string; assets: { id: string; asset_tag: string; name: string; issue: string }[] }[];
	gaps: { id: string; asset_tag: string; name: string; issue: string }[];
}

export interface DataQualityFixResult {
	updated: number;
	skipped: number;
	errors: string[];
}

export interface UnmappedLocationRow {
	label: string;
	source: string;
	asset_count: number;
}

export interface SetupHealthItem {
	key: string;
	label: string;
	ok: boolean;
	count?: number;
	href: string;
}

export interface SetupHealthReport {
	score: number;
	items: SetupHealthItem[];
}

export interface WarrantyRow {
	id: string;
	asset_tag: string;
	name: string;
	warranty_expiry: string;
	location_name: string;
	type_name: string;
	days_left: number;
}

export interface CostAnalytics {
	currency: string;
	totals: {
		total_spend: number;
		proceeds: number;
		net: number;
		capex: number;
		repair: number;
		upgrade: number;
		other: number;
	};
	depreciation: { purchase_value: number; book_value: number; depreciation: number };
	by_type: { label: string; amount: number }[];
	by_site: { label: string; amount: number }[];
	by_vendor: { label: string; amount: number }[];
	monthly: { month: string; amount: number }[];
}

export interface AssetCostRow {
	id: string;
	asset_tag: string;
	name: string;
	type_name: string;
	location_name: string;
	purchase_cost: number;
	purchase_date: string | null;
	net_cost: number;
	repair_cost: number;
	book_value: number;
	age_months: number | null;
}

export interface BudgetRow {
	org_unit_id: string;
	org_unit_name: string;
	path: string;
	budget_id: string;
	budget: number;
	actual: number;
}

export interface DepreciationRow {
	id: number;
	key: string;
	name: string;
	useful_life_months: number;
	asset_count: number;
}

export interface POLine {
	id: string;
	po_id: string;
	asset_type_id: number | null;
	model_id: number | null;
	description: string;
	quantity: number;
	unit_cost: number;
	received_qty: number;
	warranty_months: number;
	attributes: Record<string, unknown>;
	sort: number;
	asset_type?: AssetType;
}

export interface PurchaseOrder {
	id: string;
	po_number: string;
	vendor_id: string | null;
	status: string;
	requester_id: string | null;
	approver_id: string | null;
	location_id: string | null;
	currency: string;
	ordered_at: string | null;
	expected_at: string | null;
	notes: string;
	created_at: string;
	vendor?: Vendor;
	location?: Location;
	lines?: POLine[];
}

export interface RackMount {
	id: string;
	rack_id: string;
	asset_id: string;
	position: number;
	u_height: number;
	face: string;
	mounted_at: string;
	asset?: Asset;
	rack?: Rack;
}

export interface AssetPlacement {
	mount?: RackMount;
	rack_record?: Rack;
	port_count: number;
	power_watts?: number | null;
	rack_units?: number | null;
	weight_kg?: number | null;
}

export interface PortPeer {
	connection_id: string;
	cable_type: string;
	label?: string;
	port_id: string;
	port_name: string;
	asset_id: string;
	asset_tag: string;
	asset_name: string;
}

export interface Port {
	id: string;
	asset_id: string;
	name: string;
	port_type: string;
	speed: string;
	sort: number;
	connection: PortPeer | null;
	asset?: Asset;
}

export interface Rack {
	id: string;
	key: string;
	name: string;
	location_id: string | null;
	asset_id: string | null;
	u_height: number;
	starting_unit: number;
	desc_units: boolean;
	width_mm: number | null;
	depth_mm: number | null;
	pos_x: number | null;
	pos_y: number | null;
	rotation: number;
	power_capacity_w: number | null;
	attributes: Record<string, unknown>;
	notes: string;
	location?: Location;
	mounts?: RackMount[];
}

// --- IPAM ---------------------------------------------------------------

export interface Vlan {
	id: string;
	vlan_id: number;
	name: string;
	description?: string;
	location_id: string | null;
	location?: Location;
}

export interface Subnet {
	id: string;
	cidr: string;
	name: string;
	vlan_id: string | null;
	location_id: string | null;
	gateway?: string;
	description?: string;
	vlan?: Vlan;
	location?: Location;
	used_count?: number;
	capacity?: number;
}

export interface IPAddress {
	id: string;
	address: string;
	subnet_id: string | null;
	asset_id: string | null;
	port_id: string | null;
	status: string;
	dns_name?: string;
	mac?: string;
	description?: string;
	subnet?: Subnet;
	asset?: Asset;
	port?: Port;
}

// --- Network topology ---------------------------------------------------

export interface TopoNode {
	id: string;
	name: string;
	asset_tag: string;
	type_key?: string;
	location?: string;
	vlans?: number[];
	subnets?: string[];
}

export interface PortProfile {
	id: string;
	asset_type_id: number;
	label: string;
	port_type: string;
	speed: string;
	name_prefix: string;
	start_index: number;
	count: number;
	sort: number;
}

export interface CableHop {
	connection_id: string;
	cable_type: string;
	label?: string;
	length_m?: number;
	from_asset_id: string;
	from_asset_tag: string;
	from_asset: string;
	from_port: string;
	to_asset_id: string;
	to_asset_tag: string;
	to_asset: string;
	to_port: string;
	patch_panel: boolean;
}

export interface DHCPScope {
	id: string;
	subnet_id: string;
	name: string;
	range_start: string;
	range_end: string;
	gateway?: string;
	dns?: string;
	domain?: string;
	lease_hours: number;
	enabled: boolean;
	pool_size?: number;
	used?: number;
}

export interface AddressObject {
	id: string;
	asset_id: string;
	name: string;
	kind: string;
	value: string;
	description?: string;
}

export interface NetRoute {
	id: string;
	asset_id: string;
	destination: string;
	next_hop?: string;
	interface?: string;
	metric?: number | null;
	enabled: boolean;
	description?: string;
}

export interface NATRule {
	id: string;
	asset_id: string;
	seq: number;
	name?: string;
	nat_type: string;
	orig_src?: string;
	orig_dst?: string;
	orig_service?: string;
	trans_src?: string;
	trans_dst?: string;
	trans_service?: string;
	enabled: boolean;
	description?: string;
}

export interface BackupDiffLine {
	op: string;
	text: string;
}

export interface TopoLink {
	id: string;
	a_asset_id: string;
	a_port: string;
	b_asset_id: string;
	b_port: string;
	cable_type: string;
}

export interface Topology {
	nodes: TopoNode[];
	links: TopoLink[];
}

// --- Software & licensing -----------------------------------------------

export interface Software {
	id: string;
	name: string;
	publisher?: string;
	category: string;
	description?: string;
	versions?: SoftwareVersion[];
	install_count?: number;
	license_count?: number;
}

export interface SoftwareVersion {
	id: string;
	software_id: string;
	version: string;
	release_date?: string | null;
	eol_date?: string | null;
}

export interface Installation {
	id: string;
	software_id: string;
	version_id: string | null;
	asset_id: string;
	person_id: string | null;
	source: string;
	installed_at?: string | null;
	software?: Software;
	version?: SoftwareVersion;
	asset?: Asset;
}

export interface License {
	id: string;
	software_id: string | null;
	name: string;
	license_key?: string;
	license_type: string;
	seats: number | null;
	vendor_id: string | null;
	purchase_cost?: number | null;
	currency: string;
	start_date?: string | null;
	expiry_date?: string | null;
	notes?: string;
	software?: Software;
	vendor?: Vendor;
	seats_used?: number;
}

export interface LicenseAssignment {
	id: string;
	license_id: string;
	asset_id: string | null;
	person_id: string | null;
	installation_id: string | null;
	notes?: string;
	assigned_at: string;
	asset?: Asset;
	person?: Person;
}

export interface AuditEntry {
	id: number;
	subject: string;
	actor: string;
	entity_type: string;
	entity_id: string;
	occurred_at: string;
	payload: Record<string, unknown>;
}

export interface PowerFeed {
	id: string;
	key: string;
	name: string;
	location_id: string | null;
	source: string;
	capacity_w: number | null;
	voltage: number | null;
	phase?: string;
	redundancy?: string;
	notes?: string;
	location?: Location;
}

export interface PDU {
	id: string;
	key: string;
	name: string;
	rack_id: string;
	asset_id: string | null;
	feed_id: string | null;
	capacity_w: number | null;
	feed?: PowerFeed;
}

export interface RackPower {
	rack_capacity_w: number | null;
	pdu_capacity_w: number;
	capacity_w: number;
	draw_w: number;
	util_pct: number;
	pdus: PDU[];
}

export interface Sensor {
	id: string;
	key: string;
	name: string;
	metric: string;
	unit?: string;
	location_id: string | null;
	rack_id: string | null;
	asset_id: string | null;
	min_threshold: number | null;
	max_threshold: number | null;
	enabled: boolean;
	last_value?: number | null;
	last_ts?: string | null;
	status?: string;
	location?: Location;
}

export interface SensorReading {
	id: number;
	sensor_id: string;
	value: number;
	status?: string;
	source: string;
	ts: string;
}

// --- Virtualization -----------------------------------------------------
export interface Cluster {
	id: string;
	key: string;
	name: string;
	location_id: string | null;
	hypervisor?: string;
	ha: boolean;
	drs: boolean;
	notes?: string;
	location?: Location;
}

export interface VM {
	id: string;
	asset_id: string | null;
	host_id: string | null;
	cluster_id: string | null;
	name: string;
	vcpus: number | null;
	ram_gb: number | null;
	disk_gb: number | null;
	power_state: string;
	guest_os?: string;
	ip?: string;
	attributes?: Record<string, unknown>;
	asset?: Asset;
}

export interface Host {
	id: string;
	asset_id: string;
	cluster_id: string | null;
	hypervisor?: string;
	cpu_cores: number | null;
	cpu_threads: number | null;
	ram_gb: number | null;
	asset?: Asset;
	cluster?: Cluster;
	vms?: VM[];
}

// --- Storage ------------------------------------------------------------
export interface StoragePool {
	id: string;
	key: string;
	name: string;
	array_asset_id: string;
	raid?: string;
	raw_gb: number | null;
	usable_gb: number | null;
	array?: Asset;
}

export interface StorageVolume {
	id: string;
	key: string;
	name: string;
	array_asset_id: string;
	pool_id: string | null;
	capacity_gb: number | null;
	used_gb: number | null;
	protocol?: string;
	attached_asset_id: string | null;
	pool?: StoragePool;
	attached?: Asset;
}

// --- Network / firewall configuration -----------------------------------
export interface NetZone {
	id: string;
	asset_id: string;
	name: string;
	description?: string;
}

export interface NetInterface {
	id: string;
	asset_id: string;
	name: string;
	ip_cidr?: string;
	zone_id: string | null;
	ip_id?: string | null;
	vlan: number | null;
	enabled: boolean;
	zone?: NetZone;
}

export interface NetRule {
	id: string;
	asset_id: string;
	seq: number;
	name?: string;
	action: string;
	src_zone?: string;
	dst_zone?: string;
	source?: string;
	destination?: string;
	service?: string;
	protocol?: string;
	ports?: string;
	enabled: boolean;
}

export interface HAMember {
	id: string;
	group_id: string;
	asset_id: string;
	role?: string;
	priority: number | null;
	asset?: Asset;
}

export interface HAGroup {
	id: string;
	key: string;
	name: string;
	mode?: string;
	vip?: string;
	members?: HAMember[];
}

export interface ConfigBackup {
	id: number;
	asset_id: string;
	taken_at: string;
	source: string;
	version?: string;
	hash?: string;
	size_bytes: number | null;
	content?: string;
	note?: string;
}

// --- Capacity -----------------------------------------------------------
export interface CapacityRow {
	rack_id: string;
	rack_name: string;
	location_id: string | null;
	location_name: string;
	u_total: number;
	u_used: number;
	u_pct: number;
	power_capacity_w: number;
	power_draw_w: number;
	power_pct: number;
	weight_capacity_kg: number | null;
	weight_used_kg: number;
	weight_pct: number;
}

export interface CapacityReport {
	racks: CapacityRow[];
	totals: {
		u_total: number;
		u_used: number;
		power_capacity_w: number;
		power_draw_w: number;
		weight_used_kg: number;
		rack_count: number;
	};
}

export interface Connector {
	id: string;
	key: string;
	name: string;
	kind: string;
	enabled: boolean;
	direction: string;
	schedule?: string | null;
	config: Record<string, unknown>;
	last_run_at?: string | null;
	last_status?: string;
	created_at: string;
	updated_at: string;
}

export interface SyncRun {
	id: string;
	connector_id: string;
	mode: string;
	status: string;
	seen: number;
	created: number;
	updated: number;
	skipped: number;
	errors: number;
	message?: string;
	detail: Record<string, unknown>;
	started_at: string;
	finished_at?: string | null;
}

export interface FieldMap {
	source: string;
	target: string;
	transform?: string;
	arg?: string;
}

export interface FilterCond {
	source: string;
	op: string;
	value?: unknown;
}

export interface Mapping {
	id: string;
	connector_id: string;
	source_object: string;
	target_entity: string;
	enabled: boolean;
	sort: number;
	identity: string;
	match_fallbacks: string[];
	type_resolution: Record<string, unknown>;
	fields: FieldMap[];
	filters: FilterCond[];
	version: number;
}

export interface DiscoveredField {
	path: string;
	sample: string;
	mapped: boolean;
}

export interface NotificationChannel {
	id: string;
	key: string;
	name: string;
	type: string;
	config: Record<string, unknown>;
	enabled: boolean;
	created_at: string;
	updated_at: string;
}

export interface ScheduledCheck {
	id: string;
	key: string;
	name: string;
	kind: string;
	interval_seconds: number;
	params: Record<string, unknown>;
	event_subject: string;
	enabled: boolean;
	last_run_at?: string | null;
	next_run_at?: string | null;
	last_status?: string;
	last_count: number;
	created_at: string;
	updated_at: string;
}

export interface NotificationLogEntry {
	id: number;
	channel_id?: string | null;
	channel_key?: string;
	subject?: string;
	status: string;
	error?: string;
	payload: Record<string, unknown>;
	created_at: string;
}

export interface AutomationRule {
	id: number;
	key: string;
	name: string;
	on_subject: string;
	condition: Record<string, unknown>;
	action: Record<string, unknown>;
	enabled: boolean;
	sort: number;
	created_at?: string;
	updated_at?: string;
}
