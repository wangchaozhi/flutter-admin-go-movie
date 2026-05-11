import 'package:flutter/material.dart';

import '../../core/api_client.dart';
import '../../core/session.dart';
import '../../models/video.dart';

class MobileHomePage extends StatefulWidget {
  const MobileHomePage({super.key});

  @override
  State<MobileHomePage> createState() => _MobileHomePageState();
}

class _MobileHomePageState extends State<MobileHomePage> {
  int _selectedCategoryIndex = 0; // 0 = 全部
  int _selectedNav = 0;

  List<Category> _categories = [];
  List<Video> _videos = [];
  bool _loadingCategories = true;
  bool _loadingVideos = false;
  String? _username;

  @override
  void initState() {
    super.initState();
    _loadAll();
  }

  Future<void> _loadAll() async {
    final username = await Session.username();
    if (mounted) setState(() => _username = username);
    await Future.wait([_loadCategories(), _loadVideos(categoryId: 0)]);
  }

  Future<void> _loadCategories() async {
    try {
      final resp = await ApiClient().get('/api/categories');
      if (!mounted) return;
      if (resp['code'] == 0) {
        final list = (resp['data'] as List<dynamic>? ?? [])
            .map((e) => Category.fromJson(e as Map<String, dynamic>))
            .toList();
        setState(() { _categories = list; _loadingCategories = false; });
      }
    } catch (_) {
      if (mounted) setState(() => _loadingCategories = false);
    }
  }

  Future<void> _loadVideos({required int categoryId}) async {
    if (!mounted) return;
    setState(() => _loadingVideos = true);
    try {
      final path = categoryId > 0
          ? '/api/videos?category_id=$categoryId&per_page=50'
          : '/api/videos?per_page=50';
      final resp = await ApiClient().get(path);
      if (!mounted) return;
      if (resp['code'] == 0) {
        final data = resp['data'] as Map<String, dynamic>?;
        final list = (data?['items'] as List<dynamic>? ?? [])
            .map((e) => Video.fromJson(e as Map<String, dynamic>))
            .toList();
        setState(() => _videos = list);
      }
    } catch (_) {
      // ignore network errors — keep existing list
    } finally {
      if (mounted) setState(() => _loadingVideos = false);
    }
  }

  Future<void> _logout() async {
    await Session.clear();
    if (!mounted) return;
    Navigator.pushReplacementNamed(context, '/login');
  }

  void _onCategorySelected(int index) {
    setState(() => _selectedCategoryIndex = index);
    final categoryId = index == 0 ? 0 : _categories[index - 1].id;
    _loadVideos(categoryId: categoryId);
  }

  void _openVideo(Video video) {
    Navigator.pushNamed(context, '/player', arguments: video);
  }

  @override
  Widget build(BuildContext context) {
    final tabs = ['全部', ..._categories.map((c) => c.name)];
    final featuredVideo = _videos.where((v) => v.isReady).firstOrNull;

    return Scaffold(
      backgroundColor: const Color(0xFF0D0F14),
      body: SafeArea(
        bottom: false,
        child: CustomScrollView(
          slivers: [
            SliverToBoxAdapter(
              child: _TopBar(username: _username ?? '', onLogout: _logout),
            ),
            SliverToBoxAdapter(
              child: _loadingCategories
                  ? const SizedBox(
                      height: 48,
                      child: Center(child: SizedBox(width: 20, height: 20, child: CircularProgressIndicator(strokeWidth: 2, color: Color(0xFF25D0AB)))),
                    )
                  : _ChannelTabs(
                      tabs: tabs,
                      selectedIndex: _selectedCategoryIndex,
                      onSelected: _onCategorySelected,
                    ),
            ),
            if (featuredVideo != null)
              SliverToBoxAdapter(
                child: _FeaturedBanner(video: featuredVideo, onPlay: _openVideo),
              ),
            if (_loadingVideos)
              const SliverToBoxAdapter(
                child: Padding(
                  padding: EdgeInsets.all(32),
                  child: Center(child: CircularProgressIndicator(color: Color(0xFF25D0AB))),
                ),
              )
            else if (_videos.isEmpty)
              const SliverToBoxAdapter(
                child: Padding(
                  padding: EdgeInsets.all(48),
                  child: Center(
                    child: Text('暂无视频', style: TextStyle(color: Color(0xFF9CA3AF))),
                  ),
                ),
              )
            else
              SliverPadding(
                padding: const EdgeInsets.fromLTRB(16, 18, 16, 112),
                sliver: SliverList.separated(
                  itemCount: _videos.length,
                  separatorBuilder: (_, _) => const SizedBox(height: 14),
                  itemBuilder: (context, index) =>
                      _VideoTile(video: _videos[index], onTap: _openVideo),
                ),
              ),
          ],
        ),
      ),
      bottomNavigationBar: _BottomNav(
        selectedIndex: _selectedNav,
        onSelected: (index) => setState(() => _selectedNav = index),
      ),
    );
  }
}

class _TopBar extends StatelessWidget {
  const _TopBar({required this.username, required this.onLogout});

  final String username;
  final VoidCallback onLogout;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.fromLTRB(16, 12, 10, 6),
      child: Row(
        children: [
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  'Go Movie',
                  style: Theme.of(context).textTheme.headlineSmall?.copyWith(
                    color: Colors.white,
                    fontWeight: FontWeight.w900,
                  ),
                ),
                const SizedBox(height: 2),
                Text(
                  username.isNotEmpty ? '你好，$username' : '今晚想看点什么？',
                  style: const TextStyle(color: Color(0xFF9CA3AF), fontSize: 13),
                ),
              ],
            ),
          ),
          IconButton(
            onPressed: () {},
            icon: const Icon(Icons.search_rounded),
            color: Colors.white,
            tooltip: '搜索',
          ),
          IconButton(
            onPressed: onLogout,
            icon: const Icon(Icons.logout_rounded),
            color: Colors.white,
            tooltip: '退出登录',
          ),
        ],
      ),
    );
  }
}

class _ChannelTabs extends StatelessWidget {
  const _ChannelTabs({
    required this.tabs,
    required this.selectedIndex,
    required this.onSelected,
  });

  final List<String> tabs;
  final int selectedIndex;
  final ValueChanged<int> onSelected;

  @override
  Widget build(BuildContext context) {
    return SizedBox(
      height: 48,
      child: ListView.separated(
        padding: const EdgeInsets.symmetric(horizontal: 16),
        scrollDirection: Axis.horizontal,
        itemBuilder: (context, index) {
          final selected = selectedIndex == index;
          return ChoiceChip(
            selected: selected,
            showCheckmark: false,
            label: Text(tabs[index]),
            labelStyle: TextStyle(
              color: selected ? const Color(0xFF101318) : const Color(0xFFD1D5DB),
              fontWeight: selected ? FontWeight.w800 : FontWeight.w600,
            ),
            selectedColor: const Color(0xFFF7C948),
            backgroundColor: const Color(0xFF1B1F2A),
            side: BorderSide(
              color: selected ? const Color(0xFFF7C948) : const Color(0xFF2B3140),
            ),
            shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(8)),
            onSelected: (_) => onSelected(index),
          );
        },
        separatorBuilder: (_, _) => const SizedBox(width: 10),
        itemCount: tabs.length,
      ),
    );
  }
}

class _FeaturedBanner extends StatelessWidget {
  const _FeaturedBanner({required this.video, required this.onPlay});

  final Video video;
  final ValueChanged<Video> onPlay;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.fromLTRB(16, 12, 16, 0),
      child: ClipRRect(
        borderRadius: BorderRadius.circular(8),
        child: AspectRatio(
          aspectRatio: 16 / 10,
          child: Stack(
            fit: StackFit.expand,
            children: [
              video.fullCoverUrl.isNotEmpty
                  ? Image.network(
                      video.fullCoverUrl,
                      fit: BoxFit.cover,
                      errorBuilder: (_, _, _) =>
                          const ColoredBox(color: Color(0xFF202532)),
                    )
                  : const ColoredBox(color: Color(0xFF202532)),
              const DecoratedBox(
                decoration: BoxDecoration(
                  gradient: LinearGradient(
                    colors: [Color(0x99000000), Color(0x00000000), Color(0xDD000000)],
                    begin: Alignment.topCenter,
                    end: Alignment.bottomCenter,
                  ),
                ),
              ),
              Positioned(
                left: 16,
                right: 16,
                bottom: 16,
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    if (video.isVip && !video.isFree)
                      const _MetaPill(text: 'VIP 专属'),
                    const SizedBox(height: 10),
                    Text(
                      video.title,
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: Theme.of(context).textTheme.headlineSmall?.copyWith(
                        color: Colors.white,
                        fontWeight: FontWeight.w900,
                      ),
                    ),
                    if (video.description.isNotEmpty) ...[
                      const SizedBox(height: 4),
                      Text(
                        video.description,
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                        style: const TextStyle(color: Color(0xFFE5E7EB), fontSize: 13),
                      ),
                    ],
                    const SizedBox(height: 14),
                    FilledButton.icon(
                      onPressed: () => onPlay(video),
                      icon: const Icon(Icons.play_arrow_rounded),
                      label: const Text('播放'),
                      style: FilledButton.styleFrom(
                        backgroundColor: const Color(0xFFF7C948),
                        foregroundColor: const Color(0xFF101318),
                        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(8)),
                      ),
                    ),
                  ],
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _VideoTile extends StatelessWidget {
  const _VideoTile({required this.video, required this.onTap});

  final Video video;
  final ValueChanged<Video> onTap;

  @override
  Widget build(BuildContext context) {
    return Material(
      color: const Color(0xFF171B24),
      borderRadius: BorderRadius.circular(8),
      child: InkWell(
        borderRadius: BorderRadius.circular(8),
        onTap: video.isReady ? () => onTap(video) : null,
        child: Padding(
          padding: const EdgeInsets.all(10),
          child: Row(
            children: [
              ClipRRect(
                borderRadius: BorderRadius.circular(8),
                child: SizedBox(
                  width: 116,
                  height: 76,
                  child: Stack(
                    fit: StackFit.expand,
                    children: [
                      video.fullCoverUrl.isNotEmpty
                          ? Image.network(
                              video.fullCoverUrl,
                              fit: BoxFit.cover,
                              errorBuilder: (_, _, _) =>
                                  const ColoredBox(color: Color(0xFF202532)),
                            )
                          : const ColoredBox(color: Color(0xFF202532)),
                      if (video.durationLabel.isNotEmpty)
                        Align(
                          alignment: Alignment.bottomRight,
                          child: Container(
                            margin: const EdgeInsets.all(6),
                            padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 3),
                            decoration: BoxDecoration(
                              color: const Color(0xCC000000),
                              borderRadius: BorderRadius.circular(6),
                            ),
                            child: Text(
                              video.durationLabel,
                              style: const TextStyle(color: Colors.white, fontSize: 11),
                            ),
                          ),
                        ),
                      if (!video.isReady)
                        Container(
                          color: const Color(0x88000000),
                          child: Center(
                            child: Text(
                              _statusLabel(video.status),
                              style: const TextStyle(color: Colors.white70, fontSize: 11),
                            ),
                          ),
                        ),
                    ],
                  ),
                ),
              ),
              const SizedBox(width: 12),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Row(
                      children: [
                        Expanded(
                          child: Text(
                            video.title,
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                            style: const TextStyle(
                              color: Colors.white,
                              fontWeight: FontWeight.w800,
                              fontSize: 16,
                            ),
                          ),
                        ),
                        if (video.isVip && !video.isFree)
                          Container(
                            margin: const EdgeInsets.only(left: 6),
                            padding: const EdgeInsets.symmetric(horizontal: 5, vertical: 2),
                            decoration: BoxDecoration(
                              color: const Color(0xFFF7C948),
                              borderRadius: BorderRadius.circular(4),
                            ),
                            child: const Text(
                              'VIP',
                              style: TextStyle(color: Color(0xFF101318), fontSize: 10, fontWeight: FontWeight.w900),
                            ),
                          ),
                      ],
                    ),
                    const SizedBox(height: 6),
                    if (video.description.isNotEmpty)
                      Text(
                        video.description,
                        maxLines: 2,
                        overflow: TextOverflow.ellipsis,
                        style: const TextStyle(color: Color(0xFF9CA3AF), height: 1.35),
                      ),
                    if (video.categoryName.isNotEmpty) ...[
                      const SizedBox(height: 6),
                      Text(
                        video.categoryName,
                        style: const TextStyle(color: Color(0xFF25D0AB), fontSize: 12),
                      ),
                    ],
                  ],
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }

  String _statusLabel(String status) {
    return switch (status) {
      'uploading' => '上传中',
      'uploaded' => '待转码',
      'transcoding' => '转码中',
      'failed' => '转码失败',
      'offline' => '已下架',
      _ => status,
    };
  }
}

class _BottomNav extends StatelessWidget {
  const _BottomNav({required this.selectedIndex, required this.onSelected});

  final int selectedIndex;
  final ValueChanged<int> onSelected;

  @override
  Widget build(BuildContext context) {
    const items = [
      (Icons.home_rounded, '首页'),
      (Icons.explore_rounded, '发现'),
      (Icons.bookmark_rounded, '片单'),
      (Icons.person_rounded, '我的'),
    ];

    return NavigationBar(
      selectedIndex: selectedIndex,
      onDestinationSelected: onSelected,
      backgroundColor: const Color(0xFF101318),
      indicatorColor: const Color(0x3325D0AB),
      labelTextStyle: WidgetStateProperty.resolveWith(
        (states) => TextStyle(
          color: states.contains(WidgetState.selected) ? Colors.white : const Color(0xFF9CA3AF),
          fontSize: 12,
          fontWeight: states.contains(WidgetState.selected) ? FontWeight.w800 : FontWeight.w600,
        ),
      ),
      destinations: [
        for (final item in items)
          NavigationDestination(
            icon: Icon(item.$1, color: const Color(0xFF9CA3AF)),
            selectedIcon: Icon(item.$1, color: const Color(0xFF25D0AB)),
            label: item.$2,
          ),
      ],
    );
  }
}

class _MetaPill extends StatelessWidget {
  const _MetaPill({required this.text});

  final String text;

  @override
  Widget build(BuildContext context) {
    return DecoratedBox(
      decoration: BoxDecoration(
        color: const Color(0xCCF7C948),
        borderRadius: BorderRadius.circular(6),
      ),
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
        child: Text(
          text,
          style: const TextStyle(
            color: Color(0xFF101318),
            fontSize: 11,
            fontWeight: FontWeight.w900,
          ),
        ),
      ),
    );
  }
}
