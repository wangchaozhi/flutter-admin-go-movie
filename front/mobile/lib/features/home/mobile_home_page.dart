import 'package:flutter/material.dart';

import '../../core/api_client.dart';
import '../../core/l10n/app_strings.dart';
import '../../core/session.dart';
import '../../models/video.dart';
import '../series/series_detail_page.dart';
import '../series/series_models.dart';
import '../series/series_rail.dart';
import 'models/home_models.dart';
import 'widgets/home_widgets.dart';
import 'widgets/video_rail.dart';

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
  List<Video> _railPopular = [];
  List<Video> _railLatest = [];
  List<Video> _railVip = [];
  List<Video> _railContinue = [];
  List<Series> _seriesList = [];
  Map<int, int> _continueProgress = {};
  List<OrderSummary> _orders = [];
  List<HistoryEntry> _history = [];
  List<FavoriteEntry> _favorites = [];
  bool _loadingCategories = true;
  bool _loadingVideos = false;
  bool _loadingProfile = false;
  bool _loadingOrders = false;
  bool _loadingHistory = false;
  bool _loadingFavorites = false;
  bool _profileLoaded = false;
  bool _libraryLoaded = false;
  bool _favoritesLoaded = false;
  String? _username;
  MobileProfile? _profile;
  MobileSetting? _setting;
  String _profileError = '';

  @override
  void initState() {
    super.initState();
    _loadAll();
  }

  Future<void> _loadAll() async {
    final username = await Session.username();
    if (mounted) setState(() => _username = username);
    await Future.wait([
      _loadCategories(),
      _loadVideos(categoryId: 0),
      _loadHomeRails(),
      _loadSeries(),
    ]);
  }

  // Top series shown as a rail on the 全部 channel. Best-effort: a failure just
  // hides the rail and never blocks the rest of the home page.
  Future<void> _loadSeries() async {
    try {
      final resp = await ApiClient().get('/api/series');
      if (!mounted) return;
      if (resp['code'] == 0) {
        final data = resp['data'] as Map<String, dynamic>? ?? {};
        final list = (data['items'] as List<dynamic>? ?? [])
            .map((e) => Series.fromJson(e as Map<String, dynamic>))
            .where((series) => series.episodeCount > 0)
            .toList();
        setState(() => _seriesList = list);
      }
    } catch (_) {
      // series rail is optional; ignore network errors
    }
  }

  void _openSeries(Series series) {
    Navigator.of(context).push(
      MaterialPageRoute<void>(
        builder: (_) => SeriesDetailPage(seriesId: series.id, initial: series),
      ),
    );
  }

  // Aggregated recommendation rails shown on the 全部 channel. Sent with the auth
  // token so the backend can include the personalised "继续观看" rail.
  Future<void> _loadHomeRails() async {
    try {
      final resp = await ApiClient().getAuth('/api/home');
      if (!mounted) return;
      if (resp['code'] == 0) {
        final data = resp['data'] as Map<String, dynamic>? ?? {};
        List<Video> parse(String key) =>
            (data[key] as List<dynamic>? ?? [])
                .map((e) => Video.fromJson(e as Map<String, dynamic>))
                .toList();
        // The continue rail carries per-video watch progress alongside the
        // video, so build a lookup keyed by video id for the progress bars.
        final continueRaw = data['continue'] as List<dynamic>? ?? [];
        final continueProgress = <int, int>{};
        for (final entry in continueRaw) {
          final map = entry as Map<String, dynamic>;
          final id = map['id'] as int?;
          if (id != null) {
            continueProgress[id] = (map['progress'] as num?)?.toInt() ?? 0;
          }
        }
        setState(() {
          _railPopular = parse('popular');
          _railLatest = parse('latest');
          _railVip = parse('vip');
          _railContinue = parse('continue');
          _continueProgress = continueProgress;
        });
      }
    } catch (_) {
      // rails are optional; ignore network errors
    }
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

  Future<void> _loadVideos({int categoryId = 0, bool vipOnly = false}) async {
    if (!mounted) return;
    setState(() => _loadingVideos = true);
    try {
      final params = <String>['per_page=50'];
      if (categoryId > 0) params.add('category_id=$categoryId');
      if (vipOnly) params.add('is_vip=1');
      final path = '/api/videos?${params.join('&')}';
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

  // Home channel tabs are [全部, VIP, ...categories]. Index 0 = all, index 1 =
  // VIP channel (vip-only), index >= 2 = the category at index - 2.
  void _onCategorySelected(int index) {
    setState(() => _selectedCategoryIndex = index);
    if (index == 0) {
      _loadVideos();
    } else if (index == 1) {
      _loadVideos(vipOnly: true);
    } else {
      _loadVideos(categoryId: _categories[index - 2].id);
    }
  }

  void _openVideo(Video video) {
    Navigator.pushNamed(context, '/player', arguments: video);
  }

  Future<void> _openVipAndRefreshProfile() async {
    await Navigator.pushNamed(context, '/vip');
    if (!mounted) return;
    await _loadProfile();
    if (_profileLoaded) {
      await _loadOrders();
    }
  }

  Future<void> _loadProfileCenter({bool force = false}) async {
    if (_profileLoaded && !force) return;
    if (!mounted) return;
    setState(() {
      _loadingProfile = true;
      _loadingOrders = true;
      _profileError = '';
    });
    await Future.wait([
      _loadProfile(),
      _loadOrders(),
      _loadHistory(),
      _loadFavorites(),
      _loadSetting(),
    ]);
    if (mounted) {
      setState(() {
        _profileLoaded = true;
        _libraryLoaded = true;
      });
    }
  }

  Future<void> _loadProfile() async {
    try {
      final resp = await ApiClient().getAuth('/api/mobile/profile');
      if (!mounted) return;
      if (resp['code'] == 0) {
        final data = resp['data'] as Map<String, dynamic>? ?? {};
        setState(() {
          _profile = MobileProfile.fromJson(data);
          _loadingProfile = false;
        });
      } else {
        setState(() {
          _profileError = resp['msg']?.toString() ?? AppStrings.of(context).t('home.profileLoadFailed');
          _loadingProfile = false;
        });
      }
    } catch (_) {
      if (mounted) {
        setState(() {
          _profileError = AppStrings.of(context).t('home.profileLoadFailed');
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
            .map((item) => OrderSummary.fromJson(item as Map<String, dynamic>))
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

  Future<void> _loadHistory() async {
    if (!mounted) return;
    setState(() => _loadingHistory = true);
    try {
      final resp = await ApiClient().getAuth(
        '/api/mobile/watch-history?limit=20',
      );
      if (!mounted) return;
      if (resp['code'] == 0) {
        final list = (resp['data'] as List<dynamic>? ?? [])
            .map((item) => HistoryEntry.fromJson(item as Map<String, dynamic>))
            .toList();
        setState(() => _history = list);
      }
    } catch (_) {
      // keep existing history
    } finally {
      if (mounted) setState(() => _loadingHistory = false);
    }
  }

  Future<void> _loadFavorites() async {
    if (!mounted) return;
    setState(() => _loadingFavorites = true);
    try {
      final resp = await ApiClient().getAuth('/api/mobile/favorites?limit=20');
      if (!mounted) return;
      if (resp['code'] == 0) {
        final list = (resp['data'] as List<dynamic>? ?? [])
            .map((item) => FavoriteEntry.fromJson(item as Map<String, dynamic>))
            .toList();
        setState(() => _favorites = list);
      }
    } catch (_) {
      // keep existing favorites
    } finally {
      if (mounted) {
        setState(() {
          _loadingFavorites = false;
          _favoritesLoaded = true;
        });
      }
    }
  }

  Future<void> _loadSetting() async {
    try {
      final resp = await ApiClient().getAuth('/api/mobile/settings');
      if (!mounted) return;
      if (resp['code'] == 0) {
        final data = resp['data'] as Map<String, dynamic>? ?? {};
        setState(() => _setting = MobileSetting.fromJson(data));
      }
    } catch (_) {
      // settings are optional for this screen
    }
  }

  Future<void> _saveSetting(MobileSetting setting) async {
    final resp = await ApiClient().putAuth(
      '/api/mobile/settings',
      setting.toJson(),
    );
    if (!mounted) return;
    if (resp['code'] == 0) {
      final data = resp['data'] as Map<String, dynamic>? ?? {};
      setState(() => _setting = MobileSetting.fromJson(data));
    }
  }

  Future<void> _removeFavorite(Video video) async {
    await ApiClient().deleteAuth('/api/mobile/favorites/${video.id}');
    await _loadFavorites();
  }

  Future<void> _addFavorite(Video video) async {
    await ApiClient().postAuth('/api/mobile/favorites/${video.id}', {});
    await _loadFavorites();
  }

  Future<void> _loadLibrary({bool force = false}) async {
    if (_libraryLoaded && !force) return;
    await Future.wait([_loadHistory(), _loadFavorites()]);
    if (mounted) setState(() => _libraryLoaded = true);
  }

  Future<void> _showHistorySheet() async {
    await _loadHistory();
    if (!mounted) return;
    showModalBottomSheet<void>(
      context: context,
      backgroundColor: const Color(0xFF171B24),
      showDragHandle: true,
      builder: (_) => HistorySheet(
        items: _history,
        loading: _loadingHistory,
        onOpenVideo: _openVideo,
      ),
    );
  }

  Future<void> _showFavoritesSheet() async {
    await _loadFavorites();
    if (!mounted) return;
    showModalBottomSheet<void>(
      context: context,
      backgroundColor: const Color(0xFF171B24),
      showDragHandle: true,
      builder: (_) => FavoritesSheet(
        items: _favorites,
        loading: _loadingFavorites,
        onOpenVideo: _openVideo,
        onRemove: _removeFavorite,
      ),
    );
  }

  void _showOrdersSheet() {
    showModalBottomSheet<void>(
      context: context,
      backgroundColor: const Color(0xFF171B24),
      showDragHandle: true,
      builder: (_) => OrdersSheet(orders: _orders, loading: _loadingOrders),
    );
  }

  void _showSettingsSheet() {
    showModalBottomSheet<void>(
      context: context,
      backgroundColor: const Color(0xFF171B24),
      showDragHandle: true,
      builder: (_) => SettingsSheet(
        setting: _setting ?? MobileSetting.defaults(),
        onSave: _saveSetting,
      ),
    );
  }

  void _onNavSelected(int index) {
    setState(() => _selectedNav = index);
    switch (index) {
      case 1:
        if (!_favoritesLoaded) _loadFavorites();
        if (_profile == null) _loadProfile();
      case 2:
        _loadLibrary();
      case 3:
        _loadProfileCenter(force: true);
    }
  }

  @override
  Widget build(BuildContext context) {
    final s = AppStrings.of(context);
    final tabs = [s.t('home.all'), 'VIP', ..._categories.map((c) => c.name)];
    final featuredVideo = _videos.where((v) => v.isReady).firstOrNull;
    final favoriteVideoIds = _favorites.map((entry) => entry.video.id).toSet();

    final content = switch (_selectedNav) {
      1 => DiscoverView(
        categories: _categories,
        videos: _videos,
        loading: _loadingVideos,
        favoriteVideoIds: favoriteVideoIds,
        onOpenVideo: _openVideo,
        onToggleFavorite: (video) => favoriteVideoIds.contains(video.id)
            ? _removeFavorite(video)
            : _addFavorite(video),
        isVip: _profile?.isVip ?? false,
        onOpenVip: () {
          _openVipAndRefreshProfile();
        },
      ),
      2 => PlaylistView(
        favorites: _favorites,
        history: _history,
        videos: _videos,
        loadingFavorites: _loadingFavorites,
        loadingHistory: _loadingHistory,
        onOpenVideo: _openVideo,
        onToggleFavorite: (video) => favoriteVideoIds.contains(video.id)
            ? _removeFavorite(video)
            : _addFavorite(video),
        onRemoveFavorite: _removeFavorite,
        onRefresh: () => _loadLibrary(force: true),
      ),
      3 => ProfileCenter(
        username: _username ?? '',
        profile: _profile,
        orders: _orders,
        loadingProfile: _loadingProfile,
        loadingOrders: _loadingOrders,
        error: _profileError,
        videos: _videos,
        historyCount: _history.length,
        favoriteCount: _favorites.length,
        onOpenVip: () {
          _openVipAndRefreshProfile();
        },
        onOpenHistory: _showHistorySheet,
        onOpenFavorites: _showFavoritesSheet,
        onOpenOrders: _showOrdersSheet,
        onOpenSettings: _showSettingsSheet,
        onRefresh: () => _loadProfileCenter(force: true),
        onLogout: _logout,
      ),
      _ => CustomScrollView(
        slivers: [
          SliverToBoxAdapter(
            child: HomeTopBar(
              username: _username ?? '',
              onOpenVip: () {
                _openVipAndRefreshProfile();
              },
              onLogout: _logout,
            ),
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
                : ChannelTabs(
                    tabs: tabs,
                    selectedIndex: _selectedCategoryIndex,
                    onSelected: _onCategorySelected,
                    vipIndex: 1,
                  ),
          ),
          if (featuredVideo != null)
            SliverToBoxAdapter(
              child: FeaturedBanner(video: featuredVideo, onPlay: _openVideo),
            ),
          if (_selectedCategoryIndex == 0) ...[
            SliverToBoxAdapter(
              child: VideoRail(
                title: s.t('home.continueWatch'),
                videos: _railContinue,
                onOpenVideo: _openVideo,
                progress: _continueProgress,
              ),
            ),
            SliverToBoxAdapter(
              child: VideoRail(
                title: s.t('home.popular'),
                videos: _railPopular,
                onOpenVideo: _openVideo,
              ),
            ),
            SliverToBoxAdapter(
              child: VideoRail(
                title: s.t('home.newArrivals'),
                videos: _railLatest,
                onOpenVideo: _openVideo,
              ),
            ),
            SliverToBoxAdapter(
              child: SeriesRail(
                title: s.t('home.series'),
                series: _seriesList,
                onOpenSeries: _openSeries,
              ),
            ),
            SliverToBoxAdapter(
              child: VideoRail(
                title: s.t('home.vipPicks'),
                videos: _railVip,
                onOpenVideo: _openVideo,
              ),
            ),
          ],
          if (_loadingVideos)
            const SliverToBoxAdapter(
              child: Padding(
                padding: EdgeInsets.all(32),
                child: Center(
                  child: CircularProgressIndicator(color: Color(0xFF25D0AB)),
                ),
              ),
            )
          else if (_videos.isEmpty)
            SliverToBoxAdapter(
              child: Padding(
                padding: const EdgeInsets.all(48),
                child: Center(
                  child: Text(
                    _selectedCategoryIndex == 1 ? s.t('home.emptyVip') : s.t('home.emptyVideos'),
                    style: const TextStyle(color: Color(0xFF9CA3AF)),
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
                    VideoTile(video: _videos[index], onTap: _openVideo),
              ),
            ),
        ],
      ),
    };

    return Scaffold(
      backgroundColor: const Color(0xFF0D0F14),
      body: SafeArea(bottom: false, child: content),
      bottomNavigationBar: HomeBottomNav(
        selectedIndex: _selectedNav,
        onSelected: _onNavSelected,
      ),
    );
  }
}
