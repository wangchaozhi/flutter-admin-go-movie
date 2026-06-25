import 'package:flutter_test/flutter_test.dart';
import 'package:mobile/core/api_client.dart';
import 'package:mobile/features/video/playback_progress_service.dart';

/// Records calls and returns canned responses so the service can be tested
/// without a network.
class _FakeApiClient extends ApiClient {
  _FakeApiClient({this.getResponse = const {'code': 0}, this.throwOnGet = false});

  final Map<String, dynamic> getResponse;
  final bool throwOnGet;

  String? lastGetPath;
  String? lastPostPath;
  Map<String, dynamic>? lastPostBody;
  int postCount = 0;

  @override
  Future<Map<String, dynamic>> getAuth(String path) async {
    lastGetPath = path;
    if (throwOnGet) throw const ApiException('boom');
    return getResponse;
  }

  @override
  Future<Map<String, dynamic>> postAuth(
    String path,
    Map<String, dynamic> body,
  ) async {
    postCount++;
    lastPostPath = path;
    lastPostBody = body;
    return const {'code': 0};
  }
}

void main() {
  group('shouldSaveProgress', () {
    const noLast = Duration.zero;

    test('rejects a negative position', () {
      expect(
        shouldSaveProgress(
          position: const Duration(seconds: -1),
          lastSaved: noLast,
          force: true,
          isExplicitPosition: true,
        ),
        isFalse,
      );
    });

    test('rejects a bare zero position but allows an explicit one', () {
      expect(
        shouldSaveProgress(
          position: Duration.zero,
          lastSaved: const Duration(seconds: 30),
          force: false,
          isExplicitPosition: false,
        ),
        isFalse,
      );
      expect(
        shouldSaveProgress(
          position: Duration.zero,
          lastSaved: const Duration(seconds: 30),
          force: false,
          isExplicitPosition: true,
        ),
        isTrue,
      );
    });

    test('throttles moves smaller than 5s unless forced', () {
      expect(
        shouldSaveProgress(
          position: const Duration(seconds: 12),
          lastSaved: const Duration(seconds: 10),
          force: false,
          isExplicitPosition: false,
        ),
        isFalse,
      );
      expect(
        shouldSaveProgress(
          position: const Duration(seconds: 12),
          lastSaved: const Duration(seconds: 10),
          force: true,
          isExplicitPosition: false,
        ),
        isTrue,
      );
    });

    test('allows a move of 5s or more', () {
      expect(
        shouldSaveProgress(
          position: const Duration(seconds: 15),
          lastSaved: const Duration(seconds: 10),
          force: false,
          isExplicitPosition: false,
        ),
        isTrue,
      );
    });
  });

  group('canResumeAt', () {
    test('always resumes when duration is unknown', () {
      expect(canResumeAt(const Duration(seconds: 90), Duration.zero), isTrue);
    });

    test('resumes when far from the end', () {
      expect(
        canResumeAt(const Duration(minutes: 1), const Duration(minutes: 10)),
        isTrue,
      );
    });

    test('does not resume within the final 20s', () {
      expect(
        canResumeAt(
          const Duration(seconds: 595),
          const Duration(seconds: 600),
        ),
        isFalse,
      );
    });
  });

  group('VideoProgressService.load', () {
    test('returns the saved position from the envelope', () async {
      final client = _FakeApiClient(
        getResponse: const {
          'code': 0,
          'data': {'position': 125},
        },
      );
      final service = VideoProgressService(videoId: 7, client: client);

      expect(await service.load(), const Duration(seconds: 125));
      expect(client.lastGetPath, '/api/videos/7/progress');
    });

    test('returns zero on a non-success code', () async {
      final service = VideoProgressService(
        videoId: 1,
        client: _FakeApiClient(getResponse: const {'code': 1}),
      );
      expect(await service.load(), Duration.zero);
    });

    test('returns zero when the position is absent or non-positive', () async {
      final service = VideoProgressService(
        videoId: 1,
        client: _FakeApiClient(
          getResponse: const {
            'code': 0,
            'data': {'position': 0},
          },
        ),
      );
      expect(await service.load(), Duration.zero);
    });

    test('returns zero when the request throws', () async {
      final service = VideoProgressService(
        videoId: 1,
        client: _FakeApiClient(throwOnGet: true),
      );
      expect(await service.load(), Duration.zero);
    });
  });

  group('VideoProgressService.save', () {
    test('posts and advances lastSaved when allowed', () async {
      final client = _FakeApiClient();
      final service = VideoProgressService(videoId: 3, client: client);

      await service.save(const Duration(seconds: 42), const Duration(minutes: 5));

      expect(client.postCount, 1);
      expect(client.lastPostPath, '/api/videos/3/progress');
      expect(client.lastPostBody, {'position': 42, 'duration': 300});
      expect(service.lastSaved, const Duration(seconds: 42));
    });

    test('skips a throttled move without posting', () async {
      final client = _FakeApiClient();
      final service = VideoProgressService(videoId: 3, client: client);

      await service.save(const Duration(seconds: 10), const Duration(minutes: 5));
      // 2s later: below the 5s threshold, should be ignored.
      await service.save(const Duration(seconds: 12), const Duration(minutes: 5));

      expect(client.postCount, 1);
      expect(service.lastSaved, const Duration(seconds: 10));
    });
  });
}
