import '../core/api_client.dart';

class Category {
  final int id;
  final String name;
  final int sortOrder;

  const Category({required this.id, required this.name, required this.sortOrder});

  factory Category.fromJson(Map<String, dynamic> json) => Category(
    id: json['id'] as int,
    name: json['name'] as String? ?? '',
    sortOrder: json['sort_order'] as int? ?? 0,
  );
}

class Video {
  final int id;
  final String title;
  final String description;
  final int categoryId;
  final String categoryName;
  final String coverUrl;
  final int duration;
  final String status;
  final bool isVip;
  final bool isFree;

  const Video({
    required this.id,
    required this.title,
    required this.description,
    required this.categoryId,
    required this.categoryName,
    required this.coverUrl,
    required this.duration,
    required this.status,
    required this.isVip,
    required this.isFree,
  });

  factory Video.fromJson(Map<String, dynamic> json) => Video(
    id: json['id'] as int,
    title: json['title'] as String? ?? '',
    description: json['description'] as String? ?? '',
    categoryId: json['category_id'] as int? ?? 0,
    categoryName: json['category_name'] as String? ?? '',
    coverUrl: json['cover_url'] as String? ?? '',
    duration: json['duration'] as int? ?? 0,
    status: json['status'] as String? ?? '',
    isVip: json['is_vip'] as bool? ?? false,
    isFree: json['is_free'] as bool? ?? true,
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
