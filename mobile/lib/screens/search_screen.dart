import 'package:flutter/material.dart';
import 'package:provider/provider.dart';

import '../core/app_state.dart';
import '../core/models.dart';
import '../data/repository.dart';
import 'widgets.dart';

/// Free-text asset search (tag / name / serial) for when there's nothing to scan.
class SearchScreen extends StatefulWidget {
  final String? typeKey;
  final String title;

  const SearchScreen({super.key, this.typeKey, this.title = 'Find asset'});

  @override
  State<SearchScreen> createState() => _SearchScreenState();
}

class _SearchScreenState extends State<SearchScreen> {
  late final Repository _repo;
  final _controller = TextEditingController();
  List<Asset> _results = [];
  bool _loading = false;
  bool _searched = false;
  String? _error;

  @override
  void initState() {
    super.initState();
    _repo = Repository(context.read<AppController>().dio);
  }

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  Future<void> _search() async {
    final q = _controller.text.trim();
    if (q.isEmpty) return;
    FocusScope.of(context).unfocus();
    setState(() {
      _loading = true;
      _error = null;
      _searched = true;
    });
    try {
      final r = await _repo.searchAssets(q, type: widget.typeKey);
      setState(() {
        _results = r;
        _loading = false;
      });
    } on ApiException catch (e) {
      setState(() {
        _error = e.message;
        _loading = false;
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: Text(widget.title)),
      body: Column(
        children: [
          Padding(
            padding: const EdgeInsets.all(16),
            child: TextField(
              controller: _controller,
              autofocus: true,
              textInputAction: TextInputAction.search,
              onSubmitted: (_) => _search(),
              decoration: InputDecoration(
                prefixIcon: const Icon(Icons.search),
                hintText: 'Tag, name or serial',
                suffixIcon: IconButton(icon: const Icon(Icons.arrow_forward), onPressed: _search),
                border: const OutlineInputBorder(),
                isDense: true,
              ),
            ),
          ),
          Expanded(
            child: _loading
                ? const Center(child: CircularProgressIndicator())
                : _error != null
                    ? Center(child: Text(_error!, style: const TextStyle(color: Color(0xFFef4444))))
                    : !_searched
                        ? const Center(child: Text('Type a tag, name or serial to search.', style: TextStyle(color: Colors.white54)))
                        : _results.isEmpty
                            ? const Center(child: Text('No matches.', style: TextStyle(color: Colors.white54)))
                            : ListView.separated(
                                itemCount: _results.length,
                                separatorBuilder: (_, __) => const Divider(height: 1),
                                itemBuilder: (_, i) => assetTile(context, _results[i]),
                              ),
          ),
        ],
      ),
    );
  }
}
