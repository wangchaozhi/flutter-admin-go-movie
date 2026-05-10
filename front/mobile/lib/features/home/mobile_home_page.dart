import 'package:flutter/material.dart';

class MobileHomePage extends StatefulWidget {
  const MobileHomePage({super.key});

  @override
  State<MobileHomePage> createState() => _MobileHomePageState();
}

class _MobileHomePageState extends State<MobileHomePage> {
  int _selectedChannel = 0;
  int _selectedNav = 0;

  static const _channels = ['推荐', '电影', '剧集', '综艺', '动漫'];

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: const Color(0xFF0D0F14),
      body: SafeArea(
        bottom: false,
        child: CustomScrollView(
          slivers: [
            SliverToBoxAdapter(
              child: _TopBar(
                onLogout: () =>
                    Navigator.pushReplacementNamed(context, '/login'),
              ),
            ),
            SliverToBoxAdapter(
              child: _ChannelTabs(
                channels: _channels,
                selectedIndex: _selectedChannel,
                onSelected: (index) => setState(() => _selectedChannel = index),
              ),
            ),
            const SliverToBoxAdapter(child: _FeaturedPlayer()),
            const SliverToBoxAdapter(child: _ContinueWatching()),
            SliverPadding(
              padding: const EdgeInsets.fromLTRB(16, 18, 16, 112),
              sliver: SliverList.separated(
                itemCount: _videos.length,
                separatorBuilder: (_, _) => const SizedBox(height: 14),
                itemBuilder: (context, index) =>
                    _VideoTile(video: _videos[index]),
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
  const _TopBar({required this.onLogout});

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
                const Text(
                  '今晚想看点什么？',
                  style: TextStyle(color: Color(0xFF9CA3AF), fontSize: 13),
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
    required this.channels,
    required this.selectedIndex,
    required this.onSelected,
  });

  final List<String> channels;
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
            label: Text(channels[index]),
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
        itemCount: channels.length,
      ),
    );
  }
}

class _FeaturedPlayer extends StatelessWidget {
  const _FeaturedPlayer();

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
              Image.network(
                'https://images.unsplash.com/photo-1485846234645-a62644f84728?auto=format&fit=crop&w=1200&q=80',
                fit: BoxFit.cover,
                errorBuilder: (_, _, _) =>
                    const ColoredBox(color: Color(0xFF202532)),
              ),
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
                    const _MetaPill(text: '独家首播  4K HDR'),
                    const SizedBox(height: 10),
                    Text(
                      '霓虹城市追踪',
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: Theme.of(context).textTheme.headlineSmall
                          ?.copyWith(
                            color: Colors.white,
                            fontWeight: FontWeight.w900,
                          ),
                    ),
                    const SizedBox(height: 8),
                    const Text(
                      '科幻 / 悬疑 / 118 分钟',
                      style: TextStyle(color: Color(0xFFE5E7EB), fontSize: 13),
                    ),
                    const SizedBox(height: 14),
                    Row(
                      children: [
                        FilledButton.icon(
                          onPressed: () {},
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
                        const SizedBox(width: 10),
                        IconButton.filledTonal(
                          onPressed: () {},
                          icon: const Icon(Icons.add_rounded),
                          color: Colors.white,
                          tooltip: '加入片单',
                          style: IconButton.styleFrom(
                            backgroundColor: const Color(0x662B3140),
                          ),
                        ),
                      ],
                    ),
                  ],
                ),
              ),
              const Center(
                child: CircleAvatar(
                  radius: 31,
                  backgroundColor: Color(0xCC101318),
                  child: Icon(
                    Icons.play_arrow_rounded,
                    color: Colors.white,
                    size: 38,
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

class _ContinueWatching extends StatelessWidget {
  const _ContinueWatching();

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.fromLTRB(16, 22, 16, 0),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const _SectionTitle(title: '继续观看'),
          const SizedBox(height: 12),
          SizedBox(
            height: 148,
            child: ListView.separated(
              scrollDirection: Axis.horizontal,
              itemCount: _watching.length,
              separatorBuilder: (_, _) => const SizedBox(width: 12),
              itemBuilder: (context, index) =>
                  _WatchingCard(video: _watching[index]),
            ),
          ),
        ],
      ),
    );
  }
}

class _WatchingCard extends StatelessWidget {
  const _WatchingCard({required this.video});

  final VideoItem video;

  @override
  Widget build(BuildContext context) {
    return SizedBox(
      width: 174,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          ClipRRect(
            borderRadius: BorderRadius.circular(8),
            child: AspectRatio(
              aspectRatio: 16 / 9,
              child: Stack(
                fit: StackFit.expand,
                children: [
                  Image.network(
                    video.imageUrl,
                    fit: BoxFit.cover,
                    errorBuilder: (_, _, _) =>
                        const ColoredBox(color: Color(0xFF202532)),
                  ),
                  const Center(
                    child: Icon(
                      Icons.play_circle_fill_rounded,
                      color: Colors.white,
                      size: 34,
                    ),
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
              fontWeight: FontWeight.w800,
            ),
          ),
          const SizedBox(height: 6),
          ClipRRect(
            borderRadius: BorderRadius.circular(99),
            child: LinearProgressIndicator(
              value: video.progress,
              minHeight: 4,
              backgroundColor: const Color(0xFF2B3140),
              valueColor: const AlwaysStoppedAnimation(Color(0xFF25D0AB)),
            ),
          ),
        ],
      ),
    );
  }
}

class _VideoTile extends StatelessWidget {
  const _VideoTile({required this.video});

  final VideoItem video;

  @override
  Widget build(BuildContext context) {
    return Material(
      color: const Color(0xFF171B24),
      borderRadius: BorderRadius.circular(8),
      child: InkWell(
        borderRadius: BorderRadius.circular(8),
        onTap: () {},
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
                      Image.network(
                        video.imageUrl,
                        fit: BoxFit.cover,
                        errorBuilder: (_, _, _) =>
                            const ColoredBox(color: Color(0xFF202532)),
                      ),
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
                            video.duration,
                            style: const TextStyle(
                              color: Colors.white,
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
                    Text(
                      video.title,
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: const TextStyle(
                        color: Colors.white,
                        fontWeight: FontWeight.w800,
                        fontSize: 16,
                      ),
                    ),
                    const SizedBox(height: 6),
                    Text(
                      video.description,
                      maxLines: 2,
                      overflow: TextOverflow.ellipsis,
                      style: const TextStyle(
                        color: Color(0xFF9CA3AF),
                        height: 1.35,
                      ),
                    ),
                    const SizedBox(height: 8),
                    Row(
                      children: [
                        const Icon(
                          Icons.star_rounded,
                          color: Color(0xFFF7C948),
                          size: 16,
                        ),
                        const SizedBox(width: 4),
                        Text(
                          video.score,
                          style: const TextStyle(
                            color: Color(0xFFE5E7EB),
                            fontSize: 12,
                          ),
                        ),
                        const SizedBox(width: 12),
                        const Icon(
                          Icons.visibility_rounded,
                          color: Color(0xFF25D0AB),
                          size: 15,
                        ),
                        const SizedBox(width: 4),
                        Expanded(
                          child: Text(
                            video.views,
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                            style: const TextStyle(
                              color: Color(0xFFE5E7EB),
                              fontSize: 12,
                            ),
                          ),
                        ),
                      ],
                    ),
                  ],
                ),
              ),
              IconButton(
                onPressed: () {},
                icon: const Icon(Icons.more_vert_rounded),
                color: const Color(0xFFD1D5DB),
                tooltip: '更多',
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _BottomNav extends StatelessWidget {
  const _BottomNav({required this.selectedIndex, required this.onSelected});

  final int selectedIndex;
  final ValueChanged<int> onSelected;

  @override
  Widget build(BuildContext context) {
    final items = [
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

class _SectionTitle extends StatelessWidget {
  const _SectionTitle({required this.title});

  final String title;

  @override
  Widget build(BuildContext context) {
    return Text(
      title,
      style: const TextStyle(
        color: Colors.white,
        fontSize: 18,
        fontWeight: FontWeight.w900,
      ),
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

class VideoItem {
  const VideoItem({
    required this.title,
    required this.description,
    required this.imageUrl,
    required this.duration,
    required this.score,
    required this.views,
    this.progress = 0,
  });

  final String title;
  final String description;
  final String imageUrl;
  final String duration;
  final String score;
  final String views;
  final double progress;
}

const _watching = [
  VideoItem(
    title: '边境日落 第 6 集',
    description: '悬疑剧集',
    imageUrl:
        'https://images.unsplash.com/photo-1500530855697-b586d89ba3ee?auto=format&fit=crop&w=800&q=80',
    duration: '42:18',
    score: '8.8',
    views: '126 万',
    progress: 0.62,
  ),
  VideoItem(
    title: '城市厨房',
    description: '纪录片',
    imageUrl:
        'https://images.unsplash.com/photo-1514933651103-005eec06c04b?auto=format&fit=crop&w=800&q=80',
    duration: '28:04',
    score: '9.1',
    views: '82 万',
    progress: 0.36,
  ),
  VideoItem(
    title: '星际漫游',
    description: '科幻电影',
    imageUrl:
        'https://images.unsplash.com/photo-1446776811953-b23d57bd21aa?auto=format&fit=crop&w=800&q=80',
    duration: '01:55:20',
    score: '8.6',
    views: '246 万',
    progress: 0.78,
  ),
];

const _videos = [
  VideoItem(
    title: '深海信号',
    description: '一支科考队在海底接收到来自未知文明的回声。',
    imageUrl:
        'https://images.unsplash.com/photo-1507525428034-b723cf961d3e?auto=format&fit=crop&w=800&q=80',
    duration: '01:44:12',
    score: '8.9',
    views: '320 万播放',
  ),
  VideoItem(
    title: '赛博雨夜',
    description: '退役探员重返街头，追查一段被删除的城市记忆。',
    imageUrl:
        'https://images.unsplash.com/photo-1519608487953-e999c86e7455?auto=format&fit=crop&w=800&q=80',
    duration: '01:58:36',
    score: '8.5',
    views: '198 万播放',
  ),
  VideoItem(
    title: '山谷来信',
    description: '治愈系公路片，在风景与旧友之间重新找回自己。',
    imageUrl:
        'https://images.unsplash.com/photo-1500534314209-a25ddb2bd429?auto=format&fit=crop&w=800&q=80',
    duration: '01:32:08',
    score: '9.0',
    views: '154 万播放',
  ),
  VideoItem(
    title: '午夜电台',
    description: '每晚零点，一个主持人收到听众寄来的秘密故事。',
    imageUrl:
        'https://images.unsplash.com/photo-1485579149621-3123dd979885?auto=format&fit=crop&w=800&q=80',
    duration: '52:40',
    score: '8.2',
    views: '76 万播放',
  ),
];
