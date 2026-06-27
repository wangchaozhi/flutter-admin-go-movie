import 'package:flutter/material.dart';

import '../../core/api_client.dart';
import '../../core/session.dart';

const _accent = Color(0xFF25D0AB);
const _muted = Color(0xFF9CA3AF);
const _surface = Color(0xFF171B24);
const _star = Color(0xFFFFC857);

/// A review (top-level, [parentId] == null) or a reply. Replies are flattened to
/// two levels: a reply to a reply still hangs off the same root review and names
/// the @user via [replyToNickname].
class VideoComment {
  VideoComment({
    required this.id,
    required this.userId,
    required this.content,
    required this.rating,
    required this.parentId,
    required this.replyToNickname,
    required this.likeCount,
    required this.liked,
    required this.nickname,
    required this.username,
    required this.createdAt,
    required this.replies,
    required this.replyCount,
  });

  final int id;
  final int userId;
  final String content;
  final int rating;
  final int? parentId;
  final String replyToNickname;
  int likeCount;
  bool liked;
  final String nickname;
  final String username;
  final String createdAt;
  final List<VideoComment> replies;
  final int replyCount;

  bool get isReply => parentId != null;

  String get displayName {
    if (nickname.trim().isNotEmpty) return nickname.trim();
    if (username.trim().isNotEmpty) return username.trim();
    return '用户';
  }

  factory VideoComment.fromJson(Map<String, dynamic> json) {
    final repliesJson = json['replies'] as List<dynamic>? ?? const [];
    return VideoComment(
      id: (json['id'] as num?)?.toInt() ?? 0,
      userId: (json['user_id'] as num?)?.toInt() ?? 0,
      content: json['content']?.toString() ?? '',
      rating: (json['rating'] as num?)?.toInt() ?? 0,
      parentId: (json['parent_id'] as num?)?.toInt(),
      replyToNickname: json['reply_to_nickname']?.toString() ?? '',
      likeCount: (json['like_count'] as num?)?.toInt() ?? 0,
      liked: json['liked'] as bool? ?? false,
      nickname: json['nickname']?.toString() ?? '',
      username: json['username']?.toString() ?? '',
      createdAt: json['created_at']?.toString() ?? '',
      replies: repliesJson
          .map((e) => VideoComment.fromJson(e as Map<String, dynamic>))
          .toList(),
      replyCount: (json['reply_count'] as num?)?.toInt() ?? repliesJson.length,
    );
  }
}

/// Self-contained comments + rating block for the player page. Loads its own
/// data for [videoId] and posts reviews/replies via the mobile API.
class CommentsSection extends StatefulWidget {
  const CommentsSection({super.key, required this.videoId});

  final int videoId;

  @override
  State<CommentsSection> createState() => _CommentsSectionState();
}

class _CommentsSectionState extends State<CommentsSection> {
  final _controller = TextEditingController();
  final _inputFocus = FocusNode();

  List<VideoComment> _comments = [];
  int _total = 0;
  int _ratingCount = 0;
  double _average = 0;
  int _draftRating = 0;
  bool _loading = true;
  bool _submitting = false;
  String? _username;

  // When set, the composer is in reply mode targeting this comment; otherwise
  // it edits/creates the caller's own review.
  VideoComment? _replyTo;
  // The caller's existing review (if any), used to prefill the composer so a
  // re-post is an explicit edit.
  VideoComment? _ownReview;
  final Set<int> _expanded = {};

  @override
  void initState() {
    super.initState();
    _init();
  }

  @override
  void dispose() {
    _controller.dispose();
    _inputFocus.dispose();
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
      final resp = await ApiClient().getAuth(
        '/api/videos/${widget.videoId}/comments?per_page=20',
      );
      if (!mounted) return;
      if (resp['code'] == 0) {
        final data = resp['data'] as Map<String, dynamic>? ?? {};
        final list = (data['items'] as List<dynamic>? ?? [])
            .map((e) => VideoComment.fromJson(e as Map<String, dynamic>))
            .toList();
        final summary = data['summary'] as Map<String, dynamic>? ?? {};
        VideoComment? own;
        for (final c in list) {
          if (_isOwn(c)) {
            own = c;
            break;
          }
        }
        setState(() {
          _comments = list;
          _total = (data['total'] as num?)?.toInt() ?? list.length;
          _ratingCount = (summary['rating_count'] as num?)?.toInt() ?? 0;
          _average = (summary['average_rating'] as num?)?.toDouble() ?? 0;
          _ownReview = own;
        });
      }
    } catch (_) {
      // keep whatever we have
    } finally {
      if (mounted) setState(() => _loading = false);
    }
  }

  bool _isOwn(VideoComment c) =>
      _username != null && c.username.isNotEmpty && c.username == _username;

  void _startReply(VideoComment target) {
    setState(() {
      _replyTo = target;
      _controller.clear();
    });
    _inputFocus.requestFocus();
  }

  void _startEditReview() {
    final own = _ownReview;
    if (own == null) return;
    setState(() {
      _replyTo = null;
      _controller.text = own.content;
      _draftRating = own.rating;
    });
    _inputFocus.requestFocus();
  }

  void _cancelReply() {
    setState(() {
      _replyTo = null;
      _controller.clear();
    });
  }

  Future<void> _submit() async {
    final content = _controller.text.trim();
    final replyTo = _replyTo;
    if (replyTo != null) {
      if (content.isEmpty) {
        _snack('写点回复内容吧');
        return;
      }
    } else if (content.isEmpty && _draftRating == 0) {
      _snack('写点短评或选择一个评分');
      return;
    }
    setState(() => _submitting = true);
    try {
      final Map<String, dynamic> resp;
      if (replyTo != null) {
        resp = await ApiClient().postAuth(
          '/api/videos/${widget.videoId}/comments',
          {
            'content': content,
            'parent_id': replyTo.id,
            'reply_to_user_id': replyTo.userId,
          },
        );
      } else if (_ownReview != null) {
        resp = await ApiClient().putAuth(
          '/api/mobile/comments/${_ownReview!.id}',
          {'content': content, 'rating': _draftRating},
        );
      } else {
        resp = await ApiClient().postAuth(
          '/api/videos/${widget.videoId}/comments',
          {'content': content, 'rating': _draftRating},
        );
      }
      if (!mounted) return;
      if (resp['code'] == 0) {
        _controller.clear();
        setState(() {
          _draftRating = 0;
          _replyTo = null;
        });
        FocusScope.of(context).unfocus();
        if (replyTo != null) _expanded.add(replyTo.parentId ?? replyTo.id);
        await _load();
      } else {
        _snack(resp['msg']?.toString() ?? '提交失败');
      }
    } catch (_) {
      _snack('提交失败，请稍后再试');
    } finally {
      if (mounted) setState(() => _submitting = false);
    }
  }

  Future<void> _toggleLike(VideoComment comment) async {
    // Optimistic flip; revert on failure.
    setState(() {
      comment.liked = !comment.liked;
      comment.likeCount += comment.liked ? 1 : -1;
      if (comment.likeCount < 0) comment.likeCount = 0;
    });
    try {
      final resp = await ApiClient().postAuth(
        '/api/mobile/comments/${comment.id}/like',
        {},
      );
      if (!mounted) return;
      if (resp['code'] == 0) {
        final data = resp['data'] as Map<String, dynamic>? ?? {};
        setState(() {
          comment.liked = data['liked'] as bool? ?? comment.liked;
          comment.likeCount =
              (data['like_count'] as num?)?.toInt() ?? comment.likeCount;
        });
      } else {
        _revertLike(comment);
        _snack(resp['msg']?.toString() ?? '操作失败');
      }
    } catch (_) {
      _revertLike(comment);
    }
  }

  void _revertLike(VideoComment comment) {
    if (!mounted) return;
    setState(() {
      comment.liked = !comment.liked;
      comment.likeCount += comment.liked ? 1 : -1;
      if (comment.likeCount < 0) comment.likeCount = 0;
    });
  }

  Future<void> _delete(VideoComment comment) async {
    final ok = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        backgroundColor: _surface,
        title: const Text('删除', style: TextStyle(color: Colors.white)),
        content: Text(
          comment.isReply ? '确定删除这条回复吗？' : '确定删除你的影评吗？',
          style: const TextStyle(color: _muted),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(ctx, false),
            child: const Text('取消', style: TextStyle(color: _muted)),
          ),
          TextButton(
            onPressed: () => Navigator.pop(ctx, true),
            child: const Text('删除', style: TextStyle(color: Color(0xFFEF4444))),
          ),
        ],
      ),
    );
    if (ok != true) return;
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
              '影评与讨论',
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
              const Icon(Icons.star, color: _star, size: 16),
              const SizedBox(width: 4),
              Text(
                '${_average.toStringAsFixed(1)} · $_ratingCount 人打分',
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
            child: Center(child: CircularProgressIndicator(color: _accent)),
          )
        else if (_comments.isEmpty)
          const Padding(
            padding: EdgeInsets.symmetric(vertical: 24),
            child: Center(
              child: Text('还没有影评，写下第一条观后感', style: TextStyle(color: _muted)),
            ),
          )
        else
          ..._comments.map(_buildReviewTile),
      ],
    );
  }

  Widget _buildComposer() {
    final replyTo = _replyTo;
    final isReply = replyTo != null;
    return Container(
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: _surface,
        borderRadius: BorderRadius.circular(8),
        border: Border.all(color: const Color(0xFF2B3140)),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          if (isReply)
            Row(
              children: [
                const Icon(Icons.reply, color: _accent, size: 16),
                const SizedBox(width: 6),
                Expanded(
                  child: Text(
                    '回复 @${replyTo.displayName}',
                    style: const TextStyle(color: _accent, fontSize: 13),
                    overflow: TextOverflow.ellipsis,
                  ),
                ),
                GestureDetector(
                  onTap: _cancelReply,
                  child: const Icon(Icons.close, color: _muted, size: 18),
                ),
              ],
            )
          else
            Row(
              children: [
                Text(
                  _ownReview == null ? '给影片评分' : '修改你的影评',
                  style: const TextStyle(
                    color: Colors.white,
                    fontWeight: FontWeight.w800,
                  ),
                ),
                const Spacer(),
                _buildStarSelector(),
              ],
            ),
          const SizedBox(height: 8),
          TextField(
            controller: _controller,
            focusNode: _inputFocus,
            maxLines: 3,
            minLines: 1,
            maxLength: 1000,
            style: const TextStyle(color: Colors.white),
            cursorColor: _accent,
            decoration: InputDecoration(
              hintText: isReply ? '回复 @${replyTo.displayName}' : '写下观后感，或只给一个评分',
              hintStyle: const TextStyle(color: _muted),
              border: InputBorder.none,
              counterStyle: const TextStyle(color: _muted),
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
              child: Text(
                _submitting
                    ? '提交中...'
                    : isReply
                    ? '回复'
                    : _ownReview == null
                    ? '发布'
                    : '更新影评',
              ),
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildStarSelector() {
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: List.generate(5, (i) {
        final filled = i < _draftRating;
        return IconButton(
          padding: EdgeInsets.zero,
          constraints: const BoxConstraints(minWidth: 36, minHeight: 36),
          icon: Icon(
            filled ? Icons.star : Icons.star_border,
            color: filled ? _star : _muted,
          ),
          onPressed: () => setState(() => _draftRating = i + 1),
        );
      }),
    );
  }

  Widget _buildReviewTile(VideoComment review) {
    final replies = review.replies;
    final expanded = _expanded.contains(review.id);
    final shown = expanded ? replies : replies.take(2).toList();
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 8),
      child: DecoratedBox(
        decoration: BoxDecoration(
          color: const Color(0xFF101318),
          borderRadius: BorderRadius.circular(8),
          border: Border.all(color: const Color(0xFF232838)),
        ),
        child: Padding(
          padding: const EdgeInsets.fromLTRB(12, 10, 12, 8),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              _buildHeaderRow(review, showStars: true),
              if (review.content.isNotEmpty) ...[
                const SizedBox(height: 4),
                Text(
                  review.content,
                  style: const TextStyle(color: Color(0xFFD1D5DB), height: 1.4),
                ),
              ],
              const SizedBox(height: 6),
              _buildActionRow(review),
              if (shown.isNotEmpty) ...[
                const SizedBox(height: 8),
                Container(
                  padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
                  decoration: BoxDecoration(
                    color: const Color(0xFF0B0D12),
                    borderRadius: BorderRadius.circular(8),
                  ),
                  child: Column(
                    children: shown.map(_buildReplyTile).toList(),
                  ),
                ),
              ],
              if (replies.length > 2 && !expanded)
                TextButton(
                  onPressed: () => setState(() => _expanded.add(review.id)),
                  style: TextButton.styleFrom(
                    padding: const EdgeInsets.symmetric(vertical: 4),
                    minimumSize: Size.zero,
                    tapTargetSize: MaterialTapTargetSize.shrinkWrap,
                  ),
                  child: Text(
                    '展开 ${replies.length - 2} 条回复',
                    style: const TextStyle(color: _accent, fontSize: 12),
                  ),
                ),
            ],
          ),
        ),
      ),
    );
  }

  Widget _buildReplyTile(VideoComment reply) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 6),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Flexible(
                child: Text(
                  reply.displayName,
                  style: const TextStyle(
                    color: Colors.white,
                    fontWeight: FontWeight.w700,
                    fontSize: 13,
                  ),
                  overflow: TextOverflow.ellipsis,
                ),
              ),
              if (reply.replyToNickname.isNotEmpty) ...[
                const Text(
                  '  回复  ',
                  style: TextStyle(color: _muted, fontSize: 12),
                ),
                Flexible(
                  child: Text(
                    '@${reply.replyToNickname}',
                    style: const TextStyle(color: _accent, fontSize: 13),
                    overflow: TextOverflow.ellipsis,
                  ),
                ),
              ],
            ],
          ),
          const SizedBox(height: 2),
          Text(
            reply.content,
            style: const TextStyle(color: Color(0xFFD1D5DB), height: 1.4, fontSize: 13),
          ),
          const SizedBox(height: 2),
          _buildActionRow(reply, compact: true),
        ],
      ),
    );
  }

  Widget _buildHeaderRow(VideoComment comment, {bool showStars = false}) {
    return Row(
      children: [
        Text(
          comment.displayName,
          style: const TextStyle(
            color: Colors.white,
            fontWeight: FontWeight.w700,
          ),
        ),
        const SizedBox(width: 8),
        if (showStars && comment.rating > 0)
          Row(
            children: List.generate(
              comment.rating,
              (_) => const Icon(Icons.star, size: 13, color: _star),
            ),
          ),
        const Spacer(),
        if (_isOwn(comment) && !comment.isReply)
          GestureDetector(
            onTap: _startEditReview,
            child: const Padding(
              padding: EdgeInsets.only(right: 12),
              child: Icon(Icons.edit_outlined, size: 16, color: _muted),
            ),
          ),
        if (_isOwn(comment))
          GestureDetector(
            onTap: () => _delete(comment),
            child: const Icon(Icons.delete_outline, size: 18, color: _muted),
          ),
      ],
    );
  }

  Widget _buildActionRow(VideoComment comment, {bool compact = false}) {
    final size = compact ? 14.0 : 16.0;
    return Row(
      children: [
        Text(
          _formatTime(comment.createdAt),
          style: TextStyle(color: _muted, fontSize: compact ? 10 : 11),
        ),
        const Spacer(),
        GestureDetector(
          onTap: () => _startReply(comment),
          child: Row(
            children: [
              Icon(Icons.chat_bubble_outline, size: size, color: _muted),
              const SizedBox(width: 4),
              Text('回复', style: TextStyle(color: _muted, fontSize: compact ? 11 : 12)),
            ],
          ),
        ),
        const SizedBox(width: 16),
        GestureDetector(
          onTap: () => _toggleLike(comment),
          child: Row(
            children: [
              Icon(
                comment.liked ? Icons.favorite : Icons.favorite_border,
                size: size,
                color: comment.liked ? const Color(0xFFEF4444) : _muted,
              ),
              const SizedBox(width: 4),
              Text(
                comment.likeCount > 0 ? '${comment.likeCount}' : '赞',
                style: TextStyle(
                  color: comment.liked ? const Color(0xFFEF4444) : _muted,
                  fontSize: compact ? 11 : 12,
                ),
              ),
            ],
          ),
        ),
      ],
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
