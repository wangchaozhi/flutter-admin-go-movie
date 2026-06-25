import 'package:flutter/material.dart';

import '../../core/api_client.dart';
import '../../models/video.dart';
import '../home/widgets/catalog_widgets.dart';
import 'search_storage.dart';

const _accent = Color(0xFF25D0AB);
const _bg = Color(0xFF0D0F14);
const _muted = Color(0xFF9CA3AF);

class SearchPage extends StatefulWidget {
  const SearchPage({super.key});

  @override
  State<SearchPage> createState() => _SearchPageState();
}

class _SearchPageState extends State<SearchPage> {
  final _controller = TextEditingController();
  final _storage = SearchHistoryStorage();

  List<String> _history = [];
  List<Video> _results = [];
  bool _loading = false;
  bool _searched = false;
  String _error = '';

  @override
  void initState() {
    super.initState();
    _loadHistory();
  }

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  Future<void> _loadHistory() async {
    final list = await _storage.load();
    if (mounted) setState(() => _history = list);
  }

  Future<void> _runSearch(String term) async {
    final query = term.trim();
    if (query.isEmpty) return;
    _controller.value = TextEditingValue(
      text: query,
      selection: TextSelection.collapsed(offset: query.length),
    );
    FocusScope.of(context).unfocus();
    setState(() {
      _loading = true;
      _searched = true;
      _error = '';
    });
    try {
      final resp = await ApiClient().get(
        '/api/videos?per_page=50&q=${Uri.encodeQueryComponent(query)}',
      );
      if (!mounted) return;
      if (resp['code'] == 0) {
        final data = resp['data'] as Map<String, dynamic>?;
        final list = (data?['items'] as List<dynamic>? ?? [])
            .map((e) => Video.fromJson(e as Map<String, dynamic>))
            .toList();
        setState(() => _results = list);
        final hist = await _storage.add(query);
        if (mounted) setState(() => _history = hist);
      } else {
        setState(() => _error = resp['msg']?.toString() ?? '搜索失败');
      }
    } catch (_) {
      if (mounted) setState(() => _error = '搜索失败，请稍后再试');
    } finally {
      if (mounted) setState(() => _loading = false);
    }
  }

  void _openVideo(Video video) {
    Navigator.pushNamed(context, '/player', arguments: video);
  }

  Future<void> _clearHistory() async {
    await _storage.clear();
    if (mounted) setState(() => _history = []);
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: _bg,
      appBar: AppBar(
        backgroundColor: _bg,
        foregroundColor: Colors.white,
        elevation: 0,
        titleSpacing: 0,
        title: TextField(
          controller: _controller,
          autofocus: true,
          textInputAction: TextInputAction.search,
          style: const TextStyle(color: Colors.white),
          cursorColor: _accent,
          decoration: const InputDecoration(
            hintText: '搜索影片标题',
            hintStyle: TextStyle(color: _muted),
            border: InputBorder.none,
          ),
          onSubmitted: _runSearch,
        ),
        actions: [
          IconButton(
            icon: const Icon(Icons.search, color: _accent),
            onPressed: () => _runSearch(_controller.text),
          ),
        ],
      ),
      body: SafeArea(top: false, child: _buildBody()),
    );
  }

  Widget _buildBody() {
    if (_loading) {
      return const Center(child: CircularProgressIndicator(color: _accent));
    }
    if (_error.isNotEmpty) {
      return Center(child: Text(_error, style: const TextStyle(color: _muted)));
    }
    if (!_searched) {
      return _buildHistory();
    }
    if (_results.isEmpty) {
      return const Center(
        child: Text('未找到相关视频', style: TextStyle(color: _muted)),
      );
    }
    return ListView.separated(
      padding: const EdgeInsets.fromLTRB(16, 16, 16, 32),
      itemCount: _results.length,
      separatorBuilder: (_, _) => const SizedBox(height: 14),
      itemBuilder: (context, index) =>
          VideoTile(video: _results[index], onTap: _openVideo),
    );
  }

  Widget _buildHistory() {
    if (_history.isEmpty) {
      return const Center(
        child: Text('输入关键字开始搜索', style: TextStyle(color: _muted)),
      );
    }
    return Padding(
      padding: const EdgeInsets.fromLTRB(16, 18, 16, 16),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: [
              const Text(
                '搜索历史',
                style: TextStyle(
                  color: Colors.white,
                  fontWeight: FontWeight.w700,
                ),
              ),
              TextButton(
                onPressed: _clearHistory,
                child: const Text('清空', style: TextStyle(color: _muted)),
              ),
            ],
          ),
          const SizedBox(height: 8),
          Wrap(
            spacing: 8,
            runSpacing: 8,
            children: _history
                .map(
                  (term) => ActionChip(
                    label: Text(term),
                    backgroundColor: const Color(0xFF171B24),
                    labelStyle: const TextStyle(color: Colors.white),
                    side: BorderSide.none,
                    onPressed: () => _runSearch(term),
                  ),
                )
                .toList(),
          ),
        ],
      ),
    );
  }
}
