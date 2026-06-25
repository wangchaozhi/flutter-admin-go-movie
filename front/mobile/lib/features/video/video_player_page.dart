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

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addObserver(this);
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
    _webSubtitleText.dispose();
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
      if (resumePosition > Duration.zero &&
          canResumeAt(resumePosition, _player.state.duration)) {
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
    var qualities = TrackParser.parseQualities(data, _absoluteUrl);
    if (qualities.isEmpty) {
      qualities = await _parseQualitiesFromMaster(url);
    }
    final audioTracks =
        TrackParser.parseMediaTracks(data, 'audio_tracks', _absoluteUrl);
    final subtitleTracks =
        TrackParser.parseMediaTracks(data, 'subtitle_tracks', _absoluteUrl);
    final audioTrackCount = TrackParser.parseInt(data?['audio_track_count']);
    final subtitleTrackCount =
        TrackParser.parseInt(data?['subtitle_track_count']);
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
    );
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

  Future<void> _saveProgress({
    bool force = false,
    Duration? positionOverride,
  }) {
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

  Widget _buildAudioMenu(List<TrackMenuOption> options) {
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

  Widget _buildSubtitleMenu(List<TrackMenuOption> options) {
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
