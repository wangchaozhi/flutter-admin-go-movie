import '../../core/api_client.dart';

/// Whether a progress save should actually be sent to the server.
///
/// Pure decision logic, separated so it can be unit tested without a network:
/// - never persist a negative position;
/// - never persist a bare zero position (it would wipe a real resume point)
///   unless the caller explicitly asked for it (e.g. a finished video);
/// - otherwise throttle to meaningful moves of >= 5s, unless [force]d.
bool shouldSaveProgress({
  required Duration position,
  required Duration lastSaved,
  required bool force,
  required bool isExplicitPosition,
}) {
  if (position < Duration.zero) return false;
  if (position == Duration.zero && !isExplicitPosition) return false;
  if (!force && (position - lastSaved).abs() < const Duration(seconds: 5)) {
    return false;
  }
  return true;
}

/// Whether [position] is far enough from the end of a clip of [duration] to be
/// worth resuming from (rather than restarting an essentially-finished video).
/// A non-positive [duration] (unknown length) always allows a resume.
bool canResumeAt(Duration position, Duration duration) {
  if (duration <= Duration.zero) return true;
  return position < duration - const Duration(seconds: 20);
}

/// Loads and persists a single video's playback position, owning the throttle
/// state ([_lastSaved]) and re-entrancy guard ([_saving]) that used to live in
/// the player widget. The [ApiClient] is injectable so the I/O can be faked in
/// tests.
class VideoProgressService {
  VideoProgressService({required this.videoId, ApiClient? client})
    : _client = client ?? ApiClient();

  final int videoId;
  final ApiClient _client;

  Duration _lastSaved = Duration.zero;
  bool _saving = false;

  Duration get lastSaved => _lastSaved;

  /// Returns the saved resume position, or [Duration.zero] when none exists or
  /// anything goes wrong. Progress is a comfort feature, so failures are
  /// swallowed and playback continues from the start.
  Future<Duration> load() async {
    try {
      final resp = await _client.getAuth('/api/videos/$videoId/progress');
      if (resp['code'] != 0) return Duration.zero;
      final data = resp['data'];
      final position = data is Map<String, dynamic> ? data['position'] : null;
      if (position is num && position > 0) {
        return Duration(seconds: position.toInt());
      }
    } catch (_) {
      // Absent/failed progress must never break playback.
    }
    return Duration.zero;
  }

  /// Persists [position]/[duration] subject to [shouldSaveProgress]. Concurrent
  /// calls are ignored while one is in flight, and network blips are swallowed
  /// (the next tick retries).
  Future<void> save(
    Duration position,
    Duration duration, {
    bool force = false,
    bool isExplicitPosition = false,
  }) async {
    if (_saving) return;
    if (!shouldSaveProgress(
      position: position,
      lastSaved: _lastSaved,
      force: force,
      isExplicitPosition: isExplicitPosition,
    )) {
      return;
    }
    _saving = true;
    try {
      await _client.postAuth('/api/videos/$videoId/progress', {
        'position': position.inSeconds,
        'duration': duration.inSeconds,
      });
      _lastSaved = position;
    } catch (_) {
      // Expected during playback; the next tick will retry.
    } finally {
      _saving = false;
    }
  }
}
