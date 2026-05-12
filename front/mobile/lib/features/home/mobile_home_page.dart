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
  List<_OrderSummary> _orders = [];
  bool _loadingCategories = true;
  bool _loadingVideos = false;
  bool _loadingProfile = false;
  bool _loadingOrders = false;
  bool _profileLoaded = false;
  String? _username;
  _MobileProfile? _profile;
  String _profileError = '';

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
        setState(() {
          _categories = list;
          _loadingCategories = false;
        });
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

  Future<void> _loadProfileCenter({bool force = false}) async {
    if (_profileLoaded && !force) return;
    if (!mounted) return;
    setState(() {
      _loadingProfile = true;
      _loadingOrders = true;
      _profileError = '';
    });
    await Future.wait([_loadProfile(), _loadOrders()]);
    if (mounted) {
      setState(() => _profileLoaded = true);
    }
  }

  Future<void> _loadProfile() async {
    try {
      final resp = await ApiClient().getAuth('/api/mobile/profile');
      if (!mounted) return;
      if (resp['code'] == 0) {
        final data = resp['data'] as Map<String, dynamic>? ?? {};
        setState(() {
          _profile = _MobileProfile.fromJson(data);
          _loadingProfile = false;
        });
      } else {
        setState(() {
          _profileError = resp['msg']?.toString() ?? '资料加载失败';
          _loadingProfile = false;
        });
      }
    } catch (_) {
      if (mounted) {
        setState(() {
          _profileError = '资料加载失败';
          _loadingProfile = false;
        });
      }
    }
  }

  Future<void> _loadOrders() async {
    try {
      final resp = await ApiClient().getAuth('/api/orders?limit=3');
      if (!mounted) return;
      if (resp['code'] == 0) {
        final list = (resp['data'] as List<dynamic>? ?? [])
            .map((item) => _OrderSummary.fromJson(item as Map<String, dynamic>))
            .toList();
        setState(() {
          _orders = list;
          _loadingOrders = false;
        });
      } else {
        setState(() => _loadingOrders = false);
      }
    } catch (_) {
      if (mounted) setState(() => _loadingOrders = false);
    }
  }

  void _onNavSelected(int index) {
    setState(() => _selectedNav = index);
    if (index == 3) {
      _loadProfileCenter();
    }
  }

  @override
  Widget build(BuildContext context) {
    final tabs = ['全部', ..._categories.map((c) => c.name)];
    final featuredVideo = _videos.where((v) => v.isReady).firstOrNull;

    final content = _selectedNav == 3
        ? _ProfileCenter(
            username: _username ?? '',
            profile: _profile,
            orders: _orders,
            loadingProfile: _loadingProfile,
            loadingOrders: _loadingOrders,
            error: _profileError,
            videos: _videos,
            onOpenVip: () => Navigator.pushNamed(context, '/vip'),
            onRefresh: () => _loadProfileCenter(force: true),
            onLogout: _logout,
          )
        : CustomScrollView(
            slivers: [
              SliverToBoxAdapter(
                child: _TopBar(username: _username ?? '', onLogout: _logout),
              ),
              SliverToBoxAdapter(
                child: _loadingCategories
                    ? const SizedBox(
                        height: 48,
                        child: Center(
                          child: SizedBox(
                            width: 20,
                            height: 20,
                            child: CircularProgressIndicator(
                              strokeWidth: 2,
                              color: Color(0xFF25D0AB),
                            ),
                          ),
                        ),
                      )
                    : _ChannelTabs(
                        tabs: tabs,
                        selectedIndex: _selectedCategoryIndex,
                        onSelected: _onCategorySelected,
                      ),
              ),
              if (featuredVideo != null)
                SliverToBoxAdapter(
                  child: _FeaturedBanner(
                    video: featuredVideo,
                    onPlay: _openVideo,
                  ),
                ),
              if (_loadingVideos)
                const SliverToBoxAdapter(
                  child: Padding(
                    padding: EdgeInsets.all(32),
                    child: Center(
                      child: CircularProgressIndicator(
                        color: Color(0xFF25D0AB),
                      ),
                    ),
                  ),
                )
              else if (_videos.isEmpty)
                const SliverToBoxAdapter(
                  child: Padding(
                    padding: EdgeInsets.all(48),
                    child: Center(
                      child: Text(
                        '暂无视频',
                        style: TextStyle(color: Color(0xFF9CA3AF)),
                      ),
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
          );

    return Scaffold(
      backgroundColor: const Color(0xFF0D0F14),
      body: SafeArea(bottom: false, child: content),
      bottomNavigationBar: _BottomNav(
        selectedIndex: _selectedNav,
        onSelected: _onNavSelected,
      ),
    );
  }
}

class _TopBar extends StatelessWidget {
  const _TopBar({required this.username, required this.onLogout});

  final String username;
  final VoidCallback onLogout;

  void _openProfile(BuildContext context) {
    showModalBottomSheet<void>(
      context: context,
      backgroundColor: const Color(0xFF171B24),
      showDragHandle: true,
      builder: (_) => _ProfileSheet(username: username),
    );
  }

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
                  style: const TextStyle(
                    color: Color(0xFF9CA3AF),
                    fontSize: 13,
                  ),
                ),
              ],
            ),
          ),
          PopupMenuButton<_UserMenuAction>(
            tooltip: '我的',
            color: const Color(0xFF171B24),
            position: PopupMenuPosition.under,
            offset: const Offset(0, 8),
            onSelected: (action) {
              switch (action) {
                case _UserMenuAction.profile:
                  _openProfile(context);
                case _UserMenuAction.vip:
                  Navigator.pushNamed(context, '/vip');
                case _UserMenuAction.logout:
                  onLogout();
              }
            },
            itemBuilder: (context) => const [
              PopupMenuItem(
                value: _UserMenuAction.profile,
                child: _UserMenuItem(icon: Icons.badge_outlined, label: '资料'),
              ),
              PopupMenuItem(
                value: _UserMenuAction.vip,
                child: _UserMenuItem(
                  icon: Icons.workspace_premium_rounded,
                  label: 'VIP',
                  vip: true,
                ),
              ),
              PopupMenuDivider(),
              PopupMenuItem(
                value: _UserMenuAction.logout,
                child: _UserMenuItem(
                  icon: Icons.logout_rounded,
                  label: '退出登录',
                  danger: true,
                ),
              ),
            ],
            child: _AvatarButton(username: username),
          ),
        ],
      ),
    );
  }
}

enum _UserMenuAction { profile, vip, logout }

class _AvatarButton extends StatelessWidget {
  const _AvatarButton({required this.username});

  final String username;

  @override
  Widget build(BuildContext context) {
    final letter = username.trim().isEmpty
        ? '我'
        : username.trim().characters.first.toUpperCase();

    return Container(
      width: 44,
      height: 44,
      margin: const EdgeInsets.only(left: 8),
      decoration: BoxDecoration(
        shape: BoxShape.circle,
        gradient: const LinearGradient(
          colors: [Color(0xFFF7C948), Color(0xFF25D0AB)],
          begin: Alignment.topLeft,
          end: Alignment.bottomRight,
        ),
        border: Border.all(color: const Color(0x66FFFFFF), width: 1.5),
        boxShadow: const [
          BoxShadow(
            color: Color(0x3325D0AB),
            blurRadius: 16,
            offset: Offset(0, 8),
          ),
        ],
      ),
      alignment: Alignment.center,
      child: Text(
        letter,
        style: const TextStyle(
          color: Color(0xFF101318),
          fontSize: 18,
          fontWeight: FontWeight.w900,
        ),
      ),
    );
  }
}

class _UserMenuItem extends StatelessWidget {
  const _UserMenuItem({
    required this.icon,
    required this.label,
    this.vip = false,
    this.danger = false,
  });

  final IconData icon;
  final String label;
  final bool vip;
  final bool danger;

  @override
  Widget build(BuildContext context) {
    final color = danger ? const Color(0xFFF87171) : Colors.white;

    return Row(
      children: [
        if (vip) const _VipBadgeIcon() else Icon(icon, color: color, size: 21),
        const SizedBox(width: 12),
        Text(
          label,
          style: TextStyle(color: color, fontWeight: FontWeight.w800),
        ),
      ],
    );
  }
}

class _VipBadgeIcon extends StatelessWidget {
  const _VipBadgeIcon();

  @override
  Widget build(BuildContext context) {
    return Container(
      width: 28,
      height: 28,
      decoration: BoxDecoration(
        gradient: const LinearGradient(
          colors: [Color(0xFFFFE082), Color(0xFFF7C948), Color(0xFFEAB308)],
          begin: Alignment.topLeft,
          end: Alignment.bottomRight,
        ),
        borderRadius: BorderRadius.circular(8),
        boxShadow: const [
          BoxShadow(
            color: Color(0x44F7C948),
            blurRadius: 10,
            offset: Offset(0, 4),
          ),
        ],
      ),
      child: const Icon(
        Icons.workspace_premium_rounded,
        color: Color(0xFF101318),
        size: 19,
      ),
    );
  }
}

class _ProfileSheet extends StatelessWidget {
  const _ProfileSheet({required this.username});

  final String username;

  @override
  Widget build(BuildContext context) {
    final displayName = username.isEmpty ? '移动端用户' : username;

    return SafeArea(
      top: false,
      child: Padding(
        padding: const EdgeInsets.fromLTRB(20, 8, 20, 24),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                _AvatarButton(username: username),
                const SizedBox(width: 14),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        displayName,
                        style: const TextStyle(
                          color: Colors.white,
                          fontSize: 20,
                          fontWeight: FontWeight.w900,
                        ),
                      ),
                      const SizedBox(height: 3),
                      const Text(
                        'Go Movie 会员账号',
                        style: TextStyle(color: Color(0xFF9CA3AF)),
                      ),
                    ],
                  ),
                ),
                const _VipBadgeIcon(),
              ],
            ),
            const SizedBox(height: 20),
            _ProfileInfoRow(
              icon: Icons.person_outline_rounded,
              label: '用户名',
              value: displayName,
            ),
            const _ProfileInfoRow(
              icon: Icons.shield_outlined,
              label: '账号状态',
              value: '正常',
            ),
          ],
        ),
      ),
    );
  }
}

class _ProfileInfoRow extends StatelessWidget {
  const _ProfileInfoRow({
    required this.icon,
    required this.label,
    required this.value,
  });

  final IconData icon;
  final String label;
  final String value;

  @override
  Widget build(BuildContext context) {
    return Container(
      margin: const EdgeInsets.only(bottom: 10),
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 12),
      decoration: BoxDecoration(
        color: const Color(0xFF101318),
        borderRadius: BorderRadius.circular(8),
        border: Border.all(color: const Color(0xFF2B3140)),
      ),
      child: Row(
        children: [
          Icon(icon, color: const Color(0xFF25D0AB), size: 20),
          const SizedBox(width: 10),
          Text(
            label,
            style: const TextStyle(
              color: Color(0xFF9CA3AF),
              fontWeight: FontWeight.w700,
            ),
          ),
          const Spacer(),
          Text(
            value,
            style: const TextStyle(
              color: Colors.white,
              fontWeight: FontWeight.w800,
            ),
          ),
        ],
      ),
    );
  }
}

class _ProfileCenter extends StatelessWidget {
  const _ProfileCenter({
    required this.username,
    required this.profile,
    required this.orders,
    required this.loadingProfile,
    required this.loadingOrders,
    required this.error,
    required this.videos,
    required this.onOpenVip,
    required this.onRefresh,
    required this.onLogout,
  });

  final String username;
  final _MobileProfile? profile;
  final List<_OrderSummary> orders;
  final bool loadingProfile;
  final bool loadingOrders;
  final String error;
  final List<Video> videos;
  final VoidCallback onOpenVip;
  final Future<void> Function() onRefresh;
  final VoidCallback onLogout;

  @override
  Widget build(BuildContext context) {
    final displayName =
        profile?.displayName ?? (username.isEmpty ? '移动端用户' : username);
    final isVip = profile?.isVip ?? false;
    final readyCount = videos.where((video) => video.isReady).length;
    final vipCount = videos
        .where((video) => video.isVip && !video.isFree)
        .length;

    return RefreshIndicator(
      color: const Color(0xFF25D0AB),
      onRefresh: onRefresh,
      child: ListView(
        padding: const EdgeInsets.fromLTRB(16, 12, 16, 112),
        children: [
          Row(
            children: [
              Expanded(
                child: Text(
                  '我的',
                  style: Theme.of(context).textTheme.headlineSmall?.copyWith(
                    color: Colors.white,
                    fontWeight: FontWeight.w900,
                  ),
                ),
              ),
              IconButton(
                onPressed: onRefresh,
                icon: const Icon(Icons.refresh_rounded),
                color: Colors.white,
                tooltip: '刷新',
              ),
            ],
          ),
          const SizedBox(height: 12),
          _ProfileHeaderCard(
            username: displayName,
            loading: loadingProfile,
            isVip: isVip,
            vipUntilLabel: profile?.vipUntilLabel ?? '',
            error: error,
            onOpenVip: onOpenVip,
          ),
          const SizedBox(height: 14),
          _VipMemberCard(
            isVip: isVip,
            vipUntilLabel: profile?.vipUntilLabel ?? '',
            onTap: onOpenVip,
          ),
          const SizedBox(height: 14),
          _ShortcutGrid(
            items: [
              _ShortcutItem(Icons.history_rounded, '观看记录', '$readyCount 部可观看'),
              _ShortcutItem(Icons.bookmark_rounded, '我的收藏', '稍后接入'),
              _ShortcutItem(
                Icons.receipt_long_rounded,
                '订单记录',
                '${orders.length} 条最近订单',
              ),
              _ShortcutItem(Icons.settings_rounded, '设置', '偏好与缓存'),
            ],
          ),
          const SizedBox(height: 14),
          _WatchSummaryCard(readyCount: readyCount, vipCount: vipCount),
          const SizedBox(height: 14),
          _RecentOrdersCard(orders: orders, loading: loadingOrders),
          const SizedBox(height: 14),
          _SettingsCard(onLogout: onLogout),
        ],
      ),
    );
  }
}

class _ProfileHeaderCard extends StatelessWidget {
  const _ProfileHeaderCard({
    required this.username,
    required this.loading,
    required this.isVip,
    required this.vipUntilLabel,
    required this.error,
    required this.onOpenVip,
  });

  final String username;
  final bool loading;
  final bool isVip;
  final String vipUntilLabel;
  final String error;
  final VoidCallback onOpenVip;

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: const Color(0xFF171B24),
        borderRadius: BorderRadius.circular(8),
        border: Border.all(color: const Color(0xFF2B3140)),
      ),
      child: Row(
        children: [
          _AvatarButton(username: username),
          const SizedBox(width: 14),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  username,
                  style: const TextStyle(
                    color: Colors.white,
                    fontSize: 20,
                    fontWeight: FontWeight.w900,
                  ),
                ),
                const SizedBox(height: 4),
                if (loading)
                  const Text(
                    '资料加载中...',
                    style: TextStyle(color: Color(0xFF9CA3AF)),
                  )
                else if (error.isNotEmpty)
                  Text(error, style: const TextStyle(color: Color(0xFFF87171)))
                else
                  Text(
                    isVip ? 'VIP 有效期至 $vipUntilLabel' : '普通用户',
                    style: const TextStyle(color: Color(0xFF9CA3AF)),
                  ),
              ],
            ),
          ),
          if (isVip)
            const _VipBadgeIcon()
          else
            FilledButton(
              onPressed: onOpenVip,
              style: FilledButton.styleFrom(
                backgroundColor: const Color(0xFFF7C948),
                foregroundColor: const Color(0xFF101318),
                shape: RoundedRectangleBorder(
                  borderRadius: BorderRadius.circular(8),
                ),
              ),
              child: const Text('开通'),
            ),
        ],
      ),
    );
  }
}

class _VipMemberCard extends StatelessWidget {
  const _VipMemberCard({
    required this.isVip,
    required this.vipUntilLabel,
    required this.onTap,
  });

  final bool isVip;
  final String vipUntilLabel;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return Material(
      color: Colors.transparent,
      child: InkWell(
        borderRadius: BorderRadius.circular(8),
        onTap: onTap,
        child: Ink(
          padding: const EdgeInsets.all(16),
          decoration: BoxDecoration(
            gradient: const LinearGradient(
              colors: [Color(0xFF3B2F10), Color(0xFF171B24)],
              begin: Alignment.topLeft,
              end: Alignment.bottomRight,
            ),
            borderRadius: BorderRadius.circular(8),
            border: Border.all(color: const Color(0x66F7C948)),
          ),
          child: Row(
            children: [
              const _VipBadgeIcon(),
              const SizedBox(width: 14),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    const Text(
                      'VIP 会员',
                      style: TextStyle(
                        color: Colors.white,
                        fontSize: 17,
                        fontWeight: FontWeight.w900,
                      ),
                    ),
                    const SizedBox(height: 4),
                    Text(
                      isVip ? '会员有效期至 $vipUntilLabel' : '解锁会员专属影片与更多权益',
                      style: const TextStyle(
                        color: Color(0xFFE5E7EB),
                        height: 1.35,
                      ),
                    ),
                  ],
                ),
              ),
              const Icon(Icons.chevron_right_rounded, color: Color(0xFFF7C948)),
            ],
          ),
        ),
      ),
    );
  }
}

class _ShortcutGrid extends StatelessWidget {
  const _ShortcutGrid({required this.items});

  final List<_ShortcutItem> items;

  @override
  Widget build(BuildContext context) {
    return GridView.count(
      crossAxisCount: 2,
      mainAxisSpacing: 10,
      crossAxisSpacing: 10,
      childAspectRatio: 1.9,
      shrinkWrap: true,
      physics: const NeverScrollableScrollPhysics(),
      children: [for (final item in items) _ShortcutTile(item: item)],
    );
  }
}

class _ShortcutTile extends StatelessWidget {
  const _ShortcutTile({required this.item});

  final _ShortcutItem item;

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: const Color(0xFF171B24),
        borderRadius: BorderRadius.circular(8),
        border: Border.all(color: const Color(0xFF2B3140)),
      ),
      child: Row(
        children: [
          Icon(item.icon, color: const Color(0xFF25D0AB)),
          const SizedBox(width: 10),
          Expanded(
            child: Column(
              mainAxisAlignment: MainAxisAlignment.center,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  item.title,
                  style: const TextStyle(
                    color: Colors.white,
                    fontWeight: FontWeight.w900,
                  ),
                ),
                const SizedBox(height: 3),
                Text(
                  item.subtitle,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: const TextStyle(
                    color: Color(0xFF9CA3AF),
                    fontSize: 12,
                  ),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

class _WatchSummaryCard extends StatelessWidget {
  const _WatchSummaryCard({required this.readyCount, required this.vipCount});

  final int readyCount;
  final int vipCount;

  @override
  Widget build(BuildContext context) {
    return _SectionCard(
      title: '观看记录',
      trailing: '继续观看',
      child: Row(
        children: [
          Expanded(
            child: _MetricBox(label: '可观看', value: '$readyCount'),
          ),
          const SizedBox(width: 10),
          Expanded(
            child: _MetricBox(label: 'VIP 专属', value: '$vipCount'),
          ),
        ],
      ),
    );
  }
}

class _RecentOrdersCard extends StatelessWidget {
  const _RecentOrdersCard({required this.orders, required this.loading});

  final List<_OrderSummary> orders;
  final bool loading;

  @override
  Widget build(BuildContext context) {
    return _SectionCard(
      title: '订单记录',
      trailing: loading ? '加载中' : '最近订单',
      child: loading
          ? const Padding(
              padding: EdgeInsets.symmetric(vertical: 16),
              child: Center(
                child: CircularProgressIndicator(color: Color(0xFF25D0AB)),
              ),
            )
          : orders.isEmpty
          ? const Padding(
              padding: EdgeInsets.symmetric(vertical: 12),
              child: Text('暂无订单', style: TextStyle(color: Color(0xFF9CA3AF))),
            )
          : Column(
              children: [for (final order in orders) _OrderRow(order: order)],
            ),
    );
  }
}

class _OrderRow extends StatelessWidget {
  const _OrderRow({required this.order});

  final _OrderSummary order;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 10),
      child: Row(
        children: [
          const Icon(
            Icons.receipt_long_rounded,
            color: Color(0xFFF7C948),
            size: 22,
          ),
          const SizedBox(width: 10),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  order.productName,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: const TextStyle(
                    color: Colors.white,
                    fontWeight: FontWeight.w800,
                  ),
                ),
                const SizedBox(height: 2),
                Text(
                  order.orderNo,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: const TextStyle(
                    color: Color(0xFF9CA3AF),
                    fontSize: 12,
                  ),
                ),
              ],
            ),
          ),
          const SizedBox(width: 10),
          Column(
            crossAxisAlignment: CrossAxisAlignment.end,
            children: [
              Text(
                order.priceLabel,
                style: const TextStyle(
                  color: Colors.white,
                  fontWeight: FontWeight.w900,
                ),
              ),
              const SizedBox(height: 2),
              Text(
                order.statusLabel,
                style: TextStyle(
                  color: order.statusColor,
                  fontSize: 12,
                  fontWeight: FontWeight.w800,
                ),
              ),
            ],
          ),
        ],
      ),
    );
  }
}

class _SettingsCard extends StatelessWidget {
  const _SettingsCard({required this.onLogout});

  final VoidCallback onLogout;

  @override
  Widget build(BuildContext context) {
    return _SectionCard(
      title: '设置',
      trailing: '基础',
      child: Column(
        children: [
          const _SettingRow(
            icon: Icons.cleaning_services_outlined,
            label: '清理缓存',
            value: '可用',
          ),
          const _SettingRow(
            icon: Icons.info_outline_rounded,
            label: '关于 App',
            value: 'Go Movie',
          ),
          const SizedBox(height: 8),
          OutlinedButton.icon(
            onPressed: onLogout,
            icon: const Icon(Icons.logout_rounded),
            label: const Text('退出登录'),
            style: OutlinedButton.styleFrom(
              foregroundColor: const Color(0xFFF87171),
              side: const BorderSide(color: Color(0x66F87171)),
              minimumSize: const Size.fromHeight(44),
              shape: RoundedRectangleBorder(
                borderRadius: BorderRadius.circular(8),
              ),
            ),
          ),
        ],
      ),
    );
  }
}

class _SettingRow extends StatelessWidget {
  const _SettingRow({
    required this.icon,
    required this.label,
    required this.value,
  });

  final IconData icon;
  final String label;
  final String value;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 12),
      child: Row(
        children: [
          Icon(icon, color: const Color(0xFF25D0AB), size: 20),
          const SizedBox(width: 10),
          Text(
            label,
            style: const TextStyle(
              color: Colors.white,
              fontWeight: FontWeight.w800,
            ),
          ),
          const Spacer(),
          Text(value, style: const TextStyle(color: Color(0xFF9CA3AF))),
        ],
      ),
    );
  }
}

class _SectionCard extends StatelessWidget {
  const _SectionCard({
    required this.title,
    required this.trailing,
    required this.child,
  });

  final String title;
  final String trailing;
  final Widget child;

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(14),
      decoration: BoxDecoration(
        color: const Color(0xFF171B24),
        borderRadius: BorderRadius.circular(8),
        border: Border.all(color: const Color(0xFF2B3140)),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Expanded(
                child: Text(
                  title,
                  style: const TextStyle(
                    color: Colors.white,
                    fontSize: 16,
                    fontWeight: FontWeight.w900,
                  ),
                ),
              ),
              Text(
                trailing,
                style: const TextStyle(
                  color: Color(0xFF9CA3AF),
                  fontSize: 12,
                  fontWeight: FontWeight.w700,
                ),
              ),
            ],
          ),
          const SizedBox(height: 12),
          child,
        ],
      ),
    );
  }
}

class _MetricBox extends StatelessWidget {
  const _MetricBox({required this.label, required this.value});

  final String label;
  final String value;

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 12),
      decoration: BoxDecoration(
        color: const Color(0xFF101318),
        borderRadius: BorderRadius.circular(8),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            value,
            style: const TextStyle(
              color: Color(0xFFF7C948),
              fontSize: 22,
              fontWeight: FontWeight.w900,
            ),
          ),
          const SizedBox(height: 2),
          Text(
            label,
            style: const TextStyle(color: Color(0xFF9CA3AF), fontSize: 12),
          ),
        ],
      ),
    );
  }
}

class _ShortcutItem {
  const _ShortcutItem(this.icon, this.title, this.subtitle);

  final IconData icon;
  final String title;
  final String subtitle;
}

class _MobileProfile {
  const _MobileProfile({
    required this.username,
    required this.nickname,
    required this.email,
    required this.status,
    required this.isVip,
    required this.vipUntil,
  });

  factory _MobileProfile.fromJson(Map<String, dynamic> json) {
    final rawVipUntil = json['vip_until'] as String?;
    return _MobileProfile(
      username: json['username'] as String? ?? '',
      nickname: json['nickname'] as String? ?? '',
      email: json['email'] as String? ?? '',
      status: json['status'] as String? ?? '',
      isVip: json['is_vip'] as bool? ?? false,
      vipUntil: rawVipUntil == null || rawVipUntil.isEmpty
          ? null
          : DateTime.tryParse(rawVipUntil),
    );
  }

  final String username;
  final String nickname;
  final String email;
  final String status;
  final bool isVip;
  final DateTime? vipUntil;

  String get displayName => nickname.isNotEmpty ? nickname : username;

  String get vipUntilLabel {
    final value = vipUntil;
    if (value == null) return '';
    final local = value.toLocal();
    return '${local.year}-${local.month.toString().padLeft(2, '0')}-${local.day.toString().padLeft(2, '0')}';
  }
}

class _OrderSummary {
  const _OrderSummary({
    required this.orderNo,
    required this.productName,
    required this.status,
    required this.amountCents,
    required this.currency,
  });

  factory _OrderSummary.fromJson(Map<String, dynamic> json) {
    final product = json['product'] as Map<String, dynamic>? ?? {};
    return _OrderSummary(
      orderNo: json['order_no'] as String? ?? '',
      productName: product['name'] as String? ?? '会员订单',
      status: json['status'] as String? ?? '',
      amountCents: json['amount_cents'] as int? ?? 0,
      currency: json['currency'] as String? ?? 'USD',
    );
  }

  final String orderNo;
  final String productName;
  final String status;
  final int amountCents;
  final String currency;

  String get priceLabel =>
      '$currency ${(amountCents / 100).toStringAsFixed(2)}';

  String get statusLabel => switch (status) {
    'paid' => '已支付',
    'paying' => '支付中',
    'pending' => '待支付',
    'closed' => '已关闭',
    'failed' => '失败',
    _ => status.isEmpty ? '未知' : status,
  };

  Color get statusColor => switch (status) {
    'paid' => const Color(0xFF25D0AB),
    'paying' || 'pending' => const Color(0xFFF7C948),
    'closed' || 'failed' => const Color(0xFFF87171),
    _ => const Color(0xFF9CA3AF),
  };
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
              color: selected
                  ? const Color(0xFF101318)
                  : const Color(0xFFD1D5DB),
              fontWeight: selected ? FontWeight.w800 : FontWeight.w600,
            ),
            selectedColor: const Color(0xFFF7C948),
            backgroundColor: const Color(0xFF1B1F2A),
            side: BorderSide(
              color: selected
                  ? const Color(0xFFF7C948)
                  : const Color(0xFF2B3140),
            ),
            shape: RoundedRectangleBorder(
              borderRadius: BorderRadius.circular(8),
            ),
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
                    colors: [
                      Color(0x99000000),
                      Color(0x00000000),
                      Color(0xDD000000),
                    ],
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
                      style: Theme.of(context).textTheme.headlineSmall
                          ?.copyWith(
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
                        style: const TextStyle(
                          color: Color(0xFFE5E7EB),
                          fontSize: 13,
                        ),
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
                        shape: RoundedRectangleBorder(
                          borderRadius: BorderRadius.circular(8),
                        ),
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
                            padding: const EdgeInsets.symmetric(
                              horizontal: 6,
                              vertical: 3,
                            ),
                            decoration: BoxDecoration(
                              color: const Color(0xCC000000),
                              borderRadius: BorderRadius.circular(6),
                            ),
                            child: Text(
                              video.durationLabel,
                              style: const TextStyle(
                                color: Colors.white,
                                fontSize: 11,
                              ),
                            ),
                          ),
                        ),
                      if (!video.isReady)
                        Container(
                          color: const Color(0x88000000),
                          child: Center(
                            child: Text(
                              _statusLabel(video.status),
                              style: const TextStyle(
                                color: Colors.white70,
                                fontSize: 11,
                              ),
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
                            padding: const EdgeInsets.symmetric(
                              horizontal: 5,
                              vertical: 2,
                            ),
                            decoration: BoxDecoration(
                              color: const Color(0xFFF7C948),
                              borderRadius: BorderRadius.circular(4),
                            ),
                            child: const Text(
                              'VIP',
                              style: TextStyle(
                                color: Color(0xFF101318),
                                fontSize: 10,
                                fontWeight: FontWeight.w900,
                              ),
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
                        style: const TextStyle(
                          color: Color(0xFF9CA3AF),
                          height: 1.35,
                        ),
                      ),
                    if (video.categoryName.isNotEmpty) ...[
                      const SizedBox(height: 6),
                      Text(
                        video.categoryName,
                        style: const TextStyle(
                          color: Color(0xFF25D0AB),
                          fontSize: 12,
                        ),
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
          color: states.contains(WidgetState.selected)
              ? Colors.white
              : const Color(0xFF9CA3AF),
          fontSize: 12,
          fontWeight: states.contains(WidgetState.selected)
              ? FontWeight.w800
              : FontWeight.w600,
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
