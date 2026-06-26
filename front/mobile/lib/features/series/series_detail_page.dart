import 'package:flutter/material.dart';

import '../../core/api_client.dart';
import '../../core/l10n/app_strings.dart';
import '../../models/video.dart';
import 'series_models.dart';

const _accent = Color(0xFF25D0AB);
const _muted = Color(0xFF9CA3AF);

/// Series detail: header (cover + meta + synopsis) and an episode grid. Tapping
/// an episode launches the existing player by passing a minimal [Video] built
/// from the episode (the player re-fetches full detail on open).
class SeriesDetailPage extends StatefulWidget {
  const SeriesDetailPage({super.key, required this.seriesId, this.initial});

  final int seriesId;
  final Series? initial;

  @override
  State<SeriesDetailPage> createState() => _SeriesDetailPageState();
}

class _SeriesDetailPageState extends State<SeriesDetailPage> {
  Series? _series;
  List<SeriesEpisode> _episodes = const [];
  bool _loading = true;
  String? _error;

  @override
  void initState() {
    super.initState();
    _series = widget.initial;
    _load();
  }

  Future<void> _load() async {
    if (mounted) setState(() => _loading = true);
    try {
      final resp = await ApiClient().get('/api/series/${widget.seriesId}');
      if (!mounted) return;
      if (resp['code'] == 0) {
        final data = resp['data'] as Map<String, dynamic>? ?? {};
        final seriesJson = data['series'] as Map<String, dynamic>?;
        final episodes = (data['episodes'] as List<dynamic>? ?? [])
            .map((e) => SeriesEpisode.fromJson(e as Map<String, dynamic>))
            .where((e) => e.isReady)
            .toList();
        setState(() {
          if (seriesJson != null) _series = Series.fromJson(seriesJson);
          _episodes = episodes;
          _loading = false;
        });
      } else {
        setState(() {
          _error = resp['msg']?.toString() ?? '加载失败';
          _loading = false;
        });
      }
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _error = e.toString();
        _loading = false;
      });
    }
  }

  void _playEpisode(SeriesEpisode ep) {
    final video = Video(
      id: ep.id,
      title: _series == null
          ? ep.title
          : '${_series!.title} · 第${ep.episodeNumber}集',
      description: '',
      categoryId: _series?.categoryId ?? 0,
      categoryName: _series?.categoryName ?? '',
      actors: const [],
      directors: const [],
      genres: const [],
      region: _series?.region ?? '',
      releaseYear: _series?.releaseYear ?? 0,
      language: '',
      coverUrl: ep.coverUrl,
      duration: ep.duration,
      status: ep.status,
      isVip: ep.isVip,
      isFree: ep.isFree,
    );
    Navigator.pushNamed(context, '/player', arguments: video);
  }

  @override
  Widget build(BuildContext context) {
    final s = AppStrings.of(context);
    return Scaffold(
      backgroundColor: const Color(0xFF0D0F14),
      appBar: AppBar(
        backgroundColor: const Color(0xFF0D0F14),
        foregroundColor: Colors.white,
        title: Text(_series?.title ?? s.t('home.series')),
      ),
      body: _loading
          ? const Center(child: CircularProgressIndicator(color: _accent))
          : _error != null
          ? Center(
              child: Text(_error!, style: const TextStyle(color: _muted)),
            )
          : RefreshIndicator(
              color: _accent,
              onRefresh: _load,
              child: ListView(
                padding: const EdgeInsets.all(16),
                children: [
                  if (_series != null) _buildHeader(_series!),
                  const SizedBox(height: 20),
                  Text(
                    s.t('series.episodes'),
                    style: const TextStyle(
                      color: Colors.white,
                      fontSize: 16,
                      fontWeight: FontWeight.w900,
                    ),
                  ),
                  const SizedBox(height: 12),
                  if (_episodes.isEmpty)
                    Padding(
                      padding: const EdgeInsets.symmetric(vertical: 24),
                      child: Center(
                        child: Text(
                          s.t('series.empty'),
                          style: const TextStyle(color: _muted),
                        ),
                      ),
                    )
                  else
                    _buildEpisodeGrid(s),
                ],
              ),
            ),
    );
  }

  Widget _buildHeader(Series series) {
    final meta = <String>[
      if (series.region.isNotEmpty) series.region,
      if (series.releaseYear > 0) series.releaseYear.toString(),
      if (series.genres.isNotEmpty) series.genres.take(3).join(' / '),
      series.statusLabel,
    ];
    return Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        ClipRRect(
          borderRadius: BorderRadius.circular(8),
          child: SizedBox(
            width: 108,
            height: 152,
            child: series.fullCoverUrl.isNotEmpty
                ? Image.network(
                    series.fullCoverUrl,
                    fit: BoxFit.cover,
                    errorBuilder: (_, _, _) =>
                        const ColoredBox(color: Color(0xFF202532)),
                  )
                : const ColoredBox(color: Color(0xFF202532)),
          ),
        ),
        const SizedBox(width: 14),
        Expanded(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                series.title,
                style: const TextStyle(
                  color: Colors.white,
                  fontSize: 18,
                  fontWeight: FontWeight.w900,
                ),
              ),
              const SizedBox(height: 8),
              Wrap(
                spacing: 8,
                runSpacing: 6,
                children: [
                  if (series.isVip)
                    Container(
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
                  Text(
                    '${series.episodeCount} 集',
                    style: const TextStyle(color: _muted, fontSize: 12),
                  ),
                ],
              ),
              const SizedBox(height: 8),
              Text(
                meta.join(' · '),
                style: const TextStyle(color: _muted, fontSize: 12, height: 1.4),
              ),
              if (series.description.trim().isNotEmpty) ...[
                const SizedBox(height: 10),
                Text(
                  series.description.trim(),
                  maxLines: 5,
                  overflow: TextOverflow.ellipsis,
                  style: const TextStyle(
                    color: Color(0xFFD1D5DB),
                    fontSize: 13,
                    height: 1.5,
                  ),
                ),
              ],
            ],
          ),
        ),
      ],
    );
  }

  Widget _buildEpisodeGrid(AppStrings s) {
    return GridView.builder(
      shrinkWrap: true,
      physics: const NeverScrollableScrollPhysics(),
      gridDelegate: const SliverGridDelegateWithFixedCrossAxisCount(
        crossAxisCount: 5,
        mainAxisSpacing: 10,
        crossAxisSpacing: 10,
        childAspectRatio: 1.4,
      ),
      itemCount: _episodes.length,
      itemBuilder: (context, index) {
        final ep = _episodes[index];
        return GestureDetector(
          onTap: () => _playEpisode(ep),
          child: DecoratedBox(
            decoration: BoxDecoration(
              color: const Color(0xFF171B24),
              borderRadius: BorderRadius.circular(8),
              border: Border.all(color: const Color(0xFF2B3140)),
            ),
            child: Stack(
              children: [
                Center(
                  child: Text(
                    ep.episodeNumber > 0 ? '${ep.episodeNumber}' : '${index + 1}',
                    style: const TextStyle(
                      color: Colors.white,
                      fontSize: 16,
                      fontWeight: FontWeight.w800,
                    ),
                  ),
                ),
                if (ep.isVip && !ep.isFree)
                  const Positioned(
                    right: 4,
                    top: 4,
                    child: Icon(
                      Icons.workspace_premium_rounded,
                      size: 13,
                      color: Color(0xFFFFC857),
                    ),
                  ),
              ],
            ),
          ),
        );
      },
    );
  }
}
