import 'package:flutter/material.dart';

import '../../../models/video.dart';

/// A horizontally-scrolling, titled row of poster cards used on the home
/// landing page for recommendation rails (popular / latest / VIP picks).
class VideoRail extends StatelessWidget {
  const VideoRail({
    super.key,
    required this.title,
    required this.videos,
    required this.onOpenVideo,
    this.progress,
  });

  final String title;
  final List<Video> videos;
  final ValueChanged<Video> onOpenVideo;

  /// Optional per-video watch progress (0–100), keyed by video id. When set for
  /// a card, a thin progress bar is drawn at the bottom of its poster — used by
  /// the "继续观看" rail.
  final Map<int, int>? progress;

  @override
  Widget build(BuildContext context) {
    if (videos.isEmpty) return const SizedBox.shrink();
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Padding(
          padding: const EdgeInsets.fromLTRB(16, 18, 16, 10),
          child: Text(
            title,
            style: const TextStyle(
              color: Colors.white,
              fontSize: 16,
              fontWeight: FontWeight.w800,
            ),
          ),
        ),
        SizedBox(
          height: 158,
          child: ListView.separated(
            scrollDirection: Axis.horizontal,
            padding: const EdgeInsets.symmetric(horizontal: 16),
            itemCount: videos.length,
            separatorBuilder: (_, _) => const SizedBox(width: 12),
            itemBuilder: (context, index) => _PosterCard(
              video: videos[index],
              onTap: onOpenVideo,
              progress: progress?[videos[index].id],
            ),
          ),
        ),
      ],
    );
  }
}

class _PosterCard extends StatelessWidget {
  const _PosterCard({required this.video, required this.onTap, this.progress});

  final Video video;
  final ValueChanged<Video> onTap;
  final int? progress;

  @override
  Widget build(BuildContext context) {
    final showVip = video.isVip && !video.isFree;
    return GestureDetector(
      onTap: video.isReady ? () => onTap(video) : null,
      child: SizedBox(
        width: 168,
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            ClipRRect(
              borderRadius: BorderRadius.circular(8),
              child: SizedBox(
                width: 168,
                height: 100,
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
                    if (showVip)
                      Positioned(
                        left: 6,
                        top: 6,
                        child: Container(
                          padding: const EdgeInsets.symmetric(
                            horizontal: 6,
                            vertical: 2,
                          ),
                          decoration: BoxDecoration(
                            color: const Color(0xFFFFC857),
                            borderRadius: BorderRadius.circular(4),
                          ),
                          child: const Text(
                            'VIP',
                            style: TextStyle(
                              color: Colors.black,
                              fontSize: 10,
                              fontWeight: FontWeight.w800,
                            ),
                          ),
                        ),
                      ),
                    if (video.durationLabel.isNotEmpty)
                      Positioned(
                        right: 6,
                        bottom: 6,
                        child: Container(
                          padding: const EdgeInsets.symmetric(
                            horizontal: 6,
                            vertical: 2,
                          ),
                          decoration: BoxDecoration(
                            color: const Color(0xCC000000),
                            borderRadius: BorderRadius.circular(4),
                          ),
                          child: Text(
                            video.durationLabel,
                            style: const TextStyle(
                              color: Colors.white,
                              fontSize: 10,
                            ),
                          ),
                        ),
                      ),
                    if (progress != null && progress! > 0)
                      Positioned(
                        left: 0,
                        right: 0,
                        bottom: 0,
                        child: ClipRRect(
                          borderRadius: const BorderRadius.vertical(
                            bottom: Radius.circular(8),
                          ),
                          child: LinearProgressIndicator(
                            value: progress!.clamp(0, 100) / 100,
                            minHeight: 3,
                            backgroundColor: const Color(0x66000000),
                            valueColor: const AlwaysStoppedAnimation<Color>(
                              Color(0xFF25D0AB),
                            ),
                          ),
                        ),
                      ),
                  ],
                ),
              ),
            ),
            const SizedBox(height: 6),
            Text(
              video.title,
              maxLines: 2,
              overflow: TextOverflow.ellipsis,
              style: const TextStyle(
                color: Colors.white,
                fontSize: 13,
                height: 1.25,
              ),
            ),
          ],
        ),
      ),
    );
  }
}
