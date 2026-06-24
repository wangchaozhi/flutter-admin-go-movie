import 'dart:async';
import 'dart:ui';

import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:http/http.dart' as http;
import 'package:media_kit/media_kit.dart';
import 'package:media_kit_video/media_kit_video.dart';

import '../../core/api_client.dart';
import '../../models/video.dart' as model;

class _QualityOption {
  final String name;
  final String label;
  final String url;

  const _QualityOption({
    required this.name,
    required this.label,
    required this.url,
  });
}

class _MediaTrackOption {
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

  const _MediaTrackOption({
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

class _TrackMenuOption {
  final String value;
  final String label;
  final AudioTrack? audioTrack;
  final SubtitleTrack? subtitleTrack;
  final _MediaTrackOption? apiTrack;

  const _TrackMenuOption({
    required this.value,
    required this.label,
    this.audioTrack,
    this.subtitleTrack,
    this.apiTrack,
  });
}

class _SubtitleCue {
  final Duration start;
  final Duration end;
  final String text;

  const _SubtitleCue({
    required this.start,
    required this.end,
    required this.text,
  });
}

class _PlaybackSource {
  final String url;
  final List<_QualityOption> qualities;
  final List<_MediaTrackOption> audioTracks;
  final List<_MediaTrackOption> subtitleTracks;
  final bool mediaTracksScanned;
  final bool hasMultipleAudioTracks;
  final bool hasSubtitleTracks;

  const _PlaybackSource({
    required this.url,
    required this.qualities,
    required this.audioTracks,
    required this.subtitleTracks,
    required this.mediaTracksScanned,
    required this.hasMultipleAudioTracks,
    required this.hasSubtitleTracks,
  });
}

class VideoPlayerPage extends StatefulWidget {
  final model.Video video;

  const VideoPlayerPage({super.key, required this.video});

  @override
  State<VideoPlayerPage> createState() => _VideoPlayerPageState();
}

class _VideoPlayerPageState extends State<VideoPlayerPage>
    with WidgetsBindingObserver {
  late final Player _player;
  late final VideoController _controller;
  StreamSubscription<String>? _playerErrorSubscription;
  StreamSubscription<bool>? _playerCompletedSubscription;
  StreamSubscription<Tracks>? _playerTracksSubscription;
  StreamSubscription<Duration>? _playerPositionSubscription;
  bool _loading = true;
  bool _switchingQuality = false;
  bool _switchingTrack = false;
  bool _showResumePlaybackButton = false;
  String? _error;
  String _selectedQuality = 'auto';
  List<_QualityOption> _qualities = const [];
  String? _selectedAudioTrackValue;
  String? _selectedSubtitleTrackValue;
  List<_MediaTrackOption> _audioTracks = const [];
  List<_MediaTrackOption> _subtitleTracks = const [];
  List<AudioTrack> _hlsAudioTracks = const [];
  List<SubtitleTrack> _hlsSubtitleTracks = const [];
  List<_SubtitleCue> _webSubtitleCues = const [];
  String _webSubtitleText = '';
  int _webSubtitleLoadSerial = 0;
  bool _mediaTracksScanned = false;
  bool _hasMultipleAudioTracks = false;
  bool _hasSubtitleTracks = false;
  Timer? _progressTimer;
  Timer? _refreshTimer;
  Duration _lastSavedPosition = Duration.zero;
  bool _savingProgress = false;
  bool _refreshingSource = false;
  bool _recoveringPlayback = false;
  bool _isFullscreen = false;
  double _playbackRate = 1.0;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addObserver(this);
    _player = Player();
    _controller = VideoController(_player);
    _playerErrorSubscription = _player.stream.error.listen(_handlePlayerError);
    _playerCompletedSubscription = _player.stream.completed.listen(
      _handlePlayerCompleted,
    );
    _playerTracksSubscription = _player.stream.tracks.listen(
      _syncDetectedTracks,
    );
    _playerPositionSubscription = _player.stream.position.listen(
      _syncWebSubtitleCue,
    );
    SystemChrome.setPreferredOrientations([
      DeviceOrientation.landscapeLeft,
      DeviceOrientation.landscapeRight,
      DeviceOrientation.portraitUp,
    ]);
    _init();
  }

  @override
  void dispose() {
    WidgetsBinding.instance.removeObserver(this);
    _progressTimer?.cancel();
    _refreshTimer?.cancel();
    _playerErrorSubscription?.cancel();
    _playerCompletedSubscription?.cancel();
    _playerTracksSubscription?.cancel();
    _playerPositionSubscription?.cancel();
    unawaited(_saveProgress(force: true));
    unawaited(_restoreSystemUi());
    _player.dispose();
    super.dispose();
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    if (state == AppLifecycleState.inactive ||
        state == AppLifecycleState.paused ||
        state == AppLifecycleState.detached ||
        state == AppLifecycleState.hidden) {
      unawaited(_saveProgress(force: true));
    }
  }

  Future<void> _init() async {
    if (mounted) {
      setState(() {
        _loading = true;
        _error = null;
        _showResumePlaybackButton = false;
      });
    }
    try {
      final source = await _fetchPlaybackSource();
      if (mounted) {
        setState(() {
          _qualities = source.qualities;
          _audioTracks = source.audioTracks;
          _subtitleTracks = source.subtitleTracks;
          _mediaTracksScanned = source.mediaTracksScanned;
          _hasMultipleAudioTracks = source.hasMultipleAudioTracks;
          _hasSubtitleTracks = source.hasSubtitleTracks;
          _hlsAudioTracks = const [];
          _hlsSubtitleTracks = const [];
          _webSubtitleCues = const [];
          _webSubtitleText = '';
          _webSubtitleLoadSerial++;
          _selectedQuality = 'auto';
          _selectedAudioTrackValue = null;
          _selectedSubtitleTrackValue = null;
        });
      }
      final resumePosition = await _loadSavedProgress();
      await _player.open(Media(source.url), play: false);
      await _player.setRate(_playbackRate);
      await _waitForMediaReady();
      _syncDetectedTracks(_player.state.tracks);
      await _applySelectedMediaTracks();
      if (resumePosition > Duration.zero && _canResumeAt(resumePosition)) {
        await _player.seek(resumePosition);
      }
      await _resumePlayback();
      if (mounted) {
        setState(() {
          _loading = false;
        });
      }
      _startProgressTimer();
      _startRefreshTimer();
    } catch (e) {
      if (mounted) {
        setState(() {
          _error = e.toString();
          _loading = false;
        });
      }
    }
  }

  Future<_PlaybackSource> _fetchPlaybackSource() async {
    final resp = await ApiClient().getAuth(
      '/api/videos/${widget.video.id}/play',
    );
    if (resp['code'] != 0) {
      throw Exception(resp['msg']?.toString() ?? '获取播放地址失败');
    }
    final data = resp['data'] as Map<String, dynamic>?;
    final rawUrl = data?['url'] as String? ?? '';
    if (rawUrl.isEmpty) {
      throw Exception('播放地址为空');
    }
    // backend returns a relative signed path; prepend baseUrl so it works on
    // emulator (10.0.2.2), simulator (localhost), and physical device (LAN IP)
    final url = _absoluteUrl(rawUrl);
    var qualities = _parseQualities(data);
    if (qualities.isEmpty) {
      qualities = await _parseQualitiesFromMaster(url);
    }
    final audioTracks = _parseMediaTracks(data, 'audio_tracks');
    final subtitleTracks = _parseMediaTracks(data, 'subtitle_tracks');
    final audioTrackCount = _parseInt(data?['audio_track_count']);
    final subtitleTrackCount = _parseInt(data?['subtitle_track_count']);
    final mediaTracksScanned = data?['media_tracks_scanned'] == true;
    return _PlaybackSource(
      url: url,
      qualities: [
        _QualityOption(name: 'auto', label: '自动', url: url),
        ...qualities,
      ],
      audioTracks: audioTracks,
      subtitleTracks: subtitleTracks,
      mediaTracksScanned: mediaTracksScanned,
      hasMultipleAudioTracks:
          data?['has_multiple_audio_tracks'] == true ||
          audioTrackCount > 1 ||
          audioTracks.isNotEmpty,
      hasSubtitleTracks:
          data?['has_subtitle_tracks'] == true ||
          subtitleTrackCount > 0 ||
          subtitleTracks.isNotEmpty,
    );
  }

  String _absoluteUrl(String url) {
    return url.startsWith('http') ? url : '${ApiClient.baseUrl}$url';
  }

  int _parseInt(dynamic value) {
    if (value is num) return value.toInt();
    return int.tryParse(value?.toString() ?? '') ?? 0;
  }

  List<_QualityOption> _parseQualities(Map<String, dynamic>? data) {
    final rawQualities = data?['qualities'];
    if (rawQualities is! List) return const [];
    return rawQualities
        .whereType<Map<String, dynamic>>()
        .map((item) {
          final name = item['name']?.toString() ?? '';
          final label = item['label']?.toString() ?? name;
          final url = item['url']?.toString() ?? '';
          if (name.isEmpty || url.isEmpty) return null;
          return _QualityOption(
            name: name,
            label: label.isEmpty ? name : label,
            url: _absoluteUrl(url),
          );
        })
        .whereType<_QualityOption>()
        .toList();
  }

  List<_MediaTrackOption> _parseMediaTracks(
    Map<String, dynamic>? data,
    String key,
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
          return _MediaTrackOption(
            id: id,
            label: label.isEmpty ? 'Track ${id.toString()}' : label,
            url: _absoluteUrl(url),
            language: item['language']?.toString() ?? '',
            title: item['title']?.toString() ?? '',
            codec: item['codec']?.toString() ?? '',
            isDefault: item['default'] == true,
            isForced: item['forced'] == true,
            streamPosition: _parseInt(item['stream_position']),
          );
        })
        .whereType<_MediaTrackOption>()
        .toList();
  }

  Future<List<_QualityOption>> _parseQualitiesFromMaster(String url) async {
    try {
      final response = await http.get(Uri.parse(url));
      if (response.statusCode < 200 || response.statusCode >= 300) {
        return const [];
      }
      final qualities = <_QualityOption>[];
      String resolution = '';
      for (final rawLine in response.body.split('\n')) {
        final line = rawLine.trim();
        if (line.startsWith('#EXT-X-STREAM-INF:')) {
          resolution = _parseResolution(line);
          continue;
        }
        final lineUri = Uri.tryParse(line);
        final path = lineUri?.path ?? line;
        if (line.startsWith('#') || !path.endsWith('.m3u8')) {
          continue;
        }
        final uri = Uri.parse(url).resolve(line).toString();
        final name = _qualityNameFromUri(uri);
        if (name.isEmpty) {
          resolution = '';
          continue;
        }
        qualities.add(
          _QualityOption(
            name: name,
            label: _qualityLabel(name, resolution),
            url: _absoluteUrl(uri),
          ),
        );
        resolution = '';
      }
      return qualities;
    } catch (_) {
      return const [];
    }
  }

  String _parseResolution(String line) {
    for (final part in line.split(',')) {
      final value = part.trim();
      if (value.startsWith('RESOLUTION=')) {
        return value.substring('RESOLUTION='.length);
      }
    }
    return '';
  }

  String _qualityNameFromUri(String uri) {
    final segments = Uri.parse(uri).pathSegments;
    if (segments.length >= 2 && segments.last == 'index.m3u8') {
      return segments[segments.length - 2];
    }
    return '';
  }

  String _qualityLabel(String name, String resolution) {
    if (name.isNotEmpty) return name;
    final parts = resolution.split('x');
    if (parts.length == 2 && parts[1].isNotEmpty) {
      return '${parts[1]}p';
    }
    return resolution.isNotEmpty ? resolution : '清晰度';
  }

  Future<void> _applySelectedMediaTracks() async {
    final audioOption = _selectedAudioTrackValue == null
        ? null
        : _audioMenuOptions()
              .where((option) => option.value == _selectedAudioTrackValue)
              .firstOrNull;
    final subtitleOption = _selectedSubtitleTrackValue == null
        ? null
        : _subtitleMenuOptions()
              .where((option) => option.value == _selectedSubtitleTrackValue)
              .firstOrNull;

    if (audioOption == null) {
      if (kIsWeb) {
        // hls.js: rendition index 0 is the default (muxed) audio track.
        await _player.setAudioTrack(_webHlsAudioTrack(0));
      } else {
        await _player.setAudioTrack(AudioTrack.auto());
      }
    } else if (audioOption.audioTrack != null) {
      if (!kIsWeb) {
        await _player.setAudioTrack(audioOption.audioTrack!);
      }
    } else {
      final apiTrack = audioOption.apiTrack!;
      if (kIsWeb) {
        // hls.js switches by rendition index, which matches stream_position.
        await _player.setAudioTrack(_webHlsAudioTrack(apiTrack.streamPosition));
      } else {
        await _player.setAudioTrack(
          AudioTrack.uri(
            apiTrack.url,
            title: apiTrack.title.isEmpty ? apiTrack.label : apiTrack.title,
            language: apiTrack.language.isEmpty ? null : apiTrack.language,
          ),
        );
      }
    }

    if (kIsWeb) {
      await _applyWebSubtitleTrack(subtitleOption);
    } else if (subtitleOption == null) {
      await _player.setSubtitleTrack(SubtitleTrack.no());
    } else if (subtitleOption.subtitleTrack != null) {
      await _player.setSubtitleTrack(subtitleOption.subtitleTrack!);
    } else {
      final apiTrack = subtitleOption.apiTrack!;
      await _player.setSubtitleTrack(
        SubtitleTrack.uri(
          apiTrack.url,
          title: apiTrack.title.isEmpty ? apiTrack.label : apiTrack.title,
          language: apiTrack.language.isEmpty ? null : apiTrack.language,
        ),
      );
    }
  }

  Future<void> _switchAudioTrack(String value) async {
    if (_switchingTrack) return;
    final nextValue = value == 'auto' ? null : value;
    if (nextValue == _selectedAudioTrackValue) return;
    if (nextValue != null &&
        !_audioMenuOptions().any((option) => option.value == nextValue)) {
      return;
    }

    final previousValue = _selectedAudioTrackValue;
    setState(() {
      _switchingTrack = true;
      _selectedAudioTrackValue = nextValue;
    });
    try {
      await _applySelectedMediaTracks();
    } catch (e) {
      if (mounted) {
        setState(() => _selectedAudioTrackValue = previousValue);
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text('Audio track failed: $e')));
      }
      await _applySelectedMediaTracks();
    } finally {
      if (mounted) setState(() => _switchingTrack = false);
    }
  }

  Future<void> _switchSubtitleTrack(String value) async {
    if (_switchingTrack) return;
    final nextValue = value == 'off' ? null : value;
    if (nextValue == _selectedSubtitleTrackValue) return;
    if (nextValue != null &&
        !_subtitleMenuOptions().any((option) => option.value == nextValue)) {
      return;
    }

    final previousValue = _selectedSubtitleTrackValue;
    setState(() {
      _switchingTrack = true;
      _selectedSubtitleTrackValue = nextValue;
    });
    try {
      await _applySelectedMediaTracks();
    } catch (e) {
      if (mounted) {
        setState(() => _selectedSubtitleTrackValue = previousValue);
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text('Subtitle failed: $e')));
      }
      await _applySelectedMediaTracks();
    } finally {
      if (mounted) setState(() => _switchingTrack = false);
    }
  }

  /// Builds an [AudioTrack] whose id is the hls.js rendition index. The
  /// vendored media_kit fork interprets a numeric, non-uri [AudioTrack.id] on
  /// web as `hls.audioTrack = index`. See packages/media_kit/README_FORK.md.
  AudioTrack _webHlsAudioTrack(int index) {
    return AudioTrack(index.toString(), null, null);
  }

  Future<void> _applyWebSubtitleTrack(_TrackMenuOption? option) async {
    final serial = ++_webSubtitleLoadSerial;
    _setWebSubtitleCues(const []);
    if (option == null) return;

    final apiTrack = option.apiTrack;
    if (apiTrack == null) return;

    final response = await http.get(Uri.parse(apiTrack.url));
    if (response.statusCode < 200 || response.statusCode >= 300) {
      throw Exception('subtitle http ${response.statusCode}');
    }
    final cues = _parseWebVTT(response.body);
    if (serial != _webSubtitleLoadSerial) return;
    _setWebSubtitleCues(cues);
    _syncWebSubtitleCue(_player.state.position);
  }

  void _setWebSubtitleCues(List<_SubtitleCue> cues) {
    if (!kIsWeb) return;
    if (!mounted) {
      _webSubtitleCues = cues;
      _webSubtitleText = '';
      return;
    }
    setState(() {
      _webSubtitleCues = cues;
      _webSubtitleText = '';
    });
  }

  void _syncWebSubtitleCue(Duration position) {
    if (!kIsWeb || _webSubtitleCues.isEmpty) {
      if (kIsWeb && _webSubtitleText.isNotEmpty && mounted) {
        setState(() => _webSubtitleText = '');
      }
      return;
    }
    final cue = _webSubtitleCues.where((item) {
      return position >= item.start && position <= item.end;
    }).firstOrNull;
    final nextText = cue?.text ?? '';
    if (nextText == _webSubtitleText) return;
    if (!mounted) {
      _webSubtitleText = nextText;
      return;
    }
    setState(() => _webSubtitleText = nextText);
  }

  List<_SubtitleCue> _parseWebVTT(String body) {
    final normalized = body
        .replaceFirst('\uFEFF', '')
        .replaceAll('\r\n', '\n')
        .replaceAll('\r', '\n');
    final cues = <_SubtitleCue>[];
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

      final start = _parseWebVTTTimestamp(timingParts[0].trim());
      final endToken = timingParts[1].trim().split(RegExp(r'\s+')).first;
      final end = _parseWebVTTTimestamp(endToken);
      if (start == null || end == null || end <= start) continue;

      final text = lines
          .skip(timingIndex + 1)
          .map(_cleanWebVTTText)
          .where((line) => line.isNotEmpty)
          .join('\n');
      if (text.isEmpty) continue;
      cues.add(_SubtitleCue(start: start, end: end, text: text));
    }
    cues.sort((a, b) => a.start.compareTo(b.start));
    return cues;
  }

  Duration? _parseWebVTTTimestamp(String value) {
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

  String _cleanWebVTTText(String value) {
    return value
        .replaceAll(RegExp(r'<[^>]+>'), '')
        .replaceAll('&amp;', '&')
        .replaceAll('&lt;', '<')
        .replaceAll('&gt;', '>')
        .replaceAll('&nbsp;', ' ')
        .trim();
  }

  Future<void> _switchQuality(String name) async {
    if (name == _selectedQuality || _switchingQuality) return;
    final option = _qualities.where((q) => q.name == name).firstOrNull;
    if (option == null) return;

    final position = _player.state.position;
    final wasPlaying = _player.state.playing;
    final previousQuality = _selectedQuality;
    final previousOption = _qualities
        .where((quality) => quality.name == previousQuality)
        .firstOrNull;
    setState(() {
      _switchingQuality = true;
      _showResumePlaybackButton = false;
    });

    try {
      await _player.open(Media(option.url), play: false);
      await _player.setRate(_playbackRate);
      await _waitForMediaReady();
      _syncDetectedTracks(_player.state.tracks);
      await _applySelectedMediaTracks();
      if (position > Duration.zero) {
        await _player.seek(position);
      }
      if (mounted) {
        setState(() => _selectedQuality = name);
      }
      if (wasPlaying) {
        await _resumePlayback();
      } else {
        await _player.pause();
      }
    } catch (e) {
      if (previousOption != null) {
        await _restoreQuality(previousOption, position, wasPlaying);
      }
      if (mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text('${option.label} 加载失败：$e')));
      }
    } finally {
      if (mounted) setState(() => _switchingQuality = false);
    }
  }

  Future<void> _restoreQuality(
    _QualityOption option,
    Duration position,
    bool shouldPlay,
  ) async {
    try {
      await _player.open(Media(option.url), play: false);
      await _player.setRate(_playbackRate);
      await _waitForMediaReady();
      _syncDetectedTracks(_player.state.tracks);
      await _applySelectedMediaTracks();
      if (position > Duration.zero) {
        await _player.seek(position);
      }
      if (shouldPlay) {
        await _resumePlayback();
      }
    } catch (_) {
      if (mounted) {
        setState(() => _showResumePlaybackButton = true);
      }
    }
  }


  Future<void> _waitForMediaReady() async {
    if (_player.state.duration > Duration.zero) return;
    try {
      await _player.stream.duration
          .firstWhere((duration) => duration > Duration.zero)
          .timeout(const Duration(seconds: 3));
    } on TimeoutException {
      // HLS metadata can be slow on some platforms; still attempt the seek.
    }
  }

  Future<void> _resumePlayback() async {
    try {
      if (_player.state.completed) {
        await _player.seek(Duration.zero);
      }
      await _player.play();
      if (mounted) {
        setState(() => _showResumePlaybackButton = false);
      }
    } catch (_) {
      if (mounted) {
        setState(() => _showResumePlaybackButton = true);
      }
    }
  }

  Future<void> _seekRelative(Duration offset) async {
    final duration = _player.state.duration;
    final current = _player.state.position;
    var target = current + offset;
    if (target < Duration.zero) {
      target = Duration.zero;
    }
    if (duration > Duration.zero && target > duration) {
      target = duration;
    }
    await _player.seek(target);
    unawaited(_saveProgress(force: true));
  }

  Future<void> _changePlaybackRate(double rate) async {
    await _player.setRate(rate);
    if (mounted) {
      setState(() => _playbackRate = rate);
    }
  }

  void _handlePlayerCompleted(bool completed) {
    if (!completed) return;
    unawaited(_saveProgress(force: true, positionOverride: Duration.zero));
    if (mounted) {
      setState(() => _showResumePlaybackButton = true);
    }
  }

  void _syncDetectedTracks(Tracks tracks) {
    final audioTracks = _selectableHLSAudioTracks(tracks.audio);
    final subtitleTracks = _selectableHLSSubtitleTracks(tracks.subtitle);
    final nextAudioValue =
        _trackValueExists(
          _selectedAudioTrackValue,
          _audioMenuOptionsFor(audioTracks, _audioTracks),
        )
        ? _selectedAudioTrackValue
        : null;
    final nextSubtitleValue =
        _trackValueExists(
          _selectedSubtitleTrackValue,
          _subtitleMenuOptionsFor(subtitleTracks, _subtitleTracks),
        )
        ? _selectedSubtitleTrackValue
        : null;

    if (!mounted) {
      _hlsAudioTracks = audioTracks;
      _hlsSubtitleTracks = subtitleTracks;
      _selectedAudioTrackValue = nextAudioValue;
      _selectedSubtitleTrackValue = nextSubtitleValue;
      return;
    }

    setState(() {
      _hlsAudioTracks = audioTracks;
      _hlsSubtitleTracks = subtitleTracks;
      _selectedAudioTrackValue = nextAudioValue;
      _selectedSubtitleTrackValue = nextSubtitleValue;
    });
  }

  List<AudioTrack> _selectableHLSAudioTracks(List<AudioTrack> tracks) {
    if (kIsWeb) return const [];
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

  List<SubtitleTrack> _selectableHLSSubtitleTracks(List<SubtitleTrack> tracks) {
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

  bool _trackValueExists(String? value, List<_TrackMenuOption> options) {
    return value == null || options.any((option) => option.value == value);
  }

  List<_TrackMenuOption> _audioMenuOptions() {
    return _audioMenuOptionsFor(_hlsAudioTracks, _audioTracks);
  }

  List<_TrackMenuOption> _subtitleMenuOptions() {
    return _subtitleMenuOptionsFor(_hlsSubtitleTracks, _subtitleTracks);
  }

  List<_TrackMenuOption> _audioMenuOptionsFor(
    List<AudioTrack> hlsTracks,
    List<_MediaTrackOption> apiTracks,
  ) {
    if (_mediaTracksScanned && !_hasMultipleAudioTracks) {
      return const [];
    }
    if (kIsWeb) {
      return _apiTrackMenuOptions(apiTracks);
    }
    if (hlsTracks.isNotEmpty) {
      return [
        for (var i = 0; i < hlsTracks.length; i++)
          _TrackMenuOption(
            value: 'hls:${hlsTracks[i].id}',
            label: _playerTrackLabel(hlsTracks[i], 'Audio ${i + 1}'),
            audioTrack: hlsTracks[i],
          ),
      ];
    }
    return _apiTrackMenuOptions(apiTracks);
  }

  List<_TrackMenuOption> _subtitleMenuOptionsFor(
    List<SubtitleTrack> hlsTracks,
    List<_MediaTrackOption> apiTracks,
  ) {
    if (_mediaTracksScanned && !_hasSubtitleTracks) {
      return const [];
    }
    if (kIsWeb) {
      return _apiTrackMenuOptions(apiTracks);
    }
    if (hlsTracks.isNotEmpty) {
      return [
        for (var i = 0; i < hlsTracks.length; i++)
          _TrackMenuOption(
            value: 'hls:${hlsTracks[i].id}',
            label: _playerTrackLabel(hlsTracks[i], 'Subtitle ${i + 1}'),
            subtitleTrack: hlsTracks[i],
          ),
      ];
    }
    return _apiTrackMenuOptions(apiTracks);
  }

  List<_TrackMenuOption> _apiTrackMenuOptions(
    List<_MediaTrackOption> apiTracks,
  ) {
    return [
      for (final track in apiTracks)
        _TrackMenuOption(
          value: 'api:${track.id}',
          label: track.label,
          apiTrack: track,
        ),
    ];
  }

  String _playerTrackLabel(dynamic track, String fallback) {
    final title = track.title?.toString().trim() ?? '';
    if (title.isNotEmpty) return title;
    final language = track.language?.toString().trim() ?? '';
    if (language.isNotEmpty) return language;
    return fallback;
  }

  Future<Duration> _loadSavedProgress() async {
    try {
      final resp = await ApiClient().getAuth(
        '/api/videos/${widget.video.id}/progress',
      );
      if (resp['code'] != 0) return Duration.zero;
      final data = resp['data'];
      final position = data is Map<String, dynamic> ? data['position'] : null;
      if (position is num && position > 0) {
        return Duration(seconds: position.toInt());
      }
    } catch (_) {
      // Progress is a comfort feature; playback should not fail when it is absent.
    }
    return Duration.zero;
  }

  bool _canResumeAt(Duration position) {
    final duration = _player.state.duration;
    if (duration <= Duration.zero) return true;
    return position < duration - const Duration(seconds: 20);
  }

  void _startProgressTimer() {
    _progressTimer?.cancel();
    _progressTimer = Timer.periodic(const Duration(seconds: 10), (_) {
      unawaited(_saveProgress());
    });
  }

  void _handlePlayerError(String error) {
    final lower = error.toLowerCase();
    final mayBeExpiredHls =
        lower.contains('403') ||
        lower.contains('410') ||
        lower.contains('expired') ||
        lower.contains('forbidden');
    if (mayBeExpiredHls) {
      unawaited(_recoverPlaybackAfterError());
      return;
    }
    if (mounted) {
      setState(() => _showResumePlaybackButton = true);
    }
  }

  Future<void> _saveProgress({
    bool force = false,
    Duration? positionOverride,
  }) async {
    if (_savingProgress) return;
    final position = positionOverride ?? _player.state.position;
    final duration = _player.state.duration;
    if (position < Duration.zero) return;
    if (position == Duration.zero && positionOverride == null) return;
    if (!force &&
        (position - _lastSavedPosition).abs() < const Duration(seconds: 5)) {
      return;
    }

    _savingProgress = true;
    try {
      await ApiClient().postAuth('/api/videos/${widget.video.id}/progress', {
        'position': position.inSeconds,
        'duration': duration.inSeconds,
      });
      _lastSavedPosition = position;
    } catch (_) {
      // Network blips are expected during playback; the next tick will retry.
    } finally {
      _savingProgress = false;
    }
  }

  void _startRefreshTimer() {
    _refreshTimer?.cancel();
    _refreshTimer = Timer.periodic(const Duration(minutes: 20), (_) {
      unawaited(_refreshPlaybackSource());
    });
  }

  Future<void> _refreshPlaybackSource({bool resumeAfterRefresh = false}) async {
    if (_refreshingSource || _switchingQuality || _loading) return;
    _refreshingSource = true;
    final position = _player.state.position;
    final wasPlaying = resumeAfterRefresh || _player.state.playing;
    final selectedQuality = _selectedQuality;
    try {
      final source = await _fetchPlaybackSource();
      final nextOption =
          source.qualities
              .where((q) => q.name == selectedQuality)
              .firstOrNull ??
          source.qualities.first;
      if (!mounted) return;
      setState(() {
        _qualities = source.qualities;
        _audioTracks = source.audioTracks;
        _subtitleTracks = source.subtitleTracks;
        _mediaTracksScanned = source.mediaTracksScanned;
        _hasMultipleAudioTracks = source.hasMultipleAudioTracks;
        _hasSubtitleTracks = source.hasSubtitleTracks;
        _selectedQuality = nextOption.name;
        _switchingQuality = true;
      });
      await _player.open(Media(nextOption.url), play: false);
      await _player.setRate(_playbackRate);
      await _waitForMediaReady();
      _syncDetectedTracks(_player.state.tracks);
      await _applySelectedMediaTracks();
      if (position > Duration.zero) {
        await _player.seek(position);
      }
      if (wasPlaying) {
        await _resumePlayback();
      }
    } catch (_) {
      // Keep the current playback alive; this timer is only renewing signed URLs.
    } finally {
      _refreshingSource = false;
      if (mounted) setState(() => _switchingQuality = false);
    }
  }

  Future<void> _recoverPlaybackAfterError() async {
    if (_recoveringPlayback) return;
    _recoveringPlayback = true;
    try {
      await _refreshPlaybackSource(resumeAfterRefresh: true);
    } finally {
      _recoveringPlayback = false;
    }
  }

  Future<void> _toggleFullscreen() async {
    if (_isFullscreen) {
      await _exitFullscreen();
    } else {
      await _enterFullscreen();
    }
  }

  Future<void> _enterFullscreen() async {
    await SystemChrome.setPreferredOrientations([
      DeviceOrientation.landscapeLeft,
      DeviceOrientation.landscapeRight,
    ]);
    await SystemChrome.setEnabledSystemUIMode(SystemUiMode.immersiveSticky);
    if (mounted) {
      setState(() => _isFullscreen = true);
    }
  }

  Future<void> _exitFullscreen() async {
    await SystemChrome.setPreferredOrientations([
      DeviceOrientation.landscapeLeft,
      DeviceOrientation.landscapeRight,
      DeviceOrientation.portraitUp,
    ]);
    await SystemChrome.setEnabledSystemUIMode(SystemUiMode.edgeToEdge);
    if (mounted) {
      setState(() => _isFullscreen = false);
    }
  }

  Future<void> _restoreSystemUi() async {
    await SystemChrome.setPreferredOrientations([DeviceOrientation.portraitUp]);
    await SystemChrome.setEnabledSystemUIMode(SystemUiMode.edgeToEdge);
  }

  @override
  Widget build(BuildContext context) {
    if (_isFullscreen) {
      return PopScope(
        canPop: false,
        onPopInvokedWithResult: (didPop, result) {
          if (!didPop) {
            unawaited(_exitFullscreen());
          }
        },
        child: Scaffold(
          backgroundColor: Colors.black,
          body: Center(
            child: AspectRatio(
              aspectRatio: 16 / 9,
              child: _loading
                  ? const Center(
                      child: CircularProgressIndicator(
                        color: Color(0xFF25D0AB),
                      ),
                    )
                  : _error != null
                  ? _buildError()
                  : _buildPlayer(),
            ),
          ),
        ),
      );
    }

    return Scaffold(
      backgroundColor: Colors.black,
      body: SafeArea(
        child: Column(
          children: [
            AspectRatio(
              aspectRatio: 16 / 9,
              child: _loading
                  ? const Center(
                      child: CircularProgressIndicator(
                        color: Color(0xFF25D0AB),
                      ),
                    )
                  : _error != null
                  ? _buildError()
                  : _buildPlayer(),
            ),
            Expanded(
              child: SingleChildScrollView(
                padding: const EdgeInsets.all(16),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Row(
                      children: [
                        IconButton(
                          icon: const Icon(
                            Icons.arrow_back,
                            color: Colors.white,
                          ),
                          onPressed: () => Navigator.pop(context),
                        ),
                        Expanded(
                          child: Text(
                            widget.video.title,
                            style: const TextStyle(
                              color: Colors.white,
                              fontSize: 17,
                              fontWeight: FontWeight.w900,
                            ),
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                          ),
                        ),
                      ],
                    ),
                    if (widget.video.categoryName.isNotEmpty) ...[
                      const SizedBox(height: 4),
                      Padding(
                        padding: const EdgeInsets.only(left: 48),
                        child: Text(
                          widget.video.categoryName,
                          style: const TextStyle(
                            color: Color(0xFF25D0AB),
                            fontSize: 13,
                          ),
                        ),
                      ),
                    ],
                    if (widget.video.description.isNotEmpty) ...[
                      const SizedBox(height: 12),
                      Text(
                        widget.video.description,
                        style: const TextStyle(
                          color: Color(0xFF9CA3AF),
                          height: 1.5,
                        ),
                      ),
                    ],
                  ],
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildPlayer() {
    return Stack(
      children: [
        Video(controller: _controller, controls: AdaptiveVideoControls),
        _buildBufferingIndicator(),
        _buildCenterPlayButton(),
        _buildCompletedOverlay(),
        _buildWebSubtitleOverlay(),
        Positioned(left: 48, bottom: 4, child: _buildPlaybackTools()),
        Positioned(right: 48, bottom: 4, child: _buildTrackAndQualityMenus()),
        Positioned(right: 8, top: 8, child: _buildFullscreenButton()),
      ],
    );
  }

  Widget _buildBufferingIndicator() {
    return StreamBuilder<bool>(
      stream: _player.stream.buffering,
      initialData: _player.state.buffering,
      builder: (context, snapshot) {
        final buffering = snapshot.data ?? false;
        if (!buffering || _switchingQuality) return const SizedBox.shrink();

        return Positioned.fill(
          child: IgnorePointer(
            child: Container(
              color: Colors.black26,
              child: const Center(
                child: SizedBox(
                  width: 42,
                  height: 42,
                  child: CircularProgressIndicator(
                    strokeWidth: 3,
                    color: Color(0xFF25D0AB),
                  ),
                ),
              ),
            ),
          ),
        );
      },
    );
  }

  Widget _buildCenterPlayButton() {
    return StreamBuilder<bool>(
      stream: _player.stream.playing,
      initialData: _player.state.playing,
      builder: (context, snapshot) {
        final isPlaying = snapshot.data ?? false;
        final shouldShow =
            _showResumePlaybackButton || (!isPlaying && !_switchingQuality);
        if (!shouldShow) return const SizedBox.shrink();

        return Positioned.fill(
          child: Container(
            color: Colors.black26,
            child: Center(
              child: ClipOval(
                child: BackdropFilter(
                  filter: ImageFilter.blur(sigmaX: 22, sigmaY: 22),
                  child: DecoratedBox(
                    decoration: BoxDecoration(
                      shape: BoxShape.circle,
                      color: Colors.white.withValues(alpha: 0.18),
                      border: Border.all(
                        color: Colors.white.withValues(alpha: 0.42),
                      ),
                      boxShadow: [
                        BoxShadow(
                          color: Colors.black.withValues(alpha: 0.28),
                          blurRadius: 28,
                          offset: const Offset(0, 14),
                        ),
                        BoxShadow(
                          color: Colors.white.withValues(alpha: 0.16),
                          blurRadius: 18,
                          offset: const Offset(-8, -8),
                        ),
                      ],
                    ),
                    child: Material(
                      color: Colors.transparent,
                      child: InkWell(
                        customBorder: const CircleBorder(),
                        onTap: _resumePlayback,
                        child: const SizedBox(
                          width: 76,
                          height: 76,
                          child: Icon(
                            Icons.play_arrow_rounded,
                            color: Colors.white,
                            size: 48,
                          ),
                        ),
                      ),
                    ),
                  ),
                ),
              ),
            ),
          ),
        );
      },
    );
  }

  Widget _buildCompletedOverlay() {
    return StreamBuilder<bool>(
      stream: _player.stream.completed,
      initialData: _player.state.completed,
      builder: (context, snapshot) {
        final completed = snapshot.data ?? false;
        if (!completed) return const SizedBox.shrink();

        return Positioned.fill(
          child: Container(
            color: Colors.black45,
            child: Center(
              child: FilledButton.icon(
                onPressed: _resumePlayback,
                style: FilledButton.styleFrom(
                  backgroundColor: const Color(0xFF25D0AB),
                  foregroundColor: Colors.black,
                ),
                icon: const Icon(Icons.replay_rounded),
                label: const Text('重播'),
              ),
            ),
          ),
        );
      },
    );
  }

  Widget _buildWebSubtitleOverlay() {
    if (!kIsWeb || _webSubtitleText.isEmpty) {
      return const SizedBox.shrink();
    }
    return Positioned(
      left: 24,
      right: 24,
      bottom: 54,
      child: IgnorePointer(
        child: Center(
          child: DecoratedBox(
            decoration: BoxDecoration(
              color: Colors.black.withValues(alpha: 0.62),
              borderRadius: BorderRadius.circular(6),
            ),
            child: Padding(
              padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 7),
              child: Text(
                _webSubtitleText,
                textAlign: TextAlign.center,
                maxLines: 3,
                overflow: TextOverflow.ellipsis,
                style: const TextStyle(
                  color: Colors.white,
                  fontSize: 17,
                  height: 1.28,
                  fontWeight: FontWeight.w700,
                  shadows: [
                    Shadow(
                      blurRadius: 4,
                      color: Colors.black,
                      offset: Offset(0, 1),
                    ),
                  ],
                ),
              ),
            ),
          ),
        ),
      ),
    );
  }

  Widget _buildPlaybackTools() {
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        _glassIconButton(
          tooltip: '后退10秒',
          icon: Icons.replay_10_rounded,
          onPressed: () =>
              unawaited(_seekRelative(const Duration(seconds: -10))),
        ),
        const SizedBox(width: 8),
        _buildSpeedMenu(),
        const SizedBox(width: 8),
        _glassIconButton(
          tooltip: '前进10秒',
          icon: Icons.forward_10_rounded,
          onPressed: () =>
              unawaited(_seekRelative(const Duration(seconds: 10))),
        ),
      ],
    );
  }

  Widget _buildSpeedMenu() {
    const rates = [0.75, 1.0, 1.25, 1.5, 2.0];
    return PopupMenuButton<double>(
      tooltip: '播放速度',
      color: const Color(0xFF111827),
      initialValue: _playbackRate,
      onSelected: (rate) => unawaited(_changePlaybackRate(rate)),
      itemBuilder: (context) => rates
          .map(
            (rate) => PopupMenuItem<double>(
              value: rate,
              child: Row(
                mainAxisSize: MainAxisSize.min,
                children: [
                  SizedBox(
                    width: 18,
                    height: 18,
                    child: rate == _playbackRate
                        ? const Icon(
                            Icons.check,
                            color: Color(0xFF25D0AB),
                            size: 18,
                          )
                        : null,
                  ),
                  const SizedBox(width: 8),
                  Text(
                    '${_formatRate(rate)}x',
                    style: const TextStyle(color: Colors.white),
                  ),
                ],
              ),
            ),
          )
          .toList(),
      child: _glassChip('${_formatRate(_playbackRate)}x'),
    );
  }

  Widget _buildFullscreenButton() {
    return _glassIconButton(
      tooltip: _isFullscreen ? '退出全屏' : '全屏',
      icon: _isFullscreen
          ? Icons.fullscreen_exit_rounded
          : Icons.fullscreen_rounded,
      onPressed: () => unawaited(_toggleFullscreen()),
    );
  }

  Widget _buildError() {
    return Center(
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          const Icon(Icons.error_outline, color: Colors.redAccent, size: 48),
          const SizedBox(height: 12),
          Text(
            _error!,
            style: const TextStyle(color: Colors.white70),
            textAlign: TextAlign.center,
          ),
          const SizedBox(height: 16),
          Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              TextButton(
                onPressed: _init,
                child: const Text(
                  '重试',
                  style: TextStyle(color: Color(0xFF25D0AB)),
                ),
              ),
              const SizedBox(width: 12),
              TextButton(
                onPressed: () => Navigator.pop(context),
                child: const Text(
                  '返回',
                  style: TextStyle(color: Color(0xFF25D0AB)),
                ),
              ),
            ],
          ),
        ],
      ),
    );
  }

  Widget _buildTrackAndQualityMenus() {
    final audioOptions = _audioMenuOptions();
    final subtitleOptions = _subtitleMenuOptions();
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        if (audioOptions.isNotEmpty) ...[
          _buildAudioMenu(audioOptions),
          const SizedBox(width: 8),
        ],
        if (subtitleOptions.isNotEmpty) ...[
          _buildSubtitleMenu(subtitleOptions),
          const SizedBox(width: 8),
        ],
        _buildQualityMenu(),
      ],
    );
  }

  Widget _buildAudioMenu(List<_TrackMenuOption> options) {
    final selectedValue = _selectedAudioTrackValue ?? 'auto';
    return PopupMenuButton<String>(
      enabled: !_switchingTrack,
      tooltip: 'Audio',
      color: const Color(0xFF111827),
      initialValue: selectedValue,
      onSelected: (value) => unawaited(_switchAudioTrack(value)),
      itemBuilder: (context) => [
        PopupMenuItem<String>(
          value: 'auto',
          child: _buildTrackMenuItem('Default', selectedValue == 'auto'),
        ),
        ...options.map(
          (option) => PopupMenuItem<String>(
            value: option.value,
            child: _buildTrackMenuItem(
              option.label,
              selectedValue == option.value,
            ),
          ),
        ),
      ],
      child: _glassMenuIcon(Icons.audiotrack_rounded),
    );
  }

  Widget _buildSubtitleMenu(List<_TrackMenuOption> options) {
    final selectedValue = _selectedSubtitleTrackValue ?? 'off';
    return PopupMenuButton<String>(
      enabled: !_switchingTrack,
      tooltip: 'Subtitles',
      color: const Color(0xFF111827),
      initialValue: selectedValue,
      onSelected: (value) => unawaited(_switchSubtitleTrack(value)),
      itemBuilder: (context) => [
        PopupMenuItem<String>(
          value: 'off',
          child: _buildTrackMenuItem('Off', selectedValue == 'off'),
        ),
        ...options.map(
          (option) => PopupMenuItem<String>(
            value: option.value,
            child: _buildTrackMenuItem(
              option.label,
              selectedValue == option.value,
            ),
          ),
        ),
      ],
      child: _glassMenuIcon(Icons.subtitles_rounded),
    );
  }

  Widget _buildTrackMenuItem(String label, bool selected) {
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        SizedBox(
          width: 18,
          height: 18,
          child: selected
              ? const Icon(Icons.check, color: Color(0xFF25D0AB), size: 18)
              : null,
        ),
        const SizedBox(width: 8),
        ConstrainedBox(
          constraints: const BoxConstraints(maxWidth: 180),
          child: Text(
            label,
            style: const TextStyle(color: Colors.white),
            overflow: TextOverflow.ellipsis,
          ),
        ),
      ],
    );
  }

  Widget _glassMenuIcon(IconData icon) {
    return ClipOval(
      child: BackdropFilter(
        filter: ImageFilter.blur(sigmaX: 16, sigmaY: 16),
        child: DecoratedBox(
          decoration: BoxDecoration(
            shape: BoxShape.circle,
            color: Colors.white.withValues(alpha: 0.14),
            border: Border.all(color: Colors.white.withValues(alpha: 0.26)),
          ),
          child: SizedBox(
            width: 36,
            height: 36,
            child: Center(
              child: _switchingTrack
                  ? const SizedBox(
                      width: 13,
                      height: 13,
                      child: CircularProgressIndicator(
                        strokeWidth: 2,
                        color: Colors.white,
                      ),
                    )
                  : Icon(icon, color: Colors.white, size: 20),
            ),
          ),
        ),
      ),
    );
  }

  Widget _buildQualityMenu() {
    final selected = _selectedQualityLabel;

    return PopupMenuButton<String>(
      enabled: _qualities.length > 1 && !_switchingQuality,
      tooltip: '选择清晰度',
      color: const Color(0xFF111827),
      initialValue: _selectedQuality,
      onSelected: _switchQuality,
      itemBuilder: (context) => _qualities
          .map(
            (quality) => PopupMenuItem<String>(
              value: quality.name,
              child: Row(
                mainAxisSize: MainAxisSize.min,
                children: [
                  SizedBox(
                    width: 18,
                    height: 18,
                    child: quality.name == _selectedQuality
                        ? const Icon(
                            Icons.check,
                            color: Color(0xFF25D0AB),
                            size: 18,
                          )
                        : null,
                  ),
                  const SizedBox(width: 8),
                  Text(
                    quality.label,
                    style: const TextStyle(color: Colors.white),
                  ),
                ],
              ),
            ),
          )
          .toList(),
      child: ClipRRect(
        borderRadius: BorderRadius.circular(18),
        child: BackdropFilter(
          filter: ImageFilter.blur(sigmaX: 16, sigmaY: 16),
          child: DecoratedBox(
            decoration: BoxDecoration(
              color: Colors.white.withValues(alpha: 0.14),
              borderRadius: BorderRadius.circular(18),
              border: Border.all(color: Colors.white.withValues(alpha: 0.26)),
            ),
            child: SizedBox(
              height: 36,
              child: Padding(
                padding: const EdgeInsets.symmetric(horizontal: 10),
                child: Center(
                  child: Row(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      if (_switchingQuality) ...[
                        const SizedBox(
                          width: 13,
                          height: 13,
                          child: CircularProgressIndicator(
                            strokeWidth: 2,
                            color: Colors.white,
                          ),
                        ),
                        const SizedBox(width: 6),
                      ],
                      Text(
                        selected,
                        style: const TextStyle(
                          color: Colors.white,
                          fontSize: 13,
                          fontWeight: FontWeight.w700,
                        ),
                      ),
                      const Icon(
                        Icons.expand_more,
                        color: Colors.white,
                        size: 18,
                      ),
                    ],
                  ),
                ),
              ),
            ),
          ),
        ),
      ),
    );
  }

  Widget _glassIconButton({
    required String tooltip,
    required IconData icon,
    required VoidCallback onPressed,
  }) {
    return Tooltip(
      message: tooltip,
      child: ClipOval(
        child: BackdropFilter(
          filter: ImageFilter.blur(sigmaX: 16, sigmaY: 16),
          child: Material(
            color: Colors.white.withValues(alpha: 0.14),
            shape: CircleBorder(
              side: BorderSide(color: Colors.white.withValues(alpha: 0.26)),
            ),
            child: InkWell(
              customBorder: const CircleBorder(),
              onTap: onPressed,
              child: SizedBox(
                width: 36,
                height: 36,
                child: Icon(icon, color: Colors.white, size: 20),
              ),
            ),
          ),
        ),
      ),
    );
  }

  Widget _glassChip(String label) {
    return ClipRRect(
      borderRadius: BorderRadius.circular(18),
      child: BackdropFilter(
        filter: ImageFilter.blur(sigmaX: 16, sigmaY: 16),
        child: DecoratedBox(
          decoration: BoxDecoration(
            color: Colors.white.withValues(alpha: 0.14),
            borderRadius: BorderRadius.circular(18),
            border: Border.all(color: Colors.white.withValues(alpha: 0.26)),
          ),
          child: SizedBox(
            height: 36,
            child: Padding(
              padding: const EdgeInsets.symmetric(horizontal: 10),
              child: Center(
                child: Text(
                  label,
                  style: const TextStyle(
                    color: Colors.white,
                    fontSize: 13,
                    fontWeight: FontWeight.w700,
                  ),
                ),
              ),
            ),
          ),
        ),
      ),
    );
  }

  String get _selectedQualityLabel {
    return _qualities
            .where((quality) => quality.name == _selectedQuality)
            .map((quality) => quality.label)
            .firstOrNull ??
        '自动';
  }

  String _formatRate(double rate) {
    return rate == rate.roundToDouble()
        ? rate.toInt().toString()
        : rate.toStringAsFixed(2).replaceFirst(RegExp(r'0$'), '');
  }
}
