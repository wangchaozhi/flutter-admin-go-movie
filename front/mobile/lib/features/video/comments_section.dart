import 'package:flutter/material.dart';

import '../../core/api_client.dart';
import '../../core/session.dart';

const _accent = Color(0xFF25D0AB);
const _muted = Color(0xFF9CA3AF);
const _surface = Color(0xFF171B24);

class VideoComment {
  VideoComment({
    required this.id,
    required this.content,
    required this.rating,
    required this.nickname,
    required this.username,
    required this.createdAt,
  });

  final int id;
  final String content;
  final int rating;
  final String nickname;
  final String username;
  final String createdAt;

  String get displayName {
    if (nickname.trim().isNotEmpty) return nickname.trim();
    if (username.trim().isNotEmpty) return username.trim();
    return '用户';
  }

  factory VideoComment.fromJson(Map<String, dynamic> json) => VideoComment(
    id: (json['id'] as num?)?.toInt() ?? 0,
    content: json['content']?.toString() ?? '',
    rating: (json['rating'] as num?)?.toInt() ?? 0,
    nickname: json['nickname']?.toString() ?? '',
    username: json['username']?.toString() ?? '',
    createdAt: json['created_at']?.toString() ?? '',
  );
}

/// Self-contained comments + rating block for the player page. Loads its own
/// data for [videoId] and posts new comments via the mobile API.
class CommentsSection extends StatefulWidget {
  const CommentsSection({super.key, required this.videoId});

  final int videoId;

  @override
  State<CommentsSection> createState() => _CommentsSectionState();
}

class _CommentsSectionState extends State<CommentsSection> {
  final _controller = TextEditingController();

  List<VideoComment> _comments = [];
  int _total = 0;
  int _ratingCount = 0;
  double _average = 0;
  int _draftRating = 0;
  bool _loading = true;
  bool _submitting = false;
  String? _username;

  @override
  void initState() {
    super.initState();
    _init();
  }

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  Future<void> _init() async {
    _username = await Session.username();
    await _load();
  }

  Future<void> _load() async {
    if (!mounted) return;
    setState(() => _loading = true);
    try {
      final resp = await ApiClient().get(
        '/api/videos/${widget.videoId}/comments?per_page=20',
      );
      if (!mounted) return;
      if (resp['code'] == 0) {
        final data = resp['data'] as Map<String, dynamic>? ?? {};
        final list = (data['items'] as List<dynamic>? ?? [])
            .map((e) => VideoComment.fromJson(e as Map<String, dynamic>))
            .toList();
        final summary = data['summary'] as Map<String, dynamic>? ?? {};
        setState(() {
          _comments = list;
          _total = (data['total'] as num?)?.toInt() ?? list.length;
          _ratingCount = (summary['rating_count'] as num?)?.toInt() ?? 0;
          _average = (summary['average_rating'] as num?)?.toDouble() ?? 0;
        });
      }
    } catch (_) {
      // keep whatever we have
    } finally {
      if (mounted) setState(() => _loading = false);
    }
  }

  Future<void> _submit() async {
    final content = _controller.text.trim();
    if (content.isEmpty && _draftRating == 0) {
      _snack('请填写评论或选择评分');
      return;
    }
    setState(() => _submitting = true);
    try {
      final resp = await ApiClient().postAuth(
        '/api/videos/${widget.videoId}/comments',
        {'content': content, 'rating': _draftRating},
      );
      if (!mounted) return;
      if (resp['code'] == 0) {
        _controller.clear();
        setState(() => _draftRating = 0);
        await _load();
      } else {
        _snack(resp['msg']?.toString() ?? '评论失败');
      }
    } catch (_) {
      _snack('评论失败，请稍后再试');
    } finally {
      if (mounted) setState(() => _submitting = false);
    }
  }

  Future<void> _delete(VideoComment comment) async {
    try {
      final resp = await ApiClient().deleteAuth(
        '/api/mobile/comments/${comment.id}',
      );
      if (!mounted) return;
      if (resp['code'] == 0) {
        await _load();
      } else {
        _snack(resp['msg']?.toString() ?? '删除失败');
      }
    } catch (_) {
      _snack('删除失败');
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
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        const SizedBox(height: 24),
        Row(
          children: [
            const Text(
              '评论',
              style: TextStyle(
                color: Colors.white,
                fontSize: 16,
                fontWeight: FontWeight.w800,
              ),
            ),
            const SizedBox(width: 8),
            Text('($_total)', style: const TextStyle(color: _muted)),
            const Spacer(),
            if (_ratingCount > 0) ...[
              const Icon(Icons.star, color: Color(0xFFFFC857), size: 16),
              const SizedBox(width: 4),
              Text(
                '${_average.toStringAsFixed(1)} · $_ratingCount 人评分',
                style: const TextStyle(color: _muted, fontSize: 12),
              ),
            ],
          ],
        ),
        const SizedBox(height: 12),
        _buildComposer(),
        const SizedBox(height: 16),
        if (_loading)
          const Padding(
            padding: EdgeInsets.symmetric(vertical: 24),
            child: Center(
              child: CircularProgressIndicator(color: _accent),
            ),
          )
        else if (_comments.isEmpty)
          const Padding(
            padding: EdgeInsets.symmetric(vertical: 24),
            child: Center(
              child: Text('还没有评论，来说点什么吧', style: TextStyle(color: _muted)),
            ),
          )
        else
          ..._comments.map(_buildCommentTile),
      ],
    );
  }

  Widget _buildComposer() {
    return Container(
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: _surface,
        borderRadius: BorderRadius.circular(10),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          _buildStarSelector(),
          const SizedBox(height: 8),
          TextField(
            controller: _controller,
            maxLines: 3,
            minLines: 1,
            maxLength: 1000,
            style: const TextStyle(color: Colors.white),
            cursorColor: _accent,
            decoration: const InputDecoration(
              hintText: '写下你的评论…',
              hintStyle: TextStyle(color: _muted),
              border: InputBorder.none,
              counterStyle: TextStyle(color: _muted),
            ),
          ),
          Align(
            alignment: Alignment.centerRight,
            child: ElevatedButton(
              onPressed: _submitting ? null : _submit,
              style: ElevatedButton.styleFrom(
                backgroundColor: _accent,
                foregroundColor: Colors.black,
              ),
              child: Text(_submitting ? '提交中…' : '发表'),
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildStarSelector() {
    return Row(
      children: List.generate(5, (i) {
        final filled = i < _draftRating;
        return IconButton(
          padding: EdgeInsets.zero,
          constraints: const BoxConstraints(minWidth: 36, minHeight: 36),
          icon: Icon(
            filled ? Icons.star : Icons.star_border,
            color: filled ? const Color(0xFFFFC857) : _muted,
          ),
          onPressed: () => setState(() => _draftRating = i + 1),
        );
      }),
    );
  }

  Widget _buildCommentTile(VideoComment comment) {
    final isOwn =
        _username != null &&
        comment.username.isNotEmpty &&
        comment.username == _username;
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 8),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Text(
                comment.displayName,
                style: const TextStyle(
                  color: Colors.white,
                  fontWeight: FontWeight.w700,
                ),
              ),
              const SizedBox(width: 8),
              if (comment.rating > 0)
                Row(
                  children: List.generate(
                    comment.rating,
                    (_) => const Icon(
                      Icons.star,
                      size: 13,
                      color: Color(0xFFFFC857),
                    ),
                  ),
                ),
              const Spacer(),
              if (isOwn)
                GestureDetector(
                  onTap: () => _delete(comment),
                  child: const Icon(
                    Icons.delete_outline,
                    size: 18,
                    color: _muted,
                  ),
                ),
            ],
          ),
          if (comment.content.isNotEmpty) ...[
            const SizedBox(height: 4),
            Text(
              comment.content,
              style: const TextStyle(color: Color(0xFFD1D5DB), height: 1.4),
            ),
          ],
          const SizedBox(height: 4),
          Text(
            _formatTime(comment.createdAt),
            style: const TextStyle(color: _muted, fontSize: 11),
          ),
          const Divider(color: Color(0xFF232838), height: 20),
        ],
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
