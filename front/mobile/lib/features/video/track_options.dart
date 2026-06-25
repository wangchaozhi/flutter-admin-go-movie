import 'package:flutter/foundation.dart' show kIsWeb;
import 'package:media_kit/media_kit.dart';

/// A selectable playback quality (an HLS variant or a backend-provided rendition).
class QualityOption {
  final String name;
  final String label;
  final String url;

  const QualityOption({
    required this.name,
    required this.label,
    required this.url,
  });
}

/// An audio or subtitle track described by the backend's `/play` payload.
class MediaTrackOption {
  final int id;
  final String label;
  final String url;
  final String language;
  final String title;
  final String codec;
  final bool isDefault;
  final bool isForced;

  /// Position of this track within its type in the source stream order. On web
  /// this matches the hls.js audio-rendition index used to switch tracks.
  final int streamPosition;

  const MediaTrackOption({
    required this.id,
    required this.label,
    required this.url,
    required this.language,
    required this.title,
    required this.codec,
    required this.isDefault,
    required this.isForced,
    required this.streamPosition,
  });
}

/// A single entry in the audio/subtitle menu. Exactly one of [audioTrack],
/// [subtitleTrack] or [apiTrack] is set, identifying how a selection is applied.
class TrackMenuOption {
  final String value;
  final String label;
  final AudioTrack? audioTrack;
  final SubtitleTrack? subtitleTrack;
  final MediaTrackOption? apiTrack;

  const TrackMenuOption({
    required this.value,
    required this.label,
    this.audioTrack,
    this.subtitleTrack,
    this.apiTrack,
  });
}

/// Pure derivation of the audio/subtitle menus from the available tracks.
///
/// All inputs are explicit (including [isWeb]) so the web/native/scanned
/// branching can be exercised in unit tests without a running player.
class TrackMenuBuilder {
  const TrackMenuBuilder._();

  /// Player-detected HLS audio tracks worth offering: real, non-sentinel,
  /// non-external tracks, and only when there is more than one to choose from.
  /// Always empty on web, where switching goes through the backend renditions.
  static List<AudioTrack> selectableHlsAudio(
    List<AudioTrack> tracks, {
    bool isWeb = kIsWeb,
  }) {
    if (isWeb) return const [];
    final selectable = tracks
        .where(
          (track) =>
              track.id != 'auto' &&
              track.id != 'no' &&
              track.id.trim().isNotEmpty &&
              !track.uri,
        )
        .toList();
    return selectable.length > 1 ? selectable : const [];
  }

  /// Player-detected HLS subtitle tracks worth offering: real, non-sentinel,
  /// non-external, non-inline tracks.
  static List<SubtitleTrack> selectableHlsSubtitle(List<SubtitleTrack> tracks) {
    return tracks
        .where(
          (track) =>
              track.id != 'auto' &&
              track.id != 'no' &&
              track.id.trim().isNotEmpty &&
              !track.uri &&
              !track.data,
        )
        .toList();
  }

  /// Whether [value] is null (the default) or still present in [options].
  static bool valueExists(String? value, List<TrackMenuOption> options) {
    return value == null || options.any((option) => option.value == value);
  }

  /// Prefers a track's title, then its language, then [fallback]. Takes a
  /// `dynamic` track because [AudioTrack] and [SubtitleTrack] share the shape
  /// but no common supertype.
  static String trackLabel(dynamic track, String fallback) {
    final title = track.title?.toString().trim() ?? '';
    if (title.isNotEmpty) return title;
    final language = track.language?.toString().trim() ?? '';
    if (language.isNotEmpty) return language;
    return fallback;
  }

  static List<TrackMenuOption> apiOptions(List<MediaTrackOption> apiTracks) {
    return [
      for (final track in apiTracks)
        TrackMenuOption(
          value: 'api:${track.id}',
          label: track.label,
          apiTrack: track,
        ),
    ];
  }

  static List<TrackMenuOption> audioOptions({
    required List<AudioTrack> hlsTracks,
    required List<MediaTrackOption> apiTracks,
    required bool mediaTracksScanned,
    required bool hasMultipleAudioTracks,
    bool isWeb = kIsWeb,
  }) {
    if (mediaTracksScanned && !hasMultipleAudioTracks) {
      return const [];
    }
    if (isWeb) {
      return apiOptions(apiTracks);
    }
    if (hlsTracks.isNotEmpty) {
      return [
        for (var i = 0; i < hlsTracks.length; i++)
          TrackMenuOption(
            value: 'hls:${hlsTracks[i].id}',
            label: trackLabel(hlsTracks[i], 'Audio ${i + 1}'),
            audioTrack: hlsTracks[i],
          ),
      ];
    }
    return apiOptions(apiTracks);
  }

  static List<TrackMenuOption> subtitleOptions({
    required List<SubtitleTrack> hlsTracks,
    required List<MediaTrackOption> apiTracks,
    required bool mediaTracksScanned,
    required bool hasSubtitleTracks,
    bool isWeb = kIsWeb,
  }) {
    if (mediaTracksScanned && !hasSubtitleTracks) {
      return const [];
    }
    if (isWeb) {
      return apiOptions(apiTracks);
    }
    if (hlsTracks.isNotEmpty) {
      return [
        for (var i = 0; i < hlsTracks.length; i++)
          TrackMenuOption(
            value: 'hls:${hlsTracks[i].id}',
            label: trackLabel(hlsTracks[i], 'Subtitle ${i + 1}'),
            subtitleTrack: hlsTracks[i],
          ),
      ];
    }
    return apiOptions(apiTracks);
  }
}

/// Pure parsing of the backend `/play` payload into the track models. URL
/// resolution is delegated to [absoluteUrl] so this stays free of app config.
class TrackParser {
  const TrackParser._();

  static int parseInt(dynamic value) {
    if (value is num) return value.toInt();
    return int.tryParse(value?.toString() ?? '') ?? 0;
  }

  static List<QualityOption> parseQualities(
    Map<String, dynamic>? data,
    String Function(String) absoluteUrl,
  ) {
    final rawQualities = data?['qualities'];
    if (rawQualities is! List) return const [];
    return rawQualities
        .whereType<Map<String, dynamic>>()
        .map((item) {
          final name = item['name']?.toString() ?? '';
          final label = item['label']?.toString() ?? name;
          final url = item['url']?.toString() ?? '';
          if (name.isEmpty || url.isEmpty) return null;
          return QualityOption(
            name: name,
            label: label.isEmpty ? name : label,
            url: absoluteUrl(url),
          );
        })
        .whereType<QualityOption>()
        .toList();
  }

  static List<MediaTrackOption> parseMediaTracks(
    Map<String, dynamic>? data,
    String key,
    String Function(String) absoluteUrl,
  ) {
    final rawTracks = data?[key];
    if (rawTracks is! List) return const [];
    return rawTracks
        .whereType<Map<String, dynamic>>()
        .map((item) {
          final idValue = item['id'];
          final id = idValue is num
              ? idValue.toInt()
              : int.tryParse(idValue?.toString() ?? '');
          final label = item['label']?.toString() ?? '';
          final url = item['url']?.toString() ?? '';
          if (id == null || url.isEmpty) return null;
          return MediaTrackOption(
            id: id,
            label: label.isEmpty ? 'Track ${id.toString()}' : label,
            url: absoluteUrl(url),
            language: item['language']?.toString() ?? '',
            title: item['title']?.toString() ?? '',
            codec: item['codec']?.toString() ?? '',
            isDefault: item['default'] == true,
            isForced: item['forced'] == true,
            streamPosition: parseInt(item['stream_position']),
          );
        })
        .whereType<MediaTrackOption>()
        .toList();
  }
}
