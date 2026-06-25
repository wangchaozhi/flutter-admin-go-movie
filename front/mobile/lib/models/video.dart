import '../core/api_client.dart';

class Category {
  final int id;
  final String name;
  final int sortOrder;

  const Category({
    required this.id,
    required this.name,
    required this.sortOrder,
  });

  factory Category.fromJson(Map<String, dynamic> json) => Category(
    id: json['id'] as int,
    name: json['name'] as String? ?? '',
    sortOrder: json['sort_order'] as int? ?? 0,
  );
}

class VideoAIMetadata {
  final String synopsis;
  final List<String> highlights;
  final List<String> tags;
  final String provider;
  final String model;

  const VideoAIMetadata({
    required this.synopsis,
    required this.highlights,
    required this.tags,
    required this.provider,
    required this.model,
  });

  factory VideoAIMetadata.fromJson(Map<String, dynamic> json) =>
      VideoAIMetadata(
        synopsis: json['synopsis'] as String? ?? '',
        highlights: _stringList(json['highlights']),
        tags: _stringList(json['tags']),
        provider: json['provider'] as String? ?? '',
        model: json['model'] as String? ?? '',
      );

  static List<String> _stringList(dynamic value) {
    if (value is! List) return const [];
    return value
        .whereType<String>()
        .map((item) => item.trim())
        .where((item) => item.isNotEmpty)
        .toList(growable: false);
  }

  bool get hasContent =>
      synopsis.trim().isNotEmpty || highlights.isNotEmpty || tags.isNotEmpty;
}

class Video {
  final int id;
  final String title;
  final String description;
  final int categoryId;
  final String categoryName;
  final List<String> actors;
  final List<String> directors;
  final List<String> genres;
  final String region;
  final int releaseYear;
  final String language;
  final String coverUrl;
  final int duration;
  final String status;
  final bool isVip;
  final bool isFree;
  final VideoAIMetadata? aiMetadata;

  const Video({
    required this.id,
    required this.title,
    required this.description,
    required this.categoryId,
    required this.categoryName,
    required this.actors,
    required this.directors,
    required this.genres,
    required this.region,
    required this.releaseYear,
    required this.language,
    required this.coverUrl,
    required this.duration,
    required this.status,
    required this.isVip,
    required this.isFree,
    this.aiMetadata,
  });

  factory Video.fromJson(Map<String, dynamic> json) => Video(
    id: json['id'] as int,
    title: json['title'] as String? ?? '',
    description: json['description'] as String? ?? '',
    categoryId: json['category_id'] as int? ?? 0,
    categoryName: json['category_name'] as String? ?? '',
    actors: VideoAIMetadata._stringList(json['actors']),
    directors: VideoAIMetadata._stringList(json['directors']),
    genres: VideoAIMetadata._stringList(json['genres']),
    region: json['region'] as String? ?? '',
    releaseYear: json['release_year'] as int? ?? 0,
    language: json['language'] as String? ?? '',
    coverUrl: json['cover_url'] as String? ?? '',
    duration: json['duration'] as int? ?? 0,
    status: json['status'] as String? ?? '',
    isVip: json['is_vip'] as bool? ?? false,
    isFree: json['is_free'] as bool? ?? true,
    aiMetadata: json['ai_metadata'] is Map<String, dynamic>
        ? VideoAIMetadata.fromJson(json['ai_metadata'] as Map<String, dynamic>)
        : null,
  );

  String get fullCoverUrl =>
      coverUrl.isNotEmpty ? '${ApiClient.baseUrl}$coverUrl' : '';

  String get durationLabel {
    if (duration <= 0) return '';
    final m = duration ~/ 60;
    final s = duration % 60;
    if (m >= 60) {
      return '${m ~/ 60}:${(m % 60).toString().padLeft(2, '0')}:${s.toString().padLeft(2, '0')}';
    }
    return '$m:${s.toString().padLeft(2, '0')}';
  }

  bool get isReady => status == 'ready';
}
