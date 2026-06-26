import 'package:flutter/material.dart';

import 'series_models.dart';

/// A horizontally-scrolling row of series poster cards for the home page. Each
/// card shows the cover, title and episode count, and opens the series detail.
class SeriesRail extends StatelessWidget {
  const SeriesRail({
    super.key,
    required this.title,
    required this.series,
    required this.onOpenSeries,
  });

  final String title;
  final List<Series> series;
  final ValueChanged<Series> onOpenSeries;

  @override
  Widget build(BuildContext context) {
    if (series.isEmpty) return const SizedBox.shrink();
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Padding(
          padding: const EdgeInsets.fromLTRB(16, 20, 16, 10),
          child: Text(
            title,
            style: const TextStyle(
              color: Colors.white,
              fontSize: 17,
              fontWeight: FontWeight.w900,
            ),
          ),
        ),
        SizedBox(
          height: 196,
          child: ListView.separated(
            scrollDirection: Axis.horizontal,
            padding: const EdgeInsets.symmetric(horizontal: 16),
            itemCount: series.length,
            separatorBuilder: (_, _) => const SizedBox(width: 12),
            itemBuilder: (context, index) =>
                _SeriesCard(series: series[index], onTap: onOpenSeries),
          ),
        ),
      ],
    );
  }
}

class _SeriesCard extends StatelessWidget {
  const _SeriesCard({required this.series, required this.onTap});

  final Series series;
  final ValueChanged<Series> onTap;

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: () => onTap(series),
      child: SizedBox(
        width: 116,
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            ClipRRect(
              borderRadius: BorderRadius.circular(8),
              child: SizedBox(
                width: 116,
                height: 152,
                child: Stack(
                  fit: StackFit.expand,
                  children: [
                    series.fullCoverUrl.isNotEmpty
                        ? Image.network(
                            series.fullCoverUrl,
                            fit: BoxFit.cover,
                            errorBuilder: (_, _, _) =>
                                const ColoredBox(color: Color(0xFF202532)),
                          )
                        : const ColoredBox(color: Color(0xFF202532)),
                    if (series.isVip)
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
                          '${series.episodeCount} 集',
                          style: const TextStyle(
                            color: Colors.white,
                            fontSize: 10,
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
              series.title,
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: const TextStyle(
                color: Colors.white,
                fontSize: 13,
                height: 1.3,
                fontWeight: FontWeight.w700,
              ),
            ),
          ],
        ),
      ),
    );
  }
}
