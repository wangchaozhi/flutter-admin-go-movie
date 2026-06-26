import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter/scheduler.dart';
import 'package:media_kit/media_kit.dart';

import '../../core/api_client.dart';

/// One bullet comment anchored to a playback position. Mirrors the backend
/// `store.VideoDanmaku` JSON shape.
class DanmakuItem {
  final String content;
  final int timeMs;
  final int color; // 24-bit RGB, e.g. 0xFFFFFF
  final int mode; // 0 scroll, 1 top, 2 bottom

  const DanmakuItem({
    required this.content,
    required this.timeMs,
    required this.color,
    required this.mode,
  });

  factory DanmakuItem.fromJson(Map<String, dynamic> json) {
    return DanmakuItem(
      content: (json['content'] ?? '').toString(),
      timeMs: (json['time_ms'] as num?)?.toInt() ?? 0,
      color: (json['color'] as num?)?.toInt() ?? 0xFFFFFF,
      mode: (json['mode'] as num?)?.toInt() ?? 0,
    );
  }

  Color get flutterColor => Color(0xFF000000 | (color & 0xFFFFFF));
}

/// Bridges the player page to the overlay so a freshly-sent bullet appears
/// immediately without re-fetching the whole list.
class DanmakuController {
  _DanmakuOverlayState? _state;

  void _attach(_DanmakuOverlayState state) => _state = state;
  void _detach(_DanmakuOverlayState state) {
    if (_state == state) _state = null;
  }

  /// Inject a locally-authored bullet so the sender sees it right away.
  void addLocal(DanmakuItem item) => _state?._spawnNow(item);
}

/// Renders danmaku over the player. Bullets are spawned as playback passes their
/// timestamp, animated by an internal clock that only advances while the video
/// is playing (so pausing freezes them in place), and drawn by a single
/// [CustomPainter] for efficiency.
class DanmakuOverlay extends StatefulWidget {
  final int videoId;
  final Player player;
  final bool enabled;
  final DanmakuController controller;

  const DanmakuOverlay({
    super.key,
    required this.videoId,
    required this.player,
    required this.enabled,
    required this.controller,
  });

  @override
  State<DanmakuOverlay> createState() => _DanmakuOverlayState();
}

class _DanmakuOverlayState extends State<DanmakuOverlay>
    with SingleTickerProviderStateMixin {
  static const double _fontSize = 15;
  static const double _laneHeight = 26;
  static const double _laneGap = 24; // min horizontal gap between scroll bullets
  static const double _fixedDurationMs = 4500;

  late final Ticker _ticker;
  final ValueNotifier<double> _clock = ValueNotifier<double>(0);

  // Danmu clock in ms; advances only while the video is playing.
  double _danmuClockMs = 0;
  Duration _lastElapsed = Duration.zero;
  bool _playing = false;

  List<DanmakuItem> _bullets = const [];
  int _nextIndex = 0;
  int _lastPosMs = 0;

  final List<_ActiveBullet> _active = [];
  // Per-lane last scroll bullet, for collision avoidance.
  final Map<int, _ActiveBullet> _scrollLaneLast = {};
  // Occupied-until clock per fixed (top/bottom) lane.
  final Map<int, double> _topLaneUntil = {};
  final Map<int, double> _bottomLaneUntil = {};

  StreamSubscription<Duration>? _posSub;
  StreamSubscription<bool>? _playingSub;

  Size _size = Size.zero;

  @override
  void initState() {
    super.initState();
    widget.controller._attach(this);
    _ticker = createTicker(_onTick)..start();
    _playing = widget.player.state.playing;
    _posSub = widget.player.stream.position.listen(_onPosition);
    _playingSub = widget.player.stream.playing.listen((p) => _playing = p);
    unawaited(_load());
  }

  @override
  void didUpdateWidget(covariant DanmakuOverlay oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.controller != widget.controller) {
      oldWidget.controller._detach(this);
      widget.controller._attach(this);
    }
    if (oldWidget.videoId != widget.videoId) {
      _reset();
      unawaited(_load());
    }
  }

  @override
  void dispose() {
    widget.controller._detach(this);
    _ticker.dispose();
    _posSub?.cancel();
    _playingSub?.cancel();
    _clock.dispose();
    super.dispose();
  }

  Future<void> _load() async {
    try {
      final resp = await ApiClient().get('/api/videos/${widget.videoId}/danmaku');
      if (resp['code'] != 0) return;
      final data = resp['data'] as Map<String, dynamic>?;
      final rawItems = (data?['items'] as List?) ?? const [];
      final items = rawItems
          .whereType<Map<String, dynamic>>()
          .map(DanmakuItem.fromJson)
          .where((d) => d.content.isNotEmpty)
          .toList()
        ..sort((a, b) => a.timeMs.compareTo(b.timeMs));
      if (!mounted) return;
      _bullets = items;
      _seekCursorTo(_lastPosMs);
    } catch (_) {
      // Danmaku is non-essential; a load failure must not break playback.
    }
  }

  void _reset() {
    _bullets = const [];
    _nextIndex = 0;
    _active.clear();
    _scrollLaneLast.clear();
    _topLaneUntil.clear();
    _bottomLaneUntil.clear();
  }

  // Binary-search the first bullet at or after posMs so seeking jumps the cursor
  // instead of replaying everything from the start.
  void _seekCursorTo(int posMs) {
    int lo = 0, hi = _bullets.length;
    while (lo < hi) {
      final mid = (lo + hi) >> 1;
      if (_bullets[mid].timeMs < posMs) {
        lo = mid + 1;
      } else {
        hi = mid;
      }
    }
    _nextIndex = lo;
  }

  void _onPosition(Duration position) {
    final posMs = position.inMilliseconds;
    // Detect a seek (jump back, or a large jump forward) and resync the cursor,
    // clearing bullets that no longer belong to the new position.
    if (posMs < _lastPosMs - 1000 || posMs > _lastPosMs + 4000) {
      _active.clear();
      _scrollLaneLast.clear();
      _topLaneUntil.clear();
      _bottomLaneUntil.clear();
      _seekCursorTo(posMs);
    } else {
      while (_nextIndex < _bullets.length &&
          _bullets[_nextIndex].timeMs <= posMs) {
        _spawn(_bullets[_nextIndex]);
        _nextIndex++;
      }
    }
    _lastPosMs = posMs;
  }

  void _onTick(Duration elapsed) {
    final deltaMs = (elapsed - _lastElapsed).inMilliseconds;
    _lastElapsed = elapsed;
    if (_playing && deltaMs > 0) {
      _danmuClockMs += deltaMs;
    }
    // Drop bullets that have left the screen.
    if (_active.isNotEmpty) {
      _active.removeWhere((b) => b.isExpired(_danmuClockMs, _size.width));
    }
    _clock.value = _danmuClockMs;
  }

  /// Spawn a bullet sent right now by this viewer, regardless of playback time.
  void _spawnNow(DanmakuItem item) => _spawn(item);

  void _spawn(DanmakuItem item) {
    if (_size == Size.zero) return;
    final painter = TextPainter(
      text: TextSpan(text: item.content, style: _bulletStyle(item.flutterColor)),
      textDirection: TextDirection.ltr,
    )..layout();
    final width = painter.width;

    if (item.mode == 1 || item.mode == 2) {
      final lane = _pickFixedLane(item.mode == 1);
      if (lane < 0) return; // no room; drop this fixed bullet
      _active.add(_ActiveBullet.fixed(
        painter: painter,
        width: width,
        mode: item.mode,
        lane: lane,
        startClock: _danmuClockMs,
      ));
      return;
    }

    final lane = _pickScrollLane(width);
    final bullet = _ActiveBullet.scroll(
      painter: painter,
      width: width,
      lane: lane,
      startClock: _danmuClockMs,
    );
    _scrollLaneLast[lane] = bullet;
    _active.add(bullet);
  }

  int _laneCount() {
    final usable = (_size.height * 0.7) - 8; // keep bullets off the controls
    final n = (usable / _laneHeight).floor();
    return n.clamp(1, 12);
  }

  int _pickScrollLane(double width) {
    final lanes = _laneCount();
    // Prefer a lane whose last bullet has scrolled far enough to avoid overlap.
    int best = 0;
    double bestProgress = -1;
    for (int lane = 0; lane < lanes; lane++) {
      final last = _scrollLaneLast[lane];
      if (last == null) return lane;
      final progress = last.scrollProgress(_danmuClockMs);
      // Lane is clear once the previous bullet's tail has crossed the right edge.
      final needed = (last.width + _laneGap) / (_size.width + last.width);
      if (progress >= needed) return lane;
      if (progress > bestProgress) {
        bestProgress = progress;
        best = lane;
      }
    }
    // All lanes busy: reuse the most-advanced one (slight overlap beats dropping).
    return best;
  }

  int _pickFixedLane(bool top) {
    final lanes = (_laneCount() / 2).floor().clamp(1, 6);
    final occ = top ? _topLaneUntil : _bottomLaneUntil;
    for (int lane = 0; lane < lanes; lane++) {
      final until = occ[lane] ?? 0;
      if (_danmuClockMs >= until) {
        occ[lane] = _danmuClockMs + _fixedDurationMs;
        return lane;
      }
    }
    return -1;
  }

  TextStyle _bulletStyle(Color color) {
    return TextStyle(
      color: color,
      fontSize: _fontSize,
      fontWeight: FontWeight.w600,
      shadows: const [
        Shadow(blurRadius: 2, color: Colors.black, offset: Offset(0, 1)),
        Shadow(blurRadius: 2, color: Colors.black, offset: Offset(1, 0)),
      ],
    );
  }

  @override
  Widget build(BuildContext context) {
    if (!widget.enabled) return const SizedBox.shrink();
    return IgnorePointer(
      child: LayoutBuilder(
        builder: (context, constraints) {
          _size = Size(constraints.maxWidth, constraints.maxHeight);
          return CustomPaint(
            size: _size,
            painter: _DanmakuPainter(
              repaint: _clock,
              active: _active,
              clockProvider: () => _danmuClockMs,
            ),
          );
        },
      ),
    );
  }
}

class _ActiveBullet {
  final TextPainter painter;
  final double width;
  final int mode; // 0 scroll, 1 top, 2 bottom
  final int lane;
  final double startClock;

  _ActiveBullet.scroll({
    required this.painter,
    required this.width,
    required this.lane,
    required this.startClock,
  }) : mode = 0;

  _ActiveBullet.fixed({
    required this.painter,
    required this.width,
    required this.mode,
    required this.lane,
    required this.startClock,
  });

  static const double _scrollDurationMs = 9000;
  static const double _fixedDurationMs = 4500;

  double scrollProgress(double clock) {
    return ((clock - startClock) / _scrollDurationMs).clamp(0.0, 1.0);
  }

  // Left x for a scroll bullet at the given clock.
  double scrollX(double clock, double screenWidth) {
    final travel = screenWidth + width;
    final p = (clock - startClock) / _scrollDurationMs;
    return screenWidth - p * travel;
  }

  bool isExpired(double clock, double screenWidth) {
    if (mode == 0) {
      return scrollX(clock, screenWidth) + width < 0;
    }
    return clock - startClock > _fixedDurationMs;
  }
}

class _DanmakuPainter extends CustomPainter {
  final List<_ActiveBullet> active;
  final double Function() clockProvider;

  _DanmakuPainter({
    required Listenable repaint,
    required this.active,
    required this.clockProvider,
  }) : super(repaint: repaint);

  @override
  void paint(Canvas canvas, Size size) {
    final clock = clockProvider();
    const laneHeight = _DanmakuOverlayState._laneHeight;
    for (final b in active) {
      double x;
      double y;
      if (b.mode == 0) {
        x = b.scrollX(clock, size.width);
        y = 4 + b.lane * laneHeight;
      } else if (b.mode == 1) {
        x = (size.width - b.width) / 2;
        y = 4 + b.lane * laneHeight;
      } else {
        x = (size.width - b.width) / 2;
        y = size.height - 8 - (b.lane + 1) * laneHeight;
      }
      b.painter.paint(canvas, Offset(x, y));
    }
  }

  @override
  bool shouldRepaint(covariant _DanmakuPainter oldDelegate) => true;
}
