import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:media_kit/media_kit.dart';
import 'package:media_kit_video/media_kit_video.dart';

import '../../core/api_client.dart';
import '../../models/video.dart' as model;

class VideoPlayerPage extends StatefulWidget {
  final model.Video video;

  const VideoPlayerPage({super.key, required this.video});

  @override
  State<VideoPlayerPage> createState() => _VideoPlayerPageState();
}

class _VideoPlayerPageState extends State<VideoPlayerPage> {
  late final Player _player;
  late final VideoController _controller;
  bool _loading = true;
  String? _error;

  @override
  void initState() {
    super.initState();
    _player = Player();
    _controller = VideoController(_player);
    SystemChrome.setPreferredOrientations([
      DeviceOrientation.landscapeLeft,
      DeviceOrientation.landscapeRight,
      DeviceOrientation.portraitUp,
    ]);
    _init();
  }

  @override
  void dispose() {
    SystemChrome.setPreferredOrientations([DeviceOrientation.portraitUp]);
    _player.dispose();
    super.dispose();
  }

  Future<void> _init() async {
    try {
      final resp = await ApiClient().getAuth('/api/videos/${widget.video.id}/play');
      if (resp['code'] != 0) {
        if (mounted) setState(() { _error = resp['msg']?.toString() ?? '获取播放地址失败'; _loading = false; });
        return;
      }
      final rawUrl = (resp['data'] as Map<String, dynamic>?)?['url'] as String? ?? '';
      if (rawUrl.isEmpty) {
        if (mounted) setState(() { _error = '播放地址为空'; _loading = false; });
        return;
      }
      // backend returns a relative signed path; prepend baseUrl so it works on
      // emulator (10.0.2.2), simulator (localhost), and physical device (LAN IP)
      final url = rawUrl.startsWith('http') ? rawUrl : '${ApiClient.baseUrl}$rawUrl';
      await _player.open(Media(url));
      if (mounted) setState(() => _loading = false);
    } catch (e) {
      if (mounted) setState(() { _error = e.toString(); _loading = false; });
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: Colors.black,
      body: SafeArea(
        child: Column(
          children: [
            AspectRatio(
              aspectRatio: 16 / 9,
              child: _loading
                  ? const Center(child: CircularProgressIndicator(color: Color(0xFF25D0AB)))
                  : _error != null
                      ? _buildError()
                      : Video(
                          controller: _controller,
                          controls: AdaptiveVideoControls,
                        ),
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
                          icon: const Icon(Icons.arrow_back, color: Colors.white),
                          onPressed: () => Navigator.pop(context),
                        ),
                        Expanded(
                          child: Text(
                            widget.video.title,
                            style: const TextStyle(color: Colors.white, fontSize: 17, fontWeight: FontWeight.w900),
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
                        child: Text(widget.video.categoryName, style: const TextStyle(color: Color(0xFF25D0AB), fontSize: 13)),
                      ),
                    ],
                    if (widget.video.description.isNotEmpty) ...[
                      const SizedBox(height: 12),
                      Text(widget.video.description, style: const TextStyle(color: Color(0xFF9CA3AF), height: 1.5)),
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

  Widget _buildError() {
    return Center(
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          const Icon(Icons.error_outline, color: Colors.redAccent, size: 48),
          const SizedBox(height: 12),
          Text(_error!, style: const TextStyle(color: Colors.white70), textAlign: TextAlign.center),
          const SizedBox(height: 16),
          TextButton(
            onPressed: () => Navigator.pop(context),
            child: const Text('返回', style: TextStyle(color: Color(0xFF25D0AB))),
          ),
        ],
      ),
    );
  }
}
