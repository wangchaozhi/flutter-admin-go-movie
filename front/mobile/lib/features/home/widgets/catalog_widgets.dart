import 'package:flutter/material.dart';

import '../../../core/l10n/app_strings.dart';
import '../../../models/video.dart';
import '../models/home_models.dart';

class ChannelTabs extends StatelessWidget {
  const ChannelTabs({
    super.key,
    required this.tabs,
    required this.selectedIndex,
    required this.onSelected,
    this.vipIndex = -1,
  });

  final List<String> tabs;
  final int selectedIndex;
  final ValueChanged<int> onSelected;

  /// Index of the VIP channel tab, or -1 if none. The VIP tab keeps a gold
  /// accent even when unselected so it reads as a premium zone.
  final int vipIndex;

  @override
  Widget build(BuildContext context) {
    const gold = Color(0xFFF7C948);
    return SizedBox(
      height: 48,
      child: ListView.separated(
        padding: const EdgeInsets.symmetric(horizontal: 16),
        scrollDirection: Axis.horizontal,
        itemBuilder: (context, index) {
          final selected = selectedIndex == index;
          final isVip = index == vipIndex;
          final foreground = selected
              ? const Color(0xFF101318)
              : (isVip ? gold : const Color(0xFFD1D5DB));
          return ChoiceChip(
            selected: selected,
            showCheckmark: false,
            avatar: isVip
                ? Icon(
                    Icons.workspace_premium_rounded,
                    size: 16,
                    color: foreground,
                  )
                : null,
            label: Text(tabs[index]),
            labelStyle: TextStyle(
              color: foreground,
              fontWeight: selected || isVip ? FontWeight.w800 : FontWeight.w600,
            ),
            selectedColor: gold,
            backgroundColor: isVip
                ? const Color(0x1AF7C948)
                : const Color(0xFF1B1F2A),
            side: BorderSide(
              color: selected || isVip ? gold : const Color(0xFF2B3140),
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

class FeaturedBanner extends StatelessWidget {
  const FeaturedBanner({super.key, required this.video, required this.onPlay});

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

class VideoTile extends StatelessWidget {
  const VideoTile({super.key, required this.video, required this.onTap});

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

class DiscoverView extends StatelessWidget {
  const DiscoverView({
    super.key,
    required this.categories,
    required this.videos,
    required this.loading,
    required this.favoriteVideoIds,
    required this.onOpenVideo,
    required this.onToggleFavorite,
    required this.isVip,
    required this.onOpenVip,
  });

  final List<Category> categories;
  final List<Video> videos;
  final bool loading;
  final Set<int> favoriteVideoIds;
  final ValueChanged<Video> onOpenVideo;
  final Future<void> Function(Video video) onToggleFavorite;
  final bool isVip;
  final VoidCallback onOpenVip;

  @override
  Widget build(BuildContext context) {
    final readyVideos = videos.where((video) => video.isReady).toList();
    final vipVideos = readyVideos
        .where((video) => video.isVip && !video.isFree)
        .toList();
    final freeVideos = readyVideos.where((video) => video.isFree).toList();
    final latestVideos = readyVideos.take(8).toList();
    final categoryRows = categories
        .map(
          (category) => (
            category,
            readyVideos
                .where((video) => video.categoryId == category.id)
                .take(6)
                .toList(),
          ),
        )
        .where((row) => row.$2.isNotEmpty)
        .toList();

    return ListView(
      padding: const EdgeInsets.fromLTRB(16, 12, 16, 112),
      children: [
        const _PageTitle(title: '发现', subtitle: '按兴趣浏览影片与会员精选'),
        const SizedBox(height: 14),
        _DiscoverStatsBar(
          total: readyVideos.length,
          categoryCount: categories.length,
          vipCount: vipVideos.length,
        ),
        if (!isVip) ...[
          const SizedBox(height: 14),
          _VipDiscoveryBand(count: vipVideos.length, onTap: onOpenVip),
        ],
        const SizedBox(height: 18),
        if (loading)
          const _InlineLoading()
        else if (readyVideos.isEmpty)
          const _EmptyBlock(text: '暂无可发现内容')
        else ...[
          if (latestVideos.isNotEmpty)
            _HorizontalVideoSection(
              title: '最新上线',
              videos: latestVideos,
              favoriteVideoIds: favoriteVideoIds,
              onOpenVideo: onOpenVideo,
              onToggleFavorite: onToggleFavorite,
            ),
          if (freeVideos.isNotEmpty) ...[
            const SizedBox(height: 18),
            _HorizontalVideoSection(
              title: '免费可看',
              videos: freeVideos.take(8).toList(),
              favoriteVideoIds: favoriteVideoIds,
              onOpenVideo: onOpenVideo,
              onToggleFavorite: onToggleFavorite,
            ),
          ],
          for (final row in categoryRows) ...[
            const SizedBox(height: 18),
            _HorizontalVideoSection(
              title: row.$1.name,
              videos: row.$2,
              favoriteVideoIds: favoriteVideoIds,
              onOpenVideo: onOpenVideo,
              onToggleFavorite: onToggleFavorite,
            ),
          ],
        ],
      ],
    );
  }
}

class PlaylistView extends StatelessWidget {
  const PlaylistView({
    super.key,
    required this.favorites,
    required this.history,
    required this.videos,
    required this.loadingFavorites,
    required this.loadingHistory,
    required this.onOpenVideo,
    required this.onToggleFavorite,
    required this.onRemoveFavorite,
    required this.onRefresh,
  });

  final List<FavoriteEntry> favorites;
  final List<HistoryEntry> history;
  final List<Video> videos;
  final bool loadingFavorites;
  final bool loadingHistory;
  final ValueChanged<Video> onOpenVideo;
  final Future<void> Function(Video video) onToggleFavorite;
  final Future<void> Function(Video video) onRemoveFavorite;
  final Future<void> Function() onRefresh;

  @override
  Widget build(BuildContext context) {
    final vipVideos = videos
        .where((video) => video.isReady && video.isVip && !video.isFree)
        .take(8)
        .toList();
    final latestVideos = videos
        .where((video) => video.isReady)
        .take(8)
        .toList();

    return RefreshIndicator(
      color: const Color(0xFF25D0AB),
      onRefresh: onRefresh,
      child: ListView(
        padding: const EdgeInsets.fromLTRB(16, 12, 16, 112),
        children: [
          const _PageTitle(title: '片单', subtitle: '收藏、继续观看和精选内容都在这里'),
          const SizedBox(height: 14),
          _PlaylistSummary(
            favoriteCount: favorites.length,
            historyCount: history.length,
            savedCount: favorites.length + history.length,
          ),
          const SizedBox(height: 18),
          _FavoritePlaylistSection(
            favorites: favorites,
            loading: loadingFavorites,
            onOpenVideo: onOpenVideo,
            onRemoveFavorite: onRemoveFavorite,
          ),
          const SizedBox(height: 18),
          _HistoryPlaylistSection(
            history: history,
            loading: loadingHistory,
            onOpenVideo: onOpenVideo,
          ),
          if (vipVideos.isNotEmpty) ...[
            const SizedBox(height: 18),
            _HorizontalVideoSection(
              title: 'VIP 精选片单',
              videos: vipVideos,
              favoriteVideoIds: favorites
                  .map((entry) => entry.video.id)
                  .toSet(),
              onOpenVideo: onOpenVideo,
              onToggleFavorite: onToggleFavorite,
            ),
          ],
          if (latestVideos.isNotEmpty) ...[
            const SizedBox(height: 18),
            _HorizontalVideoSection(
              title: '稍后可看',
              videos: latestVideos,
              favoriteVideoIds: favorites
                  .map((entry) => entry.video.id)
                  .toSet(),
              onOpenVideo: onOpenVideo,
              onToggleFavorite: onToggleFavorite,
            ),
          ],
        ],
      ),
    );
  }
}

class _PageTitle extends StatelessWidget {
  const _PageTitle({required this.title, required this.subtitle});

  final String title;
  final String subtitle;

  @override
  Widget build(BuildContext context) {
    return Row(
      children: [
        Expanded(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                title,
                style: Theme.of(context).textTheme.headlineSmall?.copyWith(
                  color: Colors.white,
                  fontWeight: FontWeight.w900,
                ),
              ),
              const SizedBox(height: 3),
              Text(
                subtitle,
                style: const TextStyle(color: Color(0xFF9CA3AF), fontSize: 13),
              ),
            ],
          ),
        ),
      ],
    );
  }
}

class _DiscoverStatsBar extends StatelessWidget {
  const _DiscoverStatsBar({
    required this.total,
    required this.categoryCount,
    required this.vipCount,
  });

  final int total;
  final int categoryCount;
  final int vipCount;

  @override
  Widget build(BuildContext context) {
    return Row(
      children: [
        Expanded(
          child: _MiniStat(label: '可看', value: '$total'),
        ),
        const SizedBox(width: 10),
        Expanded(
          child: _MiniStat(label: '频道', value: '$categoryCount'),
        ),
        const SizedBox(width: 10),
        Expanded(
          child: _MiniStat(label: 'VIP', value: '$vipCount'),
        ),
      ],
    );
  }
}

class _PlaylistSummary extends StatelessWidget {
  const _PlaylistSummary({
    required this.favoriteCount,
    required this.historyCount,
    required this.savedCount,
  });

  final int favoriteCount;
  final int historyCount;
  final int savedCount;

  @override
  Widget build(BuildContext context) {
    return Row(
      children: [
        Expanded(
          child: _MiniStat(label: '收藏', value: '$favoriteCount'),
        ),
        const SizedBox(width: 10),
        Expanded(
          child: _MiniStat(label: '记录', value: '$historyCount'),
        ),
        const SizedBox(width: 10),
        Expanded(
          child: _MiniStat(label: '片单', value: '$savedCount'),
        ),
      ],
    );
  }
}

class _MiniStat extends StatelessWidget {
  const _MiniStat({required this.label, required this.value});

  final String label;
  final String value;

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 12),
      decoration: BoxDecoration(
        color: const Color(0xFF171B24),
        borderRadius: BorderRadius.circular(8),
        border: Border.all(color: const Color(0xFF2B3140)),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            value,
            style: const TextStyle(
              color: Color(0xFFF7C948),
              fontSize: 20,
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

class _VipDiscoveryBand extends StatelessWidget {
  const _VipDiscoveryBand({required this.count, required this.onTap});

  final int count;
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
              const Icon(
                Icons.workspace_premium_rounded,
                color: Color(0xFFF7C948),
                size: 30,
              ),
              const SizedBox(width: 12),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    const Text(
                      '会员发现',
                      style: TextStyle(
                        color: Colors.white,
                        fontWeight: FontWeight.w900,
                        fontSize: 17,
                      ),
                    ),
                    const SizedBox(height: 3),
                    Text(
                      count > 0 ? '$count 部会员内容待解锁' : '开通后解锁更多会员内容',
                      style: const TextStyle(color: Color(0xFFE5E7EB)),
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

class _HorizontalVideoSection extends StatelessWidget {
  const _HorizontalVideoSection({
    required this.title,
    required this.videos,
    required this.favoriteVideoIds,
    required this.onOpenVideo,
    required this.onToggleFavorite,
  });

  final String title;
  final List<Video> videos;
  final Set<int> favoriteVideoIds;
  final ValueChanged<Video> onOpenVideo;
  final Future<void> Function(Video video) onToggleFavorite;

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          title,
          style: const TextStyle(
            color: Colors.white,
            fontSize: 17,
            fontWeight: FontWeight.w900,
          ),
        ),
        const SizedBox(height: 10),
        SizedBox(
          height: 198,
          child: ListView.separated(
            scrollDirection: Axis.horizontal,
            itemBuilder: (context, index) {
              final video = videos[index];
              return _PosterCard(
                video: video,
                favorited: favoriteVideoIds.contains(video.id),
                onOpenVideo: onOpenVideo,
                onToggleFavorite: onToggleFavorite,
              );
            },
            separatorBuilder: (_, _) => const SizedBox(width: 12),
            itemCount: videos.length,
          ),
        ),
      ],
    );
  }
}

class _PosterCard extends StatelessWidget {
  const _PosterCard({
    required this.video,
    required this.favorited,
    required this.onOpenVideo,
    required this.onToggleFavorite,
  });

  final Video video;
  final bool favorited;
  final ValueChanged<Video> onOpenVideo;
  final Future<void> Function(Video video) onToggleFavorite;

  @override
  Widget build(BuildContext context) {
    return SizedBox(
      width: 148,
      child: Material(
        color: const Color(0xFF171B24),
        borderRadius: BorderRadius.circular(8),
        child: InkWell(
          borderRadius: BorderRadius.circular(8),
          onTap: video.isReady ? () => onOpenVideo(video) : null,
          child: Padding(
            padding: const EdgeInsets.all(8),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Expanded(
                  child: ClipRRect(
                    borderRadius: BorderRadius.circular(8),
                    child: Stack(
                      fit: StackFit.expand,
                      children: [
                        video.fullCoverUrl.isEmpty
                            ? const ColoredBox(color: Color(0xFF202532))
                            : Image.network(
                                video.fullCoverUrl,
                                fit: BoxFit.cover,
                                errorBuilder: (_, _, _) =>
                                    const ColoredBox(color: Color(0xFF202532)),
                              ),
                        Positioned(
                          top: 6,
                          right: 6,
                          child: _FavoriteIconButton(
                            favorited: favorited,
                            onPressed: () {
                              onToggleFavorite(video);
                            },
                          ),
                        ),
                        if (video.durationLabel.isNotEmpty)
                          Positioned(
                            left: 6,
                            bottom: 6,
                            child: _MetaPill(text: video.durationLabel),
                          ),
                      ],
                    ),
                  ),
                ),
                const SizedBox(height: 8),
                Text(
                  video.title,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: const TextStyle(
                    color: Colors.white,
                    fontWeight: FontWeight.w900,
                  ),
                ),
                const SizedBox(height: 2),
                Text(
                  video.categoryName.isEmpty ? '影片' : video.categoryName,
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
        ),
      ),
    );
  }
}

class _FavoritePlaylistSection extends StatelessWidget {
  const _FavoritePlaylistSection({
    required this.favorites,
    required this.loading,
    required this.onOpenVideo,
    required this.onRemoveFavorite,
  });

  final List<FavoriteEntry> favorites;
  final bool loading;
  final ValueChanged<Video> onOpenVideo;
  final Future<void> Function(Video video) onRemoveFavorite;

  @override
  Widget build(BuildContext context) {
    return _VerticalSection(
      title: '我的收藏',
      loading: loading,
      emptyText: '暂无收藏影片，可以先去发现页加入片单',
      children: [
        for (final item in favorites)
          _LibraryVideoRow(
            video: item.video,
            subtitle: item.video.categoryName.isEmpty
                ? '已加入片单'
                : item.video.categoryName,
            trailingIcon: Icons.bookmark_remove_rounded,
            onTap: () => onOpenVideo(item.video),
            onTrailingTap: () {
              onRemoveFavorite(item.video);
            },
          ),
      ],
    );
  }
}

class _HistoryPlaylistSection extends StatelessWidget {
  const _HistoryPlaylistSection({
    required this.history,
    required this.loading,
    required this.onOpenVideo,
  });

  final List<HistoryEntry> history;
  final bool loading;
  final ValueChanged<Video> onOpenVideo;

  @override
  Widget build(BuildContext context) {
    return _VerticalSection(
      title: '继续观看',
      loading: loading,
      emptyText: '暂无观看记录',
      children: [
        for (final item in history)
          _LibraryVideoRow(
            video: item.video,
            subtitle: '已观看 ${item.progress}%',
            trailingIcon: Icons.play_arrow_rounded,
            onTap: () => onOpenVideo(item.video),
          ),
      ],
    );
  }
}

class _VerticalSection extends StatelessWidget {
  const _VerticalSection({
    required this.title,
    required this.loading,
    required this.emptyText,
    required this.children,
  });

  final String title;
  final bool loading;
  final String emptyText;
  final List<Widget> children;

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          title,
          style: const TextStyle(
            color: Colors.white,
            fontSize: 17,
            fontWeight: FontWeight.w900,
          ),
        ),
        const SizedBox(height: 10),
        if (loading)
          const _InlineLoading()
        else if (children.isEmpty)
          _EmptyBlock(text: emptyText)
        else
          Column(children: children),
      ],
    );
  }
}

class _LibraryVideoRow extends StatelessWidget {
  const _LibraryVideoRow({
    required this.video,
    required this.subtitle,
    required this.trailingIcon,
    required this.onTap,
    this.onTrailingTap,
  });

  final Video video;
  final String subtitle;
  final IconData trailingIcon;
  final VoidCallback onTap;
  final VoidCallback? onTrailingTap;

  @override
  Widget build(BuildContext context) {
    return Container(
      margin: const EdgeInsets.only(bottom: 10),
      child: Material(
        color: const Color(0xFF171B24),
        borderRadius: BorderRadius.circular(8),
        child: InkWell(
          borderRadius: BorderRadius.circular(8),
          onTap: onTap,
          child: Padding(
            padding: const EdgeInsets.all(10),
            child: Row(
              children: [
                ClipRRect(
                  borderRadius: BorderRadius.circular(8),
                  child: SizedBox(
                    width: 72,
                    height: 50,
                    child: video.fullCoverUrl.isEmpty
                        ? const ColoredBox(color: Color(0xFF202532))
                        : Image.network(
                            video.fullCoverUrl,
                            fit: BoxFit.cover,
                            errorBuilder: (_, _, _) =>
                                const ColoredBox(color: Color(0xFF202532)),
                          ),
                  ),
                ),
                const SizedBox(width: 12),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        video.title,
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                        style: const TextStyle(
                          color: Colors.white,
                          fontWeight: FontWeight.w900,
                        ),
                      ),
                      const SizedBox(height: 4),
                      Text(
                        subtitle,
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
                IconButton(
                  onPressed: onTrailingTap ?? onTap,
                  icon: Icon(trailingIcon),
                  color: const Color(0xFF25D0AB),
                  tooltip: '操作',
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}

class _FavoriteIconButton extends StatelessWidget {
  const _FavoriteIconButton({required this.favorited, required this.onPressed});

  final bool favorited;
  final VoidCallback onPressed;

  @override
  Widget build(BuildContext context) {
    return SizedBox(
      width: 34,
      height: 34,
      child: IconButton.filled(
        padding: EdgeInsets.zero,
        style: IconButton.styleFrom(
          backgroundColor: const Color(0xCC101318),
          foregroundColor: favorited
              ? const Color(0xFFF7C948)
              : const Color(0xFFE5E7EB),
        ),
        onPressed: onPressed,
        icon: Icon(
          favorited ? Icons.bookmark_rounded : Icons.bookmark_border_rounded,
          size: 20,
        ),
        tooltip: favorited ? '移出片单' : '加入片单',
      ),
    );
  }
}

class _InlineLoading extends StatelessWidget {
  const _InlineLoading();

  @override
  Widget build(BuildContext context) {
    return const Padding(
      padding: EdgeInsets.symmetric(vertical: 24),
      child: Center(child: CircularProgressIndicator(color: Color(0xFF25D0AB))),
    );
  }
}

class _EmptyBlock extends StatelessWidget {
  const _EmptyBlock({required this.text});

  final String text;

  @override
  Widget build(BuildContext context) {
    return Container(
      width: double.infinity,
      padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 22),
      decoration: BoxDecoration(
        color: const Color(0xFF171B24),
        borderRadius: BorderRadius.circular(8),
        border: Border.all(color: const Color(0xFF2B3140)),
      ),
      child: Text(text, style: const TextStyle(color: Color(0xFF9CA3AF))),
    );
  }
}

class HomeBottomNav extends StatelessWidget {
  const HomeBottomNav({
    super.key,
    required this.selectedIndex,
    required this.onSelected,
  });

  final int selectedIndex;
  final ValueChanged<int> onSelected;

  @override
  Widget build(BuildContext context) {
    final s = AppStrings.of(context);
    final items = [
      (Icons.home_rounded, s.t('nav.home')),
      (Icons.explore_rounded, s.t('nav.discover')),
      (Icons.bookmark_rounded, s.t('nav.library')),
      (Icons.person_rounded, s.t('nav.mine')),
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
