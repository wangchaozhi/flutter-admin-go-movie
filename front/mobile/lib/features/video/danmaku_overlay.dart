import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter/scheduler.dart';
import 'package:media_kit/media_kit.dart';

import '../../core/api_client.dart';
import '../../core/session.dart';

/// One bullet comment anchored to a playback position. Mirrors the backend
/// `store.VideoDanmaku` JSON shape. [likeCount]/[liked] are mutable so a like
/// toggle updates the bullet in place across the overlay and the list sheet.
class DanmakuItem {
  final int id;
  final int userId;
  final String content;
  final int timeMs;
  final int color; // 24-bit RGB, e.g. 0xFFFFFF
  final int mode; // 0 scroll, 1 top, 2 bottom
  int likeCount;
  bool liked;

  DanmakuItem({
    required this.id,
    required this.userId,
    required this.content,
    required this.timeMs,
    required this.color,
    required this.mode,
    this.likeCount = 0,
    this.liked = false,
  });

  factory DanmakuItem.fromJson(Map<String, dynamic> json) {
    return DanmakuItem(
      id: (json['id'] as num?)?.toInt() ?? 0,
      userId: (json['user_id'] as num?)?.toInt() ?? 0,
      content: (json['content'] ?? '').toString(),
      timeMs: (json['time_ms'] as num?)?.toInt() ?? 0,
      color: (json['color'] as num?)?.toInt() ?? 0xFFFFFF,
      mode: (json['mode'] as num?)?.toInt() ?? 0,
      likeCount: (json['like_count'] as num?)?.toInt() ?? 0,
      liked: json['liked'] as bool? ?? false,
    );
  }

  Color get flutterColor => Color(0xFF000000 | (color & 0xFFFFFF));
}

/// Bridges the player page (and the danmaku list sheet) to the overlay: inject
/// freshly-sent bullets, read the loaded bullets, and like/delete them.
class DanmakuController {
  _DanmakuOverlayState? _state;

  void _attach(_DanmakuOverlayState state) => _state = state;
  void _detach(_DanmakuOverlayState state) {
    if (_state == state) _state = null;
  }

  /// Inject a locally-authored bullet so the sender sees it right away.
  void addLocal(DanmakuItem item) => _state?._addLocal(item);

  /// The current viewer's id (null when signed out), to tell which bullets the
  /// list sheet may delete.
  int? get currentUserId => _state?._currentUserId;

  /// All loaded bullets (server + locally-sent), newest first, for the sheet.
  List<DanmakuItem> bullets() => _state?._allBullets() ?? const [];

  /// Toggle a like; returns null on success or a user-facing error message.
  Future<String?> toggleLike(DanmakuItem item) async =>
      _state?._toggleLike(item) ?? Future.value('弹幕不可用');

  /// Delete the caller's own bullet; returns null on success or an error.
  Future<String?> deleteOwn(DanmakuItem item) async =>
      _state?._deleteOwn(item) ?? Future.value('弹幕不可用');
}

/// Renders danmaku over the player. Bullets are spawned as playback passes their
/// timestamp, animated by an internal clock that only advances while the video
/// is playing (so pausing freezes them in place), and drawn by a single
/// [CustomPainter] for efficiency. Kept behind [IgnorePointer] so taps still
/// reach the player controls — like/delete happen in the danmaku list sheet.
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

  // Server-loaded bullets (sorted by time) drive position-based spawning;
  // locally-sent bullets are tracked separately so the spawner never replays
  // them, but the list sheet still shows them.
  List<DanmakuItem> _bullets = const [];
  final List<DanmakuItem> _localBullets = [];
  int _nextIndex = 0;
  int _lastPosMs = 0;
  int? _currentUserId;

  final List<_ActiveBullet> _active = [];
  // Per-lane last scroll bullet, for collision avoidance.
  final Map<int, _ActiveBullet> _scrollLaneLast = {};
  // Occupied-until clock per fixed (top/bottom) lane.
  final Map<int, double> _topLaneUntil = {};
  final Map<int, double> _bottomLaneUntil = {};

  StreamSubscription<Duration>? _posSub;
  StreamSubscription<bool>? _playingSub;

  Size _size = Size.zero;

  // When a bullet is tapped it is frozen in place (the video keeps playing) and
  // an action card is shown anchored to it. Dismissing resumes it seamlessly.
  _ActiveBullet? _frozen;
  Offset _frozenPos = Offset.zero;
  Timer? _cardTimer;

  @override
  void initState() {
    super.initState();
    widget.controller._attach(this);
    _ticker = createTicker(_onTick)..start();
    _playing = widget.player.state.playing;
    _posSub = widget.player.stream.position.listen(_onPosition);
    _playingSub = widget.player.stream.playing.listen((p) => _playing = p);
    unawaited(_loadUser());
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
    _cardTimer?.cancel();
    _clock.dispose();
    super.dispose();
  }

  Future<void> _loadUser() async {
    final id = await Session.userId();
    if (mounted) _currentUserId = id;
  }

  Future<void> _load() async {
    try {
      // Authed request so the backend can flag which bullets we've liked.
      final resp = await ApiClient().getAuth('/api/videos/${widget.videoId}/danmaku');
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
    _localBullets.clear();
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
      if (_frozen != null) {
        _cardTimer?.cancel();
        // The frozen bullet was cleared by the seek; close its card.
        WidgetsBinding.instance.addPostFrameCallback((_) {
          if (mounted) setState(() => _frozen = null);
        });
      }
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
    // Drop bullets that have left the screen, but never the frozen one (its card
    // is open).
    if (_active.isNotEmpty) {
      _active.removeWhere((b) =>
          !identical(b, _frozen) && b.isExpired(_danmuClockMs, _size.width));
    }
    _clock.value = _danmuClockMs;
  }

  /// Inject a bullet sent right now by this viewer: spawn it immediately and
  /// remember it so the list sheet shows it too.
  void _addLocal(DanmakuItem item) {
    _localBullets.add(item);
    _spawn(item);
  }

  List<DanmakuItem> _allBullets() {
    // Newest first for the management sheet.
    final all = [..._bullets, ..._localBullets]
      ..sort((a, b) => b.id.compareTo(a.id));
    return all;
  }

  void _spawn(DanmakuItem item) {
    if (_size == Size.zero) return;
    final painter = _buildPainter(item);
    final width = painter.width;

    if (item.mode == 1 || item.mode == 2) {
      final lane = _pickFixedLane(item.mode == 1);
      if (lane < 0) return; // no room; drop this fixed bullet
      _active.add(_ActiveBullet.fixed(
        item: item,
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
      item: item,
      painter: painter,
      width: width,
      lane: lane,
      startClock: _danmuClockMs,
    );
    _scrollLaneLast[lane] = bullet;
    _active.add(bullet);
  }

  TextPainter _buildPainter(DanmakuItem item) {
    final spans = <InlineSpan>[
      TextSpan(text: item.content, style: _bulletStyle(item.flutterColor)),
    ];
    // Mainstream platforms surface popular bullets by showing a like tally
    // inline; append a small heart + count once a bullet has any likes.
    if (item.likeCount > 0) {
      spans.add(TextSpan(
        text: '  ♥${item.likeCount}',
        style: _bulletStyle(const Color(0xFFFF5A79)).copyWith(fontSize: 12),
      ));
    }
    return TextPainter(
      text: TextSpan(children: spans),
      textDirection: TextDirection.ltr,
    )..layout();
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

  // Re-layout any on-screen instances of a bullet whose like tally changed, so
  // the inline "♥N" updates without waiting for a replay.
  void _refreshActivePainter(DanmakuItem item) {
    for (final b in _active) {
      if (identical(b.item, item)) {
        b.painter = _buildPainter(item);
        b.width = b.painter.width;
      }
    }
  }

  Future<String?> _toggleLike(DanmakuItem item) async {
    if (_currentUserId == null) return '点赞需要先登录';
    if (item.id <= 0) return '弹幕尚未同步，请稍后再试';
    try {
      final resp =
          await ApiClient().postAuth('/api/mobile/danmaku/${item.id}/like', {});
      if (resp['code'] != 0) {
        return resp['msg']?.toString() ?? '操作失败';
      }
      final data = resp['data'] as Map<String, dynamic>? ?? {};
      item.liked = data['liked'] as bool? ?? !item.liked;
      item.likeCount = (data['like_count'] as num?)?.toInt() ?? item.likeCount;
      _refreshActivePainter(item);
      return null;
    } catch (_) {
      return '网络错误，请稍后再试';
    }
  }

  Future<String?> _deleteOwn(DanmakuItem item) async {
    if (item.id <= 0) return '弹幕尚未同步，请稍后再试';
    try {
      final resp = await ApiClient().deleteAuth('/api/mobile/danmaku/${item.id}');
      if (resp['code'] != 0) {
        return resp['msg']?.toString() ?? '删除失败';
      }
      _bullets = _bullets.where((b) => b.id != item.id).toList();
      _localBullets.removeWhere((b) => b.id == item.id);
      _active.removeWhere((b) => b.item.id == item.id);
      // Cursor may now point past the removed item; clamp it.
      if (_nextIndex > _bullets.length) _nextIndex = _bullets.length;
      return null;
    } catch (_) {
      return '网络错误，请稍后再试';
    }
  }

  // Current on-screen rect of a bullet, used for both hit-testing and anchoring
  // the action card.
  Rect _bulletRect(_ActiveBullet b) {
    if (identical(b, _frozen)) {
      return Rect.fromLTWH(_frozenPos.dx, _frozenPos.dy, b.width, _laneHeight);
    }
    double x;
    double y;
    if (b.mode == 0) {
      x = b.scrollX(_danmuClockMs, _size.width);
      y = 4 + b.lane * _laneHeight;
    } else if (b.mode == 1) {
      x = (_size.width - b.width) / 2;
      y = 4 + b.lane * _laneHeight;
    } else {
      x = (_size.width - b.width) / 2;
      y = _size.height - 8 - (b.lane + 1) * _laneHeight;
    }
    return Rect.fromLTWH(x, y, b.width, _laneHeight);
  }

  // Topmost bullet under a point (later-drawn bullets win), with a small
  // tap-tolerance inflation. Returns null when the point misses every bullet.
  _ActiveBullet? _bulletAt(Offset p) {
    for (int i = _active.length - 1; i >= 0; i--) {
      final b = _active[i];
      if (_bulletRect(b).inflate(4).contains(p)) return b;
    }
    return null;
  }

  void _onTapBullet(Offset pos) {
    final b = _bulletAt(pos);
    if (b == null) return;
    final rect = _bulletRect(b);
    setState(() {
      _frozen = b;
      _frozenPos = Offset(rect.left, rect.top);
    });
    _restartCardTimer();
  }

  void _restartCardTimer() {
    _cardTimer?.cancel();
    _cardTimer = Timer(const Duration(seconds: 5), _dismissCard);
  }

  void _dismissCard() {
    _cardTimer?.cancel();
    final b = _frozen;
    if (b != null && _active.contains(b)) {
      if (b.mode == 0) {
        // Resume scrolling seamlessly from the frozen x position.
        final travel = _size.width + b.width;
        final p = (_size.width - _frozenPos.dx) / travel;
        b.startClock = _danmuClockMs - p * _ActiveBullet._scrollDurationMs;
      } else {
        b.startClock = _danmuClockMs; // restart the dwell for a fixed bullet
      }
    }
    if (mounted) {
      setState(() => _frozen = null);
    } else {
      _frozen = null;
    }
  }

  Future<void> _likeFrozen(_ActiveBullet b) async {
    final err = await _toggleLike(b.item);
    if (!mounted) return;
    if (err != null) {
      _snack(err);
      return;
    }
    setState(() {}); // refresh the card's heart + count
    _restartCardTimer();
  }

  Future<void> _deleteFrozen(_ActiveBullet b) async {
    final err = await _deleteOwn(b.item);
    if (!mounted) return;
    if (err != null) {
      _snack(err);
      return;
    }
    _dismissCard();
  }

  void _snack(String message) {
    if (!mounted) return;
    ScaffoldMessenger.of(context)
        .showSnackBar(SnackBar(content: Text(message)));
  }

  @override
  Widget build(BuildContext context) {
    if (!widget.enabled) return const SizedBox.shrink();
    return LayoutBuilder(
      builder: (context, constraints) {
        _size = Size(constraints.maxWidth, constraints.maxHeight);
        final showCard = _frozen != null && _active.contains(_frozen);
        return Stack(
          children: [
            // Bullet layer: consumes a tap only when it lands on a bullet
            // (see _DanmakuHitTarget); a miss passes through to the player so
            // tap-to-toggle-controls keeps working.
            _DanmakuHitTarget(
              hitTester: (pos) => _bulletAt(pos) != null,
              child: GestureDetector(
                behavior: HitTestBehavior.opaque,
                onTapUp: (details) => _onTapBullet(details.localPosition),
                child: CustomPaint(
                  size: _size,
                  painter: _DanmakuPainter(
                    repaint: _clock,
                    active: _active,
                    clockProvider: () => _danmuClockMs,
                    frozen: _frozen,
                    frozenPos: _frozenPos,
                  ),
                ),
              ),
            ),
            if (showCard) ...[
              Positioned.fill(
                child: GestureDetector(
                  behavior: HitTestBehavior.opaque,
                  onTap: _dismissCard,
                ),
              ),
              _buildActionCard(_frozen!),
            ],
          ],
        );
      },
    );
  }

  Widget _buildActionCard(_ActiveBullet b) {
    const cardW = 170.0;
    const cardH = 44.0;
    final maxLeft = (_size.width - cardW - 8).clamp(8.0, double.infinity);
    final left = _frozenPos.dx.clamp(8.0, maxLeft);
    double top = _frozenPos.dy - cardH - 8;
    if (top < 8) top = _frozenPos.dy + _laneHeight + 8;
    final isOwn = _currentUserId != null && b.item.userId == _currentUserId;
    return Positioned(
      left: left,
      top: top,
      child: Material(
        color: Colors.transparent,
        child: DecoratedBox(
          decoration: BoxDecoration(
            color: const Color(0xF21A1F28),
            borderRadius: BorderRadius.circular(10),
            border: Border.all(color: const Color(0xFF2B3140)),
          ),
          child: Padding(
            padding: const EdgeInsets.symmetric(horizontal: 4, vertical: 2),
            child: Row(
              mainAxisSize: MainAxisSize.min,
              children: [
                _cardButton(
                  icon: b.item.liked ? Icons.favorite : Icons.favorite_border,
                  iconColor: b.item.liked
                      ? const Color(0xFFFF5A79)
                      : Colors.white,
                  label: '${b.item.likeCount}',
                  onTap: () => _likeFrozen(b),
                ),
                if (isOwn)
                  _cardButton(
                    icon: Icons.delete_outline,
                    iconColor: Colors.white,
                    label: '删除',
                    onTap: () => _deleteFrozen(b),
                  ),
              ],
            ),
          ),
        ),
      ),
    );
  }

  Widget _cardButton({
    required IconData icon,
    required Color iconColor,
    required String label,
    required VoidCallback onTap,
  }) {
    return InkWell(
      onTap: onTap,
      borderRadius: BorderRadius.circular(6),
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 8),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(icon, size: 18, color: iconColor),
            const SizedBox(width: 4),
            Text(
              label,
              style: const TextStyle(color: Colors.white, fontSize: 13),
            ),
          ],
        ),
      ),
    );
  }
}

class _ActiveBullet {
  final DanmakuItem item;
  TextPainter painter;
  double width;
  final int mode; // 0 scroll, 1 top, 2 bottom
  final int lane;
  double startClock;

  _ActiveBullet.scroll({
    required this.item,
    required this.painter,
    required this.width,
    required this.lane,
    required this.startClock,
  }) : mode = 0;

  _ActiveBullet.fixed({
    required this.item,
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
  final _ActiveBullet? frozen;
  final Offset frozenPos;

  _DanmakuPainter({
    required Listenable repaint,
    required this.active,
    required this.clockProvider,
    required this.frozen,
    required this.frozenPos,
  }) : super(repaint: repaint);

  @override
  void paint(Canvas canvas, Size size) {
    final clock = clockProvider();
    const laneHeight = _DanmakuOverlayState._laneHeight;
    for (final b in active) {
      double x;
      double y;
      if (identical(b, frozen)) {
        // A tapped bullet is pinned in place and highlighted while its card is
        // open; the rest of the track keeps flowing.
        x = frozenPos.dx;
        y = frozenPos.dy;
        final box = RRect.fromRectAndRadius(
          Rect.fromLTWH(x - 4, y - 2, b.width + 8, laneHeight),
          const Radius.circular(6),
        );
        canvas.drawRRect(box, Paint()..color = const Color(0xB3000000));
        canvas.drawRRect(
          box,
          Paint()
            ..style = PaintingStyle.stroke
            ..strokeWidth = 1.5
            ..color = const Color(0xFF25D0AB),
        );
      } else if (b.mode == 0) {
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

/// A render object that consumes a pointer only when it lands on a bullet
/// (decided by [hitTester]); otherwise it reports a miss so the event falls
/// through to the player controls behind it in the Stack. This is what lets a
/// tap on a bullet open its action card while a tap on empty space still
/// toggles the player controls.
class _DanmakuHitTarget extends SingleChildRenderObjectWidget {
  const _DanmakuHitTarget({required this.hitTester, required Widget super.child});

  final bool Function(Offset localPosition) hitTester;

  @override
  RenderObject createRenderObject(BuildContext context) =>
      _RenderDanmakuHitTarget(hitTester);

  @override
  void updateRenderObject(
    BuildContext context,
    _RenderDanmakuHitTarget renderObject,
  ) {
    renderObject.hitTester = hitTester;
  }
}

class _RenderDanmakuHitTarget extends RenderProxyBox {
  _RenderDanmakuHitTarget(this.hitTester);

  bool Function(Offset localPosition) hitTester;

  @override
  bool hitTest(BoxHitTestResult result, {required Offset position}) {
    if (size.contains(position) && hitTester(position)) {
      return super.hitTest(result, position: position);
    }
    return false;
  }
}

/// Opens the danmaku list sheet — the mainstream way to interact with bullets
/// (like / delete-own) without fighting the player's tap-to-pause. Rows show the
/// text, a like button with its tally, and a delete action on the viewer's own
/// bullets.
Future<void> showDanmakuList(BuildContext context, DanmakuController controller) {
  return showModalBottomSheet<void>(
    context: context,
    backgroundColor: const Color(0xFF14181F),
    isScrollControlled: true,
    shape: const RoundedRectangleBorder(
      borderRadius: BorderRadius.vertical(top: Radius.circular(16)),
    ),
    builder: (context) => _DanmakuListSheet(controller: controller),
  );
}

class _DanmakuListSheet extends StatefulWidget {
  const _DanmakuListSheet({required this.controller});

  final DanmakuController controller;

  @override
  State<_DanmakuListSheet> createState() => _DanmakuListSheetState();
}

class _DanmakuListSheetState extends State<_DanmakuListSheet> {
  late List<DanmakuItem> _items;
  bool _busy = false;

  @override
  void initState() {
    super.initState();
    _items = widget.controller.bullets();
  }

  void _snack(String message) {
    if (!mounted) return;
    ScaffoldMessenger.of(context)
        .showSnackBar(SnackBar(content: Text(message)));
  }

  Future<void> _like(DanmakuItem item) async {
    if (_busy) return;
    setState(() => _busy = true);
    final err = await widget.controller.toggleLike(item);
    if (!mounted) return;
    setState(() => _busy = false);
    if (err != null) _snack(err);
  }

  Future<void> _delete(DanmakuItem item) async {
    if (_busy) return;
    setState(() => _busy = true);
    final err = await widget.controller.deleteOwn(item);
    if (!mounted) return;
    setState(() {
      _busy = false;
      if (err == null) _items = widget.controller.bullets();
    });
    if (err != null) _snack(err);
  }

  @override
  Widget build(BuildContext context) {
    final myId = widget.controller.currentUserId;
    return SafeArea(
      child: Padding(
        padding: const EdgeInsets.fromLTRB(16, 12, 16, 8),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                const Icon(Icons.subtitles_rounded,
                    color: Color(0xFF25D0AB), size: 18),
                const SizedBox(width: 8),
                Text(
                  '弹幕列表 (${_items.length})',
                  style: const TextStyle(
                    color: Colors.white,
                    fontSize: 16,
                    fontWeight: FontWeight.w800,
                  ),
                ),
                const Spacer(),
                IconButton(
                  icon: const Icon(Icons.close, color: Color(0xFF9CA3AF)),
                  onPressed: () => Navigator.pop(context),
                ),
              ],
            ),
            const SizedBox(height: 4),
            if (_items.isEmpty)
              const Padding(
                padding: EdgeInsets.symmetric(vertical: 40),
                child: Center(
                  child: Text('还没有弹幕，发一条试试',
                      style: TextStyle(color: Color(0xFF9CA3AF))),
                ),
              )
            else
              Flexible(
                child: ListView.separated(
                  shrinkWrap: true,
                  itemCount: _items.length,
                  separatorBuilder: (_, _) =>
                      const Divider(height: 1, color: Color(0xFF222936)),
                  itemBuilder: (context, index) {
                    final item = _items[index];
                    final isOwn = myId != null && item.userId == myId;
                    return Padding(
                      padding: const EdgeInsets.symmetric(vertical: 8),
                      child: Row(
                        children: [
                          Expanded(
                            child: Text(
                              item.content,
                              style: TextStyle(
                                color: item.flutterColor,
                                fontSize: 14,
                                height: 1.3,
                              ),
                            ),
                          ),
                          const SizedBox(width: 8),
                          InkWell(
                            onTap: () => _like(item),
                            borderRadius: BorderRadius.circular(14),
                            child: Padding(
                              padding: const EdgeInsets.symmetric(
                                  horizontal: 8, vertical: 4),
                              child: Row(
                                mainAxisSize: MainAxisSize.min,
                                children: [
                                  Icon(
                                    item.liked
                                        ? Icons.favorite
                                        : Icons.favorite_border,
                                    size: 16,
                                    color: item.liked
                                        ? const Color(0xFFFF5A79)
                                        : const Color(0xFF9CA3AF),
                                  ),
                                  const SizedBox(width: 4),
                                  Text(
                                    '${item.likeCount}',
                                    style: const TextStyle(
                                      color: Color(0xFF9CA3AF),
                                      fontSize: 12,
                                    ),
                                  ),
                                ],
                              ),
                            ),
                          ),
                          if (isOwn)
                            IconButton(
                              visualDensity: VisualDensity.compact,
                              icon: const Icon(Icons.delete_outline,
                                  size: 18, color: Color(0xFF9CA3AF)),
                              onPressed: () => _delete(item),
                            ),
                        ],
                      ),
                    );
                  },
                ),
              ),
          ],
        ),
      ),
    );
  }
}
