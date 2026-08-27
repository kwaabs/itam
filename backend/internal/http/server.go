package http

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"reflect"
	"time"

	"itam/internal/auth"
	"itam/internal/config"
	"itam/internal/events"
	"itam/internal/metadata"
	"itam/internal/rbac"
	"itam/internal/settings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/redis/go-redis/v9"
	"github.com/uptrace/bun"
)

// Server holds all dependencies for the HTTP API.
type Server struct {
	cfg  config.Config
	db   *bun.DB
	rdb  *redis.Client
	bus  *events.Bus
	meta *metadata.Service
	rbac *rbac.Service
	auth *auth.Service
	set  *settings.Service
	log  *slog.Logger
}

func NewServer(cfg config.Config, db *bun.DB, rdb *redis.Client, bus *events.Bus,
	meta *metadata.Service, rb *rbac.Service, au *auth.Service, set *settings.Service, log *slog.Logger) *Server {
	return &Server{cfg: cfg, db: db, rdb: rdb, bus: bus, meta: meta, rbac: rb, auth: au, set: set, log: log}
}

// Router builds the chi router.
func (s *Server) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   s.cfg.CORSOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Authorization", "Content-Type"},
		AllowCredentials: true,
	}))

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("ok")) })
	r.Get("/readyz", s.handleReady)

	// Inbound ingestion: machine-to-machine, authenticated by the connector's
	// shared token/HMAC (NOT the user JWT), so it lives outside the /api group.
	r.Post("/ingest/{key}", s.handleIngest)

	r.Route("/api", func(r chi.Router) {
		r.Use(s.auth.Middleware)

		r.Get("/me", s.handleMe)
		r.Get("/field-config", s.handleFieldConfig)

		// Metadata
		r.Route("/metadata", func(r chi.Router) {
			r.Get("/asset-types", s.handleListAssetTypes)
			r.Get("/asset-types/{id}/fields", s.handleAssetTypeFields)
			r.Get("/data-types", s.handleListDataTypes)
			r.Get("/location-kinds", s.handleListLocationKinds)
			r.Get("/org-unit-kinds", s.handleListOrgUnitKinds)
			r.Get("/units", s.handleListUnits)
			r.Get("/currencies", s.handleListCurrencies)
			r.Get("/vendor-statuses", s.handleListVendorStatuses)
			r.Get("/port-profiles", s.handleListPortProfiles)
			r.Get("/lifecycles", s.handleListLifecycles)
			r.Get("/lifecycles/{id}", s.handleGetLifecycle)
			r.Get("/relationship-types", s.handleListRelationshipTypes)
			r.Get("/automation-rules", s.handleListAutomationRules)
			r.Get("/transitions/{id}/fields", s.handleListTransitionFields)

			// management (metadata.manage)
			r.With(s.require("metadata.manage")).Post("/currencies", s.handleUpsertCurrency)
			r.With(s.require("metadata.manage")).Post("/vendor-statuses", s.handleUpsertVendorStatus)
			r.With(s.require("metadata.manage")).Post("/port-profiles", s.handleUpsertPortProfile)
			r.With(s.require("metadata.manage")).Delete("/port-profiles/{id}", s.handleDeletePortProfile)
			r.With(s.require("metadata.manage")).Post("/asset-types", s.handleCreateAssetType)
			r.With(s.require("metadata.manage")).Put("/asset-types/{id}", s.handleUpdateAssetType)
			r.With(s.require("metadata.manage")).Delete("/asset-types/{id}", s.handleDeleteAssetType)
			r.With(s.require("metadata.manage")).Post("/field-definitions", s.handleCreateField)
			r.With(s.require("metadata.manage")).Put("/field-definitions/{id}", s.handleUpdateField)
			r.With(s.require("metadata.manage")).Delete("/field-definitions/{id}", s.handleDeleteField)
			r.With(s.require("metadata.manage")).Post("/relationship-types", s.handleCreateRelationshipType)
			r.With(s.require("metadata.manage")).Post("/automation-rules", s.handleCreateAutomationRule)
			r.With(s.require("metadata.manage")).Post("/transitions/{id}/fields", s.handleCreateTransitionField)
			r.With(s.require("metadata.manage")).Put("/transition-fields/{id}", s.handleUpdateTransitionField)
			r.With(s.require("metadata.manage")).Delete("/transition-fields/{id}", s.handleDeleteTransitionField)
		})

		// Assets
		r.Route("/assets", func(r chi.Router) {
			r.With(s.require("asset.read")).Get("/", s.handleListAssets)
			r.With(s.require("asset.read")).Get("/map", s.handleAssetsMap)
			r.With(s.require("asset.write")).Post("/merge", s.handleMergeAssets)
			r.With(s.require("asset.read")).Get("/{id}", s.handleGetAsset)
			r.With(s.require("asset.write")).Post("/", s.handleCreateAsset)
			r.With(s.require("asset.write")).Put("/{id}", s.handleUpdateAsset)
			r.With(s.require("asset.delete")).Delete("/{id}", s.handleDeleteAsset)

			r.With(s.require("asset.read")).Get("/{id}/transitions", s.handleListTransitions)
			r.With(s.require("asset.transition")).Post("/{id}/transition", s.handleTransition)
			r.With(s.require("asset.read")).Get("/{id}/history", s.handleAssetHistory)
			r.With(s.require("asset.read")).Get("/{id}/timeline", s.handleAssetTimeline)

			r.With(s.require("cost.read")).Get("/{id}/costs", s.handleListCosts)
			r.With(s.require("cost.manage")).Post("/{id}/costs", s.handleCreateCost)
			r.With(s.require("cost.manage")).Delete("/{id}/costs/{costID}", s.handleDeleteCost)

			r.With(s.require("asset.assign")).Post("/{id}/assign", s.handleAssign)
			r.With(s.require("asset.assign")).Post("/{id}/return", s.handleReturn)
			r.With(s.require("asset.assign")).Post("/{id}/acknowledge", s.handleAcknowledgeCustody)
			r.With(s.require("asset.assign")).Post("/{id}/transfer", s.handleTransfer)

			r.With(s.require("asset.read")).Get("/{id}/relationships", s.handleListRelationships)
			r.With(s.require("asset.write")).Post("/{id}/relationships", s.handleCreateRelationship)
			r.With(s.require("asset.write")).Delete("/relationships/{relID}", s.handleDeleteRelationship)
		})

		// Stores (stock-holding locations)
		r.With(s.require("hierarchy.read")).Get("/stores", s.handleListStores)

		// Hierarchies
		r.Route("/locations", func(r chi.Router) {
			r.With(s.require("hierarchy.read")).Get("/", s.handleListLocations)
			r.With(s.require("hierarchy.manage")).Post("/", s.handleCreateLocation)
			r.With(s.require("hierarchy.manage")).Put("/{id}", s.handleUpdateLocation)
			r.With(s.require("hierarchy.manage")).Post("/{id}/move", s.handleMoveLocation)
			r.With(s.require("hierarchy.manage")).Delete("/{id}", s.handleDeleteLocation)
		})
		r.Route("/location-aliases", func(r chi.Router) {
			r.With(s.require("hierarchy.read")).Get("/", s.handleListLocationAliases)
			r.With(s.require("hierarchy.manage")).Post("/", s.handleUpsertLocationAlias)
			r.With(s.require("hierarchy.manage")).Delete("/{alias}", s.handleDeleteLocationAlias)
		})
		r.Route("/org-units", func(r chi.Router) {
			r.With(s.require("hierarchy.read")).Get("/", s.handleListOrgUnits)
			r.With(s.require("hierarchy.manage")).Post("/", s.handleCreateOrgUnit)
			r.With(s.require("hierarchy.manage")).Put("/{id}", s.handleUpdateOrgUnit)
			r.With(s.require("hierarchy.manage")).Post("/{id}/move", s.handleMoveOrgUnit)
			r.With(s.require("hierarchy.manage")).Delete("/{id}", s.handleDeleteOrgUnit)
		})
		r.Route("/people", func(r chi.Router) {
			r.With(s.require("hierarchy.read")).Get("/", s.handleListPeople)
			r.With(s.require("hierarchy.manage")).Post("/", s.handleCreatePerson)
			r.With(s.require("hierarchy.manage")).Put("/{id}", s.handleUpdatePerson)
			r.With(s.require("hierarchy.manage")).Delete("/{id}", s.handleDeletePerson)
		})

		// IAM
		r.Route("/iam", func(r chi.Router) {
			r.With(s.require("iam.manage")).Get("/roles", s.handleListRoles)
			r.With(s.require("iam.manage")).Get("/permissions", s.handleListPermissions)
			r.With(s.require("iam.manage")).Get("/grants", s.handleListGrants)
			r.With(s.require("iam.manage")).Post("/grants", s.handleCreateGrant)
			r.With(s.require("iam.manage")).Delete("/grants/{id}", s.handleDeleteGrant)
			r.With(s.require("iam.manage")).Get("/users", s.handleListUsers)
		})

		// Procurement
		r.Route("/procurement", func(r chi.Router) {
			r.With(s.require("procurement.read")).Get("/vendors", s.handleListVendors)
			r.With(s.require("procurement.manage")).Post("/vendors", s.handleCreateVendor)
			r.With(s.require("procurement.manage")).Put("/vendors/{id}", s.handleUpdateVendor)
			r.With(s.require("procurement.manage")).Delete("/vendors/{id}", s.handleDeleteVendor)

			r.With(s.require("procurement.read")).Get("/purchase-orders", s.handleListPOs)
			r.With(s.require("procurement.read")).Get("/purchase-orders/{id}", s.handleGetPO)
			r.With(s.require("procurement.manage")).Post("/purchase-orders", s.handleCreatePO)
			r.With(s.require("procurement.manage")).Put("/purchase-orders/{id}", s.handleUpdatePO)
			r.With(s.require("procurement.manage")).Delete("/purchase-orders/{id}", s.handleDeletePO)
			r.With(s.require("procurement.manage")).Post("/purchase-orders/{id}/approve", s.handleSetPOStatus("approved"))
			r.With(s.require("procurement.manage")).Post("/purchase-orders/{id}/cancel", s.handleSetPOStatus("cancelled"))
			r.With(s.require("procurement.manage")).Post("/purchase-orders/{id}/receive", s.handleReceivePO)
		})

		// Data center (DCIM): racks + mounting
		r.Route("/dcim", func(r chi.Router) {
			r.With(s.require("dcim.read")).Get("/racks", s.handleListRacks)
			r.With(s.require("dcim.read")).Get("/racks/{id}", s.handleGetRack)
			r.With(s.require("dcim.manage")).Post("/racks", s.handleCreateRack)
			r.With(s.require("dcim.manage")).Put("/racks/{id}", s.handleUpdateRack)
			r.With(s.require("dcim.manage")).Delete("/racks/{id}", s.handleDeleteRack)
			r.With(s.require("dcim.manage")).Post("/racks/{id}/move", s.handleMoveRack)
			r.With(s.require("dcim.manage")).Post("/racks/{id}/mounts", s.handleMountAsset)
			r.With(s.require("dcim.manage")).Delete("/mounts/{mountID}", s.handleUnmount)
			r.With(s.require("dcim.read")).Get("/assets/{id}/placement", s.handleAssetPlacement)
			r.With(s.require("dcim.manage")).Post("/racks/{id}/position", s.handleSetRackPosition)

			// Ports + cabling
			r.With(s.require("dcim.read")).Get("/ports", s.handleListPorts)
			r.With(s.require("dcim.read")).Get("/ports/free", s.handleListFreePorts)
			r.With(s.require("dcim.manage")).Post("/ports", s.handleCreatePort)
			r.With(s.require("dcim.manage")).Put("/ports/{id}", s.handleUpdatePort)
			r.With(s.require("dcim.manage")).Delete("/ports/{id}", s.handleDeletePort)
			r.With(s.require("dcim.manage")).Post("/connections", s.handleCreateConnection)
			r.With(s.require("dcim.manage")).Delete("/connections/{id}", s.handleDeleteConnection)
			r.With(s.require("dcim.read")).Get("/ports/{id}/trace", s.handleCableTrace)
			r.With(s.require("dcim.manage")).Post("/assets/{id}/generate-ports", s.handleGeneratePorts)

			// Connectivity graph
			r.With(s.require("dcim.read")).Get("/topology", s.handleTopology)

			// Power chain: feeds, PDUs and rack power rollups
			r.With(s.require("dcim.read")).Get("/power/feeds", s.handleListFeeds)
			r.With(s.require("dcim.manage")).Post("/power/feeds", s.handleCreateFeed)
			r.With(s.require("dcim.manage")).Put("/power/feeds/{id}", s.handleUpdateFeed)
			r.With(s.require("dcim.manage")).Delete("/power/feeds/{id}", s.handleDeleteFeed)
			r.With(s.require("dcim.read")).Get("/racks/{id}/power", s.handleRackPower)
			r.With(s.require("dcim.read")).Get("/racks/{id}/pdus", s.handleListPDUs)
			r.With(s.require("dcim.manage")).Post("/racks/{id}/pdus", s.handleCreatePDU)
			r.With(s.require("dcim.manage")).Put("/pdus/{pduID}", s.handleUpdatePDU)
			r.With(s.require("dcim.manage")).Delete("/pdus/{pduID}", s.handleDeletePDU)

			// Environmental sensors + readings (time-series)
			r.With(s.require("dcim.read")).Get("/sensors", s.handleListSensors)
			r.With(s.require("dcim.manage")).Post("/sensors", s.handleCreateSensor)
			r.With(s.require("dcim.manage")).Put("/sensors/{id}", s.handleUpdateSensor)
			r.With(s.require("dcim.manage")).Delete("/sensors/{id}", s.handleDeleteSensor)
			r.With(s.require("dcim.read")).Get("/sensors/{id}/readings", s.handleListReadings)
			r.With(s.require("dcim.manage")).Post("/sensors/{id}/readings", s.handleAddReading)

			// Capacity dashboards (space / power / weight rollups)
			r.With(s.require("dcim.read")).Get("/capacity", s.handleCapacity)
		})

		// Virtualization: clusters, hosts, VMs
		r.Route("/virt", func(r chi.Router) {
			r.With(s.require("virt.read")).Get("/clusters", s.handleListClusters)
			r.With(s.require("virt.manage")).Post("/clusters", s.handleCreateCluster)
			r.With(s.require("virt.manage")).Put("/clusters/{id}", s.handleUpdateCluster)
			r.With(s.require("virt.manage")).Delete("/clusters/{id}", s.handleDeleteCluster)
			r.With(s.require("virt.read")).Get("/hosts", s.handleListHosts)
			r.With(s.require("virt.manage")).Post("/hosts", s.handleCreateHost)
			r.With(s.require("virt.manage")).Put("/hosts/{id}", s.handleUpdateHost)
			r.With(s.require("virt.manage")).Delete("/hosts/{id}", s.handleDeleteHost)
			r.With(s.require("virt.read")).Get("/vms", s.handleListVMs)
			r.With(s.require("virt.manage")).Post("/vms", s.handleCreateVM)
			r.With(s.require("virt.manage")).Put("/vms/{id}", s.handleUpdateVM)
			r.With(s.require("virt.manage")).Delete("/vms/{id}", s.handleDeleteVM)
		})

		// Storage: pools and volumes/LUNs
		r.Route("/storage", func(r chi.Router) {
			r.With(s.require("storage.read")).Get("/pools", s.handleListPools)
			r.With(s.require("storage.manage")).Post("/pools", s.handleCreatePool)
			r.With(s.require("storage.manage")).Put("/pools/{id}", s.handleUpdatePool)
			r.With(s.require("storage.manage")).Delete("/pools/{id}", s.handleDeletePool)
			r.With(s.require("storage.read")).Get("/volumes", s.handleListVolumes)
			r.With(s.require("storage.manage")).Post("/volumes", s.handleCreateVolume)
			r.With(s.require("storage.manage")).Put("/volumes/{id}", s.handleUpdateVolume)
			r.With(s.require("storage.manage")).Delete("/volumes/{id}", s.handleDeleteVolume)
		})

		// Network / firewall configuration (per-device drill-in)
		r.Route("/netcfg", func(r chi.Router) {
			r.With(s.require("netcfg.read")).Get("/devices", s.handleListNetDevices)
			r.With(s.require("netcfg.read")).Get("/ha-groups", s.handleListHAGroups)
			r.With(s.require("netcfg.manage")).Post("/ha-groups", s.handleCreateHAGroup)
			r.With(s.require("netcfg.manage")).Delete("/ha-groups/{id}", s.handleDeleteHAGroup)
			r.With(s.require("netcfg.manage")).Post("/ha-groups/{id}/members", s.handleAddHAMember)
			r.With(s.require("netcfg.manage")).Delete("/ha-groups/{id}/members/{subID}", s.handleDeleteHAMember)

			r.With(s.require("netcfg.read")).Get("/assets/{id}/zones", s.handleListZones)
			r.With(s.require("netcfg.manage")).Post("/assets/{id}/zones", s.handleCreateZone)
			r.With(s.require("netcfg.manage")).Delete("/assets/{id}/zones/{subID}", s.handleDeleteZone)

			r.With(s.require("netcfg.read")).Get("/assets/{id}/interfaces", s.handleListInterfaces)
			r.With(s.require("netcfg.manage")).Post("/assets/{id}/interfaces", s.handleCreateInterface)
			r.With(s.require("netcfg.manage")).Put("/assets/{id}/interfaces/{subID}", s.handleUpdateInterface)
			r.With(s.require("netcfg.manage")).Delete("/assets/{id}/interfaces/{subID}", s.handleDeleteInterface)

			r.With(s.require("netcfg.read")).Get("/assets/{id}/rules", s.handleListRules)
			r.With(s.require("netcfg.manage")).Post("/assets/{id}/rules", s.handleCreateRule)
			r.With(s.require("netcfg.manage")).Put("/assets/{id}/rules/{subID}", s.handleUpdateRule)
			r.With(s.require("netcfg.manage")).Delete("/assets/{id}/rules/{subID}", s.handleDeleteRule)

			r.With(s.require("netcfg.read")).Get("/assets/{id}/backups", s.handleListBackups)
			r.With(s.require("netcfg.read")).Get("/assets/{id}/backups/diff", s.handleBackupDiff)
			r.With(s.require("netcfg.read")).Get("/assets/{id}/backups/{subID}", s.handleGetBackup)
			r.With(s.require("netcfg.manage")).Post("/assets/{id}/backups", s.handleCreateBackup)
			r.With(s.require("netcfg.manage")).Delete("/assets/{id}/backups/{subID}", s.handleDeleteBackup)

			r.With(s.require("netcfg.manage")).Post("/interfaces/{subID}/register-ip", s.handleRegisterInterfaceIP)

			r.With(s.require("netcfg.read")).Get("/assets/{id}/address-objects", s.handleListAddressObjects)
			r.With(s.require("netcfg.manage")).Post("/assets/{id}/address-objects", s.handleCreateAddressObject)
			r.With(s.require("netcfg.manage")).Delete("/assets/{id}/address-objects/{subID}", s.handleDeleteAddressObject)

			r.With(s.require("netcfg.read")).Get("/assets/{id}/routes", s.handleListRoutes)
			r.With(s.require("netcfg.manage")).Post("/assets/{id}/routes", s.handleCreateRoute)
			r.With(s.require("netcfg.manage")).Delete("/assets/{id}/routes/{subID}", s.handleDeleteRoute)

			r.With(s.require("netcfg.read")).Get("/assets/{id}/nat-rules", s.handleListNATRules)
			r.With(s.require("netcfg.manage")).Post("/assets/{id}/nat-rules", s.handleCreateNATRule)
			r.With(s.require("netcfg.manage")).Delete("/assets/{id}/nat-rules/{subID}", s.handleDeleteNATRule)
		})

		// IPAM: VLANs, subnets and IP addresses
		r.Route("/ipam", func(r chi.Router) {
			r.With(s.require("ipam.read")).Get("/vlans", s.handleListVlans)
			r.With(s.require("ipam.manage")).Post("/vlans", s.handleCreateVlan)
			r.With(s.require("ipam.manage")).Put("/vlans/{id}", s.handleUpdateVlan)
			r.With(s.require("ipam.manage")).Delete("/vlans/{id}", s.handleDeleteVlan)

			r.With(s.require("ipam.read")).Get("/subnets", s.handleListSubnets)
			r.With(s.require("ipam.read")).Get("/subnets/{id}", s.handleGetSubnet)
			r.With(s.require("ipam.read")).Get("/subnets/{id}/next-free", s.handleNextFreeIP)
			r.With(s.require("ipam.manage")).Post("/subnets", s.handleCreateSubnet)
			r.With(s.require("ipam.manage")).Put("/subnets/{id}", s.handleUpdateSubnet)
			r.With(s.require("ipam.manage")).Delete("/subnets/{id}", s.handleDeleteSubnet)

			r.With(s.require("ipam.read")).Get("/ips", s.handleListIPs)
			r.With(s.require("ipam.manage")).Post("/ips", s.handleCreateIP)
			r.With(s.require("ipam.manage")).Put("/ips/{id}", s.handleUpdateIP)
			r.With(s.require("ipam.manage")).Delete("/ips/{id}", s.handleDeleteIP)

			r.With(s.require("ipam.read")).Get("/subnets/{id}/dhcp-scopes", s.handleListDHCPScopes)
			r.With(s.require("ipam.manage")).Post("/dhcp-scopes", s.handleCreateDHCPScope)
			r.With(s.require("ipam.manage")).Put("/dhcp-scopes/{id}", s.handleUpdateDHCPScope)
			r.With(s.require("ipam.manage")).Delete("/dhcp-scopes/{id}", s.handleDeleteDHCPScope)
		})

		// Software & licensing
		r.Route("/software", func(r chi.Router) {
			r.With(s.require("software.read")).Get("/", s.handleListSoftware)
			r.With(s.require("software.read")).Get("/installations", s.handleListInstallations)
			r.With(s.require("software.read")).Get("/licenses", s.handleListLicenses)
			r.With(s.require("software.read")).Get("/licenses/{id}", s.handleGetLicense)
			r.With(s.require("software.read")).Get("/{id}", s.handleGetSoftware)

			r.With(s.require("software.manage")).Post("/", s.handleCreateSoftware)
			r.With(s.require("software.manage")).Put("/{id}", s.handleUpdateSoftware)
			r.With(s.require("software.manage")).Delete("/{id}", s.handleDeleteSoftware)
			r.With(s.require("software.manage")).Post("/{id}/versions", s.handleCreateVersion)
			r.With(s.require("software.manage")).Delete("/versions/{verID}", s.handleDeleteVersion)

			r.With(s.require("software.manage")).Post("/installations", s.handleCreateInstallation)
			r.With(s.require("software.manage")).Delete("/installations/{id}", s.handleDeleteInstallation)

			r.With(s.require("software.manage")).Post("/licenses", s.handleCreateLicense)
			r.With(s.require("software.manage")).Put("/licenses/{id}", s.handleUpdateLicense)
			r.With(s.require("software.manage")).Delete("/licenses/{id}", s.handleDeleteLicense)
			r.With(s.require("software.manage")).Post("/licenses/{id}/assignments", s.handleCreateLicenseAssignment)
			r.With(s.require("software.manage")).Delete("/license-assignments/{asgID}", s.handleDeleteLicenseAssignment)
		})

		// Integrations: ingestion connectors + sync history
		r.Route("/integration", func(r chi.Router) {
			r.With(s.require("integration.read")).Get("/connectors", s.handleListConnectors)
			r.With(s.require("integration.read")).Get("/connectors/{id}", s.handleGetConnector)
			r.With(s.require("integration.read")).Get("/connectors/{id}/runs", s.handleListConnectorRuns)
			r.With(s.require("integration.manage")).Post("/connectors", s.handleCreateConnector)
			r.With(s.require("integration.manage")).Put("/connectors/{id}", s.handleUpdateConnector)
			r.With(s.require("integration.manage")).Post("/connectors/{id}/rotate-secret", s.handleRotateConnectorSecret)
			r.With(s.require("integration.manage")).Delete("/connectors/{id}", s.handleDeleteConnector)

			// Metadata-driven mapping + generic pull/replay/discovery
			r.With(s.require("integration.read")).Get("/connectors/{id}/mappings", s.handleListMappings)
			r.With(s.require("integration.read")).Get("/connectors/{id}/fields", s.handleDiscoverFields)
			r.With(s.require("integration.manage")).Post("/connectors/{id}/mappings", s.handleCreateMapping)
			r.With(s.require("integration.manage")).Put("/mappings/{mid}", s.handleUpdateMapping)
			r.With(s.require("integration.manage")).Delete("/mappings/{mid}", s.handleDeleteMapping)
			r.With(s.require("integration.manage")).Post("/connectors/{id}/run", s.handleRunConnector)
			r.With(s.require("integration.manage")).Post("/connectors/{id}/reprocess", s.handleReprocess)
			r.With(s.require("integration.manage")).Post("/connectors/{id}/test", s.handleTestConnector)
		})

		// Notifications: channels, scheduled checks, delivery log
		r.Route("/notifications", func(r chi.Router) {
			r.With(s.require("notification.read")).Get("/channels", s.handleListChannels)
			r.With(s.require("notification.manage")).Post("/channels", s.handleCreateChannel)
			r.With(s.require("notification.manage")).Put("/channels/{id}", s.handleUpdateChannel)
			r.With(s.require("notification.manage")).Delete("/channels/{id}", s.handleDeleteChannel)
			r.With(s.require("notification.manage")).Post("/channels/{id}/test", s.handleTestChannel)

			r.With(s.require("notification.read")).Get("/checks", s.handleListChecks)
			r.With(s.require("notification.manage")).Post("/checks", s.handleCreateCheck)
			r.With(s.require("notification.manage")).Put("/checks/{id}", s.handleUpdateCheck)
			r.With(s.require("notification.manage")).Delete("/checks/{id}", s.handleDeleteCheck)
			r.With(s.require("notification.manage")).Post("/checks/{id}/run", s.handleRunCheck)

			r.With(s.require("notification.read")).Get("/log", s.handleListNotificationLog)
		})

		// Bulk import (CSV / XLSX). Permission is enforced per target inside the handler.
		r.Route("/import", func(r chi.Router) {
			r.Get("/{target}/template", s.handleImportTemplate)
			r.Post("/{target}", s.handleImportRun)
		})

		// Reporting / analytics dashboards
		r.Route("/reports", func(r chi.Router) {
			r.With(s.require("report.read")).Get("/overview", s.handleReportOverview)
			r.With(s.require("report.read")).Get("/warranty", s.handleReportWarranty)
			r.With(s.require("report.read")).Get("/org-inventory", s.handleReportOrgInventory)
			r.With(s.require("report.read")).Get("/custody", s.handleReportCustody)
			r.With(s.require("report.read")).Get("/data-quality", s.handleReportDataQuality)
			r.With(s.require("asset.write")).Post("/data-quality/fix", s.handleReportDataQualityFix)
			r.With(s.require("report.read")).Get("/unmapped-locations", s.handleReportUnmappedLocations)
			r.With(s.require("report.read")).Get("/setup-health", s.handleReportSetupHealth)
			r.With(s.require("report.read")).Get("/connector-health", s.handleReportConnectorHealth)
			r.With(s.require("report.read")).Get("/export/assets", s.handleExportAssets)
			r.With(s.require("report.read")).Get("/export/custody", s.handleExportCustody)
			r.With(s.require("report.read")).Get("/export/warranty", s.handleExportWarranty)
			r.With(s.require("cost.read")).Get("/export/costs", s.handleExportCosts)
		})

		// Cost module: fleet TCO, depreciation/book value, and budgets
		r.Route("/costs", func(r chi.Router) {
			r.With(s.require("cost.read")).Get("/summary", s.handleCostSummary)
			r.With(s.require("cost.read")).Get("/assets", s.handleCostAssets)
			r.With(s.require("cost.read")).Get("/budgets", s.handleListBudgets)
			r.With(s.require("cost.manage")).Post("/budgets", s.handleUpsertBudget)
			r.With(s.require("cost.manage")).Put("/budgets/{id}", s.handleUpdateBudget)
			r.With(s.require("cost.manage")).Delete("/budgets/{id}", s.handleDeleteBudget)
			r.With(s.require("cost.read")).Get("/depreciation", s.handleListDepreciation)
			r.With(s.require("cost.manage")).Put("/depreciation/{id}", s.handleUpdateDepreciation)
		})

		// Audit
		r.With(s.require("audit.read")).Get("/audit", s.handleListAudit)

		// Settings (non-secret values only over the API)
		r.With(s.require("settings.read")).Get("/settings", s.handleListSettings)
		r.With(s.require("settings.manage")).Put("/settings/{key}", s.handleUpdateSetting)
		r.With(s.require("settings.manage")).Put("/settings/{key}/secret", s.handleUpdateSecret)
	})

	// Public SSO status (reads GoTrue settings; OAuth flows use GoTrue directly).
	r.Get("/auth/sso/status", s.handleSSOStatus)

	// Local (password) auth.
	r.Post("/auth/login", s.handleLogin)
	r.Post("/auth/refresh", s.handleRefresh)
	r.Post("/auth/logout", s.handleLogout)

	// Native Azure AD (Entra ID) SSO - replaces GoTrue's OAuth proxy.
	r.Get("/auth/azure/start", s.handleAzureStart)
	r.Get("/auth/azure/callback", s.handleAzureCallback)

	return r
}

func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	if err := s.db.PingContext(ctx); err != nil {
		writeErr(w, http.StatusServiceUnavailable, "database unavailable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

// ---- helpers --------------------------------------------------------------

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if v == nil {
		return
	}
	// Encode a nil slice as `[]` rather than `null` so clients can always iterate
	// list responses safely.
	if rv := reflect.ValueOf(v); rv.Kind() == reflect.Slice && rv.IsNil() {
		_, _ = w.Write([]byte("[]"))
		return
	}
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func decode(r *http.Request, v any) error {
	defer r.Body.Close()
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

func (s *Server) principal(r *http.Request) *auth.Principal {
	p, _ := auth.FromContext(r.Context())
	return p
}

// require is a scope-agnostic permission gate for routes.
func (s *Server) require(perm string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p := s.principal(r)
			if !s.rbac.Can(r.Context(), p, perm) {
				writeErr(w, http.StatusForbidden, "missing permission: "+perm)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
