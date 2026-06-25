import 'package:flutter_test/flutter_test.dart';
import 'package:mobile/features/video/playback_parsers.dart';

void main() {
  group('WebVttParser.parseTimestamp', () {
    test('parses HH:MM:SS.mmm', () {
      expect(
        WebVttParser.parseTimestamp('01:02:03.250'),
        const Duration(hours: 1, minutes: 2, seconds: 3, milliseconds: 250),
      );
    });

    test('parses MM:SS.mmm (no hours)', () {
      expect(
        WebVttParser.parseTimestamp('00:05.500'),
        const Duration(seconds: 5, milliseconds: 500),
      );
    });

    test('accepts comma as the fraction separator (SRT style)', () {
      expect(
        WebVttParser.parseTimestamp('00:00:01,200'),
        const Duration(seconds: 1, milliseconds: 200),
      );
    });

    test('pads / truncates fractional digits to milliseconds', () {
      expect(
        WebVttParser.parseTimestamp('00:00.5'),
        const Duration(milliseconds: 500),
      );
      expect(
        WebVttParser.parseTimestamp('00:00.123456'),
        const Duration(milliseconds: 123),
      );
    });

    test('rejects malformed input', () {
      expect(WebVttParser.parseTimestamp(''), isNull);
      expect(WebVttParser.parseTimestamp('12'), isNull);
      expect(WebVttParser.parseTimestamp('1:2:3:4'), isNull);
      expect(WebVttParser.parseTimestamp('ab:cd.ef'), isNull);
    });
  });

  group('WebVttParser.parse', () {
    test('parses cues, strips header/NOTE/tags and entities, and sorts', () {
      const vtt = '\u{FEFF}WEBVTT\n'
          '\n'
          'NOTE this is a comment\n'
          '\n'
          '2\n'
          '00:00:04.000 --> 00:00:06.000\n'
          'second cue\n'
          '\n'
          '1\n'
          '00:00:01.000 --> 00:00:02.500 line:90%\n'
          '<b>hello</b> &amp; <i>world</i>\n';

      final cues = WebVttParser.parse(vtt);

      expect(cues.length, 2);
      // Sorted by start time regardless of source order.
      expect(cues.first.start, const Duration(seconds: 1));
      expect(cues.first.end, const Duration(seconds: 2, milliseconds: 500));
      // Tags removed, entity decoded.
      expect(cues.first.text, 'hello & world');
      expect(cues[1].start, const Duration(seconds: 4));
      expect(cues[1].text, 'second cue');
    });

    test('handles CRLF line endings', () {
      const vtt = 'WEBVTT\r\n\r\n00:00:00.000 --> 00:00:01.000\r\nhi\r\n';
      final cues = WebVttParser.parse(vtt);
      expect(cues, hasLength(1));
      expect(cues.single.text, 'hi');
    });

    test('drops cues with non-positive duration', () {
      const vtt = 'WEBVTT\n\n00:00:02.000 --> 00:00:02.000\nzero\n';
      expect(WebVttParser.parse(vtt), isEmpty);
    });

    test('joins multi-line cue text', () {
      const vtt = 'WEBVTT\n\n00:00:00.000 --> 00:00:01.000\nline one\nline two\n';
      expect(WebVttParser.parse(vtt).single.text, 'line one\nline two');
    });
  });

  group('HlsMasterParser.parseResolution', () {
    test('extracts the RESOLUTION attribute', () {
      expect(
        HlsMasterParser.parseResolution(
          '#EXT-X-STREAM-INF:BANDWIDTH=800000,RESOLUTION=1280x720',
        ),
        '1280x720',
      );
    });

    test('returns empty when absent', () {
      expect(
        HlsMasterParser.parseResolution('#EXT-X-STREAM-INF:BANDWIDTH=800000'),
        '',
      );
    });
  });

  group('HlsMasterParser.qualityNameFromUri', () {
    test('takes the segment before index.m3u8', () {
      expect(
        HlsMasterParser.qualityNameFromUri('https://h/v/720p/index.m3u8'),
        '720p',
      );
    });

    test('returns empty for non index.m3u8 paths', () {
      expect(HlsMasterParser.qualityNameFromUri('https://h/v/720p.m3u8'), '');
    });
  });

  group('HlsMasterParser.parse', () {
    test('resolves relative variant URIs against the master URI', () {
      const master = '#EXTM3U\n'
          '#EXT-X-STREAM-INF:BANDWIDTH=2000000,RESOLUTION=1920x1080\n'
          '1080p/index.m3u8\n'
          '#EXT-X-STREAM-INF:BANDWIDTH=800000,RESOLUTION=1280x720\n'
          '720p/index.m3u8\n';
      final masterUri = Uri.parse('https://cdn.example.com/movie/master.m3u8');

      final variants = HlsMasterParser.parse(master, masterUri);

      expect(variants, hasLength(2));
      expect(variants[0].name, '1080p');
      expect(variants[0].label, '1080p');
      expect(
        variants[0].url,
        'https://cdn.example.com/movie/1080p/index.m3u8',
      );
      expect(variants[1].name, '720p');
      expect(
        variants[1].url,
        'https://cdn.example.com/movie/720p/index.m3u8',
      );
    });

    test('skips variants whose URI is not a recognizable quality', () {
      const master = '#EXTM3U\n'
          '#EXT-X-STREAM-INF:BANDWIDTH=800000,RESOLUTION=1280x720\n'
          'audio_only.m3u8\n';
      final variants = HlsMasterParser.parse(
        master,
        Uri.parse('https://cdn.example.com/movie/master.m3u8'),
      );
      expect(variants, isEmpty);
    });
  });
}
