import 'package:flutter_test/flutter_test.dart';
import 'package:media_kit/media_kit.dart';
import 'package:mobile/features/video/track_options.dart';

MediaTrackOption _apiTrack(int id, {String label = '', int streamPosition = 0}) {
  return MediaTrackOption(
    id: id,
    label: label.isEmpty ? 'Track $id' : label,
    url: 'https://h/$id.m3u8',
    language: '',
    title: '',
    codec: '',
    isDefault: false,
    isForced: false,
    streamPosition: streamPosition,
  );
}

void main() {
  group('TrackMenuBuilder.selectableHlsAudio', () {
    test('is always empty on web', () {
      final tracks = [
        const AudioTrack('1', 'English', 'en'),
        const AudioTrack('2', 'Japanese', 'ja'),
      ];
      expect(TrackMenuBuilder.selectableHlsAudio(tracks, isWeb: true), isEmpty);
    });

    test('drops sentinels and external (uri) tracks', () {
      final tracks = [
        AudioTrack.auto(),
        AudioTrack.no(),
        const AudioTrack('  ', null, null),
        AudioTrack.uri('https://h/ext.m4a'),
        const AudioTrack('1', 'English', 'en'),
        const AudioTrack('2', 'Japanese', 'ja'),
      ];
      final result = TrackMenuBuilder.selectableHlsAudio(tracks, isWeb: false);
      expect(result.map((t) => t.id), ['1', '2']);
    });

    test('requires more than one real track to bother offering a menu', () {
      final tracks = [AudioTrack.auto(), const AudioTrack('1', 'English', 'en')];
      expect(TrackMenuBuilder.selectableHlsAudio(tracks, isWeb: false), isEmpty);
    });
  });

  group('TrackMenuBuilder.selectableHlsSubtitle', () {
    test('drops sentinels, external and inline tracks', () {
      final tracks = [
        SubtitleTrack.auto(),
        SubtitleTrack.no(),
        SubtitleTrack.uri('https://h/ext.vtt'),
        SubtitleTrack.data('WEBVTT...'),
        const SubtitleTrack('1', 'English', 'en'),
      ];
      final result = TrackMenuBuilder.selectableHlsSubtitle(tracks);
      expect(result.map((t) => t.id), ['1']);
    });
  });

  group('TrackMenuBuilder.valueExists', () {
    final options = const [TrackMenuOption(value: 'api:1', label: 'A')];

    test('null (the default selection) always exists', () {
      expect(TrackMenuBuilder.valueExists(null, options), isTrue);
    });

    test('returns whether the value is present', () {
      expect(TrackMenuBuilder.valueExists('api:1', options), isTrue);
      expect(TrackMenuBuilder.valueExists('api:2', options), isFalse);
    });
  });

  group('TrackMenuBuilder.trackLabel', () {
    test('prefers title, then language, then fallback', () {
      expect(
        TrackMenuBuilder.trackLabel(const AudioTrack('1', 'English', 'en'), 'x'),
        'English',
      );
      expect(
        TrackMenuBuilder.trackLabel(const AudioTrack('1', null, 'ja'), 'x'),
        'ja',
      );
      expect(
        TrackMenuBuilder.trackLabel(const AudioTrack('1', null, null), 'Audio 1'),
        'Audio 1',
      );
    });
  });

  group('TrackMenuBuilder.audioOptions', () {
    final apiTracks = [_apiTrack(1, label: 'AAC'), _apiTrack(2, label: 'AC3')];
    final hlsTracks = [
      const AudioTrack('a', 'English', 'en'),
      const AudioTrack('b', 'Japanese', 'ja'),
    ];

    test('hides the menu when a scan found a single audio track', () {
      expect(
        TrackMenuBuilder.audioOptions(
          hlsTracks: hlsTracks,
          apiTracks: apiTracks,
          mediaTracksScanned: true,
          hasMultipleAudioTracks: false,
          isWeb: false,
        ),
        isEmpty,
      );
    });

    test('uses backend renditions on web', () {
      final options = TrackMenuBuilder.audioOptions(
        hlsTracks: hlsTracks,
        apiTracks: apiTracks,
        mediaTracksScanned: false,
        hasMultipleAudioTracks: true,
        isWeb: true,
      );
      expect(options.map((o) => o.value), ['api:1', 'api:2']);
      expect(options.every((o) => o.apiTrack != null), isTrue);
    });

    test('prefers detected HLS tracks on native', () {
      final options = TrackMenuBuilder.audioOptions(
        hlsTracks: hlsTracks,
        apiTracks: apiTracks,
        mediaTracksScanned: false,
        hasMultipleAudioTracks: true,
        isWeb: false,
      );
      expect(options.map((o) => o.value), ['hls:a', 'hls:b']);
      expect(options.first.label, 'English');
      expect(options.first.audioTrack, isNotNull);
    });

    test('falls back to backend renditions when no HLS tracks detected', () {
      final options = TrackMenuBuilder.audioOptions(
        hlsTracks: const [],
        apiTracks: apiTracks,
        mediaTracksScanned: false,
        hasMultipleAudioTracks: true,
        isWeb: false,
      );
      expect(options.map((o) => o.value), ['api:1', 'api:2']);
    });
  });

  group('TrackMenuBuilder.subtitleOptions', () {
    test('hides the menu when a scan found no subtitle tracks', () {
      expect(
        TrackMenuBuilder.subtitleOptions(
          hlsTracks: const [],
          apiTracks: [_apiTrack(1)],
          mediaTracksScanned: true,
          hasSubtitleTracks: false,
          isWeb: false,
        ),
        isEmpty,
      );
    });

    test('labels detected HLS subtitle tracks with a fallback index', () {
      final options = TrackMenuBuilder.subtitleOptions(
        hlsTracks: [const SubtitleTrack('s1', null, null)],
        apiTracks: const [],
        mediaTracksScanned: false,
        hasSubtitleTracks: true,
        isWeb: false,
      );
      expect(options.single.value, 'hls:s1');
      expect(options.single.label, 'Subtitle 1');
      expect(options.single.subtitleTrack, isNotNull);
    });
  });

  group('TrackMenuBuilder.resolveAudioTrack', () {
    test('null selection -> auto on native, rendition 0 on web', () {
      expect(
        TrackMenuBuilder.resolveAudioTrack(null, isWeb: false)?.id,
        'auto',
      );
      expect(TrackMenuBuilder.resolveAudioTrack(null, isWeb: true)?.id, '0');
    });

    test('detected HLS track is applied on native but a no-op on web', () {
      const hls = AudioTrack('a', 'English', 'en');
      final option = const TrackMenuOption(
        value: 'hls:a',
        label: 'English',
        audioTrack: hls,
      );
      expect(
        TrackMenuBuilder.resolveAudioTrack(option, isWeb: false),
        same(hls),
      );
      expect(TrackMenuBuilder.resolveAudioTrack(option, isWeb: true), isNull);
    });

    test('backend track -> rendition index on web, uri track on native', () {
      final option = TrackMenuOption(
        value: 'api:1',
        label: 'AC3',
        apiTrack: _apiTrack(1, label: 'AC3', streamPosition: 3),
      );
      expect(
        TrackMenuBuilder.resolveAudioTrack(option, isWeb: true)?.id,
        '3',
      );
      final native = TrackMenuBuilder.resolveAudioTrack(option, isWeb: false)!;
      expect(native.uri, isTrue);
      expect(native.id, 'https://h/1.m3u8');
      expect(native.title, 'AC3');
    });
  });

  group('TrackMenuBuilder.resolveNativeSubtitleTrack', () {
    test('null selection turns subtitles off', () {
      expect(TrackMenuBuilder.resolveNativeSubtitleTrack(null).id, 'no');
    });

    test('detected HLS subtitle is applied directly', () {
      const hls = SubtitleTrack('s1', 'English', 'en');
      final option = const TrackMenuOption(
        value: 'hls:s1',
        label: 'English',
        subtitleTrack: hls,
      );
      expect(
        TrackMenuBuilder.resolveNativeSubtitleTrack(option),
        same(hls),
      );
    });

    test('backend subtitle becomes a uri track with title fallback', () {
      final option = TrackMenuOption(
        value: 'api:2',
        label: 'Chinese',
        apiTrack: _apiTrack(2, label: 'Chinese'),
      );
      final track = TrackMenuBuilder.resolveNativeSubtitleTrack(option);
      expect(track.uri, isTrue);
      expect(track.id, 'https://h/2.m3u8');
      expect(track.title, 'Chinese');
    });
  });

  group('TrackParser.parseInt', () {
    test('handles num, numeric strings and junk', () {
      expect(TrackParser.parseInt(3), 3);
      expect(TrackParser.parseInt(3.9), 3);
      expect(TrackParser.parseInt('7'), 7);
      expect(TrackParser.parseInt('nope'), 0);
      expect(TrackParser.parseInt(null), 0);
    });
  });

  group('TrackParser.parseQualities', () {
    String abs(String u) => u.startsWith('http') ? u : 'https://base$u';

    test('parses entries and resolves relative URLs', () {
      final data = {
        'qualities': [
          {'name': '720p', 'label': 'HD', 'url': '/v/720p/index.m3u8'},
          {'name': '1080p', 'url': 'https://cdn/1080p/index.m3u8'},
          {'name': '', 'url': '/skipme'}, // missing name -> dropped
          {'name': 'x', 'url': ''}, // missing url -> dropped
        ],
      };
      final result = TrackParser.parseQualities(data, abs);
      expect(result.map((q) => q.name), ['720p', '1080p']);
      expect(result[0].label, 'HD');
      expect(result[0].url, 'https://base/v/720p/index.m3u8');
      expect(result[1].label, '1080p'); // label defaults to name
      expect(result[1].url, 'https://cdn/1080p/index.m3u8');
    });

    test('returns empty when the field is missing or malformed', () {
      expect(TrackParser.parseQualities(null, abs), isEmpty);
      expect(TrackParser.parseQualities({'qualities': 'nope'}, abs), isEmpty);
    });
  });

  group('TrackParser.parseMediaTracks', () {
    String abs(String u) => u.startsWith('http') ? u : 'https://base$u';

    test('parses tracks, defaults the label and resolves the URL', () {
      final data = {
        'audio_tracks': [
          {
            'id': 5,
            'url': '/a/5.m3u8',
            'language': 'en',
            'title': 'English',
            'codec': 'aac',
            'default': true,
            'forced': false,
            'stream_position': 2,
          },
          {'id': 6, 'url': '/a/6.m3u8'}, // label defaults to "Track 6"
          {'url': '/a/no-id.m3u8'}, // no id -> dropped
        ],
      };
      final result = TrackParser.parseMediaTracks(data, 'audio_tracks', abs);
      expect(result.map((t) => t.id), [5, 6]);
      expect(result[0].title, 'English');
      expect(result[0].isDefault, isTrue);
      expect(result[0].streamPosition, 2);
      expect(result[0].url, 'https://base/a/5.m3u8');
      expect(result[1].label, 'Track 6');
    });
  });
}
