import 'dart:async';
import 'dart:convert';

import 'package:http/http.dart' as http;

import 'session.dart';

/// Thrown when the server reply cannot be turned into a JSON object (empty body
/// or non-JSON). Its [toString] is the user-facing message, so it surfaces a
/// clear reason instead of a cryptic `FormatException` from `jsonDecode`.
class ApiException implements Exception {
  final String message;
  const ApiException(this.message);

  @override
  String toString() => message;
}

class ApiClient {
  // Default targets the backend on the same machine. Android emulator
  // (10.0.2.2), a physical device (LAN IP) or web/desktop pointing elsewhere
  // should override this with --dart-define=API_BASE_URL=...
  static const String baseUrl = String.fromEnvironment(
    'API_BASE_URL',
    defaultValue: 'http://localhost:8080',
  );

  /// Network calls give up after this long so a hung or unreachable server
  /// surfaces an actionable error instead of an indefinite spinner.
  static const Duration timeout = Duration(seconds: 10);

  Future<Map<String, dynamic>> post(String path, Map<String, dynamic> body) {
    return _send('POST', path, body: body);
  }

  Future<Map<String, dynamic>> get(String path) {
    return _send('GET', path);
  }

  Future<Map<String, dynamic>> getAuth(String path) {
    return _send('GET', path, auth: true);
  }

  Future<Map<String, dynamic>> postAuth(String path, Map<String, dynamic> body) {
    return _send('POST', path, body: body, auth: true);
  }

  Future<Map<String, dynamic>> putAuth(String path, Map<String, dynamic> body) {
    return _send('PUT', path, body: body, auth: true);
  }

  Future<Map<String, dynamic>> deleteAuth(String path) {
    return _send('DELETE', path, auth: true);
  }

  /// Single entry point for every request: builds the URL, attaches the auth
  /// header when needed, encodes the JSON body, enforces a [timeout], and
  /// turns transport failures into a clear [ApiException].
  Future<Map<String, dynamic>> _send(
    String method,
    String path, {
    Map<String, dynamic>? body,
    bool auth = false,
  }) async {
    final uri = Uri.parse('$baseUrl$path');
    final headers = <String, String>{};
    if (body != null) headers['Content-Type'] = 'application/json';
    if (auth) {
      final token = await Session.token();
      if (token != null) headers['Authorization'] = 'Bearer $token';
    }
    final encodedBody = body == null ? null : jsonEncode(body);
    try {
      final Future<http.Response> request;
      switch (method) {
        case 'GET':
          request = http.get(uri, headers: headers);
          break;
        case 'POST':
          request = http.post(uri, headers: headers, body: encodedBody);
          break;
        case 'PUT':
          request = http.put(uri, headers: headers, body: encodedBody);
          break;
        case 'DELETE':
          request = http.delete(uri, headers: headers);
          break;
        default:
          throw ArgumentError('Unsupported HTTP method: $method');
      }
      final response = await request.timeout(timeout);
      return _decode(response, uri);
    } on TimeoutException {
      throw ApiException('请求超时，请检查网络或服务器地址：$uri');
    } on http.ClientException catch (e) {
      throw ApiException('无法连接服务器 (${e.message})，请检查服务器地址：$uri');
    }
  }

  /// Turns a response body into a JSON map, or throws an [ApiException] with a
  /// clear message. Guards against the common case of an empty / non-JSON body
  /// (e.g. wrong base URL, a proxy error page, or a gateway timeout), which
  /// would otherwise surface as `FormatException: Unexpected end of input`.
  Map<String, dynamic> _decode(http.Response response, Uri uri) {
    final body = response.body.trim();
    if (body.isEmpty) {
      throw ApiException(
        '服务器无响应 (HTTP ${response.statusCode})，请检查服务器地址：$uri',
      );
    }
    final dynamic decoded;
    try {
      decoded = jsonDecode(body);
    } on FormatException {
      throw ApiException(
        '服务器返回了无效数据 (HTTP ${response.statusCode})，请检查服务器地址：$uri',
      );
    }
    if (decoded is Map<String, dynamic>) {
      return decoded;
    }
    throw ApiException(
      '服务器返回了非预期数据 (HTTP ${response.statusCode})：$uri',
    );
  }
}
