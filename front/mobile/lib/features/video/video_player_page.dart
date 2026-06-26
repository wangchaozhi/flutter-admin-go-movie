import 'dart:async';
import 'dart:ui';

import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:http/http.dart' as http;
import 'package:media_kit/media_kit.dart';
import 'package:media_kit_video/media_kit_video.dart';

import '../../core/api_client.dart';
import '../../core/session.dart';
import '../../models/video.dart' as model;
import 'comments_section.dart';
import 'danmaku_overlay.dart';
import 'playback_parsers.dart';
import 'playback_progress_service.dart';
import 'track_options.dart';

class _PlaybackSource {
  final String url;
  final List<QualityOption> qualities;
  final List<MediaTrackOption> audioTracks;
  final List<MediaTrackOption> subtitleTracks;
  final bool mediaTracksScanned;
  final bool hasMultipleAudioTracks;
  final bool hasSubtitleTracks;
  // VIP preview gating: when vipLocked is true the viewer is not a VIP member,
  // so playback is limited to the first previewSeconds before the paywall shows.
  final bool vipLocked;
  final int previewSeconds;

  const _PlaybackSource({
    required this.url,
    required this.qualities,
    required this.audioTracks,
    required this.subtitleTracks,
    required this.mediaTracksScanned,
    required this.hasMultipleAudioTracks,
    required this.hasSubtitleTracks,
    required this.vipLocked,
    required this.previewSeconds,
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
  late model.Video _video;
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
  List<QualityOption> _qualities = const [];
  String? _selectedAudioTrackValue;
  String? _selectedSubtitleTrackValue;
  List<MediaTrackOption> _audioTracks = const [];
  List<MediaTrackOption> _subtitleTracks = const [];
  List<AudioTrack> _hlsAudioTracks = const [];
  List<SubtitleTrack> _hlsSubtitleTracks = const [];
  List<SubtitleCue> _webSubtitleCues = const [];
  // Held in a notifier so a cue change repaints only the subtitle overlay
  // (via ValueListenableBuilder) instead of rebuilding the whole player tree
  // on every position tick.
  final ValueNotifier<String> _webSubtitleText = ValueNotifier<String>('');
  int _webSubtitleLoadSerial = 0;
  bool _mediaTracksScanned = false;
  bool _hasMultipleAudioTracks = false;
  bool _hasSubtitleTracks = false;
  Timer? _progressTimer;
  Timer? _refreshTimer;
  late final VideoProgressService _progressService;
  bool _refreshingSource = false;
  bool _recoveringPlayback = false;
  bool _isFullscreen = false;
  double _playbackRate = 1.0;
  // VIP preview gating. When _vipLocked is set, playback is capped at
  // _previewLimit; reaching it pauses and flips _previewBlocked to show the
  // paywall overlay.
  bool _vipLocked = false;
  Duration _previewLimit = Duration.zero;
  bool _previewBlocked = false;
  // Danmaku (bullet comments) overlay state.
  bool _danmakuVisible = true;
  bool _sendingDanmaku = false;
  int _danmakuColor = 0xFFFFFF;
  final DanmakuController _danmakuController = DanmakuController();
  final TextEditingController _danmakuInput = TextEditingController();

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addObserver(this);
    _video = widget.video;
    _progressService = VideoProgressService(videoId: widget.video.id);
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
      _onPositionChanged,
    );
    SystemChrome.setPreferredOrientations([
      DeviceOrientation.landscapeLeft,
      DeviceOrientation.landscapeRight,
      DeviceOrientation.portraitUp,
    ]);
    unawaited(_loadVideoDetail());
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
    _webSubtitleText.dispose();
    _danmakuInput.dispose();
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
        _previewBlocked = false;
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
          _vipLocked = source.vipLocked;
          _previewLimit = source.vipLocked && source.previewSeconds > 0
              ? Duration(seconds: source.previewSeconds)
              : Duration.zero;
          _previewBlocked = false;
          _hlsAudioTracks = const [];
          _hlsSubtitleTracks = const [];
          _webSubtitleCues = const [];
          _webSubtitleText.value = '';
          _webSubtitleLoadSerial++;
          _selectedQuality = 'auto';
          _selectedAudioTrackValue = null;
          _selectedSubtitleTrackValue = null;
        });
      }
      final resumePosition = await _progressService.load();
      await _player.open(Media(source.url), play: false);
      await _player.setRate(_playbackRate);
      await _waitForMediaReady();
      _syncDetectedTracks(_player.state.tracks);
      await _applySelectedMediaTracks();
      // Don't resume past the preview window for a locked video.
      if (resumePosition > Duration.zero &&
          canResumeAt(resumePosition, _player.state.duration) &&
          !_isBeyondPreview(resumePosition)) {
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
      throw Exception(resp['msg']?.toString() ?? '播放地址获取失败');
    }
    final data = resp['data'] as Map<String, dynamic>?;
    final rawUrl = data?['url'] as String? ?? '';
    if (rawUrl.isEmpty) {
      throw Exception('暂时没有可用的播放地址');
    }
    // backend returns a relative signed path; prepend baseUrl so it works on
    // emulator (10.0.2.2), simulator (localhost), and physical device (LAN IP)
    final url = _absoluteUrl(rawUrl);
    var qualities = TrackParser.parseQualities(data, _absoluteUrl);
    if (qualities.isEmpty) {
      qualities = await _parseQualitiesFromMaster(url);
    }
    final audioTracks = TrackParser.parseMediaTracks(
      data,
      'audio_tracks',
      _absoluteUrl,
    );
    final subtitleTracks = TrackParser.parseMediaTracks(
      data,
      'subtitle_tracks',
      _absoluteUrl,
    );
    final audioTrackCount = TrackParser.parseInt(data?['audio_track_count']);
    final subtitleTrackCount = TrackParser.parseInt(
      data?['subtitle_track_count'],
    );
    final mediaTracksScanned = data?['media_tracks_scanned'] == true;
    return _PlaybackSource(
      url: url,
      qualities: [
        QualityOption(name: 'auto', label: '自动', url: url),
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
      vipLocked: data?['vip_locked'] == true,
      previewSeconds: TrackParser.parseInt(data?['preview_seconds']),
    );
  }

  Future<void> _loadVideoDetail() async {
    try {
      final resp = await ApiClient().get('/api/videos/${widget.video.id}');
      if (resp['code'] != 0 || resp['data'] is! Map<String, dynamic>) return;
      final next = model.Video.fromJson(resp['data'] as Map<String, dynamic>);
      if (mounted) {
        setState(() => _video = next);
      }
    } catch (_) {
      // Playback should not fail just because the detail module could not load.
    }
  }

  String _absoluteUrl(String url) {
    return url.startsWith('http') ? url : '${ApiClient.baseUrl}$url';
  }

  Future<List<QualityOption>> _parseQualitiesFromMaster(String url) async {
    try {
      final response = await http.get(Uri.parse(url));
      if (response.statusCode < 200 || response.statusCode >= 300) {
        return const [];
      }
      return HlsMasterParser.parse(response.body, Uri.parse(url))
          .map(
            (variant) => QualityOption(
              name: variant.name,
              label: variant.label,
              url: _absoluteUrl(variant.url),
            ),
          )
          .toList();
    } catch (_) {
      return const [];
    }
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

    final audioTrack = TrackMenuBuilder.resolveAudioTrack(
      audioOption,
      isWeb: kIsWeb,
    );
    if (audioTrack != null) {
      await _player.setAudioTrack(audioTrack);
    }

    if (kIsWeb) {
      await _applyWebSubtitleTrack(subtitleOption);
    } else {
      await _player.setSubtitleTrack(
        TrackMenuBuilder.resolveNativeSubtitleTrack(subtitleOption),
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
        ).showSnackBar(SnackBar(content: Text('音轨切换失败：$e')));
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
        ).showSnackBar(SnackBar(content: Text('字幕切换失败：$e')));
      }
      await _applySelectedMediaTracks();
    } finally {
      if (mounted) setState(() => _switchingTrack = false);
    }
  }

  Future<void> _applyWebSubtitleTrack(TrackMenuOption? option) async {
    final serial = ++_webSubtitleLoadSerial;
    _setWebSubtitleCues(const []);
    if (option == null) return;

    final apiTrack = option.apiTrack;
    if (apiTrack == null) return;

    final response = await http.get(Uri.parse(apiTrack.url));
    if (response.statusCode < 200 || response.statusCode >= 300) {
      throw Exception('subtitle http ${response.statusCode}');
    }
    final cues = WebVttParser.parse(response.body);
    if (serial != _webSubtitleLoadSerial) return;
    _setWebSubtitleCues(cues);
    _syncWebSubtitleCue(_player.state.position);
  }

  void _setWebSubtitleCues(List<SubtitleCue> cues) {
    if (!kIsWeb) return;
    _webSubtitleCues = cues;
    // The notifier ignores no-op writes, so the overlay only repaints when the
    // visible cue actually changes. Guard against writing after dispose().
    if (mounted) _webSubtitleText.value = '';
  }

  void _syncWebSubtitleCue(Duration position) {
    if (!kIsWeb || !mounted) return;
    final cue = _webSubtitleCues
        .where((item) => position >= item.start && position <= item.end)
        .firstOrNull;
    _webSubtitleText.value = cue?.text ?? '';
  }

  void _onPositionChanged(Duration position) {
    _syncWebSubtitleCue(position);
    _enforcePreviewLimit(position);
  }

  bool _isBeyondPreview(Duration position) {
    return _vipLocked &&
        _previewLimit > Duration.zero &&
        position >= _previewLimit;
  }

  // Caps a locked video at the preview window: once the boundary is reached the
  // player pauses, snaps back to the limit, and the paywall overlay appears.
  void _enforcePreviewLimit(Duration position) {
    if (!_vipLocked || _previewLimit <= Duration.zero) return;
    if (position < _previewLimit) {
      if (_previewBlocked && mounted) {
        setState(() => _previewBlocked = false);
      }
      return;
    }
    unawaited(_player.pause());
    if (position > _previewLimit) {
      unawaited(_player.seek(_previewLimit));
    }
    if (!_previewBlocked && mounted) {
      setState(() => _previewBlocked = true);
    }
  }

  Future<void> _openVipUpgrade() async {
    await _player.pause();
    if (!mounted) return;
    await Navigator.pushNamed(context, '/vip');
    if (!mounted) return;
    // Membership may have changed; re-fetch the source to lift the gate.
    await _init();
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
    QualityOption option,
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
    if (_isBeyondPreview(_player.state.position) && !_player.state.completed) {
      if (mounted) setState(() => _previewBlocked = true);
      return;
    }
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
    final audioTracks = TrackMenuBuilder.selectableHlsAudio(tracks.audio);
    final subtitleTracks = TrackMenuBuilder.selectableHlsSubtitle(
      tracks.subtitle,
    );
    final nextAudioValue =
        TrackMenuBuilder.valueExists(
          _selectedAudioTrackValue,
          TrackMenuBuilder.audioOptions(
            hlsTracks: audioTracks,
            apiTracks: _audioTracks,
            mediaTracksScanned: _mediaTracksScanned,
            hasMultipleAudioTracks: _hasMultipleAudioTracks,
          ),
        )
        ? _selectedAudioTrackValue
        : null;
    final nextSubtitleValue =
        TrackMenuBuilder.valueExists(
          _selectedSubtitleTrackValue,
          TrackMenuBuilder.subtitleOptions(
            hlsTracks: subtitleTracks,
            apiTracks: _subtitleTracks,
            mediaTracksScanned: _mediaTracksScanned,
            hasSubtitleTracks: _hasSubtitleTracks,
          ),
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

  List<TrackMenuOption> _audioMenuOptions() {
    return TrackMenuBuilder.audioOptions(
      hlsTracks: _hlsAudioTracks,
      apiTracks: _audioTracks,
      mediaTracksScanned: _mediaTracksScanned,
      hasMultipleAudioTracks: _hasMultipleAudioTracks,
    );
  }

  List<TrackMenuOption> _subtitleMenuOptions() {
    return TrackMenuBuilder.subtitleOptions(
      hlsTracks: _hlsSubtitleTracks,
      apiTracks: _subtitleTracks,
      mediaTracksScanned: _mediaTracksScanned,
      hasSubtitleTracks: _hasSubtitleTracks,
    );
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

  Future<void> _saveProgress({bool force = false, Duration? positionOverride}) {
    final position = positionOverride ?? _player.state.position;
    return _progressService.save(
      position,
      _player.state.duration,
      force: force,
      isExplicitPosition: positionOverride != null,
    );
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
                            _video.title,
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
                    const SizedBox(height: 12),
                    _buildDanmakuBar(),
                    const SizedBox(height: 12),
                    _buildVideoInfoSection(),
                    CommentsSection(videoId: _video.id),
                  ],
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildVideoInfoSection() {
    final aiMetadata = _video.aiMetadata;
    final synopsis = (aiMetadata?.synopsis.trim().isNotEmpty ?? false)
        ? aiMetadata!.synopsis.trim()
        : _video.description.trim().isNotEmpty
        ? _video.description.trim()
        : '暂时没有详细简介，可以先查看下方分类、演员和推荐看点。';
    final highlights = aiMetadata?.highlights ?? const <String>[];
    final tags = aiMetadata?.tags ?? const <String>[];

    return DecoratedBox(
      decoration: BoxDecoration(
        color: const Color(0xFF0B0F14),
        borderRadius: BorderRadius.circular(8),
        border: Border.all(color: const Color(0xFF1F2937)),
      ),
      child: Padding(
        padding: const EdgeInsets.all(14),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Wrap(
              spacing: 8,
              runSpacing: 8,
              children: [
                if (_video.categoryName.isNotEmpty)
                  _buildMetaChip(
                    Icons.local_movies_rounded,
                    _video.categoryName,
                  ),
                if (_video.durationLabel.isNotEmpty)
                  _buildMetaChip(Icons.schedule_rounded, _video.durationLabel),
                if (_video.region.isNotEmpty)
                  _buildMetaChip(Icons.public_rounded, _video.region),
                if (_video.releaseYear > 0)
                  _buildMetaChip(
                    Icons.calendar_month_rounded,
                    _video.releaseYear.toString(),
                  ),
                if (_video.language.isNotEmpty)
                  _buildMetaChip(Icons.translate_rounded, _video.language),
                if (_video.genres.isNotEmpty)
                  _buildMetaChip(
                    Icons.category_rounded,
                    _video.genres.take(3).join(' / '),
                  ),
                _buildMetaChip(
                  _video.isVip && !_video.isFree
                      ? Icons.workspace_premium_rounded
                      : Icons.play_circle_outline_rounded,
                  _video.isVip && !_video.isFree ? '会员专属' : '免费可看',
                ),
              ],
            ),
            if (_video.directors.isNotEmpty || _video.actors.isNotEmpty) ...[
              const SizedBox(height: 14),
              if (_video.directors.isNotEmpty)
                _buildInfoLine('导演', _video.directors.join('、')),
              if (_video.actors.isNotEmpty)
                _buildInfoLine('主演', _video.actors.join('、')),
            ],
            const SizedBox(height: 14),
            const Text(
              '影片简介',
              style: TextStyle(
                color: Colors.white,
                fontSize: 16,
                fontWeight: FontWeight.w800,
              ),
            ),
            const SizedBox(height: 8),
            Text(
              synopsis,
              style: const TextStyle(
                color: Color(0xFFD1D5DB),
                fontSize: 14,
                height: 1.55,
              ),
            ),
            if (highlights.isNotEmpty) ...[
              const SizedBox(height: 16),
              const Text(
                '推荐看点',
                style: TextStyle(
                  color: Colors.white,
                  fontSize: 15,
                  fontWeight: FontWeight.w800,
                ),
              ),
              const SizedBox(height: 8),
              ...highlights.map(_buildHighlightItem),
            ],
            if (tags.isNotEmpty) ...[
              const SizedBox(height: 14),
              Wrap(
                spacing: 8,
                runSpacing: 8,
                children: tags.map(_buildTagChip).toList(),
              ),
            ],
          ],
        ),
      ),
    );
  }

  Widget _buildMetaChip(IconData icon, String label) {
    return DecoratedBox(
      decoration: BoxDecoration(
        color: const Color(0xFF111827),
        borderRadius: BorderRadius.circular(6),
        border: Border.all(color: const Color(0xFF243244)),
      ),
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: 9, vertical: 6),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(icon, size: 15, color: const Color(0xFF25D0AB)),
            const SizedBox(width: 5),
            Text(
              label,
              style: const TextStyle(
                color: Color(0xFFE5E7EB),
                fontSize: 12,
                fontWeight: FontWeight.w700,
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildInfoLine(String label, String value) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 7),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          SizedBox(
            width: 42,
            child: Text(
              label,
              style: const TextStyle(
                color: Color(0xFF6B7280),
                fontSize: 13,
                fontWeight: FontWeight.w700,
              ),
            ),
          ),
          Expanded(
            child: Text(
              value,
              style: const TextStyle(
                color: Color(0xFFE5E7EB),
                fontSize: 13,
                height: 1.35,
              ),
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildHighlightItem(String text) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 7),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const Icon(
            Icons.check_circle_rounded,
            size: 16,
            color: Color(0xFF25D0AB),
          ),
          const SizedBox(width: 8),
          Expanded(
            child: Text(
              text,
              style: const TextStyle(
                color: Color(0xFFC7D2FE),
                fontSize: 13,
                height: 1.35,
              ),
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildTagChip(String tag) {
    return DecoratedBox(
      decoration: BoxDecoration(
        color: const Color(0xFF101820),
        borderRadius: BorderRadius.circular(6),
      ),
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: 9, vertical: 5),
        child: Text(
          '#$tag',
          style: const TextStyle(
            color: Color(0xFF9CA3AF),
            fontSize: 12,
            fontWeight: FontWeight.w700,
          ),
        ),
      ),
    );
  }

  Widget _buildPlayer() {
    return Stack(
      children: [
        Video(controller: _controller, controls: AdaptiveVideoControls),
        if (_danmakuVisible && !_previewBlocked)
          Positioned.fill(
            child: DanmakuOverlay(
              videoId: widget.video.id,
              player: _player,
              enabled: _danmakuVisible,
              controller: _danmakuController,
            ),
          ),
        _buildBufferingIndicator(),
        _buildCenterPlayButton(),
        _buildCompletedOverlay(),
        _buildWebSubtitleOverlay(),
        Positioned(left: 48, bottom: 4, child: _buildPlaybackTools()),
        Positioned(right: 48, bottom: 4, child: _buildTrackAndQualityMenus()),
        Positioned(right: 8, top: 8, child: _buildFullscreenButton()),
        Positioned(right: 8, top: 52, child: _buildDanmakuToggle()),
        Positioned(right: 8, top: 96, child: _buildDanmakuListButton()),
        if (_vipLocked && !_previewBlocked)
          Positioned(left: 8, top: 8, child: _buildPreviewBadge()),
        if (_previewBlocked) _buildVipPaywallOverlay(),
      ],
    );
  }

  // Small "VIP 试看" pill shown during the preview window; tapping it opens the
  // upgrade page (the "开通VIP 图标" the viewer sees while previewing).
  Widget _buildPreviewBadge() {
    return Material(
      color: Colors.transparent,
      child: InkWell(
        borderRadius: BorderRadius.circular(20),
        onTap: () => unawaited(_openVipUpgrade()),
        child: ClipRRect(
          borderRadius: BorderRadius.circular(20),
          child: BackdropFilter(
            filter: ImageFilter.blur(sigmaX: 14, sigmaY: 14),
            child: DecoratedBox(
              decoration: BoxDecoration(
                color: const Color(0xCC1B1300),
                borderRadius: BorderRadius.circular(20),
                border: Border.all(color: const Color(0xFFF7C948)),
              ),
              child: Padding(
                padding: const EdgeInsets.symmetric(
                  horizontal: 10,
                  vertical: 6,
                ),
                child: Row(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    const Icon(
                      Icons.workspace_premium_rounded,
                      color: Color(0xFFF7C948),
                      size: 16,
                    ),
                    const SizedBox(width: 5),
                    StreamBuilder<Duration>(
                      stream: _player.stream.position,
                      initialData: _player.state.position,
                      builder: (context, snapshot) {
                        final remaining =
                            _previewLimit - (snapshot.data ?? Duration.zero);
                        final label = remaining > Duration.zero
                            ? '试看剩余 ${_formatRemaining(remaining)}'
                            : '会员试看';
                        return Text(
                          label,
                          style: const TextStyle(
                            color: Color(0xFFF7C948),
                            fontSize: 12,
                            fontWeight: FontWeight.w800,
                          ),
                        );
                      },
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

  Widget _buildVipPaywallOverlay() {
    return Positioned.fill(
      // Opaque barrier so taps can't reach the player controls behind the gate.
      child: GestureDetector(
        behavior: HitTestBehavior.opaque,
        onTap: () {},
        child: DecoratedBox(
          decoration: const BoxDecoration(
            gradient: LinearGradient(
              colors: [Color(0xE6000000), Color(0xF20B0F14)],
              begin: Alignment.topCenter,
              end: Alignment.bottomCenter,
            ),
          ),
          child: Center(
            child: Padding(
              padding: const EdgeInsets.symmetric(horizontal: 24),
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  const Icon(
                    Icons.workspace_premium_rounded,
                    color: Color(0xFFF7C948),
                    size: 44,
                  ),
                  const SizedBox(height: 12),
                  const Text(
                    '会员专属内容',
                    style: TextStyle(
                      color: Colors.white,
                      fontSize: 18,
                      fontWeight: FontWeight.w900,
                    ),
                  ),
                  const SizedBox(height: 8),
                  Text(
                    '试看已结束，开通会员后可继续观看完整影片',
                    textAlign: TextAlign.center,
                    style: const TextStyle(
                      color: Color(0xFFD1D5DB),
                      fontSize: 13,
                      height: 1.4,
                    ),
                  ),
                  const SizedBox(height: 18),
                  FilledButton.icon(
                    onPressed: () => unawaited(_openVipUpgrade()),
                    style: FilledButton.styleFrom(
                      backgroundColor: const Color(0xFFF7C948),
                      foregroundColor: const Color(0xFF101318),
                      padding: const EdgeInsets.symmetric(
                        horizontal: 22,
                        vertical: 12,
                      ),
                      shape: RoundedRectangleBorder(
                        borderRadius: BorderRadius.circular(8),
                      ),
                    ),
                    icon: const Icon(Icons.lock_open_rounded, size: 18),
                    label: const Text(
                      '开通会员',
                      style: TextStyle(fontWeight: FontWeight.w900),
                    ),
                  ),
                  const SizedBox(height: 10),
                  TextButton(
                    onPressed: () => Navigator.pop(context),
                    child: const Text(
                      '先返回',
                      style: TextStyle(color: Color(0xFF9CA3AF)),
                    ),
                  ),
                ],
              ),
            ),
          ),
        ),
      ),
    );
  }

  String _formatRemaining(Duration remaining) {
    final totalSeconds = remaining.inSeconds;
    final minutes = totalSeconds ~/ 60;
    final seconds = totalSeconds % 60;
    return '$minutes:${seconds.toString().padLeft(2, '0')}';
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
            !_previewBlocked &&
            (_showResumePlaybackButton || (!isPlaying && !_switchingQuality));
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
    if (!kIsWeb) return const SizedBox.shrink();
    return Positioned(
      left: 24,
      right: 24,
      bottom: 54,
      child: IgnorePointer(
        child: Center(
          child: ValueListenableBuilder<String>(
            valueListenable: _webSubtitleText,
            builder: (context, text, _) {
              if (text.isEmpty) return const SizedBox.shrink();
              return DecoratedBox(
                decoration: BoxDecoration(
                  color: Colors.black.withValues(alpha: 0.62),
                  borderRadius: BorderRadius.circular(6),
                ),
                child: Padding(
                  padding: const EdgeInsets.symmetric(
                    horizontal: 12,
                    vertical: 7,
                  ),
                  child: Text(
                    text,
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
              );
            },
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
          tooltip: '后退 10 秒',
          icon: Icons.replay_10_rounded,
          onPressed: () =>
              unawaited(_seekRelative(const Duration(seconds: -10))),
        ),
        const SizedBox(width: 8),
        _buildSpeedMenu(),
        const SizedBox(width: 8),
        _glassIconButton(
          tooltip: '前进 10 秒',
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
      tooltip: _isFullscreen ? '退出全屏' : '全屏播放',
      icon: _isFullscreen
          ? Icons.fullscreen_exit_rounded
          : Icons.fullscreen_rounded,
      onPressed: () => unawaited(_toggleFullscreen()),
    );
  }

  Widget _buildDanmakuToggle() {
    return _glassIconButton(
      tooltip: _danmakuVisible ? '关闭弹幕' : '开启弹幕',
      icon: _danmakuVisible
          ? Icons.subtitles_rounded
          : Icons.subtitles_off_rounded,
      onPressed: () => setState(() => _danmakuVisible = !_danmakuVisible),
    );
  }

  Widget _buildDanmakuListButton() {
    return _glassIconButton(
      tooltip: '弹幕列表',
      icon: Icons.format_list_bulleted_rounded,
      onPressed: () => unawaited(showDanmakuList(context, _danmakuController)),
    );
  }

  // Preset danmaku colours users can pick from when sending a bullet.
  static const List<int> _danmakuPalette = [
    0xFFFFFF, // white
    0xFF4D4F, // red
    0xFFD93D, // yellow
    0x25D0AB, // teal (app accent)
    0x4DA3FF, // blue
    0xFF7AC6, // pink
  ];

  Widget _buildDanmakuBar() {
    return DecoratedBox(
      decoration: BoxDecoration(
        color: const Color(0xFF171B24),
        borderRadius: BorderRadius.circular(24),
        border: Border.all(color: const Color(0xFF2B3140)),
      ),
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 2),
        child: Row(
          children: [
            _buildDanmakuColorButton(),
            Expanded(
              child: TextField(
                controller: _danmakuInput,
                maxLength: 100,
                style: const TextStyle(color: Colors.white, fontSize: 14),
                cursorColor: const Color(0xFF25D0AB),
                textInputAction: TextInputAction.send,
                onSubmitted: (_) => unawaited(_sendDanmaku()),
                decoration: const InputDecoration(
                  hintText: '发个友善的弹幕…',
                  hintStyle: TextStyle(color: Color(0xFF9CA3AF), fontSize: 14),
                  border: InputBorder.none,
                  counterText: '',
                  isDense: true,
                ),
              ),
            ),
            _sendingDanmaku
                ? const Padding(
                    padding: EdgeInsets.all(8),
                    child: SizedBox(
                      width: 18,
                      height: 18,
                      child: CircularProgressIndicator(
                        strokeWidth: 2,
                        color: Color(0xFF25D0AB),
                      ),
                    ),
                  )
                : IconButton(
                    icon: const Icon(Icons.send_rounded, color: Color(0xFF25D0AB)),
                    onPressed: () => unawaited(_sendDanmaku()),
                  ),
          ],
        ),
      ),
    );
  }

  Widget _buildDanmakuColorButton() {
    return PopupMenuButton<int>(
      tooltip: '弹幕颜色',
      color: const Color(0xFF111827),
      initialValue: _danmakuColor,
      onSelected: (color) => setState(() => _danmakuColor = color),
      itemBuilder: (context) => _danmakuPalette
          .map(
            (color) => PopupMenuItem<int>(
              value: color,
              child: Row(
                children: [
                  Container(
                    width: 18,
                    height: 18,
                    decoration: BoxDecoration(
                      color: Color(0xFF000000 | color),
                      shape: BoxShape.circle,
                      border: Border.all(color: Colors.white24),
                    ),
                  ),
                  const SizedBox(width: 10),
                  if (color == _danmakuColor)
                    const Icon(Icons.check, color: Color(0xFF25D0AB), size: 16),
                ],
              ),
            ),
          )
          .toList(),
      child: Padding(
        padding: const EdgeInsets.all(8),
        child: Container(
          width: 20,
          height: 20,
          decoration: BoxDecoration(
            color: Color(0xFF000000 | _danmakuColor),
            shape: BoxShape.circle,
            border: Border.all(color: Colors.white38, width: 2),
          ),
        ),
      ),
    );
  }

  Future<void> _sendDanmaku() async {
    final text = _danmakuInput.text.trim();
    if (text.isEmpty) {
      _snack('请输入弹幕内容');
      return;
    }
    if (text.runes.length > 100) {
      _snack('弹幕最多 100 字');
      return;
    }
    final token = await Session.token();
    if (token == null) {
      _snack('发弹幕需要先登录');
      return;
    }
    setState(() => _sendingDanmaku = true);
    final timeMs = _player.state.position.inMilliseconds;
    try {
      final resp = await ApiClient().postAuth(
        '/api/videos/${widget.video.id}/danmaku',
        {
          'content': text,
          'time_ms': timeMs,
          'color': _danmakuColor,
          'mode': 0,
        },
      );
      if (!mounted) return;
      if (resp['code'] == 0) {
        _danmakuInput.clear();
        if (!_danmakuVisible) {
          setState(() => _danmakuVisible = true);
        }
        // Build the local bullet from the server response so it carries a real
        // id/user_id and can be liked or deleted right away.
        final data = resp['data'] as Map<String, dynamic>?;
        final bullet = data != null
            ? DanmakuItem.fromJson(data)
            : DanmakuItem(
                id: 0,
                userId: 0,
                content: text,
                timeMs: timeMs,
                color: _danmakuColor,
                mode: 0,
              );
        _danmakuController.addLocal(bullet);
      } else {
        _snack(resp['msg']?.toString() ?? '弹幕发送失败');
      }
    } catch (_) {
      if (mounted) _snack('弹幕发送失败，请稍后再试');
    } finally {
      if (mounted) setState(() => _sendingDanmaku = false);
    }
  }

  void _snack(String message) {
    if (!mounted) return;
    ScaffoldMessenger.of(
      context,
    ).showSnackBar(SnackBar(content: Text(message)));
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

  Widget _buildAudioMenu(List<TrackMenuOption> options) {
    final selectedValue = _selectedAudioTrackValue ?? 'auto';
    return PopupMenuButton<String>(
      enabled: !_switchingTrack,
      tooltip: '音轨',
      color: const Color(0xFF111827),
      initialValue: selectedValue,
      onSelected: (value) => unawaited(_switchAudioTrack(value)),
      itemBuilder: (context) => [
        PopupMenuItem<String>(
          value: 'auto',
          child: _buildTrackMenuItem('默认音轨', selectedValue == 'auto'),
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

  Widget _buildSubtitleMenu(List<TrackMenuOption> options) {
    final selectedValue = _selectedSubtitleTrackValue ?? 'off';
    return PopupMenuButton<String>(
      enabled: !_switchingTrack,
      tooltip: '字幕',
      color: const Color(0xFF111827),
      initialValue: selectedValue,
      onSelected: (value) => unawaited(_switchSubtitleTrack(value)),
      itemBuilder: (context) => [
        PopupMenuItem<String>(
          value: 'off',
          child: _buildTrackMenuItem('关闭字幕', selectedValue == 'off'),
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
