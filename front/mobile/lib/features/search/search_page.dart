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
            hintText: '搜索片名、导演或关键词',
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
      return _SearchEmptyState(
        icon: Icons.wifi_off_rounded,
        title: '搜索暂时不可用',
        subtitle: _error,
      );
    }
    if (!_searched) {
      return _buildHistory();
    }
    if (_results.isEmpty) {
      return const _SearchEmptyState(
        icon: Icons.search_off_rounded,
        title: '没有找到相关影片',
        subtitle: '换个片名、演员或分类关键词再试试',
      );
    }
    return ListView.separated(
      padding: const EdgeInsets.fromLTRB(16, 16, 16, 32),
      itemCount: _results.length + 1,
      separatorBuilder: (_, index) =>
          index == 0 ? const SizedBox(height: 12) : const SizedBox(height: 14),
      itemBuilder: (context, index) {
        if (index == 0) {
          return Text(
            '找到 ${_results.length} 部相关影片',
            style: const TextStyle(
              color: Colors.white,
              fontWeight: FontWeight.w800,
            ),
          );
        }
        return VideoTile(video: _results[index - 1], onTap: _openVideo);
      },
    );
  }

  Widget _buildHistory() {
    if (_history.isEmpty) {
      return const _SearchEmptyState(
        icon: Icons.manage_search_rounded,
        title: '想看什么？',
        subtitle: '输入片名、导演、演员或分类关键词开始搜索',
      );
    }
    return ListView(
      padding: const EdgeInsets.fromLTRB(16, 18, 16, 16),
      children: [
        Row(
          mainAxisAlignment: MainAxisAlignment.spaceBetween,
          children: [
            const Text(
              '最近搜索',
              style: TextStyle(
                color: Colors.white,
                fontWeight: FontWeight.w800,
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
                  side: const BorderSide(color: Color(0xFF2B3140)),
                  onPressed: () => _runSearch(term),
                ),
              )
              .toList(),
        ),
      ],
    );
  }
}

class _SearchEmptyState extends StatelessWidget {
  const _SearchEmptyState({
    required this.icon,
    required this.title,
    required this.subtitle,
  });

  final IconData icon;
  final String title;
  final String subtitle;

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(28),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(icon, color: _accent, size: 36),
            const SizedBox(height: 14),
            Text(
              title,
              textAlign: TextAlign.center,
              style: const TextStyle(
                color: Colors.white,
                fontSize: 17,
                fontWeight: FontWeight.w900,
              ),
            ),
            const SizedBox(height: 8),
            Text(
              subtitle,
              textAlign: TextAlign.center,
              style: const TextStyle(color: _muted, height: 1.45),
            ),
          ],
        ),
      ),
    );
  }
}
