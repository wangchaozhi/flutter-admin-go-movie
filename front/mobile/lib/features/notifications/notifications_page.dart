import 'dart:async';

import 'package:flutter/material.dart';

import '../../core/api_client.dart';
import '../../models/video.dart';
import '../video/video_player_page.dart';

const _accent = Color(0xFF25D0AB);
const _muted = Color(0xFF9CA3AF);
const _bg = Color(0xFF0D0F14);
const _surface = Color(0xFF171B24);

/// One in-app notification (a reply to, or a like on, the user's comment).
class AppNotification {
  AppNotification({
    required this.id,
    required this.type,
    required this.videoId,
    required this.content,
    required this.isRead,
    required this.createdAt,
    required this.actorNickname,
    required this.actorUsername,
    required this.videoTitle,
  });

  final int id;
  final String type; // 'reply' | 'like'
  final int videoId;
  final String content;
  final bool isRead;
  final String createdAt;
  final String actorNickname;
  final String actorUsername;
  final String videoTitle;

  String get actorName {
    if (actorNickname.trim().isNotEmpty) return actorNickname.trim();
    if (actorUsername.trim().isNotEmpty) return actorUsername.trim();
    return '用户';
  }

  bool get isLike => type == 'like';

  factory AppNotification.fromJson(Map<String, dynamic> json) =>
      AppNotification(
        id: (json['id'] as num?)?.toInt() ?? 0,
        type: json['type']?.toString() ?? 'reply',
        videoId: (json['video_id'] as num?)?.toInt() ?? 0,
        content: json['content']?.toString() ?? '',
        isRead: json['is_read'] as bool? ?? false,
        createdAt: json['created_at']?.toString() ?? '',
        actorNickname: json['actor_nickname']?.toString() ?? '',
        actorUsername: json['actor_username']?.toString() ?? '',
        videoTitle: json['video_title']?.toString() ?? '',
      );
}

class NotificationsPage extends StatefulWidget {
  const NotificationsPage({super.key});

  @override
  State<NotificationsPage> createState() => _NotificationsPageState();
}

class _NotificationsPageState extends State<NotificationsPage> {
  List<AppNotification> _items = [];
  bool _loading = true;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    if (!mounted) return;
    setState(() => _loading = true);
    try {
      final resp = await ApiClient().getAuth('/api/mobile/notifications');
      if (!mounted) return;
      if (resp['code'] == 0) {
        final data = resp['data'] as Map<String, dynamic>? ?? {};
        setState(() {
          _items = (data['items'] as List<dynamic>? ?? [])
              .map((e) => AppNotification.fromJson(e as Map<String, dynamic>))
              .toList();
        });
        // Mark everything read once viewed; the badge clears on return.
        unawaited(
          ApiClient()
              .postAuth('/api/mobile/notifications/read', {})
              .catchError((_) => <String, dynamic>{}),
        );
      }
    } catch (_) {
      // keep whatever we have
    } finally {
      if (mounted) setState(() => _loading = false);
    }
  }

  Future<void> _open(AppNotification n) async {
    try {
      final resp = await ApiClient().get('/api/videos/${n.videoId}');
      if (!mounted) return;
      if (resp['code'] == 0 && resp['data'] is Map<String, dynamic>) {
        final video = Video.fromJson(resp['data'] as Map<String, dynamic>);
        if (!mounted) return;
        Navigator.pushNamed(
          context,
          '/player',
          arguments: PlayerArgs(video, scrollToComments: true),
        );
      } else {
        _snack(resp['msg']?.toString() ?? '影片不存在或已下架');
      }
    } catch (_) {
      _snack('打开失败，请稍后再试');
    }
  }

  void _snack(String message) {
    if (!mounted) return;
    ScaffoldMessenger.of(
      context,
    ).showSnackBar(SnackBar(content: Text(message)));
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: _bg,
      appBar: AppBar(
        backgroundColor: _bg,
        foregroundColor: Colors.white,
        title: const Text('消息通知', style: TextStyle(fontWeight: FontWeight.w800)),
      ),
      body: _loading
          ? const Center(child: CircularProgressIndicator(color: _accent))
          : _items.isEmpty
          ? const Center(
              child: Text('暂时没有新消息', style: TextStyle(color: _muted)),
            )
          : RefreshIndicator(
              color: _accent,
              backgroundColor: _surface,
              onRefresh: _load,
              child: ListView.separated(
                padding: const EdgeInsets.symmetric(vertical: 8),
                itemCount: _items.length,
                separatorBuilder: (_, _) =>
                    const Divider(height: 1, color: Color(0xFF1E2430)),
                itemBuilder: (context, i) => _buildTile(_items[i]),
              ),
            ),
    );
  }

  Widget _buildTile(AppNotification n) {
    return InkWell(
      onTap: () => _open(n),
      child: Container(
        color: n.isRead ? Colors.transparent : const Color(0xFF11161F),
        padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
        child: Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            CircleAvatar(
              radius: 18,
              backgroundColor: n.isLike
                  ? const Color(0x33EF4444)
                  : const Color(0x3325D0AB),
              child: Icon(
                n.isLike ? Icons.favorite : Icons.chat_bubble,
                size: 18,
                color: n.isLike ? const Color(0xFFEF4444) : _accent,
              ),
            ),
            const SizedBox(width: 12),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  RichText(
                    text: TextSpan(
                      style: const TextStyle(color: Colors.white, fontSize: 14),
                      children: [
                        TextSpan(
                          text: n.actorName,
                          style: const TextStyle(fontWeight: FontWeight.w700),
                        ),
                        TextSpan(
                          text: n.isLike ? ' 赞了你的评论' : ' 回复了你',
                          style: const TextStyle(color: _muted),
                        ),
                      ],
                    ),
                  ),
                  if (n.content.isNotEmpty) ...[
                    const SizedBox(height: 4),
                    Text(
                      n.content,
                      maxLines: 2,
                      overflow: TextOverflow.ellipsis,
                      style: const TextStyle(
                        color: Color(0xFFD1D5DB),
                        fontSize: 13,
                        height: 1.3,
                      ),
                    ),
                  ],
                  const SizedBox(height: 6),
                  Row(
                    children: [
                      if (n.videoTitle.isNotEmpty)
                        Flexible(
                          child: Text(
                            '《${n.videoTitle}》',
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                            style: const TextStyle(color: _accent, fontSize: 12),
                          ),
                        ),
                      if (n.videoTitle.isNotEmpty) const SizedBox(width: 8),
                      Text(
                        _formatTime(n.createdAt),
                        style: const TextStyle(color: _muted, fontSize: 11),
                      ),
                    ],
                  ),
                ],
              ),
            ),
            const Icon(Icons.chevron_right, color: _muted, size: 20),
          ],
        ),
      ),
    );
  }

  String _formatTime(String raw) {
    final parsed = DateTime.tryParse(raw);
    if (parsed == null) return raw;
    final local = parsed.toLocal();
    String two(int n) => n.toString().padLeft(2, '0');
    return '${local.year}-${two(local.month)}-${two(local.day)} ${two(local.hour)}:${two(local.minute)}';
  }
}

/// Bell icon for the home top bar showing the unread-notification count, with a
/// red badge. Refreshes its count after returning from [NotificationsPage].
class NotificationBell extends StatefulWidget {
  const NotificationBell({super.key});

  @override
  State<NotificationBell> createState() => _NotificationBellState();
}

class _NotificationBellState extends State<NotificationBell> {
  int _unread = 0;

  @override
  void initState() {
    super.initState();
    _refresh();
  }

  Future<void> _refresh() async {
    try {
      final resp = await ApiClient().getAuth(
        '/api/mobile/notifications/unread-count',
      );
      if (!mounted) return;
      if (resp['code'] == 0) {
        final data = resp['data'] as Map<String, dynamic>? ?? {};
        setState(() => _unread = (data['unread'] as num?)?.toInt() ?? 0);
      }
    } catch (_) {
      // ignore — the badge is best-effort
    }
  }

  Future<void> _open() async {
    await Navigator.of(context).push(
      MaterialPageRoute<void>(builder: (_) => const NotificationsPage()),
    );
    await _refresh();
  }

  @override
  Widget build(BuildContext context) {
    return Stack(
      clipBehavior: Clip.none,
      children: [
        IconButton(
          tooltip: '消息',
          icon: const Icon(Icons.notifications_none, color: Colors.white),
          onPressed: _open,
        ),
        if (_unread > 0)
          Positioned(
            right: 6,
            top: 6,
            child: Container(
              padding: const EdgeInsets.symmetric(horizontal: 5, vertical: 1),
              constraints: const BoxConstraints(minWidth: 16),
              decoration: BoxDecoration(
                color: const Color(0xFFEF4444),
                borderRadius: BorderRadius.circular(8),
              ),
              child: Text(
                _unread > 99 ? '99+' : '$_unread',
                textAlign: TextAlign.center,
                style: const TextStyle(
                  color: Colors.white,
                  fontSize: 10,
                  fontWeight: FontWeight.w700,
                ),
              ),
            ),
          ),
      ],
    );
  }
}
