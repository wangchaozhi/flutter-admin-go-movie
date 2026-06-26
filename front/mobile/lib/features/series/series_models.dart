import '../../core/api_client.dart';

/// A series (TV show, anime, variety) shown as a poster tile and opened into a
/// detail page with an episode selector.
class Series {
  final int id;
  final String title;
  final String description;
  final String coverUrl;
  final int categoryId;
  final String categoryName;
  final String region;
  final int releaseYear;
  final List<String> genres;
  final bool isVip;
  final String status;
  final int episodeCount;

  const Series({
    required this.id,
    required this.title,
    required this.description,
    required this.coverUrl,
    required this.categoryId,
    required this.categoryName,
    required this.region,
    required this.releaseYear,
    required this.genres,
    required this.isVip,
    required this.status,
    required this.episodeCount,
  });

  factory Series.fromJson(Map<String, dynamic> json) => Series(
    id: json['id'] as int,
    title: json['title'] as String? ?? '',
    description: json['description'] as String? ?? '',
    coverUrl: json['cover_url'] as String? ?? '',
    categoryId: json['category_id'] as int? ?? 0,
    categoryName: json['category_name'] as String? ?? '',
    region: json['region'] as String? ?? '',
    releaseYear: json['release_year'] as int? ?? 0,
    genres: (json['genres'] as List<dynamic>? ?? [])
        .whereType<String>()
        .map((e) => e.trim())
        .where((e) => e.isNotEmpty)
        .toList(growable: false),
    isVip: json['is_vip'] as bool? ?? false,
    status: json['status'] as String? ?? 'ongoing',
    episodeCount: (json['episode_count'] as num?)?.toInt() ?? 0,
  );

  String get fullCoverUrl =>
      coverUrl.isNotEmpty ? '${ApiClient.baseUrl}$coverUrl' : '';

  String get statusLabel {
    switch (status) {
      case 'completed':
        return '已完结';
      case 'offline':
        return '已下架';
      default:
        return '连载中';
    }
  }
}

/// One episode within a series — a trimmed view of a video row.
class SeriesEpisode {
  final int id;
  final String title;
  final int episodeNumber;
  final int duration;
  final String status;
  final bool isVip;
  final bool isFree;
  final String coverUrl;

  const SeriesEpisode({
    required this.id,
    required this.title,
    required this.episodeNumber,
    required this.duration,
    required this.status,
    required this.isVip,
    required this.isFree,
    required this.coverUrl,
  });

  factory SeriesEpisode.fromJson(Map<String, dynamic> json) => SeriesEpisode(
    id: json['id'] as int,
    title: json['title'] as String? ?? '',
    episodeNumber: (json['episode_number'] as num?)?.toInt() ?? 0,
    duration: (json['duration'] as num?)?.toInt() ?? 0,
    status: json['status'] as String? ?? '',
    isVip: json['is_vip'] as bool? ?? false,
    isFree: json['is_free'] as bool? ?? true,
    coverUrl: json['cover_url'] as String? ?? '',
  );

  bool get isReady => status == 'ready';
}
