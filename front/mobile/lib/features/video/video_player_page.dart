import 'dart:async';

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
  bool _switchingQuality = false;
  String? _error;
  String _selectedQuality = 'auto';
  List<_QualityOption> _qualities = const [];

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
      final resp = await ApiClient().getAuth(
        '/api/videos/${widget.video.id}/play',
      );
      if (resp['code'] != 0) {
        if (mounted) {
          setState(() {
            _error = resp['msg']?.toString() ?? '获取播放地址失败';
            _loading = false;
          });
        }
        return;
      }
      final rawUrl =
          (resp['data'] as Map<String, dynamic>?)?['url'] as String? ?? '';
      if (rawUrl.isEmpty) {
        if (mounted) {
          setState(() {
            _error = '播放地址为空';
            _loading = false;
          });
        }
        return;
      }
      // backend returns a relative signed path; prepend baseUrl so it works on
      // emulator (10.0.2.2), simulator (localhost), and physical device (LAN IP)
      final url = _absoluteUrl(rawUrl);
      var qualities = _parseQualities(resp['data'] as Map<String, dynamic>?);
      if (qualities.isEmpty) {
        qualities = await _parseQualitiesFromMaster(url);
      }
      await _player.open(Media(url));
      if (mounted) {
        setState(() {
          _qualities = [
            _QualityOption(name: 'auto', label: '自动', url: url),
            ...qualities,
          ];
          _loading = false;
        });
      }
    } catch (e) {
      if (mounted) {
        setState(() {
          _error = e.toString();
          _loading = false;
        });
      }
    }
  }

  String _absoluteUrl(String url) {
    return url.startsWith('http') ? url : '${ApiClient.baseUrl}$url';
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

  Future<void> _switchQuality(String name) async {
    if (name == _selectedQuality || _switchingQuality) return;
    final option = _qualities.where((q) => q.name == name).firstOrNull;
    if (option == null) return;

    final position = _player.state.position;
    final wasPlaying = _player.state.playing;
    setState(() {
      _selectedQuality = name;
      _switchingQuality = true;
    });

    try {
      await _player.open(Media(option.url), play: false);
      await _waitForMediaReady();
      if (position > Duration.zero) {
        await _player.seek(position);
      }
      if (wasPlaying) {
        await _player.play();
      } else {
        await _player.pause();
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text('${option.label} 加载失败：$e')));
      }
    } finally {
      if (mounted) setState(() => _switchingQuality = false);
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
        Positioned(right: 48, bottom: 4, child: _buildQualityMenu()),
      ],
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
          TextButton(
            onPressed: () => Navigator.pop(context),
            child: const Text('返回', style: TextStyle(color: Color(0xFF25D0AB))),
          ),
        ],
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
      child: SizedBox(
        height: 48,
        child: Padding(
          padding: const EdgeInsets.symmetric(horizontal: 8),
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
                      color: Color(0xFF25D0AB),
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
                const Icon(Icons.expand_more, color: Colors.white, size: 18),
              ],
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
}
