import 'package:flutter/material.dart';

import 'package:provider/provider.dart';



import '../core/app_state.dart';

import '../data/repository.dart';

import 'audit_screen.dart';

import 'checkout_screen.dart';

import 'create_asset_screen.dart';

import 'custody_tasks_screen.dart';

import 'field_hub_screen.dart';

import 'nearby_screen.dart';

import 'peripherals_screen.dart';

import 'recent_screen.dart';

import 'scan_screen.dart';

import 'search_screen.dart';

import 'stock_screen.dart';

import 'stores_screen.dart';



class HomeScreen extends StatefulWidget {

  const HomeScreen({super.key});



  @override

  State<HomeScreen> createState() => _HomeScreenState();

}



class _HomeScreenState extends State<HomeScreen> {

  int _custodyExceptions = 0;



  @override

  void initState() {

    super.initState();

    _loadCustodySummary();

  }



  Future<void> _loadCustodySummary() async {

    final me = context.read<AppController>().me;

    if (!(me?.can('report.read') ?? false)) return;

    try {

      final r = await Repository(context.read<AppController>().dio).custodyReport();

      if (mounted) setState(() => _custodyExceptions = r.exceptionCount);

    } catch (_) {}

  }



  @override

  Widget build(BuildContext context) {

    final app = context.watch<AppController>();

    final me = app.me;



    final canRead = me?.can('asset.read') ?? false;

    final canAssign = me?.can('asset.assign') ?? false;

    final canWrite = me?.can('asset.write') ?? false;

    final canReceive = me?.can('procurement.manage') ?? false;

    final canReport = me?.can('report.read') ?? false;



    void go(Widget screen) =>

        Navigator.push(context, MaterialPageRoute(builder: (_) => screen)).then((_) => _loadCustodySummary());



    final tiles = <_Action>[

      _Action(

        'Scan',

        Icons.qr_code_scanner,

        true,

        () => go(FieldHubScreen(

          title: 'Scan',

          subtitle: 'Identify an asset or update custody.',

          entries: [

            HubEntry(

              title: 'Scan barcode / QR',

              subtitle: 'Open asset details from a label',

              icon: Icons.qr_code_scanner,

              enabled: true,

              screen: const ScanScreen(),

            ),

            HubEntry(

              title: 'Check in / out',

              subtitle: 'Assign to someone or return to stock',

              icon: Icons.swap_horiz,

              enabled: canAssign,

              screen: const CheckoutScreen(),

            ),

          ],

        )),

      ),

      _Action(

        'Find',

        Icons.search,

        canRead,

        () => go(FieldHubScreen(

          title: 'Find',

          subtitle: 'Search and browse when you do not have a label.',

          entries: [

            HubEntry(

              title: 'Search assets',

              subtitle: 'Tag, name, or serial',

              icon: Icons.search,

              enabled: canRead,

              screen: const SearchScreen(),

            ),

            HubEntry(

              title: 'Peripherals',

              subtitle: 'Monitors, keyboards, docks, etc.',

              icon: Icons.monitor,

              enabled: canRead,

              screen: const PeripheralsScreen(),

            ),

            HubEntry(

              title: 'Recent',

              subtitle: 'Assets you opened recently on this device',

              icon: Icons.history,

              enabled: true,

              screen: const RecentScreen(),

            ),

          ],

        )),

      ),

      _Action(

        'Location',

        Icons.place,

        canRead,

        () => go(FieldHubScreen(

          title: 'Location',

          subtitle: 'See or verify what is at a site.',

          entries: [

            HubEntry(

              title: 'Stores',

              subtitle: 'Browse stock by store or warehouse',

              icon: Icons.warehouse,

              enabled: canRead,

              screen: const StoresScreen(),

            ),

            HubEntry(

              title: 'Near me',

              subtitle: 'Assets at the closest site to your GPS',

              icon: Icons.my_location,

              enabled: canRead,

              screen: const NearbyScreen(),

            ),

            HubEntry(

              title: 'Audit / stocktake',

              subtitle: 'Scan everything on site and reconcile',

              icon: Icons.fact_check,

              enabled: canRead,

              screen: const AuditScreen(),

            ),

          ],

        )),

      ),

      _Action(

        'Add inventory',

        Icons.add_circle_outline,

        canWrite || canReceive,

        () => go(FieldHubScreen(

          title: 'Add inventory',

          subtitle: 'Bring new items into the system.',

          entries: [

            HubEntry(

              title: 'Receive from PO',

              subtitle: 'Goods arriving against a purchase order',

              icon: Icons.inventory_2,

              enabled: canReceive,

              screen: const StockScreen(),

            ),

            HubEntry(

              title: 'Register new asset',

              subtitle: 'One-off entry without a PO',

              icon: Icons.add_box,

              enabled: canWrite,

              screen: const CreateAssetScreen(),

            ),

          ],

        )),

      ),

      _Action(

        'Custody tasks',

        Icons.pending_actions,

        canReport,

        () => go(const CustodyTasksScreen()),

        badge: canReport && _custodyExceptions > 0 ? _custodyExceptions : null,

      ),

    ];



    return Scaffold(

      appBar: AppBar(

        title: const Text('ITAM Field'),

        actions: [

          PopupMenuButton<String>(

            onSelected: (v) {

              if (v == 'logout') app.logout();

              if (v == 'settings') app.backToSetup();

            },

            itemBuilder: (_) => [

              PopupMenuItem(

                value: 'who',

                enabled: false,

                child: Text(me?.email ?? '', style: const TextStyle(fontSize: 12)),

              ),

              const PopupMenuDivider(),

              const PopupMenuItem(value: 'settings', child: Text('Connection settings')),

              const PopupMenuItem(value: 'logout', child: Text('Sign out')),

            ],

          ),

        ],

      ),

      body: Padding(

        padding: const EdgeInsets.all(20),

        child: Column(

          crossAxisAlignment: CrossAxisAlignment.start,

          children: [

            if (canReport && _custodyExceptions > 0) ...[

              Material(

                color: const Color(0xFF422006),

                borderRadius: BorderRadius.circular(12),

                child: InkWell(

                  borderRadius: BorderRadius.circular(12),

                  onTap: () => go(const CustodyTasksScreen()),

                  child: Padding(

                    padding: const EdgeInsets.all(14),

                    child: Row(

                      children: [

                        const Icon(Icons.pending_actions, color: Color(0xFFfbbf24)),

                        const SizedBox(width: 12),

                        Expanded(

                          child: Text(

                            '$_custodyExceptions custody exception${_custodyExceptions == 1 ? '' : 's'} need attention',

                            style: const TextStyle(fontWeight: FontWeight.w500),

                          ),

                        ),

                        const Icon(Icons.chevron_right, color: Colors.white54),

                      ],

                    ),

                  ),

                ),

              ),

              const SizedBox(height: 16),

            ],

            Text('What do you want to do?',

                style: Theme.of(context).textTheme.titleMedium?.copyWith(color: Colors.white70)),

            const SizedBox(height: 16),

            Expanded(

              child: GridView.count(

                crossAxisCount: 2,

                mainAxisSpacing: 14,

                crossAxisSpacing: 14,

                childAspectRatio: 1.15,

                children: tiles.map((t) => _ActionCard(action: t)).toList(),

              ),

            ),

          ],

        ),

      ),

    );

  }

}



class _Action {

  final String label;

  final IconData icon;

  final bool enabled;

  final VoidCallback onTap;

  final int? badge;

  _Action(this.label, this.icon, this.enabled, this.onTap, {this.badge});

}



class _ActionCard extends StatelessWidget {

  final _Action action;

  const _ActionCard({required this.action});



  @override

  Widget build(BuildContext context) {

    final c = Theme.of(context).colorScheme;

    return Opacity(

      opacity: action.enabled ? 1 : 0.4,

      child: Material(

        color: c.surfaceContainerHighest,

        borderRadius: BorderRadius.circular(16),

        child: InkWell(

          borderRadius: BorderRadius.circular(16),

          onTap: action.enabled ? action.onTap : null,

          child: Stack(

            children: [

              Column(

                mainAxisAlignment: MainAxisAlignment.center,

                children: [

                  Icon(action.icon, size: 44, color: c.primary),

                  const SizedBox(height: 12),

                  Text(

                    action.label,

                    textAlign: TextAlign.center,

                    style: const TextStyle(fontSize: 16, fontWeight: FontWeight.w600),

                  ),

                  if (!action.enabled)

                    const Padding(

                      padding: EdgeInsets.only(top: 4),

                      child: Text('No permission', style: TextStyle(fontSize: 11, color: Colors.white38)),

                    ),

                ],

              ),

              if (action.badge != null)

                Positioned(

                  top: 10,

                  right: 10,

                  child: Container(

                    padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),

                    decoration: BoxDecoration(

                      color: const Color(0xFFef4444),

                      borderRadius: BorderRadius.circular(12),

                    ),

                    child: Text(

                      '${action.badge}',

                      style: const TextStyle(fontSize: 12, fontWeight: FontWeight.bold),

                    ),

                  ),

                ),

            ],

          ),

        ),

      ),

    );

  }

}


