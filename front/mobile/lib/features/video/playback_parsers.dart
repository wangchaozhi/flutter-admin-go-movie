/// Pure parsing helpers for the video player, kept free of Flutter/network
/// dependencies so they can be unit tested in isolation.
library;

/// A single subtitle cue with its on-screen window.
class SubtitleCue {
  final Duration start;
  final Duration end;
  final String text;

  const SubtitleCue({
    required this.start,
    required this.end,
    required this.text,
  });
}

/// Parses WebVTT subtitle payloads into ordered [SubtitleCue]s.
class WebVttParser {
  const WebVttParser._();

  static List<SubtitleCue> parse(String body) {
    final normalized = body
        .replaceFirst('\uFEFF', '')
        .replaceAll('\r\n', '\n')
        .replaceAll('\r', '\n');
    final cues = <SubtitleCue>[];
    for (final block in normalized.split(RegExp(r'\n{2,}'))) {
      final lines = block
          .split('\n')
          .map((line) => line.trim())
          .where((line) => line.isNotEmpty)
          .toList();
      if (lines.isEmpty || lines.first.startsWith('WEBVTT')) continue;
      if (lines.first.startsWith('NOTE') ||
          lines.first == 'STYLE' ||
          lines.first == 'REGION') {
        continue;
      }

      final timingIndex = lines.indexWhere((line) => line.contains('-->'));
      if (timingIndex < 0) continue;
      final timingParts = lines[timingIndex].split('-->');
      if (timingParts.length < 2) continue;

      final start = parseTimestamp(timingParts[0].trim());
      final endToken = timingParts[1].trim().split(RegExp(r'\s+')).first;
      final end = parseTimestamp(endToken);
      if (start == null || end == null || end <= start) continue;

      final text = lines
          .skip(timingIndex + 1)
          .map(_cleanText)
          .where((line) => line.isNotEmpty)
          .join('\n');
      if (text.isEmpty) continue;
      cues.add(SubtitleCue(start: start, end: end, text: text));
    }
    cues.sort((a, b) => a.start.compareTo(b.start));
    return cues;
  }

  /// Parses a `HH:MM:SS.mmm` / `MM:SS.mmm` (comma or dot fraction) timestamp.
  static Duration? parseTimestamp(String value) {
    final parts = value.replaceAll(',', '.').split(':');
    if (parts.length < 2 || parts.length > 3) return null;
    final secondsPart = parts.last.split('.');
    final seconds = int.tryParse(secondsPart.first);
    if (seconds == null) return null;
    var milliseconds = 0;
    if (secondsPart.length > 1) {
      final fraction = secondsPart[1].padRight(3, '0').substring(0, 3);
      milliseconds = int.tryParse(fraction) ?? 0;
    }
    final minutes = int.tryParse(parts[parts.length - 2]);
    if (minutes == null) return null;
    final hours = parts.length == 3 ? int.tryParse(parts.first) : 0;
    if (hours == null) return null;
    return Duration(
      hours: hours,
      minutes: minutes,
      seconds: seconds,
      milliseconds: milliseconds,
    );
  }

  static String _cleanText(String value) {
    return value
        .replaceAll(RegExp(r'<[^>]+>'), '')
        .replaceAll('&amp;', '&')
        .replaceAll('&lt;', '<')
        .replaceAll('&gt;', '>')
        .replaceAll('&nbsp;', ' ')
        .trim();
  }
}

/// A quality variant discovered in an HLS master playlist. [url] is resolved
/// against the master playlist URI, so it is absolute.
class HlsVariant {
  final String name;
  final String label;
  final String url;

  const HlsVariant({
    required this.name,
    required this.label,
    required this.url,
  });
}

/// Parses the `#EXT-X-STREAM-INF` variants out of an HLS master playlist.
class HlsMasterParser {
  const HlsMasterParser._();

  static List<HlsVariant> parse(String body, Uri masterUri) {
    final variants = <HlsVariant>[];
    String resolution = '';
    for (final rawLine in body.split('\n')) {
      final line = rawLine.trim();
      if (line.startsWith('#EXT-X-STREAM-INF:')) {
        resolution = parseResolution(line);
        continue;
      }
      final lineUri = Uri.tryParse(line);
      final path = lineUri?.path ?? line;
      if (line.startsWith('#') || !path.endsWith('.m3u8')) {
        continue;
      }
      final resolved = masterUri.resolve(line).toString();
      final name = qualityNameFromUri(resolved);
      if (name.isEmpty) {
        resolution = '';
        continue;
      }
      variants.add(
        HlsVariant(
          name: name,
          label: qualityLabel(name, resolution),
          url: resolved,
        ),
      );
      resolution = '';
    }
    return variants;
  }

  static String parseResolution(String line) {
    for (final part in line.split(',')) {
      final value = part.trim();
      if (value.startsWith('RESOLUTION=')) {
        return value.substring('RESOLUTION='.length);
      }
    }
    return '';
  }

  static String qualityNameFromUri(String uri) {
    final segments = Uri.parse(uri).pathSegments;
    if (segments.length >= 2 && segments.last == 'index.m3u8') {
      return segments[segments.length - 2];
    }
    return '';
  }

  static String qualityLabel(String name, String resolution) {
    if (name.isNotEmpty) return name;
    final parts = resolution.split('x');
    if (parts.length == 2 && parts[1].isNotEmpty) {
      return '${parts[1]}p';
    }
    return resolution.isNotEmpty ? resolution : '清晰度';
  }
}
